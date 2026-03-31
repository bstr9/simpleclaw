// Package finish 提供结束对话和未知命令检测插件。
// 该插件用于检测未知的插件命令并返回错误提示。
package finish

import (
	"strings"

	"github.com/bstr9/simpleclaw/pkg/plugin"
)

// FinishPlugin 实现结束对话插件。
type FinishPlugin struct {
	*plugin.BasePlugin

	config *Config
}

// Config 表示 finish 插件的配置。
type Config struct {
	// Enabled 是否启用插件。
	Enabled bool `json:"enabled"`

	// TriggerPrefix 触发前缀。
	TriggerPrefix string `json:"trigger_prefix"`

	// ErrorMessage 未知命令错误消息。
	ErrorMessage string `json:"error_message"`
}

// 确保 FinishPlugin 实现了 Plugin 接口。
var _ plugin.Plugin = (*FinishPlugin)(nil)

// New 创建一个新的 FinishPlugin 实例。
func New() *FinishPlugin {
	bp := plugin.NewBasePlugin("finish", "1.0.0")
	bp.SetDescription("检测未知插件命令")
	bp.SetAuthor("js00000")
	bp.SetPriority(-999)
	bp.SetHidden(true)

	p := &FinishPlugin{
		BasePlugin: bp,
		config: &Config{
			Enabled:       true,
			TriggerPrefix: "$",
			ErrorMessage:  "未知插件命令\n查看插件命令列表请输入#help 插件名\n",
		},
	}
	return p
}

// Name 返回插件名称。
func (p *FinishPlugin) Name() string {
	return "finish"
}

// Version 返回插件版本。
func (p *FinishPlugin) Version() string {
	return "1.0.0"
}

// OnInit 初始化插件。
func (p *FinishPlugin) OnInit(ctx *plugin.PluginContext) error {
	ctx.Debug("[finish] 正在初始化结束对话插件")

	// 加载配置
	if ctx.Config != nil {
		if enabled, ok := ctx.Config["enabled"].(bool); ok {
			p.config.Enabled = enabled
		}
		if prefix, ok := ctx.Config["trigger_prefix"].(string); ok {
			p.config.TriggerPrefix = prefix
		}
		if errMsg, ok := ctx.Config["error_message"].(string); ok {
			p.config.ErrorMessage = errMsg
		}
	}

	ctx.Info("[finish] 插件初始化完成")
	return nil
}

// OnLoad 插件加载时调用。
func (p *FinishPlugin) OnLoad(ctx *plugin.PluginContext) error {
	ctx.Debug("[finish] 正在加载结束对话插件")
	p.RegisterHandler(plugin.EventOnHandleContext, p.onHandleContext)
	ctx.Info("[finish] 插件加载成功")
	return nil
}

// OnUnload 插件卸载时调用。
func (p *FinishPlugin) OnUnload(ctx *plugin.PluginContext) error {
	ctx.Debug("[finish] 正在卸载结束对话插件")
	return p.BasePlugin.OnUnload(ctx)
}

// OnEvent 处理插件事件。
func (p *FinishPlugin) OnEvent(event plugin.Event, ec *plugin.EventContext) error {
	return p.BasePlugin.OnEvent(event, ec)
}

// onHandleContext 处理消息上下文事件。
func (p *FinishPlugin) onHandleContext(ec *plugin.EventContext) error {
	// 检查插件是否启用
	if !p.config.Enabled {
		return nil
	}

	// 获取消息内容
	content, ok := ec.GetString("content")
	if !ok {
		return nil
	}

	// 只处理文本消息
	msgType, ok := ec.GetString("type")
	if ok && msgType != "text" && msgType != "" {
		return nil
	}

	triggerPrefix := p.config.TriggerPrefix
	if triggerPrefix == "" {
		triggerPrefix = "$"
	}

	// 检查是否以触发前缀开头
	if strings.HasPrefix(content, triggerPrefix) {
		ec.Set("reply", p.config.ErrorMessage)
		ec.BreakPass(p.Name())
	}

	return nil
}

// HelpText 返回插件帮助文本。
func (p *FinishPlugin) HelpText() string {
	return ""
}
