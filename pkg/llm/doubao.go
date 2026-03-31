// Package llm 提供与各种 LLM 提供商交互的统一接口。
// doubao.go 实现豆包（字节跳动/火山方舟）API 客户端。
package llm

import (
	"context"
	"fmt"
)

// 豆包模型常量
const (
	// DoubaoDefaultBaseURL 是豆包 API 的默认基础 URL
	DoubaoDefaultBaseURL = "https://ark.cn-beijing.volces.com/api/v3"

	// 常用豆包模型名称
	// 注意：豆包使用 endpoint ID 作为模型名，格式如：ep-xxxx-xxxx
	// 用户需要在火山引擎控制台创建推理接入点获取 endpoint ID
)

// DoubaoModel 实现豆包（字节跳动/火山方舟）API 客户端
// 豆包 API 完全兼容 OpenAI 格式，因此复用 OpenAI 客户端实现
// 特性：
//   - 兼容 OpenAI Chat Completions API
//   - 支持 streaming 和非 streaming 模式
//   - 支持 function calling / tool use
//   - 支持 thinking (reasoning) 模式（可通过配置禁用）
type DoubaoModel struct {
	*OpenAIModel // 嵌入 OpenAI 模型，复用其实现
	config       ModelConfig
}

// NewDoubaoModel 创建新的豆包模型实例
// 参数:
//   - cfg: 模型配置，包含 API Key（ark_api_key）、模型名称（endpoint ID）等
//
// 返回:
//   - *DoubaoModel: 豆包模型实例
//   - error: 错误信息
//
// 配置说明:
//   - APIKey: 火山引擎方舟平台的 API Key（ark_api_key）
//   - Model: 推理接入点的 endpoint ID，如：ep-xxxx-xxxx
//   - APIBase: 可选，默认为 https://ark.cn-beijing.volces.com/api/v3
func NewDoubaoModel(cfg ModelConfig) (*DoubaoModel, error) {
	// 验证必填字段
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("doubao api_key (ark_api_key) 是必需的")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("doubao model (endpoint ID) 是必需的")
	}

	// 设置豆包 API Base URL
	if cfg.APIBase == "" {
		cfg.APIBase = DoubaoDefaultBaseURL
	}

	// 设置 provider 标识
	cfg.Provider = ProviderDoubao

	// 设置默认模型名称（用户友好名称）
	if cfg.ModelName == "" {
		cfg.ModelName = cfg.Model
	}

	// 创建底层 OpenAI 兼容客户端
	openaiModel, err := NewOpenAIModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建豆包客户端失败: %w", err)
	}

	return &DoubaoModel{
		OpenAIModel: openaiModel,
		config:      cfg,
	}, nil
}

// Name 返回模型标识符
func (m *DoubaoModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持工具调用
// 豆包的大多数模型都支持 function calling
func (m *DoubaoModel) SupportsTools() bool {
	return true
}

// Call 执行同步对话完成请求
// 参数:
//   - ctx: 上下文，用于控制请求生命周期
//   - messages: 对话消息列表
//   - opts: 可选参数，如 temperature、max_tokens 等
//
// 返回:
//   - *Response: 模型响应
//   - error: 错误信息
//
// 注意：豆包 API 支持 thinking 模式，可通过 Extra 参数控制：
//
//	opts := []Option{
//	    WithExtra("thinking", map[string]any{"type": "disabled"}), // 禁用 thinking
//	}
func (m *DoubaoModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	// 直接委托给嵌入的 OpenAIModel
	return m.OpenAIModel.Call(ctx, messages, opts...)
}

// CallStream 执行流式对话完成请求
// 参数:
//   - ctx: 上下文，用于控制请求生命周期
//   - messages: 对话消息列表
//   - opts: 可选参数，如 temperature、max_tokens 等
//
// 返回:
//   - <-chan StreamChunk: 流式响应通道
//   - error: 错误信息
//
// 注意：流式模式下，豆包可能会返回 reasoning_content 字段（thinking 内容）
// 该内容会在 StreamChunk 中作为普通 Delta 返回
func (m *DoubaoModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	// 直接委托给嵌入的 OpenAIModel
	return m.OpenAIModel.CallStream(ctx, messages, opts...)
}

// 确保 DoubaoModel 实现 Model 接口
var _ Model = (*DoubaoModel)(nil)
