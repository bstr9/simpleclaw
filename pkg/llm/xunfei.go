// Package llm 提供与各种 LLM 提供商交互的统一接口。
// xunfei.go 通过 WebSocket API 实现科大讯飞星火模型。
package llm

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// 星火模型域常量
const (
	SparkDomainLite    = "lite"
	SparkDomainV2      = "generalv2"
	SparkDomainV3      = "generalv3"
	SparkDomainPro128K = "pro-128k"
	SparkDomainV35     = "generalv3.5"
	SparkDomainV4Ultra = "4.0Ultra"
)

// 星火模型 WebSocket URL
var sparkURLs = map[string]string{
	SparkDomainLite:    "wss://spark-api.xf-yun.com/v1.1/chat",
	SparkDomainV2:      "wss://spark-api.xf-yun.com/v2.1/chat",
	SparkDomainV3:      "wss://spark-api.xf-yun.com/v3.1/chat",
	SparkDomainPro128K: "wss://spark-api.xf-yun.com/chat/pro-128k",
	SparkDomainV35:     "wss://spark-api.xf-yun.com/v3.5/chat",
	SparkDomainV4Ultra: "wss://spark-api.xf-yun.com/v4.0/chat",
}

// XunfeiModel 实现科大讯飞星火 API 的 Model 接口。
// 星火使用 WebSocket 进行流式通信，采用 HMAC-SHA256 签名认证。
type XunfeiModel struct {
	config    ModelConfig
	appID     string
	apiKey    string
	apiSecret string
	domain    string
	sparkURL  string
	host      string
	path      string
}

// XunfeiConfig 包含讯飞特有的配置
type XunfeiConfig struct {
	AppID     string
	APIKey    string
	APISecret string
	Domain    string // 星火模型域 (lite, generalv2, generalv3 等)
}

// NewXunfeiModel 创建新的讯飞星火模型实例。
// 该模型需要 AppID、APIKey 和 APISecret 进行认证。
func NewXunfeiModel(cfg ModelConfig) (*XunfeiModel, error) {
	appID, apiKey, apiSecret, domain := extractXunfeiConfig(cfg)
	appID, apiKey, apiSecret = parseAPIKeyFormat(cfg.APIKey, appID, apiKey, apiSecret)

	if err := validateXunfeiConfig(appID, apiKey, apiSecret); err != nil {
		return nil, err
	}

	sparkURL := resolveSparkURL(domain, cfg.APIBase)
	parsedURL, err := url.Parse(sparkURL)
	if err != nil {
		return nil, fmt.Errorf("invalid spark URL: %w", err)
	}

	return &XunfeiModel{
		config:    cfg,
		appID:     appID,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		domain:    domain,
		sparkURL:  sparkURL,
		host:      parsedURL.Host,
		path:      parsedURL.Path,
	}, nil
}

// extractXunfeiConfig 从配置中提取讯飞参数
func extractXunfeiConfig(cfg ModelConfig) (appID, apiKey, apiSecret, domain string) {
	domain = SparkDomainV35
	if cfg.Extra == nil {
		return "", cfg.APIKey, "", domain
	}
	if v, ok := cfg.Extra["app_id"].(string); ok {
		appID = v
	}
	if v, ok := cfg.Extra["api_key"].(string); ok {
		apiKey = v
	}
	if v, ok := cfg.Extra["api_secret"].(string); ok {
		apiSecret = v
	}
	if v, ok := cfg.Extra["domain"].(string); ok {
		domain = v
	}
	return appID, apiKey, apiSecret, domain
}

// parseAPIKeyFormat 解析组合格式的 APIKey
func parseAPIKeyFormat(apiKeyRaw, appID, apiKey, apiSecret string) (string, string, string) {
	if !strings.Contains(apiKeyRaw, ":") {
		return appID, apiKey, apiSecret
	}
	parts := strings.Split(apiKeyRaw, ":")
	if len(parts) >= 3 {
		return parts[0], parts[1], parts[2]
	}
	return appID, apiKey, apiSecret
}

// validateXunfeiConfig 验证讯飞配置参数
func validateXunfeiConfig(appID, apiKey, apiSecret string) error {
	if appID == "" {
		return fmt.Errorf("xunfei app_id is required")
	}
	if apiKey == "" {
		return fmt.Errorf("xunfei api_key is required")
	}
	if apiSecret == "" {
		return fmt.Errorf("xunfei api_secret is required")
	}
	return nil
}

// resolveSparkURL 获取星火 API URL
func resolveSparkURL(domain, apiBase string) string {
	if sparkURL, ok := sparkURLs[domain]; ok {
		return sparkURL
	}
	if apiBase != "" {
		return apiBase
	}
	return sparkURLs[SparkDomainV35]
}

// Name 返回模型标识符
func (m *XunfeiModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回 false，因为星火模型不支持通过 WebSocket 进行工具调用
func (m *XunfeiModel) SupportsTools() bool {
	return false
}

// Call 执行同步对话完成调用
func (m *XunfeiModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	// 收集所有流式块
	chunks := make([]string, 0)
	var usage *Usage

	streamCh, err := m.CallStream(ctx, messages, opts...)
	if err != nil {
		return nil, err
	}

	for chunk := range streamCh {
		if chunk.Error != nil {
			return nil, chunk.Error
		}
		if chunk.Delta != "" {
			chunks = append(chunks, chunk.Delta)
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}

	result := &Response{
		Content: strings.Join(chunks, ""),
		Model:   m.config.Model,
	}
	if usage != nil {
		result.Usage = *usage
	}

	return result, nil
}

// CallStream 通过 WebSocket 执行流式对话完成调用
func (m *XunfeiModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	// Merge options with defaults
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	// 生成带认证签名的 WebSocket URL
	wsURL, err := m.generateAuthURL()
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth URL: %w", err)
	}

	// 创建输出通道
	ch := make(chan StreamChunk, 100)

	// 在 goroutine 中启动 WebSocket 连接
	go m.handleWebSocket(ctx, wsURL, messages, callOpts, ch)

	return ch, nil
}

// handleWebSocket 管理 WebSocket 连接和流式处理
func (m *XunfeiModel) handleWebSocket(ctx context.Context, wsURL string, messages []Message, opts CallOptions, ch chan<- StreamChunk) {
	defer close(ch)

	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("websocket connection error: %w", err)}
		return
	}
	defer conn.Close()

	req := m.buildRequest(messages, opts)
	if err := conn.WriteJSON(req); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("websocket write error: %w", err)}
		return
	}

	m.readWebSocketMessages(ctx, conn, ch)
}

func (m *XunfeiModel) readWebSocketMessages(ctx context.Context, conn *websocket.Conn, ch chan<- StreamChunk) {
	for {
		select {
		case <-ctx.Done():
			ch <- StreamChunk{Error: ctx.Err()}
			return
		default:
			if !m.readAndProcessMessage(conn, ch) {
				return
			}
		}
	}
}

func (m *XunfeiModel) readAndProcessMessage(conn *websocket.Conn, ch chan<- StreamChunk) bool {
	m.setReadDeadline(conn)

	_, message, err := conn.ReadMessage()
	if err != nil {
		return m.handleReadError(err, ch)
	}

	return m.processSparkResponse(message, ch)
}

func (m *XunfeiModel) setReadDeadline(conn *websocket.Conn) {
	if m.config.RequestTimeout > 0 {
		conn.SetReadDeadline(time.Now().Add(time.Duration(m.config.RequestTimeout) * time.Second))
	} else {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	}
}

func (m *XunfeiModel) handleReadError(err error, ch chan<- StreamChunk) bool {
	if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		ch <- StreamChunk{Done: true}
		return false
	}
	ch <- StreamChunk{Error: fmt.Errorf("websocket read error: %w", err)}
	return false
}

func (m *XunfeiModel) processSparkResponse(message []byte, ch chan<- StreamChunk) bool {
	var resp sparkResponse
	if err := json.Unmarshal(message, &resp); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("json unmarshal error: %w", err)}
		return false
	}

	if resp.Header.Code != 0 {
		ch <- StreamChunk{
			Error: fmt.Errorf("spark API error: code=%d, message=%s", resp.Header.Code, resp.Header.Message),
		}
		return false
	}

	m.sendContentChunk(&resp, ch)

	if resp.Payload.Choices.Status == 2 {
		m.sendFinalChunk(&resp, ch)
		return false
	}

	return true
}

func (m *XunfeiModel) sendContentChunk(resp *sparkResponse, ch chan<- StreamChunk) {
	if len(resp.Payload.Choices.Text) > 0 {
		content := resp.Payload.Choices.Text[0].Content
		if content != "" {
			ch <- StreamChunk{Delta: content}
		}
	}
}

func (m *XunfeiModel) sendFinalChunk(resp *sparkResponse, ch chan<- StreamChunk) {
	if resp.Payload.Usage != nil {
		ch <- StreamChunk{
			Usage: &Usage{
				PromptTokens:     resp.Payload.Usage.Text.PromptTokens,
				CompletionTokens: resp.Payload.Usage.Text.CompletionTokens,
				TotalTokens:      resp.Payload.Usage.Text.TotalTokens,
			},
			Done: true,
		}
	} else {
		ch <- StreamChunk{Done: true}
	}
}

// generateAuthURL 生成带 HMAC-SHA256 认证签名的 WebSocket URL
func (m *XunfeiModel) generateAuthURL() (string, error) {
	// 生成 RFC1123 格式的时间戳
	now := time.Now().UTC()
	date := now.Format(time.RFC1123)

	// 构建签名原始字符串
	signatureOrigin := fmt.Sprintf("host: %s\ndate: %s\nGET %s HTTP/1.1", m.host, date, m.path)

	// 计算 HMAC-SHA256 签名
	h := hmac.New(sha256.New, []byte(m.apiSecret))
	h.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 构建授权字符串
	authorizationOrigin := fmt.Sprintf(`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`, m.apiKey, signature)
	authorization := base64.StdEncoding.EncodeToString([]byte(authorizationOrigin))

	// 构建带查询参数的最终 URL
	params := url.Values{}
	params.Set("authorization", authorization)
	params.Set("date", date)
	params.Set("host", m.host)

	return fmt.Sprintf("%s?%s", m.sparkURL, params.Encode()), nil
}

// buildRequest 构建星火 API 请求载荷
func (m *XunfeiModel) buildRequest(messages []Message, opts CallOptions) *sparkRequest {
	text := m.convertMessagesToSpark(messages)

	req := &sparkRequest{
		Header: sparkHeader{
			AppID: m.appID,
			UID:   "simpleclaw",
		},
		Parameter: sparkParameter{
			Chat: sparkChatParam{
				Domain:    m.domain,
				MaxTokens: 2048,
				Auditing:  "default",
			},
		},
		Payload: sparkPayload{
			Message: sparkMessagePayload{
				Text: text,
			},
		},
	}

	m.applyRequestOptions(req, opts)

	return req
}

func (m *XunfeiModel) convertMessagesToSpark(messages []Message) []sparkMessage {
	text := make([]sparkMessage, len(messages))
	for i, msg := range messages {
		text[i] = sparkMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}
	return text
}

func (m *XunfeiModel) applyRequestOptions(req *sparkRequest, opts CallOptions) {
	if opts.Temperature != nil {
		req.Parameter.Chat.Temperature = *opts.Temperature
	} else {
		req.Parameter.Chat.Temperature = 0.5
	}

	if opts.MaxTokens > 0 {
		req.Parameter.Chat.MaxTokens = opts.MaxTokens
	}

	if opts.TopP != nil {
		req.Parameter.Chat.TopP = *opts.TopP
	}
}

// 星火 API 请求/响应结构

type sparkRequest struct {
	Header    sparkHeader    `json:"header"`
	Parameter sparkParameter `json:"parameter"`
	Payload   sparkPayload   `json:"payload"`
}

type sparkHeader struct {
	AppID string `json:"app_id"`
	UID   string `json:"uid"`
}

type sparkParameter struct {
	Chat sparkChatParam `json:"chat"`
}

type sparkChatParam struct {
	Domain      string  `json:"domain"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	MaxTokens   int     `json:"max_tokens"`
	Auditing    string  `json:"auditing"`
}

type sparkPayload struct {
	Message sparkMessagePayload `json:"message"`
}

type sparkMessagePayload struct {
	Text []sparkMessage `json:"text"`
}

type sparkMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type sparkResponse struct {
	Header  sparkResponseHeader  `json:"header"`
	Payload sparkResponsePayload `json:"payload"`
}

type sparkResponseHeader struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	SID     string `json:"sid"`
	Status  int    `json:"status"`
}

type sparkResponsePayload struct {
	Choices sparkChoices `json:"choices"`
	Usage   *sparkUsage  `json:"usage,omitempty"`
}

type sparkChoices struct {
	Status int             `json:"status"`
	Seq    int             `json:"seq"`
	Text   []sparkTextItem `json:"text"`
}

type sparkTextItem struct {
	Index   int    `json:"index"`
	Content string `json:"content"`
	Role    string `json:"role"`
}

type sparkUsage struct {
	Text sparkTextUsage `json:"text"`
}

type sparkTextUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
