// Package tool 提供工具调用插件，支持多种工具来增强 AI 机器人的能力。
// 该文件包含工具注册表的实现，负责工具的注册和查找。
package tool

import "sync"

// Tool 定义工具接口，所有工具必须实现此接口。
type Tool interface {
	// Name 返回工具的唯一标识名称。
	Name() string

	// Description 返回工具的描述信息。
	Description() string

	// Run 执行工具并返回结果。
	Run(query string, config map[string]any) (string, error)
}

// 全局工具注册表实例。
var globalRegistry = &toolRegistry{
	tools: make(map[string]Tool),
	mu:    sync.RWMutex{},
}

// toolRegistry 工具注册表。
type toolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// RegisterTool 注册工具到全局注册表。
func RegisterTool(t Tool) {
	globalRegistry.Register(t)
}

// Register 注册工具。
func (r *toolRegistry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// GetTool 从注册表获取工具。
func (r *toolRegistry) GetTool(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}
