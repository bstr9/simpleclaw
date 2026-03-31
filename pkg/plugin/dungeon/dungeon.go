// Package dungeon 提供文字冒险游戏插件。
// 用户可以与机器人一起玩文字冒险游戏，创建个性化的故事情节。
package dungeon

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/plugin"
)

// StoryTeller 管理单个游戏会话的故事进程。
type StoryTeller struct {
	// bot 会话使用的机器人实例标识。
	bot string
	// sessionID 会话唯一标识。
	sessionID string
	// firstInteract 是否是第一次交互。
	firstInteract bool
	// story 故事背景。
	story string
	// mu 保护并发访问。
	mu sync.RWMutex
}

// NewStoryTeller 创建一个新的故事讲述者。
func NewStoryTeller(sessionID, story string) *StoryTeller {
	return &StoryTeller{
		bot:           "default",
		sessionID:     sessionID,
		firstInteract: true,
		story:         story,
	}
}

// Reset 重置游戏状态。
func (s *StoryTeller) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.firstInteract = true
}

// Action 根据用户行动生成提示词。
func (s *StoryTeller) Action(userAction string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 确保用户行动以句号结尾
	if !strings.HasSuffix(userAction, "。") {
		userAction = userAction + "。"
	}

	if s.firstInteract {
		prompt := fmt.Sprintf(
			"现在来充当一个文字冒险游戏，描述时候注意节奏，不要太快，仔细描述各个人物的心情和周边环境。一次只需写四到六句话。开头是，%s %s",
			s.story, userAction,
		)
		s.firstInteract = false
		return prompt
	}

	return "继续，一次只需要续写四到六句话，总共就只讲5分钟内发生的事情。" + userAction
}

// IsFirstInteract 返回是否是第一次交互。
func (s *StoryTeller) IsFirstInteract() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.firstInteract
}

// DungeonPlugin 实现文字冒险游戏插件。
type DungeonPlugin struct {
	*plugin.BasePlugin

	mu     sync.RWMutex
	games  map[string]*StoryTeller
	config *Config
}

// Config 表示 dungeon 插件的配置。
type Config struct {
	// Enabled 是否启用插件。
	Enabled bool `json:"enabled"`

	// TriggerPrefix 触发前缀。
	TriggerPrefix string `json:"trigger_prefix"`

	// DefaultStory 默认故事背景。
	DefaultStory string `json:"default_story"`

	// SessionTimeout 会话超时时间（秒）。
	SessionTimeout int `json:"session_timeout"`
}

// 确保 DungeonPlugin 实现了 Plugin 接口。
var _ plugin.Plugin = (*DungeonPlugin)(nil)

// New 创建一个新的 DungeonPlugin 实例。
func New() *DungeonPlugin {
	bp := plugin.NewBasePlugin("dungeon", "1.0.0")
	bp.SetDescription("文字冒险游戏插件")
	bp.SetAuthor("lanvent")
	bp.SetPriority(0)

	p := &DungeonPlugin{
		BasePlugin: bp,
		games:      make(map[string]*StoryTeller),
		config: &Config{
			Enabled:        true,
			TriggerPrefix:  "$",
			SessionTimeout: 3600,
			DefaultStory:   "你在树林里冒险，指不定会从哪里蹦出来一些奇怪的东西，你握紧手上的手枪，希望这次冒险能够找到一些值钱的东西，你往树林深处走去。",
		},
	}
	return p
}

// Name 返回插件名称。
func (p *DungeonPlugin) Name() string {
	return "dungeon"
}

// Version 返回插件版本。
func (p *DungeonPlugin) Version() string {
	return "1.0.0"
}

// OnInit 初始化插件。
func (p *DungeonPlugin) OnInit(ctx *plugin.PluginContext) error {
	ctx.Debug("[dungeon] 正在初始化文字冒险插件")

	// 加载配置
	if ctx.Config != nil {
		if enabled, ok := ctx.Config["enabled"].(bool); ok {
			p.config.Enabled = enabled
		}
		if prefix, ok := ctx.Config["trigger_prefix"].(string); ok {
			p.config.TriggerPrefix = prefix
		}
		if story, ok := ctx.Config["default_story"].(string); ok {
			p.config.DefaultStory = story
		}
		if timeout, ok := ctx.Config["session_timeout"].(float64); ok {
			p.config.SessionTimeout = int(timeout)
		}
	}

	// 启动会话清理协程
	go p.cleanupExpiredSessions()

	ctx.Info("[dungeon] 插件初始化完成")
	return nil
}

// OnLoad 插件加载时调用。
func (p *DungeonPlugin) OnLoad(ctx *plugin.PluginContext) error {
	ctx.Debug("[dungeon] 正在加载文字冒险插件")
	p.RegisterHandler(plugin.EventOnHandleContext, p.onHandleContext)
	ctx.Info("[dungeon] 插件加载成功")
	return nil
}

// OnUnload 插件卸载时调用。
func (p *DungeonPlugin) OnUnload(ctx *plugin.PluginContext) error {
	ctx.Debug("[dungeon] 正在卸载文字冒险插件")
	p.mu.Lock()
	p.games = make(map[string]*StoryTeller)
	p.mu.Unlock()
	return p.BasePlugin.OnUnload(ctx)
}

// OnEvent 处理插件事件。
func (p *DungeonPlugin) OnEvent(event plugin.Event, ec *plugin.EventContext) error {
	return p.BasePlugin.OnEvent(event, ec)
}

// onHandleContext 处理消息上下文事件。
func (p *DungeonPlugin) onHandleContext(ec *plugin.EventContext) error {
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

	// 获取会话ID
	sessionID, ok := ec.GetString("session_id")
	if !ok {
		sessionID = "default"
	}

	triggerPrefix := p.config.TriggerPrefix
	if triggerPrefix == "" {
		triggerPrefix = "$"
	}

	// 解析命令
	parts := strings.SplitN(content, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	// 处理停止冒险命令
	if cmd == triggerPrefix+"停止冒险" {
		p.mu.Lock()
		if game, exists := p.games[sessionID]; exists {
			game.Reset()
			delete(p.games, sessionID)
		}
		p.mu.Unlock()

		ec.Set("reply", "冒险结束!")
		ec.BreakPass(p.Name())
		return nil
	}

	// 处理开始冒险命令或继续游戏
	if cmd == triggerPrefix+"开始冒险" {
		story := p.config.DefaultStory
		if arg != "" {
			story = arg
		}

		game := NewStoryTeller(sessionID, story)
		p.mu.Lock()
		p.games[sessionID] = game
		p.mu.Unlock()

		ec.Set("reply", "冒险开始，你可以输入任意内容，让故事继续下去。故事背景是："+story)
		ec.BreakPass(p.Name())
		return nil
	}

	// 检查是否在游戏中
	p.mu.RLock()
	game, inGame := p.games[sessionID]
	p.mu.RUnlock()

	if inGame {
		// 生成提示词并继续游戏
		prompt := game.Action(content)
		ec.Set("content", prompt)
		ec.Set("type", "text")
		ec.Break(p.Name())
	}

	return nil
}

// cleanupExpiredSessions 清理过期会话。
func (p *DungeonPlugin) cleanupExpiredSessions() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// 简单实现：可以扩展为检查每个会话的最后活动时间
		// 目前不自动清理，由用户手动停止
	}
}

// GetHelpText 获取帮助文本。
func (p *DungeonPlugin) GetHelpText(verbose bool) string {
	triggerPrefix := p.config.TriggerPrefix
	if triggerPrefix == "" {
		triggerPrefix = "$"
	}

	helpText := "可以和机器人一起玩文字冒险游戏。"

	if !verbose {
		return helpText
	}

	var sb strings.Builder
	sb.WriteString(helpText + "\n")
	sb.WriteString(fmt.Sprintf("%s开始冒险 [背景故事] - 开始一个基于{背景故事}的文字冒险，之后你的所有消息会协助完善这个故事。\n", triggerPrefix))
	sb.WriteString(fmt.Sprintf("%s停止冒险 - 结束游戏。\n", triggerPrefix))
	sb.WriteString(fmt.Sprintf("\n命令例子: '%s开始冒险 你在树林里冒险，指不定会从哪里蹦出来一些奇怪的东西，你握紧手上的手枪，希望这次冒险能够找到一些值钱的东西，你往树林深处走去。'\n", triggerPrefix))

	return sb.String()
}

// HelpText 返回插件帮助文本。
func (p *DungeonPlugin) HelpText() string {
	return p.GetHelpText(false)
}
