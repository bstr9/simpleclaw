// Package memory 提供 Agent 记忆管理功能。
// long_term.go 实现基于 SQLite 的长期存储，支持向量搜索。
package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// errMemoryClosed 表示记忆存储已关闭的错误
var errMemoryClosed = errors.New("memory is closed")

// SQL 相关常量
const (
	// sqlTimestamp 当前时间戳的 SQL 表达式
	sqlTimestamp = "strftime('%s', 'now')"
	// sqlCountFrom SELECT COUNT(*) FROM 前缀
	sqlCountFrom = "SELECT COUNT(*) FROM "
	// sqlDeleteFrom DELETE FROM 前缀
	sqlDeleteFrom = "DELETE FROM "
)

// LongTermMemory 实现基于 SQLite 的长期记忆存储。
// 支持向量相似度搜索、关键词搜索和混合搜索。
type LongTermMemory struct {
	mu sync.RWMutex

	// db 是 SQLite 数据库连接。
	db *sql.DB

	// dbPath 是 SQLite 数据库文件路径。
	dbPath string

	// config 是记忆配置。
	config *Config

	// embedder 是用于向量搜索的嵌入提供者。
	embedder Embedder

	// chunker 是用于分割内容的文本分块器。
	chunker *TextChunker

	// dirty 表示是否有未同步的更改。
	dirty bool

	// closed 表示记忆是否已关闭。
	closed bool
}

// LongTermOption 是 LongTermMemory 的函数式选项。
type LongTermOption func(*LongTermMemory)

// WithConfig 设置配置。
func WithConfigConfig(cfg *Config) LongTermOption {
	return func(m *LongTermMemory) {
		m.config = cfg
	}
}

// WithEmbedder 设置嵌入提供者。
func WithEmbedder(embedder Embedder) LongTermOption {
	return func(m *LongTermMemory) {
		m.embedder = embedder
	}
}

// WithDBPath 设置数据库路径。
func WithDBPath(path string) LongTermOption {
	return func(m *LongTermMemory) {
		m.dbPath = path
	}
}

// NewLongTermMemory 创建新的 LongTermMemory 实例。
func NewLongTermMemory(opts ...LongTermOption) (*LongTermMemory, error) {
	m := &LongTermMemory{
		config:  DefaultConfig(),
		chunker: NewTextChunker(500, 50),
		dirty:   false,
		closed:  false,
	}

	for _, opt := range opts {
		opt(m)
	}

	// 设置默认数据库路径（如果未提供）
	if m.dbPath == "" {
		m.dbPath = filepath.Join(m.config.WorkspaceRoot, "memory", "long-term", "index.db")
	}

	// 初始化数据库
	if err := m.initDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return m, nil
}

// initDB 初始化 SQLite 数据库并创建所需的表结构。
func (m *LongTermMemory) initDB() error {
	// 确保目录存在
	dir := filepath.Dir(m.dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// 打开数据库连接
	db, err := sql.Open("sqlite", m.dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(1) // SQLite 单连接效果最佳
	db.SetMaxIdleConns(1)

	m.db = db

	// 创建表
	schema := `
	CREATE TABLE IF NOT EXISTS chunks (
		id TEXT PRIMARY KEY,
		user_id TEXT,
		scope TEXT NOT NULL DEFAULT 'shared',
		source TEXT NOT NULL DEFAULT 'memory',
		path TEXT NOT NULL,
		start_line INTEGER NOT NULL,
		end_line INTEGER NOT NULL,
		text TEXT NOT NULL,
		embedding TEXT,
		hash TEXT NOT NULL,
		metadata TEXT,
		created_at INTEGER DEFAULT (strftime('%s', 'now')),
		updated_at INTEGER DEFAULT (strftime('%s', 'now'))
	);

	CREATE INDEX IF NOT EXISTS idx_chunks_user ON chunks(user_id);
	CREATE INDEX IF NOT EXISTS idx_chunks_scope ON chunks(scope);
	CREATE INDEX IF NOT EXISTS idx_chunks_hash ON chunks(path, hash);
	CREATE INDEX IF NOT EXISTS idx_chunks_text ON chunks(text);

	CREATE TABLE IF NOT EXISTS files (
		path TEXT PRIMARY KEY,
		source TEXT NOT NULL DEFAULT 'memory',
		hash TEXT NOT NULL,
		mtime INTEGER NOT NULL,
		size INTEGER NOT NULL,
		updated_at INTEGER DEFAULT (strftime('%s', 'now'))
	);

	CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT PRIMARY KEY,
		channel_type TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		last_active INTEGER NOT NULL,
		msg_count INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		seq INTEGER NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		UNIQUE (session_id, seq)
	);

	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, seq);
	CREATE INDEX IF NOT EXISTS idx_sessions_last_active ON sessions(last_active);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Add 将新消息存储到长期记忆中。
func (m *LongTermMemory) Add(ctx context.Context, msg *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errMemoryClosed
	}

	// 确保消息包含必要字段
	if msg.ID == "" {
		msg.ID = generateID(msg.Content)
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	// 对内容进行分块
	chunks := m.chunker.ChunkText(msg.Content)
	if len(chunks) == 0 {
		return nil
	}

	// 如果有嵌入器则生成向量
	var embeddings [][]float64
	var err error
	if m.embedder != nil {
		texts := make([]string, len(chunks))
		for i, chunk := range chunks {
			texts[i] = chunk.Text
		}
		embeddings, err = m.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			// 记录警告但继续执行（不带向量）
			embeddings = make([][]float64, len(chunks))
		}
	} else {
		embeddings = make([][]float64, len(chunks))
	}

	// 将分块保存到数据库
	for i, chunk := range chunks {
		chunkID := generateID(chunk.Text + string(rune(chunk.StartLine)))
		hash := ComputeHash(chunk.Text)

		var embeddingJSON string
		if embeddings[i] != nil {
			embeddingJSON = ToJSON(embeddings[i])
		}

		_, err := m.db.Exec(`
			INSERT OR REPLACE INTO chunks 
			(id, user_id, scope, source, path, start_line, end_line, text, embedding, hash, metadata, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, `+sqlTimestamp+`)
		`, chunkID, "", ScopeShared, SourceSession, "session", chunk.StartLine, chunk.EndLine,
			chunk.Text, embeddingJSON, hash, ToJSON(msg.Metadata))
		if err != nil {
			return fmt.Errorf("failed to save chunk: %w", err)
		}
	}

	m.dirty = true
	return nil
}

// Get 根据查询从长期记忆中检索消息。
func (m *LongTermMemory) Get(ctx context.Context, query string, limit int) ([]*Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, errMemoryClosed
	}

	if limit <= 0 {
		limit = m.config.MaxResults
	}

	// 执行混合搜索
	opts := &SearchOptions{
		Scopes:        []MemoryScope{ScopeShared, ScopeUser},
		MaxResults:    limit,
		MinScore:      m.config.MinScore,
		IncludeShared: true,
	}

	results, err := m.Search(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	// 将搜索结果转换为消息
	messages := make([]*Message, 0, len(results))
	for _, result := range results {
		messages = append(messages, &Message{
			ID:        generateID(result.Snippet),
			Role:      RoleUser, // 默认角色
			Content:   result.Snippet,
			CreatedAt: time.Now(),
			Metadata: map[string]any{
				"path":       result.Path,
				"start_line": result.StartLine,
				"end_line":   result.EndLine,
				"score":      result.Score,
				"source":     result.Source,
			},
		})
	}

	return messages, nil
}

// Clear 清除长期记忆中的所有消息。
func (m *LongTermMemory) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errMemoryClosed
	}

	// 清除分块
	if _, err := m.db.Exec(sqlDeleteFrom + "chunks"); err != nil {
		return fmt.Errorf("failed to clear chunks: %w", err)
	}

	// 清除文件元数据
	if _, err := m.db.Exec(sqlDeleteFrom + "files"); err != nil {
		return fmt.Errorf("failed to clear files: %w", err)
	}

	// 清除消息
	if _, err := m.db.Exec(sqlDeleteFrom + "messages"); err != nil {
		return fmt.Errorf("failed to clear messages: %w", err)
	}

	// 清除会话
	if _, err := m.db.Exec(sqlDeleteFrom + "sessions"); err != nil {
		return fmt.Errorf("failed to clear sessions: %w", err)
	}

	m.dirty = false
	return nil
}

// Summarize 生成已存储记忆的摘要。
func (m *LongTermMemory) Summarize(ctx context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return "", errMemoryClosed
	}

	// 获取统计信息
	stats, err := m.GetStats()
	if err != nil {
		return "", err
	}

	summary := "长期记忆摘要:\n"
	summary += "-----------------------\n"
	summary += fmt.Sprintf("总分块数: %d\n", stats["chunks"])
	summary += fmt.Sprintf("总文件数: %d\n", stats["files"])
	summary += fmt.Sprintf("总会话数: %d\n", stats["sessions"])
	summary += fmt.Sprintf("总消息数: %d\n", stats["messages"])
	summary += fmt.Sprintf("数据库路径: %s\n", m.dbPath)
	summary += fmt.Sprintf("向量嵌入启用: %v\n", m.embedder != nil)

	return summary, nil
}

// Search 执行混合搜索（向量 + 关键词）并返回结果。
func (m *LongTermMemory) Search(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, errMemoryClosed
	}

	if opts == nil {
		opts = DefaultSearchOptions()
	}

	// 如需要且已修改则同步
	if m.config.SyncOnSearch && m.dirty {
		m.mu.RUnlock()
		if err := m.Sync(ctx, false); err != nil {
			m.mu.RLock()
		}
		m.mu.RLock()
	}

	var vectorResults []*SearchResult
	var keywordResults []*SearchResult

	// 如果有嵌入器则执行向量搜索
	if m.embedder != nil {
		embedding, err := m.embedder.Embed(ctx, query)
		if err == nil {
			vectorResults, _ = m.SearchVector(ctx, embedding, opts)
		}
	}

	// 执行关键词搜索
	keywordResults, _ = m.SearchKeyword(ctx, query, opts)

	// 合并结果
	merged := m.mergeResults(vectorResults, keywordResults, m.config.VectorWeight, m.config.KeywordWeight)

	// 按最低分数和数量过滤
	filtered := make([]*SearchResult, 0, opts.MaxResults)
	for _, r := range merged {
		if r.Score >= opts.MinScore {
			filtered = append(filtered, r)
			if len(filtered) >= opts.MaxResults {
				break
			}
		}
	}

	return filtered, nil
}

// SearchVector 执行向量相似度搜索。
func (m *LongTermMemory) SearchVector(ctx context.Context, embedding []float64, opts *SearchOptions) ([]*SearchResult, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	query, args := m.buildVectorSearchQuery(opts)
	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("vector search query failed: %w", err)
	}
	defer rows.Close()

	results := m.scanAndScoreVectorResults(rows, embedding)
	sortResultsByScore(results)
	return m.convertToSearchResults(results, opts.MaxResults), nil
}

// buildVectorSearchQuery 构建向量搜索查询语句和参数。
func (m *LongTermMemory) buildVectorSearchQuery(opts *SearchOptions) (string, []any) {
	query := "SELECT id, path, start_line, end_line, text, source, user_id, embedding FROM chunks WHERE embedding IS NOT NULL"
	args := []any{}

	if len(opts.Scopes) > 0 {
		placeholders := make([]string, len(opts.Scopes))
		for i, scope := range opts.Scopes {
			placeholders[i] = "?"
			args = append(args, string(scope))
		}
		query += " AND scope IN (" + strings.Join(placeholders, ",") + ")"
	}

	if opts.UserID != "" {
		query += " AND (scope = 'shared' OR user_id = ?)"
		args = append(args, opts.UserID)
	}

	query += " LIMIT ?"
	args = append(args, opts.MaxResults*2)
	return query, args
}

// vectorScoredRow 表示带分数的向量搜索行结果。
type vectorScoredRow struct {
	score     float64
	id        string
	path      string
	startLine int
	endLine   int
	text      string
	source    MemorySource
	userID    string
}

// scanAndScoreVectorResults 扫描数据库行并计算向量相似度。
func (m *LongTermMemory) scanAndScoreVectorResults(rows *sql.Rows, embedding []float64) []vectorScoredRow {
	var results []vectorScoredRow
	for rows.Next() {
		var r vectorScoredRow
		var source, embeddingJSON string
		if err := rows.Scan(&r.id, &r.path, &r.startLine, &r.endLine, &r.text, &source, &r.userID, &embeddingJSON); err != nil {
			continue
		}
		r.source = MemorySource(source)

		var storedEmbedding []float64
		if err := FromJSON(embeddingJSON, &storedEmbedding); err != nil {
			continue
		}

		similarity := cosineSimilarity(embedding, storedEmbedding)
		if similarity > 0 {
			r.score = similarity
			results = append(results, r)
		}
	}
	return results
}

// convertToSearchResults 将带分数的行结果转换为搜索结果。
func (m *LongTermMemory) convertToSearchResults(results []vectorScoredRow, maxResults int) []*SearchResult {
	searchResults := make([]*SearchResult, 0, maxResults)
	for i, r := range results {
		if i >= maxResults {
			break
		}
		searchResults = append(searchResults, &SearchResult{
			Path:      r.path,
			StartLine: r.startLine,
			EndLine:   r.endLine,
			Score:     r.score,
			Snippet:   truncateText(r.text, 500),
			Source:    r.source,
			UserID:    r.userID,
		})
	}
	return searchResults
}

// sortResultsByScore 按分数降序排序结果。
func sortResultsByScore(results []vectorScoredRow) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// SearchKeyword 执行基于关键词的 LIKE 模式搜索。
func (m *LongTermMemory) SearchKeyword(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error) {
	if opts == nil {
		opts = DefaultSearchOptions()
	}

	keywords := extractKeywords(query)
	if len(keywords) == 0 {
		return nil, nil
	}

	sqlQuery, args := m.buildKeywordSearchQuery(query, keywords, opts)
	rows, err := m.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("keyword search query failed: %w", err)
	}
	defer rows.Close()

	return m.scanKeywordResults(rows, keywords), nil
}

// buildKeywordSearchQuery 构建关键词搜索查询。
func (m *LongTermMemory) buildKeywordSearchQuery(_ string, keywords []string, opts *SearchOptions) (string, []any) {
	likeConditions := make([]string, len(keywords))
	args := []any{}
	for _, kw := range keywords {
		likeConditions = append(likeConditions, "text LIKE ?")
		args = append(args, "%"+kw+"%")
	}

	sqlQuery := "SELECT id, path, start_line, end_line, text, source, user_id FROM chunks WHERE (" +
		strings.Join(likeConditions, " OR ") + ")"

	if len(opts.Scopes) > 0 {
		placeholders := make([]string, len(opts.Scopes))
		for i, scope := range opts.Scopes {
			placeholders[i] = "?"
			args = append(args, string(scope))
		}
		sqlQuery += " AND scope IN (" + strings.Join(placeholders, ",") + ")"
	}

	if opts.UserID != "" {
		sqlQuery += " AND (scope = 'shared' OR user_id = ?)"
		args = append(args, opts.UserID)
	}

	sqlQuery += " LIMIT ?"
	args = append(args, opts.MaxResults*2)
	return sqlQuery, args
}

// scanKeywordResults 扫描关键词搜索结果并计算分数。
func (m *LongTermMemory) scanKeywordResults(rows *sql.Rows, keywords []string) []*SearchResult {
	searchResults := make([]*SearchResult, 0)
	for rows.Next() {
		var id, path, text string
		var startLine, endLine int
		var source string
		var userID sql.NullString

		if err := rows.Scan(&id, &path, &startLine, &endLine, &text, &source, &userID); err != nil {
			continue
		}

		score := calculateKeywordScore(text, keywords)
		searchResults = append(searchResults, &SearchResult{
			Path:      path,
			StartLine: startLine,
			EndLine:   endLine,
			Score:     score,
			Snippet:   truncateText(text, 500),
			Source:    MemorySource(source),
			UserID:    userID.String,
		})
	}
	return searchResults
}

// Sync 从文件同步记忆。
func (m *LongTermMemory) Sync(ctx context.Context, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errMemoryClosed
	}

	memoryDir := filepath.Join(m.config.WorkspaceRoot, "memory")
	if _, err := os.Stat(memoryDir); os.IsNotExist(err) {
		return nil
	}

	err := filepath.Walk(memoryDir, func(path string, info os.FileInfo, err error) error {
		return m.syncMemoryFile(ctx, path, info, err, force)
	})

	m.dirty = false
	return err
}

// syncMemoryFile 同步单个记忆文件。
func (m *LongTermMemory) syncMemoryFile(ctx context.Context, path string, info os.FileInfo, err error, force bool) error {
	if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
		return nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	relPath, _ := filepath.Rel(m.config.WorkspaceRoot, path)
	hash := ComputeHash(string(content))

	storedHash, _ := m.GetFileHash(relPath)
	if storedHash == hash && !force {
		return nil
	}

	m.deleteByPath(relPath)

	chunks := m.chunker.ChunkText(string(content))
	if len(chunks) == 0 {
		return nil
	}

	embeddings := m.generateEmbeddings(ctx, chunks)
	scope := m.determineScopeFromPath(path)
	m.saveSyncedChunks(relPath, chunks, embeddings, scope)
	m.updateFileMetadata(relPath, SourceMemory, hash, int(info.ModTime().Unix()), int(info.Size()))

	return nil
}

// determineScopeFromPath 从路径确定作用域。
func (m *LongTermMemory) determineScopeFromPath(path string) MemoryScope {
	if strings.Contains(path, "users") {
		return ScopeUser
	}
	return ScopeShared
}

// saveSyncedChunks 保存同步的分块到数据库。
func (m *LongTermMemory) saveSyncedChunks(relPath string, chunks []TextChunk, embeddings [][]float64, scope MemoryScope) {
	for i, chunk := range chunks {
		chunkID := generateID(relPath + string(rune(chunk.StartLine)))
		chunkHash := ComputeHash(chunk.Text)

		var embeddingJSON string
		if embeddings[i] != nil {
			embeddingJSON = ToJSON(embeddings[i])
		}

		m.db.Exec(`
			INSERT OR REPLACE INTO chunks 
			(id, scope, source, path, start_line, end_line, text, embedding, hash, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, `+sqlTimestamp+`)
		`, chunkID, scope, SourceMemory, relPath, chunk.StartLine, chunk.EndLine,
			chunk.Text, embeddingJSON, chunkHash)
	}
}

// AddMemory 将内容添加到长期记忆。
func (m *LongTermMemory) AddMemory(ctx context.Context, content string, userID string, scope MemoryScope, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errMemoryClosed
	}

	if strings.TrimSpace(content) == "" {
		return nil
	}

	path = m.resolveMemoryPath(path, content, userID, scope)
	chunks := m.chunker.ChunkText(content)
	embeddings := m.generateEmbeddings(ctx, chunks)

	if err := m.saveChunks(chunks, embeddings, path, userID, scope); err != nil {
		return err
	}

	fileHash := ComputeHash(content)
	m.updateFileMetadata(path, SourceMemory, fileHash, int(time.Now().Unix()), len(content))
	m.dirty = true
	return nil
}

// resolveMemoryPath 解析或生成记忆路径
func (m *LongTermMemory) resolveMemoryPath(path, content, userID string, scope MemoryScope) string {
	if path != "" {
		return path
	}
	hash := ComputeHash(content)[:8]
	if userID != "" && scope == ScopeUser {
		return filepath.Join("memory", "users", userID, "memory_"+hash+".md")
	}
	return filepath.Join("memory", "shared", "memory_"+hash+".md")
}

// generateEmbeddings 生成嵌入向量
func (m *LongTermMemory) generateEmbeddings(ctx context.Context, chunks []TextChunk) [][]float64 {
	if m.embedder == nil {
		return make([][]float64, len(chunks))
	}

	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Text
	}

	embeddings, err := m.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return make([][]float64, len(chunks))
	}
	return embeddings
}

// saveChunks 保存分块到数据库
func (m *LongTermMemory) saveChunks(chunks []TextChunk, embeddings [][]float64, path, userID string, scope MemoryScope) error {
	for i, chunk := range chunks {
		chunkID := generateID(path + string(rune(chunk.StartLine)))
		chunkHash := ComputeHash(chunk.Text)

		var embeddingJSON string
		if embeddings[i] != nil {
			embeddingJSON = ToJSON(embeddings[i])
		}

		_, err := m.db.Exec(`
			INSERT OR REPLACE INTO chunks 
			(id, user_id, scope, source, path, start_line, end_line, text, embedding, hash, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, `+sqlTimestamp+`)
		`, chunkID, userID, scope, SourceMemory, path, chunk.StartLine, chunk.EndLine,
			chunk.Text, embeddingJSON, chunkHash)
		if err != nil {
			return fmt.Errorf("failed to save chunk: %w", err)
		}
	}
	return nil
}

// GetFileHash 获取文件的存储哈希值。
func (m *LongTermMemory) GetFileHash(path string) (string, error) {
	var hash string
	err := m.db.QueryRow("SELECT hash FROM files WHERE path = ?", path).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return hash, err
}

// deleteByPath 删除指定文件路径的所有分块。
func (m *LongTermMemory) deleteByPath(path string) {
	m.db.Exec(sqlDeleteFrom+"chunks WHERE path = ?", path)
}

// updateFileMetadata 更新文件元数据表。
func (m *LongTermMemory) updateFileMetadata(path string, source MemorySource, hash string, mtime int, size int) {
	m.db.Exec(`
		INSERT OR REPLACE INTO files (path, source, hash, mtime, size, updated_at)
		VALUES (?, ?, ?, ?, ?, `+sqlTimestamp+`)
	`, path, source, hash, mtime, size)
}

// GetStats 返回记忆存储的统计信息。
func (m *LongTermMemory) GetStats() (map[string]any, error) {
	stats := make(map[string]any)

	var chunksCount int
	err := m.db.QueryRow(sqlCountFrom + "chunks").Scan(&chunksCount)
	if err != nil {
		return nil, err
	}
	stats["chunks"] = chunksCount

	var filesCount int
	err = m.db.QueryRow(sqlCountFrom + "files").Scan(&filesCount)
	if err != nil {
		return nil, err
	}
	stats["files"] = filesCount

	var sessionsCount int
	err = m.db.QueryRow(sqlCountFrom + "sessions").Scan(&sessionsCount)
	if err != nil {
		return nil, err
	}
	stats["sessions"] = sessionsCount

	var messagesCount int
	err = m.db.QueryRow(sqlCountFrom + "messages").Scan(&messagesCount)
	if err != nil {
		return nil, err
	}
	stats["messages"] = messagesCount

	stats["workspace"] = m.config.WorkspaceRoot
	stats["dirty"] = m.dirty
	stats["embedding_enabled"] = m.embedder != nil
	stats["search_mode"] = "hybrid (vector + keyword)"
	if m.embedder == nil {
		stats["search_mode"] = "keyword only"
	}

	return stats, nil
}

// CleanupOldMessages 删除超过 maxAgeDays 天的消息。
func (m *LongTermMemory) CleanupOldMessages(maxAgeDays int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, errMemoryClosed
	}

	if maxAgeDays <= 0 {
		maxAgeDays = m.config.MaxAgeDays
	}

	cutoff := time.Now().Add(-time.Duration(maxAgeDays) * 24 * time.Hour).Unix()

	result, err := m.db.Exec(sqlDeleteFrom+"messages WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}

	deleted, _ := result.RowsAffected()
	return int(deleted), nil
}

// mergeResults 合并向量和关键词搜索结果。
func (m *LongTermMemory) mergeResults(vectorResults, keywordResults []*SearchResult, vectorWeight, keywordWeight float64) []*SearchResult {
	mergedMap := make(map[string]*SearchResult)

	m.addVectorResultsToMap(mergedMap, vectorResults, vectorWeight)
	m.addKeywordResultsToMap(mergedMap, keywordResults, keywordWeight)
	results := m.applyTemporalDecay(mergedMap)
	sortSearchResultsByScore(results)

	return results
}

// addVectorResultsToMap 将向量结果添加到合并映射。
func (m *LongTermMemory) addVectorResultsToMap(mergedMap map[string]*SearchResult, results []*SearchResult, weight float64) {
	for _, r := range results {
		key := fmt.Sprintf("%s:%d:%d", r.Path, r.StartLine, r.EndLine)
		mergedMap[key] = &SearchResult{
			Path:      r.Path,
			StartLine: r.StartLine,
			EndLine:   r.EndLine,
			Score:     r.Score * weight,
			Snippet:   r.Snippet,
			Source:    r.Source,
			UserID:    r.UserID,
		}
	}
}

// addKeywordResultsToMap 将关键词结果添加或合并到映射。
func (m *LongTermMemory) addKeywordResultsToMap(mergedMap map[string]*SearchResult, results []*SearchResult, weight float64) {
	for _, r := range results {
		key := fmt.Sprintf("%s:%d:%d", r.Path, r.StartLine, r.EndLine)
		if existing, ok := mergedMap[key]; ok {
			existing.Score += r.Score * weight
		} else {
			mergedMap[key] = &SearchResult{
				Path:      r.Path,
				StartLine: r.StartLine,
				EndLine:   r.EndLine,
				Score:     r.Score * weight,
				Snippet:   r.Snippet,
				Source:    r.Source,
				UserID:    r.UserID,
			}
		}
	}
}

// applyTemporalDecay 应用时间衰减并转换为切片。
func (m *LongTermMemory) applyTemporalDecay(mergedMap map[string]*SearchResult) []*SearchResult {
	results := make([]*SearchResult, 0, len(mergedMap))
	for _, r := range mergedMap {
		decay := computeTemporalDecay(r.Path, 30.0)
		r.Score *= decay
		results = append(results, r)
	}
	return results
}

// sortSearchResultsByScore 按分数降序排序搜索结果。
func sortSearchResultsByScore(results []*SearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// MarkDirty 标记记忆需要同步。
func (m *LongTermMemory) MarkDirty() {
	m.dirty = true
}

// Close 释放数据库资源。
func (m *LongTermMemory) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// 辅助函数

// cosineSimilarity 计算两个向量的余弦相似度。
func cosineSimilarity(vec1, vec2 []float64) float64 {
	if len(vec1) != len(vec2) {
		return 0.0
	}

	dotProduct := 0.0
	norm1 := 0.0
	norm2 := 0.0

	for i := range vec1 {
		dotProduct += vec1[i] * vec2[i]
		norm1 += vec1[i] * vec1[i]
		norm2 += vec2[i] * vec2[i]
	}

	if norm1 == 0 || norm2 == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

// extractKeywords 从查询字符串中提取关键词。
func extractKeywords(query string) []string {
	// 简单关键词提取：按空白和标点分割
	re := regexp.MustCompile(`[\p{L}\p{N}]+`)
	words := re.FindAllString(query, -1)

	// 过滤短词并转为小写
	keywords := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) >= 2 {
			keywords = append(keywords, strings.ToLower(word))
		}
	}

	return keywords
}

// truncateText 将文本截断到 maxChars 字符。
func truncateText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + "..."
}

// computeTemporalDecay 计算日期记忆文件的时间衰减系数。
func computeTemporalDecay(path string, halfLifeDays float64) float64 {
	// 检查文件名中的日期模式 (YYYY-MM-DD.md)
	re := regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})\.md$`)
	match := re.FindStringSubmatch(path)
	if match == nil {
		return 1.0 // 常青文件无衰减
	}

	// 解析日期
	year := parseInt(match[1])
	month := parseInt(match[2])
	day := parseInt(match[3])
	if year == 0 || month == 0 || day == 0 {
		return 1.0
	}

	// 计算文件年龄
	fileDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	ageDays := time.Since(fileDate).Hours() / 24
	if ageDays <= 0 {
		return 1.0
	}

	// 应用指数衰减: exp(-ln(2)/halfLife * age)
	decayLambda := math.Ln2 / halfLifeDays
	return math.Exp(-decayLambda * ageDays)
}

// parseInt 将字符串解析为整数，出错时返回 0。
func parseInt(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}
