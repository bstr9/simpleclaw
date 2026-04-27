// Package memory 提供代理记忆管理功能。
// short_term.go 实现内存中的短期对话历史存储。
package memory

import (
	"context"
	"strconv"
	"sync"
	"time"
)

// ShortTermMemory 实现近期对话历史的内存存储。
// 它提供线程安全的操作，并支持自动清理过期消息。
type ShortTermMemory struct {
	mu sync.RWMutex

	// messages 存储对话历史
	messages []*Message

	// sessionID 标识此记忆所属的会话
	sessionID string

	// userID 可选的用户标识符
	userID string

	// maxMessages 最大保留的消息数量
	maxMessages int

	// maxAge 消息过期前的最大存活时间
	maxAge time.Duration

	// createdAt 此记忆实例的创建时间
	createdAt time.Time

	// lastAccessedAt 此记忆的最后访问时间
	lastAccessedAt time.Time

	// onEvict 消息被驱逐时的可选回调函数
	onEvict func([]*Message)
}

// ShortTermOption 是 ShortTermMemory 的函数式选项。
type ShortTermOption func(*ShortTermMemory)

// WithSessionID 设置会话 ID。
func WithSessionID(sessionID string) ShortTermOption {
	return func(m *ShortTermMemory) {
		m.sessionID = sessionID
	}
}

// WithSTMUserID 设置用户 ID。
func WithSTMUserID(userID string) ShortTermOption {
	return func(m *ShortTermMemory) {
		m.userID = userID
	}
}

// WithSTMMaxMessages 设置最大消息数量。
func WithSTMMaxMessages(max int) ShortTermOption {
	return func(m *ShortTermMemory) {
		m.maxMessages = max
	}
}

// WithMaxAge 设置消息的最大存活时间。
func WithMaxAge(age time.Duration) ShortTermOption {
	return func(m *ShortTermMemory) {
		m.maxAge = age
	}
}

// WithOnEvict 设置驱逐回调函数。
func WithOnEvict(callback func([]*Message)) ShortTermOption {
	return func(m *ShortTermMemory) {
		m.onEvict = callback
	}
}

// NewShortTermMemory 创建新的 ShortTermMemory 实例。
func NewShortTermMemory(opts ...ShortTermOption) *ShortTermMemory {
	m := &ShortTermMemory{
		messages:       make([]*Message, 0),
		maxMessages:    100,
		maxAge:         24 * time.Hour,
		createdAt:      time.Now(),
		lastAccessedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Add 将新消息存入记忆。
func (m *ShortTermMemory) Add(ctx context.Context, msg *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastAccessedAt = time.Now()

	// 确保消息有 ID
	if msg.ID == "" {
		msg.ID = generateID(msg.Content)
	}

	// 确保消息有时间戳
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	// 添加消息到历史
	m.messages = append(m.messages, msg)

	// 如果超过最大消息数则裁剪
	if len(m.messages) > m.maxMessages {
		evicted := m.messages[:len(m.messages)-m.maxMessages]
		m.messages = m.messages[len(m.messages)-m.maxMessages:]
		if m.onEvict != nil {
			go m.onEvict(evicted)
		}
	}

	return nil
}

// AddBatch 批量存储多条消息。
func (m *ShortTermMemory) AddBatch(ctx context.Context, messages []*Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastAccessedAt = time.Now()

	for _, msg := range messages {
		if msg.ID == "" {
			msg.ID = generateID(msg.Content)
		}
		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = time.Now()
		}
	}

	m.messages = append(m.messages, messages...)

	// 如果超过最大消息数则裁剪
	if len(m.messages) > m.maxMessages {
		evicted := m.messages[:len(m.messages)-m.maxMessages]
		m.messages = m.messages[len(m.messages)-m.maxMessages:]
		if m.onEvict != nil {
			go m.onEvict(evicted)
		}
	}

	return nil
}

// Get 从记忆中获取消息。
// 对于短期记忆，返回最近的最多 limit 条消息。
func (m *ShortTermMemory) Get(ctx context.Context, query string, limit int) ([]*Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.lastAccessedAt = time.Now()

	if limit <= 0 {
		limit = m.maxMessages
	}

	// 返回最近的消息（简单的 LRU 行为）
	start := 0
	if len(m.messages) > limit {
		start = len(m.messages) - limit
	}

	result := make([]*Message, len(m.messages)-start)
	copy(result, m.messages[start:])

	return result, nil
}

// GetRecent 返回最近的 n 条消息。
func (m *ShortTermMemory) GetRecent(n int) []*Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.lastAccessedAt = time.Now()

	if n <= 0 || n >= len(m.messages) {
		result := make([]*Message, len(m.messages))
		copy(result, m.messages)
		return result
	}

	start := len(m.messages) - n
	result := make([]*Message, n)
	copy(result, m.messages[start:])
	return result
}

// GetAll 返回记忆中的所有消息。
func (m *ShortTermMemory) GetAll() []*Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Message, len(m.messages))
	copy(result, m.messages)
	return result
}

// GetByRole 返回按角色过滤的消息。
func (m *ShortTermMemory) GetByRole(role Role) []*Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Message
	for _, msg := range m.messages {
		if msg.Role == role {
			result = append(result, msg)
		}
	}
	return result
}

// GetSince 返回指定时间之后创建的消息。
func (m *ShortTermMemory) GetSince(t time.Time) []*Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Message
	for _, msg := range m.messages {
		if msg.CreatedAt.After(t) {
			result = append(result, msg)
		}
	}
	return result
}

// Count 返回记忆中的消息数量。
func (m *ShortTermMemory) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages)
}

// Clear 清除记忆中的所有消息。
func (m *ShortTermMemory) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastAccessedAt = time.Now()

	if m.onEvict != nil && len(m.messages) > 0 {
		evicted := make([]*Message, len(m.messages))
		copy(evicted, m.messages)
		go m.onEvict(evicted)
	}

	m.messages = make([]*Message, 0)
	return nil
}

// Summarize 生成已存储记忆的摘要。
// 对于短期记忆，返回最近消息的格式化字符串。
func (m *ShortTermMemory) Summarize(ctx context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.lastAccessedAt = time.Now()

	if len(m.messages) == 0 {
		return "No messages in memory.", nil
	}

	// 简单摘要：按角色统计
	userCount := 0
	assistantCount := 0
	for _, msg := range m.messages {
		switch msg.Role {
		case RoleUser:
			userCount++
		case RoleAssistant:
			assistantCount++
		}
	}

	summary := "Short-term memory summary:\n"
	summary += "------------------------\n"
	summary += "Session ID: " + m.sessionID + "\n"
	summary += "User ID: " + m.userID + "\n"
	summary += "Total messages: " + strconv.Itoa(len(m.messages)) + "\n"
	summary += "User messages: " + strconv.Itoa(userCount) + "\n"
	summary += "Assistant messages: " + strconv.Itoa(assistantCount) + "\n"
	summary += "Created at: " + m.createdAt.Format(time.RFC3339) + "\n"
	summary += "Last accessed: " + m.lastAccessedAt.Format(time.RFC3339) + "\n"

	return summary, nil
}

// Trim 移除旧消息，仅保留最近的 n 条消息。
func (m *ShortTermMemory) Trim(n int) []*Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastAccessedAt = time.Now()

	if n <= 0 {
		n = 0
	}

	if len(m.messages) <= n {
		return nil
	}

	evicted := m.messages[:len(m.messages)-n]
	m.messages = m.messages[len(m.messages)-n:]

	if m.onEvict != nil {
		go m.onEvict(evicted)
	}

	return evicted
}

// TrimByAge 移除超过指定时间的消息。
func (m *ShortTermMemory) TrimByAge(maxAge time.Duration) []*Message {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastAccessedAt = time.Now()

	cutoff := time.Now().Add(-maxAge)
	var evicted []*Message
	var remaining []*Message

	for _, msg := range m.messages {
		if msg.CreatedAt.Before(cutoff) {
			evicted = append(evicted, msg)
		} else {
			remaining = append(remaining, msg)
		}
	}

	m.messages = remaining

	if m.onEvict != nil && len(evicted) > 0 {
		go m.onEvict(evicted)
	}

	return evicted
}

// CleanupExpired 根据 maxAge 移除过期消息。
func (m *ShortTermMemory) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastAccessedAt = time.Now()

	if m.maxAge <= 0 {
		return 0
	}

	cutoff := time.Now().Add(-m.maxAge)
	var evicted []*Message
	var remaining []*Message

	for _, msg := range m.messages {
		if msg.CreatedAt.Before(cutoff) {
			evicted = append(evicted, msg)
		} else {
			remaining = append(remaining, msg)
		}
	}

	m.messages = remaining

	if m.onEvict != nil && len(evicted) > 0 {
		go m.onEvict(evicted)
	}

	return len(evicted)
}

// GetSessionID 返回会话 ID。
func (m *ShortTermMemory) GetSessionID() string {
	return m.sessionID
}

// GetUserID 返回用户 ID。
func (m *ShortTermMemory) GetUserID() string {
	return m.userID
}

// GetCreatedAt 返回创建时间。
func (m *ShortTermMemory) GetCreatedAt() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.createdAt
}

// GetLastAccessedAt 返回最后访问时间。
func (m *ShortTermMemory) GetLastAccessedAt() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastAccessedAt
}

// Close 释放资源。对于内存存储，这是一个空操作。
func (m *ShortTermMemory) Close() error {
	return nil
}

// Stats 返回记忆的统计信息。
func (m *ShortTermMemory) Stats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userCount := 0
	assistantCount := 0
	for _, msg := range m.messages {
		switch msg.Role {
		case RoleUser:
			userCount++
		case RoleAssistant:
			assistantCount++
		}
	}

	return map[string]any{
		"session_id":         m.sessionID,
		"user_id":            m.userID,
		"total_messages":     len(m.messages),
		"user_messages":      userCount,
		"assistant_messages": assistantCount,
		"max_messages":       m.maxMessages,
		"max_age":            m.maxAge.String(),
		"created_at":         m.createdAt.Format(time.RFC3339),
		"last_accessed":      m.lastAccessedAt.Format(time.RFC3339),
	}
}
