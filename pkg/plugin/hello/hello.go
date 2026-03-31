// Package hello 提供一个简单的示例插件，展示插件的基本结构和功能。
// 支持群聊欢迎消息、拍一拍响应、Hello/Hi 消息处理等。
package hello

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/plugin"
	"github.com/bstr9/simpleclaw/pkg/types"
)

// Config 表示 hello 插件的配置
type Config struct {
	// GroupWelcomeFixedMsg 群聊固定欢迎消息映射（群名 -> 欢迎语）
	GroupWelcomeFixedMsg map[string]string `json:"group_welc_fixed_msg"`
	// GroupWelcomePrompt 群聊欢迎提示词模板
	GroupWelcomePrompt string `json:"group_welc_prompt"`
	// GroupExitPrompt 退群提示词模板
	GroupExitPrompt string `json:"group_exit_prompt"`
	// PatPatPrompt 拍一拍提示词模板
	PatPatPrompt string `json:"patpat_prompt"`
	// UseCharacterDesc 是否使用角色描述
	UseCharacterDesc bool `json:"use_character_desc"`
}

// HelloPlugin 实现 Hello 示例插件
type HelloPlugin struct {
	*plugin.BasePlugin

	mu     sync.RWMutex
	config *Config
}

// 确保 HelloPlugin 实现了 Plugin 接口
var _ plugin.Plugin = (*HelloPlugin)(nil)

// New 创建新的 HelloPlugin 实例
func New() *HelloPlugin {
	bp := plugin.NewBasePlugin("hello", "0.1.0")
	bp.SetDescription("Hello 示例插件，展示插件基本功能")
	bp.SetAuthor("simpleclaw")
	bp.SetPriority(-1)
	bp.SetHidden(true)

	return &HelloPlugin{
		BasePlugin: bp,
		config: &Config{
			GroupWelcomeFixedMsg: make(map[string]string),
			GroupWelcomePrompt:   "请你随机使用一种风格说一句问候语来欢迎新用户\"{nickname}\"加入群聊。",
			GroupExitPrompt:      "请你随机使用一种风格跟其他群用户说他违反规则\"{nickname}\"退出群聊。",
			PatPatPrompt:         "请你随机使用一种风格介绍你自己，并告诉用户输入#help可以查看帮助信息。",
			UseCharacterDesc:     false,
		},
	}
}

// Name 返回插件名称
func (p *HelloPlugin) Name() string {
	return "hello"
}

// Version 返回插件版本
func (p *HelloPlugin) Version() string {
	return "0.1.0"
}

// OnInit 初始化插件
func (p *HelloPlugin) OnInit(ctx *plugin.PluginContext) error {
	ctx.Debug("[hello] 正在初始化 Hello 插件")

	// 加载配置文件
	configPath := filepath.Join(ctx.PluginPath, "config.json")
	if err := p.loadConfig(configPath); err != nil {
		if os.IsNotExist(err) {
			// 尝试加载模板配置
			templatePath := filepath.Join(ctx.PluginPath, "config.json.template")
			if templateErr := p.loadConfig(templatePath); templateErr != nil {
				ctx.Warn("[hello] 加载配置失败，使用默认配置")
			}
		} else {
			ctx.Warn("[hello] 加载配置失败: " + err.Error())
		}
	}

	ctx.Info("[hello] 插件初始化完成")
	return nil
}

// OnLoad 加载插件
func (p *HelloPlugin) OnLoad(ctx *plugin.PluginContext) error {
	ctx.Debug("[hello] 正在加载 Hello 插件")

	// 注册事件处理器
	p.RegisterHandler(plugin.EventOnHandleContext, p.onHandleContext)

	ctx.Info("[hello] 插件加载成功")
	return nil
}

// OnUnload 卸载插件
func (p *HelloPlugin) OnUnload(ctx *plugin.PluginContext) error {
	ctx.Debug("[hello] 正在卸载 Hello 插件")
	return p.BasePlugin.OnUnload(ctx)
}

// OnEvent 处理插件事件
func (p *HelloPlugin) OnEvent(event plugin.Event, ec *plugin.EventContext) error {
	return p.BasePlugin.OnEvent(event, ec)
}

// onHandleContext 处理 ON_HANDLE_CONTEXT 事件
func (p *HelloPlugin) onHandleContext(ec *plugin.EventContext) error {
	// 获取消息类型
	contextType, ok := ec.GetInt("type")
	if !ok {
		return nil
	}

	// 只处理特定类型消息
	switch types.ContextType(contextType) {
	case types.ContextText:
		return p.handleTextMessage(ec)
	case types.ContextJoinGroup:
		return p.handleJoinGroup(ec)
	case types.ContextExitGroup:
		return p.handleExitGroup(ec)
	case types.ContextPatPat:
		return p.handlePatPat(ec)
	}

	return nil
}

// handleTextMessage 处理文本消息
func (p *HelloPlugin) handleTextMessage(ec *plugin.EventContext) error {
	content, ok := ec.GetString("content")
	if !ok {
		return nil
	}

	isGroup, _ := ec.GetBool("is_group")
	nickname, _ := ec.GetString("nickname")
	groupName, _ := ec.GetString("group_name")

	switch content {
	case "Hello":
		reply := types.NewTextReply(p.buildHelloReply(isGroup, nickname, groupName))
		ec.Set("reply", reply)
		ec.BreakPass("hello")

	case "Hi":
		reply := types.NewTextReply("Hi")
		ec.Set("reply", reply)
		ec.Break("hello")

	case "End":
		// 转换为图片生成请求
		ec.Set("type", int(types.ContextImageCreate))
		ec.Set("content", "The World")
		// 继续事件链，让后续处理器处理
	}

	return nil
}

// handleJoinGroup 处理加入群聊事件
func (p *HelloPlugin) handleJoinGroup(ec *plugin.EventContext) error {
	groupName, _ := ec.GetString("group_name")
	nickname, _ := ec.GetString("nickname")

	p.mu.RLock()
	fixedMsg, hasFixedMsg := p.config.GroupWelcomeFixedMsg[groupName]
	prompt := p.config.GroupWelcomePrompt
	p.mu.RUnlock()

	// 如果有固定欢迎语，直接使用
	if hasFixedMsg && fixedMsg != "" {
		reply := types.NewTextReply(fixedMsg)
		ec.Set("reply", reply)
		ec.BreakPass("hello")
		return nil
	}

	// 否则使用 AI 生成欢迎语
	ec.Set("type", int(types.ContextText))
	ec.Set("content", replacePlaceholder(prompt, nickname))
	ec.Break("hello")

	return nil
}

// handleExitGroup 处理退出群聊事件
func (p *HelloPlugin) handleExitGroup(ec *plugin.EventContext) error {
	nickname, _ := ec.GetString("nickname")

	p.mu.RLock()
	prompt := p.config.GroupExitPrompt
	p.mu.RUnlock()

	ec.Set("type", int(types.ContextText))
	ec.Set("content", replacePlaceholder(prompt, nickname))
	ec.Break("hello")

	return nil
}

// handlePatPat 处理拍一拍事件
func (p *HelloPlugin) handlePatPat(ec *plugin.EventContext) error {
	p.mu.RLock()
	prompt := p.config.PatPatPrompt
	p.mu.RUnlock()

	ec.Set("type", int(types.ContextText))
	ec.Set("content", prompt)
	ec.Break("hello")

	return nil
}

// buildHelloReply 构建 Hello 回复
func (p *HelloPlugin) buildHelloReply(isGroup bool, nickname, groupName string) string {
	if isGroup {
		if nickname == "" {
			nickname = "用户"
		}
		if groupName == "" {
			groupName = "群聊"
		}
		return "Hello, " + nickname + " from " + groupName
	}
	if nickname == "" {
		nickname = "用户"
	}
	return "Hello, " + nickname
}

// replacePlaceholder 替换模板中的占位符
func replacePlaceholder(template, nickname string) string {
	if nickname == "" {
		nickname = "用户"
	}
	result := template
	result = replaceAll(result, "{nickname}", nickname)
	return result
}

// replaceAll 替换所有匹配项
func replaceAll(s, old, new string) string {
	result := ""
	for {
		idx := findIndex(s, old)
		if idx == -1 {
			result += s
			break
		}
		result += s[:idx] + new
		s = s[idx+len(old):]
	}
	return result
}

// findIndex 查找子串位置
func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// loadConfig 加载配置文件
func (p *HelloPlugin) loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	p.mu.Lock()
	p.config = &config
	p.mu.Unlock()

	return nil
}

// HelpText 返回插件帮助文本
func (p *HelloPlugin) HelpText() string {
	return "输入Hello，我会回复你的名字\n输入End，我会回复你世界的图片\n"
}
