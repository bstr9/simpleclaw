// Package google 提供 Google Cloud 语音引擎实现
// 支持 TTS (文本转语音) 和 ASR (语音转文本) 功能
// 使用 Google Cloud Text-to-Speech 和 Speech-to-Text API
package google

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/voice"
)

// Engine Google Cloud 语音引擎
// 实现 voice.VoiceEngine 接口，支持 TTS 和 ASR 功能
type Engine struct {
	config     voice.Config
	httpClient *http.Client
}

// 默认配置常量
const (
	// DefaultTTSAPIBase 默认 TTS API 地址
	DefaultTTSAPIBase = "https://texttospeech.googleapis.com/v1"
	// DefaultASRAPIBase 默认 ASR API 地址
	DefaultASRAPIBase = "https://speech.googleapis.com/v1"
	// DefaultLanguage 默认语言
	DefaultLanguage = "zh-CN"
	// DefaultVoice 默认语音名称
	DefaultVoice = "zh-CN-Wavenet-A"
	// DefaultAudioEncoding 默认音频编码
	DefaultAudioEncoding = "MP3"
)

// Google TTS API 请求/响应结构

// ttsRequest TTS 请求结构
type ttsRequest struct {
	Input       ttsInput       `json:"input"`
	Voice       ttsVoice       `json:"voice"`
	AudioConfig ttsAudioConfig `json:"audioConfig"`
}

// ttsInput TTS 输入
type ttsInput struct {
	Text string `json:"text"`
}

// ttsVoice TTS 语音配置
type ttsVoice struct {
	LanguageCode string `json:"languageCode"`
	Name         string `json:"name,omitempty"`
	SsmlGender   string `json:"ssmlGender,omitempty"`
}

// ttsAudioConfig TTS 音频配置
type ttsAudioConfig struct {
	AudioEncoding   string  `json:"audioEncoding"`
	SpeakingRate    float64 `json:"speakingRate,omitempty"`
	Pitch           float64 `json:"pitch,omitempty"`
	VolumeGainDb    float64 `json:"volumeGainDb,omitempty"`
	SampleRateHertz int     `json:"sampleRateHertz,omitempty"`
}

// ttsResponse TTS 响应结构
type ttsResponse struct {
	AudioContent string `json:"audioContent"`
	Error        *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Google ASR API 请求/响应结构

// asrRequest ASR 请求结构
type asrRequest struct {
	Config asrConfig `json:"config"`
	Audio  asrAudio  `json:"audio"`
}

// asrConfig ASR 配置
type asrConfig struct {
	Encoding                   string `json:"encoding"`
	SampleRateHertz            int    `json:"sampleRateHertz"`
	LanguageCode               string `json:"languageCode"`
	EnableAutomaticPunctuation bool   `json:"enableAutomaticPunctuation,omitempty"`
	Model                      string `json:"model,omitempty"`
}

// asrAudio ASR 音频数据
type asrAudio struct {
	Content string `json:"content"`
}

// asrResponse ASR 响应结构
type asrResponse struct {
	Results []struct {
		Alternatives []struct {
			Transcript string  `json:"transcript"`
			Confidence float64 `json:"confidence"`
		} `json:"alternatives"`
	} `json:"results"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// New 创建 Google Cloud 语音引擎实例
// cfg: 语音引擎配置
// 需要配置 APIKey (Google Cloud API Key)
func New(cfg voice.Config) (*Engine, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("google Cloud API 密钥不能为空")
	}

	// 配置 HTTP 客户端
	httpClient := &http.Client{}
	if cfg.Timeout > 0 {
		httpClient.Timeout = time.Duration(cfg.Timeout) * time.Second
	} else {
		httpClient.Timeout = 30 * time.Second
	}

	// 配置代理
	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("代理地址无效: %w", err)
		}
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	return &Engine{
		config:     cfg,
		httpClient: httpClient,
	}, nil
}

// Name 返回引擎名称
func (e *Engine) Name() string {
	return "google"
}

// TTS 将文本转换为语音
// ctx: 上下文
// text: 要转换的文本
// 返回音频数据（默认 MP3 格式）
func (e *Engine) TTS(ctx context.Context, text string) ([]byte, error) {
	// 构建请求 URL
	apiBase := e.config.APIBase
	if apiBase == "" {
		apiBase = DefaultTTSAPIBase
	}
	apiURL := fmt.Sprintf("%s/text:synthesize?key=%s", apiBase, e.config.APIKey)

	// 获取语言配置
	language := e.config.Language
	if language == "" {
		language = DefaultLanguage
	}

	// 获取语音名称
	voiceName := e.config.VoiceID
	if voiceName == "" {
		voiceName = DefaultVoice
	}

	// 获取音频编码格式
	audioEncoding := DefaultAudioEncoding
	if e.config.OutputFormat != "" {
		audioEncoding = strings.ToUpper(e.config.OutputFormat)
	}

	// 构建请求体
	req := ttsRequest{
		Input: ttsInput{
			Text: text,
		},
		Voice: ttsVoice{
			LanguageCode: language,
			Name:         voiceName,
		},
		AudioConfig: ttsAudioConfig{
			AudioEncoding: audioEncoding,
		},
	}

	// 设置采样率
	if e.config.SampleRate > 0 {
		req.AudioConfig.SampleRateHertz = e.config.SampleRate
	}

	// 设置语速（从 Extra 配置中获取）
	if rate, ok := e.config.Extra["speaking_rate"].(float64); ok {
		req.AudioConfig.SpeakingRate = rate
	}
	if pitch, ok := e.config.Extra["pitch"].(float64); ok {
		req.AudioConfig.Pitch = pitch
	}

	// 序列化请求
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("序列化请求失败: %w", err))
	}

	// 发送 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("创建请求失败: %w", err))
	}
	httpReq.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("请求失败: %w", err))
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("读取响应失败: %w", err))
	}

	// 解析响应
	var ttsResp ttsResponse
	if err := json.Unmarshal(respBody, &ttsResp); err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("解析响应失败: %w", err))
	}

	// 检查错误
	if ttsResp.Error != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("API 错误 [%d]: %s", ttsResp.Error.Code, ttsResp.Error.Message))
	}

	// 解码 Base64 音频数据
	audioData, err := base64.StdEncoding.DecodeString(ttsResp.AudioContent)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("解码音频数据失败: %w", err))
	}

	return audioData, nil
}

// ASR 将语音转换为文本
func (e *Engine) ASR(ctx context.Context, audio []byte) (string, error) {
	apiURL := e.buildASRURL()
	req := e.buildASRRequest(audio)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("序列化请求失败: %w", err))
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("创建请求失败: %w", err))
	}
	httpReq.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("请求失败: %w", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("读取响应失败: %w", err))
	}

	var asrResp asrResponse
	if err := json.Unmarshal(respBody, &asrResp); err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("解析响应失败: %w", err))
	}

	if asrResp.Error != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("API 错误 [%d]: %s", asrResp.Error.Code, asrResp.Error.Message))
	}

	return e.extractTranscript(asrResp)
}

// buildASRURL 构建 ASR API URL
func (e *Engine) buildASRURL() string {
	apiBase := e.config.APIBase
	if apiBase == "" {
		apiBase = DefaultASRAPIBase
	}
	return fmt.Sprintf("%s/speech:recognize?key=%s", apiBase, e.config.APIKey)
}

// buildASRRequest 构建 ASR 请求
func (e *Engine) buildASRRequest(audio []byte) asrRequest {
	language := e.config.Language
	if language == "" {
		language = DefaultLanguage
	}

	sampleRate := e.config.SampleRate
	if sampleRate == 0 {
		sampleRate = 16000
	}

	req := asrRequest{
		Config: asrConfig{
			Encoding:                   e.getAudioEncoding(),
			SampleRateHertz:            sampleRate,
			LanguageCode:               language,
			EnableAutomaticPunctuation: true,
		},
		Audio: asrAudio{
			Content: base64.StdEncoding.EncodeToString(audio),
		},
	}

	if model, ok := e.config.Extra["model"].(string); ok {
		req.Config.Model = model
	}

	return req
}

// getAudioEncoding 获取音频编码格式
func (e *Engine) getAudioEncoding() string {
	if e.config.OutputFormat == "" {
		return "LINEAR16"
	}
	switch strings.ToUpper(e.config.OutputFormat) {
	case "MP3":
		return "MP3"
	case "OGG", "OGG_OPUS":
		return "OGG_OPUS"
	case "FLAC":
		return "FLAC"
	default:
		return "LINEAR16"
	}
}

// extractTranscript 提取识别结果
func (e *Engine) extractTranscript(asrResp asrResponse) (string, error) {
	var transcript strings.Builder
	for _, result := range asrResp.Results {
		if len(result.Alternatives) > 0 {
			transcript.WriteString(result.Alternatives[0].Transcript)
		}
	}

	if transcript.Len() == 0 {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("未能识别出文本"))
	}

	return transcript.String(), nil
}

// init 注册引擎到工厂
func init() {
	voice.RegisterEngine(voice.EngineGoogle, func(cfg voice.Config) (voice.VoiceEngine, error) {
		return New(cfg)
	})
}
