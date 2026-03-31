// Package voice 提供语音处理核心功能
// voice.go 定义语音引擎核心接口
package voice

import (
	"context"
)

// VoiceEngine 定义语音引擎的核心接口
// 包含文本转语音(TTS)和语音转文本(ASR)能力
type VoiceEngine interface {
	// TTS 将文本转换为语音
	// text: 要转换的文本内容
	// 返回语音数据([]byte)和错误信息
	TTS(ctx context.Context, text string) ([]byte, error)

	// ASR 将语音转换为文本
	// audio: 语音数据([]byte)
	// 返回识别出的文本和错误信息
	ASR(ctx context.Context, audio []byte) (string, error)

	// Name 返回语音引擎名称
	Name() string
}

// TTSEngine 定义纯文本转语音接口
// 用于只需要TTS功能的场景
type TTSEngine interface {
	// TTS 将文本转换为语音
	TTS(ctx context.Context, text string) ([]byte, error)

	// Name 返回引擎名称
	Name() string
}

// ASREngine 定义纯语音转文本接口
// 用于只需要ASR功能的场景
type ASREngine interface {
	// ASR 将语音转换为文本
	ASR(ctx context.Context, audio []byte) (string, error)

	// Name 返回引擎名称
	Name() string
}

// EngineType 定义语音引擎类型
type EngineType string

const (
	// EngineOpenAI OpenAI语音引擎(Whisper + TTS)
	EngineOpenAI EngineType = "openai"
	// EngineAzure Azure语音服务
	EngineAzure EngineType = "azure"
	// EngineBaidu 百度语音服务
	EngineBaidu EngineType = "baidu"
	// EngineAli 阿里云语音服务
	EngineAli EngineType = "ali"
	// EngineTencent 腾讯云语音服务
	EngineTencent EngineType = "tencent"
	// EngineXunfei 讯飞语音服务
	EngineXunfei EngineType = "xunfei"
	// EngineGoogle Google语音服务
	EngineGoogle EngineType = "google"
	// EngineEdge Microsoft Edge TTS(仅TTS)
	EngineEdge EngineType = "edge"
	// EngineElevenLabs ElevenLabs TTS(仅TTS)
	EngineElevenLabs EngineType = "elevenlabs"
)

// Config 定义语音引擎配置
type Config struct {
	// EngineType 引擎类型
	EngineType EngineType `json:"engine_type"`

	// APIKey API密钥
	APIKey string `json:"api_key"`

	// SecretKey 密钥(部分服务商需要)
	SecretKey string `json:"secret_key,omitempty"`

	// Region 服务区域(Azure等需要)
	Region string `json:"region,omitempty"`

	// APIBase API基础URL(可自定义)
	APIBase string `json:"api_base,omitempty"`

	// VoiceID TTS语音ID
	VoiceID string `json:"voice_id,omitempty"`

	// Language 语音识别/合成语言
	Language string `json:"language,omitempty"`

	// Model TTS/ASR模型名称
	Model string `json:"model,omitempty"`

	// SampleRate 音频采样率
	SampleRate int `json:"sample_rate,omitempty"`

	// OutputFormat 输出音频格式(mp3, wav, pcm等)
	OutputFormat string `json:"output_format,omitempty"`

	// Proxy HTTP代理地址
	Proxy string `json:"proxy,omitempty"`

	// Timeout 请求超时时间(秒)
	Timeout int `json:"timeout,omitempty"`

	// Extra 额外配置参数
	Extra map[string]any `json:"extra,omitempty"`
}

// Result 定义语音处理结果
type Result struct {
	// Text ASR识别出的文本
	Text string `json:"text,omitempty"`

	// Audio TTS生成的语音数据
	Audio []byte `json:"audio,omitempty"`

	// Format 音频格式
	Format AudioFormat `json:"format,omitempty"`

	// Duration 音频时长(毫秒)
	Duration int64 `json:"duration,omitempty"`

	// SampleRate 采样率
	SampleRate int `json:"sample_rate,omitempty"`
}

// AudioFormat 定义音频格式类型
type AudioFormat string

const (
	// FormatMP3 MP3格式
	FormatMP3 AudioFormat = "mp3"
	// FormatWAV WAV格式
	FormatWAV AudioFormat = "wav"
	// FormatPCM PCM原始格式
	FormatPCM AudioFormat = "pcm"
	// FormatOGG OGG格式
	FormatOGG AudioFormat = "ogg"
	// FormatAMR AMR格式
	FormatAMR AudioFormat = "amr"
	// FormatSILK SILK格式(微信语音)
	FormatSILK AudioFormat = "silk"
	// FormatM4A M4A格式
	FormatM4A AudioFormat = "m4a"
)

// VoiceError 定义语音处理错误
type VoiceError struct {
	// Engine 产生错误的引擎名称
	Engine string
	// Operation 操作类型(tts/asr)
	Operation string
	// Err 原始错误
	Err error
}

// Error 实现error接口
func (e *VoiceError) Error() string {
	return "voice error [" + e.Engine + "." + e.Operation + "]: " + e.Err.Error()
}

// Unwrap 实现errors.Unwrap接口
func (e *VoiceError) Unwrap() error {
	return e.Err
}

// NewVoiceError 创建语音错误
func NewVoiceError(engine, operation string, err error) *VoiceError {
	return &VoiceError{
		Engine:    engine,
		Operation: operation,
		Err:       err,
	}
}
