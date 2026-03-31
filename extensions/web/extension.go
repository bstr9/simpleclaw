package web

import (
	"context"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/channel/web"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
)

var defaultExtension *WebExtension

func init() {
	defaultExtension = New()
	extension.RegisterExtension(defaultExtension)
}

type WebExtension struct {
	mu      sync.RWMutex
	channel *web.WebChannel
	api     extension.ExtensionAPI
	started bool
}

func New() *WebExtension {
	return &WebExtension{}
}

func (e *WebExtension) ID() string {
	return "web"
}

func (e *WebExtension) Name() string {
	return "Web"
}

func (e *WebExtension) Description() string {
	return "Web渠道扩展，提供HTTP API接口"
}

func (e *WebExtension) Version() string {
	return "1.0.0"
}

func (e *WebExtension) Register(api extension.ExtensionAPI) error {
	e.mu.Lock()
	e.api = api
	e.mu.Unlock()

	api.RegisterChannel(channel.ChannelWeb, func() (channel.Channel, error) {
		return e.createChannel()
	})

	logger.Info("[WebExtension] Extension registered")
	return nil
}

func (e *WebExtension) Startup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return nil
	}
	e.started = true
	return nil
}

func (e *WebExtension) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil
	}
	if e.channel != nil {
		e.channel.Stop()
	}
	e.started = false
	return nil
}

func (e *WebExtension) createChannel() (*web.WebChannel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.channel != nil {
		return e.channel, nil
	}

	e.channel = web.NewWebChannel(nil)
	logger.Info("[WebExtension] Channel created")
	return e.channel, nil
}
