// Package weixin 提供微信个人号渠道实现
// typing.go 实现输入状态指示器和 typing_ticket 缓存
package weixin

import (
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

const (
	// typingTicketTTL typing_ticket 缓存有效期（24 小时）
	typingTicketTTL = 24 * time.Hour

	// typingKeepaliveInterval typing 保活发送间隔（5 秒）
	typingKeepaliveInterval = 5 * time.Second
)

// typingTicketEntry typing_ticket 缓存条目
type typingTicketEntry struct {
	ticket    string
	expiresAt time.Time
}

// TypingTicketCache 缓存每个用户的 typing_ticket
// typing_ticket 有效期约 24 小时，避免每次消息都调用 getConfig
type TypingTicketCache struct {
	mu    sync.RWMutex
	cache map[string]*typingTicketEntry // userID→ticket
}

// NewTypingTicketCache 创建 typing_ticket 缓存
func NewTypingTicketCache() *TypingTicketCache {
	return &TypingTicketCache{
		cache: make(map[string]*typingTicketEntry),
	}
}

// Get 获取用户的 typing_ticket，如果缓存未过期则返回缓存值
func (c *TypingTicketCache) Get(userID string) string {
	c.mu.RLock()
	entry, ok := c.cache[userID]
	c.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		return ""
	}
	return entry.ticket
}

// Set 缓存用户的 typing_ticket
func (c *TypingTicketCache) Set(userID, ticket string) {
	if ticket == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[userID] = &typingTicketEntry{
		ticket:    ticket,
		expiresAt: time.Now().Add(typingTicketTTL),
	}
}

// TypingController 管理 typing 指示器的生命周期
type TypingController struct {
	api    *weixinAPI
	cache  *TypingTicketCache
	stopCh chan struct{} // 停止 keepalive 协程
}

// NewTypingController 创建 typing 控制器
func NewTypingController(api *weixinAPI, cache *TypingTicketCache) *TypingController {
	return &TypingController{
		api:    api,
		cache:  cache,
		stopCh: make(chan struct{}),
	}
}

// StartTyping 开始发送 typing 指示器
// 立即发送一次 typing，并启动 keepalive 协程定期重发
// 返回 stop 函数，调用者应在回复完成后调用以取消 typing
func (tc *TypingController) StartTyping(fromUserID, contextToken string) (stop func()) {
	// 获取 typing_ticket（优先从缓存）
	ticket := tc.cache.Get(fromUserID)
	if ticket == "" {
		// 缓存未命中，从 API 获取
		var err error
		ticket, err = tc.api.getConfig(fromUserID, contextToken)
		if err != nil || ticket == "" {
			// 获取失败，静默跳过 typing
			logger.Debug(logPrefix+" 获取 typing_ticket 失败，跳过 typing",
				zap.String("from_user_id", fromUserID),
				zap.Error(err))
			return func() {} // 空操作
		}
		tc.cache.Set(fromUserID, ticket)
	}

	// 发送 typing 开始
	if err := tc.api.sendTyping(fromUserID, ticket, typingStatusTyping); err != nil {
		logger.Debug(logPrefix+" 发送 typing 失败",
			zap.String("from_user_id", fromUserID),
			zap.Error(err))
		return func() {}
	}

	// 启动 keepalive 协程
	stopped := make(chan struct{})
	stopCh := make(chan struct{})

	go func() {
		defer close(stopped)
		ticker := time.NewTicker(typingKeepaliveInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := tc.api.sendTyping(fromUserID, ticket, typingStatusTyping); err != nil {
					logger.Debug(logPrefix+" typing keepalive 失败",
						zap.String("from_user_id", fromUserID),
						zap.Error(err))
				}
			case <-stopCh:
				return
			}
		}
	}()

	// 返回 stop 函数
	return func() {
		close(stopCh)
		<-stopped // 等待协程退出

		// 发送 typing 取消
		if err := tc.api.sendTyping(fromUserID, ticket, typingStatusCancel); err != nil {
			logger.Debug(logPrefix+" 取消 typing 失败",
				zap.String("from_user_id", fromUserID),
				zap.Error(err))
		}
	}
}
