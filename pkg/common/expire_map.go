// Package common 提供过期字典实现
package common

import (
	"sync"
	"time"
)

// expireEntry 过期条目
type expireEntry[T any] struct {
	value      T
	expiryTime time.Time
}

// ExpireMap 过期字典
// 存储的键值对会在指定时间后自动过期
type ExpireMap[K comparable, V any] struct {
	mu             sync.RWMutex
	data           map[K]expireEntry[V]
	expireDuration time.Duration
	cleanupTicker  *time.Ticker
	stopCleanup    chan struct{}
}

// NewExpireMap 创建新的过期字典
// expireDuration: 过期时间间隔
func NewExpireMap[K comparable, V any](expireDuration time.Duration) *ExpireMap[K, V] {
	em := &ExpireMap[K, V]{
		data:           make(map[K]expireEntry[V]),
		expireDuration: expireDuration,
		stopCleanup:    make(chan struct{}),
	}

	// 启动定期清理过期条目的协程
	em.cleanupTicker = time.NewTicker(expireDuration / 2)
	go em.cleanupLoop()

	return em
}

// cleanupLoop 定期清理过期条目
func (em *ExpireMap[K, V]) cleanupLoop() {
	for {
		select {
		case <-em.cleanupTicker.C:
			em.CleanExpired()
		case <-em.stopCleanup:
			return
		}
	}
}

// Get 获取键对应的值
// 如果键不存在或已过期，返回零值和false
func (em *ExpireMap[K, V]) Get(key K) (V, bool) {
	em.mu.RLock()
	entry, exists := em.data[key]
	em.mu.RUnlock()

	if !exists {
		var zero V
		return zero, false
	}

	// 检查是否过期
	if time.Now().After(entry.expiryTime) {
		em.mu.Lock()
		delete(em.data, key)
		em.mu.Unlock()
		var zero V
		return zero, false
	}

	// 更新过期时间（刷新）
	em.mu.Lock()
	entry.expiryTime = time.Now().Add(em.expireDuration)
	em.data[key] = entry
	em.mu.Unlock()

	return entry.value, true
}

// GetWithoutRefresh 获取键对应的值但不刷新过期时间
func (em *ExpireMap[K, V]) GetWithoutRefresh(key K) (V, bool) {
	em.mu.RLock()
	entry, exists := em.data[key]
	em.mu.RUnlock()

	if !exists {
		var zero V
		return zero, false
	}

	if time.Now().After(entry.expiryTime) {
		em.mu.Lock()
		delete(em.data, key)
		em.mu.Unlock()
		var zero V
		return zero, false
	}

	return entry.value, true
}

// Set 设置键值对
func (em *ExpireMap[K, V]) Set(key K, value V) {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.data[key] = expireEntry[V]{
		value:      value,
		expiryTime: time.Now().Add(em.expireDuration),
	}
}

// SetWithExpiry 设置键值对并指定过期时间
func (em *ExpireMap[K, V]) SetWithExpiry(key K, value V, expiry time.Time) {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.data[key] = expireEntry[V]{
		value:      value,
		expiryTime: expiry,
	}
}

// SetWithDuration 设置键值对并指定过期时长
func (em *ExpireMap[K, V]) SetWithDuration(key K, value V, duration time.Duration) {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.data[key] = expireEntry[V]{
		value:      value,
		expiryTime: time.Now().Add(duration),
	}
}

// Delete 删除键
func (em *ExpireMap[K, V]) Delete(key K) {
	em.mu.Lock()
	defer em.mu.Unlock()
	delete(em.data, key)
}

// Contains 检查键是否存在且未过期
func (em *ExpireMap[K, V]) Contains(key K) bool {
	_, exists := em.Get(key)
	return exists
}

// Len 返回未过期条目的数量
func (em *ExpireMap[K, V]) Len() int {
	em.mu.RLock()
	defer em.mu.RUnlock()

	count := 0
	now := time.Now()
	for _, entry := range em.data {
		if now.Before(entry.expiryTime) {
			count++
		}
	}
	return count
}

// Keys 返回所有未过期的键
func (em *ExpireMap[K, V]) Keys() []K {
	em.mu.RLock()
	defer em.mu.RUnlock()

	now := time.Now()
	keys := make([]K, 0)
	for key, entry := range em.data {
		if now.Before(entry.expiryTime) {
			keys = append(keys, key)
		}
	}
	return keys
}

// Values 返回所有未过期的值
func (em *ExpireMap[K, V]) Values() []V {
	em.mu.RLock()
	defer em.mu.RUnlock()

	now := time.Now()
	values := make([]V, 0)
	for _, entry := range em.data {
		if now.Before(entry.expiryTime) {
			values = append(values, entry.value)
		}
	}
	return values
}

// Items 返回所有未过期的键值对
func (em *ExpireMap[K, V]) Items() map[K]V {
	em.mu.RLock()
	defer em.mu.RUnlock()

	now := time.Now()
	result := make(map[K]V)
	for key, entry := range em.data {
		if now.Before(entry.expiryTime) {
			result[key] = entry.value
		}
	}
	return result
}

// CleanExpired 清理所有过期条目
func (em *ExpireMap[K, V]) CleanExpired() int {
	em.mu.Lock()
	defer em.mu.Unlock()

	now := time.Now()
	count := 0
	for key, entry := range em.data {
		if now.After(entry.expiryTime) {
			delete(em.data, key)
			count++
		}
	}
	return count
}

// Clear 清空所有条目
func (em *ExpireMap[K, V]) Clear() {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.data = make(map[K]expireEntry[V])
}

// GetExpiryTime 获取键的过期时间
func (em *ExpireMap[K, V]) GetExpiryTime(key K) (time.Time, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	entry, exists := em.data[key]
	if !exists {
		return time.Time{}, false
	}
	return entry.expiryTime, true
}

// IsExpired 检查键是否已过期
func (em *ExpireMap[K, V]) IsExpired(key K) bool {
	em.mu.RLock()
	entry, exists := em.data[key]
	em.mu.RUnlock()

	if !exists {
		return true
	}
	return time.Now().After(entry.expiryTime)
}

// Refresh 刷新键的过期时间
func (em *ExpireMap[K, V]) Refresh(key K) bool {
	em.mu.Lock()
	defer em.mu.Unlock()

	entry, exists := em.data[key]
	if !exists || time.Now().After(entry.expiryTime) {
		return false
	}

	entry.expiryTime = time.Now().Add(em.expireDuration)
	em.data[key] = entry
	return true
}

// Close 关闭过期字典，停止清理协程
func (em *ExpireMap[K, V]) Close() {
	if em.cleanupTicker != nil {
		em.cleanupTicker.Stop()
	}
	close(em.stopCleanup)
}

// SetExpireDuration 设置新的过期时长
// 仅影响后续设置的键值对
func (em *ExpireMap[K, V]) SetExpireDuration(duration time.Duration) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.expireDuration = duration
}

// GetExpireDuration 获取当前过期时长
func (em *ExpireMap[K, V]) GetExpireDuration() time.Duration {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.expireDuration
}

// ExpireMapString 字符串类型的过期字典快捷创建
type ExpireMapString = *ExpireMap[string, string]

// NewExpireMapString 创建字符串类型的过期字典
func NewExpireMapString(expireDuration time.Duration) ExpireMapString {
	return NewExpireMap[string, string](expireDuration)
}

// ExpireMapAny 任意类型的过期字典
type ExpireMapAny = *ExpireMap[string, any]

// NewExpireMapAny 创建任意类型的过期字典
func NewExpireMapAny(expireDuration time.Duration) ExpireMapAny {
	return NewExpireMap[string, any](expireDuration)
}
