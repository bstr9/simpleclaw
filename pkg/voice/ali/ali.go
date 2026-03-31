// Package ali 提供阿里云语音引擎实现
// 支持文本转语音(TTS)和语音转文本(ASR)功能
package ali

import (
	"bytes"
	"github.com/bstr9/simpleclaw/pkg/common"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/voice"
)

const (
	// 默认TTS API地址
	defaultTTSAPIURL = "https://nls-gateway-cn-shanghai.aliyuncs.com/stream/v1/tts"
	// 默认ASR API地址
	defaultASRAPIURL = "https://nls-gateway.cn-shanghai.aliyuncs.com/stream/v1/asr"
	// Token API地址
	tokenAPIURL = "http://nls-meta.cn-shanghai.aliyuncs.com/"
	// 默认采样率
	defaultSampleRate = 16000
)

// Engine 阿里云语音引擎
// 实现 voice.VoiceEngine 接口，支持 TTS 和 ASR 功能
type Engine struct {
	// accessKeyId 阿里云 AccessKey ID
	accessKeyId string
	// accessKeySecret 阿里云 AccessKey Secret
	accessKeySecret string
	// appKey 应用Key
	appKey string
	// ttsAPIURL TTS API地址
	ttsAPIURL string
	// asrAPIURL ASR API地址
	asrAPIURL string
	// token 认证令牌
	token string
	// tokenExpireTime 令牌过期时间
	tokenExpireTime int64
	// httpClient HTTP客户端
	httpClient *http.Client
	// config 引擎配置
	config voice.Config
}

// New 创建阿里云语音引擎实例
func New(cfg voice.Config) (voice.VoiceEngine, error) {
	// 验证必填配置
	if err := validateAliConfig(cfg); err != nil {
		return nil, err
	}

	appKey := extractAppKey(cfg.Extra)
	ttsAPIURL := extractTTSAPIURL(cfg)
	asrAPIURL := extractASRAPIURL(cfg.Extra)
	timeout := getTimeout(cfg.Timeout)

	return &Engine{
		accessKeyId:     cfg.APIKey,
		accessKeySecret: cfg.SecretKey,
		appKey:          appKey,
		ttsAPIURL:       ttsAPIURL,
		asrAPIURL:       asrAPIURL,
		config:          cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}, nil
}

// validateAliConfig 验证阿里云配置参数
func validateAliConfig(cfg voice.Config) error {
	if cfg.APIKey == "" {
		return fmt.Errorf("阿里云 AccessKey ID 不能为空")
	}
	if cfg.SecretKey == "" {
		return fmt.Errorf("阿里云 AccessKey Secret 不能为空")
	}
	appKey := extractAppKey(cfg.Extra)
	if appKey == "" {
		return fmt.Errorf("阿里云 AppKey 不能为空")
	}
	return nil
}

// extractAppKey 从配置中提取 AppKey
func extractAppKey(extra map[string]any) string {
	if extra == nil {
		return ""
	}
	if ak, ok := extra["app_key"].(string); ok {
		return ak
	}
	return ""
}

// extractTTSAPIURL 从配置中提取 TTS API URL
func extractTTSAPIURL(cfg voice.Config) string {
	if cfg.APIBase != "" {
		return cfg.APIBase
	}
	if cfg.Extra != nil {
		if u, ok := cfg.Extra["tts_api_url"].(string); ok {
			return u
		}
	}
	return defaultTTSAPIURL
}

// extractASRAPIURL 从配置中提取 ASR API URL
func extractASRAPIURL(extra map[string]any) string {
	if extra != nil {
		if u, ok := extra["asr_api_url"].(string); ok {
			return u
		}
	}
	return defaultASRAPIURL
}

// getTimeout 获取超时配置
func getTimeout(timeout int) int {
	if timeout == 0 {
		return 30
	}
	return timeout
}

// Name 返回引擎名称
func (e *Engine) Name() string {
	return "ali"
}

// TTS 将文本转换为语音
func (e *Engine) TTS(ctx context.Context, text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("文本不能为空")
	}

	// 清除文本中的特殊字符
	text = cleanText(text)

	// 获取有效token
	token, err := e.getValidToken(ctx)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", err)
	}

	// 构建请求体
	reqBody := map[string]interface{}{
		"text":   text,
		"appkey": e.appKey,
		"token":  token,
		"format": "wav",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("序列化请求失败: %w", err))
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", e.ttsAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", err)
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	// 发送请求
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", err)
	}
	defer resp.Body.Close()

	// 检查响应类型
	contentType := resp.Header.Get(common.HeaderContentType)
	if resp.StatusCode == http.StatusOK && strings.HasPrefix(contentType, "audio/") {
		audioData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("读取音频数据失败: %w", err))
		}
		return audioData, nil
	}

	// 解析错误响应
	body, _ := io.ReadAll(resp.Body)
	return nil, voice.NewVoiceError(e.Name(), "tts",
		fmt.Errorf("TTS请求失败: status=%d, body=%s", resp.StatusCode, string(body)))
}

// ASR 将语音转换为文本
func (e *Engine) ASR(ctx context.Context, audio []byte) (string, error) {
	if len(audio) == 0 {
		return "", fmt.Errorf("音频数据不能为空")
	}

	// 获取有效token
	token, err := e.getValidToken(ctx)
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", err)
	}

	// 构建请求URL
	reqURL := fmt.Sprintf("%s?appkey=%s&format=pcm&sample_rate=%d&enable_punctuation_prediction=true&enable_inverse_text_normalization=true",
		e.asrAPIURL, e.appKey, defaultSampleRate)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(audio))
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", err)
	}

	req.Header.Set("X-NLS-Token", token)
	req.Header.Set(common.HeaderContentType, "application/octet-stream")

	// 发送请求
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", err)
	}
	defer resp.Body.Close()

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("读取响应失败: %w", err))
	}

	var asrResp asrResponse
	if err := json.Unmarshal(body, &asrResp); err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("解析响应失败: %w", err))
	}

	if asrResp.Status != 20000000 {
		return "", voice.NewVoiceError(e.Name(), "asr",
			fmt.Errorf("ASR识别失败: status=%d", asrResp.Status))
	}

	return asrResp.Result, nil
}

// getValidToken 获取有效的阿里云Token
func (e *Engine) getValidToken(ctx context.Context) (string, error) {
	currentTime := time.Now().Unix()

	// 检查缓存的token是否有效
	if e.token != "" && currentTime < e.tokenExpireTime {
		return e.token, nil
	}

	// 生成新token
	token, expireTime, err := e.createToken(ctx)
	if err != nil {
		return "", err
	}

	e.token = token
	e.tokenExpireTime = expireTime - 300 // 提前5分钟过期

	return e.token, nil
}

// createToken 创建阿里云Token
func (e *Engine) createToken(ctx context.Context) (string, int64, error) {
	// 构建请求参数
	params := map[string]string{
		"Format":           "JSON",
		"Version":          "2019-02-28",
		"AccessKeyId":      e.accessKeyId,
		"SignatureMethod":  "HMAC-SHA1",
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"SignatureVersion": "1.0",
		"SignatureNonce":   fmt.Sprintf("%d", time.Now().UnixNano()),
		"Action":           "CreateToken",
		"RegionId":         "cn-shanghai",
	}

	// 计算签名
	signature := e.signRequest(params)
	params["Signature"] = signature

	// 构建请求URL
	reqURL := tokenAPIURL + "?" + buildQueryString(params)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("创建请求失败: %w", err)
	}

	// 发送请求
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("读取响应失败: %w", err)
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w", err)
	}

	return tokenResp.Token.Id, tokenResp.Token.ExpireTime, nil
}

// signRequest 为请求生成签名
func (e *Engine) signRequest(params map[string]string) string {
	// 对参数排序
	sortedParams := sortParams(params)

	// 构建待签名字符串
	canonicalizedQueryString := ""
	for i, p := range sortedParams {
		if i > 0 {
			canonicalizedQueryString += "&"
		}
		canonicalizedQueryString += percentEncode(p.key) + "=" + percentEncode(p.value)
	}

	stringToSign := "GET&%2F&" + percentEncode(canonicalizedQueryString)

	// 计算签名
	key := e.accessKeySecret + "&"
	h := hmac.New(sha1.New, []byte(key))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return signature
}

// paramItem 参数项
type paramItem struct {
	key   string
	value string
}

// sortParams 对参数排序
func sortParams(params map[string]string) []paramItem {
	items := make([]paramItem, 0, len(params))
	for k, v := range params {
		items = append(items, paramItem{key: k, value: v})
	}

	// 简单排序
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].key > items[j].key {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	return items
}

// percentEncode URL编码
func percentEncode(s string) string {
	s = url.QueryEscape(s)
	s = strings.ReplaceAll(s, "+", "%20")
	s = strings.ReplaceAll(s, "*", "%2A")
	s = strings.ReplaceAll(s, "%7E", "~")
	return s
}

// buildQueryString 构建查询字符串
func buildQueryString(params map[string]string) string {
	var parts []string
	for k, v := range params {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
	}
	return strings.Join(parts, "&")
}

// cleanText 清除文本中的特殊字符
func cleanText(text string) string {
	// 保留中文、英文、数字和基本标点
	re := regexp.MustCompile(`[^\x{4e00}-\x{9fa5}\x{3040}-\x{30FF}\x{AC00}-\x{D7AF}a-zA-Z0-9äöüÄÖÜáéíóúÁÉÍÓÚàèìòùÀÈÌÒÙâêîôûÂÊÎÔÛçÇñÑ，。！？,.]`)
	return re.ReplaceAllString(text, "")
}

// tokenResponse Token响应结构
type tokenResponse struct {
	Token struct {
		Id         string `json:"Id"`
		ExpireTime int64  `json:"ExpireTime"`
	} `json:"Token"`
}

// asrResponse ASR响应结构
type asrResponse struct {
	Status int    `json:"status"`
	Result string `json:"result"`
}

// init 注册引擎到工厂
func init() {
	voice.RegisterEngine(voice.EngineAli, func(cfg voice.Config) (voice.VoiceEngine, error) {
		return New(cfg)
	})
}
