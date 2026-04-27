// Package llm 提供与各种 LLM 提供商交互的统一接口。
// gemini.go 实现 Google Gemini API 模型客户端。
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
	"time"
)

// Gemini API 基础 URL
const (
	GeminiDefaultBaseURL = "https://generativelanguage.googleapis.com"
	GeminiAPIVersion     = "v1beta"
)

// GeminiModel 实现 Google Gemini API 的 Model 接口
// 支持 streaming 和非 streaming 模式，支持 tool calling
type GeminiModel struct {
	client        *http.Client
	config        ModelConfig
	apiKey        string
	supportsTools bool
}

// Gemini API 请求结构体

// geminiRequest 表示 Gemini API 请求
type geminiRequest struct {
	Contents          []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	SafetySettings    []geminiSafetySetting   `json:"safetySettings,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
	Tools             []geminiTool            `json:"tools,omitempty"`
}

// geminiContent 表示 Gemini 内容结构
type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

// geminiPart 表示 Gemini 内容部分
type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	InlineData       *geminiInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

// geminiInlineData 表示内联数据（如图片）
type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// geminiFunctionCall 表示函数调用
type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// geminiFunctionResponse 表示函数响应
type geminiFunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]any `json:"response"`
}

// geminiSafetySetting 安全设置
type geminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// geminiGenerationConfig 生成配置
type geminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

// geminiTool 工具定义
type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

// geminiFunctionDeclaration 函数声明
type geminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// Gemini API 响应结构体

// geminiResponse 表示 Gemini API 响应
type geminiResponse struct {
	Candidates    []geminiCandidate    `json:"candidates,omitempty"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
	Error         *geminiError         `json:"error,omitempty"`
}

// geminiCandidate 表示候选响应
type geminiCandidate struct {
	Content       geminiContent        `json:"content,omitempty"`
	FinishReason  string               `json:"finishReason,omitempty"`
	SafetyRatings []geminiSafetyRating `json:"safetyRatings,omitempty"`
}

// geminiSafetyRating 安全评级
type geminiSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// geminiUsageMetadata 使用元数据
type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// geminiError 错误信息
type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// NewGeminiModel 创建新的 Gemini 模型实例
func NewGeminiModel(cfg ModelConfig) (*GeminiModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("gemini api_key 是必需的")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("model 是必需的")
	}

	// 设置默认 API Base URL
	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = GeminiDefaultBaseURL
	}

	// 创建 HTTP 客户端
	httpClient := &http.Client{}
	if cfg.RequestTimeout > 0 {
		httpClient.Timeout = time.Duration(cfg.RequestTimeout) * time.Second
	} else {
		httpClient.Timeout = 120 * time.Second // Gemini 响应可能较慢
	}

	// 如果指定了代理则配置代理
	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("无效的代理 URL: %w", err)
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		httpClient.Transport = transport
	}

	return &GeminiModel{
		client:        httpClient,
		config:        cfg,
		apiKey:        cfg.APIKey,
		supportsTools: true, // Gemini 支持函数调用
	}, nil
}

// Name 返回模型标识符
func (m *GeminiModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持函数/工具调用
func (m *GeminiModel) SupportsTools() bool {
	return m.supportsTools
}

// Call 执行同步聊天完成调用
func (m *GeminiModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	// 合并选项与默认值
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	// 构建 Gemini 请求
	req := m.buildGeminiRequest(messages, callOpts)

	// 序列化请求体
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 构建 API URL
	apiBase := m.config.APIBase
	if apiBase == "" {
		apiBase = GeminiDefaultBaseURL
	}
	apiURL := fmt.Sprintf("%s/%s/models/%s:generateContent?key=%s",
		strings.TrimSuffix(apiBase, "/"),
		GeminiAPIVersion,
		m.config.Model,
		url.QueryEscape(m.apiKey),
	)

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	// 执行请求
	httpResp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer httpResp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查错误
	if geminiResp.Error != nil {
		return nil, fmt.Errorf("gemini api 错误 [%d]: %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	// 检查 HTTP 状态码
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini api 错误 [状态码 %d]: %s", httpResp.StatusCode, string(respBody))
	}

	// 转换为标准响应格式
	return m.convertToResponse(&geminiResp), nil
}

// CallStream 执行流式聊天完成调用
func (m *GeminiModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	// 合并选项与默认值
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	// 构建 Gemini 请求
	req := m.buildGeminiRequest(messages, callOpts)

	// 序列化请求体
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 构建 API URL（流式使用 streamGenerateContent 并添加 alt=sse 参数）
	apiBase := m.config.APIBase
	if apiBase == "" {
		apiBase = GeminiDefaultBaseURL
	}
	apiURL := fmt.Sprintf("%s/%s/models/%s:streamGenerateContent?alt=sse&key=%s",
		strings.TrimSuffix(apiBase, "/"),
		GeminiAPIVersion,
		m.config.Model,
		url.QueryEscape(m.apiKey),
	)

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	httpReq.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	// 执行请求
	httpResp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	// 检查 HTTP 状态码
	if httpResp.StatusCode != http.StatusOK {
		defer httpResp.Body.Close()
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("流式请求失败，状态码 %d: %s", httpResp.StatusCode, string(respBody))
	}

	// 创建输出通道
	ch := make(chan StreamChunk, 100)

	// 启动 goroutine 读取流
	go m.readStream(httpResp, ch)

	return ch, nil
}

// readStream 读取 SSE 流并转换为 StreamChunk
func (m *GeminiModel) readStream(httpResp *http.Response, ch chan<- StreamChunk) {
	defer close(ch)
	defer httpResp.Body.Close()

	reader := bufio.NewReader(httpResp.Body)
	var allToolCalls []ToolCall
	var lastFinishReason string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				m.sendFinalChunk(ch, allToolCalls, lastFinishReason)
				return
			}
			ch <- StreamChunk{Error: fmt.Errorf("流读取错误: %w", err)}
			return
		}

		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			m.sendFinalChunk(ch, allToolCalls, "")
			return
		}

		m.processGeminiStreamData(data, ch, &allToolCalls, &lastFinishReason)
	}
}

// sendFinalChunk 发送最终块
func (m *GeminiModel) sendFinalChunk(ch chan<- StreamChunk, toolCalls []ToolCall, finishReason string) {
	reason := "stop"
	if len(toolCalls) > 0 {
		reason = "tool_calls"
	} else if finishReason != "" {
		reason = finishReason
	}
	ch <- StreamChunk{
		Done:         true,
		FinishReason: reason,
	}
}

// processGeminiStreamData 处理 Gemini 流数据
func (m *GeminiModel) processGeminiStreamData(data string, ch chan<- StreamChunk, allToolCalls *[]ToolCall, lastFinishReason *string) {
	var geminiResp geminiResponse
	if err := json.Unmarshal([]byte(data), &geminiResp); err != nil {
		return
	}

	if geminiResp.Error != nil {
		ch <- StreamChunk{
			Error: fmt.Errorf("gemini api 错误 [%d]: %s", geminiResp.Error.Code, geminiResp.Error.Message),
		}
		return
	}

	if len(geminiResp.Candidates) == 0 {
		return
	}

	candidate := geminiResp.Candidates[0]
	if candidate.FinishReason != "" {
		*lastFinishReason = strings.ToLower(candidate.FinishReason)
	}

	m.processCandidateParts(candidate, ch, allToolCalls)
}

// processCandidateParts 处理候选响应部分
func (m *GeminiModel) processCandidateParts(candidate geminiCandidate, ch chan<- StreamChunk, allToolCalls *[]ToolCall) {
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			ch <- StreamChunk{Delta: part.Text}
		}

		if part.FunctionCall != nil {
			m.handleFunctionCall(part, ch, allToolCalls)
		}
	}
}

// handleFunctionCall 处理函数调用
func (m *GeminiModel) handleFunctionCall(part geminiPart, ch chan<- StreamChunk, allToolCalls *[]ToolCall) {
	argsJSON, _ := json.Marshal(part.FunctionCall.Args)
	toolCall := ToolCall{
		ID:   fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), len(*allToolCalls)),
		Type: "function",
		Function: FunctionCall{
			Name:      part.FunctionCall.Name,
			Arguments: string(argsJSON),
		},
	}
	*allToolCalls = append(*allToolCalls, toolCall)

	ch <- StreamChunk{
		ToolCallDelta: &ToolCallDelta{
			Index: len(*allToolCalls) - 1,
			ID:    toolCall.ID,
			Type:  "function",
			Function: &FunctionCallDelta{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		},
	}
}

// buildGeminiRequest 构建 Gemini API 请求
func (m *GeminiModel) buildGeminiRequest(messages []Message, opts CallOptions) geminiRequest {
	req := geminiRequest{
		Contents: make([]geminiContent, 0, len(messages)),
		SafetySettings: []geminiSafetySetting{
			{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
		},
	}

	systemPrompt, filteredMessages := extractSystemPrompt(messages, opts.SystemPrompt)
	m.setSystemInstruction(&req, systemPrompt)
	m.convertMessages(&req, filteredMessages)
	m.setGenerationConfig(&req, opts)

	if len(opts.Tools) > 0 {
		req.Tools = m.convertToolsToGeminiFormat(opts.Tools)
	}

	return req
}

// extractSystemPrompt 提取系统提示词
func extractSystemPrompt(messages []Message, defaultPrompt string) (string, []Message) {
	systemPrompt := defaultPrompt
	var filteredMessages []Message
	for _, msg := range messages {
		if msg.Role == RoleSystem && systemPrompt == "" {
			systemPrompt = msg.Content
			continue
		}
		if msg.Role == RoleSystem {
			continue
		}
		filteredMessages = append(filteredMessages, msg)
	}
	return systemPrompt, filteredMessages
}

// setSystemInstruction 设置系统指令
func (m *GeminiModel) setSystemInstruction(req *geminiRequest, systemPrompt string) {
	if systemPrompt == "" {
		return
	}
	req.SystemInstruction = &geminiContent{
		Parts: []geminiPart{{Text: systemPrompt}},
	}
}

// convertMessages 转换消息格式
func (m *GeminiModel) convertMessages(req *geminiRequest, messages []Message) {
	for _, msg := range messages {
		content := m.convertMessageToGeminiContent(msg)
		if content != nil {
			req.Contents = append(req.Contents, *content)
		}
	}
}

// setGenerationConfig 设置生成配置
func (m *GeminiModel) setGenerationConfig(req *geminiRequest, opts CallOptions) {
	genConfig := &geminiGenerationConfig{}
	if opts.Temperature != nil {
		genConfig.Temperature = opts.Temperature
	}
	if opts.TopP != nil {
		genConfig.TopP = opts.TopP
	}
	if opts.MaxTokens > 0 {
		genConfig.MaxOutputTokens = &opts.MaxTokens
	}
	if len(opts.Stop) > 0 {
		genConfig.StopSequences = opts.Stop
	}
	if genConfig.Temperature != nil || genConfig.TopP != nil ||
		genConfig.MaxOutputTokens != nil || len(genConfig.StopSequences) > 0 {
		req.GenerationConfig = genConfig
	}
}

// convertMessageToGeminiContent 将 Message 转换为 Gemini 内容格式
func (m *GeminiModel) convertMessageToGeminiContent(msg Message) *geminiContent {
	// 转换角色
	var role string
	switch msg.Role {
	case RoleUser, RoleTool:
		role = "user"
	case RoleAssistant:
		role = "model"
	default:
		return nil
	}

	content := &geminiContent{
		Role:  role,
		Parts: make([]geminiPart, 0),
	}

	// 处理工具调用结果
	if msg.Role == RoleTool && msg.ToolCallID != "" {
		// 解析工具响应
		var responseData map[string]any
		if err := json.Unmarshal([]byte(msg.Content), &responseData); err != nil {
			responseData = map[string]any{"result": msg.Content}
		}
		content.Parts = append(content.Parts, geminiPart{
			FunctionResponse: &geminiFunctionResponse{
				Name:     msg.Name, // 使用 Name 存储函数名
				Response: responseData,
			},
		})
		return content
	}

	// 处理助手消息中的工具调用
	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{}
			}
			content.Parts = append(content.Parts, geminiPart{
				FunctionCall: &geminiFunctionCall{
					Name: tc.Function.Name,
					Args: args,
				},
			})
		}
		// 如果还有文本内容，也添加进去
		if msg.Content != "" {
			content.Parts = append(content.Parts, geminiPart{Text: msg.Content})
		}
		return content
	}

	// 普通文本消息
	if msg.Content != "" {
		content.Parts = append(content.Parts, geminiPart{Text: msg.Content})
	}

	return content
}

// convertToolsToGeminiFormat 将工具定义转换为 Gemini 格式
func (m *GeminiModel) convertToolsToGeminiFormat(tools []ToolDefinition) []geminiTool {
	declarations := make([]geminiFunctionDeclaration, 0, len(tools))

	for _, tool := range tools {
		params := make(map[string]any)
		if tool.Function.Parameters != nil {
			// 尝试转换为 map
			if p, ok := tool.Function.Parameters.(map[string]any); ok {
				params = p
			} else {
				// 尝试 JSON 序列化再反序列化
				data, err := json.Marshal(tool.Function.Parameters)
				if err == nil {
					json.Unmarshal(data, &params)
				}
			}
		}

		declarations = append(declarations, geminiFunctionDeclaration{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  params,
		})
	}

	if len(declarations) == 0 {
		return nil
	}

	return []geminiTool{{FunctionDeclarations: declarations}}
}

// convertToResponse 将 Gemini 响应转换为标准 Response 格式
func (m *GeminiModel) convertToResponse(geminiResp *geminiResponse) *Response {
	resp := &Response{
		Model: m.config.Model,
	}

	// 处理使用统计
	if geminiResp.UsageMetadata != nil {
		resp.Usage = Usage{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		}
	}

	// 检查候选响应
	if len(geminiResp.Candidates) == 0 {
		resp.FinishReason = "error"
		return resp
	}

	candidate := geminiResp.Candidates[0]
	resp.FinishReason = strings.ToLower(candidate.FinishReason)

	// 处理内容部分
	var textContent string
	var toolCalls []ToolCall

	for _, part := range candidate.Content.Parts {
		// 提取文本
		if part.Text != "" {
			textContent += part.Text
		}

		// 提取函数调用
		if part.FunctionCall != nil {
			argsJSON, _ := json.Marshal(part.FunctionCall.Args)
			toolCalls = append(toolCalls, ToolCall{
				ID:   fmt.Sprintf("call_%d", time.Now().UnixNano()),
				Type: "function",
				Function: FunctionCall{
					Name:      part.FunctionCall.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}

	resp.Content = textContent
	resp.ToolCalls = toolCalls

	// 如果有工具调用，更新 finish reason
	if len(toolCalls) > 0 {
		resp.FinishReason = "tool_calls"
	}

	return resp
}
