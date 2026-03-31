// Package extension 提供统一的扩展系统。
// extension.go 定义 Extension 接口和 ExtensionAPI。
package extension

import (
	"context"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/channel"
	"go.uber.org/zap"
)

// Extension 扩展接口。
// 所有扩展（通道、工具集等）都需要实现此接口。
type Extension interface {
	// ID 返回扩展唯一标识。
	ID() string

	// Name 返回扩展名称。
	Name() string

	// Description 返回扩展描述。
	Description() string

	// Version 返回扩展版本。
	Version() string

	// Register 注册扩展提供的组件。
	// 通过 api 参数注册通道、工具、技能等。
	Register(api ExtensionAPI) error

	// Startup 启动扩展（可选）。
	// 用于需要后台运行的扩展，如 WebSocket 连接。
	Startup(ctx context.Context) error

	// Shutdown 关闭扩展（可选）。
	// 用于清理资源。
	Shutdown(ctx context.Context) error
}

// ExtensionAPI 扩展注册 API。
// 提供给扩展使用的注册接口。
type ExtensionAPI interface {
	// RegisterChannel 注册通道。
	RegisterChannel(channelType string, creator channel.ChannelCreator)

	// RegisterTool 注册工具。
	RegisterTool(tool agent.Tool)

	// RegisterSkillPath 注册技能目录。
	RegisterSkillPath(path string)

	// RegisterEventHandler 注册事件处理器。
	RegisterEventHandler(event string, handler EventHandler)

	// Logger 获取日志器。
	Logger() *zap.Logger

	// Config 获取扩展配置。
	Config(key string) any

	// ConfigString 获取字符串配置。
	ConfigString(key string) string

	// WorkingDir 获取工作目录。
	WorkingDir() string

	// ExtensionDir 获取扩展目录。
	ExtensionDir() string
}

// EventHandler 事件处理器。
type EventHandler func(ctx context.Context, event any) error

// ConfigSchema 配置项定义。
type ConfigSchema struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
	Description string `json:"description"`
}

// ExtensionInfo 扩展元信息。
type ExtensionInfo struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Version     string          `json:"version"`
	ConfigPath  string          `json:"config_path,omitempty"`
	SkillsPath  string          `json:"skills_path,omitempty"`
	Configs     []*ConfigSchema `json:"configs,omitempty"`
}
