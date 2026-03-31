// Package tencent 提供腾讯云语音引擎实现
// 支持文本转语音(TTS)和语音转文本(ASR)功能
package tencent

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/voice"
)

const (
	// 默认服务区域
	defaultRegion = "ap-guangzhou"
	// TTS API 地址
	ttsAPIURL = "tts.tencentcloudapi.com"
	// ASR API 地址
	asrAPIURL = "asr.tencentcloudapi.com"
	// 默认语音类型(客服女声)
	defaultVoiceType = 1003
	// 默认采样率
	defaultSampleRate = 16000
)

// Engine 腾讯云语音引擎
// 实现 voice.VoiceEngine 接口，支持 TTS 和 ASR 功能
type Engine struct {
	// secretId 腾讯云 SecretId
	secretId string
	// secretKey 腾讯云 SecretKey
	secretKey string
	// region 服务区域
	region string
	// voiceType 语音类型
	voiceType int
	// sampleRate 采样率
	sampleRate int
	// httpClient HTTP客户端
	httpClient *http.Client
	// config 引擎配置
	config voice.Config
}

// New 创建腾讯云语音引擎实例
// cfg: 语音引擎配置
func New(cfg voice.Config) (voice.VoiceEngine, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("腾讯云 SecretId 不能为空")
	}
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("腾讯云 SecretKey 不能为空")
	}

	region := cfg.Region
	if region == "" {
		region = defaultRegion
	}

	voiceType := defaultVoiceType
	if cfg.Extra != nil {
		if vt, ok := cfg.Extra["voice_type"].(int); ok {
			voiceType = vt
		} else if vt, ok := cfg.Extra["voice_type"].(float64); ok {
			voiceType = int(vt)
		}
	}

	sampleRate := cfg.SampleRate
	if sampleRate == 0 {
		sampleRate = defaultSampleRate
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30
	}

	return &Engine{
		secretId:   cfg.APIKey,
		secretKey:  cfg.SecretKey,
		region:     region,
		voiceType:  voiceType,
		sampleRate: sampleRate,
		config:     cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}, nil
}

// Name 返回引擎名称
func (e *Engine) Name() string {
	return "tencent"
}

// TTS 将文本转换为语音
// ctx: 上下文
// text: 要转换的文本
// 返回 MP3 格式的语音数据
func (e *Engine) TTS(ctx context.Context, text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("文本不能为空")
	}

	// 构建请求参数
	params := map[string]interface{}{
		"Text":            text,
		"SessionId":       fmt.Sprintf("%d", time.Now().Unix()),
		"Volume":          5,
		"Speed":           0,
		"ProjectId":       0,
		"ModelType":       1,
		"PrimaryLanguage": 1,
		"SampleRate":      e.sampleRate,
		"VoiceType":       e.voiceType,
	}

	// 发送请求
	resp, err := e.sendRequest(ctx, ttsAPIURL, "TextToVoice", params)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", err)
	}

	// 解析响应
	var ttsResp ttsResponse
	if err := json.Unmarshal(resp, &ttsResp); err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("解析响应失败: %w", err))
	}

	if ttsResp.Response.Error != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts",
			fmt.Errorf("TTS API错误: %s", ttsResp.Response.Error.Message))
	}

	// Base64解码音频数据
	audioData, err := base64.StdEncoding.DecodeString(ttsResp.Response.Audio)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("解码音频数据失败: %w", err))
	}

	return audioData, nil
}

// ASR 将语音转换为文本
// ctx: 上下文
// audio: 音频数据
// 返回识别出的文本
func (e *Engine) ASR(ctx context.Context, audio []byte) (string, error) {
	if len(audio) == 0 {
		return "", fmt.Errorf("音频数据不能为空")
	}

	// Base64编码音频数据
	base64Audio := base64.StdEncoding.EncodeToString(audio)

	// 构建请求参数
	params := map[string]interface{}{
		"ProjectId":      0,
		"SubServiceType": 2,
		"EngSerViceType": "16k_zh",
		"SourceType":     1,
		"VoiceFormat":    "wav",
		"UsrAudioKey":    "voice_recognition",
		"Data":           base64Audio,
	}

	// 发送请求
	resp, err := e.sendRequest(ctx, asrAPIURL, "SentenceRecognition", params)
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", err)
	}

	// 解析响应
	var asrResp asrResponse
	if err := json.Unmarshal(resp, &asrResp); err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("解析响应失败: %w", err))
	}

	if asrResp.Response.Error != nil {
		return "", voice.NewVoiceError(e.Name(), "asr",
			fmt.Errorf("ASR API错误: %s", asrResp.Response.Error.Message))
	}

	return asrResp.Response.Result, nil
}

// sendRequest 发送腾讯云API请求
func (e *Engine) sendRequest(ctx context.Context, host, action string, params map[string]interface{}) ([]byte, error) {
	// 构建请求URL
	requestURL := fmt.Sprintf("https://%s", host)

	// 序列化参数
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("序列化请求参数失败: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", "2019-08-23")
	req.Header.Set("X-TC-Region", e.region)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))

	// 生成签名
	signature := e.generateSignature(host, string(body))
	req.Header.Set("Authorization", signature)

	// 发送请求
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// generateSignature 生成腾讯云API签名
func (e *Engine) generateSignature(host, body string) string {
	// 简化版签名，实际生产环境应使用腾讯云官方SDK
	// 这里使用TC3-HMAC-SHA256签名方法
	timestamp := time.Now().Unix()
	date := time.Unix(timestamp, 0).Format("2006-01-02")

	// 构建签名字符串
	canonicalRequest := fmt.Sprintf("POST\n/\n\ncontent-type:application/json\nhost:%s\n\ncontent-type;host\n%s",
		host, e.sha256Hex(body))

	// 构建待签名字符串
	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%d\n%s\n%s",
		timestamp, date, e.sha256Hex(canonicalRequest))

	// 计算签名
	secretDate := e.hmacSha256([]byte("TC3"+e.secretKey), date)
	secretService := e.hmacSha256(secretDate, "tts")
	secretSigning := e.hmacSha256(secretService, "tc3_request")
	signature := e.hmacSha256Hex(secretSigning, stringToSign)

	// 构建Authorization
	authorization := fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s/tts/tc3_request, SignedHeaders=content-type;host, Signature=%s",
		e.secretId, date, signature)

	return authorization
}

// sha256Hex 计算SHA256哈希值(十六进制)
func (e *Engine) sha256Hex(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// hmacSha256 计算HMAC-SHA256
func (e *Engine) hmacSha256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// hmacSha256Hex 计算HMAC-SHA256(十六进制)
func (e *Engine) hmacSha256Hex(key []byte, data string) string {
	return hex.EncodeToString(e.hmacSha256(key, data))
}

// TTS响应结构
type ttsResponse struct {
	Response struct {
		Audio string `json:"Audio"`
		Error *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error,omitempty"`
	} `json:"Response"`
}

// ASR响应结构
type asrResponse struct {
	Response struct {
		Result string `json:"Result"`
		Error  *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error,omitempty"`
	} `json:"Response"`
}

// init 注册引擎到工厂
func init() {
	voice.RegisterEngine(voice.EngineTencent, func(cfg voice.Config) (voice.VoiceEngine, error) {
		return New(cfg)
	})
}
