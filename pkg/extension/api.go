// Package extension 提供统一的扩展系统。
// api.go 实现 ExtensionAPI 适配器。
package extension

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/extension/registry"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// API 实现 ExtensionAPI 接口。
// 将扩展注册转发到现有的各个子系统。
type API struct {
	mu sync.RWMutex

	workingDir   string
	extensionDir string
	config       map[string]any
	extConfig    map[string]any

	channelRegistry *channelRegistry
	skillPaths      []string
	eventHandlers   map[string][]EventHandler
}

type channelRegistry struct {
	creators map[string]channel.ChannelCreator
}

// NewAPI 创建扩展 API。
func NewAPI(opts ...APIOption) *API {
	api := &API{
		config:        make(map[string]any),
		extConfig:     make(map[string]any),
		skillPaths:    make([]string, 0),
		eventHandlers: make(map[string][]EventHandler),
		channelRegistry: &channelRegistry{
			creators: make(map[string]channel.ChannelCreator),
		},
	}

	for _, opt := range opts {
		opt(api)
	}

	return api
}

// APIOption API 选项。
type APIOption func(*API)

// WithAPIWorkingDir 设置工作目录。
func WithAPIWorkingDir(dir string) APIOption {
	return func(a *API) {
		a.workingDir = dir
	}
}

// WithAPIExtensionDir 设置扩展目录。
func WithAPIExtensionDir(dir string) APIOption {
	return func(a *API) {
		a.extensionDir = dir
	}
}

// RegisterChannel 注册通道。
func (a *API) RegisterChannel(channelType string, creator channel.ChannelCreator) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.channelRegistry.creators[channelType]; exists {
		logger.Warn("[ExtensionAPI] Overwriting existing channel",
			zap.String("channel", channelType))
	}

	a.channelRegistry.creators[channelType] = creator
	channel.RegisterChannel(channelType, creator)

	logger.Info("[ExtensionAPI] Channel registered",
		zap.String("channel", channelType))
}

// RegisterTool 注册工具。
func (a *API) RegisterTool(tool agent.Tool) {
	registry.RegisterTool(tool)
	logger.Info("[ExtensionAPI] Tool registered",
		zap.String("tool", tool.Name()))
}

// RegisterSkillPath 注册技能目录。
func (a *API) RegisterSkillPath(path string) {
	a.mu.Lock()
	a.skillPaths = append(a.skillPaths, path)
	a.mu.Unlock()

	registry.RegisterSkillPath(path)

	logger.Info("[ExtensionAPI] Skill path registered",
		zap.String("path", path))
}

// RegisterEventHandler 注册事件处理器。
func (a *API) RegisterEventHandler(event string, handler EventHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.eventHandlers[event] = append(a.eventHandlers[event], handler)
	logger.Debug("[ExtensionAPI] Event handler registered",
		zap.String("event", event))
}

// Logger 获取日志器。
func (a *API) Logger() *zap.Logger {
	return logger.GetLogger()
}

// Config 获取扩展配置。
func (a *API) Config(key string) any {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.extConfig != nil {
		for _, cfg := range a.extConfig {
			if m, ok := cfg.(map[string]any); ok {
				if v, exists := m[key]; exists {
					return v
				}
			}
		}
	}

	if a.config != nil {
		return a.config[key]
	}

	return nil
}

// ConfigString 获取字符串配置。
func (a *API) ConfigString(key string) string {
	v := a.Config(key)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// WorkingDir 获取工作目录。
func (a *API) WorkingDir() string {
	return a.workingDir
}

// ExtensionDir 获取扩展目录。
func (a *API) ExtensionDir() string {
	return a.extensionDir
}

// GetSkillPaths 获取所有技能目录。
func (a *API) GetSkillPaths() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	paths := make([]string, len(a.skillPaths))
	copy(paths, a.skillPaths)
	return paths
}

// GetChannelCreators 获取所有通道创建器。
func (a *API) GetChannelCreators() map[string]channel.ChannelCreator {
	a.mu.RLock()
	defer a.mu.RUnlock()

	creators := make(map[string]channel.ChannelCreator)
	for k, v := range a.channelRegistry.creators {
		creators[k] = v
	}
	return creators
}

// EmitEvent 触发事件。
func (a *API) EmitEvent(ctx context.Context, event string, data any) error {
	a.mu.RLock()
	handlers := a.eventHandlers[event]
	a.mu.RUnlock()

	for _, h := range handlers {
		if err := h(ctx, data); err != nil {
			logger.Error("[ExtensionAPI] Event handler error",
				zap.String("event", event),
				zap.Error(err))
			return err
		}
	}
	return nil
}

// ResolvePath 解析路径。
// 相对路径会相对于扩展目录解析。
func (a *API) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(a.extensionDir, path)
}
