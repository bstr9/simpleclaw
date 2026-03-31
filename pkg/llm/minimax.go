// Package llm 提供与各种 LLM 提供商交互的统一接口。
// minimax.go 实现 MiniMax API 模型客户端。
// MiniMax API 兼容 OpenAI 格式，支持 reasoning_split 特性用于分离思考过程。
package llm

import (
	"bytes"
	"github.com/bstr9/simpleclaw/pkg/common"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// MiniMax 默认配置常量
const (
	// MiniMaxDefaultBaseURL MiniMax API 默认基础地址
	MiniMaxDefaultBaseURL = "https://api.minimax.chat/v1"
	// MiniMaxDefaultModel MiniMax 默认模型
	MiniMaxDefaultModel = "MiniMax-M2.1"
)

// MiniMaxReasoningDetail 表示 MiniMax 返回的思考过程详情
type MiniMaxReasoningDetail struct {
	ID    string `json:"id,omitempty"`    // 思考块 ID
	Index int    `json:"index,omitempty"` // 思考块索引
	Text  string `json:"text"`            // 思考内容
}

// MiniMaxMessage 扩展标准消息，支持 MiniMax 特有字段
type MiniMaxMessage struct {
	// 标准消息字段
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
	Name    string `json:"name,omitempty"`
	// ToolCalls 工具调用
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID 工具调用 ID
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ReasoningDetails 思考过程详情 (MiniMax 特有)
	ReasoningDetails []MiniMaxReasoningDetail `json:"reasoning_details,omitempty"`
}

// MiniMaxResponse 表示 MiniMax API 的完整响应
type MiniMaxResponse struct {
	ID      string          `json:"id,omitempty"`
	Object  string          `json:"object,omitempty"`
	Created int64           `json:"created,omitempty"`
	Model   string          `json:"model,omitempty"`
	Choices []MiniMaxChoice `json:"choices"`
	Usage   MiniMaxUsage    `json:"usage,omitempty"`
}

// MiniMaxChoice 表示 MiniMax 响应中的选择项
type MiniMaxChoice struct {
	Index        int            `json:"index"`
	Message      MiniMaxMessage `json:"message,omitempty"`
	Delta        MiniMaxDelta   `json:"delta,omitempty"`
	FinishReason string         `json:"finish_reason,omitempty"`
}

// MiniMaxDelta 表示流式响应中的增量内容
type MiniMaxDelta struct {
	Role             string                   `json:"role,omitempty"`
	Content          string                   `json:"content,omitempty"`
	ToolCalls        []MiniMaxToolCallDelta   `json:"tool_calls,omitempty"`
	ReasoningDetails []MiniMaxReasoningDetail `json:"reasoning_details,omitempty"`
}

// MiniMaxToolCallDelta 表示流式响应中的工具调用增量
type MiniMaxToolCallDelta struct {
	Index    int                       `json:"index,omitempty"`
	ID       string                    `json:"id,omitempty"`
	Type     string                    `json:"type,omitempty"`
	Function *MiniMaxFunctionCallDelta `json:"function,omitempty"`
}

// MiniMaxFunctionCallDelta 表示流式响应中的函数调用增量
type MiniMaxFunctionCallDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// MiniMaxUsage 表示 MiniMax API 的 token 使用情况
type MiniMaxUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// MiniMaxExtraConfig 包含 MiniMax 特有的配置选项
type MiniMaxExtraConfig struct {
	// ReasoningSplit 是否启用思考过程分离
	// 设置为 true 时，思考内容会单独返回在 reasoning_details 字段中
	ReasoningSplit bool `json:"reasoning_split,omitempty"`
	// ShowThinking 是否在流式输出中显示思考过程
	ShowThinking bool `json:"show_thinking,omitempty"`
}

// MiniMaxModel 实现 MiniMax API 模型
// 继承 OpenAI 兼容客户端，添加 MiniMax 特有功能
type MiniMaxModel struct {
	client        *openai.Client
	config        ModelConfig
	extraConfig   MiniMaxExtraConfig
	supportsTools bool
}

// NewMiniMaxModel 创建 MiniMax 模型实例
func NewMiniMaxModel(cfg ModelConfig) (*MiniMaxModel, error) {
	// 验证必要参数
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	if cfg.Model == "" {
		cfg.Model = MiniMaxDefaultModel
	}

	// 设置默认 API Base URL
	if cfg.APIBase == "" {
		cfg.APIBase = MiniMaxDefaultBaseURL
	}

	// 创建 OpenAI 兼容配置
	config := openai.DefaultConfig(cfg.APIKey)
	config.BaseURL = cfg.APIBase

	// 配置代理
	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		config.HTTPClient = &http.Client{
			Transport: transport,
		}
	}

	// 设置请求超时
	if cfg.RequestTimeout > 0 {
		if httpClient, ok := config.HTTPClient.(*http.Client); ok {
			httpClient.Timeout = time.Duration(cfg.RequestTimeout) * time.Second
		}
	}

	// 解析额外配置
	extraConfig := parseMiniMaxExtraConfig(cfg.Extra)

	return &MiniMaxModel{
		client:        openai.NewClientWithConfig(config),
		config:        cfg,
		extraConfig:   extraConfig,
		supportsTools: true, // MiniMax 支持工具调用
	}, nil
}

// parseMiniMaxExtraConfig 从配置中解析 MiniMax 特有选项
func parseMiniMaxExtraConfig(extra map[string]any) MiniMaxExtraConfig {
	cfg := MiniMaxExtraConfig{
		ReasoningSplit: true, // 默认启用思考分离
		ShowThinking:   false,
	}

	if extra == nil {
		return cfg
	}

	if v, ok := extra["reasoning_split"].(bool); ok {
		cfg.ReasoningSplit = v
	}
	if v, ok := extra["show_thinking"].(bool); ok {
		cfg.ShowThinking = v
	}

	return cfg
}

// Name 返回模型名称
func (m *MiniMaxModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持工具调用
func (m *MiniMaxModel) SupportsTools() bool {
	return m.supportsTools
}

// Call 执行同步对话补全调用
func (m *MiniMaxModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	// 合并选项与默认值
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	// 构建请求
	req := m.buildRequest(messages, callOpts, false)

	// 发起 API 调用
	resp, err := m.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("minimax chat completion error: %w", err)
	}

	// 解析响应
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices returned")
	}

	choice := resp.Choices[0]
	result := &Response{
		Content:      choice.Message.Content,
		Usage:        convertUsage(resp.Usage),
		FinishReason: string(choice.FinishReason),
		Model:        resp.Model,
	}

	// 解析工具调用
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
				Function: FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return result, nil
}

// CallStream 执行流式对话补全调用
func (m *MiniMaxModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	req := m.buildRequest(messages, callOpts, true)

	stream, err := m.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("minimax stream creation error: %w", err)
	}

	ch := make(chan StreamChunk, 100)

	go m.processStream(stream, ch)

	return ch, nil
}

// processStream 处理 MiniMax 流式响应
func (m *MiniMaxModel) processStream(stream *openai.ChatCompletionStream, ch chan<- StreamChunk) {
	defer close(ch)
	defer stream.Close()

	toolCallsIndex := make(map[int]int)

	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			ch <- StreamChunk{Done: true}
			return
		}
		if err != nil {
			ch <- StreamChunk{Error: err}
			return
		}

		if len(response.Choices) == 0 {
			continue
		}

		choice := response.Choices[0]
		m.handleStreamChoice(choice, ch, toolCallsIndex)
	}
}

func (m *MiniMaxModel) handleStreamChoice(choice openai.ChatCompletionStreamChoice, ch chan<- StreamChunk, toolCallsIndex map[int]int) {
	delta := choice.Delta

	if delta.Content != "" {
		ch <- StreamChunk{Delta: delta.Content}
	}

	for _, tc := range delta.ToolCalls {
		m.handleStreamToolCall(tc, ch, toolCallsIndex)
	}

	if choice.FinishReason != "" {
		ch <- StreamChunk{
			FinishReason: string(choice.FinishReason),
			Done:         true,
		}
	}
}

func (m *MiniMaxModel) handleStreamToolCall(tc openai.ToolCall, ch chan<- StreamChunk, toolCallsIndex map[int]int) {
	idx := 0
	if tc.Index != nil {
		idx = *tc.Index
	}
	if _, exists := toolCallsIndex[idx]; !exists {
		toolCallsIndex[idx] = len(toolCallsIndex)
	}

	chunk := StreamChunk{
		ToolCallDelta: &ToolCallDelta{
			Index: toolCallsIndex[idx],
			ID:    tc.ID,
			Type:  string(tc.Type),
		},
	}

	if tc.Function.Name != "" || tc.Function.Arguments != "" {
		chunk.ToolCallDelta.Function = &FunctionCallDelta{
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		}
	}

	ch <- chunk
}

// buildRequest 构建请求
// 添加 MiniMax 特有的 reasoning_split 参数
func (m *MiniMaxModel) buildRequest(messages []Message, opts CallOptions, stream bool) openai.ChatCompletionRequest {
	req := openai.ChatCompletionRequest{
		Model:    m.config.Model,
		Messages: convertMessages(messages),
		Stream:   stream,
	}

	m.applyRequestOptions(&req, opts)
	m.applyTools(&req, opts)
	m.applyResponseFormat(&req, opts)
	m.applySystemPrompt(&req, opts)

	return req
}

// applyRequestOptions 应用请求选项
func (m *MiniMaxModel) applyRequestOptions(req *openai.ChatCompletionRequest, opts CallOptions) {
	if opts.Temperature != nil {
		req.Temperature = float32(*opts.Temperature)
	}
	if opts.MaxTokens > 0 {
		req.MaxTokens = opts.MaxTokens
	}
	if opts.TopP != nil {
		req.TopP = float32(*opts.TopP)
	}
	if len(opts.Stop) > 0 {
		req.Stop = opts.Stop
	}
	if opts.FrequencyPenalty != nil {
		req.FrequencyPenalty = float32(*opts.FrequencyPenalty)
	}
	if opts.PresencePenalty != nil {
		req.PresencePenalty = float32(*opts.PresencePenalty)
	}
	if opts.User != "" {
		req.User = opts.User
	}
}

// applyTools 应用工具定义
func (m *MiniMaxModel) applyTools(req *openai.ChatCompletionRequest, opts CallOptions) {
	if len(opts.Tools) == 0 {
		return
	}

	req.Tools = make([]openai.Tool, len(opts.Tools))
	for i, tool := range opts.Tools {
		req.Tools[i] = openai.Tool{
			Type: openai.ToolType(tool.Type),
			Function: &openai.FunctionDefinition{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		}
	}

	if opts.ToolChoice != nil {
		switch v := opts.ToolChoice.(type) {
		case string:
			req.ToolChoice = v
		case map[string]any:
			req.ToolChoice = v
		}
	}
}

// applyResponseFormat 应用响应格式
func (m *MiniMaxModel) applyResponseFormat(req *openai.ChatCompletionRequest, opts CallOptions) {
	if opts.ResponseFormat == nil {
		return
	}

	switch v := opts.ResponseFormat.(type) {
	case string:
		req.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatType(v),
		}
	case map[string]any:
		if t, ok := v["type"].(string); ok {
			req.ResponseFormat = &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatType(t),
			}
		}
	}
}

// applySystemPrompt 应用系统提示词
func (m *MiniMaxModel) applySystemPrompt(req *openai.ChatCompletionRequest, opts CallOptions) {
	if opts.SystemPrompt == "" || len(req.Messages) == 0 {
		return
	}

	if req.Messages[0].Role != openai.ChatMessageRoleSystem {
		systemMsg := openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: opts.SystemPrompt,
		}
		req.Messages = append([]openai.ChatCompletionMessage{systemMsg}, req.Messages...)
	}
}

// CallWithReasoning 执行带思考过程分离的调用
// 这是 MiniMax 特有的功能，返回思考过程和回答内容
func (m *MiniMaxModel) CallWithReasoning(ctx context.Context, messages []Message, opts ...Option) (*MiniMaxReasoningResponse, error) {
	// 确保启用 reasoning_split
	extraConfig := m.extraConfig
	extraConfig.ReasoningSplit = true

	// 合并选项
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	// 构建原始 HTTP 请求以支持 MiniMax 特有参数
	reqBody := m.buildMiniMaxRequestBody(messages, callOpts, false, extraConfig)

	// 发送请求
	resp, err := m.sendMiniMaxRequest(ctx, reqBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 解析响应
	var result MiniMaxResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, errors.New("no choices returned")
	}

	choice := result.Choices[0]
	reasoningResp := &MiniMaxReasoningResponse{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Model:        result.Model,
		Usage: Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}

	// 提取思考过程
	if len(choice.Message.ReasoningDetails) > 0 {
		reasoningResp.Reasoning = make([]string, len(choice.Message.ReasoningDetails))
		for i, r := range choice.Message.ReasoningDetails {
			reasoningResp.Reasoning[i] = r.Text
		}
	}

	// 解析工具调用
	if len(choice.Message.ToolCalls) > 0 {
		reasoningResp.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			reasoningResp.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return reasoningResp, nil
}

// MiniMaxReasoningResponse 包含思考过程的响应
type MiniMaxReasoningResponse struct {
	Content      string     `json:"content"`
	Reasoning    []string   `json:"reasoning,omitempty"` // 思考过程内容
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	Usage        Usage      `json:"usage"`
	FinishReason string     `json:"finish_reason"`
	Model        string     `json:"model"`
}

// miniMaxRequestBody 表示 MiniMax API 请求体
type miniMaxRequestBody struct {
	Model          string                         `json:"model"`
	Messages       []openai.ChatCompletionMessage `json:"messages"`
	Temperature    *float64                       `json:"temperature,omitempty"`
	MaxTokens      int                            `json:"max_tokens,omitempty"`
	TopP           *float64                       `json:"top_p,omitempty"`
	Stream         bool                           `json:"stream"`
	Tools          []openai.Tool                  `json:"tools,omitempty"`
	ToolChoice     any                            `json:"tool_choice,omitempty"`
	ReasoningSplit bool                           `json:"reasoning_split,omitempty"` // MiniMax 特有
}

// buildMiniMaxRequestBody 构建 MiniMax 特有的请求体
func (m *MiniMaxModel) buildMiniMaxRequestBody(messages []Message, opts CallOptions, stream bool, extraConfig MiniMaxExtraConfig) miniMaxRequestBody {
	req := miniMaxRequestBody{
		Model:          m.config.Model,
		Messages:       convertMessages(messages),
		Stream:         stream,
		ReasoningSplit: extraConfig.ReasoningSplit,
	}

	if opts.Temperature != nil {
		temp := float64(*opts.Temperature)
		req.Temperature = &temp
	}
	if opts.MaxTokens > 0 {
		req.MaxTokens = opts.MaxTokens
	}
	if opts.TopP != nil {
		topP := float64(*opts.TopP)
		req.TopP = &topP
	}

	if len(opts.Tools) > 0 {
		req.Tools = make([]openai.Tool, len(opts.Tools))
		for i, tool := range opts.Tools {
			req.Tools[i] = openai.Tool{
				Type: openai.ToolType(tool.Type),
				Function: &openai.FunctionDefinition{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			}
		}
	}

	if opts.ToolChoice != nil {
		req.ToolChoice = opts.ToolChoice
	}

	return req
}

// sendMiniMaxRequest 发送 MiniMax API 请求
func (m *MiniMaxModel) sendMiniMaxRequest(ctx context.Context, reqBody miniMaxRequestBody) (*http.Response, error) {
	// 序列化请求体
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "POST", m.config.APIBase+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Authorization", common.AuthPrefixBearer+m.config.APIKey)

	// 发送请求
	httpClient := &http.Client{}
	if m.config.RequestTimeout > 0 {
		httpClient.Timeout = time.Duration(m.config.RequestTimeout) * time.Second
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return resp, nil
}
