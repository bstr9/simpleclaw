// Package config 提供配置管理功能，支持 JSON 文件加载和环境变量覆盖
package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// Config 应用配置结构体
type Config struct {
	// 核心配置
	Model       string `mapstructure:"model"`
	ModelName   string `mapstructure:"model_name"`
	BotType     string `mapstructure:"bot_type"`
	ChannelType string `mapstructure:"channel_type"`

	// OpenAI 配置
	OpenAIAPIKey  string `mapstructure:"open_ai_api_key"`
	OpenAIAPIBase string `mapstructure:"open_ai_api_base"`

	// Agent 模式配置
	Agent                 bool   `mapstructure:"agent"`
	AgentWorkspace        string `mapstructure:"agent_workspace"`
	AgentMaxContextTokens int    `mapstructure:"agent_max_context_tokens"`
	AgentMaxContextTurns  int    `mapstructure:"agent_max_context_turns"`
	AgentMaxSteps         int    `mapstructure:"agent_max_steps"`

	// 工具配置（参考 openclaw 配置结构）
	Tools *ToolsConfig `mapstructure:"tools"`

	// 服务配置
	WebPort int  `mapstructure:"web_port"`
	Debug   bool `mapstructure:"debug"`

	// API Server 配置
	APIServer *APIServerConfig `mapstructure:"api_server"`

	// Claude 配置
	ClaudeAPIKey  string `mapstructure:"claude_api_key"`
	ClaudeAPIBase string `mapstructure:"claude_api_base"`

	// Gemini 配置
	GeminiAPIKey  string `mapstructure:"gemini_api_key"`
	GeminiAPIBase string `mapstructure:"gemini_api_base"`

	// 代理配置
	Proxy string `mapstructure:"proxy"`

	// 会话配置
	ExpiresInSeconds      int    `mapstructure:"expires_in_seconds"`
	CharacterDesc         string `mapstructure:"character_desc"`
	ConversationMaxTokens int    `mapstructure:"conversation_max_tokens"`

	// 流式输出配置
	StreamOutput bool `mapstructure:"stream_output"`

	// 安全配置
	// SyncToEnv 控制是否将 API 密钥同步到环境变量
	// 默认启用以保持向后兼容，生产环境建议关闭
	SyncToEnv bool `mapstructure:"sync_to_env"`

	// ChatGPT API 参数
	Temperature      float64 `mapstructure:"temperature"`
	TopP             float64 `mapstructure:"top_p"`
	FrequencyPenalty float64 `mapstructure:"frequency_penalty"`
	PresencePenalty  float64 `mapstructure:"presence_penalty"`
	RequestTimeout   int     `mapstructure:"request_timeout"`
	Timeout          int     `mapstructure:"timeout"`

	// 单聊配置
	SingleChatPrefix      []string `mapstructure:"single_chat_prefix"`
	SingleChatReplyPrefix string   `mapstructure:"single_chat_reply_prefix"`
	SingleChatReplySuffix string   `mapstructure:"single_chat_reply_suffix"`

	// 群聊配置
	GroupChatPrefix           []string `mapstructure:"group_chat_prefix"`
	GroupChatReplyPrefix      string   `mapstructure:"group_chat_reply_prefix"`
	GroupChatReplySuffix      string   `mapstructure:"group_chat_reply_suffix"`
	GroupChatKeyword          []string `mapstructure:"group_chat_keyword"`
	GroupNameWhiteList        []string `mapstructure:"group_name_white_list"`
	GroupNameKeywordWhiteList []string `mapstructure:"group_name_keyword_white_list"`
	NoNeedAt                  bool     `mapstructure:"no_need_at"`
	GroupAtOff                bool     `mapstructure:"group_at_off"`
	GroupSharedSession        bool     `mapstructure:"group_shared_session"`

	// 用户黑名单
	NickNameBlackList []string `mapstructure:"nick_name_black_list"`

	// 图片生成配置
	TextToImage     string `mapstructure:"text_to_image"`
	ImageCreateSize string `mapstructure:"image_create_size"`
	ImageProxy      bool   `mapstructure:"image_proxy"`

	// 语音配置
	SpeechRecognition bool   `mapstructure:"speech_recognition"`
	VoiceReplyVoice   bool   `mapstructure:"voice_reply_voice"`
	VoiceToText       string `mapstructure:"voice_to_text"`
	TextToVoice       string `mapstructure:"text_to_voice"`
	TextToVoiceModel  string `mapstructure:"text_to_voice_model"`
	TTSVoiceID        string `mapstructure:"tts_voice_id"`

	// DeepSeek 配置
	DeepSeekAPIKey  string `mapstructure:"deepseek_api_key"`
	DeepSeekAPIBase string `mapstructure:"deepseek_api_base"`

	// 其他平台 API 配置
	ZhipuAIAPIKey   string `mapstructure:"zhipu_ai_api_key"`
	ZhipuAIAPIBase  string `mapstructure:"zhipu_ai_api_base"`
	MoonshotAPIKey  string `mapstructure:"moonshot_api_key"`
	MoonshotBaseURL string `mapstructure:"moonshot_base_url"`
	MinimaxAPIKey   string `mapstructure:"minimax_api_key"`
	MinimaxGroupID  string `mapstructure:"minimax_group_id"`
	MinimaxBaseURL  string `mapstructure:"minimax_base_url"`

	// LinkAI 配置
	UseLinkAI     bool   `mapstructure:"use_linkai"`
	LinkAIAPIKey  string `mapstructure:"linkai_api_key"`
	LinkAIAppCode string `mapstructure:"linkai_app_code"`
	LinkAIAPIBase string `mapstructure:"linkai_api_base"`

	// 飞书配置
	FeishuPort      int    `mapstructure:"feishu_port"`
	FeishuAppID     string `mapstructure:"feishu_app_id"`
	FeishuAppSecret string `mapstructure:"feishu_app_secret"`
	FeishuToken     string `mapstructure:"feishu_token"`
	FeishuBotName   string `mapstructure:"feishu_bot_name"`
	FeishuEventMode string `mapstructure:"feishu_event_mode"`
	// LarkCLIPath lark-cli 可执行文件路径（默认 "lark-cli"）
	LarkCLIPath string `mapstructure:"lark_cli_path"`

	// 钉钉配置
	DingtalkClientID     string `mapstructure:"dingtalk_client_id"`
	DingtalkClientSecret string `mapstructure:"dingtalk_client_secret"`

	// 微信配置
	WeixinToken string `mapstructure:"weixin_token"`

	// 微信公众号配置
	WechatmpToken     string `mapstructure:"wechatmp_token"`
	WechatmpPort      int    `mapstructure:"wechatmp_port"`
	WechatmpAppID     string `mapstructure:"wechatmp_app_id"`
	WechatmpAppSecret string `mapstructure:"wechatmp_app_secret"`

	// 数据目录
	AppdataDir string `mapstructure:"appdata_dir"`

	// 插件配置
	PluginTriggerPrefix string   `mapstructure:"plugin_trigger_prefix"`
	PluginEnabled       []string `mapstructure:"plugin_enabled"`

	// 清除记忆命令
	ClearMemoryCommands []string `mapstructure:"clear_memory_commands"`

	// 百度文心配置
	BaiduAPIKey    string `mapstructure:"baidu_api_key"`
	BaiduSecretKey string `mapstructure:"baidu_secret_key"`

	// 讯飞星火配置
	XunfeiAppID     string `mapstructure:"xunfei_app_id"`
	XunfeiAPIKey    string `mapstructure:"xunfei_api_key"`
	XunfeiAPISecret string `mapstructure:"xunfei_api_secret"`

	// QQ 渠道配置
	QQWebSocketURL string `mapstructure:"qq_websocket_url"`
	QQAccessToken  string `mapstructure:"qq_access_token"`

	// 企业微信配置
	WecomCorpID         string `mapstructure:"wecom_corp_id"`
	WecomAgentID        int    `mapstructure:"wecom_agent_id"`
	WecomSecret         string `mapstructure:"wecom_secret"`
	WecomToken          string `mapstructure:"wecom_token"`
	WecomEncodingAESKey string `mapstructure:"wecom_encoding_aes_key"`

	// Agent Memory 配置
	MemoryType         string `mapstructure:"memory_type"`
	MemoryMaxTokens    int    `mapstructure:"memory_max_tokens"`
	MemorySummaryModel string `mapstructure:"memory_summary_model"`

	// Pair 配对系统配置
	PairEnabled         bool `mapstructure:"pair_enabled"`
	PairCleanupInterval int  `mapstructure:"pair_cleanup_interval"`

	// Admin 管理后台配置
	Admin *AdminConfig `mapstructure:"admin"`
}

type AdminConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Host          string `mapstructure:"host"`
	Port          int    `mapstructure:"port"`
	Username      string `mapstructure:"username"`
	PasswordHash  string `mapstructure:"password_hash"`
	SessionSecret string `mapstructure:"session_secret"`
	StaticDir     string `mapstructure:"static_dir"`
}

// 全局配置实例
var (
	cfg   *Config
	cfgMu sync.RWMutex
)

// Load 从配置文件加载配置
// configPath: 配置文件路径，默认为 ./config.json
// 注意：Load 可以多次调用（不同于 sync.Once），
// 但只有第一次成功加载的配置生效，后续调用会被忽略。
func Load(configPath ...string) error {
	cfgMu.RLock()
	if cfg != nil {
		cfgMu.RUnlock()
		return nil
	}
	cfgMu.RUnlock()

	// 慢路径：需要加载配置
	cfgMu.Lock()
	defer cfgMu.Unlock()

	// 双重检查：可能另一个 goroutine 已经加载
	if cfg != nil {
		return nil
	}

	newCfg := &Config{}
	if err := loadConfig(newCfg, configPath...); err != nil {
		return err
	}
	cfg = newCfg
	return nil
}

// loadConfig 实际加载配置的逻辑
func loadConfig(c *Config, configPath ...string) error {
	v := viper.New()

	// 设置配置文件
	path := "./config.json"
	if len(configPath) > 0 && configPath[0] != "" {
		path = configPath[0]
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 尝试模板文件
		templatePath := "./config-template.json"
		if _, err := os.Stat(templatePath); err == nil {
			path = templatePath
		}
	}

	v.SetConfigFile(path)
	v.SetConfigType("json")

	// 设置默认值
	setDefaults(v)

	// 环境变量配置
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	// 绑定环境变量（snake_case 到大写下划线格式）
	bindEnvVars(v)

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置到结构体
	if err := v.Unmarshal(c); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	// 设置配置到环境变量（供子进程使用）
	syncToEnv(c)

	return nil
}

// bindEnvVars 绑定环境变量，支持 snake_case 键名转大写下划线
func bindEnvVars(v *viper.Viper) {
	envBindings := []string{
		// OpenAI
		"open_ai_api_key",
		"open_ai_api_base",
		// Agent
		"agent",
		"agent_workspace",
		"agent_max_context_tokens",
		"agent_max_context_turns",
		"agent_max_steps",
		// 模型
		"model",
		"bot_type",
		"channel_type",
		// Claude
		"claude_api_key",
		"claude_api_base",
		// Gemini
		"gemini_api_key",
		"gemini_api_base",
		// 其他配置
		"web_port",
		"debug",
		"proxy",
		// 飞书
		"feishu_app_id",
		"feishu_app_secret",
		// 钉钉
		"dingtalk_client_id",
		"dingtalk_client_secret",
		// 微信
		"weixin_token",
		"wechatmp_app_id",
		"wechatmp_app_secret",
		// 其他平台
		"zhipu_ai_api_key",
		"moonshot_api_key",
		"minimax_api_key",
		"linkai_api_key",
	}

	for _, key := range envBindings {
		envKey := strings.ToUpper(key)
		v.BindEnv(key, envKey)
	}
}

// syncToEnv 同步配置到环境变量（供子进程如 skill 脚本使用）
// 注意：此功能会将 API 密钥写入环境变量，存在安全风险
// 可通过配置 sync_to_env: false 关闭
func syncToEnv(c *Config) {
	// 默认启用以保持向后兼容
	if !c.SyncToEnv {
		return
	}

	envMappings := map[string]string{
		"OPENAI_API_KEY":         c.OpenAIAPIKey,
		"OPENAI_API_BASE":        c.OpenAIAPIBase,
		"CLAUDE_API_KEY":         c.ClaudeAPIKey,
		"CLAUDE_API_BASE":        c.ClaudeAPIBase,
		"GEMINI_API_KEY":         c.GeminiAPIKey,
		"GEMINI_API_BASE":        c.GeminiAPIBase,
		"MINIMAX_API_KEY":        c.MinimaxAPIKey,
		"MINIMAX_BASE_URL":       c.MinimaxBaseURL,
		"ZHIPU_AI_API_KEY":       c.ZhipuAIAPIKey,
		"ZHIPU_AI_API_BASE":      c.ZhipuAIAPIBase,
		"MOONSHOT_API_KEY":       c.MoonshotAPIKey,
		"MOONSHOT_API_BASE":      c.MoonshotBaseURL,
		"LINKAI_API_KEY":         c.LinkAIAPIKey,
		"LINKAI_API_BASE":        c.LinkAIAPIBase,
		"FEISHU_APP_ID":          c.FeishuAppID,
		"FEISHU_APP_SECRET":      c.FeishuAppSecret,
		"DINGTALK_CLIENT_ID":     c.DingtalkClientID,
		"DINGTALK_CLIENT_SECRET": c.DingtalkClientSecret,
		"WECHATMP_APP_ID":        c.WechatmpAppID,
		"WECHATMP_APP_SECRET":    c.WechatmpAppSecret,
		"WEIXIN_TOKEN":           c.WeixinToken,
	}

	for envKey, value := range envMappings {
		if value != "" && os.Getenv(envKey) == "" {
			os.Setenv(envKey, value)
		}
	}
}

// Get 获取全局配置实例（线程安全）
func Get() *Config {
	cfgMu.RLock()
	c := cfg
	cfgMu.RUnlock()
	if c != nil {
		return c
	}
	// 慢路径：需要初始化默认配置
	cfgMu.Lock()
	if cfg == nil {
		cfg = getDefaultConfig()
	}
	c = cfg
	cfgMu.Unlock()
	return c
}

// Reload 重新加载配置
func Reload(configPath ...string) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	newCfg := &Config{}
	if err := loadConfig(newCfg, configPath...); err != nil {
		return err
	}
	cfg = newCfg
	return nil
}

// Set 更新配置（线程安全）
// 主要用于测试环境设置测试配置
func Set(newCfg *Config) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	cfg = newCfg
}

// IsAgentEnabled 检查是否启用 Agent 模式
func (c *Config) IsAgentEnabled() bool {
	return c.Agent
}

// GetModel 获取模型名称
func (c *Config) GetModel() string {
	return c.Model
}

// GetChannelType 获取通道类型
func (c *Config) GetChannelType() string {
	return c.ChannelType
}

// GetOpenAIAPIKey 获取 OpenAI API Key（敏感信息）
func (c *Config) GetOpenAIAPIKey() string {
	return c.OpenAIAPIKey
}

// GetWorkspace 获取 Agent 工作空间路径
func (c *Config) GetWorkspace() string {
	if c.AgentWorkspace == "" {
		return "~/cow"
	}
	return c.AgentWorkspace
}

// IsDebugEnabled 检查是否开启调试模式
func (c *Config) IsDebugEnabled() bool {
	return c.Debug
}

// GetTools 获取工具配置
func (c *Config) GetTools() *ToolsConfig {
	if c.Tools == nil {
		return &ToolsConfig{}
	}
	return c.Tools
}

// GetWebSearch 获取网络搜索配置
func (c *Config) GetWebSearch() *WebSearchConfig {
	tools := c.GetTools()
	if tools.Web == nil {
		return nil
	}
	return tools.Web.Search
}

// GetWebFetch 获取网页获取配置
func (c *Config) GetWebFetch() *WebFetchConfig {
	tools := c.GetTools()
	if tools.Web == nil {
		return nil
	}
	return tools.Web.Fetch
}

// IsWebSearchEnabled 检查网络搜索是否启用
// DuckDuckGo 免费，默认启用
func (c *Config) IsWebSearchEnabled() bool {
	search := c.GetWebSearch()
	if search == nil {
		return true // DuckDuckGo 默认启用
	}
	return search.IsSearchEnabled()
}

// IsWebFetchEnabled 检查网页获取是否启用
func (c *Config) IsWebFetchEnabled() bool {
	fetch := c.GetWebFetch()
	if fetch == nil {
		return true // 默认启用
	}
	return fetch.IsFetchEnabled()
}

// MaskSensitive 返回脱敏后的配置（用于日志打印）
func (c *Config) MaskSensitive() map[string]interface{} {
	return map[string]interface{}{
		"model":            c.Model,
		"bot_type":         c.BotType,
		"channel_type":     c.ChannelType,
		"open_ai_api_key":  maskKey(c.OpenAIAPIKey),
		"open_ai_api_base": c.OpenAIAPIBase,
		"agent":            c.Agent,
		"agent_workspace":  c.AgentWorkspace,
		"agent_max_steps":  c.AgentMaxSteps,
		"web_port":         c.WebPort,
		"debug":            c.Debug,
	}
}

// APIServerConfig API 服务配置
type APIServerConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	APIKey     string `mapstructure:"api_key"`
	EnableCORS bool   `mapstructure:"enable_cors"`
	RateLimit  int    `mapstructure:"rate_limit"`
}

// IsAPIServerEnabled 检查 API Server 是否启用
func (c *Config) IsAPIServerEnabled() bool {
	if c.APIServer == nil {
		return false
	}
	return c.APIServer.Enabled
}

// GetAPIServerConfig 获取 API Server 配置，返回默认值
func (c *Config) GetAPIServerConfig() *APIServerConfig {
	if c.APIServer == nil {
		return &APIServerConfig{
			Enabled:    false,
			Host:       "0.0.0.0",
			Port:       8080,
			EnableCORS: true,
			RateLimit:  60,
		}
	}
	return c.APIServer
}

// maskKey 对 API Key 进行脱敏处理
func maskKey(key string) string {
	if len(key) <= 6 {
		return "***"
	}
	return key[:3] + "*****" + key[len(key)-3:]
}

func (c *Config) GetAdminConfig() *AdminConfig {
	if c.Admin == nil {
		return &AdminConfig{
			Enabled: true,
			Host:    "0.0.0.0",
			Port:    31415,
		}
	}
	return c.Admin
}

func (c *Config) IsAdminEnabled() bool {
	cfg := c.GetAdminConfig()
	return cfg.Enabled
}
