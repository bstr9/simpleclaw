package weixin

import (
	"context"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/channel/weixin"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/pair"
	"github.com/bstr9/simpleclaw/pkg/pair/providers"
	"go.uber.org/zap"
)

var defaultExtension *WeixinExtension

func init() {
	defaultExtension = New()
	extension.RegisterExtension(defaultExtension)
}

type WeixinExtension struct {
	mu          sync.RWMutex
	channel     *weixin.WeixinChannel
	pairManager *pair.Manager
	api         extension.ExtensionAPI
	started     bool
}

func New() *WeixinExtension {
	return &WeixinExtension{}
}

func (e *WeixinExtension) ID() string {
	return "weixin"
}

func (e *WeixinExtension) Name() string {
	return "Weixin"
}

func (e *WeixinExtension) Description() string {
	return "微信渠道扩展，支持个人微信消息收发"
}

func (e *WeixinExtension) Version() string {
	return "1.0.0"
}

func (e *WeixinExtension) Register(api extension.ExtensionAPI) error {
	e.mu.Lock()
	e.api = api
	e.mu.Unlock()

	api.RegisterChannel(channel.ChannelWeixin, func() (channel.Channel, error) {
		return e.createChannel()
	})

	api.RegisterChannel(channel.ChannelWeixinAlias, func() (channel.Channel, error) {
		return e.createChannel()
	})

	logger.Info("[WeixinExtension] Extension registered")
	return nil
}

func (e *WeixinExtension) Startup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return nil
	}

	// 初始化 PairManager
	cfg := config.Get()
	if cfg.PairEnabled {
		if err := e.initPairManager(); err != nil {
			logger.Warn("[WeixinExtension] PairManager 初始化失败", zap.Error(err))
		}
	}

	e.started = true
	return nil
}

func (e *WeixinExtension) initPairManager() error {
	workspaceDir := config.Get().AgentWorkspace
	if workspaceDir == "" {
		workspaceDir = "./data/workspace"
	}

	store, err := pair.NewStore(workspaceDir)
	if err != nil {
		return err
	}

	e.pairManager = pair.NewManager(store)

	// 注册微信 Provider，绑定渠道的登录状态和二维码 URL
	weixinProvider := providers.NewWeixinProvider()
	e.pairManager.RegisterProvider(weixinProvider)

	// 启动过期数据清理循环
	cleanupInterval := 30 * time.Minute
	if config.Get().PairCleanupInterval > 0 {
		cleanupInterval = time.Duration(config.Get().PairCleanupInterval) * time.Minute
	}
	e.pairManager.StartCleanupLoop(cleanupInterval)

	// 如果渠道已创建，绑定函数注入
	if e.channel != nil {
		weixinProvider.SetLoginStatusFunc(e.channel.GetLoginStatusString)
		weixinProvider.SetQRURLFunc(e.channel.GetCurrentQRURL)
	}

	logger.Info("[WeixinExtension] PairManager 初始化完成",
		zap.Duration("cleanup_interval", cleanupInterval))
	return nil
}

func (e *WeixinExtension) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil
	}

	if e.pairManager != nil {
		if err := e.pairManager.Close(); err != nil {
			logger.Warn("[WeixinExtension] Failed to close PairManager", zap.Error(err))
		}
		e.pairManager = nil
	}

	e.started = false
	return nil
}

func (e *WeixinExtension) createChannel() (*weixin.WeixinChannel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.channel != nil {
		return e.channel, nil
	}

	e.channel = weixin.NewWeixinChannel()

	// 如果 PairManager 已初始化，绑定微信 Provider 的函数注入
	if e.pairManager != nil {
		if p, ok := e.pairManager.GetProvider("weixin").(*providers.WeixinProvider); ok {
			p.SetLoginStatusFunc(e.channel.GetLoginStatusString)
			p.SetQRURLFunc(e.channel.GetCurrentQRURL)
		}
	}

	logger.Info("[WeixinExtension] Channel created")
	return e.channel, nil
}
