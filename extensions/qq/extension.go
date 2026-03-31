package qq

import (
	"context"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/channel/qq"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
)

var defaultExtension *QQExtension

func init() {
	defaultExtension = New()
	extension.RegisterExtension(defaultExtension)
}

type QQExtension struct {
	mu      sync.RWMutex
	channel *qq.QQChannel
	api     extension.ExtensionAPI
	started bool
}

func New() *QQExtension {
	return &QQExtension{}
}

func (e *QQExtension) ID() string {
	return "qq"
}

func (e *QQExtension) Name() string {
	return "QQ"
}

func (e *QQExtension) Description() string {
	return "QQ渠道扩展，支持QQ消息收发"
}

func (e *QQExtension) Version() string {
	return "1.0.0"
}

func (e *QQExtension) Register(api extension.ExtensionAPI) error {
	e.mu.Lock()
	e.api = api
	e.mu.Unlock()

	api.RegisterChannel(channel.ChannelQQ, func() (channel.Channel, error) {
		return e.createChannel()
	})

	logger.Info("[QQExtension] Extension registered")
	return nil
}

func (e *QQExtension) Startup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return nil
	}
	e.started = true
	return nil
}

func (e *QQExtension) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil
	}
	e.started = false
	return nil
}

func (e *QQExtension) createChannel() (*qq.QQChannel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.channel != nil {
		return e.channel, nil
	}

	e.channel = qq.NewQQChannel()
	logger.Info("[QQExtension] Channel created")
	return e.channel, nil
}
