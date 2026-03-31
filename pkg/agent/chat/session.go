// Package chat 提供对话管理功能，支持多轮对话和上下文管理。
// 该包实现了 ChatService 用于处理对话流程，SessionManager 用于会话生命周期管理。
package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/llm"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// SessionStatus 会话状态
type SessionStatus string

const (
	// StatusActive 活跃状态
	StatusActive SessionStatus = "active"
	// StatusIdle 空闲状态
	StatusIdle SessionStatus = "idle"
	// StatusClosed 已关闭状态
	StatusClosed SessionStatus = "closed"

	errSessionNotFound = "会话 %s 不存在"
)

// Session 表示一个对话会话
type Session struct {
	// ID 会话唯一标识符
	ID string `json:"id"`
	// UserID 用户标识符
	UserID string `json:"user_id,omitempty"`
	// ChannelType 渠道类型 (web, feishu 等)
	ChannelType string `json:"channel_type,omitempty"`
	// Status 会话状态
	Status SessionStatus `json:"status"`
	// Messages 对话消息历史
	Messages []llm.Message `json:"messages"`
	// Metadata 会话元数据
	Metadata map[string]any `json:"metadata,omitempty"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 最后更新时间
	UpdatedAt time.Time `json:"updated_at"`
	// LastActiveAt 最后活跃时间
	LastActiveAt time.Time `json:"last_active_at"`
	// mu 消息操作互斥锁
	mu sync.RWMutex
}

// NewSession 创建新的会话实例
func NewSession(id string, opts ...SessionOption) *Session {
	now := time.Now()
	s := &Session{
		ID:           id,
		Status:       StatusActive,
		Messages:     make([]llm.Message, 0),
		Metadata:     make(map[string]any),
		CreatedAt:    now,
		UpdatedAt:    now,
		LastActiveAt: now,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// SessionOption 会话配置选项函数
type SessionOption func(*Session)

// WithUserID 设置用户ID
func WithUserID(userID string) SessionOption {
	return func(s *Session) {
		s.UserID = userID
	}
}

// WithChannelType 设置渠道类型
func WithChannelType(channelType string) SessionOption {
	return func(s *Session) {
		s.ChannelType = channelType
	}
}

// WithMetadata 设置元数据
func WithMetadata(metadata map[string]any) SessionOption {
	return func(s *Session) {
		s.Metadata = metadata
	}
}

// AddMessage 添加消息到会话历史
func (s *Session) AddMessage(msg llm.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
	s.LastActiveAt = time.Now()
}

// AddUserMessage 添加用户消息
func (s *Session) AddUserMessage(content string) {
	s.AddMessage(llm.Message{
		Role:    llm.RoleUser,
		Content: content,
	})
}

// AddAssistantMessage 添加助手消息
func (s *Session) AddAssistantMessage(content string) {
	s.AddMessage(llm.Message{
		Role:    llm.RoleAssistant,
		Content: content,
	})
}

// AddToolCallMessage 添加工具调用消息
func (s *Session) AddToolCallMessage(toolCalls []llm.ToolCall) {
	s.AddMessage(llm.Message{
		Role:      llm.RoleAssistant,
		ToolCalls: toolCalls,
	})
}

// AddToolResultMessage 添加工具结果消息
func (s *Session) AddToolResultMessage(toolCallID, content string) {
	s.AddMessage(llm.Message{
		Role:       llm.RoleTool,
		Content:    content,
		ToolCallID: toolCallID,
	})
}

// GetMessages 获取消息历史（线程安全副本）
func (s *Session) GetMessages() []llm.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]llm.Message, len(s.Messages))
	copy(result, s.Messages)
	return result
}

// GetMessagesWithSystem 获取包含系统提示的消息历史
func (s *Session) GetMessagesWithSystem(systemPrompt string) []llm.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]llm.Message, 0, len(s.Messages)+1)

	if systemPrompt != "" {
		result = append(result, llm.Message{
			Role:    llm.RoleSystem,
			Content: systemPrompt,
		})
	}

	result = append(result, s.Messages...)
	return result
}

// ClearMessages 清空消息历史
func (s *Session) ClearMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = make([]llm.Message, 0)
	s.UpdatedAt = time.Now()
}

// TrimMessages 裁剪消息历史，保留最近的 n 条
func (s *Session) TrimMessages(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.Messages) <= n {
		return
	}

	s.Messages = s.Messages[len(s.Messages)-n:]
	s.UpdatedAt = time.Now()
}

// GetMessageCount 获取消息数量
func (s *Session) GetMessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.Messages)
}

// IsActive 检查会话是否活跃
func (s *Session) IsActive() bool {
	return s.Status == StatusActive
}

// Close 关闭会话
func (s *Session) Close() {
	s.Status = StatusClosed
	s.UpdatedAt = time.Now()
}

// SetIdle 设置会话为空闲状态
func (s *Session) SetIdle() {
	s.Status = StatusIdle
	s.UpdatedAt = time.Now()
}

// Activate 激活会话
func (s *Session) Activate() {
	s.Status = StatusActive
	s.LastActiveAt = time.Now()
	s.UpdatedAt = time.Now()
}

// ToJSON 序列化会话为 JSON
func (s *Session) ToJSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SessionManager 会话管理器
type SessionManager struct {
	// sessions 会话存储
	sessions map[string]*Session
	// mu 会话操作互斥锁
	mu sync.RWMutex
	// store 可选的持久化存储
	store SessionStore
	// maxSessions 最大会话数
	maxSessions int
	// sessionTimeout 会话超时时间
	sessionTimeout time.Duration
	// cleanupInterval 清理间隔
	cleanupInterval time.Duration
	// stopCleanup 停止清理信号
	stopCleanup chan struct{}
}

// SessionManagerOption 会话管理器配置选项
type SessionManagerOption func(*SessionManager)

// WithSessionStore 设置持久化存储
func WithSessionStore(store SessionStore) SessionManagerOption {
	return func(sm *SessionManager) {
		sm.store = store
	}
}

// WithMaxSessions 设置最大会话数
func WithMaxSessions(max int) SessionManagerOption {
	return func(sm *SessionManager) {
		sm.maxSessions = max
	}
}

// WithSessionTimeout 设置会话超时时间
func WithSessionTimeout(timeout time.Duration) SessionManagerOption {
	return func(sm *SessionManager) {
		sm.sessionTimeout = timeout
	}
}

// WithCleanupInterval 设置清理间隔
func WithCleanupInterval(interval time.Duration) SessionManagerOption {
	return func(sm *SessionManager) {
		sm.cleanupInterval = interval
	}
}

// NewSessionManager 创建新的会话管理器
func NewSessionManager(opts ...SessionManagerOption) *SessionManager {
	sm := &SessionManager{
		sessions:        make(map[string]*Session),
		maxSessions:     1000,
		sessionTimeout:  30 * time.Minute,
		cleanupInterval: 5 * time.Minute,
		stopCleanup:     make(chan struct{}),
	}

	for _, opt := range opts {
		opt(sm)
	}

	// 启动后台清理任务
	go sm.cleanupLoop()

	return sm
}

// CreateSession 创建新会话
func (sm *SessionManager) CreateSession(id string, opts ...SessionOption) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查是否已存在
	if _, exists := sm.sessions[id]; exists {
		return nil, fmt.Errorf("会话 %s 已存在", id)
	}

	// 检查最大会话数限制
	if len(sm.sessions) >= sm.maxSessions {
		// 尝试清理过期会话
		sm.cleanupExpiredSessionsLocked()
		if len(sm.sessions) >= sm.maxSessions {
			return nil, fmt.Errorf("已达到最大会话数限制")
		}
	}

	session := NewSession(id, opts...)
	sm.sessions[id] = session

	// 持久化会话
	if sm.store != nil {
		if err := sm.store.Save(session); err != nil {
			logger.Warn("持久化会话失败", zap.String("session_id", id), zap.Error(err))
		}
	}

	logger.Info("创建新会话", zap.String("session_id", id))
	return session, nil
}

// GetSession 获取会话
func (sm *SessionManager) GetSession(id string) (*Session, error) {
	sm.mu.RLock()
	session, exists := sm.sessions[id]
	sm.mu.RUnlock()

	if exists {
		return session, nil
	}

	// 尝试从存储加载
	if sm.store != nil {
		session, err := sm.store.Load(id)
		if err == nil && session != nil {
			sm.mu.Lock()
			sm.sessions[id] = session
			sm.mu.Unlock()
			return session, nil
		}
	}

	return nil, fmt.Errorf(errSessionNotFound, id)
}

// GetOrCreateSession 获取或创建会话
func (sm *SessionManager) GetOrCreateSession(id string, opts ...SessionOption) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[id]
	if exists {
		session.Activate()
		return session, nil
	}

	// 尝试从存储加载
	if sm.store != nil {
		loaded, err := sm.store.Load(id)
		if err == nil && loaded != nil {
			loaded.Activate()
			sm.sessions[id] = loaded
			return loaded, nil
		}
	}

	// 创建新会话
	session = NewSession(id, opts...)
	sm.sessions[id] = session

	if sm.store != nil {
		if err := sm.store.Save(session); err != nil {
			logger.Warn("持久化会话失败", zap.String("session_id", id), zap.Error(err))
		}
	}

	return session, nil
}

// DeleteSession 删除会话
func (sm *SessionManager) DeleteSession(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[id]
	if !exists {
		return fmt.Errorf(errSessionNotFound, id)
	}

	session.Close()
	delete(sm.sessions, id)

	// 从存储中删除
	if sm.store != nil {
		if err := sm.store.Delete(id); err != nil {
			logger.Warn("删除持久化会话失败", zap.String("session_id", id), zap.Error(err))
		}
	}

	logger.Info("删除会话", zap.String("session_id", id))
	return nil
}

// ListSessions 列出所有会话
func (sm *SessionManager) ListSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*Session, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		result = append(result, session)
	}
	return result
}

// ListActiveSessions 列出活跃会话
func (sm *SessionManager) ListActiveSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*Session, 0)
	for _, session := range sm.sessions {
		if session.IsActive() {
			result = append(result, session)
		}
	}
	return result
}

// SessionCount 获取会话数量
func (sm *SessionManager) SessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return len(sm.sessions)
}

// SaveSession 保存会话到存储
func (sm *SessionManager) SaveSession(id string) error {
	if sm.store == nil {
		return nil
	}

	sm.mu.RLock()
	session, exists := sm.sessions[id]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf(errSessionNotFound, id)
	}

	return sm.store.Save(session)
}

// cleanupLoop 后台清理循环
func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(sm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.cleanupExpiredSessions()
		case <-sm.stopCleanup:
			return
		}
	}
}

// cleanupExpiredSessions 清理过期会话
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.cleanupExpiredSessionsLocked()
}

// cleanupExpiredSessionsLocked 清理过期会话（需要持有锁）
func (sm *SessionManager) cleanupExpiredSessionsLocked() {
	now := time.Now()
	for id, session := range sm.sessions {
		if now.Sub(session.LastActiveAt) > sm.sessionTimeout {
			session.Close()
			delete(sm.sessions, id)
			logger.Info("清理过期会话", zap.String("session_id", id))
		}
	}
}

// Close 关闭会话管理器
func (sm *SessionManager) Close() error {
	close(sm.stopCleanup)

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 保存所有活跃会话
	if sm.store != nil {
		for id, session := range sm.sessions {
			session.Close()
			if err := sm.store.Save(session); err != nil {
				logger.Warn("保存会话失败", zap.String("session_id", id), zap.Error(err))
			}
		}
	}

	sm.sessions = make(map[string]*Session)
	return nil
}

// SessionStore 会话持久化存储接口
type SessionStore interface {
	// Save 保存会话
	Save(session *Session) error
	// Load 加载会话
	Load(id string) (*Session, error)
	// Delete 删除会话
	Delete(id string) error
	// List 列出所有会话
	List() ([]*Session, error)
}

// GenerateSessionID 生成会话ID
func GenerateSessionID(prefix string) string {
	hash := sha256.Sum256([]byte(time.Now().String() + prefix))
	return prefix + "_" + hex.EncodeToString(hash[:])[:16]
}
