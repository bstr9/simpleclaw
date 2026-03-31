// Package llm 提供与各种 LLM 提供商交互的统一接口。
// deepseek.go 实现 DeepSeek API 客户端。
package llm

import (
	"context"
	"fmt"
)

// DeepSeek 模型常量
const (
	// DeepSeekDefaultBaseURL 是 DeepSeek API 的默认基础 URL
	DeepSeekDefaultBaseURL = "https://api.deepseek.com"

	// DeepSeekChat 是 DeepSeek 对话模型
	DeepSeekChat = "deepseek-chat"
	// DeepSeekCoder 是 DeepSeek 代码模型
	DeepSeekCoder = "deepseek-coder"
	// DeepSeekReasoner 是 DeepSeek 推理模型
	DeepSeekReasoner = "deepseek-reasoner"
)

// DeepSeekModel 实现 DeepSeek API 客户端
// DeepSeek API 完全兼容 OpenAI 格式，因此复用 OpenAI 客户端实现
type DeepSeekModel struct {
	*OpenAIModel // 嵌入 OpenAI 模型，复用其实现
	config       ModelConfig
}

// NewDeepSeekModel 创建新的 DeepSeek 模型实例
// 参数:
//   - cfg: 模型配置，包含 API Key、模型名称等
//
// 返回:
//   - *DeepSeekModel: DeepSeek 模型实例
//   - error: 错误信息
func NewDeepSeekModel(cfg ModelConfig) (*DeepSeekModel, error) {
	// 验证必填字段
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("deepseek api_key 是必需的")
	}
	if cfg.Model == "" {
		// 设置默认模型
		cfg.Model = DeepSeekChat
	}

	// 设置 DeepSeek API Base URL
	if cfg.APIBase == "" {
		cfg.APIBase = DeepSeekDefaultBaseURL + "/v1"
	}

	// 设置 provider 标识
	cfg.Provider = ProviderDeepSeek

	// 设置默认模型名称（用户友好名称）
	if cfg.ModelName == "" {
		cfg.ModelName = cfg.Model
	}

	// 创建底层 OpenAI 兼容客户端
	openaiModel, err := NewOpenAIModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 DeepSeek 客户端失败: %w", err)
	}

	return &DeepSeekModel{
		OpenAIModel: openaiModel,
		config:      cfg,
	}, nil
}

// Name 返回模型标识符
func (m *DeepSeekModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持工具调用
// DeepSeek 的 deepseek-chat 和 deepseek-coder 模型支持 function calling
func (m *DeepSeekModel) SupportsTools() bool {
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
func (m *DeepSeekModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
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
func (m *DeepSeekModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	// 直接委托给嵌入的 OpenAIModel
	return m.OpenAIModel.CallStream(ctx, messages, opts...)
}

// 确保 DeepSeekModel 实现 Model 接口
var _ Model = (*DeepSeekModel)(nil)
