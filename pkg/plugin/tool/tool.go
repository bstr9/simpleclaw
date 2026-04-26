// Package tool 提供工具调用插件，支持多种工具来增强 AI 机器人的能力。
// 该文件包含 ToolPlugin 核心结构、配置管理和生命周期方法。
package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/llm"
	"github.com/bstr9/simpleclaw/pkg/plugin"
)

// 工具常量
const (
	ToolURLGet    = "url-get"
	BingSearchURL = "https://api.bing.microsoft.com/v7.0/search"
)

// ToolConfig 表示单个工具的配置。
type ToolConfig struct {
	// Enabled 是否启用该工具。
	Enabled bool `json:"enabled"`

	// Config 工具特定的配置。
	Config map[string]any `json:"config,omitempty"`
}

// Config 表示 tool 插件的配置。
type Config struct {
	// Tools 要加载的工具列表。
	Tools []string `json:"tools"`

	// ToolConfigs 工具配置映射。
	ToolConfigs map[string]ToolConfig `json:"tool_configs,omitempty"`

	// Kwargs 全局参数配置。
	Kwargs map[string]any `json:"kwargs"`

	// TriggerPrefix 触发前缀，默认为 "$"。
	TriggerPrefix string `json:"trigger_prefix"`

	// Debug 是否开启调试模式。
	Debug bool `json:"debug"`

	// NoDefault 是否不加载默认工具。
	NoDefault bool `json:"no_default"`

	// ThinkDepth 一个问题最多使用多少次工具。
	ThinkDepth int `json:"think_depth"`

	// RequestTimeout 请求超时时间（秒）。
	RequestTimeout int `json:"request_timeout"`

	// ModelName 使用的模型名称。
	ModelName string `json:"model_name"`

	// Temperature LLM 温度参数。
	Temperature float64 `json:"temperature"`

	// LLM API 配置
	LLMAPIKey  string `json:"llm_api_key"`
	LLMAPIBase string `json:"llm_api_base"`

	// 搜索引擎配置
	BingSubscriptionKey string `json:"bing_subscription_key"`
	BingSearchURL       string `json:"bing_search_url"`
	GoogleAPIKey        string `json:"google_api_key"`
	GoogleCSEID         string `json:"google_cse_id"`
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		Tools:          []string{ToolURLGet, "meteo"},
		ToolConfigs:    make(map[string]ToolConfig),
		Kwargs:         make(map[string]any),
		TriggerPrefix:  "$",
		Debug:          false,
		NoDefault:      false,
		ThinkDepth:     2,
		RequestTimeout: 120,
		ModelName:      "gpt-3.5-turbo",
		Temperature:    0,
		BingSearchURL:  BingSearchURL,
	}
}

// ToolPlugin 实现工具调用插件。
type ToolPlugin struct {
	*plugin.BasePlugin

	mu           sync.RWMutex
	config       *Config
	tools        map[string]Tool
	toolRegistry map[string]Tool // 全局工具注册表
	llmModel     llm.Model       // LLM 模型实例
}

// 确保 ToolPlugin 实现了 Plugin 接口。
var _ plugin.Plugin = (*ToolPlugin)(nil)

// New 创建一个新的 ToolPlugin 实例。
func New() *ToolPlugin {
	bp := plugin.NewBasePlugin("tool", "0.5.0")
	bp.SetDescription("为 AI 机器人提供各种工具调用能力，如联网搜索、数字运算等")
	bp.SetAuthor("goldfishh")
	bp.SetPriority(0)

	p := &ToolPlugin{
		BasePlugin:   bp,
		config:       DefaultConfig(),
		tools:        make(map[string]Tool),
		toolRegistry: globalRegistry.tools,
	}
	return p
}

// Name 返回插件名称。
func (p *ToolPlugin) Name() string {
	return "tool"
}

// Version 返回插件版本。
func (p *ToolPlugin) Version() string {
	return "0.5.0"
}

// OnInit 初始化插件并加载配置。
func (p *ToolPlugin) OnInit(ctx *plugin.PluginContext) error {
	ctx.Debug("[tool] 正在初始化 tool 插件")

	// 加载配置文件
	configPath := filepath.Join(ctx.PluginPath, "config.json")
	if err := p.loadConfig(configPath); err != nil {
		ctx.Warn("[tool] 加载配置失败: " + err.Error())
		// 如果配置文件不存在，创建默认配置
		if os.IsNotExist(err) {
			if createErr := p.createDefaultConfig(configPath); createErr != nil {
				ctx.Warn("[tool] 创建默认配置失败: " + createErr.Error())
			}
		}
	}

	// 初始化 LLM 模型
	if err := p.initLLMModel(); err != nil {
		ctx.Warn("[tool] 初始化 LLM 模型失败: " + err.Error())
	}

	// 加载工具
	if err := p.loadTools(); err != nil {
		ctx.Warn("[tool] 加载工具失败: " + err.Error())
	}

	// 检查是否有工具加载
	if len(p.config.Tools) == 0 {
		ctx.Warn("[tool] 未配置任何工具，插件初始化失败")
		return fmt.Errorf("config.json 未找到或未配置工具")
	}

	ctx.Info("[tool] 插件初始化完成，已加载工具: " + strings.Join(p.getLoadedToolNames(), ", "))
	return nil
}

// initLLMModel 初始化 LLM 模型。
func (p *ToolPlugin) initLLMModel() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果没有配置 API Key，则不初始化 LLM
	if p.config.LLMAPIKey == "" {
		return fmt.Errorf("LLM API Key 未配置")
	}

	modelConfig := llm.ModelConfig{
		ModelName: p.config.ModelName,
		Model:     p.config.ModelName,
		APIKey:    p.config.LLMAPIKey,
		APIBase:   p.config.LLMAPIBase,
	}

	if modelConfig.APIBase == "" {
		modelConfig.APIBase = "https://api.openai.com/v1"
	}

	model, err := llm.NewModel(modelConfig)
	if err != nil {
		return err
	}

	p.llmModel = model
	return nil
}

// OnLoad 插件加载时调用，注册事件处理器。
func (p *ToolPlugin) OnLoad(ctx *plugin.PluginContext) error {
	ctx.Debug("[tool] 正在加载 tool 插件")

	// 注册消息处理事件
	p.RegisterHandler(plugin.EventOnHandleContext, p.onHandleContext)

	ctx.Info("[tool] 插件加载成功")
	return nil
}

// OnUnload 插件卸载时调用，清理资源。
func (p *ToolPlugin) OnUnload(ctx *plugin.PluginContext) error {
	ctx.Debug("[tool] 正在卸载 tool 插件")

	p.mu.Lock()
	p.tools = make(map[string]Tool)
	p.llmModel = nil
	p.mu.Unlock()

	return p.BasePlugin.OnUnload(ctx)
}

// OnEvent 处理插件事件。
func (p *ToolPlugin) OnEvent(event plugin.Event, ec *plugin.EventContext) error {
	return p.BasePlugin.OnEvent(event, ec)
}

// onHandleContext 处理消息上下文事件。
func (p *ToolPlugin) onHandleContext(ec *plugin.EventContext) error {
	content, ok := ec.GetString("content")
	if !ok {
		return nil
	}

	if !p.shouldHandleMessage(ec, content) {
		return nil
	}

	contentList := strings.SplitN(content, " ", 3)

	if len(contentList) == 1 {
		return p.replyHelp(ec)
	}

	subCmd := strings.TrimSpace(contentList[1])

	if handled := p.handleResetCommand(ec, subCmd); handled {
		return nil
	}

	toolName, query := p.parseToolQuery(subCmd, contentList)

	result, err := p.executeTool(toolName, query)
	if err != nil {
		ec.Set("reply", fmt.Sprintf("工具执行出错: %s", err.Error()))
		ec.BreakPass(p.Name())
		return nil
	}

	ec.Set("reply", result)
	ec.BreakPass(p.Name())
	return nil
}

// shouldHandleMessage 判断是否应该处理该消息。
func (p *ToolPlugin) shouldHandleMessage(ec *plugin.EventContext, content string) bool {
	msgType, ok := ec.GetString("type")
	if ok && msgType != "text" && msgType != "" {
		return false
	}

	triggerPrefix := p.getTriggerPrefix()
	return strings.HasPrefix(content, triggerPrefix+"tool")
}

// replyHelp 显示帮助信息。
func (p *ToolPlugin) replyHelp(ec *plugin.EventContext) error {
	ec.Set("reply", p.getHelpText(true))
	ec.BreakPass(p.Name())
	return nil
}

// handleResetCommand 处理重置命令，返回是否已处理。
func (p *ToolPlugin) handleResetCommand(ec *plugin.EventContext, subCmd string) bool {
	if subCmd == "reset" {
		p.resetTools()
		ec.Set("reply", "重置工具成功")
		ec.BreakPass(p.Name())
		return true
	}

	if strings.HasPrefix(subCmd, "reset") {
		ec.Set("reply", "如果想重置 tool 插件，reset 之后不要加任何字符")
		ec.BreakPass(p.Name())
		return true
	}

	return false
}

// parseToolQuery 解析工具名和查询内容。
func (p *ToolPlugin) parseToolQuery(subCmd string, contentList []string) (toolName, query string) {
	query = subCmd

	for _, name := range p.getLoadedToolNames() {
		if strings.HasPrefix(subCmd, name) {
			toolName = name
			query = strings.TrimSpace(strings.TrimPrefix(subCmd, name))
			break
		}
	}

	if len(contentList) > 2 {
		if query != "" {
			query = query + " " + contentList[2]
		} else {
			query = contentList[2]
		}
	}

	return toolName, query
}

// getTriggerPrefix 获取触发前缀。
func (p *ToolPlugin) getTriggerPrefix() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.config.TriggerPrefix == "" {
		return "$"
	}
	return p.config.TriggerPrefix
}

// getHelpText 获取帮助文本。
func (p *ToolPlugin) getHelpText(verbose bool) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	helpText := "这是一个能让 AI 机器人联网、搜索、数字运算的插件，赋予强大的扩展能力。"
	triggerPrefix := "$"
	if p.config.TriggerPrefix != "" {
		triggerPrefix = p.config.TriggerPrefix
	}

	if !verbose {
		return helpText
	}

	toolNames := p.getLoadedToolNames()
	toolsStr := "未加载任何工具"
	if len(toolNames) > 0 {
		toolsStr = strings.Join(toolNames, ", ")
	}

	var sb strings.Builder
	sb.WriteString(helpText)
	sb.WriteString("\n\n使用说明：\n")
	sb.WriteString(fmt.Sprintf("%stool [命令] - 根据命令选择使用哪些工具处理请求\n", triggerPrefix))
	sb.WriteString(fmt.Sprintf("%stool [工具名] [命令] - 使用指定工具处理请求\n", triggerPrefix))
	sb.WriteString(fmt.Sprintf("%stool reset - 重置工具\n\n", triggerPrefix))
	sb.WriteString(fmt.Sprintf("已加载工具列表: \n%s\n", toolsStr))

	return sb.String()
}

// getLoadedToolNames 获取已加载的工具名称列表。
func (p *ToolPlugin) getLoadedToolNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	names := make([]string, 0, len(p.tools))
	for name := range p.tools {
		names = append(names, name)
	}
	return names
}

// loadTools 加载配置的工具。
func (p *ToolPlugin) loadTools() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.tools = make(map[string]Tool)

	for _, toolName := range p.config.Tools {
		tool, ok := globalRegistry.GetTool(toolName)
		if !ok {
			continue
		}
		p.tools[toolName] = tool
	}

	return nil
}

// resetTools 重置工具状态。
func (p *ToolPlugin) resetTools() {
	p.loadTools()
}

// loadConfig 加载配置文件。
func (p *ToolPlugin) loadConfig(path string) error {
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

// createDefaultConfig 创建默认配置文件。
func (p *ToolPlugin) createDefaultConfig(path string) error {
	defaultConfig := Config{
		Tools: []string{
			ToolURLGet,
			"meteo",
		},
		ToolConfigs:   make(map[string]ToolConfig),
		Kwargs:        map[string]any{},
		TriggerPrefix: "$",
		Debug:         false,
		NoDefault:     false,
		ThinkDepth:    2,
		ModelName:     "gpt-3.5-turbo",
		BingSearchURL: BingSearchURL,
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

// HelpText 返回插件帮助文本。
func (p *ToolPlugin) HelpText() string {
	return p.getHelpText(false)
}
