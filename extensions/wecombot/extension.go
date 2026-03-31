package wecombot

import (
	"context"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/channel/wecombot"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
)

var defaultExtension *WecomBotExtension

func init() {
	defaultExtension = New()
	extension.RegisterExtension(defaultExtension)
}

type WecomBotExtension struct {
	mu      sync.RWMutex
	channel *wecombot.WecomBotChannel
	api     extension.ExtensionAPI
	started bool
}

func New() *WecomBotExtension {
	return &WecomBotExtension{}
}

func (e *WecomBotExtension) ID() string {
	return "wecombot"
}

func (e *WecomBotExtension) Name() string {
	return "WecomBot"
}

func (e *WecomBotExtension) Description() string {
	return "企业微信机器人渠道扩展"
}

func (e *WecomBotExtension) Version() string {
	return "1.0.0"
}

func (e *WecomBotExtension) Register(api extension.ExtensionAPI) error {
	e.mu.Lock()
	e.api = api
	e.mu.Unlock()

	api.RegisterChannel(channel.ChannelWecomBot, func() (channel.Channel, error) {
		return e.createChannel()
	})

	logger.Info("[WecomBotExtension] Extension registered")
	return nil
}

func (e *WecomBotExtension) Startup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return nil
	}
	e.started = true
	return nil
}

func (e *WecomBotExtension) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil
	}
	e.started = false
	return nil
}

func (e *WecomBotExtension) createChannel() (*wecombot.WecomBotChannel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.channel != nil {
		return e.channel, nil
	}

	e.channel = wecombot.NewWecomBotChannel(nil)
	logger.Info("[WecomBotExtension] Channel created")
	return e.channel, nil
}
