// Package google 提供Google翻译服务的实现。
// google.go 实现了基于Google Cloud Translation API v2 的翻译器。
package google

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bstr9/simpleclaw/pkg/translate"
)

const (
	// Google 翻译 API 端点
	googleEndpoint = "https://translation.googleapis.com/language/translate/v2"

	// 请求超时时间
	requestTimeout = 10 * time.Second

	// 重试次数
	maxRetries = 3
)

// GoogleTranslator Google翻译器实现
type GoogleTranslator struct {
	apiKey string // Google Cloud API Key
	client *http.Client
}

// googleRequest Google翻译 API 请求结构
type googleRequest struct {
	Q      string `json:"q"`      // 需要翻译的文本
	Source string `json:"source,omitempty"` // 源语言代码，省略表示自动检测
	Target string `json:"target"`          // 目标语言代码
	Format string `json:"format"`          // 文本格式：text 或 html
}

// googleResponse Google翻译 API 响应结构
type googleResponse struct {
	Data struct {
		Translations []struct {
			TranslatedText         string `json:"translatedText"`          // 翻译结果
			DetectedSourceLanguage string `json:"detectedSourceLanguage"`  // 检测到的源语言
		} `json:"translations"`
	} `json:"data"`
	Error *struct {
		Code    int    `json:"code"`    // 错误码
		Message string `json:"message"` // 错误信息
		Status  string `json:"status"`  // 错误状态
	} `json:"error,omitempty"`
}

// Config Google翻译器配置
type Config struct {
	APIKey string // Google Cloud API Key
}

// NewGoogleTranslator 创建Google翻译器实例
//
// 参数：
//   - cfg: 翻译器配置，包含 API Key
//
// 返回：
//   - translate.Translator: 翻译器实例
//   - error: 创建失败时的错误信息
//
// 示例：
//
//	translator, err := NewGoogleTranslator(google.Config{
//	    APIKey: "your_api_key",
//	})
func NewGoogleTranslator(cfg Config) (translate.Translator, error) {
	// 验证必要参数
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Google翻译 API Key 不能为空")
	}

	return &GoogleTranslator{
		apiKey: cfg.APIKey,
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
// Google翻译语言代码说明：
//   - 空字符串: 自动检测（省略source字段）
//   - zh: 中文
//   - en: 英语
//   - ja: 日语
//   - ko: 韩语
//   - 更多语言代码请参考：https://cloud.google.com/translate/docs/languages
func (t *GoogleTranslator) Translate(text string, from string, to string) (string, error) {
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
func (t *GoogleTranslator) doRequest(q, from, to string) (string, error) {
	// 构建请求体
	reqBody := googleRequest{
		Q:      q,
		Target: to,
		Format: "text", // 使用纯文本格式，避免HTML处理
	}

	// 源语言为空时省略source字段，由API自动检测
	if from != "" {
		reqBody.Source = from
	}

	// 序列化请求体
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// 构建完整 URL（包含 API Key）
	requestURL := googleEndpoint + "?key=" + t.apiKey

	// 创建请求
	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

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
	var googleResp googleResponse
	if err := json.Unmarshal(body, &googleResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查错误
	if googleResp.Error != nil {
		return "", fmt.Errorf("Google翻译错误 [%d]: %s", googleResp.Error.Code, googleResp.Error.Message)
	}

	// 提取翻译结果
	if len(googleResp.Data.Translations) == 0 {
		return "", fmt.Errorf("翻译结果为空")
	}

	// 拼接所有翻译结果（支持多段文本）
	var result string
	for i, item := range googleResp.Data.Translations {
		if i > 0 {
			result += "\n"
		}
		result += item.TranslatedText
	}

	return result, nil
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	// 可以根据具体错误类型判断是否可重试
	// 这里简单返回 true，实际应用中可以更精细地判断
	return true
}

// init 注册Google翻译器到工厂
func init() {
	translate.RegisterTranslator(translate.TranslatorGoogle, func() (translate.Translator, error) {
		// 从配置或环境变量获取配置
		// 这里返回错误，提示用户使用 NewGoogleTranslator 直接创建
		return nil, fmt.Errorf("请使用 google.NewGoogleTranslator() 创建Google翻译器实例")
	})
}
