// Package baidu 提供百度翻译服务的实现。
// baidu.go 实现了基于百度翻译 API 的翻译器。
package baidu

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bstr9/simpleclaw/pkg/translate"
)

const (
	// 百度翻译 API 端点
	baiduEndpoint = "https://fanyi-api.baidu.com"
	baiduPath     = "/api/trans/vip/translate"

	// 请求超时时间
	requestTimeout = 10 * time.Second

	// 重试次数
	maxRetries = 3
)

// 百度翻译 API 错误码定义
// 参考：https://fanyi-api.baidu.com/doc/21
const (
	ErrCodeSuccess      = "52000" // 成功
	ErrCodeTimeout      = "52001" // 请求超时
	ErrCodeSystemError  = "52002" // 系统错误
	ErrCodeUnauthorized = "52003" // 未授权用户
	ErrCodeInvalidParam = "54000" // 必填参数为空
	// 更多错误码请参考百度翻译 API 文档
)

// BaiduTranslator 百度翻译器实现
type BaiduTranslator struct {
	appID  string // 百度翻译 App ID
	appKey string // 百度翻译 App Key
	client *http.Client
}

// baiduResponse 百度翻译 API 响应结构
type baiduResponse struct {
	From        string `json:"from"`
	To          string `json:"to"`
	TransResult []struct {
		Src string `json:"src"` // 原文
		Dst string `json:"dst"` // 译文
	} `json:"trans_result"`
	ErrorCode string `json:"error_code,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}

// Config 百度翻译器配置
type Config struct {
	AppID  string // 百度翻译 App ID
	AppKey string // 百度翻译 App Key
}

// NewBaiduTranslator 创建百度翻译器实例
//
// 参数：
//   - cfg: 翻译器配置，包含 AppID 和 AppKey
//
// 返回：
//   - translate.Translator: 翻译器实例
//   - error: 创建失败时的错误信息
//
// 示例：
//
//	translator, err := NewBaiduTranslator(baidu.Config{
//	    AppID:  "your_app_id",
//	    AppKey: "your_app_key",
//	})
func NewBaiduTranslator(cfg Config) (translate.Translator, error) {
	// 验证必要参数
	if cfg.AppID == "" {
		return nil, fmt.Errorf("百度翻译 App ID 不能为空")
	}
	if cfg.AppKey == "" {
		return nil, fmt.Errorf("百度翻译 App Key 不能为空")
	}

	return &BaiduTranslator{
		appID:  cfg.AppID,
		appKey: cfg.AppKey,
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
// 百度翻译语言代码说明：
//   - auto: 自动检测
//   - zh: 中文
//   - en: 英语
//   - jp: 日语
//   - kor: 韩语
//   - 更多语言代码请参考：https://fanyi-api.baidu.com/doc/21
func (t *BaiduTranslator) Translate(text string, from string, to string) (string, error) {
	// 参数验证
	if text == "" {
		return "", nil
	}
	if to == "" {
		return "", fmt.Errorf("目标语言代码不能为空")
	}

	// 处理源语言：空字符串表示自动检测
	if from == "" {
		from = "auto"
	}

	// 构建请求参数
	salt := generateSalt()
	sign := t.generateSign(text, salt)

	params := url.Values{}
	params.Set("q", text)
	params.Set("from", from)
	params.Set("to", to)
	params.Set("appid", t.appID)
	params.Set("salt", strconv.Itoa(salt))
	params.Set("sign", sign)

	// 带重试的请求
	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		result, err := t.doRequest(params)
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
func (t *BaiduTranslator) doRequest(params url.Values) (string, error) {
	// 构建完整 URL
	requestURL := baiduEndpoint + baiduPath + "?" + params.Encode()

	// 创建请求
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

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
	var baiduResp baiduResponse
	if err := json.Unmarshal(body, &baiduResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查错误码
	if baiduResp.ErrorCode != "" && baiduResp.ErrorCode != ErrCodeSuccess {
		return "", fmt.Errorf("百度翻译错误 [%s]: %s", baiduResp.ErrorCode, baiduResp.ErrorMsg)
	}

	// 提取翻译结果
	if len(baiduResp.TransResult) == 0 {
		return "", fmt.Errorf("翻译结果为空")
	}

	// 拼接所有翻译结果（支持多段文本）
	var result string
	for i, item := range baiduResp.TransResult {
		if i > 0 {
			result += "\n"
		}
		result += item.Dst
	}

	return result, nil
}

// generateSign 生成签名
// 签名算法：MD5(appid + query + salt + appkey)
func (t *BaiduTranslator) generateSign(query string, salt int) string {
	// 构建签名字符串
	signStr := fmt.Sprintf("%s%s%d%s", t.appID, query, salt, t.appKey)

	// 计算 MD5
	hash := md5.New()
	hash.Write([]byte(signStr))
	return hex.EncodeToString(hash.Sum(nil))
}

// generateSalt 生成随机盐值
func generateSalt() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Intn(32768) + 32768 // 范围: 32768 - 65535
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	// 可以根据具体错误类型判断是否可重试
	// 这里简单返回 true，实际应用中可以更精细地判断
	return true
}

// init 注册百度翻译器到工厂
func init() {
	translate.RegisterTranslator(translate.TranslatorBaidu, func() (translate.Translator, error) {
		// 从配置或环境变量获取配置
		// 这里返回错误，提示用户使用 NewBaiduTranslator 直接创建
		return nil, fmt.Errorf("请使用 baidu.NewBaiduTranslator() 创建百度翻译器实例")
	})
}
