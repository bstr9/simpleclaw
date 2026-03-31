// Package agent 提供基于多智能体协作的任务处理插件。
// 该插件使用 AgentMesh 框架实现对终端、浏览器、文件系统、搜索引擎等工具的执行，
// 并支持多智能体协作完成复杂任务。
package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/plugin"
)

// Config 表示 agent 插件的配置。
type Config struct {
	// DefaultTeam 默认使用的团队名称。
	DefaultTeam string `json:"default_team"`

	// Teams 团队配置映射。
	Teams map[string]TeamConfig `json:"teams"`

	// Tools 工具配置。
	Tools map[string]any `json:"tools"`

	// TriggerPrefix 触发前缀，默认为 "$"。
	TriggerPrefix string `json:"trigger_prefix"`
}

// TeamConfig 表示团队配置。
type TeamConfig struct {
	// Description 团队描述。
	Description string `json:"description"`

	// Rule 团队规则。
	Rule string `json:"rule"`

	// Model 使用的模型名称。
	Model string `json:"model"`

	// MaxSteps 最大执行步数。
	MaxSteps int `json:"max_steps"`

	// Agents 团队中的智能体列表。
	Agents []AgentConfig `json:"agents"`
}

// AgentConfig 表示智能体配置。
type AgentConfig struct {
	// Name 智能体名称。
	Name string `json:"name"`

	// Description 智能体描述。
	Description string `json:"description"`

	// SystemPrompt 系统提示词。
	SystemPrompt string `json:"system_prompt"`

	// Model 使用的模型（可选，默认使用团队模型）。
	Model string `json:"model"`

	// MaxSteps 最大执行步数。
	MaxSteps int `json:"max_steps"`

	// Tools 可用的工具列表。
	Tools []string `json:"tools"`
}

// AgentPlugin 实现多智能体协作插件。
type AgentPlugin struct {
	*plugin.BasePlugin

	mu     sync.RWMutex
	config *Config
}

// 确保 AgentPlugin 实现了 Plugin 接口。
var _ plugin.Plugin = (*AgentPlugin)(nil)

// New 创建一个新的 AgentPlugin 实例。
func New() *AgentPlugin {
	bp := plugin.NewBasePlugin("agent", "0.1.0")
	bp.SetDescription("使用 AgentMesh 框架实现多智能体协作任务处理")
	bp.SetAuthor("Saboteur7")
	bp.SetPriority(1)

	p := &AgentPlugin{
		BasePlugin: bp,
		config:     &Config{TriggerPrefix: "$"},
	}
	return p
}

// Name 返回插件名称。
func (p *AgentPlugin) Name() string {
	return "agent"
}

// Version 返回插件版本。
func (p *AgentPlugin) Version() string {
	return "0.1.0"
}

// OnInit 初始化插件并加载配置。
func (p *AgentPlugin) OnInit(ctx *plugin.PluginContext) error {
	ctx.Debug("[agent] 正在初始化 agent 插件")

	// 加载配置文件
	configPath := filepath.Join(ctx.PluginPath, "config.json")
	if err := p.loadConfig(configPath); err != nil {
		ctx.Warn("[agent] 加载配置失败: " + err.Error())
		// 如果配置文件不存在，创建默认配置
		if os.IsNotExist(err) {
			if createErr := p.createDefaultConfig(configPath); createErr != nil {
				ctx.Warn("[agent] 创建默认配置失败: " + createErr.Error())
			}
		}
	}

	ctx.Info("[agent] 插件初始化完成")
	return nil
}

// OnLoad 插件加载时调用，注册事件处理器。
func (p *AgentPlugin) OnLoad(ctx *plugin.PluginContext) error {
	ctx.Debug("[agent] 正在加载 agent 插件")

	// 注册消息处理事件
	p.RegisterHandler(plugin.EventOnHandleContext, p.onHandleContext)

	ctx.Info("[agent] 插件加载成功")
	return nil
}

// OnUnload 插件卸载时调用，清理资源。
func (p *AgentPlugin) OnUnload(ctx *plugin.PluginContext) error {
	ctx.Debug("[agent] 正在卸载 agent 插件")
	return p.BasePlugin.OnUnload(ctx)
}

// OnEvent 处理插件事件。
func (p *AgentPlugin) OnEvent(event plugin.Event, ec *plugin.EventContext) error {
	return p.BasePlugin.OnEvent(event, ec)
}

// onHandleContext 处理消息上下文事件。
func (p *AgentPlugin) onHandleContext(ec *plugin.EventContext) error {
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

	// 获取触发前缀
	triggerPrefix := p.getTriggerPrefix()

	// 检查是否是 agent 命令
	if !strings.HasPrefix(content, triggerPrefix+"agent ") {
		return nil
	}

	// 提取任务内容
	task := strings.TrimSpace(strings.TrimPrefix(content, triggerPrefix+"agent "))

	// 如果任务为空，返回帮助信息
	if task == "" {
		ec.Set("reply", p.getHelpText(true))
		ec.BreakPass(p.Name())
		return nil
	}

	// 检查是否是查询可用团队
	if p.isTeamsQueryCommand(task) {
		p.handleTeamsQuery(ec)
		return nil
	}

	// 解析团队名称和任务
	teamName, task, handled := p.parseTeamAndTask(task, ec)
	if handled {
		return nil
	}

	// 解析默认团队名称
	if teamName == "" {
		teamName = p.resolveDefaultTeam(ec)
		if teamName == "" {
			return nil
		}
	}

	// 执行任务
	result, err := p.executeTask(teamName, task)
	if err != nil {
		ec.Set("reply", fmt.Sprintf("执行任务时出错: %s", err.Error()))
		ec.BreakPass(p.Name())
		return nil
	}

	ec.Set("reply", result)
	ec.BreakPass(p.Name())
	return nil
}

// isTeamsQueryCommand 检查是否是查询团队命令。
func (p *AgentPlugin) isTeamsQueryCommand(task string) bool {
	lowerTask := strings.ToLower(task)
	return lowerTask == "teams" || lowerTask == "list teams" || lowerTask == "show teams"
}

// handleTeamsQuery 处理团队查询命令。
func (p *AgentPlugin) handleTeamsQuery(ec *plugin.EventContext) {
	teams := p.getAvailableTeams()
	if len(teams) == 0 {
		ec.Set("reply", "未配置任何团队。请检查 config.json 文件。")
	} else {
		ec.Set("reply", fmt.Sprintf("可用团队: %s", strings.Join(teams, ", ")))
	}
	ec.BreakPass(p.Name())
}

// parseTeamAndTask 解析团队名称和任务内容，返回 (团队名, 任务, 是否已处理)。
func (p *AgentPlugin) parseTeamAndTask(task string, ec *plugin.EventContext) (string, string, bool) {
	if !strings.HasPrefix(task, "use ") {
		return "", task, false
	}

	parts := strings.SplitN(strings.TrimPrefix(task, "use "), " ", 2)
	if len(parts) == 0 {
		return "", task, false
	}

	teamName := parts[0]
	if len(parts) > 1 {
		return teamName, strings.TrimSpace(parts[1]), false
	}

	// 只指定了团队名，没有任务
	ec.Set("reply", fmt.Sprintf("已选择团队 '%s'。请输入您想执行的任务。", teamName))
	ec.BreakPass(p.Name())
	return teamName, "", true
}

// resolveDefaultTeam 解析默认团队名称。
func (p *AgentPlugin) resolveDefaultTeam(ec *plugin.EventContext) string {
	teamName := p.config.DefaultTeam
	if teamName != "" {
		return teamName
	}

	teams := p.getAvailableTeams()
	if len(teams) == 0 {
		ec.Set("reply", "未配置任何团队。请检查 config.json 文件。")
		ec.BreakPass(p.Name())
		return ""
	}
	return teams[0]
}

// executeTask 执行指定团队的任务。
func (p *AgentPlugin) executeTask(teamName, task string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 检查团队是否存在
	teamConfig, exists := p.config.Teams[teamName]
	if !exists {
		return "", fmt.Errorf("团队 '%s' 不存在", teamName)
	}

	// 这里是模拟执行，实际实现需要集成 AgentMesh 框架
	// 返回团队信息和任务
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🤖 团队: %s\n", teamName))
	sb.WriteString(fmt.Sprintf("📝 描述: %s\n", teamConfig.Description))
	sb.WriteString(fmt.Sprintf("🎯 任务: %s\n\n", task))
	sb.WriteString("--- 执行结果 ---\n")
	sb.WriteString("（此处应集成 AgentMesh 框架执行实际任务）\n")

	return sb.String(), nil
}

// getAvailableTeams 获取可用的团队列表。
func (p *AgentPlugin) getAvailableTeams() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	teams := make([]string, 0, len(p.config.Teams))
	for name := range p.config.Teams {
		teams = append(teams, name)
	}
	return teams
}

// getTriggerPrefix 获取触发前缀。
func (p *AgentPlugin) getTriggerPrefix() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.config.TriggerPrefix == "" {
		return "$"
	}
	return p.config.TriggerPrefix
}

// getHelpText 获取帮助文本。
func (p *AgentPlugin) getHelpText(verbose bool) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	helpText := "通过 AgentMesh 实现对终端、浏览器、文件系统、搜索引擎等工具的执行，并支持多智能体协作。"
	triggerPrefix := "$"
	if p.config.TriggerPrefix != "" {
		triggerPrefix = p.config.TriggerPrefix
	}

	if !verbose {
		return helpText
	}

	teams := p.getAvailableTeams()
	teamsStr := "未配置任何团队"
	if len(teams) > 0 {
		teamsStr = strings.Join(teams, ", ")
	}

	var sb strings.Builder
	sb.WriteString(helpText)
	sb.WriteString("\n\n使用说明：\n")
	sb.WriteString(fmt.Sprintf("%sagent [task] - 使用默认团队执行任务\n", triggerPrefix))
	sb.WriteString(fmt.Sprintf("%sagent teams - 列出可用的团队\n", triggerPrefix))
	sb.WriteString(fmt.Sprintf("%sagent use [team_name] [task] - 使用特定团队执行任务\n\n", triggerPrefix))
	sb.WriteString(fmt.Sprintf("可用团队: \n%s\n\n", teamsStr))
	sb.WriteString("示例:\n")
	sb.WriteString(fmt.Sprintf("%sagent 帮我查看当前文件夹路径\n", triggerPrefix))
	sb.WriteString(fmt.Sprintf("%sagent use software_team 帮我写一个产品预约体验的表单页面\n", triggerPrefix))

	return sb.String()
}

// loadConfig 加载配置文件。
func (p *AgentPlugin) loadConfig(path string) error {
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
func (p *AgentPlugin) createDefaultConfig(path string) error {
	defaultConfig := Config{
		TriggerPrefix: "$",
		DefaultTeam:   "default",
		Teams: map[string]TeamConfig{
			"default": {
				Description: "默认团队",
				Rule:        "协作完成任务",
				Model:       "gpt-4",
				MaxSteps:    20,
				Agents: []AgentConfig{
					{
						Name:         "assistant",
						Description:  "通用助手",
						SystemPrompt: "你是一个有用的AI助手",
						MaxSteps:     10,
						Tools:        []string{},
					},
				},
			},
		},
		Tools: map[string]any{},
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
func (p *AgentPlugin) HelpText() string {
	return p.getHelpText(false)
}
