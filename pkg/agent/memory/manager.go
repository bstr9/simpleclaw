// Package memory 提供统一的内存管理器
// manager.go 整合短期记忆和长期记忆
package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// Manager 统一的内存管理器
// 整合短期会话记忆和长期持久化记忆
type Manager struct {
	mu sync.RWMutex

	// config 内存配置
	config *Config

	// longTerm 长期记忆存储
	longTerm *LongTermMemory

	// shortTerm 短期记忆存储
	shortTerm *ShortTermMemory

	// conversation 会话记忆存储
	conversation *ConversationStore

	// flushManager 记忆刷新管理器
	flushManager *MemoryFlushManager

	// embedder 向量嵌入器
	embedder Embedder

	// workspace 工作目录
	workspace string
}

// ManagerOption 内存管理器选项
type ManagerOption func(*Manager)

// WithManagerConfig 设置配置
func WithManagerConfig(cfg *Config) ManagerOption {
	return func(m *Manager) {
		m.config = cfg
	}
}

// WithManagerEmbedder 设置嵌入器
func WithManagerEmbedder(embedder Embedder) ManagerOption {
	return func(m *Manager) {
		m.embedder = embedder
	}
}

// WithManagerWorkspace 设置工作目录
func WithManagerWorkspace(workspace string) ManagerOption {
	return func(m *Manager) {
		m.workspace = workspace
	}
}

// NewManager 创建新的内存管理器
func NewManager(opts ...ManagerOption) (*Manager, error) {
	// 获取全局配置
	cfg := config.Get()

	// 设置工作目录
	workspace := cfg.AgentWorkspace
	if workspace == "" {
		workspace = "~/cow"
	}
	// 使用 common.ExpandPath 展开路径
	workspace = common.ExpandPath(workspace)

	m := &Manager{
		config:    DefaultConfig(),
		workspace: workspace,
	}

	// 应用选项
	for _, opt := range opts {
		opt(m)
	}

	// 更新配置的工作目录
	m.config.WorkspaceRoot = m.workspace

	// 初始化长期记忆
	var err error
	longTermOpts := []LongTermOption{
		WithDBPath(filepath.Join(m.workspace, "memory", "long-term", "index.db")),
	}
	if m.embedder != nil {
		longTermOpts = append(longTermOpts, WithEmbedder(m.embedder))
	}

	m.longTerm, err = NewLongTermMemory(longTermOpts...)
	if err != nil {
		logger.Warn("初始化长期记忆失败，将禁用长期记忆功能", zap.Error(err))
		// 不返回错误，长期记忆是可选功能
	}

	// 初始化会话记忆
	m.conversation, err = NewConversationStore(
		WithConversationDBPath(filepath.Join(m.workspace, "memory", "conversations.db")),
	)
	if err != nil {
		logger.Warn("初始化会话记忆失败", zap.Error(err))
	}

	// 初始化短期记忆
	m.shortTerm = NewShortTermMemory(
		WithSTMMaxMessages(100),
		WithMaxAge(24*60*60*1e9), // 24小时
	)

	logger.Info("内存管理器初始化完成",
		zap.String("workspace", m.workspace),
		zap.Bool("long_term", m.longTerm != nil),
		zap.Bool("conversation", m.conversation != nil),
		zap.Bool("short_term", m.shortTerm != nil))

	return m, nil
}

// AddMessage 添加消息到记忆
func (m *Manager) AddMessage(ctx context.Context, sessionID string, msg *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 添加到会话记忆
	if m.conversation != nil && sessionID != "" {
		messages := []map[string]interface{}{
			{
				"role":    string(msg.Role),
				"content": msg.Content,
			},
		}
		if err := m.conversation.AppendMessages(ctx, sessionID, "", messages); err != nil {
			logger.Debug("添加会话记忆失败", zap.Error(err))
		}
	}

	// 添加到长期记忆（仅助手消息）
	if m.longTerm != nil && msg.Role == RoleAssistant {
		if err := m.longTerm.Add(ctx, msg); err != nil {
			logger.Debug("添加长期记忆失败", zap.Error(err))
		}
	}

	return nil
}

// Search 搜索记忆
func (m *Manager) Search(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error) {
	if m.longTerm == nil {
		return nil, fmt.Errorf("长期记忆未初始化")
	}

	if opts == nil {
		opts = DefaultSearchOptions()
	}

	return m.longTerm.Search(ctx, query, opts)
}

// GetSessionMessages 获取会话消息历史
func (m *Manager) GetSessionMessages(ctx context.Context, sessionID string, limit int) ([]map[string]interface{}, error) {
	if m.conversation == nil {
		return nil, fmt.Errorf("会话记忆未初始化")
	}

	return m.conversation.LoadMessages(ctx, sessionID, limit)
}

// ClearSession 清除会话记忆
func (m *Manager) ClearSession(ctx context.Context, sessionID string) error {
	if m.conversation == nil {
		return nil
	}

	return m.conversation.ClearSession(ctx, sessionID)
}

// GetLongTerm 获取长期记忆实例
func (m *Manager) GetLongTerm() *LongTermMemory {
	return m.longTerm
}

// GetConversation 获取会话记忆实例
func (m *Manager) GetConversation() *ConversationStore {
	return m.conversation
}

// GetFlushManager 获取记忆刷新管理器
func (m *Manager) GetFlushManager() *MemoryFlushManager {
	return m.flushManager
}

// FlushMessages 刷新消息到持久化存储
func (m *Manager) FlushMessages(ctx context.Context, messages []*Message, opts ...FlushOption) error {
	if m.flushManager == nil {
		return fmt.Errorf("记忆刷新管理器未初始化")
	}
	return m.flushManager.FlushFromMessages(ctx, messages, opts...)
}

// CreateDailySummary 创建每日摘要
func (m *Manager) CreateDailySummary(ctx context.Context, messages []*Message, userID string) error {
	if m.flushManager == nil {
		return fmt.Errorf("记忆刷新管理器未初始化")
	}
	return m.flushManager.CreateDailySummary(ctx, messages, userID)
}

// Close 关闭内存管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	if m.longTerm != nil {
		if err := m.longTerm.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if m.conversation != nil {
		if err := m.conversation.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("关闭内存管理器时发生错误: %v", errs)
	}

	return nil
}

// GetStats 获取内存统计信息
func (m *Manager) GetStats(ctx context.Context) map[string]interface{} {
	stats := map[string]interface{}{
		"workspace": m.workspace,
	}

	if m.longTerm != nil {
		if longTermStats, err := m.longTerm.GetStats(); err == nil {
			stats["long_term"] = longTermStats
		}
	}

	if m.conversation != nil {
		if convStats, err := m.conversation.GetStats(ctx); err == nil {
			stats["conversation"] = convStats
		}
	}

	return stats
}

// Sync 同步长期记忆
func (m *Manager) Sync(ctx context.Context, force bool) error {
	if m.longTerm == nil {
		return nil
	}

	return m.longTerm.Sync(ctx, force)
}

// AddMemory 添加记忆到长期存储
func (m *Manager) AddMemory(ctx context.Context, content string, userID string, scope MemoryScope) error {
	if m.longTerm == nil {
		return fmt.Errorf("长期记忆未初始化")
	}

	return m.longTerm.AddMemory(ctx, content, userID, scope, "")
}

// AddToShortTerm 添加消息到短期记忆
func (m *Manager) AddToShortTerm(ctx context.Context, msg *Message) error {
	if m.shortTerm == nil {
		return nil
	}
	return m.shortTerm.Add(ctx, msg)
}

// GetShortTermMessages 获取短期记忆消息
func (m *Manager) GetShortTermMessages(limit int) []*Message {
	if m.shortTerm == nil {
		return nil
	}
	msgs, err := m.shortTerm.Get(context.Background(), "", limit)
	if err != nil {
		return nil
	}
	return msgs
}

// ClearShortTerm 清空短期记忆
func (m *Manager) ClearShortTerm(ctx context.Context) error {
	if m.shortTerm == nil {
		return nil
	}
	return m.shortTerm.Clear(ctx)
}

// GetShortTerm 获取短期记忆实例
func (m *Manager) GetShortTerm() *ShortTermMemory {
	return m.shortTerm
}
