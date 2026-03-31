// Package common 提供通用常量定义
package common

// HTTP 头部常量
const (
	HeaderContentType   = "Content-Type"
	HeaderAuthorization = "Authorization"
	ContentTypeJSON     = "application/json"
	AuthPrefixBearer    = "Bearer "
)

// 厂商类型常量
const (
	OpenAI        = "openai"
	ChatGPT       = "chatGPT" // 兼容性别名
	Baidu         = "baidu"
	XunFei        = "xunfei"
	ChatGPTAzure  = "chatGPTOnAzure"
	LinkAI        = "linkai"
	ClaudeAPI     = "claudeAPI"
	Qwen          = "qwen"
	QwenDashScope = "dashscope" // 新版千问(百炼)
	Gemini        = "gemini"
	ZhipuAI       = "zhipu"
	Moonshot      = "moonshot"
	MiniMax       = "minimax"
	DeepSeek      = "deepseek"
	ModelScope    = "modelscope"
	Doubao        = "doubao"
)

// Claude 模型常量 (Anthropic)
const (
	Claude3            = "claude-3-opus-20240229"
	Claude3Opus        = "claude-3-opus-latest"
	Claude3Opus0229    = "claude-3-opus-20240229"
	Claude3Sonnet      = "claude-3-sonnet-20240229"
	Claude3Haiku       = "claude-3-haiku-20240307"
	Claude35Sonnet     = "claude-3-5-sonnet-latest"
	Claude35Sonnet1022 = "claude-3-5-sonnet-20241022"
	Claude35Sonnet0620 = "claude-3-5-sonnet-20240620"
	Claude4Opus        = "claude-opus-4-0"
	Claude46Opus       = "claude-opus-4-6" // Agent推荐模型
	Claude4Sonnet      = "claude-sonnet-4-0"
	Claude45Sonnet     = "claude-sonnet-4-5" // Agent推荐模型
	Claude46Sonnet     = "claude-sonnet-4-6" // Agent推荐模型
)

// Gemini 模型常量 (Google)
const (
	GeminiPro            = "gemini-1.0-pro"
	Gemini15Flash        = "gemini-1.5-flash"
	Gemini15Pro          = "gemini-1.5-pro"
	Gemini20FlashExp     = "gemini-2.0-flash-exp" // 实验模型
	Gemini20Flash        = "gemini-2.0-flash"
	Gemini25FlashPre     = "gemini-2.5-flash-preview-05-20"
	Gemini25ProPre       = "gemini-2.5-pro-preview-05-06"
	Gemini3FlashPre      = "gemini-3-flash-preview" // Agent推荐模型
	Gemini3ProPre        = "gemini-3-pro-preview"
	Gemini31ProPre       = "gemini-3.1-pro-preview"        // Agent推荐模型
	Gemini31FlashLitePre = "gemini-3.1-flash-lite-preview" // Agent推荐模型
)

// OpenAI 模型常量
const (
	GPT35             = "gpt-3.5-turbo"
	GPT350125         = "gpt-3.5-turbo-0125"
	GPT351106         = "gpt-3.5-turbo-1106"
	GPT4              = "gpt-4"
	GPT40613          = "gpt-4-0613"
	GPT432k           = "gpt-4-32k"
	GPT432k0613       = "gpt-4-32k-0613"
	GPT4Turbo         = "gpt-4-turbo"
	GPT4TurboPreview  = "gpt-4-turbo-preview"
	GPT4Turbo0125     = "gpt-4-0125-preview"
	GPT4Turbo1106     = "gpt-4-1106-preview"
	GPT4Turbo0409     = "gpt-4-turbo-2024-04-09"
	GPT4VisionPreview = "gpt-4-vision-preview"
	GPT4o             = "gpt-4o"
	GPT4o0806         = "gpt-4o-2024-08-06"
	GPT4oMini         = "gpt-4o-mini"
	GPT41             = "gpt-4.1"
	GPT41Mini         = "gpt-4.1-mini"
	GPT41Nano         = "gpt-4.1-nano"
	GPT5              = "gpt-5"
	GPT5Mini          = "gpt-5-mini"
	GPT5Nano          = "gpt-5-nano"
	GPT54             = "gpt-5.4" // Agent推荐模型
	GPT54Mini         = "gpt-5.4-mini"
	GPT54Nano         = "gpt-5.4-nano"
	O1                = "o1-preview"
	O1Mini            = "o1-mini"
	Whisper1          = "whisper-1"
	TTS1              = "tts-1"
	TTS1HD            = "tts-1-hd"
)

// DeepSeek 模型常量
const (
	DeepSeekChat     = "deepseek-chat"     // DeepSeek-V3对话模型
	DeepSeekReasoner = "deepseek-reasoner" // DeepSeek-R1模型
)

// Qwen 模型常量 (通义千问 - 阿里云)
const (
	QwenTurbo  = "qwen-turbo"
	QwenPlus   = "qwen-plus"
	QwenMax    = "qwen-max"
	QwenLong   = "qwen-long"
	Qwen3Max   = "qwen3-max"    // Agent推荐模型
	Qwen35Plus = "qwen3.5-plus" // 多模态模型
	QwqPlus    = "qwq-plus"
)

// MiniMax 模型常量
const (
	MiniMaxM27          = "MiniMax-M2.7"
	MiniMaxM25          = "MiniMax-M2.5"
	MiniMaxM21          = "MiniMax-M2.1"
	MiniMaxM21Lightning = "MiniMax-M2.1-lightning" // 极速版
	MiniMaxM2           = "MiniMax-M2"
	MiniMaxABAB65       = "abab6.5-chat"
)

// GLM 模型常量 (智谱AI)
const (
	GLM5Turbo    = "glm-5-turbo"
	GLM5         = "glm-5"
	GLM4         = "glm-4"
	GLM4Plus     = "glm-4-plus"
	GLM4Flash    = "glm-4-flash"
	GLM4Long     = "glm-4-long"
	GLM4AllTools = "glm-4-alltools"
	GLM40520     = "glm-4-0520"
	GLM4Air      = "glm-4-air"
	GLM4AirX     = "glm-4-airx"
	GLM47        = "glm-4.7" // Agent推荐模型
)

// Kimi 模型常量 (Moonshot)
const (
	KimiK2  = "kimi-k2"
	KimiK25 = "kimi-k2.5"
)

// Doubao 模型常量 (火山引擎)
const (
	DoubaoSeed2Code = "doubao-seed-2-0-code-preview-260215"
	DoubaoSeed2Pro  = "doubao-seed-2-0-pro-260215"
	DoubaoSeed2Lite = "doubao-seed-2-0-lite-260215"
	DoubaoSeed2Mini = "doubao-seed-2-0-mini-260215"
)

// 其他模型常量
const (
	WenXin       = "wenxin"
	WenXin4      = "wenxin-4"
	LinkAI35     = "linkai-3.5"
	LinkAI4Turbo = "linkai-4-turbo"
	LinkAI4o     = "linkai-4o"
)

// 渠道类型常量
const (
	Feishu   = "feishu"
	DingTalk = "dingtalk"
	WecomBot = "wecom_bot"
	QQ       = "qq"
	Weixin   = "weixin"
)

// ModelList 所有支持的模型列表
var ModelList = []string{
	// Claude
	Claude3, Claude46Sonnet, Claude46Opus, Claude4Opus, Claude45Sonnet, Claude4Sonnet,
	Claude3Opus, Claude3Opus0229, Claude35Sonnet, Claude35Sonnet1022, Claude35Sonnet0620,
	Claude3Sonnet, Claude3Haiku,
	"claude", "claude-3-haiku", "claude-3-sonnet", "claude-3-opus", "claude-3.5-sonnet",

	// Gemini
	Gemini31FlashLitePre, Gemini31ProPre, Gemini3ProPre, Gemini3FlashPre,
	Gemini25ProPre, Gemini25FlashPre, Gemini20Flash, Gemini20FlashExp,
	Gemini15Pro, Gemini15Flash, GeminiPro, Gemini,

	// OpenAI
	GPT35, GPT350125, GPT351106, "gpt-3.5-turbo-16k",
	GPT4, GPT40613, GPT432k, GPT432k0613,
	GPT4Turbo, GPT4TurboPreview, GPT4Turbo0125, GPT4Turbo1106, GPT4Turbo0409,
	GPT4o, GPT4o0806, GPT4oMini,
	GPT41, GPT41Mini, GPT41Nano,
	GPT5, GPT5Mini, GPT5Nano,
	GPT54, GPT54Mini, GPT54Nano,
	O1, O1Mini,

	// DeepSeek
	DeepSeekChat, DeepSeekReasoner,

	// Qwen
	Qwen, QwenTurbo, QwenPlus, QwenMax, QwenLong, Qwen3Max, Qwen35Plus,

	// MiniMax
	MiniMax, MiniMaxM27, MiniMaxM25, MiniMaxM21, MiniMaxM21Lightning, MiniMaxM2, MiniMaxABAB65,

	// GLM
	ZhipuAI, GLM5Turbo, GLM5, GLM4, GLM4Plus, GLM4Flash, GLM4Long, GLM4AllTools,
	GLM40520, GLM4Air, GLM4AirX, GLM47,

	// Kimi
	Moonshot, "moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k",
	KimiK2, KimiK25,

	// Doubao
	Doubao, DoubaoSeed2Code, DoubaoSeed2Pro, DoubaoSeed2Lite, DoubaoSeed2Mini,

	// 其他
	WenXin, WenXin4, XunFei,
	LinkAI35, LinkAI4Turbo, LinkAI4o,
	ModelScope,
}

// GiteeAIModelList Gitee AI 模型列表
var GiteeAIModelList = []string{
	"Yi-34B-Chat", "InternVL2-8B", "deepseek-coder-33B-instruct",
	"InternVL2.5-26B", "Qwen2-VL-72B", "Qwen2.5-32B-Instruct",
	"glm-4-9b-chat", "codegeex4-all-9b", "Qwen2.5-Coder-32B-Instruct",
	"Qwen2.5-72B-Instruct", "Qwen2.5-7B-Instruct", "Qwen2-72B-Instruct",
	"Qwen2-7B-Instruct", "code-raccoon-v1", "Qwen2.5-14B-Instruct",
}

// ModelScopeModelList ModelScope 模型列表
var ModelScopeModelList = []string{
	"LLM-Research/c4ai-command-r-plus-08-2024",
	"mistralai/Mistral-Small-Instruct-2409",
	"mistralai/Ministral-8B-Instruct-2410",
	"mistralai/Mistral-Large-Instruct-2407",
	"Qwen/Qwen2.5-Coder-32B-Instruct",
	"Qwen/Qwen2.5-Coder-14B-Instruct",
	"Qwen/Qwen2.5-Coder-7B-Instruct",
	"Qwen/Qwen2.5-72B-Instruct",
	"Qwen/Qwen2.5-32B-Instruct",
	"Qwen/Qwen2.5-14B-Instruct",
	"Qwen/Qwen2.5-7B-Instruct",
	"Qwen/QwQ-32B-Preview",
	"LLM-Research/Llama-3.3-70B-Instruct",
	"opencompass/CompassJudger-1-32B-Instruct",
	"Qwen/QVQ-72B-Preview",
	"LLM-Research/Meta-Llama-3.1-405B-Instruct",
	"LLM-Research/Meta-Llama-3.1-8B-Instruct",
	"Qwen/Qwen2-VL-7B-Instruct",
	"LLM-Research/Meta-Llama-3.1-70B-Instruct",
	"Qwen/Qwen2.5-14B-Instruct-1M",
	"Qwen/Qwen2.5-7B-Instruct-1M",
	"Qwen/Qwen2.5-VL-3B-Instruct",
	"Qwen/Qwen2.5-VL-7B-Instruct",
	"Qwen/Qwen2.5-VL-72B-Instruct",
	"deepseek-ai/DeepSeek-R1-Distill-Llama-70B",
	"deepseek-ai/DeepSeek-R1-Distill-Llama-8B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-32B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-14B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-7B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B",
	"deepseek-ai/DeepSeek-R1",
	"deepseek-ai/DeepSeek-V3",
	"Qwen/QwQ-32B",
}

func init() {
	// 合并模型列表
	ModelList = append(ModelList, GiteeAIModelList...)
	ModelList = append(ModelList, ModelScopeModelList...)
}
