// Package llm 提供与各种 LLM 提供商交互的统一接口。
// zhipu.go 实现智谱 AI GLM 系列模型支持。
package llm

import (
	"context"
	"fmt"
)

// ZhipuModel 实现智谱 AI GLM 系列模型的支持
// 智谱 API 兼容 OpenAI 格式，内部使用 OpenAI 兼容客户端
// 支持的模型包括: glm-4, glm-4-flash, glm-4-plus, glm-4-air, glm-4-long,
// glm-4.7-thinking, glm-5 等
type ZhipuModel struct {
	*OpenAIModel // 组合 OpenAI 兼容客户端
	config       ModelConfig
}

// 智谱 API 默认配置
const (
	// ZhipuAPIBaseURL 智谱 AI API 基础 URL
	ZhipuAPIBaseURL = "https://open.bigmodel.cn/api/paas/v4"
	// ZhipuDefaultModel 智谱默认模型
	ZhipuDefaultModel = "glm-4-flash"
)

// 智谱支持的模型列表及其能力
var zhipuModels = map[string]ModelInfo{
	"glm-4": {
		ID:                "glm-4",
		Name:              "GLM-4",
		Provider:          ProviderZhipu,
		ContextWindow:     128000,
		SupportsVision:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	ZhipuDefaultModel: {
		ID:                ZhipuDefaultModel,
		Name:              "GLM-4-Flash",
		Provider:          ProviderZhipu,
		ContextWindow:     128000,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	"glm-4-plus": {
		ID:                "glm-4-plus",
		Name:              "GLM-4-Plus",
		Provider:          ProviderZhipu,
		ContextWindow:     128000,
		SupportsVision:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	"glm-4-air": {
		ID:                "glm-4-air",
		Name:              "GLM-4-Air",
		Provider:          ProviderZhipu,
		ContextWindow:     128000,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	"glm-4-airx": {
		ID:                "glm-4-airx",
		Name:              "GLM-4-AirX",
		Provider:          ProviderZhipu,
		ContextWindow:     8000,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	"glm-4-long": {
		ID:                "glm-4-long",
		Name:              "GLM-4-Long",
		Provider:          ProviderZhipu,
		ContextWindow:     1000000,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	"glm-4v": {
		ID:                "glm-4v",
		Name:              "GLM-4V",
		Provider:          ProviderZhipu,
		ContextWindow:     2000,
		SupportsVision:    true,
		SupportsTools:     false,
		SupportsStreaming: true,
	},
	"glm-4.7-thinking": {
		ID:                "glm-4.7-thinking",
		Name:              "GLM-4.7 Thinking",
		Provider:          ProviderZhipu,
		ContextWindow:     131072,
		SupportsVision:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	"glm-5": {
		ID:                "glm-5",
		Name:              "GLM-5",
		Provider:          ProviderZhipu,
		ContextWindow:     131072,
		SupportsVision:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	"glm-z1-air": {
		ID:                "glm-z1-air",
		Name:              "GLM-Z1-Air",
		Provider:          ProviderZhipu,
		ContextWindow:     131072,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	"glm-z1-airx": {
		ID:                "glm-z1-airx",
		Name:              "GLM-Z1-AirX",
		Provider:          ProviderZhipu,
		ContextWindow:     8192,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	"glm-z1-flash": {
		ID:                "glm-z1-flash",
		Name:              "GLM-Z1-Flash",
		Provider:          ProviderZhipu,
		ContextWindow:     131072,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
}

// NewZhipuModel 创建智谱 AI 模型实例
func NewZhipuModel(cfg ModelConfig) (*ZhipuModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("智谱 AI API 密钥不能为空")
	}

	if cfg.Model == "" {
		cfg.Model = ZhipuDefaultModel
	}

	if cfg.APIBase == "" {
		cfg.APIBase = ZhipuAPIBaseURL
	}

	cfg.Provider = ProviderZhipu

	openaiModel, err := NewOpenAIModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建智谱模型失败: %w", err)
	}

	return &ZhipuModel{
		OpenAIModel: openaiModel,
		config:      cfg,
	}, nil
}

// Name 返回模型名称
func (m *ZhipuModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持工具调用
func (m *ZhipuModel) SupportsTools() bool {
	// 检查模型是否在已知模型列表中
	if info, ok := zhipuModels[m.config.Model]; ok {
		return info.SupportsTools
	}
	// 默认支持工具调用
	return true
}

// Call 执行同步对话调用
func (m *ZhipuModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	return m.OpenAIModel.Call(ctx, messages, opts...)
}

// CallStream 执行流式对话调用
func (m *ZhipuModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	return m.OpenAIModel.CallStream(ctx, messages, opts...)
}

// GetModelInfo 获取模型信息
func (m *ZhipuModel) GetModelInfo() *ModelInfo {
	if info, ok := zhipuModels[m.config.Model]; ok {
		return &info
	}
	// 返回默认模型信息
	return &ModelInfo{
		ID:                m.config.Model,
		Name:              m.config.Model,
		Provider:          ProviderZhipu,
		SupportsTools:     true,
		SupportsStreaming: true,
	}
}

// ListZhipuModels 返回所有支持的智谱模型列表
func ListZhipuModels() []ModelInfo {
	models := make([]ModelInfo, 0, len(zhipuModels))
	for _, info := range zhipuModels {
		models = append(models, info)
	}
	return models
}

var _ Model = (*ZhipuModel)(nil)
