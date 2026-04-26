// Package deepl 提供 DeepL 翻译服务的实现。
// deepl.go 实现了基于 DeepL 翻译 API 的翻译器。
package deepl

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/translate"
)

const (
	// DeepL 翻译 API 端点
	deeplProEndpoint  = "https://api.deepl.com/v2/translate"
	deeplFreeEndpoint = "https://api-free.deepl.com/v2/translate"
	freeKeySuffix     = ":fx" // Free 账户 API Key 后缀

	// 请求超时时间
	requestTimeout = 10 * time.Second

	// 重试次数
	maxRetries = 3
)

// DeepLTranslator DeepL翻译器实现
type DeepLTranslator struct {
	apiKey string     // DeepL API Key
	apiURL string     // 根据账户类型自动选择端点
	client *http.Client
}

// deeplRequest DeepL翻译 API 请求结构
type deeplRequest struct {
	Text       []string `json:"text"`        // 待翻译文本数组
	TargetLang string   `json:"target_lang"` // 目标语言代码（大写）
	SourceLang string   `json:"source_lang,omitempty"` // 源语言代码（大写），省略表示自动检测
}

// deeplResponse DeepL翻译 API 响应结构
type deeplResponse struct {
	Translations []struct {
		DetectedSourceLanguage string `json:"detected_source_language"` // 检测到的源语言
		Text                   string `json:"text"`                     // 翻译结果
	} `json:"translations"`
	Message string `json:"message,omitempty"` // 错误信息
}

// Config DeepL翻译器配置
type Config struct {
	APIKey string // DeepL API Key
}

// NewDeepLTranslator 创建DeepL翻译器实例
//
// 参数：
//   - cfg: 翻译器配置，包含 APIKey
//
// 返回：
//   - translate.Translator: 翻译器实例
//   - error: 创建失败时的错误信息
//
// 示例：
//
//	translator, err := NewDeepLTranslator(deepl.Config{
//	    APIKey: "your-api-key:fx", // Free 账户以 :fx 结尾
//	})
func NewDeepLTranslator(cfg Config) (translate.Translator, error) {
	// 验证 API Key
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("DeepL API Key 不能为空")
	}

	// 根据 Key 后缀判断 Free/Pro 账户
	apiURL := deeplProEndpoint
	if strings.HasSuffix(cfg.APIKey, freeKeySuffix) {
		apiURL = deeplFreeEndpoint
	}

	return &DeepLTranslator{
		apiKey: cfg.APIKey,
		apiURL: apiURL,
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
// DeepL 翻译语言代码说明：
//   - 自动检测: 省略 source_lang 参数
//   - ZH: 中文（简体）
//   - EN: 英语
//   - DE: 德语
//   - JA: 日语
//   - KO: 韩语
//   - 更多语言代码请参考：https://www.deepl.com/docs-api/translate-text
//
// 注意：DeepL 使用大写语言代码，内部接口使用小写，会自动转换
func (t *DeepLTranslator) Translate(text string, from string, to string) (string, error) {
	// 参数验证
	if text == "" {
		return "", nil
	}
	if to == "" {
		return "", fmt.Errorf("目标语言代码不能为空")
	}

	// 带重试的请求
	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		result, err := t.doRequest(text, from, to)
		if err != nil {
			lastErr = err
			// 对于可重试的错误，进行重试
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
func (t *DeepLTranslator) doRequest(text, from, to string) (string, error) {
	// 构建请求体，语言代码转为大写
	reqBody := deeplRequest{
		Text:       []string{text},       // DeepL 要求 text 为数组
		TargetLang: strings.ToUpper(to),  // 目标语言代码转大写
	}

	// 源语言为空时省略 source_lang，让 DeepL 自动检测
	if from != "" {
		reqBody.SourceLang = strings.ToUpper(from) // 源语言代码转大写
	}

	// 序列化请求体
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 创建请求
	req, err := http.NewRequest(http.MethodPost, t.apiURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "DeepL-Auth-Key "+t.apiKey)

	// 发送请求
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		// 尝试从响应体中提取错误信息
		var errResp deeplResponse
		errMsg := string(respBody)
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Message != "" {
			errMsg = errResp.Message
		}

		switch resp.StatusCode {
		case http.StatusForbidden:
			return "", fmt.Errorf("DeepL 认证失败，请检查 API Key: %s", errMsg)
		case http.StatusTooManyRequests:
			return "", fmt.Errorf("DeepL 请求频率超限，请稍后重试: %s", errMsg)
		case 456:
			return "", fmt.Errorf("DeepL 配额已用尽: %s", errMsg)
		default:
			return "", fmt.Errorf("DeepL 翻译错误 [HTTP %d]: %s", resp.StatusCode, errMsg)
		}
	}

	// 解析响应
	var deeplResp deeplResponse
	if err := json.Unmarshal(respBody, &deeplResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查错误信息
	if deeplResp.Message != "" {
		return "", fmt.Errorf("DeepL 翻译错误: %s", deeplResp.Message)
	}

	// 提取翻译结果
	if len(deeplResp.Translations) == 0 {
		return "", fmt.Errorf("翻译结果为空")
	}

	// 拼接所有翻译结果（支持多段文本）
	var result string
	for i, item := range deeplResp.Translations {
		if i > 0 {
			result += "\n"
		}
		result += item.Text
	}

	return result, nil
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	// 可以根据具体错误类型判断是否可重试
	// 对于超时和频率限制等临时性错误，进行重试
	return true
}

// init 注册DeepL翻译器到工厂
func init() {
	translate.RegisterTranslator(translate.TranslatorDeepL, func() (translate.Translator, error) {
		// 从配置或环境变量获取配置
		// 这里返回错误，提示用户使用 NewDeepLTranslator 直接创建
		return nil, fmt.Errorf("请使用 deepl.NewDeepLTranslator() 创建DeepL翻译器实例")
	})
}
