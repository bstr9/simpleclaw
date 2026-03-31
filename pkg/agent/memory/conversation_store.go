// Package memory 提供 Agent 记忆管理功能。
// conversation_store.go 实现对话历史持久化存储。
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultMaxAgeDays 默认消息保留天数。
const DefaultMaxAgeDays = 30

// ConversationSession 表示一个对话会话。
type ConversationSession struct {
	SessionID   string    `json:"session_id"`
	ChannelType string    `json:"channel_type"`
	CreatedAt   time.Time `json:"created_at"`
	LastActive  time.Time `json:"last_active"`
	MsgCount    int       `json:"msg_count"`
}

// ConversationMessage 表示存储的对话消息。
type ConversationMessage struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	Seq       int       `json:"seq"`
	Role      Role      `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// DisplayTurn 表示用于显示的对话轮次。
type DisplayTurn struct {
	Role      string                   `json:"role"`
	Content   string                   `json:"content"`
	ToolCalls []map[string]interface{} `json:"tool_calls,omitempty"`
	CreatedAt int64                    `json:"created_at,omitempty"`
}

// ConversationStore 实现 SQLite 后端的对话存储。
type ConversationStore struct {
	mu     sync.Mutex
	dbPath string
	db     *sql.DB
}

// ConversationStoreOption 是 ConversationStore 的函数式选项。
type ConversationStoreOption func(*ConversationStore)

// WithDBPath 设置数据库路径。
func WithConversationDBPath(path string) ConversationStoreOption {
	return func(s *ConversationStore) {
		s.dbPath = path
	}
}

// NewConversationStore 创建一个新的对话存储实例。
func NewConversationStore(opts ...ConversationStoreOption) (*ConversationStore, error) {
	s := &ConversationStore{
		dbPath: "./conversations.db",
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
func (s *ConversationStore) initDB() error {
	dir := filepath.Dir(s.dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", s.dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s.db = db

	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		session_id   TEXT    PRIMARY KEY,
		channel_type TEXT    NOT NULL DEFAULT '',
		created_at   INTEGER NOT NULL,
		last_active  INTEGER NOT NULL,
		msg_count    INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS messages (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id   TEXT    NOT NULL,
		seq          INTEGER NOT NULL,
		role         TEXT    NOT NULL,
		content      TEXT    NOT NULL,
		created_at   INTEGER NOT NULL,
		UNIQUE (session_id, seq)
	);

	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, seq);
	CREATE INDEX IF NOT EXISTS idx_sessions_last_active ON sessions(last_active);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("创建表结构失败: %w", err)
	}

	return nil
}

// loadMsgRow 表示加载的消息行。
type loadMsgRow struct {
	seq     int
	role    string
	content string
}

// LoadMessages 加载指定会话的最近消息。
func (s *ConversationStore) LoadMessages(ctx context.Context, sessionID string, maxTurns int) ([]map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if maxTurns <= 0 {
		maxTurns = 30
	}

	allRows, err := s.queryMessageRows(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if len(allRows) == 0 {
		return nil, nil
	}

	cutoffSeq := s.calculateCutoffSeq(allRows, maxTurns)
	return s.buildMessagesResult(allRows, cutoffSeq), nil
}

// queryMessageRows 查询消息行。
func (s *ConversationStore) queryMessageRows(ctx context.Context, sessionID string) ([]loadMsgRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT seq, role, content FROM messages
		WHERE session_id = ?
		ORDER BY seq DESC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allRows []loadMsgRow
	for rows.Next() {
		var r loadMsgRow
		if err := rows.Scan(&r.seq, &r.role, &r.content); err != nil {
			continue
		}
		allRows = append(allRows, r)
	}
	return allRows, nil
}

// calculateCutoffSeq 计算截止序列号。
func (s *ConversationStore) calculateCutoffSeq(rows []loadMsgRow, maxTurns int) *int {
	visibleTurnSeqs := s.collectVisibleTurnSeqs(rows)
	if len(visibleTurnSeqs) <= maxTurns {
		return nil
	}
	return &visibleTurnSeqs[maxTurns-1]
}

// collectVisibleTurnSeqs 收集可见轮次的序列号。
func (s *ConversationStore) collectVisibleTurnSeqs(rows []loadMsgRow) []int {
	var seqs []int
	for _, r := range rows {
		if r.role != "user" {
			continue
		}
		content := s.parseContent(r.content)
		if s.isVisibleUserMessage(content) {
			seqs = append(seqs, r.seq)
		}
	}
	return seqs
}

// buildMessagesResult 构建消息结果。
func (s *ConversationStore) buildMessagesResult(rows []loadMsgRow, cutoffSeq *int) []map[string]interface{} {
	var result []map[string]interface{}
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if cutoffSeq != nil && r.seq < *cutoffSeq {
			continue
		}
		content := s.parseContent(r.content)
		result = append(result, map[string]interface{}{
			"role":    r.role,
			"content": content,
		})
	}
	return result
}

// parseContent 解析消息内容。
func (s *ConversationStore) parseContent(content string) interface{} {
	var parsed interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return content
	}
	return parsed
}

// AppendMessages 追加消息到会话。
func (s *ConversationStore) AppendMessages(ctx context.Context, sessionID, channelType string, messages []map[string]interface{}) error {
	if len(messages) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 插入或更新会话
	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO sessions (session_id, channel_type, created_at, last_active, msg_count)
		VALUES (?, ?, ?, ?, 0)
	`, sessionID, channelType, now, now)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "UPDATE sessions SET last_active = ? WHERE session_id = ?", now, sessionID)
	if err != nil {
		return err
	}

	// 获取下一个 seq
	var nextSeq int
	row := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(seq), -1) FROM messages WHERE session_id = ?", sessionID)
	if err := row.Scan(&nextSeq); err != nil {
		return err
	}
	nextSeq++

	// 插入消息
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		contentBytes, _ := json.Marshal(msg["content"])
		content := string(contentBytes)

		_, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO messages (session_id, seq, role, content, created_at)
			VALUES (?, ?, ?, ?, ?)
		`, sessionID, nextSeq, role, content, now)
		if err != nil {
			return err
		}
		nextSeq++
	}

	// 更新消息计数
	_, err = tx.ExecContext(ctx, `
		UPDATE sessions SET msg_count = (SELECT COUNT(*) FROM messages WHERE session_id = ?)
		WHERE session_id = ?
	`, sessionID, sessionID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ClearSession 清除指定会话的所有消息。
func (s *ConversationStore) ClearSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM messages WHERE session_id = ?", sessionID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE session_id = ?", sessionID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CleanupOldSessions 清理过期会话。
func (s *ConversationStore) CleanupOldSessions(ctx context.Context, maxAgeDays int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if maxAgeDays <= 0 {
		maxAgeDays = DefaultMaxAgeDays
	}

	cutoff := time.Now().Add(-time.Duration(maxAgeDays) * 24 * time.Hour).Unix()

	rows, err := s.db.QueryContext(ctx, "SELECT session_id FROM sessions WHERE last_active < ?", cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			continue
		}
		sessionIDs = append(sessionIDs, sid)
	}

	deleted := 0
	for _, sid := range sessionIDs {
		_, err := s.db.ExecContext(ctx, "DELETE FROM messages WHERE session_id = ?", sid)
		if err != nil {
			continue
		}
		_, err = s.db.ExecContext(ctx, "DELETE FROM sessions WHERE session_id = ?", sid)
		if err != nil {
			continue
		}
		deleted++
	}

	return deleted, nil
}

// LoadHistoryPage 加载分页的历史记录用于 UI 显示。
func (s *ConversationStore) LoadHistoryPage(ctx context.Context, sessionID string, page, pageSize int) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	page, pageSize = normalizePagination(page, pageSize)

	rawRows, err := s.queryRawMsgRows(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	visible := s.groupIntoDisplayTurns(rawRows)
	pageItems := paginateDisplayTurns(visible, page, pageSize)

	return buildHistoryPageResult(pageItems, len(visible), page, pageSize), nil
}

// normalizePagination 规范化分页参数。
func normalizePagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	return page, pageSize
}

// queryRawMsgRows 查询原始消息行。
func (s *ConversationStore) queryRawMsgRows(ctx context.Context, sessionID string) ([]rawMsgRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT role, content, created_at FROM messages
		WHERE session_id = ?
		ORDER BY seq ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rawRows []rawMsgRow
	for rows.Next() {
		var r rawMsgRow
		if err := rows.Scan(&r.role, &r.content, &r.createdAt); err != nil {
			continue
		}
		rawRows = append(rawRows, r)
	}
	return rawRows, nil
}

// paginateDisplayTurns 对显示轮次进行分页。
func paginateDisplayTurns(turns []DisplayTurn, page, pageSize int) []DisplayTurn {
	total := len(turns)
	offset := (page - 1) * pageSize

	if offset >= total {
		return nil
	}

	end := offset + pageSize
	if end > total {
		end = total
	}

	reversed := reverseDisplayTurns(turns)
	return reverseDisplayTurns(reversed[offset:end])
}

// reverseDisplayTurns 反转显示轮次。
func reverseDisplayTurns(turns []DisplayTurn) []DisplayTurn {
	result := make([]DisplayTurn, len(turns))
	for i, t := range turns {
		result[len(turns)-1-i] = t
	}
	return result
}

// buildHistoryPageResult 构建历史页面结果。
func buildHistoryPageResult(pageItems []DisplayTurn, total, page, pageSize int) map[string]interface{} {
	return map[string]interface{}{
		"messages":  pageItems,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"has_more":  (page-1)*pageSize+pageSize < total,
	}
}

// GetStats 获取存储统计信息。
func (s *ConversationStore) GetStats(ctx context.Context) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var totalSessions int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&totalSessions)
	if err != nil {
		return nil, err
	}

	var totalMessages int
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages").Scan(&totalMessages)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT channel_type, COUNT(*) as cnt FROM sessions
		GROUP BY channel_type ORDER BY cnt DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byChannel := make(map[string]int)
	for rows.Next() {
		var channelType string
		var cnt int
		if err := rows.Scan(&channelType, &cnt); err != nil {
			continue
		}
		if channelType == "" {
			channelType = "unknown"
		}
		byChannel[channelType] = cnt
	}

	return map[string]interface{}{
		"total_sessions": totalSessions,
		"total_messages": totalMessages,
		"by_channel":     byChannel,
	}, nil
}

// Close 关闭存储连接。
func (s *ConversationStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// isVisibleUserMessage 判断是否为可见的用户消息。
func (s *ConversationStore) isVisibleUserMessage(content interface{}) bool {
	switch c := content.(type) {
	case string:
		return len(c) > 0
	case []interface{}:
		for _, b := range c {
			if bm, ok := b.(map[string]interface{}); ok {
				if t, ok := bm["type"].(string); ok && t == "text" {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

// extractDisplayText 提取可显示文本。
func (s *ConversationStore) extractDisplayText(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		return extractTextFromBlocks(c)
	default:
		return ""
	}
}

func extractTextFromBlocks(blocks []interface{}) string {
	var parts []string
	for _, b := range blocks {
		bm, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		if t, ok := bm["type"].(string); ok && t == "text" {
			if text, ok := bm["text"].(string); ok {
				parts = append(parts, text)
			}
		}
	}
	return joinNonEmpty(parts, "\n")
}

// extractToolCalls 提取工具调用信息。
func (s *ConversationStore) extractToolCalls(content interface{}) []map[string]interface{} {
	result := []map[string]interface{}{}
	c, ok := content.([]interface{})
	if !ok {
		return result
	}
	for _, b := range c {
		if bm, ok := b.(map[string]interface{}); ok {
			if t, ok := bm["type"].(string); ok && t == "tool_use" {
				tc := map[string]interface{}{
					"id":        bm["id"],
					"name":      bm["name"],
					"arguments": bm["input"],
				}
				result = append(result, tc)
			}
		}
	}
	return result
}

// extractToolResults 提取工具结果。
func (s *ConversationStore) extractToolResults(content interface{}) map[string]string {
	result := make(map[string]string)
	blocks, ok := content.([]interface{})
	if !ok {
		return result
	}

	for _, b := range blocks {
		s.extractToolResultFromBlock(b, result)
	}
	return result
}

// extractToolResultFromBlock 从单个块中提取工具结果。
func (s *ConversationStore) extractToolResultFromBlock(b interface{}, result map[string]string) {
	bm, ok := b.(map[string]interface{})
	if !ok {
		return
	}

	t, ok := bm["type"].(string)
	if !ok || t != "tool_result" {
		return
	}

	toolID, _ := bm["tool_use_id"].(string)
	result[toolID] = s.extractToolResultContent(bm["content"])
}

// extractToolResultContent 提取工具结果内容。
func (s *ConversationStore) extractToolResultContent(content interface{}) string {
	switch rct := content.(type) {
	case string:
		return rct
	case []interface{}:
		return s.extractTextFromToolResultBlocks(rct)
	default:
		return ""
	}
}

// extractTextFromToolResultBlocks 从工具结果块中提取文本。
func (s *ConversationStore) extractTextFromToolResultBlocks(blocks []interface{}) string {
	var parts []string
	for _, rb := range blocks {
		text := s.extractTextFromResultBlock(rb)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return joinNonEmpty(parts, "\n")
}

// extractTextFromResultBlock 从单个结果块提取文本。
func (s *ConversationStore) extractTextFromResultBlock(rb interface{}) string {
	rbm, ok := rb.(map[string]interface{})
	if !ok {
		return ""
	}

	rt, ok := rbm["type"].(string)
	if !ok || rt != "text" {
		return ""
	}

	text, _ := rbm["text"].(string)
	return text
}

// rawMsgRow 表示原始消息行。
type rawMsgRow struct {
	role      string
	content   string
	createdAt int64
}

// msgGroup 表示消息分组。
type msgGroup struct {
	user *struct {
		content   interface{}
		createdAt int64
	}
	rest []struct {
		role      string
		content   interface{}
		createdAt int64
	}
}

// groupIntoDisplayTurns 将原始消息分组为显示轮次。
func (s *ConversationStore) groupIntoDisplayTurns(rows []rawMsgRow) []DisplayTurn {
	groups := s.groupMessages(rows)
	return s.buildDisplayTurns(groups)
}

// groupMessages 将原始消息按用户可见消息分组。
func (s *ConversationStore) groupMessages(rows []rawMsgRow) []msgGroup {
	groups := &msgGroupBuilder{}

	for _, r := range rows {
		content := s.parseContent(r.content)
		s.processRowForGrouping(groups, r, content)
	}

	return groups.Build()
}

// processRowForGrouping 处理单行消息的分组逻辑。
func (s *ConversationStore) processRowForGrouping(groups *msgGroupBuilder, r rawMsgRow, content interface{}) {
	if r.role == "user" && s.isVisibleUserMessage(content) {
		groups.StartNewGroup(content, r.createdAt)
	} else {
		groups.AddRest(r.role, content, r.createdAt)
	}
}

// msgGroupBuilder 消息分组构建器。
type msgGroupBuilder struct {
	groups  []msgGroup
	started bool
	curUser *struct {
		content   interface{}
		createdAt int64
	}
	curRest []struct {
		role      string
		content   interface{}
		createdAt int64
	}
}

// StartNewGroup 开始新的分组。
func (b *msgGroupBuilder) StartNewGroup(content interface{}, createdAt int64) {
	if b.started {
		b.groups = append(b.groups, msgGroup{user: b.curUser, rest: b.curRest})
	}
	b.curUser = &struct {
		content   interface{}
		createdAt int64
	}{content: content, createdAt: createdAt}
	b.curRest = nil
	b.started = true
}

// AddRest 添加非用户消息到当前分组。
func (b *msgGroupBuilder) AddRest(role string, content interface{}, createdAt int64) {
	b.curRest = append(b.curRest, struct {
		role      string
		content   interface{}
		createdAt int64
	}{role: role, content: content, createdAt: createdAt})
}

// Build 构建最终的分组结果。
func (b *msgGroupBuilder) Build() []msgGroup {
	if b.started {
		b.groups = append(b.groups, msgGroup{user: b.curUser, rest: b.curRest})
	}
	return b.groups
}

// buildDisplayTurns 从消息分组构建显示轮次。
func (s *ConversationStore) buildDisplayTurns(groups []msgGroup) []DisplayTurn {
	var turns []DisplayTurn

	for _, g := range groups {
		turns = s.appendUserTurn(turns, g)
		turns = s.appendAssistantTurn(turns, g)
	}

	return turns
}

// appendUserTurn 添加用户轮次。
func (s *ConversationStore) appendUserTurn(turns []DisplayTurn, g msgGroup) []DisplayTurn {
	if g.user == nil {
		return turns
	}
	text := s.extractDisplayText(g.user.content)
	if text == "" {
		return turns
	}
	return append(turns, DisplayTurn{
		Role:      "user",
		Content:   text,
		CreatedAt: g.user.createdAt,
	})
}

// appendAssistantTurn 添加助手轮次。
func (s *ConversationStore) appendAssistantTurn(turns []DisplayTurn, g msgGroup) []DisplayTurn {
	var allToolCalls []map[string]interface{}
	toolResults := make(map[string]string)
	var finalText string
	var finalTS int64

	for _, r := range g.rest {
		s.collectRestContent(r, &allToolCalls, toolResults, &finalText, &finalTS)
	}

	// 附加工具结果
	for _, tc := range allToolCalls {
		if id, ok := tc["id"].(string); ok {
			tc["result"] = toolResults[id]
		}
	}

	if finalText == "" && len(allToolCalls) == 0 {
		return turns
	}

	ts := finalTS
	if ts == 0 && g.user != nil {
		ts = g.user.createdAt
	}
	return append(turns, DisplayTurn{
		Role:      "assistant",
		Content:   finalText,
		ToolCalls: allToolCalls,
		CreatedAt: ts,
	})
}

// collectRestContent 收集 rest 消息内容。
func (s *ConversationStore) collectRestContent(r struct {
	role      string
	content   interface{}
	createdAt int64
}, allToolCalls *[]map[string]interface{}, toolResults map[string]string, finalText *string, finalTS *int64) {
	if r.role == "user" {
		for k, v := range s.extractToolResults(r.content) {
			toolResults[k] = v
		}
	} else if r.role == "assistant" {
		tcs := s.extractToolCalls(r.content)
		*allToolCalls = append(*allToolCalls, tcs...)
		t := s.extractDisplayText(r.content)
		if t != "" {
			*finalText = t
		}
		*finalTS = r.createdAt
	}
}

// joinNonEmpty 连接非空字符串。
func joinNonEmpty(parts []string, sep string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	result := ""
	for i, p := range nonEmpty {
		if i > 0 {
			result += sep
		}
		result += p
	}
	return result
}
