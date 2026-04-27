// Package memory 提供代理记忆管理功能。
// storage.go 实现存储接口，支持多种后端存储。
package memory

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Storage 定义了存储后端的接口。
type Storage interface {
	// SaveChunk 保存一个记忆分块。
	SaveChunk(ctx context.Context, chunk *MemoryChunk) error
	// SaveChunksBatch 批量保存记忆分块。
	SaveChunksBatch(ctx context.Context, chunks []*MemoryChunk) error
	// GetChunk 根据 ID 获取分块。
	GetChunk(ctx context.Context, chunkID string) (*MemoryChunk, error)
	// DeleteByPath 删除指定路径的所有分块。
	DeleteByPath(ctx context.Context, path string) error
	// SearchVector 执行向量相似度搜索。
	SearchVector(ctx context.Context, embedding []float64, opts *SearchOptions) ([]*SearchResult, error)
	// SearchKeyword 执行关键词搜索。
	SearchKeyword(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error)
	// GetFileHash 获取文件存储的哈希值。
	GetFileHash(ctx context.Context, path string) (string, error)
	// UpdateFileMetadata 更新文件元数据。
	UpdateFileMetadata(ctx context.Context, path string, source MemorySource, hash string, mtime, size int) error
	// GetStats 获取存储统计信息。
	GetStats(ctx context.Context) (map[string]any, error)
	// Close 关闭存储连接。
	Close() error
}

// SQLiteStorage 实现 SQLite 后端的存储。
type SQLiteStorage struct {
	mu   sync.RWMutex
	db   *sql.DB
	path string
}

// SQLiteStorageOption 是 SQLiteStorage 的函数式选项。
type SQLiteStorageOption func(*SQLiteStorage)

// WithStoragePath 设置存储路径。
func WithStoragePath(path string) SQLiteStorageOption {
	return func(s *SQLiteStorage) {
		s.path = path
	}
}

// NewSQLiteStorage 创建一个新的 SQLite 存储实例。
func NewSQLiteStorage(opts ...SQLiteStorageOption) (*SQLiteStorage, error) {
	s := &SQLiteStorage{
		path: "./memory.db",
	}

	for _, opt := range opts {
		opt(s)
	}

	if err := s.initDB(); err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	return s, nil
}

// initDB 初始化数据库表结构。
func (s *SQLiteStorage) initDB() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", s.path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s.db = db

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
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("创建表结构失败: %w", err)
	}

	return nil
}

// SaveChunk 保存一个记忆分块。
func (s *SQLiteStorage) SaveChunk(ctx context.Context, chunk *MemoryChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var embeddingJSON string
	if chunk.Embedding != nil {
		embeddingJSON = ToJSON(chunk.Embedding)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO chunks 
		(id, user_id, scope, source, path, start_line, end_line, text, embedding, hash, metadata, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now'))
	`, chunk.ID, chunk.UserID, chunk.Scope, chunk.Source, chunk.Path,
		chunk.StartLine, chunk.EndLine, chunk.Text, embeddingJSON, chunk.Hash, ToJSON(chunk.Metadata))

	return err
}

// SaveChunksBatch 批量保存记忆分块。
func (s *SQLiteStorage) SaveChunksBatch(ctx context.Context, chunks []*MemoryChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO chunks 
		(id, user_id, scope, source, path, start_line, end_line, text, embedding, hash, metadata, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now'))
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		var embeddingJSON string
		if chunk.Embedding != nil {
			embeddingJSON = ToJSON(chunk.Embedding)
		}

		_, err := stmt.ExecContext(ctx, chunk.ID, chunk.UserID, chunk.Scope, chunk.Source, chunk.Path,
			chunk.StartLine, chunk.EndLine, chunk.Text, embeddingJSON, chunk.Hash, ToJSON(chunk.Metadata))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetChunk 根据 ID 获取分块。
func (s *SQLiteStorage) GetChunk(ctx context.Context, chunkID string) (*MemoryChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, scope, source, path, start_line, end_line, text, embedding, hash, metadata
		FROM chunks WHERE id = ?
	`, chunkID)

	chunk := &MemoryChunk{}
	var userID, embeddingJSON, metadataJSON sql.NullString

	err := row.Scan(&chunk.ID, &userID, &chunk.Scope, &chunk.Source, &chunk.Path,
		&chunk.StartLine, &chunk.EndLine, &chunk.Text, &embeddingJSON, &chunk.Hash, &metadataJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	chunk.UserID = userID.String
	if embeddingJSON.Valid {
		FromJSON(embeddingJSON.String, &chunk.Embedding)
	}
	if metadataJSON.Valid {
		FromJSON(metadataJSON.String, &chunk.Metadata)
	}

	return chunk, nil
}

// DeleteByPath 删除指定路径的所有分块。
func (s *SQLiteStorage) DeleteByPath(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, "DELETE FROM chunks WHERE path = ?", path)
	return err
}

// vectorSearchRow 表示向量搜索的中间结果行。
type vectorSearchRow struct {
	id        string
	path      string
	startLine int
	endLine   int
	text      string
	source    MemorySource
	userID    string
	embedding string
}

// scoredVectorResult 表示带分数的向量搜索结果。
type scoredVectorResult struct {
	score float64
	row   vectorSearchRow
}

// buildVectorSearchQuery 构建向量搜索的 SQL 查询和参数。
func buildVectorSearchQuery(opts *SearchOptions) (string, []any) {
	query := "SELECT id, path, start_line, end_line, text, source, user_id, embedding FROM chunks WHERE embedding IS NOT NULL"
	args := []any{}

	if len(opts.Scopes) > 0 {
		placeholders := ""
		for i, scope := range opts.Scopes {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, string(scope))
		}
		query += " AND scope IN (" + placeholders + ")"
	}

	if opts.UserID != "" {
		query += " AND (scope = 'shared' OR user_id = ?)"
		args = append(args, opts.UserID)
	}

	query += " LIMIT ?"
	args = append(args, opts.MaxResults*2)

	return query, args
}

// scanVectorRow 扫描单行向量搜索结果。
func scanVectorRow(rows *sql.Rows) (*scoredVectorResult, bool) {
	var r scoredVectorResult
	var source string
	if err := rows.Scan(&r.row.id, &r.row.path, &r.row.startLine, &r.row.endLine,
		&r.row.text, &source, &r.row.userID, &r.row.embedding); err != nil {
		return nil, false
	}
	r.row.source = MemorySource(source)
	return &r, true
}

// calculateSimilarity 计算向量相似度并返回带分数的结果。
func calculateSimilarity(r *scoredVectorResult, embedding []float64) (*scoredVectorResult, bool) {
	var storedEmbedding []float64
	if err := FromJSON(r.row.embedding, &storedEmbedding); err != nil {
		return nil, false
	}

	similarity := cosineSimilarity(embedding, storedEmbedding)
	if similarity <= 0 {
		return nil, false
	}

	r.score = similarity
	return r, true
}

// sortVectorResults 按分数降序排序向量搜索结果。
func sortVectorResults(results []scoredVectorResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// convertToSearchResults 将排序后的结果转换为 SearchResult 切片。
func convertToSearchResults(results []scoredVectorResult, maxResults int) []*SearchResult {
	searchResults := make([]*SearchResult, 0, maxResults)
	for i, r := range results {
		if i >= maxResults {
			break
		}
		searchResults = append(searchResults, &SearchResult{
			Path:      r.row.path,
			StartLine: r.row.startLine,
			EndLine:   r.row.endLine,
			Score:     r.score,
			Snippet:   truncateText(r.row.text, 500),
			Source:    r.row.source,
			UserID:    r.row.userID,
		})
	}
	return searchResults
}

// SearchVector 执行向量相似度搜索。
func (s *SQLiteStorage) SearchVector(ctx context.Context, embedding []float64, opts *SearchOptions) ([]*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if opts == nil {
		opts = DefaultSearchOptions()
	}

	query, args := buildVectorSearchQuery(opts)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []scoredVectorResult
	for rows.Next() {
		r, ok := scanVectorRow(rows)
		if !ok {
			continue
		}

		r, ok = calculateSimilarity(r, embedding)
		if !ok {
			continue
		}

		results = append(results, *r)
	}

	sortVectorResults(results)

	return convertToSearchResults(results, opts.MaxResults), nil
}

// keywordSearchRow 表示关键词搜索的中间结果行。
type keywordSearchRow struct {
	id        string
	path      string
	startLine int
	endLine   int
	text      string
	source    MemorySource
	userID    string
}

// buildKeywordSearchQuery 构建关键词搜索的 SQL 查询和参数。
func buildKeywordSearchQuery(keywords []string, opts *SearchOptions) (string, []any) {
	likeConditions := ""
	args := []any{}
	for i, kw := range keywords {
		if i > 0 {
			likeConditions += " OR "
		}
		likeConditions += "text LIKE ?"
		args = append(args, "%"+kw+"%")
	}

	sqlQuery := "SELECT id, path, start_line, end_line, text, source, user_id FROM chunks WHERE (" + likeConditions + ")"

	if len(opts.Scopes) > 0 {
		placeholders := ""
		for i, scope := range opts.Scopes {
			if i > 0 {
				placeholders += ","
			}
			placeholders += "?"
			args = append(args, string(scope))
		}
		sqlQuery += " AND scope IN (" + placeholders + ")"
	}

	if opts.UserID != "" {
		sqlQuery += " AND (scope = 'shared' OR user_id = ?)"
		args = append(args, opts.UserID)
	}

	sqlQuery += " LIMIT ?"
	args = append(args, opts.MaxResults*2)

	return sqlQuery, args
}

// scanKeywordRow 扫描单行关键词搜索结果。
func scanKeywordRow(rows *sql.Rows) (*keywordSearchRow, bool) {
	var r keywordSearchRow
	var source string
	var userID sql.NullString

	if err := rows.Scan(&r.id, &r.path, &r.startLine, &r.endLine, &r.text, &source, &userID); err != nil {
		return nil, false
	}
	r.source = MemorySource(source)
	r.userID = userID.String
	return &r, true
}

// calculateKeywordScore 计算关键词匹配分数。
func calculateKeywordScore(text string, keywords []string) float64 {
	score := 0.5
	for _, kw := range keywords {
		if containsString(text, kw) {
			score += 0.1
		}
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// rowToSearchResult 将关键词搜索行转换为 SearchResult。
func rowToSearchResult(r *keywordSearchRow, score float64) *SearchResult {
	return &SearchResult{
		Path:      r.path,
		StartLine: r.startLine,
		EndLine:   r.endLine,
		Score:     score,
		Snippet:   truncateText(r.text, 500),
		Source:    r.source,
		UserID:    r.userID,
	}
}

// SearchKeyword 执行关键词搜索。
func (s *SQLiteStorage) SearchKeyword(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if opts == nil {
		opts = DefaultSearchOptions()
	}

	keywords := extractKeywords(query)
	if len(keywords) == 0 {
		return nil, nil
	}

	sqlQuery, args := buildKeywordSearchQuery(keywords, opts)

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	searchResults := make([]*SearchResult, 0)
	for rows.Next() {
		r, ok := scanKeywordRow(rows)
		if !ok {
			continue
		}

		score := calculateKeywordScore(r.text, keywords)
		searchResults = append(searchResults, rowToSearchResult(r, score))
	}

	return searchResults, nil
}

// GetFileHash 获取文件存储的哈希值。
func (s *SQLiteStorage) GetFileHash(ctx context.Context, path string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var hash string
	err := s.db.QueryRowContext(ctx, "SELECT hash FROM files WHERE path = ?", path).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return hash, err
}

// UpdateFileMetadata 更新文件元数据。
func (s *SQLiteStorage) UpdateFileMetadata(ctx context.Context, path string, source MemorySource, hash string, mtime, size int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO files (path, source, hash, mtime, size, updated_at)
		VALUES (?, ?, ?, ?, ?, strftime('%s', 'now'))
	`, path, source, hash, mtime, size)
	return err
}

// GetStats 获取存储统计信息。
func (s *SQLiteStorage) GetStats(ctx context.Context) (map[string]any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]any)

	var chunksCount int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM chunks").Scan(&chunksCount)
	if err != nil {
		return nil, err
	}
	stats["chunks"] = chunksCount

	var filesCount int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM files").Scan(&filesCount)
	if err != nil {
		return nil, err
	}
	stats["files"] = filesCount

	stats["path"] = s.path

	return stats, nil
}

// Close 关闭存储连接。
func (s *SQLiteStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// MemoryStorageConfig 包含内存存储的配置。
type MemoryStorageConfig struct {
	DBPath        string
	WorkspaceRoot string
}

// DefaultMemoryStorageConfig 返回默认配置。
func DefaultMemoryStorageConfig() *MemoryStorageConfig {
	return &MemoryStorageConfig{
		DBPath:        "./memory/index.db",
		WorkspaceRoot: "./workspace",
	}
}

func containsString(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// FileMetadata 表示文件的元数据。
type FileMetadata struct {
	Path      string
	Source    MemorySource
	Hash      string
	Mtime     int64
	Size      int64
	UpdatedAt time.Time
}

// StorageStats 表示存储的统计信息。
type StorageStats struct {
	TotalChunks     int64
	TotalFiles      int64
	TotalEmbeddings int64
	LastUpdated     time.Time
}
