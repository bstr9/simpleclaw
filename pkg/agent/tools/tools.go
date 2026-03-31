// Package tools 提供代理内置工具实现
package tools

import (
	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/config"
)

// RegisterBuiltInTools 注册所有内置工具到注册表
func RegisterBuiltInTools(registry *agent.ToolRegistry, opts ...ToolOption) {
	cfg := &toolConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	appConfig := config.Get()

	// 文件操作工具
	registry.Register(NewReadTool(cfg.workingDir))
	registry.Register(NewWriteTool(cfg.workingDir))
	registry.Register(NewEditTool(cfg.workingDir))
	registry.Register(NewLsTool(cfg.workingDir))

	// Web tools - 根据配置决定是否启用
	if appConfig.IsWebSearchEnabled() {
		webSearchOpts := []WebSearchOption{}
		if cfg.webSearchProvider != "" {
			webSearchOpts = append(webSearchOpts, WithSearchProvider(cfg.webSearchProvider))
		}
		if cfg.webSearchBaseURL != "" {
			webSearchOpts = append(webSearchOpts, WithSearchBaseURL(cfg.webSearchBaseURL))
		}
		if cfg.webSearchAPIKey != "" {
			webSearchOpts = append(webSearchOpts, WithSearchAPIKey(cfg.webSearchAPIKey))
		}
		registry.Register(NewWebSearchTool(webSearchOpts...))
	}
	if appConfig.IsWebFetchEnabled() {
		registry.Register(NewWebFetchTool(cfg.workingDir))
	}

	// 实用工具
	registry.Register(NewTimeTool())
	registry.Register(NewBashTool(
		WithBashWorkingDir(cfg.workingDir),
	))

	// 代理工具
	registry.Register(NewBrowserTool())
	registry.Register(NewEnvConfigTool())
	registry.Register(NewMemoryTool(cfg.workingDir))
	registry.Register(NewSchedulerTool())
	registry.Register(NewSendTool(cfg.workingDir))
	registry.Register(NewVisionTool())
}

// ToolOption 配置工具创建选项
type ToolOption func(*toolConfig)

type toolConfig struct {
	workingDir        string
	webSearchProvider string
	webSearchBaseURL  string
	webSearchAPIKey   string
}

// WithWorkingDir 设置文件操作工具的工作目录
func WithWorkingDir(dir string) ToolOption {
	return func(c *toolConfig) {
		c.workingDir = dir
	}
}
