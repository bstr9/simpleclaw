// Package llm 提供与各种 LLM 提供商交互的统一接口。
// 支持 OpenAI、Claude、Gemini、DeepSeek、GLM、Qwen、MiniMax、Kimi 和其他
// OpenAI 兼容 API。
package llm

import (
	"context"
)

// Role 表示对话中消息的角色。
type Role string

const (
	// RoleSystem 表示设置助手行为的系统消息。
	RoleSystem Role = "system"
	// RoleUser 表示用户消息。
	RoleUser Role = "user"
	// RoleAssistant 表示助手消息。
	RoleAssistant Role = "assistant"
	// RoleTool 表示工具/函数调用结果消息。
	RoleTool Role = "tool"
)

// Message 表示对话中的单条消息。
type Message struct {
	// Role 表示发言者（system、user、assistant、tool）。
	Role Role `json:"role"`
	// Content 是消息的文本内容。
	Content string `json:"content"`
	// Name 是参与者的可选名称（用于多方对话）。
	Name string `json:"name,omitempty"`
	// ToolCalls 包含助手发起的工具/函数调用。
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID 是此消息响应的工具调用 ID（用于 role: tool）。
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolCall 表示助手发起的工具/函数调用。
type ToolCall struct {
	// ID 是工具调用的唯一标识符。
	ID string `json:"id"`
	// Type 是工具调用类型（目前总是 "function"）。
	Type string `json:"type"`
	// Function 包含函数调用详情。
	Function FunctionCall `json:"function"`
}

// FunctionCall 表示带有名称和参数的函数调用。
type FunctionCall struct {
	// Name 是要调用的函数名称。
	Name string `json:"name"`
	// Arguments 是函数参数的 JSON 编码字符串。
	Arguments string `json:"arguments"`
}

// ToolDefinition 表示可被 LLM 调用的工具/函数。
type ToolDefinition struct {
	// Type 是工具类型（目前总是 "function"）。
	Type string `json:"type"`
	// Function 包含函数定义。
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition 定义可调用的函数。
type FunctionDefinition struct {
	// Name 是函数名称。
	Name string `json:"name"`
	// Description 说明函数的功能。
	Description string `json:"description,omitempty"`
	// Parameters 是函数参数的 JSON Schema。
	Parameters any `json:"parameters,omitempty"`
}

// Usage 表示令牌使用信息。
type Usage struct {
	// PromptTokens 是提示词中的令牌数。
	PromptTokens int `json:"prompt_tokens"`
	// CompletionTokens 是补全中的令牌数。
	CompletionTokens int `json:"completion_tokens"`
	// TotalTokens 是使用的总令牌数。
	TotalTokens int `json:"total_tokens"`
}

// Response 表示 LLM 的完整响应。
type Response struct {
	// Content 是响应的文本内容。
	Content string `json:"content"`
	// ToolCalls 包含助手发起的工具/函数调用。
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// Usage 包含令牌使用信息。
	Usage Usage `json:"usage,omitempty"`
	// FinishReason 表示模型停止生成的原因。
	FinishReason string `json:"finish_reason,omitempty"`
	// Model 是响应用到的实际模型。
	Model string `json:"model,omitempty"`
}

// StreamChunk 表示流式响应的一个数据块。
type StreamChunk struct {
	// Delta 是增量文本内容。
	Delta string `json:"delta,omitempty"`
	// ToolCallDelta 包含增量工具调用数据。
	ToolCallDelta *ToolCallDelta `json:"tool_call_delta,omitempty"`
	// Done 表示流是否已完成。
	Done bool `json:"done,omitempty"`
	// Error 包含流式传输中发生的错误。
	Error error `json:"error,omitempty"`
	// Usage 包含令牌使用信息（某些提供商在最后一个数据块中提供）。
	Usage *Usage `json:"usage,omitempty"`
	// FinishReason 表示模型停止生成的原因。
	FinishReason string `json:"finish_reason,omitempty"`
}

// ToolCallDelta 表示流式响应中的增量工具调用数据。
type ToolCallDelta struct {
	// Index 是工具调用在数组中的索引。
	Index int `json:"index"`
	// ID 是工具调用 ID（在第一个数据块中提供）。
	ID string `json:"id,omitempty"`
	// Type 是工具调用类型。
	Type string `json:"type,omitempty"`
	// Function 包含增量函数调用数据。
	Function *FunctionCallDelta `json:"function,omitempty"`
}

// FunctionCallDelta 表示增量函数调用数据。
type FunctionCallDelta struct {
	// Name 是函数名称（在第一个数据块中提供）。
	Name string `json:"name,omitempty"`
	// Arguments 是增量参数字符串。
	Arguments string `json:"arguments,omitempty"`
}

// CallOptions 包含单次 LLM 调用的选项。
type CallOptions struct {
	// Temperature 控制随机性（0-2，越高越随机）。
	Temperature *float64 `json:"temperature,omitempty"`
	// MaxTokens 限制响应中的令牌数量。
	MaxTokens int `json:"max_tokens,omitempty"`
	// TopP 通过核采样控制多样性。
	TopP *float64 `json:"top_p,omitempty"`
	// Stop 是停止生成的序列。
	Stop []string `json:"stop,omitempty"`
	// Tools 是模型可调用的工具列表。
	Tools []ToolDefinition `json:"tools,omitempty"`
	// ToolChoice 控制工具调用行为（"auto"、"none"、"required" 或指定工具）。
	ToolChoice any `json:"tool_choice,omitempty"`
	// SystemPrompt 覆盖默认系统提示词。
	SystemPrompt string `json:"system_prompt,omitempty"`
	// ResponseFormat 控制响应格式（"text"、"json_object" 或 JSON schema）。
	ResponseFormat any `json:"response_format,omitempty"`
	// FrequencyPenalty 减少重复（-2.0 到 2.0）。
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	// PresencePenalty 鼓励新话题（-2.0 到 2.0）。
	PresencePenalty *float64 `json:"presence_penalty,omitempty"`
	// User 是终端用户的唯一标识符。
	User string `json:"user,omitempty"`
	// Extra 包含提供商特定的选项。
	Extra map[string]any `json:"extra,omitempty"`
}

// Option 是修改 CallOptions 的函数。
type Option func(*CallOptions)

// WithTemperature 设置温度选项。
func WithTemperature(temp float64) Option {
	return func(o *CallOptions) {
		o.Temperature = &temp
	}
}

// WithMaxTokens 设置最大令牌选项。
func WithMaxTokens(maxTokens int) Option {
	return func(o *CallOptions) {
		o.MaxTokens = maxTokens
	}
}

// WithTools 设置工具选项。
func WithTools(tools []ToolDefinition) Option {
	return func(o *CallOptions) {
		o.Tools = tools
	}
}

// WithSystemPrompt 设置系统提示词选项。
func WithSystemPrompt(prompt string) Option {
	return func(o *CallOptions) {
		o.SystemPrompt = prompt
	}
}

// Model 定义 LLM 交互接口。
type Model interface {
	// Name 返回模型标识符。
	Name() string

	// Call 执行同步补全调用。
	// 返回完整响应或错误。
	Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error)

	// CallStream 执行流式补全调用。
	// 返回一个通道，输出 StreamChunk 对象。
	// 调用者必须读取所有数据块，直到 Done 为 true 或 Error 被设置。
	CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error)

	// SupportsTools 返回模型是否支持函数/工具调用。
	SupportsTools() bool
}

// ModelConfig 包含创建 Model 实例的配置。
type ModelConfig struct {
	// ModelName 是模型面向用户的名称。
	ModelName string `json:"model_name"`
	// Model 是实际的模型标识符（如 "gpt-4"、"claude-3-opus"）。
	Model string `json:"model"`
	// APIBase 是 API 的基础 URL（用于 OpenAI 兼容提供商）。
	APIBase string `json:"api_base,omitempty"`
	// APIKey 是认证密钥。
	APIKey string `json:"api_key"`
	// SecretKey 是需要它的提供商的密钥（如百度）。
	SecretKey string `json:"secret_key,omitempty"`
	// Proxy 是可选的 HTTP 代理 URL。
	Proxy string `json:"proxy,omitempty"`
	// Provider 表示提供商类型（openai、anthropic、gemini 等）。
	Provider string `json:"provider,omitempty"`
	// RequestTimeout 是 API 请求超时时间（秒）。
	RequestTimeout int `json:"request_timeout,omitempty"`
	// DefaultOptions 是此模型的默认调用选项。
	DefaultOptions CallOptions `json:"default_options,omitempty"`
	// Extra 包含提供商特定的配置。
	Extra map[string]any `json:"extra,omitempty"`
}

// ModelInfo 提供模型的元数据。
type ModelInfo struct {
	// ID 是模型标识符。
	ID string `json:"id"`
	// Name 是显示名称。
	Name string `json:"name"`
	// Provider 是提供商名称。
	Provider string `json:"provider"`
	// ContextWindow 是最大上下文长度。
	ContextWindow int `json:"context_window,omitempty"`
	// SupportsVision 表示模型是否支持图像输入。
	SupportsVision bool `json:"supports_vision,omitempty"`
	// SupportsTools 表示模型是否支持函数调用。
	SupportsTools bool `json:"supports_tools,omitempty"`
	// SupportsStreaming 表示模型是否支持流式输出。
	SupportsStreaming bool `json:"supports_streaming,omitempty"`
}

// ApplyDefaults 将默认选项应用到给定选项。
func (co *CallOptions) ApplyDefaults(defaults CallOptions) {
	co.applyPointerDefaults(defaults)
	co.applyValueDefaults(defaults)
	co.applySliceDefaults(defaults)
}

func (co *CallOptions) applyPointerDefaults(defaults CallOptions) {
	if co.Temperature == nil {
		co.Temperature = defaults.Temperature
	}
	if co.TopP == nil {
		co.TopP = defaults.TopP
	}
	if co.ToolChoice == nil {
		co.ToolChoice = defaults.ToolChoice
	}
	if co.ResponseFormat == nil {
		co.ResponseFormat = defaults.ResponseFormat
	}
	if co.FrequencyPenalty == nil {
		co.FrequencyPenalty = defaults.FrequencyPenalty
	}
	if co.PresencePenalty == nil {
		co.PresencePenalty = defaults.PresencePenalty
	}
	if co.Extra == nil {
		co.Extra = defaults.Extra
	}
}

func (co *CallOptions) applyValueDefaults(defaults CallOptions) {
	if co.MaxTokens == 0 {
		co.MaxTokens = defaults.MaxTokens
	}
	if co.SystemPrompt == "" {
		co.SystemPrompt = defaults.SystemPrompt
	}
	if co.User == "" {
		co.User = defaults.User
	}
}

func (co *CallOptions) applySliceDefaults(defaults CallOptions) {
	if len(co.Stop) == 0 {
		co.Stop = defaults.Stop
	}
	if len(co.Tools) == 0 {
		co.Tools = defaults.Tools
	}
}
