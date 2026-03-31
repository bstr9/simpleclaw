// Package llm 提供与各种 LLM 提供商交互的统一接口。
// chatgpt.go 实现 ChatGPT Web 模型，支持通过 OpenAI API 兼容方式调用。
package llm

import (
	"fmt"
)

// ChatGPT 默认配置
const (
	DefaultChatGPTAPIBase = "https://api.openai.com/v1"
	DefaultChatGPTModel   = "gpt-3.5-turbo"
)

// ChatGPTModel 实现 ChatGPT Web 模型
// 支持通过 OpenAI API 兼容方式调用，复用 OpenAI 客户端实现
type ChatGPTModel struct {
	*OpenAIModel
	config ModelConfig
}

// NewChatGPTModel 创建新的 ChatGPT 模型实例
func NewChatGPTModel(cfg ModelConfig) (*ChatGPTModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key 是必需的")
	}
	if cfg.Model == "" {
		cfg.Model = DefaultChatGPTModel
	}

	// 设置 API Base URL
	if cfg.APIBase == "" {
		cfg.APIBase = DefaultChatGPTAPIBase
	}

	// 设置 provider 标识
	cfg.Provider = ProviderOpenAI

	// 设置默认模型名称
	if cfg.ModelName == "" {
		cfg.ModelName = cfg.Model
	}

	// 创建底层 OpenAI 兼容客户端
	openaiModel, err := NewOpenAIModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 ChatGPT 客户端失败: %w", err)
	}

	return &ChatGPTModel{
		OpenAIModel: openaiModel,
		config:      cfg,
	}, nil
}

// Name 返回模型标识符
func (m *ChatGPTModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持工具调用
func (m *ChatGPTModel) SupportsTools() bool {
	return isChatGPTModelSupportsTools(m.config.Model)
}

// isChatGPTModelSupportsTools 检查指定模型是否支持工具调用
func isChatGPTModelSupportsTools(model string) bool {
	unsupportedModels := []string{
		"gpt-3.5-turbo-instruct",
	}
	for _, unsupported := range unsupportedModels {
		if model == unsupported {
			return false
		}
	}
	return true
}

// 确保 ChatGPTModel 实现 Model 接口
var _ Model = (*ChatGPTModel)(nil)
