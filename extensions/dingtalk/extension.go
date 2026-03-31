package dingtalk

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/bstr9/simpleclaw/extensions/dingtalk/tools"
	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/channel/dingtalk"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
)

var defaultExtension *DingtalkExtension

func init() {
	defaultExtension = New()
	extension.RegisterExtension(defaultExtension)
}

type DingtalkExtension struct {
	mu      sync.RWMutex
	channel *dingtalk.DingtalkChannel
	api     extension.ExtensionAPI
	started bool
}

func New() *DingtalkExtension {
	return &DingtalkExtension{}
}

func (e *DingtalkExtension) ID() string {
	return "dingtalk"
}

func (e *DingtalkExtension) Name() string {
	return "Dingtalk"
}

func (e *DingtalkExtension) Description() string {
	return "钉钉渠道扩展，支持钉钉群消息收发、审批、消息推送等"
}

func (e *DingtalkExtension) Version() string {
	return "1.0.0"
}

func (e *DingtalkExtension) Register(api extension.ExtensionAPI) error {
	e.mu.Lock()
	e.api = api
	e.mu.Unlock()

	api.RegisterChannel(channel.ChannelDingtalk, func() (channel.Channel, error) {
		return e.createChannel()
	})

	skillsDir := filepath.Join(api.ExtensionDir(), "dingtalk", "skills")
	api.RegisterSkillPath(skillsDir)

	cfg := config.Get()
	if cfg.DingtalkClientID != "" && cfg.DingtalkClientSecret != "" {
		dtTool := tools.NewDingtalkTool(cfg.DingtalkClientID, cfg.DingtalkClientSecret)
		api.RegisterTool(dtTool)
		logger.Info("[DingtalkExtension] Tool registered")
	}

	logger.Info("[DingtalkExtension] Extension registered")
	return nil
}

func (e *DingtalkExtension) ValidateConfig() error {
	cfg := config.Get()
	if cfg.DingtalkClientID == "" || cfg.DingtalkClientSecret == "" {
		return nil
	}
	logger.Info("[DingtalkExtension] Config validated")
	return nil
}

func (e *DingtalkExtension) Startup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.started {
		return nil
	}
	e.started = true
	return nil
}

func (e *DingtalkExtension) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started {
		return nil
	}
	e.started = false
	return nil
}

func (e *DingtalkExtension) createChannel() (*dingtalk.DingtalkChannel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.channel != nil {
		return e.channel, nil
	}

	e.channel = dingtalk.NewDingtalkChannel(nil)
	logger.Info("[DingtalkExtension] Channel created")
	return e.channel, nil
}
