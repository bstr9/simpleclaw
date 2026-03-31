// Package protocol 提供 Agent 交互协议的核心定义。
// protocol.go 定义了 Agent 交互协议的核心接口和类型。
package protocol

import (
	"context"
)

// Executor 定义 Agent 执行器接口
type Executor interface {
	// Run 执行任务并返回结果
	Run(ctx context.Context, task *Task, onEvent func(event map[string]any)) (*AgentResult, error)

	// RunWithMessages 使用已有消息历史执行任务
	RunWithMessages(ctx context.Context, messages []Message, onEvent func(event map[string]any)) (*AgentResult, error)
}

// 回调事件类型定义
const (
	// EventTypeText 文本输出事件
	EventTypeText = "text"
	// EventTypeToolCall 工具调用事件
	EventTypeToolCall = "tool_call"
	// EventTypeToolResult 工具结果事件
	EventTypeToolResult = "tool_result"
	// EventTypeError 错误事件
	EventTypeError = "error"
	// EventTypeStepStart 步骤开始事件
	EventTypeStepStart = "step_start"
	// EventTypeStepEnd 步骤结束事件
	EventTypeStepEnd = "step_end"
	// EventTypeComplete 完成事件
	EventTypeComplete = "complete"
	// EventTypeThinking 思考事件
	EventTypeThinking = "thinking"
)

// Event 表示执行过程中的事件
type Event struct {
	// Type 事件类型
	Type string `json:"type"`
	// Timestamp 事件时间戳
	Timestamp int64 `json:"timestamp"`
	// Data 事件数据
	Data map[string]any `json:"data,omitempty"`
}

// NewEvent 创建新事件
func NewEvent(eventType string, data map[string]any) *Event {
	return &Event{
		Type:      eventType,
		Timestamp: currentTimeMillis(),
		Data:      data,
	}
}

// TextEventData 文本事件数据
type TextEventData struct {
	// Text 文本内容
	Text string `json:"text"`
	// Delta 增量文本（流式输出）
	Delta string `json:"delta,omitempty"`
}

// ToolCallEventData 工具调用事件数据
type ToolCallEventData struct {
	// ToolName 工具名称
	ToolName string `json:"tool_name"`
	// ToolCallID 工具调用 ID
	ToolCallID string `json:"tool_call_id"`
	// Arguments 工具参数
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolResultEventData 工具结果事件数据
type ToolResultEventData struct {
	// ToolCallID 工具调用 ID
	ToolCallID string `json:"tool_call_id"`
	// ToolName 工具名称
	ToolName string `json:"tool_name"`
	// Result 工具执行结果
	Result any `json:"result"`
	// Status 执行状态
	Status string `json:"status"`
	// ExecutionTime 执行耗时（秒）
	ExecutionTime float64 `json:"execution_time"`
}

// ErrorEventData 错误事件数据
type ErrorEventData struct {
	// Message 错误消息
	Message string `json:"message"`
	// Code 错误代码
	Code string `json:"code,omitempty"`
}

// StepEventData 步骤事件数据
type StepEventData struct {
	// Step 当前步数
	Step int `json:"step"`
	// MaxSteps 最大步数
	MaxSteps int `json:"max_steps"`
	// Action 当前动作描述
	Action string `json:"action,omitempty"`
}

// CompleteEventData 完成事件数据
type CompleteEventData struct {
	// FinalAnswer 最终回答
	FinalAnswer string `json:"final_answer"`
	// StepCount 执行步数
	StepCount int `json:"step_count"`
	// Status 完成状态
	Status string `json:"status"`
}

// ThinkingEventData 思考事件数据
type ThinkingEventData struct {
	// Thought 思考内容
	Thought string `json:"thought"`
}

// Protocol 协议配置
type Protocol struct {
	// Name 协议名称
	Name string `json:"name"`
	// Version 协议版本
	Version string `json:"version"`
	// Description 协议描述
	Description string `json:"description"`
}

// DefaultProtocol 默认协议配置
var DefaultProtocol = &Protocol{
	Name:        "uai-agent-protocol",
	Version:     "1.0.0",
	Description: "UAI Agent 交互协议",
}

// GetProtocol 获取默认协议配置
func GetProtocol() *Protocol {
	return DefaultProtocol
}
