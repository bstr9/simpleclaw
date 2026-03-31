// Package feishu 飞书扩展。
// 提供飞书通道、lark-cli 工具封装、技能路径注册等。
package feishu

import (
	"context"
	"path/filepath"
	"sync"
	"time"

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
	mu           sync.RWMutex
	channel      *feishu.FeishuChannel
	api          extension.ExtensionAPI
	started      bool
	extensionDir string
	stopUpdate   chan struct{}
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

	// 注册扩展内技能目录
	skillsDir := filepath.Join(api.ExtensionDir(), "skills")
	api.RegisterSkillPath(skillsDir)

	// 注册 lark-cli 工具（封装 lark-cli 命令）
	larkCLITool := tools.NewLarkCLITool()
	api.RegisterTool(larkCLITool)
	logger.Info("[FeishuExtension] lark_cli tool registered")

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

	// 检测环境状态
	larkTool := tools.NewLarkCLITool()
	status := larkTool.Status()

	if !status["npm_installed"] {
		logger.Warn("[FeishuExtension] npm 未安装，请先安装 Node.js (https://nodejs.org/)")
	} else if !status["lark_cli_installed"] {
		logger.Warn("[FeishuExtension] lark-cli 未安装，请使用 lark_cli 工具的 install 操作进行安装")
	} else if !status["lark_skills_installed"] {
		logger.Warn("[FeishuExtension] lark-cli skills 未安装，请使用 lark_cli 工具的 install 操作进行安装")
	} else {
		logger.Info("[FeishuExtension] lark-cli 和 skills 已就绪")
		e.stopUpdate = make(chan struct{})
		go e.backgroundUpdateCheck()
	}

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

	// 停止后台更新检查
	if e.stopUpdate != nil {
		close(e.stopUpdate)
		e.stopUpdate = nil
	}

	if e.channel != nil {
		if err := e.channel.Stop(); err != nil {
			logger.Warn("[FeishuExtension] Failed to stop channel", zap.Error(err))
		}
	}

	e.started = false
	return nil
}

// backgroundUpdateCheck 后台定期检查更新（每天检查一次）。
func (e *FeishuExtension) backgroundUpdateCheck() {
	// 启动后 5 分钟进行首次检查
	select {
	case <-time.After(5 * time.Minute):
	case <-e.stopUpdate:
		return
	}

	e.checkAndNotifyUpdate()

	// 之后每 24 小时检查一次
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.checkAndNotifyUpdate()
		case <-e.stopUpdate:
			return
		}
	}
}

// checkAndNotifyUpdate 检查更新并通知。
func (e *FeishuExtension) checkAndNotifyUpdate() {
	info, err := tools.CheckUpdate()
	if err != nil {
		logger.Debug("[FeishuExtension] 检查更新失败", zap.Error(err))
		return
	}

	if info["update_available"] == "true" {
		logger.Info("[FeishuExtension] 发现新版本",
			zap.String("current", info["lark_cli_current"]),
			zap.String("latest", info["lark_cli_latest"]),
			zap.String("hint", "请使用 lark_cli 工具的 update 操作进行更新"))
	}
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
