package wechatcom

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/bstr9/simpleclaw/extensions/wechatcom/tools"
	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/channel/wechatcom"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
)

var defaultExtension *WechatComExtension

func init() {
	defaultExtension = New()
	extension.RegisterExtension(defaultExtension)
}

type WechatComExtension struct {
	mu      sync.RWMutex
	channel *wechatcom.WechatcomChannel
	api     extension.ExtensionAPI
	started bool
}

func New() *WechatComExtension {
	return &WechatComExtension{}
}

func (e *WechatComExtension) ID() string {
	return "wechatcom"
}

func (e *WechatComExtension) Name() string {
	return "WechatCom"
}

func (e *WechatComExtension) Description() string {
	return "企业微信应用渠道扩展，支持消息收发、通讯录管理等"
}

func (e *WechatComExtension) Version() string {
	return "1.0.0"
}

func (e *WechatComExtension) Register(api extension.ExtensionAPI) error {
	e.mu.Lock()
	e.api = api
	e.mu.Unlock()

	api.RegisterChannel(channel.ChannelWechatComApp, func() (channel.Channel, error) {
		return e.createChannel()
	})

	skillsDir := filepath.Join(api.ExtensionDir(), "wechatcom", "skills")
	api.RegisterSkillPath(skillsDir)

	cfg := config.Get()
	if cfg.WecomCorpID != "" {
		wcTool := tools.NewWechatComTool(cfg.WecomCorpID, "")
		api.RegisterTool(wcTool)
		logger.Info("[WechatComExtension] Tool registered")
	}

	logger.Info("[WechatComExtension] Extension registered")
	return nil
}

func (e *WechatComExtension) ValidateConfig() error {
	cfg := config.Get()
	if cfg.WecomCorpID == "" {
		return nil
	}
	logger.Info("[WechatComExtension] Config validated")
	return nil
}

func (e *WechatComExtension) Startup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return nil
	}
	e.started = true
	return nil
}

func (e *WechatComExtension) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil
	}
	e.started = false
	return nil
}

func (e *WechatComExtension) createChannel() (*wechatcom.WechatcomChannel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.channel != nil {
		return e.channel, nil
	}

	cfg := config.Get()
	if cfg.WecomCorpID == "" || cfg.WecomSecret == "" {
		logger.Warn("[WechatComExtension] 跳过企业微信渠道：缺少 corp_id 或 secret 配置")
		return nil, nil
	}

	e.channel = wechatcom.NewWechatcomChannel()
	logger.Info("[WechatComExtension] Channel created")
	return e.channel, nil
}
