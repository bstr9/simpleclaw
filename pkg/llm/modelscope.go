// Package llm 提供与各种 LLM 提供商交互的统一接口。
// modelscope.go 实现魔搭社区 API 模型客户端。
package llm

import (
	"context"
	"fmt"
)

// ModelScope API 基础 URL
const (
	// ModelScopeDefaultBaseURL 是魔搭社区 API 推理服务的默认地址
	ModelScopeDefaultBaseURL = "https://api-inference.modelscope.cn/v1"
)

// ModelScope 模型标识符常量
const (
	// ModelScopeQwen25_7B 通义千问 2.5 7B 指令模型
	ModelScopeQwen25_7B = "Qwen/Qwen2.5-7B-Instruct"
	// ModelScopeQwen25_14B 通义千问 2.5 14B 指令模型
	ModelScopeQwen25_14B = "Qwen/Qwen2.5-14B-Instruct"
	// ModelScopeQwen25_32B 通义千问 2.5 32B 指令模型
	ModelScopeQwen25_32B = "Qwen/Qwen2.5-32B-Instruct"
	// ModelScopeQwen25_72B 通义千问 2.5 72B 指令模型
	ModelScopeQwen25_72B = "Qwen/Qwen2.5-72B-Instruct"
	// ModelScopeQwQ32B QwQ-32B 推理模型
	ModelScopeQwQ32B = "Qwen/QwQ-32B"
	// ModelScopeDeepSeekV3 DeepSeek V3 模型
	ModelScopeDeepSeekV3 = "deepseek-ai/DeepSeek-V3"
	// ModelScopeDeepSeekR1 DeepSeek R1 推理模型
	ModelScopeDeepSeekR1 = "deepseek-ai/DeepSeek-R1"
)

// modelscopeModelInfo 定义模型的能力信息
var modelscopeModelInfo = map[string]ModelInfo{
	ModelScopeQwen25_7B: {
		ID:                ModelScopeQwen25_7B,
		Name:              "通义千问 2.5 7B 指令版",
		Provider:          ProviderModelScope,
		ContextWindow:     32768,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	ModelScopeQwen25_14B: {
		ID:                ModelScopeQwen25_14B,
		Name:              "通义千问 2.5 14B 指令版",
		Provider:          ProviderModelScope,
		ContextWindow:     32768,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	ModelScopeQwen25_32B: {
		ID:                ModelScopeQwen25_32B,
		Name:              "通义千问 2.5 32B 指令版",
		Provider:          ProviderModelScope,
		ContextWindow:     32768,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	ModelScopeQwen25_72B: {
		ID:                ModelScopeQwen25_72B,
		Name:              "通义千问 2.5 72B 指令版",
		Provider:          ProviderModelScope,
		ContextWindow:     32768,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	ModelScopeQwQ32B: {
		ID:                ModelScopeQwQ32B,
		Name:              "QwQ-32B 推理模型",
		Provider:          ProviderModelScope,
		ContextWindow:     32768,
		SupportsVision:    false,
		SupportsTools:     false,
		SupportsStreaming: true,
	},
	ModelScopeDeepSeekV3: {
		ID:                ModelScopeDeepSeekV3,
		Name:              "DeepSeek V3",
		Provider:          ProviderModelScope,
		ContextWindow:     64000,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	ModelScopeDeepSeekR1: {
		ID:                ModelScopeDeepSeekR1,
		Name:              "DeepSeek R1 推理模型",
		Provider:          ProviderModelScope,
		ContextWindow:     64000,
		SupportsVision:    false,
		SupportsTools:     false,
		SupportsStreaming: true,
	},
}

// ModelScopeModel 实现魔搭社区 API 的 Model 接口
// 魔搭社区 API 兼容 OpenAI 格式，因此直接复用 OpenAIModel 实现
// 支持 streaming 和非 streaming 模式
type ModelScopeModel struct {
	*OpenAIModel // 嵌入 OpenAIModel，复用其实现
	config       ModelConfig
	modelInfo    *ModelInfo
}

// NewModelScopeModel 创建新的魔搭社区模型实例
// 魔搭社区 API 完全兼容 OpenAI 格式，可以直接使用 OpenAI 兼容客户端
//
// 参数说明:
//   - cfg.Model: 模型标识符，如 "Qwen/Qwen2.5-7B-Instruct", "Qwen/QwQ-32B" 等
//   - cfg.APIKey: 魔搭社区 API Key (从 https://modelscope.cn 获取)
//   - cfg.APIBase: 可选，自定义 API 地址，默认使用魔搭社区推理服务地址
//   - cfg.Proxy: 可选，HTTP 代理地址
//   - cfg.RequestTimeout: 可选，请求超时时间（秒）
//
// 示例:
//
//	model, err := NewModelScopeModel(ModelConfig{
//	    Model:  "Qwen/Qwen2.5-7B-Instruct",
//	    APIKey: "your-modelscope-api-key",
//	})
func NewModelScopeModel(cfg ModelConfig) (*ModelScopeModel, error) {
	// 验证必要参数
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("modelscope api_key 是必需的")
	}
	if cfg.Model == "" {
		// 设置默认模型
		cfg.Model = ModelScopeQwen25_7B
	}

	// 设置默认 API Base URL
	if cfg.APIBase == "" {
		cfg.APIBase = ModelScopeDefaultBaseURL
	}

	// 设置默认 Provider
	if cfg.Provider == "" {
		cfg.Provider = ProviderModelScope
	}

	// 设置默认模型名称（用户友好名称）
	if cfg.ModelName == "" {
		cfg.ModelName = cfg.Model
	}

	// 创建 OpenAI 兼容模型
	openaiModel, err := NewOpenAIModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建魔搭社区模型失败: %w", err)
	}

	// 获取模型信息
	var info *ModelInfo
	if modelInfo, ok := modelscopeModelInfo[cfg.Model]; ok {
		info = &modelInfo
	} else {
		// 未知模型，使用默认信息
		info = &ModelInfo{
			ID:                cfg.Model,
			Name:              cfg.Model,
			Provider:          ProviderModelScope,
			SupportsTools:     true,
			SupportsStreaming: true,
		}
	}

	return &ModelScopeModel{
		OpenAIModel: openaiModel,
		config:      cfg,
		modelInfo:   info,
	}, nil
}

// Name 返回模型标识符
func (m *ModelScopeModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持函数/工具调用
func (m *ModelScopeModel) SupportsTools() bool {
	if m.modelInfo != nil {
		return m.modelInfo.SupportsTools
	}
	return true // 默认支持
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
func (m *ModelScopeModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
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
func (m *ModelScopeModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	// 直接委托给嵌入的 OpenAIModel
	return m.OpenAIModel.CallStream(ctx, messages, opts...)
}

// GetModelInfo 返回模型的能力信息
func (m *ModelScopeModel) GetModelInfo() *ModelInfo {
	return m.modelInfo
}

// GetModelScopeModelInfo 获取指定模型的能力信息
func GetModelScopeModelInfo(model string) *ModelInfo {
	if info, ok := modelscopeModelInfo[model]; ok {
		return &info
	}
	return nil
}

// ListModelScopeModels 返回所有支持的魔搭社区模型列表
func ListModelScopeModels() []ModelInfo {
	models := make([]ModelInfo, 0, len(modelscopeModelInfo))
	for _, info := range modelscopeModelInfo {
		models = append(models, info)
	}
	return models
}

// 确保 ModelScopeModel 实现 Model 接口
var _ Model = (*ModelScopeModel)(nil)
