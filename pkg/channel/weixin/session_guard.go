// Package weixin 提供微信个人号渠道实现
// session_guard.go 实现会话暂停/恢复机制
// 当微信 API 返回 errcode -14（会话过期）时，暂停轮询而非立即 re-login
package weixin

import (
	"fmt"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

const (
	// defaultSessionPauseMinutes 默认会话暂停时长（分钟）
	defaultSessionPauseMinutes = 60

	// maxSessionPauseAttempts 最大连续暂停次数，超过后触发 re-login
	maxSessionPauseAttempts = 3
)

// SessionGuard 管理账号级别的会话暂停状态
// 会话过期时暂停所有 API 请求，暂停到期后自动恢复
type SessionGuard struct {
	mu         sync.RWMutex
	pauseUntil map[string]time.Time // 账号ID→暂停到期时间
	pauseCount map[string]int       // 账号ID→连续暂停次数
	pauseDur   time.Duration        // 暂停时长
}

// NewSessionGuard 创建会话暂停守卫
// pauseMinutes 为 0 时使用默认值（60 分钟）
func NewSessionGuard(pauseMinutes int) *SessionGuard {
	if pauseMinutes <= 0 {
		pauseMinutes = defaultSessionPauseMinutes
	}
	return &SessionGuard{
		pauseUntil: make(map[string]time.Time),
		pauseCount: make(map[string]int),
		pauseDur:   time.Duration(pauseMinutes) * time.Minute,
	}
}

// Pause 暂停指定账号的会话
func (g *SessionGuard) Pause(accountID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	until := time.Now().Add(g.pauseDur)
	g.pauseUntil[accountID] = until
	g.pauseCount[accountID]++

	logger.Info(logPrefix+" 会话已暂停",
		zap.String("account_id", accountID),
		zap.Time("until", until),
		zap.Int("pause_count", g.pauseCount[accountID]))
}

// IsPaused 检查指定账号是否处于暂停状态
// 如果暂停已过期，自动清除暂停状态
func (g *SessionGuard) IsPaused(accountID string) bool {
	g.mu.RLock()
	until, exists := g.pauseUntil[accountID]
	g.mu.RUnlock()

	if !exists {
		return false
	}

	if time.Now().After(until) {
		// 暂停已过期，清除状态
		g.mu.Lock()
		delete(g.pauseUntil, accountID)
		g.mu.Unlock()
		return false
	}

	return true
}

// GetRemainingPause 返回剩余暂停时长
func (g *SessionGuard) GetRemainingPause(accountID string) time.Duration {
	g.mu.RLock()
	until, exists := g.pauseUntil[accountID]
	g.mu.RUnlock()

	if !exists {
		return 0
	}

	remaining := time.Until(until)
	if remaining <= 0 {
		return 0
	}
	return remaining
}

// ShouldRelogin 判断是否应该触发重新登录
// 连续暂停次数超过阈值后返回 true
func (g *SessionGuard) ShouldRelogin(accountID string) bool {
	g.mu.RLock()
	count := g.pauseCount[accountID]
	g.mu.RUnlock()
	return count >= maxSessionPauseAttempts
}

// ClearPause 清除指定账号的暂停状态（re-login 成功后调用）
func (g *SessionGuard) ClearPause(accountID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.pauseUntil, accountID)
	delete(g.pauseCount, accountID)
}

// AssertActive 断言会话处于活跃状态，否则返回错误
// 用于保护出站 API 调用
func (g *SessionGuard) AssertActive(accountID string) error {
	if g.IsPaused(accountID) {
		remaining := g.GetRemainingPause(accountID)
		return fmt.Errorf("会话已暂停，剩余 %.0f 分钟 (errcode %d)",
			remaining.Minutes(), sessionExpiredErrCode)
	}
	return nil
}
