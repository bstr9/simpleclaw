// Package bridge 提供消息处理的核心路由层
// agent_bridge_initializer.go Agent 初始化器
package bridge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/agent/prompt"
	"github.com/bstr9/simpleclaw/pkg/agent/tools"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/extension/registry"
	"github.com/bstr9/simpleclaw/pkg/llm"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// AgentInitializer 处理 Agent 初始化
type AgentInitializer struct {
	agentBridge *AgentBridge
}

// NewAgentInitializer 创建新的 AgentInitializer
func NewAgentInitializer(ab *AgentBridge) *AgentInitializer {
	return &AgentInitializer{
		agentBridge: ab,
	}
}

// Initialize 创建已初始化的 Agent 实例
func (ai *AgentInitializer) Initialize(sessionID string) (*agent.Agent, error) {
	cfg := config.Get()

	workspaceDir := ai.getWorkspaceDir(cfg)
	toolRegistry := ai.createToolRegistry(workspaceDir)
	systemPrompt := ai.buildSystemPrompt(toolRegistry, workspaceDir)

	model, err := ai.createModel()
	if err != nil {
		return nil, err
	}

	maxSteps := ai.getMaxSteps(cfg)
	a := ai.createAgent(systemPrompt, model, toolRegistry, maxSteps)

	ai.restoreSessionHistory(sessionID, a)

	if sessionID == "" {
		ai.logInitialization(workspaceDir, toolRegistry, maxSteps)
	}

	return a, nil
}

// getWorkspaceDir 获取工作空间目录
func (ai *AgentInitializer) getWorkspaceDir(cfg *config.Config) string {
	workspaceDir := cfg.AgentWorkspace
	if workspaceDir == "" {
		workspaceDir = "~/cow"
	}
	return workspaceDir
}

// createToolRegistry 创建工具注册表
func (ai *AgentInitializer) createToolRegistry(workspaceDir string) *agent.ToolRegistry {
	toolRegistry := agent.NewToolRegistry()
	ai.loadTools(toolRegistry, workspaceDir)
	return toolRegistry
}

// getMaxSteps 获取最大执行步数
func (ai *AgentInitializer) getMaxSteps(cfg *config.Config) int {
	maxSteps := cfg.AgentMaxSteps
	if maxSteps <= 0 {
		return 15
	}
	return maxSteps
}

// createAgent 创建 Agent 实例
func (ai *AgentInitializer) createAgent(systemPrompt string, model llm.Model, toolRegistry *agent.ToolRegistry, maxSteps int) *agent.Agent {
	agentOpts := []agent.Option{
		agent.WithSystemPrompt(systemPrompt),
		agent.WithModel(model),
		agent.WithTools(toolRegistry.GetAll()),
		agent.WithMaxSteps(maxSteps),
		agent.WithStream(true),
	}
	return agent.NewAgent(agentOpts...)
}

// restoreSessionHistory 恢复会话历史
func (ai *AgentInitializer) restoreSessionHistory(sessionID string, a *agent.Agent) {
	if sessionID == "" || ai.agentBridge == nil || ai.agentBridge.memoryMgr == nil {
		return
	}

	ctx := context.Background()
	history, err := ai.agentBridge.memoryMgr.GetSessionMessages(ctx, sessionID, 50)
	if err != nil || len(history) == 0 {
		return
	}

	messages := ai.convertHistoryToMessages(history)
	if len(messages) > 0 {
		a.SetMessages(messages)
		logger.Debug("[AgentInitializer] Restored session history",
			zap.String("session_id", sessionID),
			zap.Int("messages", len(messages)))
	}
}

// convertHistoryToMessages 转换历史记录为消息列表
func (ai *AgentInitializer) convertHistoryToMessages(history []map[string]any) []llm.Message {
	messages := make([]llm.Message, 0, len(history))
	for _, h := range history {
		role, _ := h["role"].(string)
		content, _ := h["content"].(string)
		if role != "" && content != "" {
			messages = append(messages, llm.Message{
				Role:    llm.Role(role),
				Content: content,
			})
		}
	}
	return messages
}

// logInitialization 记录初始化日志
func (ai *AgentInitializer) logInitialization(workspaceDir string, toolRegistry *agent.ToolRegistry, maxSteps int) {
	logger.Info("[AgentInitializer] Agent initialized",
		zap.String("workspace", workspaceDir),
		zap.Int("tools", toolRegistry.Count()),
		zap.Int("max_steps", maxSteps))
}

// loadTools 加载所有可用工具到注册表
func (ai *AgentInitializer) loadTools(toolRegistry *agent.ToolRegistry, workspaceDir string) {
	tools.RegisterBuiltInTools(toolRegistry, tools.WithWorkingDir(workspaceDir))

	for _, tool := range registry.GetTools() {
		toolRegistry.Register(tool)
	}
}

// buildSystemPrompt 构建 Agent 的完整系统提示词
func (ai *AgentInitializer) buildSystemPrompt(toolRegistry *agent.ToolRegistry, workspaceDir string) string {
	toolInfos := ai.getToolInfos(toolRegistry)
	skillsPrompt := ai.buildSkillsPrompt()

	opts := &prompt.BuildOptions{
		Tools:        toolInfos,
		WorkspaceDir: workspaceDir,
		Language:     "zh",
		Runtime:      ai.getRuntimeInfo(workspaceDir),
		SkillsPrompt: skillsPrompt,
	}

	return prompt.BuildSystemPrompt(opts)
}

// buildSkillsPrompt 构建技能列表提示词
func (ai *AgentInitializer) buildSkillsPrompt() string {
	ab := ai.agentBridge
	if ab == nil || ab.skillsRegistry == nil {
		return ""
	}

	entries := ab.skillsRegistry.ListEnabled()
	if len(entries) == 0 {
		return ""
	}

	var lines []string
	for _, entry := range entries {
		skillPath := filepath.Join(entry.SkillInfo.BaseDir, "SKILL.md")
		lines = append(lines, fmt.Sprintf("<skill>\n<name>%s</name>\n<description>%s</description>\n<location>%s</location>\n</skill>",
			entry.Skill.Name(),
			entry.Skill.Description(),
			skillPath,
		))
	}

	return strings.Join(lines, "\n")
}

// getToolInfos 从注册表提取工具信息
func (ai *AgentInitializer) getToolInfos(registry *agent.ToolRegistry) []*prompt.ToolInfo {
	tools := registry.GetAll()
	infos := make([]*prompt.ToolInfo, 0, len(tools))

	for _, t := range tools {
		infos = append(infos, &prompt.ToolInfo{
			Name:        t.Name(),
			Description: t.Description(),
			Summary:     ai.getToolSummary(t.Name()),
		})
	}

	return infos
}

// getToolSummary 返回常用工具的简短摘要
func (ai *AgentInitializer) getToolSummary(name string) string {
	summaries := map[string]string{
		"read":       "读取文件内容",
		"write":      "创建或覆盖文件",
		"edit":       "精确编辑文件",
		"ls":         "列出目录内容",
		"bash":       "执行shell命令",
		"web_search": "网络搜索",
		"web_fetch":  "获取URL内容",
		"browser":    "控制浏览器",
		"memory":     "管理记忆",
		"env_config": "管理API密钥和配置",
		"scheduler":  "管理定时任务",
		"send":       "发送本地文件给用户",
		"time":       "获取当前时间",
		"vision":     "图像识别",
	}

	if s, ok := summaries[name]; ok {
		return s
	}
	return ""
}

// createModel 创建 LLM 模型实例
func (ai *AgentInitializer) createModel() (llm.Model, error) {
	return ai.agentBridge.bridge.GetBot(BotTypeOpenAI)
}

// getRuntimeInfo 返回系统提示词的运行时信息
func (ai *AgentInitializer) getRuntimeInfo(workspaceDir string) *prompt.RuntimeInfo {
	cfg := config.Get()

	modelName := cfg.ModelName
	if modelName == "" {
		modelName = cfg.Model
	}

	return &prompt.RuntimeInfo{
		Model:     modelName,
		Workspace: workspaceDir,
		Channel:   cfg.ChannelType,
	}
}
