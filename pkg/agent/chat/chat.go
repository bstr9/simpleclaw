// Package chat 提供对话管理功能，支持多轮对话和上下文管理。
package chat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/llm"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// ChunkType 流式输出块类型
type ChunkType string

const (
	// ChunkTypeContent 文本内容块
	ChunkTypeContent ChunkType = "content"
	// ChunkTypeToolStart 工具开始执行
	ChunkTypeToolStart ChunkType = "tool_start"
	// ChunkTypeToolCalls 工具调用结果
	ChunkTypeToolCalls ChunkType = "tool_calls"
	// ChunkTypeError 错误块
	ChunkTypeError ChunkType = "error"
	// ChunkTypeComplete 完成块
	ChunkTypeComplete ChunkType = "complete"
)

// EventType 事件类型
type EventType string

const (
	// EventText 文本事件
	EventText EventType = "text"
	// EventToolCall 工具调用事件
	EventToolCall EventType = "tool_call"
	// EventToolResult 工具结果事件
	EventToolResult EventType = "tool_result"
	// EventError 错误事件
	EventError EventType = "error"
	// EventStepStart 步骤开始事件
	EventStepStart EventType = "step_start"
	// EventStepEnd 步骤结束事件
	EventStepEnd EventType = "step_end"
	// EventComplete 完成事件
	EventComplete EventType = "complete"
	// EventTurnEnd 轮次结束事件
	EventTurnEnd EventType = "turn_end"
)

// Chunk 表示流式输出的数据块
type Chunk struct {
	// Type 块类型
	Type ChunkType `json:"chunk_type"`
	// Delta 增量文本内容
	Delta string `json:"delta,omitempty"`
	// SegmentID 段落ID
	SegmentID int `json:"segment_id,omitempty"`
	// Tool 工具名称
	Tool string `json:"tool,omitempty"`
	// Arguments 工具参数
	Arguments map[string]any `json:"arguments,omitempty"`
	// ToolCalls 工具调用结果列表
	ToolCalls []ToolCallInfo `json:"tool_calls,omitempty"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
}

// ToolCallInfo 工具调用信息
type ToolCallInfo struct {
	// Name 工具名称
	Name string `json:"name"`
	// Arguments 工具参数
	Arguments map[string]any `json:"arguments,omitempty"`
	// Result 执行结果
	Result string `json:"result,omitempty"`
	// Status 执行状态
	Status string `json:"status,omitempty"`
	// Elapsed 执行耗时
	Elapsed string `json:"elapsed,omitempty"`
}

// ChatOptions 对话选项
type ChatOptions struct {
	// MaxContextTurns 最大上下文轮次
	MaxContextTurns int
	// Temperature 温度参数
	Temperature float64
	// MaxTokens 最大token数
	MaxTokens int
	// Stream 是否流式输出
	Stream bool
}

// DefaultChatOptions 默认对话选项
func DefaultChatOptions() *ChatOptions {
	return &ChatOptions{
		MaxContextTurns: 20,
		Temperature:     0.7,
		MaxTokens:       2048,
		Stream:          true,
	}
}

// ChatOption 对话选项函数
type ChatOption func(*ChatOptions)

// WithMaxContextTurns 设置最大上下文轮次
func WithMaxContextTurns(turns int) ChatOption {
	return func(o *ChatOptions) {
		o.MaxContextTurns = turns
	}
}

// WithTemperature 设置温度参数
func WithTemperature(temp float64) ChatOption {
	return func(o *ChatOptions) {
		o.Temperature = temp
	}
}

// WithMaxTokens 设置最大token数
func WithMaxTokens(tokens int) ChatOption {
	return func(o *ChatOptions) {
		o.MaxTokens = tokens
	}
}

// WithStream 设置是否流式输出
func WithStream(stream bool) ChatOption {
	return func(o *ChatOptions) {
		o.Stream = stream
	}
}

// AgentExecutor Agent执行器接口
type AgentExecutor interface {
	// Run 执行Agent并返回结果
	Run(ctx context.Context, query string, onEvent func(event map[string]any)) (string, error)
	// RunWithHistory 使用历史消息执行Agent
	RunWithHistory(ctx context.Context, messages []llm.Message, onEvent func(event map[string]any)) (string, error)
}

// ChatService 对话服务
type ChatService struct {
	// sessionManager 会话管理器
	sessionManager *SessionManager
	// agentFactory Agent工厂函数
	agentFactory func(sessionID string) (AgentExecutor, error)
	// options 默认对话选项
	options *ChatOptions
}

// NewChatService 创建新的对话服务
func NewChatService(sessionManager *SessionManager, agentFactory func(sessionID string) (AgentExecutor, error), opts ...ChatOption) *ChatService {
	options := DefaultChatOptions()
	for _, opt := range opts {
		opt(options)
	}

	return &ChatService{
		sessionManager: sessionManager,
		agentFactory:   agentFactory,
		options:        options,
	}
}

// Run 执行对话
func (s *ChatService) Run(ctx context.Context, sessionID string, query string, sendChunk func(chunk Chunk), opts ...ChatOption) (string, error) {
	options := *s.options
	for _, opt := range opts {
		opt(&options)
	}

	session, err := s.sessionManager.GetOrCreateSession(sessionID)
	if err != nil {
		return "", fmt.Errorf("获取会话失败: %w", err)
	}

	agent, err := s.agentFactory(sessionID)
	if err != nil {
		return "", fmt.Errorf("创建Agent失败: %w", err)
	}

	state := newStreamState()

	onEvent := func(event map[string]any) {
		s.handleEvent(event, state, sendChunk)
	}

	session.AddUserMessage(query)

	response, err := agent.RunWithHistory(ctx, session.GetMessages(), onEvent)
	if err != nil {
		sendChunk(Chunk{
			Type:  ChunkTypeError,
			Error: err.Error(),
		})
		return "", err
	}

	if response != "" {
		session.AddAssistantMessage(response)
	}

	sendChunk(Chunk{
		Type: ChunkTypeComplete,
	})

	logger.Info("对话完成",
		zap.String("session_id", sessionID),
		zap.Int("message_count", session.GetMessageCount()))

	return response, nil
}

// RunWithSession 使用现有会话执行对话
func (s *ChatService) RunWithSession(ctx context.Context, session *Session, query string, sendChunk func(chunk Chunk), opts ...ChatOption) (string, error) {
	options := *s.options
	for _, opt := range opts {
		opt(&options)
	}

	agent, err := s.agentFactory(session.ID)
	if err != nil {
		return "", fmt.Errorf("创建Agent失败: %w", err)
	}

	state := newStreamState()

	onEvent := func(event map[string]any) {
		s.handleEvent(event, state, sendChunk)
	}

	session.AddUserMessage(query)

	response, err := agent.RunWithHistory(ctx, session.GetMessages(), onEvent)
	if err != nil {
		sendChunk(Chunk{
			Type:  ChunkTypeError,
			Error: err.Error(),
		})
		return "", err
	}

	if response != "" {
		session.AddAssistantMessage(response)
	}

	sendChunk(Chunk{
		Type: ChunkTypeComplete,
	})

	return response, nil
}

// handleEvent 处理Agent事件并转换为Chunk
func (s *ChatService) handleEvent(event map[string]any, state *streamState, sendChunk func(chunk Chunk)) {
	eventType, ok := event["type"].(string)
	if !ok {
		return
	}

	data, _ := event["data"].(map[string]any)

	switch EventType(eventType) {
	case EventText:
		delta, _ := data["delta"].(string)
		if delta != "" {
			sendChunk(Chunk{
				Type:      ChunkTypeContent,
				Delta:     delta,
				SegmentID: state.segmentID,
			})
		}

	case EventToolCall:
		toolName, _ := data["tool_name"].(string)
		arguments, _ := data["arguments"].(map[string]any)
		toolCallID, _ := data["tool_call_id"].(string)
		if toolCallID == "" {
			toolCallID = toolName
		}
		state.pendingToolArguments[toolCallID] = arguments

		sendChunk(Chunk{
			Type:      ChunkTypeToolStart,
			Tool:      toolName,
			Arguments: arguments,
		})

	case EventToolResult:
		toolName, _ := data["tool_name"].(string)
		toolCallID, _ := data["tool_call_id"].(string)
		if toolCallID == "" {
			toolCallID = toolName
		}

		arguments := state.pendingToolArguments[toolCallID]
		delete(state.pendingToolArguments, toolCallID)

		result := s.formatResult(data["result"])
		status, _ := data["status"].(string)
		if status == "" {
			status = "unknown"
		}

		executionTime, _ := data["execution_time"].(float64)

		toolInfo := ToolCallInfo{
			Name:      toolName,
			Arguments: arguments,
			Result:    result,
			Status:    status,
			Elapsed:   fmt.Sprintf("%.2fs", executionTime),
		}

		if state.pendingToolResults != nil {
			state.pendingToolResults = append(state.pendingToolResults, toolInfo)
		}

	case EventTurnEnd:
		hasToolCalls, _ := data["has_tool_calls"].(bool)
		if hasToolCalls && len(state.pendingToolResults) > 0 {
			sendChunk(Chunk{
				Type:      ChunkTypeToolCalls,
				ToolCalls: state.pendingToolResults,
			})
			state.pendingToolResults = nil
			state.segmentID++
		}

	case EventComplete:
		sendChunk(Chunk{
			Type: ChunkTypeComplete,
		})
	}
}

// formatResult 格式化工具执行结果
func (s *ChatService) formatResult(result any) string {
	if result == nil {
		return ""
	}

	switch v := result.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", result)
	}
}

// streamState 流式输出状态
type streamState struct {
	segmentID            int
	pendingToolResults   []ToolCallInfo
	pendingToolArguments map[string]map[string]any
}

// newStreamState 创建新的流式状态
func newStreamState() *streamState {
	return &streamState{
		segmentID:            0,
		pendingToolResults:   make([]ToolCallInfo, 0),
		pendingToolArguments: make(map[string]map[string]any),
	}
}

// ContextManager 上下文管理器
type ContextManager struct {
	// maxMessages 最大消息数
	maxMessages int
	// maxTokens 最大token数（估算）
	maxTokens int
	// summarizer 摘要生成器
	summarizer ContextSummarizer
}

// ContextSummarizer 上下文摘要接口
type ContextSummarizer interface {
	Summarize(ctx context.Context, messages []llm.Message) (string, error)
}

// NewContextManager 创建新的上下文管理器
func NewContextManager(maxMessages, maxTokens int) *ContextManager {
	return &ContextManager{
		maxMessages: maxMessages,
		maxTokens:   maxTokens,
	}
}

// WithSummarizer 设置摘要生成器
func (cm *ContextManager) WithSummarizer(summarizer ContextSummarizer) *ContextManager {
	cm.summarizer = summarizer
	return cm
}

// TrimMessages 裁剪消息历史
func (cm *ContextManager) TrimMessages(messages []llm.Message, keepSystem bool) []llm.Message {
	if len(messages) <= cm.maxMessages {
		return messages
	}

	result := make([]llm.Message, 0, cm.maxMessages+1)

	if keepSystem && len(messages) > 0 && messages[0].Role == llm.RoleSystem {
		result = append(result, messages[0])
		messages = messages[1:]
	}

	start := len(messages) - cm.maxMessages
	if start < 0 {
		start = 0
	}

	result = append(result, messages[start:]...)

	return result
}

// EstimateTokens 估算消息的token数
func (cm *ContextManager) EstimateTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content) / 4
		for _, tc := range msg.ToolCalls {
			total += len(tc.Function.Name) / 4
			total += len(tc.Function.Arguments) / 4
		}
	}
	return total
}

// ShouldSummarize 判断是否需要生成摘要
func (cm *ContextManager) ShouldSummarize(messages []llm.Message) bool {
	return len(messages) > cm.maxMessages || cm.EstimateTokens(messages) > cm.maxTokens
}

// ConversationHistory 对话历史管理
type ConversationHistory struct {
	// messages 消息列表
	messages []llm.Message
	// mu 互斥锁
	mu sync.RWMutex
	// maxMessages 最大消息数
	maxMessages int
}

// NewConversationHistory 创建新的对话历史
func NewConversationHistory(maxMessages int) *ConversationHistory {
	return &ConversationHistory{
		messages:    make([]llm.Message, 0),
		maxMessages: maxMessages,
	}
}

// Add 添加消息
func (h *ConversationHistory) Add(msg llm.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, msg)

	if h.maxMessages > 0 && len(h.messages) > h.maxMessages {
		start := len(h.messages) - h.maxMessages
		h.messages = h.messages[start:]
	}
}

// AddUser 添加用户消息
func (h *ConversationHistory) AddUser(content string) {
	h.Add(llm.Message{Role: llm.RoleUser, Content: content})
}

// AddAssistant 添加助手消息
func (h *ConversationHistory) AddAssistant(content string) {
	h.Add(llm.Message{Role: llm.RoleAssistant, Content: content})
}

// GetAll 获取所有消息
func (h *ConversationHistory) GetAll() []llm.Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]llm.Message, len(h.messages))
	copy(result, h.messages)
	return result
}

// GetLast 获取最近n条消息
func (h *ConversationHistory) GetLast(n int) []llm.Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if n <= 0 || n >= len(h.messages) {
		result := make([]llm.Message, len(h.messages))
		copy(result, h.messages)
		return result
	}

	result := make([]llm.Message, n)
	copy(result, h.messages[len(h.messages)-n:])
	return result
}

// Clear 清空历史
func (h *ConversationHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = make([]llm.Message, 0)
}

// Count 获取消息数量
func (h *ConversationHistory) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.messages)
}

// ChatResult 对话结果
type ChatResult struct {
	// Response 响应内容
	Response string
	// MessageCount 消息数量
	MessageCount int
	// TokenUsage token使用情况
	TokenUsage *llm.Usage
	// Duration 执行耗时
	Duration time.Duration
	// ToolCalls 工具调用记录
	ToolCalls []ToolCallInfo
}
