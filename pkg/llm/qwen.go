// Package llm 提供与各种 LLM 提供商交互的统一接口。
// qwen.go 实现阿里通义千问 API 模型客户端。
package llm

import (
	"fmt"
)

// Qwen API 基础 URL (OpenAI 兼容模式)
const (
	// QwenDefaultBaseURL 是通义千问 OpenAI 兼容模式的默认 API 地址
	QwenDefaultBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
)

// Qwen 模型标识符常量
const (
	// QwenTurbo 通义千问 Turbo 版本，快速响应
	QwenTurbo = "qwen-turbo"
	// QwenPlus 通义千问 Plus 版本，平衡性能与成本
	QwenPlus = "qwen-plus"
	// QwenMax 通义千问 Max 版本，最强性能
	QwenMax = "qwen-max"
	// QwenMaxLongContext 通义千问 Max 长上下文版本
	QwenMaxLongContext = "qwen-max-longcontext"
	// QwenLong 通义千问长文本版本
	QwenLong = "qwen-long"
	// QwenVL 通义千问视觉语言模型
	QwenVL = "qwen-vl-plus"
	// QwenVLMax 通义千问视觉语言模型 Max 版本
	QwenVLMax = "qwen-vl-max"
	// QwenMath 通义千问数学专用模型
	QwenMath = "qwen-math-plus"
	// QwenCode 通义千问代码专用模型
	QwenCode = "qwen-coder-plus"
)

// qwenModelInfo 定义模型的能力信息
var qwenModelInfo = map[string]ModelInfo{
	QwenTurbo: {
		ID:                QwenTurbo,
		Name:              "通义千问 Turbo",
		Provider:          ProviderQwen,
		ContextWindow:     8192,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	QwenPlus: {
		ID:                QwenPlus,
		Name:              "通义千问 Plus",
		Provider:          ProviderQwen,
		ContextWindow:     32768,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	QwenMax: {
		ID:                QwenMax,
		Name:              "通义千问 Max",
		Provider:          ProviderQwen,
		ContextWindow:     32768,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	QwenMaxLongContext: {
		ID:                QwenMaxLongContext,
		Name:              "通义千问 Max 长上下文",
		Provider:          ProviderQwen,
		ContextWindow:     28000,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	QwenLong: {
		ID:                QwenLong,
		Name:              "通义千问 Long",
		Provider:          ProviderQwen,
		ContextWindow:     1000000,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	QwenVL: {
		ID:                QwenVL,
		Name:              "通义千问 VL",
		Provider:          ProviderQwen,
		ContextWindow:     8192,
		SupportsVision:    true,
		SupportsTools:     false,
		SupportsStreaming: true,
	},
	QwenVLMax: {
		ID:                QwenVLMax,
		Name:              "通义千问 VL Max",
		Provider:          ProviderQwen,
		ContextWindow:     32768,
		SupportsVision:    true,
		SupportsTools:     false,
		SupportsStreaming: true,
	},
	QwenMath: {
		ID:                QwenMath,
		Name:              "通义千问 Math",
		Provider:          ProviderQwen,
		ContextWindow:     4096,
		SupportsVision:    false,
		SupportsTools:     false,
		SupportsStreaming: true,
	},
	QwenCode: {
		ID:                QwenCode,
		Name:              "通义千问 Coder",
		Provider:          ProviderQwen,
		ContextWindow:     65536,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
}

// QwenModel 实现阿里通义千问 API 的 Model 接口
// 通义千问 API 兼容 OpenAI 格式，因此直接复用 OpenAIModel 实现
// 支持 streaming 和非 streaming 模式，支持 tool calling
type QwenModel struct {
	*OpenAIModel // 嵌入 OpenAIModel，复用其实现
	config       ModelConfig
	modelInfo    *ModelInfo
}

// NewQwenModel 创建新的通义千问模型实例
// 通义千问 API 完全兼容 OpenAI 格式，可以直接使用 OpenAI 兼容客户端
//
// 参数说明:
//   - cfg.Model: 模型标识符，如 "qwen-turbo", "qwen-plus", "qwen-max" 等
//   - cfg.APIKey: 阿里云 DashScope API Key
//   - cfg.APIBase: 可选，自定义 API 地址，默认使用阿里云 DashScope 兼容模式地址
//   - cfg.Proxy: 可选，HTTP 代理地址
//   - cfg.RequestTimeout: 可选，请求超时时间（秒）
//
// 示例:
//
//	model, err := NewQwenModel(ModelConfig{
//	    Model:  "qwen-plus",
//	    APIKey: "sk-xxx",
//	})
func NewQwenModel(cfg ModelConfig) (*QwenModel, error) {
	// 验证必要参数
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("qwen api_key is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	// 设置默认 API Base URL
	if cfg.APIBase == "" {
		cfg.APIBase = QwenDefaultBaseURL
	}

	// 设置默认 Provider
	if cfg.Provider == "" {
		cfg.Provider = ProviderQwen
	}

	// 映射模型名称
	model := mapQwenModel(cfg.Model)
	cfg.Model = model

	// 创建 OpenAI 兼容模型
	openaiModel, err := NewOpenAIModel(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create qwen model: %w", err)
	}

	// 获取模型信息
	var info *ModelInfo
	if modelInfo, ok := qwenModelInfo[model]; ok {
		info = &modelInfo
	} else {
		// 未知模型，使用默认信息
		info = &ModelInfo{
			ID:                model,
			Name:              model,
			Provider:          ProviderQwen,
			SupportsTools:     true,
			SupportsStreaming: true,
		}
	}

	return &QwenModel{
		OpenAIModel: openaiModel,
		config:      cfg,
		modelInfo:   info,
	}, nil
}

// Name 返回模型标识符
func (m *QwenModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持函数/工具调用
func (m *QwenModel) SupportsTools() bool {
	if m.modelInfo != nil {
		return m.modelInfo.SupportsTools
	}
	return true // 默认支持
}

// GetModelInfo 返回模型的能力信息
func (m *QwenModel) GetModelInfo() *ModelInfo {
	return m.modelInfo
}

// mapQwenModel 将用户输入的模型名称映射到标准模型标识符
func mapQwenModel(model string) string {
	// 模型名称映射表
	modelAliases := map[string]string{
		"qwen":            QwenTurbo,
		"qwen-turbo":      QwenTurbo,
		"qwen-plus":       QwenPlus,
		"qwen-max":        QwenMax,
		"qwen-max-long":   QwenMaxLongContext,
		"qwen-long":       QwenLong,
		"qwen-vl":         QwenVL,
		"qwen-vl-plus":    QwenVL,
		"qwen-vl-max":     QwenVLMax,
		"qwen-math":       QwenMath,
		"qwen-math-plus":  QwenMath,
		"qwen-coder":      QwenCode,
		"qwen-coder-plus": QwenCode,
		// 兼容 chatgpt-on-wechat 的命名
		"qwenapi": QwenTurbo,
	}

	if mapped, ok := modelAliases[model]; ok {
		return mapped
	}
	return model
}

// GetQwenModelInfo 获取指定模型的能力信息
func GetQwenModelInfo(model string) *ModelInfo {
	model = mapQwenModel(model)
	if info, ok := qwenModelInfo[model]; ok {
		return &info
	}
	return nil
}

// ListQwenModels 返回所有支持的通义千问模型列表
func ListQwenModels() []ModelInfo {
	models := make([]ModelInfo, 0, len(qwenModelInfo))
	for _, info := range qwenModelInfo {
		models = append(models, info)
	}
	return models
}
