// Package keyword 提供基于关键词的自动回复插件。
// 支持精确匹配、模糊匹配、正则表达式、多条随机回复和规则优先级。
package keyword

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/plugin"
)

// Rule 定义单个关键词匹配规则。
type Rule struct {
	// Keyword 是要匹配消息的模式。
	Keyword string `json:"keyword"`

	// Reply 包含可能的回复消息。如果提供多条回复，
	// 规则匹配时将随机选择一条。
	Reply []string `json:"reply"`

	// IsRegex 指示关键词是否为正则表达式模式。
	IsRegex bool `json:"is_regex"`

	// Priority 决定规则评估顺序。优先级高的规则
	// 先被检查。默认为 0。
	Priority int `json:"priority"`

	// FuzzyMatch 启用子字符串匹配。为 false 时只进行精确匹配。
	// 当 IsRegex 为 true 时忽略此选项。
	FuzzyMatch bool `json:"fuzzy_match"`

	// CaseSensitive 决定匹配是否区分大小写。
	// 默认为 false（不区分大小写）。
	CaseSensitive bool `json:"case_sensitive"`

	// compiledRegex 缓存 IsRegex 规则编译后的正则表达式。
	compiledRegex *regexp.Regexp
}

// Config 表示 keyword 插件的配置。
type Config struct {
	// Rules 是关键词匹配规则列表。
	Rules []Rule `json:"rules"`

	// DefaultReply 是没有规则匹配时的回复（可选）。
	DefaultReply string `json:"default_reply,omitempty"`

	// Enabled 控制插件是否激活。
	Enabled bool `json:"enabled"`
}

// KeywordPlugin 实现基于关键词的自动回复插件。
type KeywordPlugin struct {
	*plugin.BasePlugin

	mu     sync.RWMutex
	config *Config
	rules  []Rule
	rng    *rand.Rand
}

// 确保 KeywordPlugin 实现了 Plugin 接口。
var _ plugin.Plugin = (*KeywordPlugin)(nil)

// New 创建一个新的 KeywordPlugin 实例。
func New() *KeywordPlugin {
	bp := plugin.NewBasePlugin("keyword", "1.0.0")
	bp.SetDescription("Keyword-based auto-reply plugin")
	bp.SetAuthor("simpleclaw")
	bp.SetPriority(900)
	bp.SetHidden(true)

	p := &KeywordPlugin{
		BasePlugin: bp,
		config:     &Config{Enabled: true},
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	return p
}

// Name 返回插件名称。
func (p *KeywordPlugin) Name() string {
	return "keyword"
}

// Version 返回插件版本。
func (p *KeywordPlugin) Version() string {
	return "1.0.0"
}

// OnInit 初始化插件并加载配置。
func (p *KeywordPlugin) OnInit(ctx *plugin.PluginContext) error {
	ctx.Debug("[keyword] 正在初始化关键词插件")

	// 加载配置
	configPath := filepath.Join(ctx.PluginPath, "config.json")
	if err := p.loadConfig(configPath); err != nil {
		ctx.Warn("[keyword] 加载配置失败: " + err.Error())
		// 如果配置文件不存在，创建默认配置
		if os.IsNotExist(err) {
			if createErr := p.createDefaultConfig(configPath); createErr != nil {
				ctx.Warn("[keyword] 创建默认配置失败: " + createErr.Error())
			}
		}
	}

	// 编译正则表达式并按优先级排序规则
	p.prepareRules()

	ctx.Info("[keyword] 插件已初始化，包含 " + string(rune(len(p.rules))) + " 条规则")
	return nil
}

// OnLoad 当插件加载并启用时调用。
func (p *KeywordPlugin) OnLoad(ctx *plugin.PluginContext) error {
	ctx.Debug("[keyword] 正在加载关键词插件")

	// 注册消息处理事件处理器
	p.RegisterHandler(plugin.EventOnHandleContext, p.onHandleContext)

	ctx.Info("[keyword] 插件加载成功")
	return nil
}

// OnUnload 当插件卸载时调用。
func (p *KeywordPlugin) OnUnload(ctx *plugin.PluginContext) error {
	ctx.Debug("[keyword] 正在卸载关键词插件")
	p.mu.Lock()
	p.rules = nil
	p.mu.Unlock()
	return p.BasePlugin.OnUnload(ctx)
}

// OnEvent 通过委托给 BasePlugin 处理插件事件。
func (p *KeywordPlugin) OnEvent(event plugin.Event, ec *plugin.EventContext) error {
	return p.BasePlugin.OnEvent(event, ec)
}

// onHandleContext 处理 ON_HANDLE_CONTEXT 事件。
func (p *KeywordPlugin) onHandleContext(ec *plugin.EventContext) error {
	// 检查插件是否启用
	p.mu.RLock()
	enabled := p.config.Enabled
	p.mu.RUnlock()

	if !enabled {
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

	// 尝试匹配规则
	content = strings.TrimSpace(content)
	if reply, matched := p.Match(content); matched {
		ec.Set("reply", reply)
		ec.BreakPass("keyword")
	}

	return nil
}

// Match 尝试将输入文本与所有规则匹配。
// 如果找到匹配则返回回复文本和 true。
// 规则按优先级顺序评估（最高优先）。
func (p *KeywordPlugin) Match(text string) (string, bool) {
	p.mu.RLock()
	rules := p.rules
	p.mu.RUnlock()

	for _, rule := range rules {
		if matched := p.matchRule(text, rule); matched {
			reply := p.selectReply(rule.Reply)
			return reply, true
		}
	}

	// 如果配置了默认回复，则返回
	p.mu.RLock()
	defaultReply := p.config.DefaultReply
	p.mu.RUnlock()

	if defaultReply != "" {
		return defaultReply, true
	}

	return "", false
}

// matchRule 检查文本是否匹配单个规则。
func (p *KeywordPlugin) matchRule(text string, rule Rule) bool {
	keyword := rule.Keyword
	searchText := text

	// 处理大小写敏感
	if !rule.CaseSensitive {
		keyword = strings.ToLower(keyword)
		searchText = strings.ToLower(text)
	}

	// Regex matching
	if rule.IsRegex {
		if rule.compiledRegex == nil {
			// 编译并缓存正则表达式
			compiled, err := regexp.Compile(rule.Keyword)
			if err != nil {
				return false
			}
			rule.compiledRegex = compiled
		}
		return rule.compiledRegex.MatchString(text)
	}

	// 模糊匹配（子字符串）
	if rule.FuzzyMatch {
		return strings.Contains(searchText, keyword)
	}

	// 精确匹配
	return searchText == keyword
}

// selectReply 从列表中选择回复，如果有多条则随机选择。
func (p *KeywordPlugin) selectReply(replies []string) string {
	if len(replies) == 0 {
		return ""
	}
	if len(replies) == 1 {
		return replies[0]
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	return replies[p.rng.Intn(len(replies))]
}

// loadConfig 从 JSON 文件加载配置。
func (p *KeywordPlugin) loadConfig(path string) error {
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
	p.rules = config.Rules
	p.mu.Unlock()

	return nil
}

// createDefaultConfig 创建默认配置文件。
func (p *KeywordPlugin) createDefaultConfig(path string) error {
	defaultConfig := Config{
		Enabled: true,
		Rules: []Rule{
			{
				Keyword:    "hello",
				Reply:      []string{"Hello! How can I help you?", "Hi there!"},
				Priority:   100,
				FuzzyMatch: false,
			},
			{
				Keyword:    "help",
				Reply:      []string{"I'm here to help! What do you need?"},
				Priority:   50,
				FuzzyMatch: true,
			},
		},
	}

	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// prepareRules 编译正则表达式并按优先级排序规则。
func (p *KeywordPlugin) prepareRules() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 编译正则表达式
	for i := range p.rules {
		if p.rules[i].IsRegex {
			compiled, err := regexp.Compile(p.rules[i].Keyword)
			if err == nil {
				p.rules[i].compiledRegex = compiled
			}
		}
	}

	// 按优先级排序规则（降序）
	sort.Slice(p.rules, func(i, j int) bool {
		return p.rules[i].Priority > p.rules[j].Priority
	})
}

// AddRule 向插件添加新规则。
func (p *KeywordPlugin) AddRule(rule Rule) {
	p.mu.Lock()
	p.rules = append(p.rules, rule)
	p.mu.Unlock()

	// 重新编译和排序
	p.prepareRules()
}

// RemoveRule 根据关键词移除规则。
func (p *KeywordPlugin) RemoveRule(keyword string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var filtered []Rule
	for _, r := range p.rules {
		if r.Keyword != keyword {
			filtered = append(filtered, r)
		}
	}
	p.rules = filtered
}

// GetRules 返回所有规则的副本。
func (p *KeywordPlugin) GetRules() []Rule {
	p.mu.RLock()
	defer p.mu.RUnlock()

	rules := make([]Rule, len(p.rules))
	copy(rules, p.rules)
	return rules
}

// SetEnabled 启用或禁用插件。
func (p *KeywordPlugin) SetEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.Enabled = enabled
}

// IsEnabled 返回插件是否启用。
func (p *KeywordPlugin) IsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.Enabled
}

// SetDefaultReply 设置未匹配消息的默认回复。
func (p *KeywordPlugin) SetDefaultReply(reply string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.DefaultReply = reply
}
