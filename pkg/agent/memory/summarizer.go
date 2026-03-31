// Package memory 提供 Agent 记忆管理功能。
// summarizer.go 实现文本摘要器，用于将对话内容压缩成简洁的记忆摘要。
package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/common"
)

// SummarizeSystemPrompt 用于 LLM 摘要的系统提示词。
const SummarizeSystemPrompt = `你是一个记忆提取助手。你的任务是从对话记录中提取值得记住的信息，生成简洁的记忆摘要。

输出要求：
1. 以事件/关键信息为维度记录，每条一行，用 "- " 开头
2. 记录有价值的关键信息，例如用户提出的要求及助手的解决方案，对话中涉及的事实信息，用户的偏好、决策或重要结论
3. 每条摘要需要简明扼要，只保留关键信息
4. 直接输出摘要内容，不要加任何前缀说明
5. 当对话没有任何记录价值例如只是简单问候，可回复"无"`

// SummarizeUserPromptTemplate 用于 LLM 摘要的用户提示词模板。
const SummarizeUserPromptTemplate = `请从以下对话记录中提取关键信息，生成记忆摘要：

%s`

// 摘要相关常量
const (
	// summaryEmptyMark 表示摘要无内容时的标记
	summaryEmptyMark = "无"
	// rolePrefixUser 用户角色前缀
	rolePrefixUser = "用户: "
	// rolePrefixAssistant 助手角色前缀
	rolePrefixAssistant = "助手: "
)

// LLMClient 定义 LLM 客户端接口。
type LLMClient interface {
	// Call 调用 LLM 生成响应。
	Call(ctx context.Context, systemPrompt, userPrompt string, opts ...LLMOption) (string, error)
}

// LLMOption 是 LLM 调用的选项。
type LLMOption func(*LLMConfig)

// LLMConfig 包含 LLM 调用的配置。
type LLMConfig struct {
	Temperature float64
	MaxTokens   int
	Stream      bool
}

// WithLLMTemperature 设置温度参数。
func WithLLMTemperature(temp float64) LLMOption {
	return func(c *LLMConfig) {
		c.Temperature = temp
	}
}

// WithLLMMaxTokens 设置最大 token 数。
func WithLLMMaxTokens(maxTokens int) LLMOption {
	return func(c *LLMConfig) {
		c.MaxTokens = maxTokens
	}
}

// WithLLMStream 设置是否流式输出。
func WithLLMStream(stream bool) LLMOption {
	return func(c *LLMConfig) {
		c.Stream = stream
	}
}

// MemoryFlushManager 管理记忆刷新操作。
// 当对话上下文被裁剪或溢出时触发刷新，将丢弃的内容摘要写入持久化存储。
type MemoryFlushManager struct {
	mu sync.Mutex

	// workspaceDir 是工作空间目录。
	workspaceDir string
	// llmClient 是 LLM 客户端。
	llmClient LLMClient
	// memoryDir 是记忆存储目录。
	memoryDir string
	// lastFlushTimestamp 是上次刷新的时间戳。
	lastFlushTimestamp *time.Time
	// trimFlushedHashes 是已刷新内容的哈希集合，用于去重。
	trimFlushedHashes map[string]bool
	// lastFlushedContentHash 是上次刷新内容的哈希，用于每日摘要去重。
	lastFlushedContentHash string
}

// FlushOption 是刷新操作的选项。
type FlushOption func(*FlushConfig)

// FlushConfig 包含刷新操作的配置。
type FlushConfig struct {
	UserID      string
	Reason      string
	MaxMessages int
}

// WithFlushUserID 设置用户 ID。
func WithFlushUserID(userID string) FlushOption {
	return func(c *FlushConfig) {
		c.UserID = userID
	}
}

// WithFlushReason 设置刷新原因。
func WithFlushReason(reason string) FlushOption {
	return func(c *FlushConfig) {
		c.Reason = reason
	}
}

// WithFlushMaxMessages 设置最大消息数。
func WithFlushMaxMessages(max int) FlushOption {
	return func(c *FlushConfig) {
		c.MaxMessages = max
	}
}

// NewMemoryFlushManager 创建一个新的记忆刷新管理器。
func NewMemoryFlushManager(workspaceDir string, llmClient LLMClient) (*MemoryFlushManager, error) {
	memoryDir := filepath.Join(workspaceDir, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return nil, fmt.Errorf("创建记忆目录失败: %w", err)
	}

	return &MemoryFlushManager{
		workspaceDir:      workspaceDir,
		llmClient:         llmClient,
		memoryDir:         memoryDir,
		trimFlushedHashes: make(map[string]bool),
	}, nil
}

// GetTodayMemoryFile 获取今日记忆文件路径。
func (m *MemoryFlushManager) GetTodayMemoryFile(userID string, ensureExists bool) (string, error) {
	today := time.Now().Format("2006-01-02")

	var filePath string
	if userID != "" {
		userDir := filepath.Join(m.memoryDir, "users", userID)
		if ensureExists {
			if err := os.MkdirAll(userDir, 0755); err != nil {
				return "", err
			}
		}
		filePath = filepath.Join(userDir, today+".md")
	} else {
		filePath = filepath.Join(m.memoryDir, today+".md")
	}

	if ensureExists {
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return "", err
		}
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			content := fmt.Sprintf("# Daily Memory: %s\n\n", today)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return "", err
			}
		}
	}

	return filePath, nil
}

// GetMainMemoryFile 获取主记忆文件路径。
func (m *MemoryFlushManager) GetMainMemoryFile(userID string) string {
	if userID != "" {
		userDir := filepath.Join(m.memoryDir, "users", userID)
		os.MkdirAll(userDir, 0755)
		return filepath.Join(userDir, common.MemoryFileName)
	}
	return filepath.Join(m.workspaceDir, common.MemoryFileName)
}

// GetStatus 返回刷新管理器的状态。
func (m *MemoryFlushManager) GetStatus() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	status := map[string]interface{}{
		"today_file": "",
		"main_file":  m.GetMainMemoryFile(""),
	}

	if m.lastFlushTimestamp != nil {
		status["last_flush_time"] = m.lastFlushTimestamp.Format(time.RFC3339)
	}

	todayFile, _ := m.GetTodayMemoryFile("", false)
	status["today_file"] = todayFile

	return status
}

// FlushFromMessages 从消息列表生成摘要并写入持久化存储。
func (m *MemoryFlushManager) FlushFromMessages(ctx context.Context, messages []*Message, opts ...FlushOption) error {
	config := &FlushConfig{
		Reason:      "trim",
		MaxMessages: 0,
	}
	for _, opt := range opts {
		opt(config)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 去重
	var deduped []*Message
	for _, msg := range messages {
		if msg.Content == "" || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		hash := ComputeHash(msg.Content)
		if !m.trimFlushedHashes[hash] {
			m.trimFlushedHashes[hash] = true
			deduped = append(deduped, msg)
		}
	}

	if len(deduped) == 0 {
		return nil
	}

	// 异步执行摘要和写入
	go m.flushWorker(context.Background(), deduped, config.UserID, config.Reason, config.MaxMessages)

	return nil
}

// flushWorker 后台工作线程：使用 LLM 生成摘要并写入文件。
func (m *MemoryFlushManager) flushWorker(ctx context.Context, messages []*Message, userID, reason string, maxMessages int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	summary, err := m.summarizeMessages(ctx, messages, maxMessages)
	if err != nil {
		return
	}

	if summary == "" || strings.TrimSpace(summary) == "" || strings.TrimSpace(summary) == summaryEmptyMark {
		return
	}

	dailyFile, err := m.GetTodayMemoryFile(userID, true)
	if err != nil {
		return
	}

	var header, note string
	now := time.Now().Format("15:04")

	switch reason {
	case "overflow":
		header = fmt.Sprintf("## Context Overflow Recovery (%s)", now)
		note = "The following conversation was trimmed due to context overflow:\n"
	case "trim":
		header = fmt.Sprintf("## Trimmed Context (%s)", now)
		note = ""
	case "daily_summary":
		header = fmt.Sprintf("## Daily Summary (%s)", now)
		note = ""
	default:
		header = fmt.Sprintf("## Session Notes (%s)", now)
		note = ""
	}

	flushEntry := fmt.Sprintf("\n%s\n\n%s%s\n", header, note, summary)

	f, err := os.OpenFile(dailyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.WriteString(flushEntry)

	timestamp := time.Now()
	m.lastFlushTimestamp = &timestamp
}

// CreateDailySummary 生成每日摘要。
func (m *MemoryFlushManager) CreateDailySummary(ctx context.Context, messages []*Message, userID string) error {
	content := ""
	for _, msg := range messages {
		content += msg.Content
	}
	contentHash := ComputeHash(content)

	m.mu.Lock()
	if contentHash == m.lastFlushedContentHash {
		m.mu.Unlock()
		return nil
	}
	m.lastFlushedContentHash = contentHash
	m.mu.Unlock()

	return m.FlushFromMessages(ctx, messages,
		WithFlushUserID(userID),
		WithFlushReason("daily_summary"),
		WithFlushMaxMessages(0),
	)
}

// summarizeMessages 使用 LLM 摘要消息，带有规则后备。
func (m *MemoryFlushManager) summarizeMessages(ctx context.Context, messages []*Message, maxMessages int) (string, error) {
	conversationText := m.formatConversationForSummary(messages, maxMessages)
	if strings.TrimSpace(conversationText) == "" {
		return "", nil
	}

	// 尝试 LLM 摘要
	if m.llmClient != nil {
		summary, err := m.llmClient.Call(ctx, SummarizeSystemPrompt,
			fmt.Sprintf(SummarizeUserPromptTemplate, conversationText),
			WithLLMTemperature(0),
			WithLLMMaxTokens(500),
		)
		if err == nil && summary != "" && strings.TrimSpace(summary) != summaryEmptyMark {
			return strings.TrimSpace(summary), nil
		}
	}

	// 规则后备
	return m.extractSummaryFallback(messages, maxMessages), nil
}

// formatConversationForSummary 格式化消息用于摘要。
func (m *MemoryFlushManager) formatConversationForSummary(messages []*Message, maxMessages int) string {
	msgs := messages
	if maxMessages > 0 && len(messages) > maxMessages*2 {
		msgs = messages[len(messages)-maxMessages*2:]
	}

	var lines []string
	for _, msg := range msgs {
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}
		if len(text) > 500 {
			text = text[:500]
		}
		switch msg.Role {
		case RoleUser:
			lines = append(lines, fmt.Sprintf("%s%s", rolePrefixUser, text))
		case RoleAssistant:
			lines = append(lines, fmt.Sprintf("%s%s", rolePrefixAssistant, text))
		}
	}
	return strings.Join(lines, "\n")
}

// extractSummaryFallback 规则后备摘要方法。
func (m *MemoryFlushManager) extractSummaryFallback(messages []*Message, maxMessages int) string {
	msgs := messages
	if maxMessages > 0 && len(messages) > maxMessages*2 {
		msgs = messages[len(messages)-maxMessages*2:]
	}

	var items []string
	for _, msg := range msgs {
		if len(items) >= 15 {
			break
		}
		if item := m.extractMessageSummaryItem(msg); item != "" {
			items = append(items, item)
		}
	}

	return strings.Join(items, "\n")
}

// extractMessageSummaryItem 从单条消息提取摘要项。
func (m *MemoryFlushManager) extractMessageSummaryItem(msg *Message) string {
	text := strings.TrimSpace(msg.Content)
	if text == "" {
		return ""
	}

	switch msg.Role {
	case RoleUser:
		return m.extractUserSummaryItem(text)
	case RoleAssistant:
		return m.extractAssistantSummaryItem(text)
	default:
		return ""
	}
}

// extractUserSummaryItem 从用户消息提取摘要项。
func (m *MemoryFlushManager) extractUserSummaryItem(text string) string {
	if len(text) <= 5 {
		return ""
	}
	if len(text) > 200 {
		text = text[:200]
	}
	return fmt.Sprintf("- 用户请求: %s", text)
}

// extractAssistantSummaryItem 从助手消息提取摘要项。
func (m *MemoryFlushManager) extractAssistantSummaryItem(text string) string {
	firstLine := strings.Split(text, "\n")[0]
	firstLine = strings.TrimSpace(firstLine)
	if len(firstLine) <= 10 {
		return ""
	}
	if len(firstLine) > 200 {
		firstLine = firstLine[:200]
	}
	return fmt.Sprintf("- 处理结果: %s", firstLine)
}

// CreateMemoryFilesIfNeeded 创建必要的记忆文件。
func CreateMemoryFilesIfNeeded(workspaceDir, userID string) error {
	memoryDir := filepath.Join(workspaceDir, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return err
	}

	var mainMemory string
	if userID != "" {
		userDir := filepath.Join(memoryDir, "users", userID)
		if err := os.MkdirAll(userDir, 0755); err != nil {
			return err
		}
		mainMemory = filepath.Join(userDir, common.MemoryFileName)
	} else {
		mainMemory = filepath.Join(workspaceDir, common.MemoryFileName)
	}

	if _, err := os.Stat(mainMemory); os.IsNotExist(err) {
		if err := os.WriteFile(mainMemory, []byte(""), 0644); err != nil {
			return err
		}
	}

	return nil
}

// EnsureDailyMemoryFile 确保今日记忆文件存在。
func EnsureDailyMemoryFile(workspaceDir, userID string) (string, error) {
	memoryDir := filepath.Join(workspaceDir, "memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return "", err
	}

	today := time.Now().Format("2006-01-02")
	var todayMemory string

	if userID != "" {
		userDir := filepath.Join(memoryDir, "users", userID)
		if err := os.MkdirAll(userDir, 0755); err != nil {
			return "", err
		}
		todayMemory = filepath.Join(userDir, today+".md")
	} else {
		todayMemory = filepath.Join(memoryDir, today+".md")
	}

	if _, err := os.Stat(todayMemory); os.IsNotExist(err) {
		content := fmt.Sprintf("# Daily Memory: %s\n\n", today)
		if err := os.WriteFile(todayMemory, []byte(content), 0644); err != nil {
			return "", err
		}
	}

	return todayMemory, nil
}

// TextSummarizer 实现简单的文本摘要功能。
type TextSummarizer struct {
	llmClient LLMClient
}

// NewTextSummarizer 创建一个新的文本摘要器。
func NewTextSummarizer(llmClient LLMClient) *TextSummarizer {
	return &TextSummarizer{
		llmClient: llmClient,
	}
}

// Summarize 从消息生成摘要。
func (s *TextSummarizer) Summarize(ctx context.Context, messages []*Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	conversationText := ""
	for _, msg := range messages {
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}
		switch msg.Role {
		case RoleUser:
			conversationText += fmt.Sprintf("%s%s\n", rolePrefixUser, text)
		case RoleAssistant:
			conversationText += fmt.Sprintf("%s%s\n", rolePrefixAssistant, text)
		}
	}

	if strings.TrimSpace(conversationText) == "" {
		return "", nil
	}

	if s.llmClient != nil {
		summary, err := s.llmClient.Call(ctx, SummarizeSystemPrompt,
			fmt.Sprintf(SummarizeUserPromptTemplate, conversationText),
			WithLLMTemperature(0),
			WithLLMMaxTokens(500),
		)
		if err == nil {
			return strings.TrimSpace(summary), nil
		}
	}

	// 简单后备：返回消息数量统计
	return fmt.Sprintf("共 %d 条消息", len(messages)), nil
}

// Flush 将摘要写入持久化存储。
func (s *TextSummarizer) Flush(ctx context.Context, messages []*Message, userID string, reason string) error {
	summary, err := s.Summarize(ctx, messages)
	if err != nil {
		return err
	}

	if summary == "" || strings.TrimSpace(summary) == summaryEmptyMark {
		return nil
	}

	// 写入今日记忆文件
	dailyFile, err := EnsureDailyMemoryFile(".", userID)
	if err != nil {
		return err
	}

	now := time.Now().Format("15:04")
	header := fmt.Sprintf("## Session Summary (%s)", now)
	if reason != "" {
		header = fmt.Sprintf("## Session Summary - %s (%s)", reason, now)
	}

	flushEntry := fmt.Sprintf("\n%s\n\n%s\n", header, summary)

	f, err := os.OpenFile(dailyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(flushEntry)
	return err
}
