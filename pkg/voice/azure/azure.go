// Package azure 提供 Azure Speech Services 语音引擎实现
// 支持文本转语音(TTS)和语音转文本(ASR)功能
package azure

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/voice"
)

const (
	// 默认Azure语音服务区域
	defaultRegion = "eastasia"
	// 默认TTS语音名称(中文女声)
	defaultVoiceName = "zh-CN-XiaoxiaoNeural"
	// 默认识别语言
	defaultRecognitionLanguage = "zh-CN"
	// TTS API 路径
	ttsAPIPath = "/cognitiveservices/v1"
	// ASR API 路径
	asrAPIPath = "/speech/recognition/conversation/cognitiveservices/v1"
)

// AzureEngine Azure语音引擎实现
type AzureEngine struct {
	// config 配置
	config voice.Config
	// apiKey Azure订阅密钥
	apiKey string
	// region Azure服务区域
	region string
	// voiceName TTS语音名称
	voiceName string
	// recognitionLanguage ASR识别语言
	recognitionLanguage string
	// httpClient HTTP客户端
	httpClient *http.Client
	// 语言自动检测配置
	languageVoices map[string]string
}

// 语言代码映射到默认语音名称
var defaultLanguageVoices = map[string]string{
	"zh": "zh-CN-YunxiNeural",
	"en": "en-US-JacobNeural",
	"ja": "ja-JP-AoiNeural",
	"ko": "ko-KR-SoonBokNeural",
	"de": "de-DE-LouisaNeural",
	"fr": "fr-FR-BrigitteNeural",
	"es": "es-ES-LaiaNeural",
}

// New 创建Azure语音引擎实例
func New(cfg voice.Config) (voice.VoiceEngine, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("azure API Key不能为空")
	}

	region := valueOrDefault(cfg.Region, defaultRegion)
	voiceName := valueOrDefault(cfg.VoiceID, defaultVoiceName)
	recognitionLanguage := valueOrDefault(cfg.Language, defaultRecognitionLanguage)
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30
	}

	languageVoices := buildLanguageVoices(cfg.Extra)

	return &AzureEngine{
		config:              cfg,
		apiKey:              cfg.APIKey,
		region:              region,
		voiceName:           voiceName,
		recognitionLanguage: recognitionLanguage,
		languageVoices:      languageVoices,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}, nil
}

// valueOrDefault 返回非空值或默认值
func valueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// buildLanguageVoices 构建语言语音映射
func buildLanguageVoices(extra map[string]any) map[string]string {
	languageVoices := make(map[string]string)
	for k, v := range defaultLanguageVoices {
		languageVoices[k] = v
	}

	if extra == nil {
		return languageVoices
	}

	voices, ok := extra["language_voices"].(map[string]interface{})
	if !ok {
		return languageVoices
	}

	for lang, voice := range voices {
		if v, ok := voice.(string); ok {
			languageVoices[lang] = v
		}
	}

	return languageVoices
}

// Name 返回引擎名称
func (e *AzureEngine) Name() string {
	return "azure"
}

// TTS 文本转语音
func (e *AzureEngine) TTS(ctx context.Context, text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("文本不能为空")
	}

	// 确定使用的语音名称
	voiceName := e.selectVoiceName(text)

	// 构建SSML
	ssml := e.buildSSML(text, voiceName)

	// 构建请求URL
	ttsURL := fmt.Sprintf("https://%s.tts.speech.microsoft.com%s", e.region, ttsAPIPath)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", ttsURL, strings.NewReader(ssml))
	if err != nil {
		return nil, voice.NewVoiceError("azure", "tts", err)
	}

	// 设置请求头
	req.Header.Set("Ocp-Apim-Subscription-Key", e.apiKey)
	req.Header.Set(common.HeaderContentType, "application/ssml+xml")
	req.Header.Set("X-Microsoft-OutputFormat", e.getOutputFormat())
	req.Header.Set("User-Agent", "simpleclaw")

	// 发送请求
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, voice.NewVoiceError("azure", "tts", err)
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, voice.NewVoiceError("azure", "tts",
			fmt.Errorf("TTS请求失败: status=%d, body=%s", resp.StatusCode, string(body)))
	}

	// 读取音频数据
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, voice.NewVoiceError("azure", "tts", err)
	}

	return audioData, nil
}

// ASR 语音转文本
func (e *AzureEngine) ASR(ctx context.Context, audio []byte) (string, error) {
	if len(audio) == 0 {
		return "", fmt.Errorf("音频数据不能为空")
	}

	// 构建请求URL
	asrURL := fmt.Sprintf("https://%s.stt.speech.microsoft.com%s", e.region, asrAPIPath)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", asrURL, strings.NewReader(string(audio)))
	if err != nil {
		return "", voice.NewVoiceError("azure", "asr", err)
	}

	// 设置请求参数
	q := url.Values{}
	q.Set("language", e.recognitionLanguage)
	req.URL.RawQuery = q.Encode()

	// 设置请求头
	req.Header.Set("Ocp-Apim-Subscription-Key", e.apiKey)
	req.Header.Set(common.HeaderContentType, "audio/wav; codecs=audio/pcm; samplerate=16000")
	req.Header.Set("Accept", common.ContentTypeJSON)

	// 发送请求
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", voice.NewVoiceError("azure", "asr", err)
	}
	defer resp.Body.Close()

	// 检查响应
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", voice.NewVoiceError("azure", "asr",
			fmt.Errorf("ASR请求失败: status=%d, body=%s", resp.StatusCode, string(body)))
	}

	// 解析响应
	var asrResp ASRResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", voice.NewVoiceError("azure", "asr", err)
	}

	if err := xml.Unmarshal(body, &asrResp); err != nil {
		return "", voice.NewVoiceError("azure", "asr", err)
	}

	return asrResp.DisplayText, nil
}

// selectVoiceName 根据文本内容选择合适的语音名称
func (e *AzureEngine) selectVoiceName(text string) string {
	autoDetect, ok := e.config.Extra["auto_detect"].(bool)
	if !ok || !autoDetect {
		return e.voiceName
	}

	lang := detectLanguage(text)
	if voice, ok := e.languageVoices[lang]; ok {
		return voice
	}
	return e.voiceName
}

// detectLanguage 检测文本语言
func detectLanguage(text string) string {
	switch {
	case containsChinese(text):
		return "zh"
	case containsJapanese(text):
		return "ja"
	case containsKorean(text):
		return "ko"
	default:
		return "en"
	}
}

// buildSSML 构建SSML语音合成标记
func (e *AzureEngine) buildSSML(text, voiceName string) string {
	// 转义XML特殊字符
	escapedText := escapeXML(text)
	return fmt.Sprintf(`<speak version="1.0" xmlns="http://www.w3.org/2001/10/synthesis" xml:lang="zh-CN"><voice name="%s">%s</voice></speak>`,
		voiceName, escapedText)
}

// getOutputFormat 获取输出音频格式
func (e *AzureEngine) getOutputFormat() string {
	format := e.config.OutputFormat
	switch format {
	case "mp3":
		return "audio-16khz-32kbitrate-mono-mp3"
	case "wav":
		return "riff-16khz-16bit-mono-pcm"
	case "ogg":
		return "ogg-16khz-16bit-mono-opus"
	default:
		return "audio-16khz-128kbitrate-mono-mp3"
	}
}

// containsChinese 检查字符串是否包含中文
func containsChinese(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

// containsJapanese 检查字符串是否包含日文
func containsJapanese(s string) bool {
	for _, r := range s {
		if (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF) {
			return true
		}
	}
	return false
}

// containsKorean 检查字符串是否包含韩文
func containsKorean(s string) bool {
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7AF {
			return true
		}
	}
	return false
}

// escapeXML 转义XML特殊字符
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// ASRResponse ASR响应结构
type ASRResponse struct {
	XMLName           xml.Name `xml:"SpeechRecognitionResult"`
	DisplayText       string   `xml:"DisplayText"`
	RecognitionStatus string   `xml:"RecognitionStatus"`
}

// init 注册到工厂
func init() {
	voice.RegisterEngine(voice.EngineAzure, New)
}
