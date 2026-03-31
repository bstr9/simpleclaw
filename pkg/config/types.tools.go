// Package config 提供工具配置类型定义
// 参考 openclaw 的配置结构，保持兼容性
package config

// ToolsConfig 工具配置
type ToolsConfig struct {
	Web *WebToolsConfig `mapstructure:"web"`
}

// WebToolsConfig Web 工具配置
type WebToolsConfig struct {
	Search *WebSearchConfig `mapstructure:"search"`
	Fetch  *WebFetchConfig  `mapstructure:"fetch"`
}

// WebSearchConfig 网络搜索配置
type WebSearchConfig struct {
	// Enabled 启用网络搜索工具（存在 API Key 时默认为 true）
	Enabled *bool `mapstructure:"enabled"`

	// Provider 搜索提供商: "brave", "gemini", "grok", "kimi", "perplexity"
	Provider string `mapstructure:"provider"`

	// APIKey Brave Search API Key（回退: BRAVE_API_KEY 环境变量）
	APIKey string `mapstructure:"api_key"`

	// MaxResults 默认搜索结果数量 (1-10)
	MaxResults int `mapstructure:"max_results"`

	// TimeoutSeconds 搜索请求超时时间（秒）
	TimeoutSeconds int `mapstructure:"timeout_seconds"`

	// CacheTTLMinutes 搜索结果缓存时间（分钟）
	CacheTTLMinutes int `mapstructure:"cache_ttl_minutes"`

	// Brave Brave 搜索特定配置
	Brave *BraveSearchConfig `mapstructure:"brave"`

	// Gemini Gemini 搜索特定配置
	Gemini *GeminiSearchConfig `mapstructure:"gemini"`

	// Grok Grok (xAI) 搜索特定配置
	Grok *GrokSearchConfig `mapstructure:"grok"`

	// Kimi Kimi 搜索特定配置
	Kimi *KimiSearchConfig `mapstructure:"kimi"`

	// Perplexity Perplexity 搜索特定配置
	Perplexity *PerplexitySearchConfig `mapstructure:"perplexity"`
}

// BraveSearchConfig Brave 搜索配置
type BraveSearchConfig struct {
	// Mode 搜索模式: "web" (标准结果) 或 "llm-context" (预提取页面内容)
	Mode string `mapstructure:"mode"`
}

// GeminiSearchConfig Gemini 搜索配置
type GeminiSearchConfig struct {
	// APIKey Gemini API Key（回退: GEMINI_API_KEY 环境变量）
	APIKey string `mapstructure:"api_key"`

	// Model 用于搜索的模型（默认: "gemini-2.5-flash"）
	Model string `mapstructure:"model"`
}

// GrokSearchConfig Grok (xAI) 搜索配置
type GrokSearchConfig struct {
	// APIKey xAI API Key（回退: XAI_API_KEY 环境变量）
	APIKey string `mapstructure:"api_key"`

	// Model 使用的模型（默认: "grok-4-1-fast"）
	Model string `mapstructure:"model"`

	// InlineCitations 在响应文本中包含内联引用作为 markdown 链接
	InlineCitations bool `mapstructure:"inline_citations"`
}

// KimiSearchConfig Kimi 搜索配置
type KimiSearchConfig struct {
	// APIKey Moonshot/Kimi API Key（回退: KIMI_API_KEY 或 MOONSHOT_API_KEY 环境变量）
	APIKey string `mapstructure:"api_key"`

	// BaseURL API 请求的基础 URL（默认: "https://api.moonshot.ai/v1"）
	BaseURL string `mapstructure:"base_url"`

	// Model 使用的模型（默认: "moonshot-v1-128k"）
	Model string `mapstructure:"model"`
}

// PerplexitySearchConfig Perplexity 搜索配置
type PerplexitySearchConfig struct {
	// APIKey Perplexity API Key（回退: PERPLEXITY_API_KEY 环境变量）
	APIKey string `mapstructure:"api_key"`

	// BaseURL API 基础 URL（已弃用，仅用于旧版 Sonar/OpenRouter）
	BaseURL string `mapstructure:"base_url"`

	// Model 模型（已弃用，仅用于旧版 Sonar/OpenRouter）
	Model string `mapstructure:"model"`
}

// WebFetchConfig 网页获取配置
type WebFetchConfig struct {
	// Enabled 启用网页获取工具（默认: true）
	Enabled *bool `mapstructure:"enabled"`

	// MaxChars 返回内容的最大字符数
	MaxChars int `mapstructure:"max_chars"`

	// MaxCharsCap MaxChars 的硬性上限（默认: 50000）
	MaxCharsCap int `mapstructure:"max_chars_cap"`

	// TimeoutSeconds 获取请求超时时间（秒）
	TimeoutSeconds int `mapstructure:"timeout_seconds"`

	// CacheTTLMinutes 获取内容缓存时间（分钟）
	CacheTTLMinutes int `mapstructure:"cache_ttl_minutes"`

	// MaxRedirects 最大重定向次数（默认: 3）
	MaxRedirects int `mapstructure:"max_redirects"`

	// UserAgent 覆盖 User-Agent 头
	UserAgent string `mapstructure:"user_agent"`

	// UseReadability 使用 Readability 提取主要内容（默认: true）
	UseReadability bool `mapstructure:"use_readability"`
}

// IsSearchEnabled 检查网络搜索是否启用
// DuckDuckGo 是免费的默认搜索引擎，无需 API Key
func (c *WebSearchConfig) IsSearchEnabled() bool {
	if c == nil {
		return true // DuckDuckGo 默认启用
	}
	if c.Enabled != nil {
		return *c.Enabled
	}
	// DuckDuckGo 免费，默认启用
	return true
}

// IsFetchEnabled 检查网页获取是否启用
func (c *WebFetchConfig) IsFetchEnabled() bool {
	if c == nil {
		return true // 默认启用
	}
	if c.Enabled != nil {
		return *c.Enabled
	}
	return true
}

// GetSearchProvider 获取搜索提供商，如果未配置则自动检测
// 默认返回 duckduckgo（免费，无需 API Key）
func (c *WebSearchConfig) GetSearchProvider() string {
	if c == nil {
		return "duckduckgo"
	}
	if c.Provider != "" {
		return c.Provider
	}
	// 自动检测：根据可用的 API Key 选择提供商
	if c.APIKey != "" {
		return "brave"
	}
	if c.Gemini != nil && c.Gemini.APIKey != "" {
		return "gemini"
	}
	if c.Grok != nil && c.Grok.APIKey != "" {
		return "grok"
	}
	if c.Kimi != nil && c.Kimi.APIKey != "" {
		return "kimi"
	}
	if c.Perplexity != nil && c.Perplexity.APIKey != "" {
		return "perplexity"
	}
	// 无 API Key 时使用 DuckDuckGo（免费）
	return "duckduckgo"
}
