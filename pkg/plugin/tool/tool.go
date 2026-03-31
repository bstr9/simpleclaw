// Package tool 提供工具调用插件，支持多种工具来增强 AI 机器人的能力。
// 该插件可以让机器人联网搜索、进行数字运算、访问 URL 等多种扩展功能。
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/llm"
	"github.com/bstr9/simpleclaw/pkg/plugin"
)

// 工具常量
const (
	ToolURLGet    = "url-get"
	BingSearchURL = "https://api.bing.microsoft.com/v7.0/search"

	// 重复字符串提取 (SonarQube go:S1192)
	errMsgNoToolHandle   = "暂无工具能够处理该请求"
	errMsgAllToolsFailed = "所有工具都无法处理该请求: %w"
)

// Tool 定义工具接口，所有工具必须实现此接口。
type Tool interface {
	// Name 返回工具的唯一标识名称。
	Name() string

	// Description 返回工具的描述信息。
	Description() string

	// Run 执行工具并返回结果。
	Run(query string, config map[string]any) (string, error)
}

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

// 全局工具注册表实例。
var globalRegistry = &toolRegistry{
	tools: make(map[string]Tool),
	mu:    sync.RWMutex{},
}

// toolRegistry 工具注册表。
type toolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// RegisterTool 注册工具到全局注册表。
func RegisterTool(t Tool) {
	globalRegistry.Register(t)
}

// Register 注册工具。
func (r *toolRegistry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// GetTool 从注册表获取工具。
func (r *toolRegistry) GetTool(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

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

// executeTool 执行工具调用。
func (p *ToolPlugin) executeTool(toolName, query string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 如果指定了特定工具
	if toolName != "" {
		tool, ok := p.tools[toolName]
		if !ok {
			return "", fmt.Errorf("工具 '%s' 未找到", toolName)
		}

		toolConfig := p.getToolConfig(toolName)
		return tool.Run(query, toolConfig)
	}

	// 使用 LLM 智能选择工具
	if p.llmModel != nil {
		return p.selectToolWithLLM(query)
	}

	// 如果没有 LLM，按顺序尝试每个工具
	var lastErr error
	for _, name := range p.getLoadedToolNames() {
		tool, ok := p.tools[name]
		if !ok {
			continue
		}

		toolConfig := p.getToolConfig(name)
		result, err := tool.Run(query, toolConfig)
		if err != nil {
			lastErr = err
			continue
		}

		if result != "" {
			return result, nil
		}
	}

	if lastErr != nil {
		return "", fmt.Errorf(errMsgAllToolsFailed, lastErr)
	}

	return errMsgNoToolHandle, nil
}

// selectToolWithLLM 使用 LLM 智能选择工具。
func (p *ToolPlugin) selectToolWithLLM(query string) (string, error) {
	toolNames := p.getLoadedToolNames()
	toolDescriptions := p.buildToolDescriptions(toolNames)

	systemPrompt := p.buildSystemPrompt(toolDescriptions)

	selectedTool, err := p.callLLMForToolSelection(query, systemPrompt)
	if err != nil {
		return "", err
	}

	if result, ok := p.tryToolByName(toolNames, selectedTool, query); ok {
		return result, nil
	}

	return p.tryAllTools(toolNames, query)
}

// buildToolDescriptions 构建工具描述列表。
func (p *ToolPlugin) buildToolDescriptions(toolNames []string) []string {
	descriptions := make([]string, 0, len(toolNames))
	for _, name := range toolNames {
		if tool, ok := p.tools[name]; ok {
			descriptions = append(descriptions, fmt.Sprintf("- %s: %s", name, tool.Description()))
		}
	}
	return descriptions
}

// buildSystemPrompt 构建系统提示词。
func (p *ToolPlugin) buildSystemPrompt(toolDescriptions []string) string {
	return `你是一个工具选择助手。根据用户的查询，选择最合适的工具来处理请求。
可用的工具有：
` + strings.Join(toolDescriptions, "\n") + `

请直接回复工具名称，不要包含其他内容。如果不确定使用哪个工具，回复 "unknown"。`
}

// callLLMForToolSelection 调用 LLM 获取工具选择。
func (p *ToolPlugin) callLLMForToolSelection(query, systemPrompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: query},
	}

	opts := []llm.Option{
		llm.WithTemperature(0),
		llm.WithMaxTokens(50),
	}

	resp, err := p.llmModel.Call(ctx, messages, opts...)
	if err != nil {
		return "", fmt.Errorf("LLM 调用失败: %w", err)
	}

	selectedTool := strings.TrimSpace(strings.ToLower(resp.Content))
	return strings.Trim(selectedTool, "\"'`"), nil
}

// tryToolByName 按名称尝试指定工具。
func (p *ToolPlugin) tryToolByName(toolNames []string, selectedTool, query string) (string, bool) {
	for _, name := range toolNames {
		if strings.ToLower(name) != selectedTool {
			continue
		}

		tool, ok := p.tools[name]
		if !ok {
			continue
		}

		toolConfig := p.getToolConfig(name)
		result, err := tool.Run(query, toolConfig)
		if err != nil {
			return "", false
		}
		return result, true
	}
	return "", false
}

// tryAllTools 按顺序尝试所有工具。
func (p *ToolPlugin) tryAllTools(toolNames []string, query string) (string, error) {
	var lastErr error
	for _, name := range toolNames {
		tool, ok := p.tools[name]
		if !ok {
			continue
		}

		toolConfig := p.getToolConfig(name)
		result, err := tool.Run(query, toolConfig)
		if err != nil {
			lastErr = err
			continue
		}

		if result != "" {
			return result, nil
		}
	}

	if lastErr != nil {
		return "", fmt.Errorf(errMsgAllToolsFailed, lastErr)
	}

	return errMsgNoToolHandle, nil
}

// getToolConfig 获取工具配置。
func (p *ToolPlugin) getToolConfig(toolName string) map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 合并全局配置和工具特定配置
	config := make(map[string]any)

	// 添加全局 kwargs
	for k, v := range p.config.Kwargs {
		config[k] = v
	}

	// 添加全局配置
	config["llm_api_key"] = p.config.LLMAPIKey
	config["llm_api_base"] = p.config.LLMAPIBase
	config["bing_subscription_key"] = p.config.BingSubscriptionKey
	config["bing_search_url"] = p.config.BingSearchURL
	config["google_api_key"] = p.config.GoogleAPIKey
	config["google_cse_id"] = p.config.GoogleCSEID
	config["request_timeout"] = p.config.RequestTimeout
	config["debug"] = p.config.Debug

	// 添加工具特定配置
	if tc, ok := p.config.ToolConfigs[toolName]; ok {
		for k, v := range tc.Config {
			config[k] = v
		}
	}

	return config
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

// ===== 内置工具实现 =====

// URLGetTool URL 获取工具。
type URLGetTool struct {
	client *http.Client
}

// Name 返回工具名称。
func (t *URLGetTool) Name() string {
	return ToolURLGet
}

// Description 返回工具描述。
func (t *URLGetTool) Description() string {
	return "获取 URL 内容，用于访问网页并提取文本内容"
}

// Run 执行工具。
func (t *URLGetTool) Run(query string, config map[string]any) (string, error) {
	// 初始化 HTTP 客户端
	if t.client == nil {
		timeout := 60
		if reqTimeout, ok := config["request_timeout"].(float64); ok {
			timeout = int(reqTimeout)
		} else if reqTimeout, ok := config["request_timeout"].(int); ok {
			timeout = reqTimeout
		}
		t.client = &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		}
	}

	// 提取 URL
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("请提供要获取的 URL")
	}

	// 验证 URL
	parsedURL, err := url.Parse(query)
	if err != nil {
		return "", fmt.Errorf("无效的 URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("仅支持 HTTP 和 HTTPS 协议")
	}

	// 创建请求
	req, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头，模拟浏览器
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	// 发送请求
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP 错误: %d %s", resp.StatusCode, resp.Status)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 提取文本内容（简单实现，移除 HTML 标签）
	content := string(body)
	content = t.extractText(content)

	// 限制返回内容长度
	maxLength := 4000
	if len(content) > maxLength {
		content = content[:maxLength] + "\n... (内容已截断)"
	}

	return fmt.Sprintf("URL: %s\n\n%s", query, content), nil
}

// extractText 从 HTML 中提取文本内容。
func (t *URLGetTool) extractText(html string) string {
	// 移除 script 和 style 标签及其内容
	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	styleRegex := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	html = scriptRegex.ReplaceAllString(html, "")
	html = styleRegex.ReplaceAllString(html, "")

	// 移除所有 HTML 标签
	tagRegex := regexp.MustCompile(`<[^>]+>`)
	text := tagRegex.ReplaceAllString(html, "")

	// 解码 HTML 实体
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// 清理多余空白
	whitespaceRegex := regexp.MustCompile(`\s+`)
	text = whitespaceRegex.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	return text
}

// MeteoTool 天气工具，使用 Open-Meteo API。
type MeteoTool struct {
	client *http.Client
}

// Name 返回工具名称。
func (t *MeteoTool) Name() string {
	return "meteo"
}

// Description 返回工具描述。
func (t *MeteoTool) Description() string {
	return "查询天气信息，支持查询任意城市的当前天气和天气预报"
}

// Run 执行工具。
func (t *MeteoTool) Run(query string, config map[string]any) (string, error) {
	// 初始化 HTTP 客户端
	if t.client == nil {
		timeout := 30
		if reqTimeout, ok := config["request_timeout"].(float64); ok {
			timeout = int(reqTimeout)
		} else if reqTimeout, ok := config["request_timeout"].(int); ok {
			timeout = reqTimeout
		}
		t.client = &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		}
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("请提供要查询的城市名称")
	}

	// 解析城市名称（支持中文格式）
	city := t.extractCity(query)

	// 步骤1：获取城市的经纬度
	lat, lon, locationName, err := t.getGeocoding(city)
	if err != nil {
		return "", fmt.Errorf("获取城市坐标失败: %w", err)
	}

	// 步骤2：获取天气数据
	weatherData, err := t.getWeather(lat, lon)
	if err != nil {
		return "", fmt.Errorf("获取天气数据失败: %w", err)
	}

	// 格式化输出
	return t.formatWeatherResponse(locationName, weatherData), nil
}

// extractCity 从查询中提取城市名称。
func (t *MeteoTool) extractCity(query string) string {
	// 移除常见的查询词
	replacements := []string{
		"今天", "明天", "后天", "大后天",
		"的天气", "天气", "天气预报",
		"怎么样", "如何", "怎样",
		"查询", "查一下", "看看",
		"？", "?",
	}

	city := query
	for _, r := range replacements {
		city = strings.ReplaceAll(city, r, "")
	}

	return strings.TrimSpace(city)
}

// getGeocoding 获取城市的经纬度。
func (t *MeteoTool) getGeocoding(city string) (lat, lon float64, name string, err error) {
	// 使用 Open-Meteo Geocoding API
	apiURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=zh&format=json",
		url.QueryEscape(city))

	resp, err := t.client.Get(apiURL)
	if err != nil {
		return 0, 0, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, "", err
	}

	// 解析响应
	var geocodingResp struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Country   string  `json:"country"`
			Admin1    string  `json:"admin1"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &geocodingResp); err != nil {
		return 0, 0, "", err
	}

	if len(geocodingResp.Results) == 0 {
		return 0, 0, "", fmt.Errorf("未找到城市: %s", city)
	}

	result := geocodingResp.Results[0]
	locationName := result.Name
	if result.Admin1 != "" {
		locationName = result.Admin1 + ", " + result.Name
	}
	if result.Country != "" {
		locationName = result.Name + ", " + result.Country
	}

	return result.Latitude, result.Longitude, locationName, nil
}

// getWeather 获取天气数据。
func (t *MeteoTool) getWeather(lat, lon float64) (map[string]any, error) {
	// 使用 Open-Meteo Weather API
	apiURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m,wind_direction_10m&daily=weather_code,temperature_2m_max,temperature_2m_min&timezone=auto&forecast_days=3",
		lat, lon)

	resp, err := t.client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var weatherData map[string]any
	if err := json.Unmarshal(body, &weatherData); err != nil {
		return nil, err
	}

	return weatherData, nil
}

// formatWeatherResponse 格式化天气响应。
func (t *MeteoTool) formatWeatherResponse(locationName string, data map[string]any) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📍 地点: %s\n\n", locationName))

	t.formatCurrentWeather(data, &sb)
	t.formatDailyForecast(data, &sb)

	return sb.String()
}

// formatCurrentWeather 格式化当前天气
func (t *MeteoTool) formatCurrentWeather(data map[string]any, sb *strings.Builder) {
	current, ok := data["current"].(map[string]any)
	if !ok {
		return
	}

	sb.WriteString("🌡️ 当前天气:\n")
	if temp, ok := current["temperature_2m"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  温度: %.1f°C\n", temp))
	}
	if apparent, ok := current["apparent_temperature"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  体感温度: %.1f°C\n", apparent))
	}
	if humidity, ok := current["relative_humidity_2m"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  湿度: %.0f%%\n", humidity))
	}
	if windSpeed, ok := current["wind_speed_10m"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  风速: %.1f km/h\n", windSpeed))
	}
	if weatherCode, ok := current["weather_code"].(float64); ok {
		sb.WriteString(fmt.Sprintf("  天气: %s\n", t.getWeatherDescription(int(weatherCode))))
	}
}

// formatDailyForecast 格式化每日天气预报
func (t *MeteoTool) formatDailyForecast(data map[string]any, sb *strings.Builder) {
	daily, ok := data["daily"].(map[string]any)
	if !ok {
		return
	}

	times, ok := daily["time"].([]any)
	if !ok {
		return
	}

	sb.WriteString("\n📅 未来天气预报:\n")
	maxTemps, _ := daily["temperature_2m_max"].([]any)
	minTemps, _ := daily["temperature_2m_min"].([]any)
	weatherCodes, _ := daily["weather_code"].([]any)

	for i, time := range times {
		if i >= 3 {
			break
		}
		t.formatDailyEntry(i, time, maxTemps, minTemps, weatherCodes, sb)
	}
}

// formatDailyEntry 格式化单日天气预报
func (t *MeteoTool) formatDailyEntry(i int, time any, maxTemps, minTemps, weatherCodes []any, sb *strings.Builder) {
	date := time.(string)
	maxTemp := t.getArrayValue(maxTemps, i)
	minTemp := t.getArrayValue(minTemps, i)
	weather := t.getWeatherDescFromArray(weatherCodes, i)
	sb.WriteString(fmt.Sprintf("  %s: %s ~ %s°C, %s\n", date, minTemp, maxTemp, weather))
}

// getArrayValue 安全获取数组元素并格式化
func (t *MeteoTool) getArrayValue(arr []any, idx int) string {
	if idx >= len(arr) {
		return ""
	}
	if val, ok := arr[idx].(float64); ok {
		return fmt.Sprintf("%.1f", val)
	}
	return ""
}

// getWeatherDescFromArray 从数组获取天气描述
func (t *MeteoTool) getWeatherDescFromArray(arr []any, idx int) string {
	if idx >= len(arr) {
		return ""
	}
	if val, ok := arr[idx].(float64); ok {
		return t.getWeatherDescription(int(val))
	}
	return ""
}

// getWeatherDescription 根据天气代码返回描述。
func (t *MeteoTool) getWeatherDescription(code int) string {
	weatherCodes := map[int]string{
		0: "晴朗",
		1: "大部晴朗", 2: "多云", 3: "阴天",
		45: "雾", 48: "雾凇",
		51: "小毛毛雨", 53: "中毛毛雨", 55: "大毛毛雨",
		56: "冻毛毛雨", 57: "冻毛毛雨",
		61: "小雨", 63: "中雨", 65: "大雨",
		66: "冻雨", 67: "冻雨",
		71: "小雪", 73: "中雪", 75: "大雪",
		77: "雪粒",
		80: "小阵雨", 81: "中阵雨", 82: "大阵雨",
		85: "小阵雪", 86: "大阵雪",
		95: "雷暴",
		96: "雷暴伴小冰雹", 99: "雷暴伴大冰雹",
	}

	if desc, ok := weatherCodes[code]; ok {
		return desc
	}
	return fmt.Sprintf("天气代码 %d", code)
}

// CalculatorTool 计算器工具。
type CalculatorTool struct{}

// Name 返回工具名称。
func (t *CalculatorTool) Name() string {
	return "calculator"
}

// Description 返回工具描述。
func (t *CalculatorTool) Description() string {
	return "执行数学计算，支持加减乘除、幂运算、括号和数学函数"
}

// Run 执行工具。
func (t *CalculatorTool) Run(query string, config map[string]any) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("请提供要计算的表达式")
	}

	// 清理表达式
	expr := t.cleanExpression(query)

	// 解析并计算表达式
	result, err := t.evaluate(expr)
	if err != nil {
		return "", fmt.Errorf("计算错误: %w", err)
	}

	return fmt.Sprintf("计算结果: %s = %v", query, result), nil
}

// cleanExpression 清理表达式。
func (t *CalculatorTool) cleanExpression(expr string) string {
	// 移除中文运算符和常见描述
	replacements := map[string]string{
		"加": "+", "减": "-", "乘": "*", "除": "/",
		"等于": "=", "是多少": "", "计算": "",
		"请问": "", "帮我": "", "算一下": "",
		"多少": "", "？": "", "?": "",
		"×": "*", "÷": "/", "－": "-", "＋": "+",
		"（": "(", "）": ")",
	}

	result := expr
	for old, newStr := range replacements {
		result = strings.ReplaceAll(result, old, newStr)
	}

	// 移除所有空白字符
	result = strings.ReplaceAll(result, " ", "")
	result = strings.ReplaceAll(result, "\t", "")
	result = strings.ReplaceAll(result, "\n", "")

	return result
}

// evaluate 计算表达式。
func (t *CalculatorTool) evaluate(expr string) (float64, error) {
	// 使用简单的表达式解析器
	// 支持加减乘除、括号和幂运算

	// 预处理：将 ^ 替换为幂运算标记
	expr = strings.ReplaceAll(expr, "^", "**")

	// 解析并计算
	return t.parseExpression(expr)
}

// parseExpression 解析表达式（递归下降解析器）。
func (t *CalculatorTool) parseExpression(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, fmt.Errorf("空表达式")
	}

	if t.hasMathFunction(expr) {
		return t.evaluateWithFunctions(expr)
	}

	if num, err := strconv.ParseFloat(expr, 64); err == nil {
		return num, nil
	}

	expr, err := t.evaluateParentheses(expr)
	if err != nil {
		return 0, err
	}

	if result, ok, err := t.tryEvaluatePower(expr); ok {
		return result, err
	}

	if result, ok, err := t.tryEvaluateAddSub(expr); ok {
		return result, err
	}

	if result, ok, err := t.tryEvaluateMulDiv(expr); ok {
		return result, err
	}

	return strconv.ParseFloat(expr, 64)
}

// hasMathFunction 检查表达式是否包含数学函数。
func (t *CalculatorTool) hasMathFunction(expr string) bool {
	functions := []string{"sin(", "cos(", "tan(", "sqrt(", "log(", "ln(", "abs("}
	for _, f := range functions {
		if strings.Contains(expr, f) {
			return true
		}
	}
	return false
}

// evaluateParentheses 处理括号表达式，返回简化后的表达式。
func (t *CalculatorTool) evaluateParentheses(expr string) (string, error) {
	for strings.Contains(expr, "(") {
		start := strings.LastIndex(expr, "(")
		if start == -1 {
			break
		}
		end := strings.Index(expr[start:], ")")
		if end == -1 {
			return "", fmt.Errorf("括号不匹配")
		}
		end += start

		inner := expr[start+1 : end]
		result, err := t.parseExpression(inner)
		if err != nil {
			return "", err
		}

		expr = expr[:start] + fmt.Sprintf("%g", result) + expr[end+1:]
	}
	return expr, nil
}

// tryEvaluatePower 尝试处理幂运算。
func (t *CalculatorTool) tryEvaluatePower(expr string) (float64, bool, error) {
	if strings.Index(expr, "**") <= 0 {
		return 0, false, nil
	}

	parts := strings.Split(expr, "**")
	if len(parts) < 2 {
		return 0, false, nil
	}

	result, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil {
		return 0, true, fmt.Errorf("无效的操作数: %s", parts[len(parts)-1])
	}

	for i := len(parts) - 2; i >= 0; i-- {
		base, err := strconv.ParseFloat(parts[i], 64)
		if err != nil {
			return 0, true, fmt.Errorf("无效的操作数: %s", parts[i])
		}
		result = t.pow(base, result)
	}
	return result, true, nil
}

// tryEvaluateAddSub 尝试处理加法和减法。
func (t *CalculatorTool) tryEvaluateAddSub(expr string) (float64, bool, error) {
	for i := len(expr) - 1; i >= 0; i-- {
		c := expr[i]
		if !t.isAddSubOperator(expr, i) {
			continue
		}

		if result, ok, err := t.evaluateBinaryOp(c, expr[:i], expr[i+1:]); ok {
			return result, true, err
		}
	}
	return 0, false, nil
}

// isAddSubOperator 检查位置 i 是否为有效的加减运算符。
func (t *CalculatorTool) isAddSubOperator(expr string, i int) bool {
	c := expr[i]
	if (c != '+' && c != '-') || i == 0 {
		return false
	}
	return !t.isNegativeSign(expr, i)
}

// evaluateBinaryOp 计算二元运算。
func (t *CalculatorTool) evaluateBinaryOp(op byte, left, right string) (float64, bool, error) {
	leftVal, err := t.parseExpression(left)
	if err != nil {
		return 0, false, nil
	}

	rightVal, err := t.parseExpression(right)
	if err != nil {
		return 0, true, err
	}

	if op == '+' {
		return leftVal + rightVal, true, nil
	}
	if op == '-' {
		return leftVal - rightVal, true, nil
	}
	if op == '*' {
		return leftVal * rightVal, true, nil
	}
	if op == '/' {
		if rightVal == 0 {
			return 0, true, fmt.Errorf("除数不能为零")
		}
		return leftVal / rightVal, true, nil
	}
	return 0, false, nil
}

// isNegativeSign 判断减号是否为负号。
func (t *CalculatorTool) isNegativeSign(expr string, idx int) bool {
	if idx == 0 {
		return true
	}
	prev := expr[idx-1]
	return prev == '(' || prev == '+' || prev == '-' || prev == '*' || prev == '/'
}

// tryEvaluateMulDiv 尝试处理乘法和除法。
func (t *CalculatorTool) tryEvaluateMulDiv(expr string) (float64, bool, error) {
	for i := len(expr) - 1; i >= 0; i-- {
		c := expr[i]
		if c != '*' && c != '/' {
			continue
		}

		if result, ok, err := t.evaluateBinaryOp(c, expr[:i], expr[i+1:]); ok {
			return result, true, err
		}
	}
	return 0, false, nil
}

// evaluateWithFunctions 计算包含数学函数的表达式。
func (t *CalculatorTool) evaluateWithFunctions(expr string) (float64, error) {
	functions := t.getMathFunctions()

	for _, f := range functions {
		if !strings.Contains(expr, f.name) {
			continue
		}

		start := strings.Index(expr, f.name)
		if start == -1 {
			continue
		}

		end, err := t.findFunctionEnd(expr, start, f.name)
		if err != nil {
			return 0, err
		}

		arg := expr[start+len(f.name) : end]
		argVal, err := t.parseExpression(arg)
		if err != nil {
			return 0, err
		}

		result := f.fn(argVal)
		expr = expr[:start] + fmt.Sprintf("%g", result) + expr[end+1:]
		return t.parseExpression(expr)
	}

	return 0, fmt.Errorf("未知的数学函数")
}

// getMathFunctions 返回支持的数学函数列表。
func (t *CalculatorTool) getMathFunctions() []struct {
	name string
	fn   func(float64) float64
} {
	return []struct {
		name string
		fn   func(float64) float64
	}{
		{name: "sin(", fn: func(x float64) float64 { return t.sin(x * 3.14159265359 / 180) }},
		{name: "cos(", fn: func(x float64) float64 { return t.cos(x * 3.14159265359 / 180) }},
		{name: "tan(", fn: func(x float64) float64 { return t.tan(x * 3.14159265359 / 180) }},
		{name: "sqrt(", fn: func(x float64) float64 { return t.sqrt(x) }},
		{name: "log(", fn: func(x float64) float64 { return t.log10(x) }},
		{name: "ln(", fn: func(x float64) float64 { return t.ln(x) }},
		{name: "abs(", fn: func(x float64) float64 {
			if x < 0 {
				return -x
			}
			return x
		}},
	}
}

// findFunctionEnd 找到函数的结束括号位置。
func (t *CalculatorTool) findFunctionEnd(expr string, start int, funcName string) (int, error) {
	parenCount := 0
	for i := start + len(funcName); i < len(expr); i++ {
		if expr[i] == '(' {
			parenCount++
		} else if expr[i] == ')' {
			if parenCount == 0 {
				return i, nil
			}
			parenCount--
		}
	}
	return -1, fmt.Errorf("函数括号不匹配")
}

// 数学函数实现（简化版本，避免导入 math 包）
func (t *CalculatorTool) pow(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	if exp == 1 {
		return base
	}
	// 简化实现：仅支持整数指数
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

func (t *CalculatorTool) sqrt(x float64) float64 {
	// 牛顿法求平方根
	if x < 0 {
		return 0
	}
	z := x
	for i := 0; i < 100; i++ {
		z = (z + x/z) / 2
		if z*z-x < 1e-10 && z*z-x > -1e-10 {
			break
		}
	}
	return z
}

func (t *CalculatorTool) sin(x float64) float64 {
	// 泰勒级数展开
	result := x
	term := x
	for n := 1; n < 20; n++ {
		term *= -x * x / float64((2*n)*(2*n+1))
		result += term
	}
	return result
}

func (t *CalculatorTool) cos(x float64) float64 {
	// 泰勒级数展开
	result := 1.0
	term := 1.0
	for n := 1; n < 20; n++ {
		term *= -x * x / float64((2*n-1)*(2*n))
		result += term
	}
	return result
}

func (t *CalculatorTool) tan(x float64) float64 {
	return t.sin(x) / t.cos(x)
}

func (t *CalculatorTool) log10(x float64) float64 {
	// 简化实现
	return t.ln(x) / 2.30258509299
}

func (t *CalculatorTool) ln(x float64) float64 {
	// 泰勒级数展开（仅对 x > 0 有效）
	if x <= 0 {
		return 0
	}
	// 使用 ln(x) = 2 * artanh((x-1)/(x+1))
	y := (x - 1) / (x + 1)
	result := 0.0
	yPow := y
	for n := 1; n < 100; n += 2 {
		result += yPow / float64(n)
		yPow *= y * y
	}
	return 2 * result
}

// SearchTool 搜索工具。
type SearchTool struct {
	client *http.Client
}

// Name 返回工具名称。
func (t *SearchTool) Name() string {
	return "search"
}

// Description 返回工具描述。
func (t *SearchTool) Description() string {
	return "网络搜索，支持 Bing 搜索引擎，返回搜索结果"
}

// Run 执行工具。
func (t *SearchTool) Run(query string, config map[string]any) (string, error) {
	// 初始化 HTTP 客户端
	if t.client == nil {
		timeout := 30
		if reqTimeout, ok := config["request_timeout"].(float64); ok {
			timeout = int(reqTimeout)
		} else if reqTimeout, ok := config["request_timeout"].(int); ok {
			timeout = reqTimeout
		}
		t.client = &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		}
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("请提供搜索关键词")
	}

	// 优先使用 Bing 搜索
	if apiKey, ok := config["bing_subscription_key"].(string); ok && apiKey != "" {
		return t.bingSearch(query, apiKey, config)
	}

	// 使用 Google 搜索（如果配置了）
	if apiKey, ok := config["google_api_key"].(string); ok && apiKey != "" {
		cseID, _ := config["google_cse_id"].(string)
		return t.googleSearch(query, apiKey, cseID)
	}

	// 如果没有配置搜索引擎，返回提示信息
	return "", fmt.Errorf("未配置搜索引擎 API Key，请在配置文件中设置 bing_subscription_key 或 google_api_key")
}

// bingSearch 使用 Bing 搜索 API。
func (t *SearchTool) bingSearch(query, apiKey string, config map[string]any) (string, error) {
	searchURL := BingSearchURL
	if url, ok := config["bing_search_url"].(string); ok && url != "" {
		searchURL = url
	}

	// 构建请求 URL
	reqURL := fmt.Sprintf("%s?q=%s&count=5&mkt=zh-CN", searchURL, url.QueryEscape(query))

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Ocp-Apim-Subscription-Key", apiKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 解析响应
	var bingResp struct {
		WebPages struct {
			Value []struct {
				Name            string `json:"name"`
				URL             string `json:"url"`
				Snippet         string `json:"snippet"`
				DateLastCrawled string `json:"dateLastCrawled"`
			} `json:"value"`
		} `json:"webPages"`
	}

	if err := json.Unmarshal(body, &bingResp); err != nil {
		return "", err
	}

	// 格式化结果
	if len(bingResp.WebPages.Value) == 0 {
		return fmt.Sprintf("未找到与 '%s' 相关的搜索结果", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 搜索: %s\n\n", query))
	sb.WriteString("搜索结果:\n\n")

	for i, result := range bingResp.WebPages.Value {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Name))
		sb.WriteString(fmt.Sprintf("   链接: %s\n", result.URL))
		if result.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   摘要: %s\n", result.Snippet))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// googleSearch 使用 Google Custom Search API。
func (t *SearchTool) googleSearch(query, apiKey, cseID string) (string, error) {
	if cseID == "" {
		return "", fmt.Errorf("未配置 Google Custom Search Engine ID (google_cse_id)")
	}

	// 构建请求 URL
	reqURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=5",
		apiKey, cseID, url.QueryEscape(query))

	resp, err := t.client.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 解析响应
	var googleResp struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &googleResp); err != nil {
		return "", err
	}

	// 格式化结果
	if len(googleResp.Items) == 0 {
		return fmt.Sprintf("未找到与 '%s' 相关的搜索结果", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 搜索: %s\n\n", query))
	sb.WriteString("搜索结果:\n\n")

	for i, result := range googleResp.Items {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		sb.WriteString(fmt.Sprintf("   链接: %s\n", result.Link))
		if result.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   摘要: %s\n", result.Snippet))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// 初始化时注册内置工具和插件创建器。
func init() {
	RegisterTool(&URLGetTool{})
	RegisterTool(&MeteoTool{})
	RegisterTool(&CalculatorTool{})
	RegisterTool(&SearchTool{})
}
