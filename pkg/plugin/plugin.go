// Package plugin 提供插件系统，用于扩展 simpleclaw 应用程序。
// plugin.go 定义了 Plugin 接口和插件元数据。
package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/logger"
)

// 日志前缀
const logPluginPrefix = "[Plugin:"

// PluginContext 提供插件运行时的上下文环境。
type PluginContext struct {
	Config     map[string]any
	PluginDir  string
	PluginPath string
	DataDir    string
	LogDir     string
	EventBus   *EventBus
	PluginName string
}

// Debug 输出调试日志。
func (c *PluginContext) Debug(msg string) {
	logger.Debug(logPluginPrefix + c.PluginName + "] " + msg)
}

// Info 输出信息日志。
func (c *PluginContext) Info(msg string) {
	logger.Info(logPluginPrefix + c.PluginName + "] " + msg)
}

// Warn 输出警告日志。
func (c *PluginContext) Warn(msg string) {
	logger.Warn(logPluginPrefix + c.PluginName + "] " + msg)
}

// Error 输出错误日志。
func (c *PluginContext) Error(msg string) {
	logger.Error(logPluginPrefix + c.PluginName + "] " + msg)
}

// EventBus 提供事件发布订阅功能。
type EventBus struct {
	handlers map[Event][]EventHandler
	mu       sync.RWMutex
}

// NewEventBus 创建新的事件总线。
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[Event][]EventHandler),
	}
}

// Subscribe 订阅事件。
func (b *EventBus) Subscribe(event Event, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[event] = append(b.handlers[event], handler)
}

// Publish 发布事件。
func (b *EventBus) Publish(event Event, ec *EventContext) error {
	b.mu.RLock()
	handlers := b.handlers[event]
	b.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ec); err != nil {
			return err
		}
	}
	return nil
}

// Plugin 定义所有插件必须实现的接口。
type Plugin interface {
	// Name 返回插件的唯一标识名称。
	Name() string

	// Version 返回插件的版本字符串。
	Version() string

	// OnInit 在插件首次初始化时调用。
	// 此方法在 OnLoad 之前调用，用于基本设置。
	OnInit(ctx *PluginContext) error

	// OnLoad 在插件加载并启用时调用。
	// 用于注册事件处理器和启动插件功能。
	OnLoad(ctx *PluginContext) error

	// OnUnload 在插件卸载时调用。
	// 用于清理资源和注销处理器。
	OnUnload(ctx *PluginContext) error

	// OnEvent 在插件监听的事件触发时调用。
	// 返回错误表示事件处理失败。
	OnEvent(event Event, ec *EventContext) error
}

// EventHandler 是处理特定事件类型的函数。
type EventHandler func(ec *EventContext) error

// Metadata 包含插件元数据信息。
type Metadata struct {
	// Name 是插件的唯一标识符。
	Name string `json:"name"`

	// NameCN 是用于显示的中文名称。
	NameCN string `json:"name_cn,omitempty"`

	// Version 是插件版本字符串。
	Version string `json:"version"`

	// Description 是插件的简要描述。
	Description string `json:"description,omitempty"`

	// Author 是插件作者。
	Author string `json:"author,omitempty"`

	// Priority 决定插件执行顺序（数值越大越先执行）。
	Priority int `json:"priority"`

	// Hidden 决定插件是否对用户隐藏。
	Hidden bool `json:"hidden,omitempty"`

	// Enabled 决定插件当前是否启用。
	Enabled bool `json:"enabled"`

	// Dependencies 列出该插件依赖的其他插件。
	Dependencies []string `json:"dependencies,omitempty"`

	// Path 是插件目录的文件系统路径。
	Path string `json:"path,omitempty"`
}

// BasePlugin 提供 Plugin 接口的基础实现。
// 插件可以嵌入此结构体以避免实现所有方法。
type BasePlugin struct {
	mu       sync.RWMutex
	metadata *Metadata
	handlers map[Event]EventHandler
	config   map[string]any
	path     string
}

// NewBasePlugin 创建一个具有默认值的新 BasePlugin。
func NewBasePlugin(name, version string) *BasePlugin {
	return &BasePlugin{
		metadata: &Metadata{
			Name:     name,
			Version:  version,
			Priority: 0,
			Enabled:  true,
		},
		handlers: make(map[Event]EventHandler),
		config:   make(map[string]any),
	}
}

// Name 返回插件名称。
func (p *BasePlugin) Name() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata.Name
}

// Version 返回插件版本。
func (p *BasePlugin) Version() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata.Version
}

// OnInit 提供默认初始化（不做任何操作）。
func (p *BasePlugin) OnInit(ctx *PluginContext) error {
	return nil
}

// OnLoad 提供默认加载（不做任何操作）。
func (p *BasePlugin) OnLoad(ctx *PluginContext) error {
	return nil
}

// OnUnload 提供默认卸载（清除处理器）。
func (p *BasePlugin) OnUnload(ctx *PluginContext) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers = make(map[Event]EventHandler)
	return nil
}

// OnEvent 将事件分发给已注册的处理器。
func (p *BasePlugin) OnEvent(event Event, ec *EventContext) error {
	p.mu.RLock()
	handler, ok := p.handlers[event]
	p.mu.RUnlock()

	if !ok {
		return nil
	}
	return handler(ec)
}

// RegisterHandler 为特定事件类型注册事件处理器。
func (p *BasePlugin) RegisterHandler(event Event, handler EventHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[event] = handler
}

// UnregisterHandler 移除特定事件类型的事件处理器。
func (p *BasePlugin) UnregisterHandler(event Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.handlers, event)
}

// GetMetadata 返回插件元数据。
func (p *BasePlugin) GetMetadata() *Metadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata
}

// SetMetadata 设置插件元数据。
func (p *BasePlugin) SetMetadata(meta *Metadata) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadata = meta
}

// SetPriority 设置插件优先级。
func (p *BasePlugin) SetPriority(priority int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadata.Priority = priority
}

// SetDescription 设置插件描述。
func (p *BasePlugin) SetDescription(desc string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadata.Description = desc
}

// SetAuthor 设置插件作者。
func (p *BasePlugin) SetAuthor(author string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadata.Author = author
}

// SetHidden 设置插件是否隐藏。
func (p *BasePlugin) SetHidden(hidden bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadata.Hidden = hidden
}

// SetEnabled 设置插件是否启用。
func (p *BasePlugin) SetEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadata.Enabled = enabled
}

// SetDependencies 设置插件依赖项。
func (p *BasePlugin) SetDependencies(deps []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadata.Dependencies = deps
}

// SetPath 设置插件路径。
func (p *BasePlugin) SetPath(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metadata.Path = path
	p.path = path
}

// Path 返回插件路径。
func (p *BasePlugin) Path() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.path
}

// LoadConfig 从 JSON 文件加载插件配置。
// 首先尝试从全局配置目录加载，然后从插件目录加载。
func (p *BasePlugin) LoadConfig(globalConfigDir string) (map[string]any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 首先尝试全局配置
	if globalConfigDir != "" {
		globalConfigPath := filepath.Join(globalConfigDir, p.metadata.Name+".json")
		if data, err := os.ReadFile(globalConfigPath); err == nil {
			var config map[string]any
			if err := json.Unmarshal(data, &config); err != nil {
				logger.Warn(fmt.Sprintf("[Plugin:%s] 解析全局配置文件失败 %s: %v", p.metadata.Name, globalConfigPath, err))
			} else {
				p.config = config
				return config, nil
			}
		}
	}

	// 尝试插件目录配置
	if p.path != "" {
		configPath := filepath.Join(p.path, "config.json")
		if data, err := os.ReadFile(configPath); err == nil {
			var config map[string]any
			if err := json.Unmarshal(data, &config); err != nil {
				logger.Warn(fmt.Sprintf("[Plugin:%s] 解析插件配置文件失败 %s: %v", p.metadata.Name, configPath, err))
			} else {
				p.config = config
				return config, nil
			}
		}
	}

	return p.config, nil
}

// SaveConfig 将插件配置保存到 JSON 文件。
func (p *BasePlugin) SaveConfig(globalConfigDir string) error {
	p.mu.RLock()
	config := p.config
	p.mu.RUnlock()

	if globalConfigDir == "" {
		return nil
	}

	// 确保目录存在
	if err := os.MkdirAll(globalConfigDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(globalConfigDir, p.metadata.Name+".json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// GetConfig 根据键返回配置值。
func (p *BasePlugin) GetConfig(key string) (any, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	val, ok := p.config[key]
	return val, ok
}

// SetConfig 设置配置值。
func (p *BasePlugin) SetConfig(key string, value any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config[key] = value
}

// GetConfigString 返回字符串类型的配置值。
func (p *BasePlugin) GetConfigString(key string) string {
	if val, ok := p.GetConfig(key); ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// GetConfigInt 返回整数类型的配置值。
func (p *BasePlugin) GetConfigInt(key string) int {
	if val, ok := p.GetConfig(key); ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

// GetConfigBool 返回布尔类型的配置值。
func (p *BasePlugin) GetConfigBool(key string) bool {
	if val, ok := p.GetConfig(key); ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// HelpText 返回插件的帮助文本。
func (p *BasePlugin) HelpText() string {
	return "No help information available"
}
