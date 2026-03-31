// Package elevent 提供 ElevenLabs TTS 语音引擎实现
// 仅支持文本转语音(TTS)功能
package elevent

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bstr9/simpleclaw/pkg/voice"
)

const (
	// DefaultAPIBase 默认API地址
	DefaultAPIBase = "https://api.elevenlabs.io/v1"
	// DefaultModel 默认模型
	DefaultModel = "eleven_multilingual_v2"
	// DefaultVoice 默认语音
	DefaultVoice = "Rachel"
)

// Engine ElevenLabs TTS引擎
// 实现 voice.VoiceEngine 接口，仅支持 TTS 功能
type Engine struct {
	// apiKey API密钥
	apiKey string
	// voiceID 语音ID
	voiceID string
	// model 模型名称
	model string
	// apiBase API基础地址
	apiBase string
	// httpClient HTTP客户端
	httpClient *http.Client
	// config 引擎配置
	config voice.Config
}

// New 创建 ElevenLabs TTS 引擎实例
func New(cfg voice.Config) (voice.VoiceEngine, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("ElevenLabs API Key 不能为空")
	}

	voiceID := cfg.VoiceID
	if voiceID == "" {
		voiceID = DefaultVoice
	}

	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = DefaultAPIBase
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60
	}

	return &Engine{
		apiKey:     cfg.APIKey,
		voiceID:    voiceID,
		model:      model,
		apiBase:    apiBase,
		config:     cfg,
		httpClient: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

// Name 返回引擎名称
func (e *Engine) Name() string {
	return "elevenlabs"
}

// TTS 将文本转换为语音
func (e *Engine) TTS(ctx context.Context, text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("文本不能为空")
	}

	// 构建请求URL
	url := fmt.Sprintf("%s/text-to-speech/%s", e.apiBase, e.voiceID)

	// 构建请求体
	reqBody := map[string]interface{}{
		"text":  text,
		"model": e.model,
	}

	// 添加语音设置
	if e.config.Extra != nil {
		if settings, ok := e.config.Extra["voice_settings"].(map[string]interface{}); ok {
			reqBody["voice_settings"] = settings
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("序列化请求失败: %w", err))
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", err)
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("xi-api-key", e.apiKey)

	// 发送请求
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", err)
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, voice.NewVoiceError(e.Name(), "tts",
			fmt.Errorf("TTS请求失败: status=%d, body=%s", resp.StatusCode, string(body)))
	}

	// 读取音频数据
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("读取音频数据失败: %w", err))
	}

	return audioData, nil
}

// ASR ElevenLabs 不支持语音识别
func (e *Engine) ASR(ctx context.Context, audio []byte) (string, error) {
	return "", voice.NewVoiceError(e.Name(), "asr",
		fmt.Errorf("ElevenLabs 不支持语音识别功能"))
}

// init 注册引擎到工厂
func init() {
	voice.RegisterEngine(voice.EngineElevenLabs, func(cfg voice.Config) (voice.VoiceEngine, error) {
		return New(cfg)
	})
}
