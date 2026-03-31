// Package feishu 飞书扩展。
// 提供飞书通道、文档工具、知识库工具等。
package feishu

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/bstr9/simpleclaw/extensions/feishu/tools"
	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/channel/feishu"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

var defaultExtension *FeishuExtension

func init() {
	defaultExtension = New()
	extension.RegisterExtension(defaultExtension)
}

// FeishuExtension 飞书扩展。
type FeishuExtension struct {
	mu sync.RWMutex

	channel      *feishu.FeishuChannel
	api          extension.ExtensionAPI
	started      bool
	extensionDir string
}

// New 创建飞书扩展。
func New() *FeishuExtension {
	return &FeishuExtension{}
}

// ID 返回扩展唯一标识。
func (e *FeishuExtension) ID() string {
	return "feishu"
}

// Name 返回扩展名称。
func (e *FeishuExtension) Name() string {
	return "Feishu"
}

// Description 返回扩展描述。
func (e *FeishuExtension) Description() string {
	return "飞书/Lark 渠道扩展，提供消息通道、文档工具、知识库工具等"
}

// Version 返回扩展版本。
func (e *FeishuExtension) Version() string {
	return "1.0.0"
}

// Register 注册扩展组件。
func (e *FeishuExtension) Register(api extension.ExtensionAPI) error {
	e.mu.Lock()
	e.api = api
	e.extensionDir = api.ExtensionDir()
	e.mu.Unlock()

	// 注册飞书通道
	api.RegisterChannel("feishu", func() (channel.Channel, error) {
		return e.createChannel()
	})

	// 注册技能目录
	skillsDir := filepath.Join(api.ExtensionDir(), "skills")
	api.RegisterSkillPath(skillsDir)

	// 注册飞书文档工具
	cfg := config.Get()
	if cfg.FeishuAppID != "" && cfg.FeishuAppSecret != "" {
		docTool := tools.NewFeishuDocTool(cfg.FeishuAppID, cfg.FeishuAppSecret)
		api.RegisterTool(docTool)
		logger.Info("[FeishuExtension] Doc tool registered")
	}

	logger.Info("[FeishuExtension] Extension registered")
	return nil
}

// Startup 启动扩展。
func (e *FeishuExtension) Startup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return nil
	}

	logger.Info("[FeishuExtension] Starting extension")
	e.started = true

	return nil
}

// Shutdown 关闭扩展。
func (e *FeishuExtension) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started {
		return nil
	}

	logger.Info("[FeishuExtension] Shutting down extension")

	if e.channel != nil {
		if err := e.channel.Stop(); err != nil {
			logger.Warn("[FeishuExtension] Failed to stop channel", zap.Error(err))
		}
	}

	e.started = false
	return nil
}

// createChannel 创建飞书通道。
func (e *FeishuExtension) createChannel() (*feishu.FeishuChannel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.channel != nil {
		return e.channel, nil
	}

	cfg := config.Get()

	feishuConfig := &feishu.Config{
		AppID:              cfg.FeishuAppID,
		AppSecret:          cfg.FeishuAppSecret,
		VerificationToken:  cfg.FeishuToken,
		Port:               cfg.FeishuPort,
		EventMode:          cfg.FeishuEventMode,
		BotName:            cfg.FeishuBotName,
		GroupSharedSession: cfg.GroupSharedSession,
		StreamOutput:       cfg.StreamOutput,
	}

	e.channel = feishu.NewFeishuChannel(feishuConfig)

	logger.Info("[FeishuExtension] Channel created",
		zap.String("app_id", feishuConfig.AppID),
		zap.String("event_mode", feishuConfig.EventMode))

	return e.channel, nil
}
