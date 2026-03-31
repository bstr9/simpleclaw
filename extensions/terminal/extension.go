package terminal

import (
	"context"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/channel/terminal"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
)

var defaultExtension *TerminalExtension

func init() {
	defaultExtension = New()
	extension.RegisterExtension(defaultExtension)
}

type TerminalExtension struct {
	mu      sync.RWMutex
	channel *terminal.TerminalChannel
	api     extension.ExtensionAPI
	started bool
}

func New() *TerminalExtension {
	return &TerminalExtension{}
}

func (e *TerminalExtension) ID() string {
	return "terminal"
}

func (e *TerminalExtension) Name() string {
	return "Terminal"
}

func (e *TerminalExtension) Description() string {
	return "终端渠道扩展，提供命令行交互"
}

func (e *TerminalExtension) Version() string {
	return "1.0.0"
}

func (e *TerminalExtension) Register(api extension.ExtensionAPI) error {
	e.mu.Lock()
	e.api = api
	e.mu.Unlock()

	api.RegisterChannel(channel.ChannelTerminal, func() (channel.Channel, error) {
		return e.createChannel()
	})

	logger.Info("[TerminalExtension] Extension registered")
	return nil
}

func (e *TerminalExtension) Startup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return nil
	}
	e.started = true
	return nil
}

func (e *TerminalExtension) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil
	}
	e.started = false
	return nil
}

func (e *TerminalExtension) createChannel() (*terminal.TerminalChannel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.channel != nil {
		return e.channel, nil
	}

	e.channel = terminal.NewTerminalChannel()
	logger.Info("[TerminalExtension] Channel created")
	return e.channel, nil
}
