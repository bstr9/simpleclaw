// Package llm 提供与各种 LLM 提供商交互的统一接口。
// factory.go 提供模型工厂函数。
package llm

import (
	"fmt"
	"strings"
)

// 已知 LLM 提供商常量。
const (
	ProviderOpenAI     = "openai"
	ProviderChatGPT    = "chatgpt"
	ProviderAnthropic  = "anthropic"
	ProviderGemini     = "gemini"
	ProviderDeepSeek   = "deepseek"
	ProviderGLM        = "glm"
	ProviderQwen       = "qwen"
	ProviderMiniMax    = "minimax"
	ProviderKimi       = "kimi"
	ProviderMoonshot   = "moonshot"
	ProviderZhipu      = "zhipu"
	ProviderBaidu      = "baidu"
	ProviderDoubao     = "doubao"
	ProviderDashScope  = "dashscope"
	ProviderModelScope = "modelscope"
	ProviderLinkAI     = "linkai"
	ProviderXunfei     = "xunfei"
)

// 已知提供商的基础 URL，便于使用。
var providerBaseURLs = map[string]string{
	ProviderOpenAI:     "https://api.openai.com/v1",
	ProviderChatGPT:    "https://api.openai.com/v1",
	ProviderAnthropic:  "https://api.anthropic.com",
	ProviderDeepSeek:   "https://api.deepseek.com/v1",
	ProviderGLM:        "https://open.bigmodel.cn/api/paas/v4",
	ProviderQwen:       "https://dashscope.aliyuncs.com/compatible-mode/v1",
	ProviderMiniMax:    "https://api.minimax.chat/v1",
	ProviderKimi:       "https://api.moonshot.cn/v1",
	ProviderMoonshot:   "https://api.moonshot.cn/v1",
	ProviderZhipu:      "https://open.bigmodel.cn/api/paas/v4",
	ProviderDoubao:     "https://ark.cn-beijing.volces.com/api/v3",
	ProviderDashScope:  "https://dashscope.aliyuncs.com/api/v1",
	ProviderModelScope: "https://api-inference.modelscope.cn/v1",
	ProviderLinkAI:     "https://api.link-ai.tech/v1",
}

type modelFactory func(cfg ModelConfig) (Model, error)

var providerFactories = map[string]modelFactory{
	ProviderBaidu:      func(cfg ModelConfig) (Model, error) { return NewBaiduModel(cfg) },
	ProviderXunfei:     func(cfg ModelConfig) (Model, error) { return NewXunfeiModel(cfg) },
	ProviderAnthropic:  func(cfg ModelConfig) (Model, error) { return NewClaudeModel(cfg) },
	ProviderChatGPT:    func(cfg ModelConfig) (Model, error) { return NewChatGPTModel(cfg) },
	ProviderMiniMax:    func(cfg ModelConfig) (Model, error) { return NewMiniMaxModel(cfg) },
	ProviderGemini:     func(cfg ModelConfig) (Model, error) { return NewGeminiModel(cfg) },
	ProviderDeepSeek:   func(cfg ModelConfig) (Model, error) { return NewDeepSeekModel(cfg) },
	ProviderQwen:       func(cfg ModelConfig) (Model, error) { return NewQwenModel(cfg) },
	ProviderZhipu:      func(cfg ModelConfig) (Model, error) { return NewZhipuModel(cfg) },
	ProviderGLM:        func(cfg ModelConfig) (Model, error) { return NewZhipuModel(cfg) },
	ProviderDoubao:     func(cfg ModelConfig) (Model, error) { return NewDoubaoModel(cfg) },
	ProviderDashScope:  func(cfg ModelConfig) (Model, error) { return NewDashScopeModel(cfg) },
	ProviderModelScope: func(cfg ModelConfig) (Model, error) { return NewModelScopeModel(cfg) },
	ProviderLinkAI:     func(cfg ModelConfig) (Model, error) { return NewLinkAIModel(cfg) },
}

// NewModel 根据提供的配置创建新的 Model 实例。
// 它自动从配置中检测提供商类型，并创建相应的模型实现。
//
// 支持的功能：
// - OpenAI 及所有 OpenAI 兼容 API（DeepSeek、GLM、Qwen、MiniMax、Kimi 等）
// - 百度文心（ERNIE）模型，需要单独的 API Key 和 Secret Key
// - 从模型名称前缀自动检测提供商（如 "openai/gpt-4"、"deepseek/deepseek-chat"）
// - 自定义 API 基础 URL
// - 代理配置
//
// 使用示例：
//
//	model, err := NewModel(ModelConfig{
//	    ModelName: "gpt-4",
//	    Model:     "gpt-4",
//	    APIKey:    "sk-xxx",
//	    Provider:  "openai",
//	})
func NewModel(cfg ModelConfig) (Model, error) {
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}

	if cfg.Provider == "" {
		cfg.Provider = detectProvider(cfg.Model, cfg.APIBase)
	}

	cfg.Model = stripProviderPrefix(cfg.Model)

	if factory, ok := providerFactories[cfg.Provider]; ok {
		return factory(cfg)
	}

	if cfg.Provider == ProviderMoonshot || cfg.Provider == ProviderKimi {
		if cfg.APIBase == "" {
			cfg.APIBase = providerBaseURLs[ProviderMoonshot]
		}
		return NewMoonshotModel(cfg)
	}

	if cfg.APIBase == "" {
		if baseURL, ok := providerBaseURLs[cfg.Provider]; ok {
			cfg.APIBase = baseURL
		}
	}

	return NewOpenAIModel(cfg)
}

// detectProvider 尝试从模型名称或 API 基础 URL 检测提供商。
func detectProvider(model, apiBase string) string {
	if idx := strings.Index(model, "/"); idx > 0 {
		provider := strings.ToLower(model[:idx])
		if _, ok := providerBaseURLs[provider]; ok {
			return provider
		}
	}

	if apiBase != "" {
		apiBaseLower := strings.ToLower(apiBase)
		for provider, baseURL := range providerBaseURLs {
			if strings.Contains(apiBaseLower, strings.ToLower(baseURL)) {
				return provider
			}
		}
	}

	return ProviderOpenAI
}

func stripProviderPrefix(model string) string {
	if idx := strings.Index(model, "/"); idx > 0 {
		return model[idx+1:]
	}
	return model
}

// NewModelWithProvider 使用显式指定的提供商创建新的 Model 实例。
// 当提供商无法自动检测时很有用。
func NewModelWithProvider(provider string, cfg ModelConfig) (Model, error) {
	cfg.Provider = provider
	return NewModel(cfg)
}

// RegisterProvider 注册自定义提供商及其基础 URL。
// 允许扩展工厂以支持新的 OpenAI 兼容提供商。
func RegisterProvider(name, baseURL string) {
	providerBaseURLs[strings.ToLower(name)] = baseURL
}

// GetProviderBaseURL 返回已知提供商的基础 URL。
// 如果提供商未注册，返回空字符串。
func GetProviderBaseURL(provider string) string {
	return providerBaseURLs[strings.ToLower(provider)]
}

// ListProviders 返回所有已注册的提供商名称列表。
func ListProviders() []string {
	providers := make([]string, 0, len(providerBaseURLs))
	for p := range providerBaseURLs {
		providers = append(providers, p)
	}
	return providers
}
