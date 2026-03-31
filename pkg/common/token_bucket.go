// Package common 提供令牌桶限流器
package common

import (
	"context"
	"sync"
	"time"
)

// TokenBucket 令牌桶限流器
// 用于控制请求速率，支持每分钟令牌数(TPM)限制
type TokenBucket struct {
	capacity  int           // 令牌桶容量
	tokens    int           // 当前令牌数
	rate      float64       // 令牌每秒生成速率
	timeout   time.Duration // 等待令牌超时时间
	mu        sync.Mutex    // 互斥锁
	cond      *sync.Cond    // 条件变量
	isRunning bool          // 运行状态
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewTokenBucket 创建新的令牌桶
// tpm: 每分钟令牌数(tokens per minute)
// timeout: 等待令牌超时时间，0表示无限等待
func NewTokenBucket(tpm int, timeout time.Duration) *TokenBucket {
	ctx, cancel := context.WithCancel(context.Background())
	tb := &TokenBucket{
		capacity:  tpm,
		tokens:    0,
		rate:      float64(tpm) / 60.0, // 每秒生成速率
		timeout:   timeout,
		isRunning: true,
		ctx:       ctx,
		cancel:    cancel,
	}
	tb.cond = sync.NewCond(&tb.mu)

	// 启动令牌生成协程
	go tb.generateTokens()

	return tb
}

// generateTokens 生成令牌的内部方法
func (tb *TokenBucket) generateTokens() {
	// 计算生成间隔
	interval := time.Duration(float64(time.Second) / tb.rate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-tb.ctx.Done():
			return
		case <-ticker.C:
			tb.mu.Lock()
			if tb.tokens < tb.capacity {
				tb.tokens++
			}
			tb.cond.Broadcast() // 通知等待的协程
			tb.mu.Unlock()
		}
	}
}

// GetToken 获取一个令牌
// 返回 true 表示成功获取令牌，false 表示超时
func (tb *TokenBucket) GetToken() bool {
	return tb.GetTokenWithContext(context.Background())
}

// GetTokenWithContext 带上下文的令牌获取
// 支持取消和超时控制
func (tb *TokenBucket) GetTokenWithContext(ctx context.Context) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	for tb.tokens <= 0 {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return false
		default:
		}

		// 如果设置了超时，使用定时等待
		if tb.timeout > 0 {
			// 使用channel实现超时等待
			timeoutChan := time.After(tb.timeout)

			// 释放锁并等待
			done := make(chan struct{})
			go func() {
				tb.cond.Wait()
				close(done)
			}()

			tb.mu.Unlock()
			select {
			case <-timeoutChan:
				tb.mu.Lock()
				return false
			case <-done:
				tb.mu.Lock()
				continue
			case <-ctx.Done():
				tb.mu.Lock()
				return false
			}
		}

		// 无限等待
		tb.cond.Wait()
	}

	tb.tokens--
	return true
}

// GetTokens 获取多个令牌
// count: 需要获取的令牌数量
// 返回实际获取的令牌数量
func (tb *TokenBucket) GetTokens(count int) int {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.tokens >= count {
		tb.tokens -= count
		return count
	}

	available := tb.tokens
	tb.tokens = 0
	return available
}

// TryGetToken 尝试获取令牌（非阻塞）
// 返回 true 表示成功获取令牌，false 表示无可用令牌
func (tb *TokenBucket) TryGetToken() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

// AvailableTokens 返回当前可用令牌数
func (tb *TokenBucket) AvailableTokens() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.tokens
}

// Capacity 返回令牌桶容量
func (tb *TokenBucket) Capacity() int {
	return tb.capacity
}

// Close 关闭令牌桶，停止令牌生成
func (tb *TokenBucket) Close() {
	tb.mu.Lock()
	tb.isRunning = false
	tb.mu.Unlock()

	tb.cancel()
	tb.cond.Broadcast()
}

// IsRunning 返回令牌桶是否正在运行
func (tb *TokenBucket) IsRunning() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.isRunning
}

// Refill 手动填充令牌
// count: 要添加的令牌数量
// 返回添加后的令牌数
func (tb *TokenBucket) Refill(count int) int {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.tokens += count
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.cond.Broadcast()
	return tb.tokens
}

// Rate 返回令牌生成速率（令牌/秒）
func (tb *TokenBucket) Rate() float64 {
	return tb.rate
}

// RatePerMinute 返回令牌生成速率（令牌/分钟）
func (tb *TokenBucket) RatePerMinute() float64 {
	return tb.rate * 60
}

// SetTimeout 设置等待超时时间
func (tb *TokenBucket) SetTimeout(timeout time.Duration) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.timeout = timeout
}
