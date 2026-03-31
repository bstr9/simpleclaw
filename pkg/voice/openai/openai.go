// Package openai 提供 OpenAI 语音引擎实现
// 支持 TTS (文本转语音) 和 ASR (语音转文本) 功能
package openai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/bstr9/simpleclaw/pkg/voice"
)

// Engine OpenAI 语音引擎
// 实现 voice.VoiceEngine 接口，支持 TTS 和 ASR 功能
type Engine struct {
	client *openai.Client
	config voice.Config
}

// 默认配置常量
const (
	// DefaultTTSModel 默认 TTS 模型
	DefaultTTSModel = "tts-1"
	// DefaultASRModel 默认 ASR 模型 (Whisper)
	DefaultASRModel = "whisper-1"
	// DefaultVoice 默认语音
	DefaultVoice = "alloy"
	// DefaultAPIBase 默认 API 基础地址
	DefaultAPIBase = "https://api.openai.com/v1"
)

// New 创建 OpenAI 语音引擎实例
// cfg: 语音引擎配置
func New(cfg voice.Config) (*Engine, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API 密钥不能为空")
	}

	// 配置 OpenAI 客户端
	config := openai.DefaultConfig(cfg.APIKey)

	// 设置自定义 API 地址
	if cfg.APIBase != "" {
		config.BaseURL = cfg.APIBase
	} else {
		config.BaseURL = DefaultAPIBase
	}

	// 配置 HTTP 客户端（含代理和超时）
	httpClient := &http.Client{}
	if cfg.Timeout > 0 {
		httpClient.Timeout = time.Duration(cfg.Timeout) * time.Second
	}
	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("代理地址无效: %w", err)
		}
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}
	config.HTTPClient = httpClient

	return &Engine{
		client: openai.NewClientWithConfig(config),
		config: cfg,
	}, nil
}

// Name 返回引擎名称
func (e *Engine) Name() string {
	return "openai"
}

// TTS 将文本转换为语音
// ctx: 上下文
// text: 要转换的文本
// 返回 MP3 格式的语音数据
func (e *Engine) TTS(ctx context.Context, text string) ([]byte, error) {
	// 获取 TTS 模型配置
	model := e.config.Model
	if model == "" {
		model = DefaultTTSModel
	}

	// 获取语音配置
	voiceID := e.config.VoiceID
	if voiceID == "" {
		voiceID = DefaultVoice
	}

	// 构建请求
	req := openai.CreateSpeechRequest{
		Model: openai.SpeechModel(model),
		Input: text,
		Voice: openai.SpeechVoice(voiceID),
	}

	// 设置响应格式
	if e.config.OutputFormat != "" {
		req.ResponseFormat = openai.SpeechResponseFormat(e.config.OutputFormat)
	} else {
		req.ResponseFormat = openai.SpeechResponseFormatMp3
	}

	// 调用 OpenAI TTS API
	resp, err := e.client.CreateSpeech(ctx, req)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", err)
	}
	defer resp.Close()

	// 读取音频数据
	audioData, err := io.ReadAll(resp)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("读取音频数据失败: %w", err))
	}

	return audioData, nil
}

// ASR 将语音转换为文本
// ctx: 上下文
// audio: 音频数据
// 返回识别出的文本
func (e *Engine) ASR(ctx context.Context, audio []byte) (string, error) {
	// 获取 ASR 模型配置
	model := e.config.Model
	if model == "" {
		model = DefaultASRModel
	}

	// 构建 ASR 请求
	req := openai.AudioRequest{
		Model:    model,
		FilePath: "audio.mp3", // 文件名仅用于内容类型推断
		Reader:   NewByteReader(audio),
	}

	// 设置语言
	if e.config.Language != "" {
		req.Language = e.config.Language
	}

	// 调用 OpenAI Whisper API
	resp, err := e.client.CreateTranscription(ctx, req)
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", err)
	}

	return resp.Text, nil
}

// ByteReader 实现 io.Reader 接口，用于从字节数组读取
type ByteReader struct {
	data   []byte
	offset int
}

// NewByteReader 创建字节读取器
func NewByteReader(data []byte) *ByteReader {
	return &ByteReader{data: data}
}

// Read 实现 io.Reader 接口
func (r *ByteReader) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

// init 注册引擎到工厂
func init() {
	voice.RegisterEngine(voice.EngineOpenAI, func(cfg voice.Config) (voice.VoiceEngine, error) {
		return New(cfg)
	})
}
