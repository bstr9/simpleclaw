// Package llm 提供与各种 LLM 提供商交互的统一接口。
// dashscope.go 实现阿里云灵积平台 API 客户端。
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

// DashScope API 常量
const (
	// DashScopeDefaultBaseURL 是阿里云灵积平台 API 的默认地址
	DashScopeDefaultBaseURL = "https://dashscope.aliyuncs.com/api/v1"

	// DashScopeGenerationEndpoint 是文本生成 API 端点
	DashScopeGenerationEndpoint = "/services/aigc/text-generation/generation"
	// DashScopeMultiModalEndpoint 是多模态对话 API 端点
	DashScopeMultiModalEndpoint = "/services/aigc/multimodal-generation/generation"
)

// DashScope 模型标识符常量
const (
	// 通义千问系列模型
	DashScopeQwenTurbo      = "qwen-turbo"
	DashScopeQwenPlus       = "qwen-plus"
	DashScopeQwenMax        = "qwen-max"
	DashScopeQwenMaxLongCtx = "qwen-max-longcontext"
	DashScopeQwenLong       = "qwen-long"
	DashScopeQwenVLPlus     = "qwen-vl-plus"
	DashScopeQwenVLMax      = "qwen-vl-max"
	DashScopeQwenMathPlus   = "qwen-math-plus"
	DashScopeQwenCoderPlus  = "qwen-coder-plus"
	DashScopeQwen2_5_72B    = "qwen2.5-72b-instruct"
	DashScopeQwen2_5_32B    = "qwen2.5-32b-instruct"
	DashScopeQwen2_5_14B    = "qwen2.5-14b-instruct"
	DashScopeQwen2_5_7B     = "qwen2.5-7b-instruct"
	DashScopeQwen2_5_3B     = "qwen2.5-3b-instruct"
	DashScopeQwen2_5_1_5B   = "qwen2.5-1.5b-instruct"
	DashScopeQwen2_5_0_5B   = "qwen2.5-0.5b-instruct"
	// Qwen3.5 系列（多模态模型）
	DashScopeQwen3_5_72B  = "qwen3.5-72b-instruct"
	DashScopeQwen3_5_32B  = "qwen3.5-32b-instruct"
	DashScopeQwen3_5_14B  = "qwen3.5-14b-instruct"
	DashScopeQwen3_5_7B   = "qwen3.5-7b-instruct"
	DashScopeQwen3_5_3B   = "qwen3.5-3b-instruct"
	DashScopeQwen3_5_1_5B = "qwen3.5-1.5b-instruct"
	DashScopeQwen3_5_0_5B = "qwen3.5-0.5b-instruct"
	// 推理模型
	DashScopeQwQPlus = "qwq-plus"
	DashScopeQwQ32B  = "qwq-32b"
)

// 多模态模型前缀（这些模型需要使用 MultiModalConversation API）
var dashScopeMultiModalPrefixes = []string{"qwen3.", "qwen3-", "qwq-"}

// dashScopeModelInfo 定义模型的能力信息
var dashScopeModelInfo = map[string]ModelInfo{
	DashScopeQwenTurbo: {
		ID:                DashScopeQwenTurbo,
		Name:              "通义千问 Turbo",
		Provider:          ProviderDashScope,
		ContextWindow:     8192,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	DashScopeQwenPlus: {
		ID:                DashScopeQwenPlus,
		Name:              "通义千问 Plus",
		Provider:          ProviderDashScope,
		ContextWindow:     32768,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	DashScopeQwenMax: {
		ID:                DashScopeQwenMax,
		Name:              "通义千问 Max",
		Provider:          ProviderDashScope,
		ContextWindow:     32768,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	DashScopeQwenMaxLongCtx: {
		ID:                DashScopeQwenMaxLongCtx,
		Name:              "通义千问 Max 长上下文",
		Provider:          ProviderDashScope,
		ContextWindow:     28000,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	DashScopeQwenLong: {
		ID:                DashScopeQwenLong,
		Name:              "通义千问 Long",
		Provider:          ProviderDashScope,
		ContextWindow:     1000000,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	DashScopeQwenVLPlus: {
		ID:                DashScopeQwenVLPlus,
		Name:              "通义千问 VL Plus",
		Provider:          ProviderDashScope,
		ContextWindow:     8192,
		SupportsVision:    true,
		SupportsTools:     false,
		SupportsStreaming: true,
	},
	DashScopeQwenVLMax: {
		ID:                DashScopeQwenVLMax,
		Name:              "通义千问 VL Max",
		Provider:          ProviderDashScope,
		ContextWindow:     32768,
		SupportsVision:    true,
		SupportsTools:     false,
		SupportsStreaming: true,
	},
	DashScopeQwenMathPlus: {
		ID:                DashScopeQwenMathPlus,
		Name:              "通义千问 Math Plus",
		Provider:          ProviderDashScope,
		ContextWindow:     4096,
		SupportsVision:    false,
		SupportsTools:     false,
		SupportsStreaming: true,
	},
	DashScopeQwenCoderPlus: {
		ID:                DashScopeQwenCoderPlus,
		Name:              "通义千问 Coder Plus",
		Provider:          ProviderDashScope,
		ContextWindow:     65536,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	DashScopeQwen2_5_72B: {
		ID:                DashScopeQwen2_5_72B,
		Name:              "Qwen2.5 72B Instruct",
		Provider:          ProviderDashScope,
		ContextWindow:     131072,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	DashScopeQwen2_5_32B: {
		ID:                DashScopeQwen2_5_32B,
		Name:              "Qwen2.5 32B Instruct",
		Provider:          ProviderDashScope,
		ContextWindow:     131072,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	DashScopeQwen3_5_72B: {
		ID:                DashScopeQwen3_5_72B,
		Name:              "Qwen3.5 72B Instruct",
		Provider:          ProviderDashScope,
		ContextWindow:     131072,
		SupportsVision:    true,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	DashScopeQwQPlus: {
		ID:                DashScopeQwQPlus,
		Name:              "QwQ Plus 推理模型",
		Provider:          ProviderDashScope,
		ContextWindow:     131072,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
	DashScopeQwQ32B: {
		ID:                DashScopeQwQ32B,
		Name:              "QwQ 32B 推理模型",
		Provider:          ProviderDashScope,
		ContextWindow:     131072,
		SupportsVision:    false,
		SupportsTools:     true,
		SupportsStreaming: true,
	},
}

// DashScope API 请求/响应数据结构

// dashScopeRequest 表示 DashScope API 请求
type dashScopeRequest struct {
	Model      string                 `json:"model"`
	Input      dashScopeInput         `json:"input"`
	Parameters dashScopeParameters    `json:"parameters,omitempty"`
	Debug      map[string]interface{} `json:"debug,omitempty"`
}

// dashScopeInput 表示输入内容
type dashScopeInput struct {
	Messages []dashScopeMessage `json:"messages"`
}

// dashScopeMessage 表示单条消息
type dashScopeMessage struct {
	Role       string              `json:"role"`
	Content    interface{}         `json:"content"` // 可以是字符串或数组（多模态）
	Name       string              `json:"name,omitempty"`
	ToolCalls  []dashScopeToolCall `json:"tool_calls,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
}

// dashScopeContentBlock 表示多模态内容块
type dashScopeContentBlock struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
	Type  string `json:"type,omitempty"`
}

// dashScopeToolCall 表示工具调用
type dashScopeToolCall struct {
	ID       string                `json:"id"`
	Type     string                `json:"type"`
	Function dashScopeFunctionCall `json:"function"`
}

// dashScopeFunctionCall 表示函数调用
type dashScopeFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// dashScopeParameters 表示请求参数
type dashScopeParameters struct {
	ResultFormat      string                 `json:"result_format,omitempty"`
	Temperature       float64                `json:"temperature,omitempty"`
	TopP              float64                `json:"top_p,omitempty"`
	MaxTokens         int                    `json:"max_tokens,omitempty"`
	Stop              []string               `json:"stop,omitempty"`
	Tools             []dashScopeTool        `json:"tools,omitempty"`
	ToolChoice        interface{}            `json:"tool_choice,omitempty"`
	EnableThinking    bool                   `json:"enable_thinking,omitempty"`
	ThinkingBudget    int                    `json:"thinking_budget,omitempty"`
	IncrementalOutput bool                   `json:"incremental_output,omitempty"`
	Extra             map[string]interface{} `json:",omitempty"` // 额外参数
}

// dashScopeTool 表示工具定义
type dashScopeTool struct {
	Type     string               `json:"type"`
	Function dashScopeFunctionDef `json:"function"`
}

// dashScopeFunctionDef 表示函数定义
type dashScopeFunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// dashScopeResponse 表示 DashScope API 响应
type dashScopeResponse struct {
	RequestID  string          `json:"request_id,omitempty"`
	Output     dashScopeOutput `json:"output"`
	Usage      dashScopeUsage  `json:"usage,omitempty"`
	Code       string          `json:"code,omitempty"`
	Message    string          `json:"message,omitempty"`
	StatusCode int             `json:"status_code,omitempty"`
}

// dashScopeOutput 表示输出内容
type dashScopeOutput struct {
	Choices []dashScopeChoice `json:"choices,omitempty"`
	Text    string            `json:"text,omitempty"` // 旧版 API 使用
}

// dashScopeChoice 表示选择项
type dashScopeChoice struct {
	FinishReason string           `json:"finish_reason,omitempty"`
	Message      dashScopeMessage `json:"message,omitempty"`
}

// dashScopeUsage 表示 token 使用统计
type dashScopeUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

// DashScopeModel 实现阿里云灵积平台 API 的 Model 接口
type DashScopeModel struct {
	client    *http.Client
	config    ModelConfig
	modelInfo *ModelInfo
	baseURL   string
}

// NewDashScopeModel 创建新的灵积平台模型实例
// 参数:
//   - cfg: 模型配置，包含 API Key、模型名称等
//
// 返回:
//   - *DashScopeModel: 灵积平台模型实例
//   - error: 错误信息
func NewDashScopeModel(cfg ModelConfig) (*DashScopeModel, error) {
	// 验证必填字段
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("dashscope api_key 是必需的")
	}
	if cfg.Model == "" {
		// 设置默认模型
		cfg.Model = DashScopeQwenPlus
	}

	// 设置默认 API Base URL
	baseURL := cfg.APIBase
	if baseURL == "" {
		baseURL = DashScopeDefaultBaseURL
	}

	// 设置 provider 标识
	cfg.Provider = ProviderDashScope

	// 设置默认模型名称（用户友好名称）
	if cfg.ModelName == "" {
		cfg.ModelName = cfg.Model
	}

	// 映射模型名称
	model := mapDashScopeModel(cfg.Model)
	cfg.Model = model

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	if cfg.RequestTimeout > 0 {
		client.Timeout = time.Duration(cfg.RequestTimeout) * time.Second
	}

	// 配置代理
	if cfg.Proxy != "" {
		proxyURL, err := parseProxyURL(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("无效的代理 URL: %w", err)
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		client.Transport = transport
	}

	// 获取模型信息
	var info *ModelInfo
	if modelInfo, ok := dashScopeModelInfo[model]; ok {
		info = &modelInfo
	} else {
		// 未知模型，使用默认信息
		info = &ModelInfo{
			ID:                model,
			Name:              model,
			Provider:          ProviderDashScope,
			SupportsTools:     true,
			SupportsStreaming: true,
		}
	}

	return &DashScopeModel{
		client:    client,
		config:    cfg,
		modelInfo: info,
		baseURL:   baseURL,
	}, nil
}

// Name 返回模型标识符
func (m *DashScopeModel) Name() string {
	return m.config.ModelName
}

// SupportsTools 返回模型是否支持工具调用
func (m *DashScopeModel) SupportsTools() bool {
	if m.modelInfo != nil {
		return m.modelInfo.SupportsTools
	}
	return true // 默认支持
}

// Call 执行同步对话完成请求
// 参数:
//   - ctx: 上下文，用于控制请求生命周期
//   - messages: 对话消息列表
//   - opts: 可选参数，如 temperature、max_tokens 等
//
// 返回:
//   - *Response: 模型响应
//   - error: 错误信息
func (m *DashScopeModel) Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error) {
	// 合并选项和默认值
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	// 构建请求
	req, err := m.buildRequest(messages, callOpts, false)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}

	// 发送请求
	resp, err := m.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("API 请求失败: %w", err)
	}

	// 解析响应
	return m.parseResponse(resp)
}

// CallStream 执行流式对话完成请求
// 参数:
//   - ctx: 上下文，用于控制请求生命周期
//   - messages: 对话消息列表
//   - opts: 可选参数
//
// 返回:
//   - <-chan StreamChunk: 流式响应通道
//   - error: 错误信息
func (m *DashScopeModel) CallStream(ctx context.Context, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	// 合并选项和默认值
	callOpts := m.config.DefaultOptions
	for _, opt := range opts {
		opt(&callOpts)
	}

	// 构建请求
	req, err := m.buildRequest(messages, callOpts, true)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}

	// 设置流式请求头
	req.Header.Set("X-DashScope-SSE", "enable")

	// 发送请求
	resp, err := m.doStreamRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("流式 API 请求失败: %w", err)
	}

	// 创建输出通道
	ch := make(chan StreamChunk, 100)

	// 启动 goroutine 处理流式响应
	go m.handleStreamResponse(resp, ch)

	return ch, nil
}

// buildRequest 构建 DashScope API 请求
func (m *DashScopeModel) buildRequest(messages []Message, opts CallOptions, stream bool) (*http.Request, error) {
	// 转换消息格式
	dsMessages := m.convertMessages(messages)

	// 构建请求参数
	params := dashScopeParameters{
		ResultFormat: "message",
	}

	// 应用可选参数
	if opts.Temperature != nil {
		params.Temperature = *opts.Temperature
	}
	if opts.TopP != nil {
		params.TopP = *opts.TopP
	}
	if opts.MaxTokens > 0 {
		params.MaxTokens = opts.MaxTokens
	}
	if len(opts.Stop) > 0 {
		params.Stop = opts.Stop
	}

	// 流式模式下启用增量输出
	if stream {
		params.IncrementalOutput = true
	}

	// 添加工具定义
	if len(opts.Tools) > 0 {
		params.Tools = make([]dashScopeTool, len(opts.Tools))
		for i, tool := range opts.Tools {
			params.Tools[i] = dashScopeTool{
				Type: tool.Type,
				Function: dashScopeFunctionDef{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				},
			}
		}
	}

	// 添加工具选择
	if opts.ToolChoice != nil {
		params.ToolChoice = opts.ToolChoice
	}

	// 构建请求体
	reqBody := dashScopeRequest{
		Model: m.config.Model,
		Input: dashScopeInput{
			Messages: dsMessages,
		},
		Parameters: params,
	}

	// 序列化请求体
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 确定 API 端点
	endpoint := DashScopeGenerationEndpoint
	if m.isMultiModalModel(m.config.Model) {
		endpoint = DashScopeMultiModalEndpoint
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest("POST", m.baseURL+endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Authorization", common.AuthPrefixBearer+m.config.APIKey)

	return req, nil
}

// doRequest 执行 HTTP 请求
func (m *DashScopeModel) doRequest(ctx context.Context, req *http.Request) (*dashScopeResponse, error) {
	req = req.WithContext(ctx)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var dsResp dashScopeResponse
	if err := json.Unmarshal(bodyBytes, &dsResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w, body: %s", err, string(bodyBytes))
	}

	// 检查错误
	if dsResp.Code != "" && dsResp.Code != "Success" {
		return nil, fmt.Errorf("API 错误: %s - %s", dsResp.Code, dsResp.Message)
	}

	return &dsResp, nil
}

// doStreamRequest 执行流式 HTTP 请求
func (m *DashScopeModel) doStreamRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP 错误: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}

// handleStreamResponse 处理流式响应
func (m *DashScopeModel) handleStreamResponse(resp *http.Response, ch chan<- StreamChunk) {
	defer close(ch)
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var lastContent string

	for scanner.Scan() {
		line := scanner.Text()
		if m.processStreamLine(line, ch, &lastContent) {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("读取流式响应失败: %w", err)}
	}
}

// processStreamLine 处理单行流数据
func (m *DashScopeModel) processStreamLine(line string, ch chan<- StreamChunk, lastContent *string) bool {
	if line == "" || !strings.HasPrefix(line, "data:") {
		return false
	}

	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

	if data == "[DONE]" {
		ch <- StreamChunk{Done: true}
		return true
	}

	return m.processStreamData(data, ch, lastContent)
}

// processStreamData 处理流数据
func (m *DashScopeModel) processStreamData(data string, ch chan<- StreamChunk, lastContent *string) bool {
	var dsResp dashScopeResponse
	if err := json.Unmarshal([]byte(data), &dsResp); err != nil {
		ch <- StreamChunk{Error: fmt.Errorf("解析流式响应失败: %w", err)}
		return false
	}

	if dsResp.Code != "" && dsResp.Code != "Success" {
		ch <- StreamChunk{Error: fmt.Errorf("API 错误: %s - %s", dsResp.Code, dsResp.Message)}
		return false
	}

	if len(dsResp.Output.Choices) == 0 {
		return false
	}

	choice := dsResp.Output.Choices[0]
	m.sendContentDelta(choice.Message, ch, lastContent)
	m.sendToolCalls(choice.Message, ch)

	if choice.FinishReason != "" {
		ch <- StreamChunk{FinishReason: choice.FinishReason, Done: true}
		return true
	}

	return false
}

// sendContentDelta 发送增量内容
func (m *DashScopeModel) sendContentDelta(message dashScopeMessage, ch chan<- StreamChunk, lastContent *string) {
	content := m.extractContent(message.Content)
	if content == "" {
		return
	}

	delta := content
	if strings.HasPrefix(content, *lastContent) {
		delta = content[len(*lastContent):]
	}
	*lastContent = content

	if delta != "" {
		ch <- StreamChunk{Delta: delta}
	}
}

// sendToolCalls 发送工具调用
func (m *DashScopeModel) sendToolCalls(message dashScopeMessage, ch chan<- StreamChunk) {
	for _, tc := range message.ToolCalls {
		ch <- StreamChunk{
			ToolCallDelta: &ToolCallDelta{
				ID:   tc.ID,
				Type: tc.Type,
				Function: &FunctionCallDelta{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			},
		}
	}
}

// parseResponse 解析响应
func (m *DashScopeModel) parseResponse(resp *dashScopeResponse) (*Response, error) {
	if len(resp.Output.Choices) == 0 {
		return nil, errors.New("响应中没有选择项")
	}

	choice := resp.Output.Choices[0]
	message := choice.Message

	result := &Response{
		Content:      m.extractContent(message.Content),
		FinishReason: choice.FinishReason,
		Model:        m.config.Model,
		Usage: Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// 解析工具调用
	if len(message.ToolCalls) > 0 {
		result.ToolCalls = make([]ToolCall, len(message.ToolCalls))
		for i, tc := range message.ToolCalls {
			result.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return result, nil
}

// convertMessages 转换消息格式
func (m *DashScopeModel) convertMessages(messages []Message) []dashScopeMessage {
	result := make([]dashScopeMessage, len(messages))

	for i, msg := range messages {
		dsMsg := dashScopeMessage{
			Role: string(msg.Role),
		}

		// 处理多模态模型的消息格式
		if m.isMultiModalModel(m.config.Model) {
			// 多模态模型需要将内容转换为数组格式
			dsMsg.Content = []dashScopeContentBlock{{Text: msg.Content}}
		} else {
			dsMsg.Content = msg.Content
		}

		// 设置名称
		if msg.Name != "" {
			dsMsg.Name = msg.Name
		}

		// 转换工具调用
		if len(msg.ToolCalls) > 0 {
			dsMsg.ToolCalls = make([]dashScopeToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				dsMsg.ToolCalls[j] = dashScopeToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: dashScopeFunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}

		// 设置工具调用 ID
		if msg.ToolCallID != "" {
			dsMsg.ToolCallID = msg.ToolCallID
		}

		result[i] = dsMsg
	}

	return result
}

// extractContent 从消息内容中提取文本
func (m *DashScopeModel) extractContent(content interface{}) string {
	if content == nil {
		return ""
	}

	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		// 多模态模型返回的内容块数组
		var result string
		for _, item := range v {
			if block, ok := item.(map[string]interface{}); ok {
				if text, ok := block["text"].(string); ok {
					result += text
				}
			}
		}
		return result
	default:
		return fmt.Sprintf("%v", content)
	}
}

// isMultiModalModel 检查是否为多模态模型
func (m *DashScopeModel) isMultiModalModel(model string) bool {
	modelLower := strings.ToLower(model)
	for _, prefix := range dashScopeMultiModalPrefixes {
		if strings.HasPrefix(modelLower, prefix) {
			return true
		}
	}
	// 额外检查已知的多模态模型
	multiModalModels := map[string]bool{
		DashScopeQwenVLPlus:   true,
		DashScopeQwenVLMax:    true,
		DashScopeQwen3_5_72B:  true,
		DashScopeQwen3_5_32B:  true,
		DashScopeQwen3_5_14B:  true,
		DashScopeQwen3_5_7B:   true,
		DashScopeQwen3_5_3B:   true,
		DashScopeQwen3_5_1_5B: true,
		DashScopeQwen3_5_0_5B: true,
	}
	return multiModalModels[model]
}

// mapDashScopeModel 将用户输入的模型名称映射到标准模型标识符
func mapDashScopeModel(model string) string {
	// 模型名称别名映射
	aliases := map[string]string{
		"qwen":                 DashScopeQwenTurbo,
		"qwen-turbo":           DashScopeQwenTurbo,
		"qwen-plus":            DashScopeQwenPlus,
		"qwen-max":             DashScopeQwenMax,
		"qwen-max-long":        DashScopeQwenMaxLongCtx,
		"qwen-max-longcontext": DashScopeQwenMaxLongCtx,
		"qwen-long":            DashScopeQwenLong,
		"qwen-vl":              DashScopeQwenVLPlus,
		"qwen-vl-plus":         DashScopeQwenVLPlus,
		"qwen-vl-max":          DashScopeQwenVLMax,
		"qwen-math":            DashScopeQwenMathPlus,
		"qwen-math-plus":       DashScopeQwenMathPlus,
		"qwen-coder":           DashScopeQwenCoderPlus,
		"qwen-coder-plus":      DashScopeQwenCoderPlus,
		"qwq":                  DashScopeQwQPlus,
		"qwq-plus":             DashScopeQwQPlus,
		"qwq-32b":              DashScopeQwQ32B,
	}

	if mapped, ok := aliases[model]; ok {
		return mapped
	}
	return model
}

// parseProxyURL 解析代理 URL
func parseProxyURL(proxy string) (*url.URL, error) {
	return url.Parse(proxy)
}

// GetDashScopeModelInfo 获取指定模型的能力信息
func GetDashScopeModelInfo(model string) *ModelInfo {
	model = mapDashScopeModel(model)
	if info, ok := dashScopeModelInfo[model]; ok {
		return &info
	}
	return nil
}

// ListDashScopeModels 返回所有支持的灵积平台模型列表
func ListDashScopeModels() []ModelInfo {
	models := make([]ModelInfo, 0, len(dashScopeModelInfo))
	for _, info := range dashScopeModelInfo {
		models = append(models, info)
	}
	return models
}

// 确保 DashScopeModel 实现 Model 接口
var _ Model = (*DashScopeModel)(nil)
