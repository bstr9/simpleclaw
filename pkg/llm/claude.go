// Package llm 提供与各种 LLM 提供商交互的统一接口。
// claude.go 实现 Anthropic Claude API 模型客户端。
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

// Claude API 常量
const (
	// ClaudeDefaultBaseURL 是 Anthropic Claude API 的默认基础 URL
	ClaudeDefaultBaseURL = "https://api.anthropic.com"

	// ClaudeAPIVersion 是 Claude API 版本
	ClaudeAPIVersion = "2023-06-01"

	// Claude 模型常量
	Claude3Opus   = "claude-3-opus-20240229"
	Claude3Sonnet = "claude-3-sonnet-20240229"
	Claude3Haiku  = "claude-3-haiku-20240307"

	// Claude 3.5 模型
	Claude35Sonnet = "claude-3-5-sonnet-20241022"
	Claude35Haiku  = "claude-3-5-haiku-20241022"

	// Claude 4 模型
	Claude4Sonnet = "claude-sonnet-4-20250514"
	Claude4Opus   = "claude-opus-4-20250514"
)

// ClaudeModel 实现 Anthropic Claude API 的 Model 接口
// 支持 streaming 和非 streaming 模式，支持 tool calling
type ClaudeModel struct {
	client        *http.Client
	config        ModelConfig
	apiKey        string
	supportsTools bool
}

// Claude API 请求结构体

// claudeRequest 表示 Claude API 请求
type claudeRequest struct {
	Model         string          `json:"model"`
	MaxTokens     int             `json:"max_tokens"`
	Messages      []claudeMessage `json:"messages"`
	System        string          `json:"system,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	TopK          *int            `json:"top_k,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Tools         []claudeTool    `json:"tools,omitempty"`
	ToolChoice    any             `json:"tool_choice,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
}

// claudeMessage 表示 Claude 消息格式
type claudeMessage struct {
	Role    string        `json:"role"`
	Content claudeContent `json:"content"`
}

// claudeContent 表示 Claude 内容，可以是字符串或内容块数组
type claudeContent []claudeContentBlock

// claudeContentBlock 表示 Claude 内容块
type claudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	// Tool use 字段
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`

	// Tool result 字段
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// claudeTool 表示 Claude 工具定义
type claudeTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// Claude API 响应结构体

// claudeResponse 表示 Claude API 响应
type claudeResponse struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Role         string               `json:"role"`
	Content      []claudeContentBlock `json:"content"`
	Model        string               `json:"model"`
	StopReason   string               `json:"stop_reason"`
	StopSequence string               `json:"stop_sequence,omitempty"`
	Usage        claudeUsage          `json:"usage"`
}

// claudeUsage 表示 Claude 使用统计
type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// claudeError 表示 Claude API 错误
type claudeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// claudeErrorResponse 表示 Claude API 错误响应
type claudeErrorResponse struct {
	Error claudeError `json:"error"`
}

// Claude 流式事件类型
const (
	ClaudeEventMessageStart      = "message_start"
	ClaudeEventContentBlockStart = "content_block_start"
	ClaudeEventContentBlockDelta = "content_block_delta"
	ClaudeEventContentBlockStop  = "content_block_stop"
	ClaudeEventMessageDelta      = "message_delta"
	ClaudeEventMessageStop       = "message_stop"
	ClaudeEventError             = "error"
)

// claudeStreamEvent 表示 Claude 流式事件
type claudeStreamEvent struct {
	Type         string              `json:"type"`
	Index        int                 `json:"index,omitempty"`
	Message      *claudeResponse     `json:"message,omitempty"`
	ContentBlock *claudeContentBlock `json:"content_block,omitempty"`
	Delta        *claudeStreamDelta  `json:"delta,omitempty"`
	Error        *claudeError        `json:"error,omitempty"`
}

// claudeStreamDelta 表示 Claude 流式增量
type claudeStreamDelta struct {
	Type         string `json:"type,omitempty"`
	Text         string `json:"text,omitempty"`
	PartialJSON  string `json:"partial_json,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

// NewClaudeModel 创建新的 Claude 模型实例
func NewClaudeModel(cfg ModelConfig) (*ClaudeModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("claude api_key 是必需的")
	}
	if cfg.Model == "" {
		// 设置默认模型
		cfg.Model = Claude35Sonnet
	}

	// 设置默认 API Base URL
	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = ClaudeDefaultBaseURL
	}
	cfg.APIBase = apiBase

	// 创建 HTTP 客户端
	httpClient := &http.Client{}
	if cfg.RequestTimeout > 0 {
		httpClient.Timeout = time.Duration(cfg.RequestTimeout) * time.Second
	} else {
		httpClient.Timeout = 120 * time.Second
	}

	// 配置代理
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

	// 设置默认模型名称
	if cfg.ModelName == "" {
		cfg.ModelName = cfg.Model
	}

	return &ClaudeModel{
		client:        httpClient,
		config:        cfg,
		apiKey:        cfg.APIKey,
		supportsTools: true, // Claude 支持工具调用
	}, nil
}

// Name 返回模型标识符
func (m *ClaudeModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持函数/工具调用
func (m *ClaudeModel) SupportsTools() bool {
	return m.supportsTools
}

// Call 执行同步聊天完成调用
func (m *ClaudeModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	// 合并选项与默认值
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	// 构建 Claude 请求
	req := m.buildClaudeRequest(messages, callOpts, false)

	// 序列化请求体
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 构建 API URL
	apiURL := fmt.Sprintf("%s/v1/messages", strings.TrimSuffix(m.config.APIBase, "/"))

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	m.setHeaders(httpReq)

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

	// 检查 HTTP 状态码
	if httpResp.StatusCode != http.StatusOK {
		var errResp claudeErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("claude api 错误 [%d]: %s", httpResp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("claude api 错误 [状态码 %d]: %s", httpResp.StatusCode, string(respBody))
	}

	// 解析响应
	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 转换为标准响应格式
	return m.convertToResponse(&claudeResp), nil
}

// CallStream 执行流式聊天完成调用
func (m *ClaudeModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	// 合并选项与默认值
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	// 构建 Claude 请求
	req := m.buildClaudeRequest(messages, callOpts, true)

	// 序列化请求体
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 构建 API URL
	apiURL := fmt.Sprintf("%s/v1/messages", strings.TrimSuffix(m.config.APIBase, "/"))

	// 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	m.setHeaders(httpReq)

	// 执行请求
	httpResp, err := m.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	// 检查 HTTP 状态码
	if httpResp.StatusCode != http.StatusOK {
		defer httpResp.Body.Close()
		respBody, _ := io.ReadAll(httpResp.Body)
		var errResp claudeErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("claude api 错误 [%d]: %s", httpResp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("流式请求失败，状态码 %d: %s", httpResp.StatusCode, string(respBody))
	}

	// 创建输出通道
	ch := make(chan StreamChunk, 100)

	// 启动 goroutine 读取流
	go m.readStream(httpResp, ch)

	return ch, nil
}

// setHeaders 设置 Claude API 请求头
func (m *ClaudeModel) setHeaders(req *http.Request) {
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("x-api-key", m.apiKey)
	req.Header.Set("anthropic-version", ClaudeAPIVersion)
}

// buildClaudeRequest 构建 Claude API 请求
func (m *ClaudeModel) buildClaudeRequest(messages []Message, opts CallOptions, stream bool) claudeRequest {
	maxTokens := m.resolveMaxTokens(opts.MaxTokens)

	req := claudeRequest{
		Model:     m.config.Model,
		MaxTokens: maxTokens,
		Messages:  make([]claudeMessage, 0),
		Stream:    stream,
	}

	systemPrompt, filteredMessages := m.extractSystemPrompt(messages, opts.SystemPrompt)
	if systemPrompt != "" {
		req.System = systemPrompt
	}

	req.Messages = m.convertMessagesToClaude(filteredMessages)
	m.applyCallOptions(&req, opts)

	return req
}

// resolveMaxTokens 解析最大 token 数
func (m *ClaudeModel) resolveMaxTokens(maxTokens int) int {
	if maxTokens == 0 {
		return m.getMaxTokens(m.config.Model)
	}
	return maxTokens
}

// extractSystemPrompt 提取系统提示词
func (m *ClaudeModel) extractSystemPrompt(messages []Message, defaultPrompt string) (string, []Message) {
	systemPrompt := defaultPrompt
	var filteredMessages []Message
	for _, msg := range messages {
		if msg.Role == RoleSystem {
			if systemPrompt == "" {
				systemPrompt = msg.Content
			}
			continue
		}
		filteredMessages = append(filteredMessages, msg)
	}
	return systemPrompt, filteredMessages
}

// applyCallOptions 应用调用选项
func (m *ClaudeModel) applyCallOptions(req *claudeRequest, opts CallOptions) {
	if opts.Temperature != nil {
		req.Temperature = opts.Temperature
	}
	if opts.TopP != nil {
		req.TopP = opts.TopP
	}
	if len(opts.Stop) > 0 {
		req.StopSequences = opts.Stop
	}
	if len(opts.Tools) > 0 {
		req.Tools = m.convertToolsToClaudeFormat(opts.Tools)
	}
	if opts.ToolChoice != nil {
		req.ToolChoice = opts.ToolChoice
	}
}

// getMaxTokens 获取模型的最大输出 token 数
func (m *ClaudeModel) getMaxTokens(model string) int {
	// Claude 3.5 和 3.7 模型支持更高的 max_tokens
	if strings.HasPrefix(model, "claude-3-5") || strings.HasPrefix(model, "claude-3.5") ||
		strings.HasPrefix(model, "claude-3-7") || strings.HasPrefix(model, "claude-3.7") {
		return 8192
	}
	// Claude 3 Opus 默认 4096
	if strings.Contains(strings.ToLower(model), "opus") &&
		(strings.HasPrefix(model, "claude-3-") || strings.HasPrefix(model, "claude-3.")) {
		return 4096
	}
	// Claude 4 模型支持更高的 max_tokens
	if strings.HasPrefix(model, "claude-sonnet-4") || strings.HasPrefix(model, "claude-opus-4") {
		return 64000
	}
	// 默认值
	return 8192
}

// convertMessagesToClaude 将消息转换为 Claude 格式
func (m *ClaudeModel) convertMessagesToClaude(messages []Message) []claudeMessage {
	result := make([]claudeMessage, 0, len(messages))

	for _, msg := range messages {
		claudeMsg := m.convertSingleMessage(msg)
		if claudeMsg != nil {
			result = append(result, *claudeMsg)
		}
	}

	return result
}

// convertSingleMessage 转换单条消息
func (m *ClaudeModel) convertSingleMessage(msg Message) *claudeMessage {
	if msg.Role == RoleTool && msg.ToolCallID != "" {
		return m.convertToolResultMessage(msg)
	}

	if msg.Role == RoleAssistant && len(msg.ToolCalls) > 0 {
		return m.convertAssistantToolCallMessage(msg)
	}

	return m.convertTextMessage(msg)
}

// convertToolResultMessage 转换工具调用结果消息
func (m *ClaudeModel) convertToolResultMessage(msg Message) *claudeMessage {
	content := json.RawMessage(fmt.Sprintf(`"%s"`, msg.Content))
	return &claudeMessage{
		Role: "user",
		Content: claudeContent{{
			Type:      "tool_result",
			ToolUseID: msg.ToolCallID,
			Content:   content,
			IsError:   false,
		}},
	}
}

// convertAssistantToolCallMessage 转换助手消息中的工具调用
func (m *ClaudeModel) convertAssistantToolCallMessage(msg Message) *claudeMessage {
	content := make(claudeContent, 0)

	if msg.Content != "" {
		content = append(content, claudeContentBlock{
			Type: "text",
			Text: msg.Content,
		})
	}

	for _, tc := range msg.ToolCalls {
		args := m.parseToolCallArgs(tc.Function.Arguments)
		content = append(content, claudeContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: args,
		})
	}

	return &claudeMessage{
		Role:    string(RoleAssistant),
		Content: content,
	}
}

// parseToolCallArgs 解析工具调用参数
func (m *ClaudeModel) parseToolCallArgs(argsJSON string) any {
	var args any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return argsJSON
	}
	return args
}

// convertTextMessage 转换普通文本消息
func (m *ClaudeModel) convertTextMessage(msg Message) *claudeMessage {
	if msg.Content == "" {
		return nil
	}
	return &claudeMessage{
		Role: string(msg.Role),
		Content: claudeContent{{
			Type: "text",
			Text: msg.Content,
		}},
	}
}

// convertToolsToClaudeFormat 将工具定义转换为 Claude 格式
func (m *ClaudeModel) convertToolsToClaudeFormat(tools []ToolDefinition) []claudeTool {
	result := make([]claudeTool, 0, len(tools))

	for _, tool := range tools {
		// 只处理 function 类型的工具
		if tool.Type != "function" {
			continue
		}

		params := make(map[string]interface{})
		if tool.Function.Parameters != nil {
			// 尝试转换为 map
			if p, ok := tool.Function.Parameters.(map[string]interface{}); ok {
				params = p
			} else {
				// 尝试 JSON 序列化再反序列化
				data, err := json.Marshal(tool.Function.Parameters)
				if err == nil {
					json.Unmarshal(data, &params)
				}
			}
		}

		result = append(result, claudeTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: params,
		})
	}

	return result
}

// convertToResponse 将 Claude 响应转换为标准 Response 格式
func (m *ClaudeModel) convertToResponse(claudeResp *claudeResponse) *Response {
	resp := &Response{
		Model:        claudeResp.Model,
		FinishReason: claudeResp.StopReason,
		Usage: Usage{
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
	}

	// 处理内容块
	var textContent string
	var toolCalls []ToolCall

	for _, block := range claudeResp.Content {
		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			// 将 input 转换为 JSON 字符串
			argsJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: FunctionCall{
					Name:      block.Name,
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

// readStream 读取 SSE 流并转换为 StreamChunk
func (m *ClaudeModel) readStream(httpResp *http.Response, ch chan<- StreamChunk) {
	defer close(ch)
	defer httpResp.Body.Close()

	reader := bufio.NewReader(httpResp.Body)

	// 用于累积工具调用数据
	toolUsesMap := make(map[int]*ToolCall)
	var stopReason string
	var usage *Usage

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				m.sendFinalStreamChunk(ch, toolUsesMap, stopReason, usage)
				return
			}
			ch <- StreamChunk{Error: fmt.Errorf("流读取错误: %w", err)}
			return
		}

		// 跳过空行和事件类型行
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "event: ") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		// 解析并处理事件
		m.processStreamEvent(data, ch, toolUsesMap, &stopReason, &usage)
	}
}

// sendFinalStreamChunk 发送最终流块
func (m *ClaudeModel) sendFinalStreamChunk(ch chan<- StreamChunk, toolUsesMap map[int]*ToolCall, stopReason string, usage *Usage) {
	finishReason := m.determineFinishReason(toolUsesMap, stopReason)
	ch <- StreamChunk{
		Done:         true,
		FinishReason: finishReason,
		Usage:        usage,
	}
}

// determineFinishReason 确定结束原因
func (m *ClaudeModel) determineFinishReason(toolUsesMap map[int]*ToolCall, stopReason string) string {
	if len(toolUsesMap) > 0 {
		return "tool_calls"
	}
	if stopReason != "" {
		return stopReason
	}
	return "stop"
}

// processStreamEvent 处理单个流事件
func (m *ClaudeModel) processStreamEvent(data string, ch chan<- StreamChunk, toolUsesMap map[int]*ToolCall, stopReason *string, usage **Usage) {
	var event claudeStreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return // 跳过解析错误
	}

	switch event.Type {
	case ClaudeEventError:
		m.handleStreamError(event, ch)
	case ClaudeEventMessageStart:
		m.handleMessageStart(event, usage)
	case ClaudeEventContentBlockStart:
		m.handleContentBlockStart(event, toolUsesMap)
	case ClaudeEventContentBlockDelta:
		m.handleContentBlockDelta(event, ch, toolUsesMap)
	case ClaudeEventMessageDelta:
		m.handleMessageDelta(event, stopReason, usage)
	case ClaudeEventMessageStop:
		m.handleMessageStop(ch, toolUsesMap, stopReason, usage)
	}
}

// handleStreamError 处理流错误事件
func (m *ClaudeModel) handleStreamError(event claudeStreamEvent, ch chan<- StreamChunk) {
	if event.Error != nil {
		ch <- StreamChunk{
			Error: fmt.Errorf("claude api 错误: %s", event.Error.Message),
		}
	}
}

// handleMessageStart 处理消息开始事件
func (m *ClaudeModel) handleMessageStart(event claudeStreamEvent, usage **Usage) {
	if event.Message != nil {
		*usage = &Usage{
			PromptTokens: event.Message.Usage.InputTokens,
		}
	}
}

// handleContentBlockStart 处理内容块开始事件
func (m *ClaudeModel) handleContentBlockStart(event claudeStreamEvent, toolUsesMap map[int]*ToolCall) {
	if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
		toolUsesMap[event.Index] = &ToolCall{
			ID:   event.ContentBlock.ID,
			Type: "function",
			Function: FunctionCall{
				Name: event.ContentBlock.Name,
			},
		}
	}
}

// handleContentBlockDelta 处理内容块增量事件
func (m *ClaudeModel) handleContentBlockDelta(event claudeStreamEvent, ch chan<- StreamChunk, toolUsesMap map[int]*ToolCall) {
	if event.Delta == nil {
		return
	}

	switch event.Delta.Type {
	case "text_delta":
		if event.Delta.Text != "" {
			ch <- StreamChunk{Delta: event.Delta.Text}
		}
	case "input_json_delta":
		if toolUse, ok := toolUsesMap[event.Index]; ok {
			toolUse.Function.Arguments += event.Delta.PartialJSON
		}
	}
}

// handleMessageDelta 处理消息增量事件
func (m *ClaudeModel) handleMessageDelta(event claudeStreamEvent, stopReason *string, usage **Usage) {
	if event.Delta != nil {
		if event.Delta.StopReason != "" {
			*stopReason = event.Delta.StopReason
		}
		if *usage != nil {
			(*usage).CompletionTokens = event.Delta.OutputTokens
			(*usage).TotalTokens = (*usage).PromptTokens + (*usage).CompletionTokens
		}
	}
}

// handleMessageStop 处理消息停止事件
func (m *ClaudeModel) handleMessageStop(ch chan<- StreamChunk, toolUsesMap map[int]*ToolCall, stopReason *string, usage **Usage) {
	// 发送所有累积的工具调用
	if len(toolUsesMap) > 0 {
		m.sendToolCalls(ch, toolUsesMap)
	}

	// 发送完成块
	finishReason := m.determineFinishReason(toolUsesMap, *stopReason)
	ch <- StreamChunk{
		Done:         true,
		FinishReason: finishReason,
		Usage:        *usage,
	}
}

// sendToolCalls 发送工具调用
func (m *ClaudeModel) sendToolCalls(ch chan<- StreamChunk, toolUsesMap map[int]*ToolCall) {
	for idx := 0; idx < len(toolUsesMap); idx++ {
		if toolCall, ok := toolUsesMap[idx]; ok {
			ch <- StreamChunk{
				ToolCallDelta: &ToolCallDelta{
					Index: idx,
					ID:    toolCall.ID,
					Type:  toolCall.Type,
					Function: &FunctionCallDelta{
						Name:      toolCall.Function.Name,
						Arguments: toolCall.Function.Arguments,
					},
				},
			}
		}
	}
}

// 确保 ClaudeModel 实现 Model 接口
var _ Model = (*ClaudeModel)(nil)
