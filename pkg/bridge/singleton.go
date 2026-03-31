// Package bridge 提供消息处理的核心路由层
// singleton.go 实现线程安全的单例模式
package bridge

import (
	"sync"
)

// 单例实例
var (
	instance *Bridge
	once     sync.Once
)

// GetBridge 获取 Bridge 单例实例
// 使用 sync.Once 确保只创建一次
func GetBridge() *Bridge {
	once.Do(func() {
		instance = newBridge()
	})
	return instance
}

// ResetBridge 重置 Bridge 实例（仅用于测试）
func ResetBridge() {
	instance = nil
	once = sync.Once{}
}
