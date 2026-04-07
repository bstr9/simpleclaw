package wechatmp

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/bstr9/simpleclaw/extensions/wechatmp/tools"
	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/channel/wechatmp"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
)

var defaultExtension *WechatMPExtension

func init() {
	defaultExtension = New()
	extension.RegisterExtension(defaultExtension)
}

type WechatMPExtension struct {
	mu      sync.RWMutex
	channel *wechatmp.WechatmpChannel
	api     extension.ExtensionAPI
	started bool
}

func New() *WechatMPExtension {
	return &WechatMPExtension{}
}

func (e *WechatMPExtension) ID() string {
	return "wechatmp"
}

func (e *WechatMPExtension) Name() string {
	return "WechatMP"
}

func (e *WechatMPExtension) Description() string {
	return "微信公众号渠道扩展，支持消息收发、模板消息、菜单管理等"
}

func (e *WechatMPExtension) Version() string {
	return "1.0.0"
}

func (e *WechatMPExtension) Register(api extension.ExtensionAPI) error {
	e.mu.Lock()
	e.api = api
	e.mu.Unlock()

	api.RegisterChannel(channel.ChannelWechatMP, func() (channel.Channel, error) {
		return e.createChannel()
	})

	skillsDir := filepath.Join(api.ExtensionDir(), "wechatmp", "skills")
	api.RegisterSkillPath(skillsDir)

	cfg := config.Get()
	if cfg.WechatmpAppID != "" && cfg.WechatmpAppSecret != "" {
		mpTool := tools.NewWechatMPTool(cfg.WechatmpAppID, cfg.WechatmpAppSecret)
		api.RegisterTool(mpTool)
		logger.Info("[WechatMPExtension] Tool registered")
	}

	logger.Info("[WechatMPExtension] Extension registered")
	return nil
}

func (e *WechatMPExtension) ValidateConfig() error {
	cfg := config.Get()
	if cfg.WechatmpAppID == "" || cfg.WechatmpAppSecret == "" {
		return nil
	}
	logger.Info("[WechatMPExtension] Config validated")
	return nil
}

func (e *WechatMPExtension) Startup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return nil
	}
	e.started = true
	return nil
}

func (e *WechatMPExtension) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil
	}
	e.started = false
	return nil
}

func (e *WechatMPExtension) createChannel() (*wechatmp.WechatmpChannel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.channel != nil {
		return e.channel, nil
	}

	cfg := config.Get()
	if cfg.WechatmpAppID == "" || cfg.WechatmpAppSecret == "" {
		logger.Warn("[WechatMPExtension] 跳过微信公众号渠道：缺少 app_id 或 app_secret 配置")
		return nil, nil
	}

	e.channel = wechatmp.NewWechatmpChannel(nil)
	logger.Info("[WechatMPExtension] Channel created")
	return e.channel, nil
}
