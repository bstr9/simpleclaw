// Package llm 提供与各种 LLM 提供商交互的统一接口。
// moonshot.go 实现 Moonshot (Kimi) API 客户端。
package llm

import (
	"fmt"
)

// Moonshot API 基础 URL
const (
	MoonshotBaseURL = "https://api.moonshot.cn/v1"
)

// MoonshotModel 实现 Moonshot (Kimi) API 模型。
// Moonshot API 完全兼容 OpenAI 格式，因此复用 OpenAI 客户端实现。
type MoonshotModel struct {
	*OpenAIModel
	config ModelConfig
}

// NewMoonshotModel 创建一个新的 Moonshot 模型实例。
func NewMoonshotModel(cfg ModelConfig) (*MoonshotModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key 是必需的")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("model 是必需的")
	}

	// 设置默认 API Base URL
	if cfg.APIBase == "" {
		cfg.APIBase = MoonshotBaseURL
	}

	// 设置 provider 标识
	cfg.Provider = ProviderMoonshot

	// 设置默认模型名称
	if cfg.ModelName == "" {
		cfg.ModelName = cfg.Model
	}

	// 创建底层 OpenAI 兼容客户端
	openaiModel, err := NewOpenAIModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 Moonshot 客户端失败: %w", err)
	}

	return &MoonshotModel{
		OpenAIModel: openaiModel,
		config:      cfg,
	}, nil
}

// Name 返回模型标识符
func (m *MoonshotModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持工具调用
// Moonshot 模型支持 tool calling
func (m *MoonshotModel) SupportsTools() bool {
	return true
}

// 确保 MoonshotModel 实现 Model 接口
var _ Model = (*MoonshotModel)(nil)
