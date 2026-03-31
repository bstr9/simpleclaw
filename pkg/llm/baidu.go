// Package llm 提供与各种 LLM 提供商交互的统一接口。
// baidu.go 实现百度文心（ERNIE）模型客户端。
package llm

import (
	"bufio"
	"bytes"
	"github.com/bstr9/simpleclaw/pkg/common"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 百度 API 端点
const (
	baiduTokenURL    = "https://aip.baidubce.com/oauth/2.0/token"
	baiduChatBaseURL = "https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat"
)

// 百度模型标识符
const (
	BaiduModelERNIEBot       = "completions"      // ERNIE-Bot (wenxin)
	BaiduModelERNIEBotTurbo  = "eb-instant"       // ERNIE-Bot-turbo
	BaiduModelERNIEBot4      = "completions_pro"  // ERNIE-Bot 4.0
	BaiduModelERNIEBot8K     = "ernie_bot_8k"     // ERNIE-Bot 8K context
	BaiduModelERNIESpeed     = "ernie_speed"      // ERNIE-Speed
	BaiduModelERNIESpeed128K = "ernie-speed-128k" // ERNIE-Speed 128K context
	BaiduModelERNIELite      = "ernie_lite"       // ERNIE-Lite
	BaiduModelERNIELite8K    = "ernie-lite-8k"    // ERNIE-Lite 8K
	BaiduModelERNIETiny      = "ernie-tiny"       // ERNIE-Tiny
)

// BaiduModel 实现百度文心（ERNIE）模型的 Model 接口。
// 支持流式和非流式响应。
// 认证通过 API Key 和 Secret Key 获取 access_token 完成。
type BaiduModel struct {
	client        *http.Client
	config        ModelConfig
	apiKey        string
	secretKey     string
	accessToken   string
	tokenExpiry   time.Time
	tokenMutex    sync.RWMutex
	supportsTools bool
}

// baiduTokenResponse 表示百度令牌 API 的响应
type baiduTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Error       string `json:"error,omitempty"`
	ErrorDesc   string `json:"error_description,omitempty"`
}

// baiduChatRequest 表示百度对话 API 的请求
type baiduChatRequest struct {
	Messages []baiduMessage `json:"messages"`
	System   string         `json:"system,omitempty"`
	Stream   bool           `json:"stream,omitempty"`
	UserID   string         `json:"user_id,omitempty"`
}

// baiduMessage 表示百度对话 API 中的消息
type baiduMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// baiduChatResponse 表示百度对话 API 的响应
type baiduChatResponse struct {
	ID           string     `json:"id"`
	Object       string     `json:"object"`
	Created      int64      `json:"created"`
	Result       string     `json:"result"`
	Usage        baiduUsage `json:"usage"`
	ErrorCode    string     `json:"error_code,omitempty"`
	ErrorMsg     string     `json:"error_msg,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

// baiduUsage 表示百度响应中的令牌使用情况
type baiduUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// baiduStreamResponse 表示百度 API 的流式响应块
type baiduStreamResponse struct {
	ID              string                `json:"id"`
	Object          string                `json:"object"`
	Created         int64                 `json:"created"`
	SentenceResults []baiduSentenceResult `json:"sentence_results,omitempty"`
	Result          string                `json:"result,omitempty"`
	IsEnd           bool                  `json:"is_end"`
	Usage           baiduUsage            `json:"usage"`
	ErrorCode       string                `json:"error_code,omitempty"`
	ErrorMsg        string                `json:"error_msg,omitempty"`
	FinishReason    string                `json:"finish_reason,omitempty"`
}

// baiduSentenceResult 表示流式响应中的句子
type baiduSentenceResult struct {
	SentenceID int    `json:"sentence_id"`
	Result     string `json:"result"`
	IsEnd      bool   `json:"is_end"`
}

// NewBaiduModel 创建新的百度文心模型实例。
// ModelConfig.APIKey 应包含 API Key (client_id)，
// SecretKey 应通过 Extra["secret_key"] 或单独字段提供。
func NewBaiduModel(cfg ModelConfig) (*BaiduModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("baidu api_key is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	secretKey := cfg.SecretKey
	if secretKey == "" {
		return nil, fmt.Errorf("baidu secret_key is required")
	}

	// 创建 HTTP 客户端
	httpClient := &http.Client{}
	if cfg.RequestTimeout > 0 {
		httpClient.Timeout = time.Duration(cfg.RequestTimeout) * time.Second
	} else {
		httpClient.Timeout = 60 * time.Second
	}

	// 如果指定了代理则配置
	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		httpClient.Transport = transport
	}

	// 百度 ERNIE 模型通常支持函数调用
	supportsTools := isBaiduModelSupportsTools(cfg.Model)

	return &BaiduModel{
		client:        httpClient,
		config:        cfg,
		apiKey:        cfg.APIKey,
		secretKey:     secretKey,
		supportsTools: supportsTools,
	}, nil
}

// isBaiduModelSupportsTools 检查百度模型是否支持函数调用
func isBaiduModelSupportsTools(model string) bool {
	// ERNIE-Bot 4.0 和 ERNIE-Bot 支持函数调用
	supportedModels := map[string]bool{
		BaiduModelERNIEBot:   true,
		BaiduModelERNIEBot4:  true,
		BaiduModelERNIEBot8K: true,
		BaiduModelERNIESpeed: true,
		BaiduModelERNIELite:  true,
	}
	return supportedModels[model]
}

// Name 返回模型标识符
func (m *BaiduModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持函数/工具调用
func (m *BaiduModel) SupportsTools() bool {
	return m.supportsTools
}

// getAccessToken 从百度 OAuth API 获取访问令牌。
// 令牌会被缓存并重复使用直到过期。
func (m *BaiduModel) getAccessToken(ctx context.Context) (string, error) {
	// 检查是否有有效的缓存令牌
	m.tokenMutex.RLock()
	if m.accessToken != "" && time.Now().Before(m.tokenExpiry) {
		token := m.accessToken
		m.tokenMutex.RUnlock()
		return token, nil
	}
	m.tokenMutex.RUnlock()

	// 需要获取新令牌
	m.tokenMutex.Lock()
	defer m.tokenMutex.Unlock()

	// 获取写锁后再次检查
	if m.accessToken != "" && time.Now().Before(m.tokenExpiry) {
		return m.accessToken, nil
	}

	// 构建令牌请求 URL
	reqURL := fmt.Sprintf("%s?grant_type=client_credentials&client_id=%s&client_secret=%s",
		baiduTokenURL,
		url.QueryEscape(m.apiKey),
		url.QueryEscape(m.secretKey),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	var tokenResp baiduTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("token error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	// 缓存令牌，在实际过期前留出缓冲时间
	m.accessToken = tokenResp.AccessToken
	expiryBuffer := 5 * time.Minute
	if tokenResp.ExpiresIn > 0 {
		m.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Add(-expiryBuffer)
	} else {
		// 如果未提供 expires_in，默认为 24 小时
		m.tokenExpiry = time.Now().Add(24 * time.Hour).Add(-expiryBuffer)
	}

	return m.accessToken, nil
}

// Call 执行同步对话完成调用
func (m *BaiduModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	// Merge options with defaults
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	// Get access token
	token, err := m.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Build request
	req := m.buildBaiduRequest(messages, callOpts, false)

	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build API URL
	apiURL := fmt.Sprintf("%s/%s?access_token=%s",
		baiduChatBaseURL,
		m.config.Model,
		url.QueryEscape(token),
	)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	// Execute request
	httpResp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 解析响应
	var baiduResp baiduChatResponse
	if err := json.Unmarshal(respBody, &baiduResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 检查错误
	if baiduResp.ErrorCode != "" {
		return nil, fmt.Errorf("baidu api error [%s]: %s", baiduResp.ErrorCode, baiduResp.ErrorMsg)
	}

	// 构建响应
	result := &Response{
		Content: baiduResp.Result,
		Usage: Usage{
			PromptTokens:     baiduResp.Usage.PromptTokens,
			CompletionTokens: baiduResp.Usage.CompletionTokens,
			TotalTokens:      baiduResp.Usage.TotalTokens,
		},
		FinishReason: baiduResp.FinishReason,
		Model:        m.config.Model,
	}

	return result, nil
}

// CallStream 执行流式对话完成调用
func (m *BaiduModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	token, err := m.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	req := m.buildBaiduRequest(messages, callOpts, true)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/%s?access_token=%s",
		baiduChatBaseURL,
		m.config.Model,
		url.QueryEscape(token),
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	httpResp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		defer httpResp.Body.Close()
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("stream request failed with status %d: %s", httpResp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk, 100)
	go m.processBaiduStream(httpResp.Body, ch)

	return ch, nil
}

// processBaiduStream 处理百度流式响应
func (m *BaiduModel) processBaiduStream(body io.ReadCloser, ch chan<- StreamChunk) {
	defer close(ch)
	defer body.Close()

	reader := bufio.NewReader(body)
	var totalUsage Usage

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			m.handleStreamReadError(err, ch, &totalUsage)
			return
		}

		if shouldContinue := m.processBaiduStreamLine(line, ch, &totalUsage); shouldContinue {
			continue
		} else {
			return
		}
	}
}

// handleStreamReadError 处理流读取错误
func (m *BaiduModel) handleStreamReadError(err error, ch chan<- StreamChunk, totalUsage *Usage) {
	if errors.Is(err, io.EOF) {
		ch <- StreamChunk{Done: true}
		return
	}
	ch <- StreamChunk{Error: fmt.Errorf("stream read error: %w", err)}
}

// processBaiduStreamLine 处理单行流数据
func (m *BaiduModel) processBaiduStreamLine(line string, ch chan<- StreamChunk, totalUsage *Usage) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}

	if !strings.HasPrefix(line, "data: ") {
		return true
	}

	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" {
		m.sendFinalChunk(ch, totalUsage)
		return false
	}

	return m.processBaiduStreamData(data, ch, totalUsage)
}

// sendFinalChunk 发送最终数据块
func (m *BaiduModel) sendFinalChunk(ch chan<- StreamChunk, totalUsage *Usage) {
	if totalUsage.TotalTokens > 0 {
		ch <- StreamChunk{Done: true, Usage: totalUsage}
	} else {
		ch <- StreamChunk{Done: true}
	}
}

// processBaiduStreamData 处理流数据
func (m *BaiduModel) processBaiduStreamData(data string, ch chan<- StreamChunk, totalUsage *Usage) bool {
	var streamResp baiduStreamResponse
	if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("failed to parse stream response: %w", err)}
		return false
	}

	if streamResp.ErrorCode != "" {
		ch <- StreamChunk{Error: fmt.Errorf("baidu api error [%s]: %s", streamResp.ErrorCode, streamResp.ErrorMsg)}
		return false
	}

	if streamResp.Usage.TotalTokens > 0 {
		*totalUsage = Usage{
			PromptTokens:     streamResp.Usage.PromptTokens,
			CompletionTokens: streamResp.Usage.CompletionTokens,
			TotalTokens:      streamResp.Usage.TotalTokens,
		}
	}

	content := extractBaiduStreamContent(streamResp)
	if content != "" {
		ch <- StreamChunk{Delta: content}
	}

	if streamResp.IsEnd {
		ch <- StreamChunk{Done: true, FinishReason: streamResp.FinishReason, Usage: totalUsage}
		return false
	}

	return true
}

// extractBaiduStreamContent 从百度流响应中提取内容
func extractBaiduStreamContent(resp baiduStreamResponse) string {
	if len(resp.SentenceResults) > 0 {
		for _, sr := range resp.SentenceResults {
			if sr.Result != "" {
				return sr.Result
			}
		}
	}
	return resp.Result
}

// buildBaiduRequest 根据消息和选项构建百度对话请求
func (m *BaiduModel) buildBaiduRequest(messages []Message, opts CallOptions, stream bool) baiduChatRequest {
	req := baiduChatRequest{
		Messages: make([]baiduMessage, 0, len(messages)),
		Stream:   stream,
	}

	// 将消息转换为百度格式
	for _, msg := range messages {
		// 跳过工具消息，因为百度的工具支持有限
		if msg.Role == RoleTool {
			continue
		}
		req.Messages = append(req.Messages, baiduMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	// 如果提供了系统提示则设置
	if opts.SystemPrompt != "" {
		req.System = opts.SystemPrompt
	}

	// 如果提供了用户 ID 则设置
	if opts.User != "" {
		req.UserID = opts.User
	}

	return req
}
