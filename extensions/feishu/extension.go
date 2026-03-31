// Package feishu 飞书扩展。
// 提供飞书通道、lark-cli 工具封装、技能路径注册等。
package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/extensions/feishu/tools"
	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/channel/feishu"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/pair"
	"github.com/bstr9/simpleclaw/pkg/pair/providers"
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
	pairManager  *pair.Manager
	api          extension.ExtensionAPI
	started      bool
	extensionDir string
	workspaceDir string
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

	// 注册 lark-cli 工具（封装 lark-cli 命令，与 channel 共享配置）
	cfg := config.Get()
	larkCLITool := tools.NewLarkCLITool(
		tools.WithAppCredentials(cfg.FeishuAppID, cfg.FeishuAppSecret),
	)
	api.RegisterTool(larkCLITool)
	logger.Info("[FeishuExtension] lark_cli tool registered",
		zap.String("app_id", cfg.FeishuAppID))

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

	// 初始化 PairManager
	if err := e.initPairManager(); err != nil {
		logger.Warn("[FeishuExtension] PairManager 初始化失败", zap.Error(err))
	}

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

		// 检查 lark-cli 配置是否与 simpleclaw 一致
		cfg := config.Get()
		if err := e.checkLarkCLIConfig(cfg); err != nil {
			logger.Warn("[FeishuExtension] lark-cli 配置检查失败", zap.Error(err))
		}

		e.stopUpdate = make(chan struct{})
		go e.backgroundUpdateCheck()
	}

	e.started = true

	return nil
}

func (e *FeishuExtension) initPairManager() error {
	workspaceDir := config.Get().AgentWorkspace
	if workspaceDir == "" {
		workspaceDir = "./data/workspace"
	}

	e.workspaceDir = workspaceDir

	store, err := pair.NewStore(workspaceDir)
	if err != nil {
		return fmt.Errorf("创建 PairStore 失败: %w", err)
	}

	e.pairManager = pair.NewManager(store)

	cfg := config.Get()
	feishuProvider := providers.NewFeishuProvider(cfg.FeishuAppID, cfg.FeishuAppSecret)
	e.pairManager.RegisterProvider(feishuProvider)

	logger.Info("[FeishuExtension] PairManager 初始化完成")
	return nil
}

// checkLarkCLIConfig 检查 lark-cli 配置是否与 simpleclaw 一致
func (e *FeishuExtension) checkLarkCLIConfig(cfg *config.Config) error {
	if cfg.FeishuAppID == "" {
		return nil
	}

	larkTool := tools.NewLarkCLITool(
		tools.WithAppCredentials(cfg.FeishuAppID, cfg.FeishuAppSecret),
	)

	result, err := larkTool.Execute(map[string]any{
		"command": "config show",
		"format":  "json",
	})
	if err != nil {
		logger.Warn("[FeishuExtension] 无法获取 lark-cli 配置，将尝试初始化", zap.Error(err))
		return e.initLarkCLIConfig(cfg)
	}

	var configData struct {
		AppID string `json:"appId"`
	}
	resultStr, ok := result.Result.(string)
	if !ok {
		return fmt.Errorf("lark-cli config show 返回非字符串结果")
	}
	if err := json.Unmarshal([]byte(resultStr), &configData); err != nil {
		return fmt.Errorf("解析 lark-cli 配置失败: %w", err)
	}

	if configData.AppID != cfg.FeishuAppID {
		logger.Info("[FeishuExtension] lark-cli 配置与 simpleclaw 不一致，重新初始化",
			zap.String("lark_cli_app_id", configData.AppID),
			zap.String("simpleclaw_app_id", cfg.FeishuAppID))
		return e.initLarkCLIConfig(cfg)
	}

	logger.Info("[FeishuExtension] lark-cli 配置检查通过", zap.String("app_id", cfg.FeishuAppID))
	return nil
}

// initLarkCLIConfig 初始化 lark-cli 配置
func (e *FeishuExtension) initLarkCLIConfig(cfg *config.Config) error {
	if cfg.FeishuAppID == "" || cfg.FeishuAppSecret == "" {
		return fmt.Errorf("feishu_app_id 或 feishu_app_secret 未配置")
	}

	larkTool := tools.NewLarkCLITool()

	_, err := larkTool.Execute(map[string]any{
		"command": fmt.Sprintf("config init --app-id %s --brand feishu", cfg.FeishuAppID),
	})
	if err != nil {
		return fmt.Errorf("初始化 lark-cli 配置失败: %w", err)
	}

	logger.Info("[FeishuExtension] lark-cli 配置初始化成功", zap.String("app_id", cfg.FeishuAppID))
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

	if e.pairManager != nil {
		e.channel.SetPairManager(&pairManagerAdapter{
			Manager:      e.pairManager,
			workspaceDir: e.workspaceDir,
		})
	}

	logger.Info("[FeishuExtension] Channel created",
		zap.String("app_id", feishuConfig.AppID),
		zap.String("event_mode", feishuConfig.EventMode))

	return e.channel, nil
}

type pairManagerAdapter struct {
	*pair.Manager
	workspaceDir string
}

func (a *pairManagerAdapter) CheckSessionPair(sessionID, userID, channelType string) (*feishu.PairCheckResult, error) {
	status, err := a.Manager.CheckSessionPair(sessionID, userID, channelType)
	if err != nil {
		return nil, err
	}
	return &feishu.PairCheckResult{
		Paired:  status.Paired,
		Status:  status.Status,
		AuthURL: status.AuthURL,
	}, nil
}

func (a *pairManagerAdapter) StartPair(sessionID, userID, channelType string) (*feishu.PairResult, error) {
	result, err := a.Manager.StartPair(sessionID, userID, channelType)
	if err != nil {
		return nil, err
	}
	return &feishu.PairResult{
		Success: result.Success,
		AuthURL: result.AuthURL,
		Message: result.Message,
	}, nil
}

func (a *pairManagerAdapter) CompletePair(sessionID, userID, channelType string) error {
	err := a.Manager.CompletePair(sessionID, userID, channelType)
	if err != nil {
		return err
	}

	auth, _ := a.Manager.GetUserAuth(userID, channelType)
	if auth != nil && auth.Name != "" {
		a.updateUserMD(auth.Name, userID)
	}

	return nil
}

func (a *pairManagerAdapter) updateUserMD(name, userID string) {
	if a.workspaceDir == "" {
		return
	}

	userMDPath := a.workspaceDir + "/USER.md"
	content, err := os.ReadFile(userMDPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	var updated []string
	for _, line := range lines {
		if strings.HasPrefix(line, "- **Name:**") {
			updated = append(updated, fmt.Sprintf("- **Name:** %s", name))
		} else if strings.HasPrefix(line, "- **What to call them:**") {
			updated = append(updated, fmt.Sprintf("- **What to call them:** %s", name))
		} else {
			updated = append(updated, line)
		}
	}

	os.WriteFile(userMDPath, []byte(strings.Join(updated, "\n")), 0644)
	logger.Info("[FeishuExtension] Updated USER.md with user name", zap.String("name", name))
}
