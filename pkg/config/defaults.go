package config

import "github.com/spf13/viper"

// getDefaultConfig 返回默认配置
// 通过 viper 反序列化 setDefaults 设置的默认值，确保默认值只维护一处
func getDefaultConfig() *Config {
	v := viper.New()
	setDefaults(v)

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		// 如果 Unmarshal 失败，返回硬编码的最低限度默认值
		return &Config{
			Model:        "gpt-3.5-turbo",
			OpenAIAPIBase: "https://api.openai.com/v1",
			WebPort:      9899,
		}
	}
	return cfg
}

// setDefaults 设置 viper 默认值
func setDefaults(v *viper.Viper) {
	// 核心配置
	v.SetDefault("model", "gpt-3.5-turbo")
	v.SetDefault("bot_type", "")
	v.SetDefault("channel_type", "")

	// OpenAI 配置
	v.SetDefault("open_ai_api_key", "")
	v.SetDefault("open_ai_api_base", "https://api.openai.com/v1")

	// Agent 配置
	v.SetDefault("agent", false)
	v.SetDefault("agent_workspace", "~/cow")
	v.SetDefault("agent_max_context_tokens", 50000)
	v.SetDefault("agent_max_context_turns", 30)
	v.SetDefault("agent_max_steps", 15)

	// 服务配置
	v.SetDefault("web_port", 9899)
	v.SetDefault("debug", false)

	// Claude 配置
	v.SetDefault("claude_api_key", "")
	v.SetDefault("claude_api_base", "https://api.anthropic.com/v1")

	// Gemini 配置
	v.SetDefault("gemini_api_key", "")
	v.SetDefault("gemini_api_base", "https://generativelanguage.googleapis.com")

	// 代理配置
	v.SetDefault("proxy", "")

	// 会话配置
	v.SetDefault("expires_in_seconds", 3600)
	v.SetDefault("character_desc", "你是ChatGPT, 一个由OpenAI训练的大型语言模型, 你旨在回答并解决人们的任何问题，并且可以使用多种语言与人交流。")
	v.SetDefault("conversation_max_tokens", 1000)

	// 流式输出配置
	v.SetDefault("stream_output", true)

	// 安全配置
	v.SetDefault("sync_to_env", true) // 默认启用以保持向后兼容

	// ChatGPT API 参数
	v.SetDefault("temperature", 0.9)
	v.SetDefault("top_p", 1.0)
	v.SetDefault("frequency_penalty", 0.0)
	v.SetDefault("presence_penalty", 0.0)
	v.SetDefault("request_timeout", 180)
	v.SetDefault("timeout", 120)

	// 单聊配置
	v.SetDefault("single_chat_prefix", []string{"bot", "@bot"})
	v.SetDefault("single_chat_reply_prefix", "[bot] ")
	v.SetDefault("single_chat_reply_suffix", "")

	// 群聊配置
	v.SetDefault("group_chat_prefix", []string{"@bot"})
	v.SetDefault("group_chat_reply_prefix", "")
	v.SetDefault("group_chat_reply_suffix", "")
	v.SetDefault("group_chat_keyword", []string{})
	v.SetDefault("group_name_white_list", []string{"ChatGPT测试群", "ChatGPT测试群2"})
	v.SetDefault("group_name_keyword_white_list", []string{})
	v.SetDefault("no_need_at", false)
	v.SetDefault("group_at_off", false)
	v.SetDefault("group_shared_session", false)

	// 用户黑名单
	v.SetDefault("nick_name_black_list", []string{})

	// 图片生成配置
	v.SetDefault("text_to_image", "dall-e-2")
	v.SetDefault("image_create_size", "256x256")
	v.SetDefault("image_proxy", true)

	// 语音配置
	v.SetDefault("speech_recognition", true)
	v.SetDefault("voice_reply_voice", false)
	v.SetDefault("voice_to_text", "openai")
	v.SetDefault("text_to_voice", "openai")
	v.SetDefault("text_to_voice_model", "tts-1")
	v.SetDefault("tts_voice_id", "alloy")

	// 其他平台 API 配置
	v.SetDefault("zhipu_ai_api_key", "")
	v.SetDefault("zhipu_ai_api_base", "https://open.bigmodel.cn/api/paas/v4")
	v.SetDefault("moonshot_api_key", "")
	v.SetDefault("moonshot_base_url", "https://api.moonshot.cn/v1")
	v.SetDefault("minimax_api_key", "")
	v.SetDefault("minimax_group_id", "")
	v.SetDefault("minimax_base_url", "")

	// LinkAI 配置
	v.SetDefault("use_linkai", false)
	v.SetDefault("linkai_api_key", "")
	v.SetDefault("linkai_app_code", "")
	v.SetDefault("linkai_api_base", "https://api.link-ai.tech")

	// 飞书配置
	v.SetDefault("feishu_port", 80)
	v.SetDefault("feishu_app_id", "")
	v.SetDefault("feishu_app_secret", "")
	v.SetDefault("feishu_token", "")
	v.SetDefault("feishu_bot_name", "")
	v.SetDefault("feishu_event_mode", "websocket")
	v.SetDefault("lark_cli_path", "lark-cli")

	// 钉钉配置
	v.SetDefault("dingtalk_client_id", "")
	v.SetDefault("dingtalk_client_secret", "")
	v.SetDefault("dingtalk_card_enabled", false)

	// 微信配置
	v.SetDefault("weixin_token", "")
	v.SetDefault("weixin_base_url", "https://ilinkai.weixin.qq.com")

	// 微信公众号配置
	v.SetDefault("wechatmp_token", "")
	v.SetDefault("wechatmp_port", 8080)
	v.SetDefault("wechatmp_app_id", "")
	v.SetDefault("wechatmp_app_secret", "")

	// 数据目录
	v.SetDefault("appdata_dir", "")

	// 插件配置
	v.SetDefault("plugin_trigger_prefix", "$")

	// 清除记忆命令
	v.SetDefault("clear_memory_commands", []string{"#清除记忆"})

	// Pair 配对系统
	v.SetDefault("pair_enabled", true)
	v.SetDefault("pair_cleanup_interval", 30)

	// Admin 配置
	v.SetDefault("admin.enabled", true)
	v.SetDefault("admin.host", "0.0.0.0")
	v.SetDefault("admin.port", 31415)
	v.SetDefault("admin.username", "admin")
}
