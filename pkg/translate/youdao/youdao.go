// Package youdao 提供有道智云翻译服务的实现。
// youdao.go 实现了基于有道翻译 API 的翻译器。
package youdao

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/translate"
)

const (
	// 有道翻译 API 端点
	youdaoEndpoint = "https://openapi.youdao.com/api"

	// 请求超时时间
	requestTimeout = 10 * time.Second

	// 重试次数
	maxRetries = 3
)

// 有道翻译 API 错误码定义
// 参考：https://ai.youdao.com/DOCSIRMA/html/trans/api/wbfy/index.html
const (
	YoudaoErrSuccess        = "0"   // 成功
	YoudaoErrMissingParam   = "101" // 缺少必填参数
	YoudaoErrLangNotSupport = "102" // 语言不支持
	YoudaoErrInvalidAppID   = "108" // 应用 ID 无效
	YoudaoErrInvalidKey     = "110" // 应用密钥无效
	YoudaoErrSignFailed     = "202" // 签名检验失败
	YoudaoErrTextTooLong    = "207" // 文本过长
)

// YoudaoTranslator 有道翻译器实现
type YoudaoTranslator struct {
	appKey    string // 有道翻译 App Key
	appSecret string // 有道翻译 App Secret
	client    *http.Client
}

// youdaoResponse 有道翻译 API 响应结构
type youdaoResponse struct {
	ErrorCode   string   `json:"errorCode"`
	Query       string   `json:"query"`
	Translation []string `json:"translation"`
	ErrorMsg    string   `json:"errorMsg,omitempty"`
}

// Config 有道翻译器配置
type Config struct {
	AppKey    string // 有道翻译 App Key
	AppSecret string // 有道翻译 App Secret
}

// NewYoudaoTranslator 创建有道翻译器实例
//
// 参数：
//   - cfg: 翻译器配置，包含 AppKey 和 AppSecret
//
// 返回：
//   - translate.Translator: 翻译器实例
//   - error: 创建失败时的错误信息
//
// 示例：
//
//	translator, err := NewYoudaoTranslator(youdao.Config{
//	    AppKey:    "your_app_key",
//	    AppSecret: "your_app_secret",
//	})
func NewYoudaoTranslator(cfg Config) (translate.Translator, error) {
	// 验证必要参数
	if cfg.AppKey == "" {
		return nil, fmt.Errorf("有道翻译 App Key 不能为空")
	}
	if cfg.AppSecret == "" {
		return nil, fmt.Errorf("有道翻译 App Secret 不能为空")
	}

	return &YoudaoTranslator{
		appKey:    cfg.AppKey,
		appSecret: cfg.AppSecret,
		client: &http.Client{
			Timeout: requestTimeout,
		},
	}, nil
}

// Translate 实现翻译接口
// 将文本从源语言翻译到目标语言
//
// 参数：
//   - text: 需要翻译的文本
//   - from: 源语言代码，空字符串表示自动检测
//   - to: 目标语言代码
//
// 有道翻译语言代码说明：
//   - auto: 自动检测
//   - zh-CHS: 简体中文（注意：不是 zh）
//   - en: 英语
//   - ja: 日语
//   - ko: 韩语
//   - fr: 法语
//   - de: 德语
//   - 更多语言代码请参考：https://ai.youdao.com/DOCSIRMA/html/trans/api/wbfy/index.html
func (t *YoudaoTranslator) Translate(text string, from string, to string) (string, error) {
	// 参数验证
	if text == "" {
		return "", nil
	}
	if to == "" {
		return "", fmt.Errorf("目标语言代码不能为空")
	}

	// 映射语言代码
	from = mapLanguageCode(from)
	to = mapLanguageCode(to)

	// 带重试的请求
	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		result, err := t.doRequest(text, from, to)
		if err != nil {
			lastErr = err
			// 对于超时和系统错误，进行重试
			if isRetryableError(err) {
				time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond)
				continue
			}
			return "", err
		}
		return result, nil
	}

	return "", fmt.Errorf("翻译请求失败，已重试 %d 次: %w", maxRetries, lastErr)
}

// doRequest 执行翻译请求
// 有道翻译 API 使用 POST application/x-www-form-urlencoded 方式提交
func (t *YoudaoTranslator) doRequest(q, from, to string) (string, error) {
	// 构建请求参数
	salt := generateSalt()
	curtime := strconv.FormatInt(time.Now().Unix(), 10)
	sign := t.generateSign(q, salt, curtime)

	params := url.Values{}
	params.Set("q", q)
	params.Set("from", from)
	params.Set("to", to)
	params.Set("appKey", t.appKey)
	params.Set("salt", salt)
	params.Set("sign", sign)
	params.Set("signType", "v3")
	params.Set("curtime", curtime)

	// 创建 POST 请求
	req, err := http.NewRequest(http.MethodPost, youdaoEndpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 发送请求
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var youdaoResp youdaoResponse
	if err := json.Unmarshal(body, &youdaoResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查错误码
	if youdaoResp.ErrorCode != YoudaoErrSuccess {
		return "", fmt.Errorf("有道翻译错误 [%s]: %s", youdaoResp.ErrorCode, errorMsg(youdaoResp.ErrorCode))
	}

	// 提取翻译结果
	if len(youdaoResp.Translation) == 0 {
		return "", fmt.Errorf("翻译结果为空")
	}

	return youdaoResp.Translation[0], nil
}

// generateSign 生成签名
// 签名算法：SHA256(appKey + input + salt + curtime + appSecret)
func (t *YoudaoTranslator) generateSign(q, salt, curtime string) string {
	// 截断文本用于签名计算
	input := truncate(q)
	signStr := t.appKey + input + salt + curtime + t.appSecret

	// 计算 SHA256
	hash := sha256.Sum256([]byte(signStr))
	return hex.EncodeToString(hash[:])
}

// truncate 截断文本用于签名计算
// 规则：文本长度≤20时直接使用原文；文本长度>20时取前10个字符+长度+后10个字符
// 注意：使用 rune 索引以正确处理中文等多字节字符
func truncate(q string) string {
	r := []rune(q)
	if len(r) <= 20 {
		return q
	}
	return string(r[:10]) + strconv.Itoa(len(r)) + string(r[len(r)-10:])
}

// mapLanguageCode 将标准语言代码映射为有道语言代码
func mapLanguageCode(lang string) string {
	if lang == "" {
		return "auto"
	}
	if lang == "zh" {
		return "zh-CHS"
	}
	return lang
}

// generateSalt 生成随机盐值
func generateSalt() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return strconv.Itoa(r.Intn(32768) + 32768) // 范围: 32768 - 65535
}

// errorMsg 根据错误码返回错误信息
func errorMsg(code string) string {
	switch code {
	case YoudaoErrMissingParam:
		return "缺少必填参数"
	case YoudaoErrLangNotSupport:
		return "语言不支持"
	case YoudaoErrInvalidAppID:
		return "应用 ID 无效"
	case YoudaoErrInvalidKey:
		return "应用密钥无效"
	case YoudaoErrSignFailed:
		return "签名检验失败"
	case YoudaoErrTextTooLong:
		return "文本过长"
	default:
		return "未知错误"
	}
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	// 可以根据具体错误类型判断是否可重试
	// 这里简单返回 true，实际应用中可以更精细地判断
	return true
}

// init 注册有道翻译器到工厂
func init() {
	translate.RegisterTranslator(translate.TranslatorYoudao, func() (translate.Translator, error) {
		// 从配置或环境变量获取配置
		// 这里返回错误，提示用户使用 NewYoudaoTranslator 直接创建
		return nil, fmt.Errorf("请使用 youdao.NewYoudaoTranslator() 创建有道翻译器实例")
	})
}
