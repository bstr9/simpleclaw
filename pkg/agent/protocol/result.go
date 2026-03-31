// Package protocol 提供 Agent 交互协议的核心定义。
// result.go 定义了结果相关的类型和接口。
package protocol

import (
	"time"

	"github.com/google/uuid"
)

// AgentActionType 定义 Agent 动作类型枚举
type AgentActionType string

const (
	// ActionTypeToolUse 工具调用动作
	ActionTypeToolUse AgentActionType = "tool_use"
	// ActionTypeThinking 思考动作
	ActionTypeThinking AgentActionType = "thinking"
	// ActionTypeFinalAnswer 最终回答动作
	ActionTypeFinalAnswer AgentActionType = "final_answer"
)

// String 返回 AgentActionType 的字符串表示
func (t AgentActionType) String() string {
	return string(t)
}

// ToolResult 表示工具执行的返回结果
type ToolResult struct {
	// ToolName 工具名称
	ToolName string `json:"tool_name"`
	// InputParams 传递给工具的参数
	InputParams map[string]any `json:"input_params"`
	// Output 工具执行的输出结果
	Output any `json:"output"`
	// Status 执行状态 ("success" 或 "error")
	Status string `json:"status"`
	// ErrorMessage 错误信息（如果执行失败）
	ErrorMessage string `json:"error_message,omitempty"`
	// ExecutionTime 执行耗时（秒）
	ExecutionTime float64 `json:"execution_time"`
}

// NewToolResult 创建成功的工具结果
func NewToolResult(toolName string, inputParams map[string]any, output any, executionTime float64) *ToolResult {
	return &ToolResult{
		ToolName:      toolName,
		InputParams:   inputParams,
		Output:        output,
		Status:        "success",
		ExecutionTime: executionTime,
	}
}

// NewErrorToolResult 创建失败的工具结果
func NewErrorToolResult(toolName string, inputParams map[string]any, errMsg string, executionTime float64) *ToolResult {
	return &ToolResult{
		ToolName:      toolName,
		InputParams:   inputParams,
		Output:        nil,
		Status:        "error",
		ErrorMessage:  errMsg,
		ExecutionTime: executionTime,
	}
}

// IsSuccess 检查工具执行是否成功
func (r *ToolResult) IsSuccess() bool {
	return r.Status == "success"
}

// IsError 检查工具执行是否失败
func (r *ToolResult) IsError() bool {
	return r.Status == "error"
}

// AgentAction 表示 Agent 执行的一个动作
type AgentAction struct {
	// ID 动作的唯一标识符
	ID string `json:"id"`
	// AgentID 执行动作的 Agent ID
	AgentID string `json:"agent_id"`
	// AgentName 执行动作的 Agent 名称
	AgentName string `json:"agent_name"`
	// ActionType 动作类型
	ActionType AgentActionType `json:"action_type"`
	// Content 动作内容（思考内容或最终回答内容）
	Content string `json:"content"`
	// ToolResult 工具执行结果（当 ActionType 为 ToolUse 时）
	ToolResult *ToolResult `json:"tool_result,omitempty"`
	// Thought 思考过程
	Thought string `json:"thought,omitempty"`
	// Timestamp 动作执行时间戳
	Timestamp time.Time `json:"timestamp"`
}

// NewAgentAction 创建新的 Agent 动作
func NewAgentAction(agentID, agentName string, actionType AgentActionType, opts ...ActionOption) *AgentAction {
	action := &AgentAction{
		ID:         uuid.New().String(),
		AgentID:    agentID,
		AgentName:  agentName,
		ActionType: actionType,
		Timestamp:  time.Now(),
	}

	for _, opt := range opts {
		opt(action)
	}

	return action
}

// ActionOption 动作配置选项函数
type ActionOption func(*AgentAction)

// WithContent 设置动作内容
func WithContent(content string) ActionOption {
	return func(a *AgentAction) {
		a.Content = content
	}
}

// WithToolResult 设置工具结果
func WithToolResult(result *ToolResult) ActionOption {
	return func(a *AgentAction) {
		a.ToolResult = result
	}
}

// WithThought 设置思考过程
func WithThought(thought string) ActionOption {
	return func(a *AgentAction) {
		a.Thought = thought
	}
}

// AgentResult 表示 Agent 执行的最终结果
type AgentResult struct {
	// FinalAnswer 最终回答
	FinalAnswer string `json:"final_answer"`
	// StepCount 执行步数
	StepCount int `json:"step_count"`
	// Status 执行状态 ("success" 或 "error")
	Status string `json:"status"`
	// ErrorMessage 错误信息（如果执行失败）
	ErrorMessage string `json:"error_message,omitempty"`
	// Actions 执行过程中记录的所有动作
	Actions []*AgentAction `json:"actions,omitempty"`
	// Usage Token 使用情况
	Usage *TokenUsage `json:"usage,omitempty"`
}

// TokenUsage 表示 Token 使用情况
type TokenUsage struct {
	// PromptTokens 输入 Token 数
	PromptTokens int `json:"prompt_tokens"`
	// CompletionTokens 输出 Token 数
	CompletionTokens int `json:"completion_tokens"`
	// TotalTokens 总 Token 数
	TotalTokens int `json:"total_tokens"`
}

// NewAgentResult 创建新的 Agent 结果
func NewAgentResult(finalAnswer string, stepCount int) *AgentResult {
	return &AgentResult{
		FinalAnswer: finalAnswer,
		StepCount:   stepCount,
		Status:      "success",
		Actions:     make([]*AgentAction, 0),
	}
}

// NewSuccessResult 创建成功的 Agent 结果
func NewSuccessResult(finalAnswer string, stepCount int) *AgentResult {
	return &AgentResult{
		FinalAnswer: finalAnswer,
		StepCount:   stepCount,
		Status:      "success",
		Actions:     make([]*AgentAction, 0),
	}
}

// NewErrorResult 创建失败的 Agent 结果
func NewErrorResult(errorMessage string, stepCount int) *AgentResult {
	return &AgentResult{
		FinalAnswer:  "Error: " + errorMessage,
		StepCount:    stepCount,
		Status:       "error",
		ErrorMessage: errorMessage,
		Actions:      make([]*AgentAction, 0),
	}
}

// IsSuccess 检查执行是否成功
func (r *AgentResult) IsSuccess() bool {
	return r.Status == "success"
}

// IsError 检查执行是否失败
func (r *AgentResult) IsError() bool {
	return r.Status == "error"
}

// AddAction 添加执行动作
func (r *AgentResult) AddAction(action *AgentAction) {
	if r.Actions == nil {
		r.Actions = make([]*AgentAction, 0)
	}
	r.Actions = append(r.Actions, action)
}

// SetUsage 设置 Token 使用情况
func (r *AgentResult) SetUsage(promptTokens, completionTokens int) {
	r.Usage = &TokenUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}

// GetToolActions 获取所有工具调用动作
func (r *AgentResult) GetToolActions() []*AgentAction {
	var toolActions []*AgentAction
	for _, action := range r.Actions {
		if action.ActionType == ActionTypeToolUse {
			toolActions = append(toolActions, action)
		}
	}
	return toolActions
}

// GetFinalAnswerAction 获取最终回答动作
func (r *AgentResult) GetFinalAnswerAction() *AgentAction {
	for _, action := range r.Actions {
		if action.ActionType == ActionTypeFinalAnswer {
			return action
		}
	}
	return nil
}
