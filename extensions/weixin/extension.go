package weixin

import (
	"context"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/channel/weixin"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
)

var defaultExtension *WeixinExtension

func init() {
	defaultExtension = New()
	extension.RegisterExtension(defaultExtension)
}

type WeixinExtension struct {
	mu      sync.RWMutex
	channel *weixin.WeixinChannel
	api     extension.ExtensionAPI
	started bool
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

	// 微信个人号不启动 PairManager：
	// WeixinProvider.CheckStatus() 始终返回 Paired:true，无需配对验证，
	// 启动清理循环纯属资源浪费。若未来微信接入 OAuth 授权，可在此启用。
	// 参考飞书扩展的 PairManager 集成模式（extensions/feishu/extension.go）

	e.started = true
	return nil
}

func (e *WeixinExtension) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil
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

	logger.Info("[WeixinExtension] Channel created")
	return e.channel, nil
}
