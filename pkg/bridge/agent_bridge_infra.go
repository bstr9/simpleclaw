// Package bridge 提供消息处理的核心路由层
// agent_bridge_infra.go 基础设施方法（翻译、记忆、嵌入、缓存、速率限制、时间、协议、任务）
package bridge

import (
	"context"

	"github.com/bstr9/simpleclaw/pkg/agent/memory"
	"github.com/bstr9/simpleclaw/pkg/agent/protocol"
	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/translate"
)

// ========== Protocol 模块集成方法 ==========

// CreateTask 创建新的协议任务
func (ab *AgentBridge) CreateTask(content string, opts ...protocol.TaskOption) *protocol.Task {
	return protocol.NewTask(content, opts...)
}

// CreateTextTask 创建文本类型任务
func (ab *AgentBridge) CreateTextTask(content string) *protocol.Task {
	return protocol.NewTask(content, protocol.WithTaskType(protocol.TaskTypeText))
}

// CreateImageTask 创建图片类型任务
func (ab *AgentBridge) CreateImageTask(content string, images []string) *protocol.Task {
	return protocol.NewTask(content,
		protocol.WithTaskType(protocol.TaskTypeImage),
		protocol.WithTaskImages(images),
	)
}

// CreateAudioTask 创建音频类型任务
func (ab *AgentBridge) CreateAudioTask(content string, audios []string) *protocol.Task {
	return protocol.NewTask(content,
		protocol.WithTaskType(protocol.TaskTypeAudio),
		protocol.WithTaskAudios(audios),
	)
}

// CreateMixedTask 创建混合类型任务
func (ab *AgentBridge) CreateMixedTask(content string, images, videos, audios, files []string) *protocol.Task {
	return protocol.NewTask(content,
		protocol.WithTaskType(protocol.TaskTypeMixed),
		protocol.WithTaskImages(images),
		protocol.WithTaskVideos(videos),
		protocol.WithTaskAudios(audios),
		protocol.WithTaskFiles(files),
	)
}

// RunTask 使用协议任务执行
func (ab *AgentBridge) RunTask(ctx context.Context, task *protocol.Task, onEvent func(event map[string]any)) (*protocol.AgentResult, error) {
	sessionID := ""
	if task.Metadata != nil {
		if sid, ok := task.Metadata["session_id"].(string); ok {
			sessionID = sid
		}
	}

	ag, err := ab.GetAgent(sessionID)
	if err != nil {
		return protocol.NewErrorResult("Failed to get agent: "+err.Error(), 0), err
	}

	task.UpdateStatus(protocol.TaskStatusProcessing)

	response, err := ag.Run(ctx, task.Content, onEvent)
	if err != nil {
		task.UpdateStatus(protocol.TaskStatusFailed)
		return protocol.NewErrorResult("Agent error: "+err.Error(), 0), err
	}

	task.UpdateStatus(protocol.TaskStatusCompleted)
	return protocol.NewSuccessResult(response, 0), nil
}

// CreateTeamContext 创建团队协作上下文
func (ab *AgentBridge) CreateTeamContext(name, description, rule string, agents []string, maxSteps int) *protocol.TeamContext {
	return protocol.NewTeamContext(name, description, rule, agents, maxSteps)
}

// SanitizeMessages 验证并修复消息列表（使用 protocol 工具函数）
func (ab *AgentBridge) SanitizeMessages(messages *[]protocol.Message) int {
	return protocol.SanitizeMessages(messages)
}

// CreateEvent 创建执行事件
func (ab *AgentBridge) CreateEvent(eventType string, data map[string]any) *protocol.Event {
	return protocol.NewEvent(eventType, data)
}

// CreateTextEvent 创建文本输出事件
func (ab *AgentBridge) CreateTextEvent(text, delta string) *protocol.Event {
	return protocol.NewEvent(protocol.EventTypeText, map[string]any{
		"text":  text,
		"delta": delta,
	})
}

// CreateToolCallEvent 创建工具调用事件
func (ab *AgentBridge) CreateToolCallEvent(toolName, toolCallID string, args map[string]any) *protocol.Event {
	return protocol.NewEvent(protocol.EventTypeToolCall, map[string]any{
		"tool_name":    toolName,
		"tool_call_id": toolCallID,
		"arguments":    args,
	})
}

// CreateToolResultEvent 创建工具结果事件
func (ab *AgentBridge) CreateToolResultEvent(toolCallID, toolName string, result any, status string, execTime float64) *protocol.Event {
	return protocol.NewEvent(protocol.EventTypeToolResult, map[string]any{
		"tool_call_id":   toolCallID,
		"tool_name":      toolName,
		"result":         result,
		"status":         status,
		"execution_time": execTime,
	})
}

// CreateCompleteEvent 创建完成事件
func (ab *AgentBridge) CreateCompleteEvent(finalAnswer string, stepCount int, status string) *protocol.Event {
	return protocol.NewEvent(protocol.EventTypeComplete, map[string]any{
		"final_answer": finalAnswer,
		"step_count":   stepCount,
		"status":       status,
	})
}

// GetProtocol 获取默认协议配置
func (ab *AgentBridge) GetProtocol() *protocol.Protocol {
	return protocol.GetProtocol()
}

// ========== Rate Limiter 集成方法 ==========

// TryAcquireToken 尝试获取速率限制令牌
func (ab *AgentBridge) TryAcquireToken() bool {
	if ab.rateLimiter == nil {
		return true
	}
	return ab.rateLimiter.TryGetToken()
}

// AcquireToken 获取速率限制令牌（阻塞）
func (ab *AgentBridge) AcquireToken(ctx context.Context) bool {
	if ab.rateLimiter == nil {
		return true
	}
	return ab.rateLimiter.GetTokenWithContext(ctx)
}

// GetRateLimiter 获取速率限制器
func (ab *AgentBridge) GetRateLimiter() *common.TokenBucket {
	return ab.rateLimiter
}

// TruncateString 截断字符串到指定长度
func (ab *AgentBridge) TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ========== Memory 模块集成方法 ==========

// GetMemoryManager 获取内存管理器
func (ab *AgentBridge) GetMemoryManager() *memory.Manager {
	return ab.memoryMgr
}

// AddMemory 添加长期记忆
func (ab *AgentBridge) AddMemory(ctx context.Context, content, userID string, scope memory.MemoryScope) error {
	if ab.memoryMgr == nil {
		return nil
	}
	return ab.memoryMgr.AddMemory(ctx, content, userID, scope)
}

// SearchMemory 搜索记忆
func (ab *AgentBridge) SearchMemory(ctx context.Context, query string, limit int) ([]*memory.SearchResult, error) {
	if ab.memoryMgr == nil {
		return nil, nil
	}
	opts := memory.DefaultSearchOptions()
	opts.MaxResults = limit
	return ab.memoryMgr.Search(ctx, query, opts)
}

// GetMemoryStats 获取内存统计信息
func (ab *AgentBridge) GetMemoryStats(ctx context.Context) map[string]any {
	if ab.memoryMgr == nil {
		return nil
	}
	return ab.memoryMgr.GetStats(ctx)
}

// SyncMemory 同步长期记忆
func (ab *AgentBridge) SyncMemory(ctx context.Context, force bool) error {
	if ab.memoryMgr == nil {
		return nil
	}
	return ab.memoryMgr.Sync(ctx, force)
}

// ========== Translate 模块集成方法 ==========

// GetTranslator 获取翻译器实例
func (ab *AgentBridge) GetTranslator() translate.Translator {
	return ab.translator
}

// HasTranslator 检查是否配置了翻译器
func (ab *AgentBridge) HasTranslator() bool {
	return ab.translator != nil
}

// Translate 翻译文本
func (ab *AgentBridge) Translate(text, from, to string) (string, error) {
	if ab.translator == nil {
		return "", nil
	}
	return ab.translator.Translate(text, from, to)
}

// TranslateToChinese 翻译文本到中文
func (ab *AgentBridge) TranslateToChinese(text string) (string, error) {
	return ab.Translate(text, "", "zh")
}

// TranslateToEnglish 翻译文本到英文
func (ab *AgentBridge) TranslateToEnglish(text string) (string, error) {
	return ab.Translate(text, "", "en")
}

// ListTranslators 列出所有已注册的翻译器
func (ab *AgentBridge) ListTranslators() []string {
	return translate.GetRegisteredTranslators()
}

// ========== 缓存方法 ==========

// CacheResponse 缓存响应
func (ab *AgentBridge) CacheResponse(key, response string) {
	if ab.responseCache == nil {
		return
	}
	ab.responseCache.Set(key, response)
}

// GetCachedResponse 获取缓存的响应
func (ab *AgentBridge) GetCachedResponse(key string) (string, bool) {
	if ab.responseCache == nil {
		return "", false
	}
	return ab.responseCache.Get(key)
}

// ClearCache 清除缓存
func (ab *AgentBridge) ClearCache() {
	if ab.responseCache == nil {
		return
	}
	ab.responseCache.Clear()
}

// GetCacheSize 获取缓存大小
func (ab *AgentBridge) GetCacheSize() int {
	if ab.responseCache == nil {
		return 0
	}
	return ab.responseCache.Len()
}

// ========== 时间检查方法 ==========

// IsInServiceTime 检查是否在服务时间内
func (ab *AgentBridge) IsInServiceTime() bool {
	if ab.timeChecker == nil {
		return true
	}
	return ab.timeChecker.IsInServiceTime()
}

// SetServiceTimeRange 设置服务时间范围
func (ab *AgentBridge) SetServiceTimeRange(start, end string) error {
	if ab.timeChecker == nil {
		return nil
	}
	return ab.timeChecker.SetTimeRange(start, end)
}

// GetServiceTimeRange 获取服务时间范围
func (ab *AgentBridge) GetServiceTimeRange() (string, string) {
	if ab.timeChecker == nil {
		return "00:00", "24:00"
	}
	return ab.timeChecker.GetTimeRange()
}

// ========== Embedding 模块集成方法 ==========

// GetEmbedder 获取嵌入器实例
func (ab *AgentBridge) GetEmbedder() memory.Embedder {
	return ab.embedder
}

// HasEmbedder 检查是否配置了嵌入器
func (ab *AgentBridge) HasEmbedder() bool {
	return ab.embedder != nil
}

// Embed 生成文本的嵌入向量
func (ab *AgentBridge) Embed(ctx context.Context, text string) ([]float64, error) {
	if ab.embedder == nil {
		return nil, nil
	}
	return ab.embedder.Embed(ctx, text)
}

// EmbedBatch 批量生成文本的嵌入向量
func (ab *AgentBridge) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if ab.embedder == nil {
		return nil, nil
	}
	return ab.embedder.EmbedBatch(ctx, texts)
}

// GetEmbeddingDimensions 获取嵌入向量维度
func (ab *AgentBridge) GetEmbeddingDimensions() int {
	if ab.embedder == nil {
		return 0
	}
	return ab.embedder.Dimensions()
}

// GetEmbeddingCache 获取嵌入缓存实例
func (ab *AgentBridge) GetEmbeddingCache() *memory.EmbeddingCache {
	return ab.embeddingCache
}

// ClearEmbeddingCache 清空嵌入缓存
func (ab *AgentBridge) ClearEmbeddingCache() {
	if ab.embeddingCache != nil {
		ab.embeddingCache.Clear()
	}
}

// GetEmbeddingCacheSize 获取嵌入缓存大小
func (ab *AgentBridge) GetEmbeddingCacheSize() int {
	if ab.embeddingCache == nil {
		return 0
	}
	return ab.embeddingCache.Size()
}
