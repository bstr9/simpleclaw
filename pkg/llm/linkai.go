// Package llm 提供与各种 LLM 提供商交互的统一接口。
// linkai.go 实现 LinkAI 平台 API 模型客户端。
package llm

import (
	"context"
	"fmt"
)

// LinkAI API 基础 URL
const (
	// LinkAIDefaultBaseURL 是 LinkAI 平台的默认 API 地址
	LinkAIDefaultBaseURL = "https://api.link-ai.tech/v1"
)

// LinkAI 模型标识符常量
const (
	// LinkAIGPT35Turbo GPT-3.5 Turbo 模型
	LinkAIGPT35Turbo = "gpt-3.5-turbo"
	// LinkAIGPT35Turbo16K GPT-3.5 Turbo 16K 上下文模型
	LinkAIGPT35Turbo16K = "gpt-3.5-turbo-16k"
	// LinkAIGPT4 GPT-4 模型
	LinkAIGPT4 = "gpt-4"
	// LinkAIGPT4Turbo GPT-4 Turbo 模型
	LinkAIGPT4Turbo = "gpt-4-turbo"
	// LinkAIGPT4o GPT-4o 模型
	LinkAIGPT4o = "gpt-4o"
	// LinkAIWenxin 百度文心模型
	LinkAIWenxin = "wenxin"
	// LinkAIXunfei 讯飞星火模型
	LinkAIXunfei = "xunfei"
)

// linkaiModelInfo 定义模型的能力信息
var linkaiModelInfo = map[string]ModelInfo{
	LinkAIGPT35Turbo: {
		ID:                LinkAIGPT35Turbo,
		Name:              "GPT-3.5 Turbo",
		Provider:          ProviderLinkAI,
		ContextWindow:     4096,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	LinkAIGPT35Turbo16K: {
		ID:                LinkAIGPT35Turbo16K,
		Name:              "GPT-3.5 Turbo 16K",
		Provider:          ProviderLinkAI,
		ContextWindow:     16384,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	LinkAIGPT4: {
		ID:                LinkAIGPT4,
		Name:              "GPT-4",
		Provider:          ProviderLinkAI,
		ContextWindow:     8192,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	LinkAIGPT4Turbo: {
		ID:                LinkAIGPT4Turbo,
		Name:              "GPT-4 Turbo",
		Provider:          ProviderLinkAI,
		ContextWindow:     128000,
		SupportsVision:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	LinkAIGPT4o: {
		ID:                LinkAIGPT4o,
		Name:              "GPT-4o",
		Provider:          ProviderLinkAI,
		ContextWindow:     128000,
		SupportsVision:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	LinkAIWenxin: {
		ID:                LinkAIWenxin,
		Name:              "百度文心",
		Provider:          ProviderLinkAI,
		ContextWindow:     4096,
		SupportsVision:    false,
		SupportsTools:     false,
		SupportsStreaming: true,
	},
	LinkAIXunfei: {
		ID:                LinkAIXunfei,
		Name:              "讯飞星火",
		Provider:          ProviderLinkAI,
		ContextWindow:     4096,
		SupportsVision:    false,
		SupportsTools:     false,
		SupportsStreaming: true,
	},
}

// LinkAIModel 实现 LinkAI 平台 API 的 Model 接口
// LinkAI API 兼容 OpenAI 格式，因此直接复用 OpenAIModel 实现
// 支持 streaming 和非 streaming 模式，支持知识库和应用码
type LinkAIModel struct {
	*OpenAIModel // 嵌入 OpenAIModel，复用其实现
	config       ModelConfig
	modelInfo    *ModelInfo
	// AppCode 是 LinkAI 应用码，用于指定特定的知识库或应用
	appCode string
}

// NewLinkAIModel 创建新的 LinkAI 模型实例
// LinkAI API 完全兼容 OpenAI 格式，可以直接使用 OpenAI 兼容客户端
//
// 参数说明:
//   - cfg.Model: 模型标识符，如 "gpt-3.5-turbo", "gpt-4", "wenxin" 等
//   - cfg.APIKey: LinkAI API Key
//   - cfg.APIBase: 可选，自定义 API 地址，默认使用 LinkAI 官方地址
//   - cfg.Proxy: 可选，HTTP 代理地址
//   - cfg.RequestTimeout: 可选，请求超时时间（秒）
//   - cfg.Extra["app_code"]: 可选，LinkAI 应用码
//
// 示例:
//
//	model, err := NewLinkAIModel(ModelConfig{
//	    Model:  "gpt-3.5-turbo",
//	    APIKey: "your-linkai-api-key",
//	    Extra: map[string]any{"app_code": "your-app-code"},
//	})
func NewLinkAIModel(cfg ModelConfig) (*LinkAIModel, error) {
	// 验证必要参数
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("linkai api_key 是必需的")
	}
	if cfg.Model == "" {
		// 设置默认模型
		cfg.Model = LinkAIGPT35Turbo
	}

	// 设置默认 API Base URL
	if cfg.APIBase == "" {
		cfg.APIBase = LinkAIDefaultBaseURL
	}

	// 设置默认 Provider
	if cfg.Provider == "" {
		cfg.Provider = ProviderLinkAI
	}

	// 设置默认模型名称（用户友好名称）
	if cfg.ModelName == "" {
		cfg.ModelName = cfg.Model
	}

	// 提取 app_code
	appCode := ""
	if cfg.Extra != nil {
		if ac, ok := cfg.Extra["app_code"].(string); ok {
			appCode = ac
		}
	}

	// 创建 OpenAI 兼容模型
	openaiModel, err := NewOpenAIModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 LinkAI 模型失败: %w", err)
	}

	// 获取模型信息
	var info *ModelInfo
	if modelInfo, ok := linkaiModelInfo[cfg.Model]; ok {
		info = &modelInfo
	} else {
		// 未知模型，使用默认信息
		info = &ModelInfo{
			ID:                cfg.Model,
			Name:              cfg.Model,
			Provider:          ProviderLinkAI,
			SupportsTools:     true,
			SupportsStreaming: true,
		}
	}

	return &LinkAIModel{
		OpenAIModel: openaiModel,
		config:      cfg,
		modelInfo:   info,
		appCode:     appCode,
	}, nil
}

// Name 返回模型标识符
func (m *LinkAIModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持函数/工具调用
func (m *LinkAIModel) SupportsTools() bool {
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
func (m *LinkAIModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
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
func (m *LinkAIModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	// 直接委托给嵌入的 OpenAIModel
	return m.OpenAIModel.CallStream(ctx, messages, opts...)
}

// GetModelInfo 返回模型的能力信息
func (m *LinkAIModel) GetModelInfo() *ModelInfo {
	return m.modelInfo
}

// AppCode 返回配置的应用码
func (m *LinkAIModel) AppCode() string {
	return m.appCode
}

// GetLinkAIModelInfo 获取指定模型的能力信息
func GetLinkAIModelInfo(model string) *ModelInfo {
	if info, ok := linkaiModelInfo[model]; ok {
		return &info
	}
	return nil
}

// ListLinkAIModels 返回所有支持的 LinkAI 模型列表
func ListLinkAIModels() []ModelInfo {
	models := make([]ModelInfo, 0, len(linkaiModelInfo))
	for _, info := range linkaiModelInfo {
		models = append(models, info)
	}
	return models
}

// 确保 LinkAIModel 实现 Model 接口
var _ Model = (*LinkAIModel)(nil)
