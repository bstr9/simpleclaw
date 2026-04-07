// Package agent 提供 AI Agent 核心实现
package agent

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/llm"
)

// AgentOption Agent 配置选项函数
type Option func(*Agent)

// WithSystemPrompt 设置系统提示
func WithSystemPrompt(prompt string) Option {
	return func(a *Agent) {
		a.systemPrompt = prompt
	}
}

// WithModel 设置 LLM 模型
func WithModel(model llm.Model) Option {
	return func(a *Agent) {
		a.model = model
	}
}

// WithTools 设置工具列表
func WithTools(tools []Tool) Option {
	return func(a *Agent) {
		a.tools = tools
	}
}

// WithMaxSteps 设置最大执行步数
func WithMaxSteps(maxSteps int) Option {
	return func(a *Agent) {
		a.maxSteps = maxSteps
	}
}

// WithStream 设置是否启用流式输出
func WithStream(stream bool) Option {
	return func(a *Agent) {
		a.stream = stream
	}
}

// 回调事件类型
const (
	EventTypeText       = "text"
	EventTypeToolCall   = "tool_call"
	EventTypeToolResult = "tool_result"
	EventTypeError      = "error"
	EventTypeStepStart  = "step_start"
	EventTypeStepEnd    = "step_end"
	EventTypeComplete   = "complete"
)

// Agent 主体结构
type Agent struct {
	systemPrompt string
	model        llm.Model
	tools        []Tool
	toolRegistry *ToolRegistry
	maxSteps     int
	maxTokens    int
	temperature  float64
	stream       bool
	messages     []llm.Message
	mu           sync.RWMutex
	toolCtx      *ToolContext
}

// NewAgent 创建新的 Agent 实例
func NewAgent(opts ...Option) *Agent {
	a := &Agent{
		messages:     make([]llm.Message, 0),
		tools:        make([]Tool, 0),
		toolRegistry: NewToolRegistry(),
		maxSteps:     10,
		maxTokens:    2048,
		temperature:  0.7,
	}

	for _, opt := range opts {
		opt(a)
	}

	for _, tool := range a.tools {
		a.toolRegistry.Register(tool)
	}

	return a
}

// AddMessage 添加消息到历史
func (a *Agent) AddMessage(role llm.Role, content string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.messages = append(a.messages, llm.Message{
		Role:    role,
		Content: content,
	})
}

// AddUserMessage 添加用户消息
func (a *Agent) AddUserMessage(content string) {
	a.AddMessage(llm.RoleUser, content)
}

// AddAssistantMessage 添加助手消息
func (a *Agent) AddAssistantMessage(content string) {
	a.AddMessage(llm.RoleAssistant, content)
}

// AddToolCallMessage 添加工具调用消息（assistant）
func (a *Agent) AddToolCallMessage(toolCalls []llm.ToolCall) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.messages = append(a.messages, llm.Message{
		Role:      llm.RoleAssistant,
		Content:   "",
		ToolCalls: toolCalls,
	})
}

// AddToolResultMessage 添加工具结果消息（tool）
func (a *Agent) AddToolResultMessage(toolCallID, content string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.messages = append(a.messages, llm.Message{
		Role:       llm.RoleTool,
		Content:    content,
		ToolCallID: toolCallID,
	})
}

// GetMessages 获取消息历史（线程安全）
func (a *Agent) GetMessages() []llm.Message {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]llm.Message, len(a.messages))
	copy(result, a.messages)
	return result
}

// GetMessagesWithSystem 获取包含系统提示的消息历史
func (a *Agent) GetMessagesWithSystem() []llm.Message {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]llm.Message, 0, len(a.messages)+1)

	if a.systemPrompt != "" {
		result = append(result, llm.Message{
			Role:    llm.RoleSystem,
			Content: a.systemPrompt,
		})
	}

	result = append(result, a.messages...)
	return result
}

// SetMessages 设置消息历史（用于从持久化存储恢复）
func (a *Agent) SetMessages(messages []llm.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.messages = make([]llm.Message, len(messages))
	copy(a.messages, messages)
}

// ClearHistory 清空消息历史
func (a *Agent) ClearHistory() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.messages = make([]llm.Message, 0)
}

// TrimHistory 裁剪消息历史，保留最近的 n 条
func (a *Agent) TrimHistory(n int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.messages) <= n {
		return
	}

	a.messages = a.messages[len(a.messages)-n:]
}

// SetSystemPrompt 设置系统提示
func (a *Agent) SetSystemPrompt(prompt string) {
	a.systemPrompt = prompt
}

// GetSystemPrompt 获取系统提示
func (a *Agent) GetSystemPrompt() string {
	return a.systemPrompt
}

// GetTools 获取工具列表
func (a *Agent) GetTools() []Tool {
	return a.tools
}

// GetToolRegistry 获取工具注册表
func (a *Agent) GetToolRegistry() *ToolRegistry {
	return a.toolRegistry
}

// SetMaxSteps 设置最大执行步数
func (a *Agent) SetMaxSteps(maxSteps int) {
	a.maxSteps = maxSteps
}

// GetMaxSteps 获取最大执行步数
func (a *Agent) GetMaxSteps() int {
	return a.maxSteps
}

// GetModel 获取模型
func (a *Agent) GetModel() llm.Model {
	return a.model
}

// GetTemperature 获取温度参数
func (a *Agent) GetTemperature() float64 {
	return a.temperature
}

// GetMaxTokens 获取最大 token 数
func (a *Agent) GetMaxTokens() int {
	return a.maxTokens
}

// SetToolContext 设置工具上下文
func (a *Agent) SetToolContext(ctx *ToolContext) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.toolCtx = ctx
}

// GetToolContext 获取工具上下文
func (a *Agent) GetToolContext() *ToolContext {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.toolCtx
}

// Run 执行 Agent
func (a *Agent) Run(ctx context.Context, userMessage string, onEvent func(event map[string]any)) (string, error) {
	return newExecutor(a, onEvent).run(ctx, userMessage)
}

// RunWithHistory 使用已有历史执行 Agent
func (a *Agent) RunWithHistory(ctx context.Context, messages []llm.Message, onEvent func(event map[string]any)) (string, error) {
	a.mu.Lock()
	a.messages = messages
	a.mu.Unlock()

	return newExecutor(a, onEvent).run(ctx, "")
}

// parseToolCallArgs 解析工具调用参数
func parseToolCallArgs(toolCall llm.ToolCall) (map[string]any, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return nil, err
	}
	return args, nil
}

// emitEvent 发送事件
func emitEvent(onEvent func(event map[string]any), eventType string, data map[string]any) {
	if onEvent == nil {
		return
	}

	event := map[string]any{
		"type": eventType,
	}
	for k, v := range data {
		event[k] = v
	}

	onEvent(event)
}
