// Package llm 提供与各种 LLM 提供商交互的统一接口。
// openai.go 实现 OpenAI 兼容客户端。
package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAIModel 实现 OpenAI 和 OpenAI 兼容 API 的 Model 接口。
// 支持流式响应、函数调用，以及 DeepSeek、GLM、Qwen、MiniMax、Kimi 等提供商的自定义基础 URL。
type OpenAIModel struct {
	client        *openai.Client
	config        ModelConfig
	supportsTools bool
}

// NewOpenAIModel 创建新的 OpenAI 兼容模型实例。
// 客户端根据提供的 ModelConfig 配置，支持：
// - OpenAI 兼容提供商的自定义 API 基础 URL
// - 代理设置
// - 请求超时
func NewOpenAIModel(cfg ModelConfig) (*OpenAIModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	config := openai.DefaultConfig(cfg.APIKey)

	// 为 OpenAI 兼容提供商设置自定义基础 URL
	if cfg.APIBase != "" {
		config.BaseURL = cfg.APIBase
	}

	// 如果指定了代理则配置代理
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

	// 如果指定了请求超时则设置
	if cfg.RequestTimeout > 0 {
		if httpClient, ok := config.HTTPClient.(*http.Client); ok {
			httpClient.Timeout = time.Duration(cfg.RequestTimeout) * time.Second
		}
	}

	// 根据提供商/模型确定是否支持工具
	supportsTools := true // 大多数现代模型支持工具
	if cfg.Provider == "openai" || cfg.Provider == "" {
		// OpenAI 模型支持工具
		supportsTools = true
	}

	return &OpenAIModel{
		client:        openai.NewClientWithConfig(config),
		config:        cfg,
		supportsTools: supportsTools,
	}, nil
}

// Name 返回模型标识符。
func (m *OpenAIModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持函数/工具调用。
func (m *OpenAIModel) SupportsTools() bool {
	return m.supportsTools
}

// Call 执行同步聊天补全调用。
func (m *OpenAIModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
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
		return nil, fmt.Errorf("chat completion error: %w", err)
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

	// 如果存在工具调用则解析
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

// CallStream 执行流式聊天补全调用。
func (m *OpenAIModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	req := m.buildRequest(messages, callOpts, true)

	stream, err := m.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("stream creation error: %w", err)
	}

	ch := make(chan StreamChunk, 100)

	go m.processStream(stream, ch)

	return ch, nil
}

func (m *OpenAIModel) processStream(stream *openai.ChatCompletionStream, ch chan<- StreamChunk) {
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

func (m *OpenAIModel) handleStreamChoice(choice openai.ChatCompletionStreamChoice, ch chan<- StreamChunk, toolCallsIndex map[int]int) {
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

func (m *OpenAIModel) handleStreamToolCall(tc openai.ToolCall, ch chan<- StreamChunk, toolCallsIndex map[int]int) {
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

// buildRequest 从消息和选项构建 OpenAI ChatCompletionRequest。
func (m *OpenAIModel) buildRequest(messages []Message, opts CallOptions, stream bool) openai.ChatCompletionRequest {
	req := openai.ChatCompletionRequest{
		Model:    m.config.Model,
		Messages: convertMessages(messages),
		Stream:   stream,
	}

	m.applyBasicOptions(&req, opts)
	m.applyToolsToRequest(&req, opts)
	m.applyResponseFormat(&req, opts)
	m.applySystemPrompt(&req, opts)

	return req
}

func (m *OpenAIModel) applyBasicOptions(req *openai.ChatCompletionRequest, opts CallOptions) {
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

func (m *OpenAIModel) applyToolsToRequest(req *openai.ChatCompletionRequest, opts CallOptions) {
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

	if opts.ToolChoice == nil {
		return
	}

	switch v := opts.ToolChoice.(type) {
	case string:
		req.ToolChoice = v
	case map[string]any:
		req.ToolChoice = v
	}
}

func (m *OpenAIModel) applyResponseFormat(req *openai.ChatCompletionRequest, opts CallOptions) {
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

func (m *OpenAIModel) applySystemPrompt(req *openai.ChatCompletionRequest, opts CallOptions) {
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

// convertMessages 将 llm.Message 切片转换为 openai.ChatCompletionMessage 切片。
func convertMessages(messages []Message) []openai.ChatCompletionMessage {
	result := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		result[i] = openai.ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
			Name:    msg.Name,
		}

		// 转换工具调用
		if len(msg.ToolCalls) > 0 {
			result[i].ToolCalls = make([]openai.ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				result[i].ToolCalls[j] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolType(tc.Type),
					Function: openai.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}

		// 为工具响应消息设置工具调用 ID
		if msg.ToolCallID != "" {
			result[i].ToolCallID = msg.ToolCallID
		}
	}
	return result
}

// convertUsage 将 openai.Usage 转换为 llm.Usage。
func convertUsage(usage openai.Usage) Usage {
	return Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}
