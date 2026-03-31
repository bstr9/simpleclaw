// Package bridge 提供消息处理的核心路由层
// agent_bridge.go 集成 Agent 系统与桥接层
package bridge

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/agent/chat"
	"github.com/bstr9/simpleclaw/pkg/agent/memory"
	"github.com/bstr9/simpleclaw/pkg/agent/prompt"
	"github.com/bstr9/simpleclaw/pkg/agent/protocol"
	"github.com/bstr9/simpleclaw/pkg/agent/skills"
	"github.com/bstr9/simpleclaw/pkg/agent/tools"
	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/extension/registry"
	"github.com/bstr9/simpleclaw/pkg/llm"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/plugin"
	"github.com/bstr9/simpleclaw/pkg/translate"
	baidutrans "github.com/bstr9/simpleclaw/pkg/translate/baidu"
	"github.com/bstr9/simpleclaw/pkg/types"
	"github.com/bstr9/simpleclaw/pkg/voice"
	"go.uber.org/zap"
)

// AgentBridge 集成 Agent 系统与桥接层
// 管理每个会话的 Agent 实例，实现会话隔离
type AgentBridge struct {
	mu sync.RWMutex

	// bridge 父桥接实例
	bridge *Bridge

	// agents 会话ID到Agent实例的映射
	agents map[string]*agent.Agent

	// defaultAgent 默认Agent，用于向后兼容
	defaultAgent *agent.Agent

	// sessionLocks 会话执行锁，确保同一会话的消息串行处理
	sessionLocks map[string]*sync.Mutex

	// initializer Agent实例创建器
	initializer *AgentInitializer

	// workspaceDir Agent工作空间目录
	workspaceDir string

	// memoryMgr 会话历史记忆管理器
	memoryMgr *memory.Manager

	// skillsRegistry 技能加载和执行注册表
	skillsRegistry *skills.Registry

	// sessionMgr 会话生命周期管理器
	sessionMgr *chat.SessionManager

	// pluginMgr 插件生命周期管理器
	pluginMgr *plugin.Manager

	// voiceEngine 语音引擎（TTS/ASR）
	voiceEngine voice.VoiceEngine

	// rateLimiter 速率限制器
	rateLimiter *common.TokenBucket

	// translator 翻译器
	translator translate.Translator

	// responseCache 响应缓存
	responseCache *common.ExpireMap[string, string]

	// timeChecker 时间检查器
	timeChecker *common.TimeChecker

	// embedder 向量嵌入器
	embedder memory.Embedder

	// embeddingCache 嵌入向量缓存
	embeddingCache *memory.EmbeddingCache
}

// NewAgentBridge 创建新的 AgentBridge 实例
func NewAgentBridge(bridge *Bridge) *AgentBridge {
	cfg := config.Get()
	workspaceDir := cfg.AgentWorkspace
	if workspaceDir == "" {
		workspaceDir = "~/cow"
	}

	// 展开路径
	if len(workspaceDir) > 0 && workspaceDir[0] == '~' {
		home, _ := os.UserHomeDir()
		workspaceDir = filepath.Join(home, workspaceDir[1:])
	}

	ab := &AgentBridge{
		bridge:       bridge,
		agents:       make(map[string]*agent.Agent),
		sessionLocks: make(map[string]*sync.Mutex),
		workspaceDir: workspaceDir,
	}

	// 初始化内存管理器
	memMgr, err := memory.NewManager(
		memory.WithManagerWorkspace(workspaceDir),
	)
	if err != nil {
		logger.Warn("初始化内存管理器失败", zap.Error(err))
	} else {
		ab.memoryMgr = memMgr
	}

	// 初始化技能注册表
	ab.skillsRegistry = skills.NewRegistry()
	skillsDir := filepath.Join(workspaceDir, "skills")
	builtinSkillsDir := filepath.Join(skillsDir, "builtin")
	customSkillsDir := filepath.Join(skillsDir, "custom")

	skillDirs := []string{skillsDir, builtinSkillsDir, customSkillsDir}
	skillDirs = append(skillDirs, registry.GetSkillPaths()...)

	if err := ab.skillsRegistry.LoadFromDir(skillDirs...); err != nil {
		logger.Debug("加载技能失败", zap.Error(err))
	}
	logger.Info("技能加载完成", zap.Int("skills", len(ab.skillsRegistry.ListEnabled())))

	// 初始化会话管理器
	ab.sessionMgr = chat.NewSessionManager(
		chat.WithMaxSessions(1000),
		chat.WithSessionTimeout(30*60*1e9), // 30分钟
	)

	// 初始化插件管理器
	ab.pluginMgr = plugin.GetManager()
	ab.pluginMgr.SetPluginDir(filepath.Join(workspaceDir, "plugins"))

	// 初始化速率限制器
	ab.rateLimiter = common.NewTokenBucket(100, 10)

	// 并行初始化可选组件
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		ab.initVoiceEngine(cfg)
	}()
	go func() {
		defer wg.Done()
		ab.initTranslator(cfg)
	}()
	go func() {
		defer wg.Done()
		ab.initEmbedder(cfg)
	}()

	// 初始化响应缓存（快速操作，无需并行）
	ab.initResponseCache()

	// 等待可选组件初始化完成
	wg.Wait()

	ab.initializer = NewAgentInitializer(ab)

	// 后台预初始化默认 Agent
	go func() {
		if _, err := ab.initDefaultAgent(); err != nil {
			logger.Warn("[AgentBridge] 预初始化默认 Agent 失败", zap.Error(err))
		}
	}()

	return ab
}

func (ab *AgentBridge) initTranslator(cfg *config.Config) {
	translatorType := translate.TranslatorBaidu
	if !translate.IsTranslatorRegistered(translatorType) {
		return
	}

	t, err := translate.CreateTranslator(translatorType)
	if err != nil {
		logger.Warn("初始化翻译器失败", zap.Error(err))
		return
	}

	ab.translator = t
	logger.Info("翻译器初始化成功", zap.String("type", translatorType))
}

func (ab *AgentBridge) initResponseCache() {
	ab.responseCache = common.NewExpireMap[string, string](5 * time.Minute)
	ab.timeChecker = common.NewTimeChecker()
	ab.embeddingCache = memory.NewEmbeddingCache(10000)
}

func (ab *AgentBridge) initEmbedder(cfg *config.Config) {
	if cfg.OpenAIAPIKey == "" {
		return
	}

	embedder, err := memory.CreateEmbeddingProvider(
		"openai",
		"text-embedding-3-small",
		cfg.OpenAIAPIKey,
		cfg.OpenAIAPIBase,
		nil,
	)
	if err != nil {
		logger.Warn("初始化嵌入器失败", zap.Error(err))
		return
	}

	if ab.embeddingCache != nil {
		ab.embedder = memory.NewCachedEmbedder(embedder, ab.embeddingCache, "openai")
	} else {
		ab.embedder = embedder
	}

	logger.Info("嵌入器初始化成功")
}

func (ab *AgentBridge) initVoiceEngine(cfg *config.Config) {
	voiceType := cfg.TextToVoice
	if voiceType == "" && cfg.VoiceToText == "" {
		return
	}

	engineType := voiceType
	if engineType == "" {
		engineType = cfg.VoiceToText
	}

	voiceCfg := voice.Config{
		EngineType: voice.EngineType(engineType),
		VoiceID:    cfg.TTSVoiceID,
		Model:      cfg.TextToVoiceModel,
	}

	engine, err := voice.NewEngine(voiceCfg)
	if err != nil {
		logger.Warn("初始化语音引擎失败", zap.Error(err), zap.String("type", engineType))
		return
	}

	ab.voiceEngine = engine
	logger.Info("语音引擎初始化成功", zap.String("type", engineType))
}

// GetAgent 获取指定会话的 Agent 实例
// 如果 sessionID 为空，返回默认 Agent
// 如果 Agent 不存在，会自动创建
func (ab *AgentBridge) GetAgent(sessionID string) (*agent.Agent, error) {
	ab.mu.RLock()
	if sessionID == "" {
		a := ab.defaultAgent
		ab.mu.RUnlock()
		if a != nil {
			return a, nil
		}
		return ab.initDefaultAgent()
	}

	a, exists := ab.agents[sessionID]
	ab.mu.RUnlock()

	if exists {
		return a, nil
	}

	return ab.initAgentForSession(sessionID)
}

// initDefaultAgent 初始化默认 Agent
func (ab *AgentBridge) initDefaultAgent() (*agent.Agent, error) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if ab.defaultAgent != nil {
		return ab.defaultAgent, nil
	}

	a, err := ab.initializer.Initialize("")
	if err != nil {
		return nil, err
	}

	ab.defaultAgent = a
	logger.Info("[AgentBridge] Default agent initialized")
	return a, nil
}

// getSessionLock 获取会话的执行锁
func (ab *AgentBridge) getSessionLock(sessionID string) *sync.Mutex {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if sessionID == "" {
		sessionID = "_default_"
	}

	lock, exists := ab.sessionLocks[sessionID]
	if !exists {
		lock = &sync.Mutex{}
		ab.sessionLocks[sessionID] = lock
	}
	return lock
}

// initAgentForSession 为指定会话初始化 Agent
func (ab *AgentBridge) initAgentForSession(sessionID string) (*agent.Agent, error) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if a, exists := ab.agents[sessionID]; exists {
		return a, nil
	}

	a, err := ab.initializer.Initialize(sessionID)
	if err != nil {
		return nil, err
	}

	ab.agents[sessionID] = a
	logger.Debug("[AgentBridge] Agent initialized for session",
		zap.String("session_id", sessionID))
	return a, nil
}

// AgentReply 使用 Agent 系统处理查询
func (ab *AgentBridge) AgentReply(ctx context.Context, query string, context *types.Context, onEvent func(event map[string]any)) (*types.Reply, error) {
	sessionID := ab.extractSessionID(context)

	sessionLock := ab.getSessionLock(sessionID)
	sessionLock.Lock()
	defer sessionLock.Unlock()

	a, err := ab.GetAgent(sessionID)
	if err != nil {
		logger.Error("[AgentBridge] Failed to get agent", zap.Error(err))
		return types.NewErrorReply("Failed to initialize agent: " + err.Error()), err
	}

	ab.setAgentToolContext(a, context, sessionID)

	logger.Info("[AgentBridge] Agent mode processing",
		zap.String("query", truncate(query, 50)),
		zap.String("session_id", sessionID))

	a.AddUserMessage(query)

	response, err := a.Run(ctx, query, onEvent)
	if err != nil {
		logger.Error("[AgentBridge] Agent run failed", zap.Error(err))
		return types.NewErrorReply("Agent error: " + err.Error()), err
	}

	ab.persistSessionMessages(ctx, sessionID, query, response)

	return types.NewTextReply(response), nil
}

// extractSessionID 从上下文提取会话 ID
func (ab *AgentBridge) extractSessionID(context *types.Context) string {
	if context == nil {
		return ""
	}
	sessionID, _ := context.GetString("session_id")
	return sessionID
}

// setAgentToolContext 设置 Agent 的工具上下文
func (ab *AgentBridge) setAgentToolContext(a *agent.Agent, context *types.Context, sessionID string) {
	if context == nil {
		return
	}

	toolCtx := &agent.ToolContext{
		SessionID: sessionID,
	}

	ab.populateToolContextFromContext(toolCtx, context)
	a.SetToolContext(toolCtx)
}

// populateToolContextFromContext 从上下文填充工具上下文
func (ab *AgentBridge) populateToolContextFromContext(toolCtx *agent.ToolContext, context *types.Context) {
	if val, ok := context.GetString("user_id"); ok {
		toolCtx.UserID = val
	}
	if val, ok := context.GetString("group_id"); ok {
		toolCtx.GroupID = val
	}
	if val, ok := context.GetBool("is_group"); ok {
		toolCtx.IsGroup = val
	}
	if val, ok := context.GetString("channel_type"); ok {
		toolCtx.ChannelType = val
	}
	if val, ok := context.GetString("receiver"); ok {
		toolCtx.Receiver = val
	}
	if val, ok := context.GetString("receive_id_type"); ok {
		toolCtx.ReceiveIDType = val
	}
}

// persistSessionMessages 持久化会话消息
func (ab *AgentBridge) persistSessionMessages(ctx context.Context, sessionID, query, response string) {
	if ab.memoryMgr == nil || sessionID == "" {
		return
	}

	ab.memoryMgr.AddMessage(ctx, sessionID, &memory.Message{
		Role:    memory.RoleUser,
		Content: query,
	})
	ab.memoryMgr.AddMessage(ctx, sessionID, &memory.Message{
		Role:    memory.RoleAssistant,
		Content: response,
	})
}

// ClearSession 清除指定会话的 Agent
func (ab *AgentBridge) ClearSession(sessionID string) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if agent, exists := ab.agents[sessionID]; exists {
		agent.ClearHistory()
		logger.Info("[AgentBridge] Clearing session", zap.String("session_id", sessionID))
		delete(ab.agents, sessionID)
	}

	if ab.sessionMgr != nil {
		_ = ab.sessionMgr.DeleteSession(sessionID)
	}

	if ab.memoryMgr != nil {
		_ = ab.memoryMgr.ClearSession(context.Background(), sessionID)
	}
}

// GetSessionHistory 获取会话的消息历史
func (ab *AgentBridge) GetSessionHistory(sessionID string) []llm.Message {
	ab.mu.RLock()
	agent, exists := ab.agents[sessionID]
	ab.mu.RUnlock()

	if !exists {
		return nil
	}

	return agent.GetMessages()
}

// TrimSessionHistory 裁剪会话的消息历史
func (ab *AgentBridge) TrimSessionHistory(sessionID string, keepLast int) {
	ab.mu.RLock()
	agent, exists := ab.agents[sessionID]
	ab.mu.RUnlock()

	if !exists {
		return
	}

	agent.TrimHistory(keepLast)
	logger.Info("[AgentBridge] Trimmed session history",
		zap.String("session_id", sessionID),
		zap.Int("keep_last", keepLast))
}

// GetAgentToolRegistry 返回 Agent 的工具注册表
func (ab *AgentBridge) GetAgentToolRegistry(sessionID string) *agent.ToolRegistry {
	ag, err := ab.GetAgent(sessionID)
	if err != nil {
		return nil
	}
	return ag.GetToolRegistry()
}

// GetAgentModel 返回 Agent 使用的模型
func (ab *AgentBridge) GetAgentModel(sessionID string) llm.Model {
	ag, err := ab.GetAgent(sessionID)
	if err != nil {
		return nil
	}
	return ag.GetModel()
}

// GetAgentMaxSteps 返回 Agent 的最大执行步数
func (ab *AgentBridge) GetAgentMaxSteps(sessionID string) int {
	ag, err := ab.GetAgent(sessionID)
	if err != nil {
		return 0
	}
	return ag.GetMaxSteps()
}

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

// GetPluginManager 返回插件管理器实例
func (ab *AgentBridge) GetPluginManager() *plugin.Manager {
	return ab.pluginMgr
}

// RegisterPlugin 向插件管理器注册插件
func (ab *AgentBridge) RegisterPlugin(p plugin.Plugin) error {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.Register(p)
}

// PublishEvent 向所有插件发布事件
func (ab *AgentBridge) PublishEvent(event plugin.Event, ctx *plugin.EventContext) error {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.PublishEvent(event, ctx)
}

// UnregisterPlugin 注销插件
func (ab *AgentBridge) UnregisterPlugin(name string) error {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.Unregister(name)
}

// LoadPlugin 加载插件
func (ab *AgentBridge) LoadPlugin(name string) error {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.LoadPlugin(name)
}

// UnloadPlugin 卸载插件
func (ab *AgentBridge) UnloadPlugin(name string) error {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.UnloadPlugin(name)
}

// ReloadPlugin 重载插件
func (ab *AgentBridge) ReloadPlugin(name string) bool {
	if ab.pluginMgr == nil {
		return false
	}
	return ab.pluginMgr.ReloadPlugin(name)
}

// GetPlugin 获取插件实例
func (ab *AgentBridge) GetPlugin(name string) (plugin.Plugin, bool) {
	if ab.pluginMgr == nil {
		return nil, false
	}
	return ab.pluginMgr.GetPlugin(name)
}

// ListPlugins 列出所有插件
func (ab *AgentBridge) ListPlugins() map[string]*plugin.Metadata {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.ListPlugins()
}

// GetPluginMetadata 获取插件元数据
func (ab *AgentBridge) GetPluginMetadata(name string) (*plugin.Metadata, bool) {
	if ab.pluginMgr == nil {
		return nil, false
	}
	return ab.pluginMgr.GetMetadata(name)
}

// ClearAllSessions 清除所有 Agent 会话
func (ab *AgentBridge) ClearAllSessions() {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	logger.Info("[AgentBridge] Clearing all sessions",
		zap.Int("count", len(ab.agents)))

	ab.agents = make(map[string]*agent.Agent)
	ab.defaultAgent = nil

	// 关闭会话管理器
	if ab.sessionMgr != nil {
		_ = ab.sessionMgr.Close()
	}
}

// GetSessionManager 返回会话管理器实例
func (ab *AgentBridge) GetSessionManager() *chat.SessionManager {
	return ab.sessionMgr
}

// GetOrCreateChatSession 获取或创建聊天会话
func (ab *AgentBridge) GetOrCreateChatSession(sessionID string) (*chat.Session, error) {
	if ab.sessionMgr == nil {
		return nil, nil
	}
	return ab.sessionMgr.GetOrCreateSession(sessionID)
}

// CreateChatService 创建 ChatService 实例用于高级聊天管理
func (ab *AgentBridge) CreateChatService(opts ...chat.ChatOption) *chat.ChatService {
	if ab.sessionMgr == nil {
		return nil
	}

	agentFactory := func(sessionID string) (chat.AgentExecutor, error) {
		return ab.GetAgent(sessionID)
	}

	return chat.NewChatService(ab.sessionMgr, agentFactory, opts...)
}

// SessionCount 返回活跃会话数量
func (ab *AgentBridge) SessionCount() int {
	ab.mu.RLock()
	defer ab.mu.RUnlock()
	return len(ab.agents) + boolToInt(ab.defaultAgent != nil)
}

// ========== Voice 模块集成方法 ==========

// GetVoiceEngine 获取语音引擎实例
func (ab *AgentBridge) GetVoiceEngine() voice.VoiceEngine {
	return ab.voiceEngine
}

// HasVoiceEngine 检查是否配置了语音引擎
func (ab *AgentBridge) HasVoiceEngine() bool {
	return ab.voiceEngine != nil
}

// TextToSpeech 文本转语音
func (ab *AgentBridge) TextToSpeech(ctx context.Context, text string) ([]byte, error) {
	if ab.voiceEngine == nil {
		return nil, nil
	}
	return ab.voiceEngine.TTS(ctx, text)
}

// SpeechToText 语音转文本
func (ab *AgentBridge) SpeechToText(ctx context.Context, audio []byte) (string, error) {
	if ab.voiceEngine == nil {
		return "", nil
	}
	return ab.voiceEngine.ASR(ctx, audio)
}

// ListVoiceEngines 列出所有已注册的语音引擎
func (ab *AgentBridge) ListVoiceEngines() []voice.EngineType {
	return voice.ListEngines()
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

// ========== Common 工具方法 ==========

// MinInt 返回两个整数中的较小值
func (ab *AgentBridge) MinInt(a, b int) int {
	return common.MinInt(a, b)
}

// MaxInt 返回两个整数中的较大值
func (ab *AgentBridge) MaxInt(a, b int) int {
	return common.MaxInt(a, b)
}

// ContainsString 检查字符串切片是否包含指定字符串
func (ab *AgentBridge) ContainsString(slice []string, s string) bool {
	return common.ContainsString(slice, s)
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

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// AgentInitializer 处理 Agent 初始化
type AgentInitializer struct {
	agentBridge *AgentBridge
}

// NewAgentInitializer 创建新的 AgentInitializer
func NewAgentInitializer(ab *AgentBridge) *AgentInitializer {
	return &AgentInitializer{
		agentBridge: ab,
	}
}

// Initialize 创建已初始化的 Agent 实例
func (ai *AgentInitializer) Initialize(sessionID string) (*agent.Agent, error) {
	cfg := config.Get()

	workspaceDir := ai.getWorkspaceDir(cfg)
	toolRegistry := ai.createToolRegistry(workspaceDir)
	systemPrompt := ai.buildSystemPrompt(toolRegistry, workspaceDir)

	model, err := ai.createModel()
	if err != nil {
		return nil, err
	}

	maxSteps := ai.getMaxSteps(cfg)
	a := ai.createAgent(systemPrompt, model, toolRegistry, maxSteps)

	ai.restoreSessionHistory(sessionID, a)

	if sessionID == "" {
		ai.logInitialization(workspaceDir, toolRegistry, maxSteps)
	}

	return a, nil
}

// getWorkspaceDir 获取工作空间目录
func (ai *AgentInitializer) getWorkspaceDir(cfg *config.Config) string {
	workspaceDir := cfg.AgentWorkspace
	if workspaceDir == "" {
		workspaceDir = "~/cow"
	}
	return workspaceDir
}

// createToolRegistry 创建工具注册表
func (ai *AgentInitializer) createToolRegistry(workspaceDir string) *agent.ToolRegistry {
	toolRegistry := agent.NewToolRegistry()
	ai.loadTools(toolRegistry, workspaceDir)
	return toolRegistry
}

// getMaxSteps 获取最大执行步数
func (ai *AgentInitializer) getMaxSteps(cfg *config.Config) int {
	maxSteps := cfg.AgentMaxSteps
	if maxSteps <= 0 {
		return 15
	}
	return maxSteps
}

// createAgent 创建 Agent 实例
func (ai *AgentInitializer) createAgent(systemPrompt string, model llm.Model, toolRegistry *agent.ToolRegistry, maxSteps int) *agent.Agent {
	agentOpts := []agent.Option{
		agent.WithSystemPrompt(systemPrompt),
		agent.WithModel(model),
		agent.WithTools(toolRegistry.GetAll()),
		agent.WithMaxSteps(maxSteps),
	}
	return agent.NewAgent(agentOpts...)
}

// restoreSessionHistory 恢复会话历史
func (ai *AgentInitializer) restoreSessionHistory(sessionID string, a *agent.Agent) {
	if sessionID == "" || ai.agentBridge == nil || ai.agentBridge.memoryMgr == nil {
		return
	}

	ctx := context.Background()
	history, err := ai.agentBridge.memoryMgr.GetSessionMessages(ctx, sessionID, 50)
	if err != nil || len(history) == 0 {
		return
	}

	messages := ai.convertHistoryToMessages(history)
	if len(messages) > 0 {
		a.SetMessages(messages)
		logger.Debug("[AgentInitializer] Restored session history",
			zap.String("session_id", sessionID),
			zap.Int("messages", len(messages)))
	}
}

// convertHistoryToMessages 转换历史记录为消息列表
func (ai *AgentInitializer) convertHistoryToMessages(history []map[string]any) []llm.Message {
	messages := make([]llm.Message, 0, len(history))
	for _, h := range history {
		role, _ := h["role"].(string)
		content, _ := h["content"].(string)
		if role != "" && content != "" {
			messages = append(messages, llm.Message{
				Role:    llm.Role(role),
				Content: content,
			})
		}
	}
	return messages
}

// logInitialization 记录初始化日志
func (ai *AgentInitializer) logInitialization(workspaceDir string, toolRegistry *agent.ToolRegistry, maxSteps int) {
	logger.Info("[AgentInitializer] Agent initialized",
		zap.String("workspace", workspaceDir),
		zap.Int("tools", toolRegistry.Count()),
		zap.Int("max_steps", maxSteps))
}

// loadTools 加载所有可用工具到注册表
func (ai *AgentInitializer) loadTools(toolRegistry *agent.ToolRegistry, workspaceDir string) {
	tools.RegisterBuiltInTools(toolRegistry, tools.WithWorkingDir(workspaceDir))

	for _, tool := range registry.GetTools() {
		toolRegistry.Register(tool)
	}
}

// buildSystemPrompt 构建 Agent 的完整系统提示词
func (ai *AgentInitializer) buildSystemPrompt(toolRegistry *agent.ToolRegistry, workspaceDir string) string {
	toolInfos := ai.getToolInfos(toolRegistry)
	skillsPrompt := ai.buildSkillsPrompt()

	opts := &prompt.BuildOptions{
		Tools:        toolInfos,
		WorkspaceDir: workspaceDir,
		Language:     "zh",
		Runtime:      ai.getRuntimeInfo(workspaceDir),
		SkillsPrompt: skillsPrompt,
	}

	return prompt.BuildSystemPrompt(opts)
}

// buildSkillsPrompt 构建技能列表提示词
func (ai *AgentInitializer) buildSkillsPrompt() string {
	ab := ai.agentBridge
	if ab == nil || ab.skillsRegistry == nil {
		return ""
	}

	entries := ab.skillsRegistry.ListEnabled()
	if len(entries) == 0 {
		return ""
	}

	var lines []string
	for _, entry := range entries {
		skillPath := filepath.Join(entry.SkillInfo.BaseDir, "SKILL.md")
		lines = append(lines, fmt.Sprintf("<skill>\n<name>%s</name>\n<description>%s</description>\n<location>%s</location>\n</skill>",
			entry.Skill.Name(),
			entry.Skill.Description(),
			skillPath,
		))
	}

	return strings.Join(lines, "\n")
}

// getToolInfos 从注册表提取工具信息
func (ai *AgentInitializer) getToolInfos(registry *agent.ToolRegistry) []*prompt.ToolInfo {
	tools := registry.GetAll()
	infos := make([]*prompt.ToolInfo, 0, len(tools))

	for _, t := range tools {
		infos = append(infos, &prompt.ToolInfo{
			Name:        t.Name(),
			Description: t.Description(),
			Summary:     ai.getToolSummary(t.Name()),
		})
	}

	return infos
}

// getToolSummary 返回常用工具的简短摘要
func (ai *AgentInitializer) getToolSummary(name string) string {
	summaries := map[string]string{
		"read":       "读取文件内容",
		"write":      "创建或覆盖文件",
		"edit":       "精确编辑文件",
		"ls":         "列出目录内容",
		"bash":       "执行shell命令",
		"web_search": "网络搜索",
		"web_fetch":  "获取URL内容",
		"browser":    "控制浏览器",
		"memory":     "管理记忆",
		"env_config": "管理API密钥和配置",
		"scheduler":  "管理定时任务",
		"send":       "发送本地文件给用户",
		"time":       "获取当前时间",
		"vision":     "图像识别",
	}

	if s, ok := summaries[name]; ok {
		return s
	}
	return ""
}

// createModel 创建 LLM 模型实例
func (ai *AgentInitializer) createModel() (llm.Model, error) {
	return ai.agentBridge.bridge.GetBot(BotTypeOpenAI)
}

// getRuntimeInfo 返回系统提示词的运行时信息
func (ai *AgentInitializer) getRuntimeInfo(workspaceDir string) *prompt.RuntimeInfo {
	cfg := config.Get()

	modelName := cfg.ModelName
	if modelName == "" {
		modelName = cfg.Model
	}

	return &prompt.RuntimeInfo{
		Model:     modelName,
		Workspace: workspaceDir,
		Channel:   cfg.ChannelType,
	}
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

// ========== Reply 工厂方法 ==========

// NewTextReply 创建文本回复
func (ab *AgentBridge) NewTextReply(content string) *types.Reply {
	return types.NewTextReply(content)
}

// NewErrorReply 创建错误回复
func (ab *AgentBridge) NewErrorReply(content string) *types.Reply {
	return types.NewErrorReply(content)
}

// NewInfoReply 创建信息回复
func (ab *AgentBridge) NewInfoReply(content string) *types.Reply {
	return types.NewInfoReply(content)
}

// NewImageReply 创建图片回复
func (ab *AgentBridge) NewImageReply(path string) *types.Reply {
	return types.NewImageReply(path)
}

// NewImageURLReply 创建图片URL回复
func (ab *AgentBridge) NewImageURLReply(url string) *types.Reply {
	return types.NewImageURLReply(url)
}

// NewVoiceReply 创建语音回复
func (ab *AgentBridge) NewVoiceReply(path string) *types.Reply {
	return types.NewVoiceReply(path)
}

// NewVideoReply 创建视频回复
func (ab *AgentBridge) NewVideoReply(path string) *types.Reply {
	return types.NewVideoReply(path)
}

// NewVideoURLReply 创建视频URL回复
func (ab *AgentBridge) NewVideoURLReply(url string) *types.Reply {
	return types.NewVideoURLReply(url)
}

// NewFileReply 创建文件回复
func (ab *AgentBridge) NewFileReply(path string) *types.Reply {
	return types.NewFileReply(path)
}

// NewCardReply 创建卡片回复
func (ab *AgentBridge) NewCardReply(card any) *types.Reply {
	return types.NewCardReply(card)
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

// ========== 工具函数 ==========

// FileSize 获取文件大小
func (ab *AgentBridge) FileSize(file any) (int64, error) {
	return common.FileSize(file)
}

// SplitStringByUTF8Length 按 UTF-8 字节长度分割字符串
func (ab *AgentBridge) SplitStringByUTF8Length(s string, maxLen, maxSplit int) []string {
	return common.SplitStringByUTF8Length(s, maxLen, maxSplit)
}

// RemoveMarkdownSymbol 移除 Markdown 格式符号
func (ab *AgentBridge) RemoveMarkdownSymbol(text string) string {
	return common.RemoveMarkdownSymbol(text)
}

// UniqueString 去重字符串切片
func (ab *AgentBridge) UniqueString(slice []string) []string {
	return common.UniqueString(slice)
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

// CreateEmbeddingProvider 创建嵌入提供者的工厂方法
func (ab *AgentBridge) CreateEmbeddingProvider(providerType, model, apiKey, apiBase string, extraHeaders map[string]string) (memory.EmbeddingProvider, error) {
	return memory.CreateEmbeddingProvider(providerType, model, apiKey, apiBase, extraHeaders)
}

// NewOpenAIEmbeddingProvider 创建 OpenAI 嵌入提供者
func (ab *AgentBridge) NewOpenAIEmbeddingProvider(model, apiKey string, opts ...memory.OpenAIEmbeddingOption) (*memory.OpenAIEmbeddingProvider, error) {
	return memory.NewOpenAIEmbeddingProvider(model, apiKey, opts...)
}

// NewMockEmbedder 创建模拟嵌入器（用于测试）
func (ab *AgentBridge) NewMockEmbedder(dimensions int) *memory.MockEmbedder {
	return memory.NewMockEmbedder(dimensions)
}

// NewCachedEmbedder 创建带缓存的嵌入器
func (ab *AgentBridge) NewCachedEmbedder(provider memory.EmbeddingProvider, cache *memory.EmbeddingCache, providerName string) *memory.CachedEmbedder {
	return memory.NewCachedEmbedder(provider, cache, providerName)
}

// WithAPIBase 设置 API 基础 URL 的选项函数
func (ab *AgentBridge) WithAPIBase(apiBase string) memory.OpenAIEmbeddingOption {
	return memory.WithAPIBase(apiBase)
}

// WithExtraHeaders 设置额外请求头的选项函数
func (ab *AgentBridge) WithExtraHeaders(headers map[string]string) memory.OpenAIEmbeddingOption {
	return memory.WithExtraHeaders(headers)
}

// WithTimeout 设置超时时间的选项函数
func (ab *AgentBridge) WithTimeout(timeout time.Duration) memory.OpenAIEmbeddingOption {
	return memory.WithTimeout(timeout)
}

// WithHTTPClient 设置自定义 HTTP 客户端的选项函数
func (ab *AgentBridge) WithHTTPClient(client *http.Client) memory.OpenAIEmbeddingOption {
	return memory.WithHTTPClient(client)
}

// ========== Chat 模块集成方法 ==========

// NewContextManager 创建新的上下文管理器
func (ab *AgentBridge) NewContextManager(maxMessages, maxTokens int) *chat.ContextManager {
	return chat.NewContextManager(maxMessages, maxTokens)
}

// NewConversationHistory 创建新的对话历史
func (ab *AgentBridge) NewConversationHistory(maxMessages int) *chat.ConversationHistory {
	return chat.NewConversationHistory(maxMessages)
}

// WithMaxContextTurns 设置最大上下文轮次的选项函数
func (ab *AgentBridge) WithMaxContextTurns(turns int) chat.ChatOption {
	return chat.WithMaxContextTurns(turns)
}

// WithChatTemperature 设置温度参数的选项函数
func (ab *AgentBridge) WithChatTemperature(temp float64) chat.ChatOption {
	return chat.WithTemperature(temp)
}

// WithMaxTokens 设置最大 token 数的选项函数
func (ab *AgentBridge) WithMaxTokens(tokens int) chat.ChatOption {
	return chat.WithMaxTokens(tokens)
}

// WithStream 设置是否流式输出的选项函数
func (ab *AgentBridge) WithStream(stream bool) chat.ChatOption {
	return chat.WithStream(stream)
}

// WithUserID 设置用户 ID 的会话选项
func (ab *AgentBridge) WithUserID(userID string) chat.SessionOption {
	return chat.WithUserID(userID)
}

// WithChannelType 设置渠道类型的会话选项
func (ab *AgentBridge) WithChannelType(channelType string) chat.SessionOption {
	return chat.WithChannelType(channelType)
}

// WithMetadata 设置元数据的会话选项
func (ab *AgentBridge) WithMetadata(metadata map[string]any) chat.SessionOption {
	return chat.WithMetadata(metadata)
}

// GenerateSessionID 生成会话 ID
func (ab *AgentBridge) GenerateSessionID(prefix string) string {
	return chat.GenerateSessionID(prefix)
}

// ========== Memory Storage 集成方法 ==========

// NewSQLiteStorage 创建 SQLite 存储实例
func (ab *AgentBridge) NewSQLiteStorage(path string) (*memory.SQLiteStorage, error) {
	return memory.NewSQLiteStorage(memory.WithStoragePath(path))
}

// WithStoragePath 设置存储路径的选项函数
func (ab *AgentBridge) WithStoragePath(path string) memory.SQLiteStorageOption {
	return memory.WithStoragePath(path)
}

// ========== Memory Summarizer 集成方法 ==========

// NewMemoryFlushManager 创建记忆刷新管理器
func (ab *AgentBridge) NewMemoryFlushManager(workspaceDir string, llmClient memory.LLMClient) (*memory.MemoryFlushManager, error) {
	return memory.NewMemoryFlushManager(workspaceDir, llmClient)
}

// NewTextSummarizer 创建文本摘要器
func (ab *AgentBridge) NewTextSummarizer(llmClient memory.LLMClient) *memory.TextSummarizer {
	return memory.NewTextSummarizer(llmClient)
}

// CreateMemoryFilesIfNeeded 创建记忆文件（如果需要）
func (ab *AgentBridge) CreateMemoryFilesIfNeeded(workspaceDir, userID string) error {
	return memory.CreateMemoryFilesIfNeeded(workspaceDir, userID)
}

// EnsureDailyMemoryFile 确保每日记忆文件存在
func (ab *AgentBridge) EnsureDailyMemoryFile(workspaceDir, userID string) (string, error) {
	return memory.EnsureDailyMemoryFile(workspaceDir, userID)
}

// WithLLMStream 设置是否流式输出的 LLM 选项
func (ab *AgentBridge) WithLLMStream(stream bool) memory.LLMOption {
	return memory.WithLLMStream(stream)
}

// ========== Memory Chunker 集成方法 ==========

// NewTextChunker 创建文本分块器
func (ab *AgentBridge) NewTextChunker(maxTokens, overlapTokens int) *memory.TextChunker {
	return memory.NewTextChunker(maxTokens, overlapTokens)
}

// NewTextChunkerWithOptions 使用选项创建文本分块器
func (ab *AgentBridge) NewTextChunkerWithOptions(opts ...memory.ChunkerOption) *memory.TextChunker {
	return memory.NewTextChunkerWithOptions(opts...)
}

// WithChunkerMaxTokens 设置分块最大 token 数的选项
func (ab *AgentBridge) WithChunkerMaxTokens(maxTokens int) memory.ChunkerOption {
	return memory.WithChunkerMaxTokens(maxTokens)
}

// WithOverlapTokens 设置分块重叠 token 数的选项
func (ab *AgentBridge) WithOverlapTokens(overlapTokens int) memory.ChunkerOption {
	return memory.WithOverlapTokens(overlapTokens)
}

// WithCharsPerToken 设置每 token 字符数的选项
func (ab *AgentBridge) WithCharsPerToken(charsPerToken int) memory.ChunkerOption {
	return memory.WithCharsPerToken(charsPerToken)
}

// ========== Memory Manager 选项集成 ==========

// WithManagerConfigFunc 设置内存管理器配置的选项
func (ab *AgentBridge) WithManagerConfigFunc(cfg *memory.Config) memory.ManagerOption {
	return memory.WithManagerConfig(cfg)
}

// WithManagerEmbedderFunc 设置嵌入器的选项
func (ab *AgentBridge) WithManagerEmbedderFunc(embedder memory.Embedder) memory.ManagerOption {
	return memory.WithManagerEmbedder(embedder)
}

// ========== Prompt 模块集成方法 ==========

// NewBuilder 创建提示词构建器
func (ab *AgentBridge) NewBuilder(workspaceDir, language string) *prompt.Builder {
	return prompt.NewBuilder(workspaceDir, language)
}

// NewTemplate 创建模板
func (ab *AgentBridge) NewTemplate(name, content string, opts ...prompt.TemplateOption) (*prompt.DefaultTemplate, error) {
	return prompt.NewTemplate(name, content, opts...)
}

// NewSimpleTemplate 创建简单模板
func (ab *AgentBridge) NewSimpleTemplate(content string) *prompt.SimpleTemplate {
	return prompt.NewSimpleTemplate(content)
}

// NewTemplateManager 创建模板管理器
func (ab *AgentBridge) NewTemplateManager() *prompt.TemplateManager {
	return prompt.NewTemplateManager()
}

// InitBuiltInTemplates 初始化内置模板
func (ab *AgentBridge) InitBuiltInTemplates(mgr *prompt.TemplateManager) error {
	return prompt.InitBuiltInTemplates(mgr)
}

// ExtractVariables 从模板中提取变量
func (ab *AgentBridge) ExtractVariables(template string) []string {
	return prompt.ExtractVariables(template)
}

// WithPromptDelimiters 设置模板分隔符的选项
func (ab *AgentBridge) WithPromptDelimiters(left, right string) prompt.TemplateOption {
	return prompt.WithDelimiters(left, right)
}

// WithEscapeHTML 设置 HTML 转义的选项
func (ab *AgentBridge) WithEscapeHTML(escape bool) prompt.TemplateOption {
	return prompt.WithEscapeHTML(escape)
}

// WithPromptFuncMap 设置模板函数映射的选项
func (ab *AgentBridge) WithPromptFuncMap(funcMap map[string]any) prompt.TemplateOption {
	return prompt.WithFuncMap(funcMap)
}

// WithBaseTemplates 设置基础模板的选项
func (ab *AgentBridge) WithBaseTemplates(templates ...prompt.Template) prompt.TemplateOption {
	return prompt.WithBaseTemplates(templates...)
}

// ========== Protocol 工厂方法集成 ==========

// NewToolResult 创建成功的工具结果
func (ab *AgentBridge) NewToolResult(toolName string, inputParams map[string]any, output any, executionTime float64) *protocol.ToolResult {
	return protocol.NewToolResult(toolName, inputParams, output, executionTime)
}

// NewErrorToolResult 创建失败的工具结果
func (ab *AgentBridge) NewErrorToolResult(toolName string, inputParams map[string]any, errMsg string, executionTime float64) *protocol.ToolResult {
	return protocol.NewErrorToolResult(toolName, inputParams, errMsg, executionTime)
}

// NewAgentAction 创建新的 Agent 动作
func (ab *AgentBridge) NewAgentAction(agentID, agentName string, actionType protocol.AgentActionType, opts ...protocol.ActionOption) *protocol.AgentAction {
	return protocol.NewAgentAction(agentID, agentName, actionType, opts...)
}

// WithActionContent 设置动作内容的选项
func (ab *AgentBridge) WithActionContent(content string) protocol.ActionOption {
	return protocol.WithContent(content)
}

// WithToolResultOption 设置工具结果的选项
func (ab *AgentBridge) WithToolResultOption(result *protocol.ToolResult) protocol.ActionOption {
	return protocol.WithToolResult(result)
}

// ExtractTextFromContent 从消息内容中提取文本
func (ab *AgentBridge) ExtractTextFromContent(content any) string {
	return protocol.ExtractTextFromContent(content)
}

// CompressTurnToTextOnly 压缩对话轮次为纯文本
func (ab *AgentBridge) CompressTurnToTextOnly(messages []protocol.Message) []protocol.Message {
	return protocol.CompressTurnToTextOnly(messages)
}

// NewAgentOutput 创建 Agent 输出
func (ab *AgentBridge) NewAgentOutput(content string, finishReason string) *protocol.AgentOutput {
	return protocol.NewAgentOutput(content, finishReason)
}

// UnixMilli 返回当前 Unix 毫秒时间戳
func (ab *AgentBridge) UnixMilli() int64 {
	return protocol.UnixMilli()
}

// ========== Common Utils 集成方法 ==========

// GetPathSuffix 获取路径的文件后缀
func (ab *AgentBridge) GetPathSuffix(path string) string {
	return common.GetPathSuffix(path)
}

// ContainsInt 检查整数切片是否包含指定整数
func (ab *AgentBridge) ContainsInt(slice []int, n int) bool {
	return common.ContainsInt(slice, n)
}

// RemoveString 从字符串切片中移除指定字符串
func (ab *AgentBridge) RemoveString(slice []string, s string) []string {
	return common.RemoveString(slice, s)
}

// MinInt64 返回两个 int64 中的较小值
func (ab *AgentBridge) MinInt64(a, b int64) int64 {
	return common.MinInt64(a, b)
}

// MaxInt64 返回两个 int64 中的较大值
func (ab *AgentBridge) MaxInt64(a, b int64) int64 {
	return common.MaxInt64(a, b)
}

// TernaryInt 三元运算符（int 类型）
func (ab *AgentBridge) TernaryInt(condition bool, trueVal, falseVal int) int {
	return common.Ternary(condition, trueVal, falseVal)
}

// TernaryString 三元运算符（string 类型）
func (ab *AgentBridge) TernaryString(condition bool, trueVal, falseVal string) string {
	return common.Ternary(condition, trueVal, falseVal)
}

// DefaultIfEmpty 如果字符串为空则返回默认值
func (ab *AgentBridge) DefaultIfEmpty(s, defaultValue string) string {
	return common.DefaultIfEmpty(s, defaultValue)
}

// IntPtr 返回 int 值的指针
func (ab *AgentBridge) IntPtr(v int) *int {
	return common.Ptr(v)
}

// StringPtr 返回 string 值的指针
func (ab *AgentBridge) StringPtr(v string) *string {
	return common.Ptr(v)
}

// IntValueOrDefault 返回 int 指针的值，如果为 nil 则返回零值
func (ab *AgentBridge) IntValueOrDefault(p *int) int {
	return common.ValueOrDefault(p)
}

// StringValueOrDefault 返回 string 指针的值，如果为 nil 则返回零值
func (ab *AgentBridge) StringValueOrDefault(p *string) string {
	return common.ValueOrDefault(p)
}

// IntValueOr 返回 int 指针的值，如果为 nil 则返回默认值
func (ab *AgentBridge) IntValueOr(p *int, defaultVal int) int {
	return common.ValueOr(p, defaultVal)
}

// StringValueOr 返回 string 指针的值，如果为 nil 则返回默认值
func (ab *AgentBridge) StringValueOr(p *string, defaultVal string) string {
	return common.ValueOr(p, defaultVal)
}

// ========== Agent Tools 选项集成 ==========

// WithBashTimeout 设置 Bash 命令超时时间
func (ab *AgentBridge) WithBashTimeout(timeout time.Duration) tools.BashToolOption {
	return tools.WithBashTimeout(timeout)
}

// WithBashAllowList 设置 Bash 允许的命令列表
func (ab *AgentBridge) WithBashAllowList(cmds []string) tools.BashToolOption {
	return tools.WithBashAllowList(cmds)
}

// WithBashDenyList 设置 Bash 禁止的命令列表
func (ab *AgentBridge) WithBashDenyList(cmds []string) tools.BashToolOption {
	return tools.WithBashDenyList(cmds)
}

// WithBrowserHeadless 设置浏览器无头模式
func (ab *AgentBridge) WithBrowserHeadless(headless bool) tools.BrowserToolOption {
	return tools.WithBrowserHeadless(headless)
}

// WithBrowserTimeout 设置浏览器超时时间
func (ab *AgentBridge) WithBrowserTimeout(timeout int) tools.BrowserToolOption {
	return tools.WithBrowserTimeout(timeout)
}

// WithVisionAPIKey 设置视觉工具 API 密钥
func (ab *AgentBridge) WithVisionAPIKey(key string) tools.VisionToolOption {
	return tools.WithVisionAPIKey(key)
}

// WithVisionAPIBase 设置视觉工具 API 基础 URL
func (ab *AgentBridge) WithVisionAPIBase(base string) tools.VisionToolOption {
	return tools.WithVisionAPIBase(base)
}

// WithVisionModel 设置视觉工具模型
func (ab *AgentBridge) WithVisionModel(model string) tools.VisionToolOption {
	return tools.WithVisionModel(model)
}

// WithVisionTimeout 设置视觉工具超时时间
func (ab *AgentBridge) WithVisionTimeout(timeout time.Duration) tools.VisionToolOption {
	return tools.WithVisionTimeout(timeout)
}

// WithSearchTimeout 设置搜索超时时间
func (ab *AgentBridge) WithSearchTimeout(timeout time.Duration) tools.WebSearchOption {
	return tools.WithSearchTimeout(timeout)
}

// ========== Common 选项集成 ==========

// WithTimeRange 设置时间范围
func (ab *AgentBridge) WithTimeRange(start, end string) common.TimeCheckerOption {
	return common.WithTimeRange(start, end)
}

// WithEnabled 设置是否启用
func (ab *AgentBridge) WithEnabled(enabled bool) common.TimeCheckerOption {
	return common.WithEnabled(enabled)
}

// WithDebugMode 设置调试模式
func (ab *AgentBridge) WithDebugMode(debug bool) common.TimeCheckerOption {
	return common.WithDebugMode(debug)
}

// ParseTimeRange 解析时间范围
func (ab *AgentBridge) ParseTimeRange(s string) (string, string, error) {
	return common.ParseTimeRange(s)
}

// FormatTimeFormat 格式化时间
func (ab *AgentBridge) FormatTimeFormat(t time.Time) string {
	return common.FormatTimeFormat(t)
}

// ParseTimeString 解析时间字符串
func (ab *AgentBridge) ParseTimeString(s string) (time.Time, error) {
	return common.ParseTimeString(s)
}

// NewExpireMapString 创建字符串类型的过期映射
func (ab *AgentBridge) NewExpireMapString(ttl time.Duration) common.ExpireMapString {
	return common.NewExpireMapString(ttl)
}

// NewExpireMapAny 创建任意类型的过期映射
func (ab *AgentBridge) NewExpireMapAny(ttl time.Duration) common.ExpireMapAny {
	return common.NewExpireMapAny(ttl)
}

// ========== Logger 格式化函数集成 ==========

// Debugf 格式化调试日志
func (ab *AgentBridge) Debugf(format string, args ...any) {
	logger.Debugf(format, args...)
}

// Warnf 格式化警告日志
func (ab *AgentBridge) Warnf(format string, args ...any) {
	logger.Warnf(format, args...)
}

// Fatalf 格式化致命错误日志
func (ab *AgentBridge) Fatalf(format string, args ...any) {
	logger.Fatalf(format, args...)
}

// LoggerInit 初始化日志
func (ab *AgentBridge) LoggerInit(debug bool) error {
	return logger.Init(debug)
}

// ========== Types 消息/回复构造器集成 ==========

// NewBaseMessage 创建基础消息
func (ab *AgentBridge) NewBaseMessage(msgID, fromUserID, toUserID, content string) *types.BaseMessage {
	return types.NewBaseMessage(msgID, fromUserID, toUserID, content)
}

// NewGroupMessage 创建群组消息
func (ab *AgentBridge) NewGroupMessage(msgID, fromUserID, toUserID, groupID, content string) *types.BaseMessage {
	return types.NewGroupMessage(msgID, fromUserID, toUserID, groupID, content)
}

// NewTextMessage 创建文本消息
func (ab *AgentBridge) NewTextMessage(msgID, fromUserID, toUserID, content string, opts ...types.MessageOption) *types.BaseMessage {
	return types.NewTextMessage(msgID, fromUserID, toUserID, content, opts...)
}

// NewGroupTextMessage 创建群组文本消息
func (ab *AgentBridge) NewGroupTextMessage(msgID, fromUserID, toUserID, groupID, content string, opts ...types.MessageOption) *types.BaseMessage {
	return types.NewGroupTextMessage(msgID, fromUserID, toUserID, groupID, content, opts...)
}

// NewReply 创建回复
func (ab *AgentBridge) NewReply(replyType types.ReplyType, content any) *types.Reply {
	return types.NewReply(replyType, content)
}

// NewInviteRoomReply 创建邀请进群回复
func (ab *AgentBridge) NewInviteRoomReply(roomID string) *types.Reply {
	return types.NewInviteRoomReply(roomID)
}

// NewMiniAppReply 创建小程序回复
func (ab *AgentBridge) NewMiniAppReply(miniAppInfo any) *types.Reply {
	return types.NewMiniAppReply(miniAppInfo)
}

// ========== Translate 缓存和翻译器集成 ==========

// GetCachedTranslator 获取缓存的翻译器
func (ab *AgentBridge) GetCachedTranslator(name string) translate.Translator {
	return translate.GetCachedTranslator(name)
}

// SetCachedTranslator 设置缓存的翻译器
func (ab *AgentBridge) SetCachedTranslator(name string, t translate.Translator) {
	translate.SetCachedTranslator(name, t)
}

// ClearTranslatorCache 清除翻译器缓存
func (ab *AgentBridge) ClearTranslatorCache(name string) {
	translate.ClearTranslatorCache(name)
}

// ClearAllTranslatorCache 清除所有翻译器缓存
func (ab *AgentBridge) ClearAllTranslatorCache() {
	translate.ClearAllTranslatorCache()
}

// NewTranslatorBuilder 创建翻译器构建器
func (ab *AgentBridge) NewTranslatorBuilder(name string) *translate.TranslatorBuilder {
	return translate.NewTranslatorBuilder(name)
}

// ========== Protocol 更多选项集成 ==========

// WithThought 设置思考内容的选项
func (ab *AgentBridge) WithThought(thought string) protocol.ActionOption {
	return protocol.WithThought(thought)
}

// NewAgentResult 创建 Agent 结果
func (ab *AgentBridge) NewAgentResult(finalAnswer string, steps int) *protocol.AgentResult {
	return protocol.NewAgentResult(finalAnswer, steps)
}

// WithTaskStatus 设置任务状态的选项
func (ab *AgentBridge) WithTaskStatus(status protocol.TaskStatus) protocol.TaskOption {
	return protocol.WithTaskStatus(status)
}

// WithTaskMetadata 设置任务元数据的选项
func (ab *AgentBridge) WithTaskMetadata(metadata map[string]any) protocol.TaskOption {
	return protocol.WithTaskMetadata(metadata)
}

// WithTaskID 设置任务 ID 的选项
func (ab *AgentBridge) WithTaskID(id string) protocol.TaskOption {
	return protocol.WithTaskID(id)
}

// ========== Voice Convert 集成 ==========

// NewAudioConverter 创建音频转换器
func (ab *AgentBridge) NewAudioConverter() *voice.AudioConverter {
	return voice.NewAudioConverter()
}

// DefaultConvertOptions 返回默认音频转换选项
func (ab *AgentBridge) DefaultConvertOptions() voice.ConvertOptions {
	return voice.DefaultConvertOptions()
}

// FindClosestSilkRate 查找最接近的 SILK 采样率
func (ab *AgentBridge) FindClosestSilkRate(rate int) int {
	return voice.FindClosestSilkRate(rate)
}

// GetWAVInfo 获取 WAV 文件信息
func (ab *AgentBridge) GetWAVInfo(data []byte) (sampleRate, channels, bits int, err error) {
	return voice.GetWAVInfo(data)
}

// ResampleSimple 简单重采样
func (ab *AgentBridge) ResampleSimple(data []byte, opts voice.ResampleOptions) ([]byte, error) {
	return voice.ResampleSimple(data, opts)
}

// SplitAudio 分割音频
func (ab *AgentBridge) SplitAudio(data []byte, chunkSize int) ([][]byte, error) {
	return voice.SplitAudio(data, chunkSize)
}

// ReadPCMFromReader 从读取器读取 PCM 数据
func (ab *AgentBridge) ReadPCMFromReader() ([]byte, error) {
	return voice.ReadPCMFromReader(nil)
}

// WritePCMToWriter 将 PCM 数据写入写入器
func (ab *AgentBridge) WritePCMToWriter(data []byte) error {
	return voice.WritePCMToWriter(nil, data)
}

// ValidatePCMData 验证 PCM 数据
func (ab *AgentBridge) ValidatePCMData(data []byte, bitDepth int) error {
	return voice.ValidatePCMData(data, bitDepth)
}

// ========== Types Message 选项集成 ==========

// WithMsgType 设置消息类型选项
func (ab *AgentBridge) WithMsgType(msgType int) types.MessageOption {
	return types.WithMsgType(msgType)
}

// WithCreateTime 设置创建时间选项
func (ab *AgentBridge) WithCreateTime(t time.Time) types.MessageOption {
	return types.WithCreateTime(t)
}

// WithContext 设置上下文选项
func (ab *AgentBridge) WithContext(ctx *types.Context) types.MessageOption {
	return types.WithContext(ctx)
}

// ========== Agent 内部方法集成 ==========

// WithSessionStore 设置会话存储的选项
func (ab *AgentBridge) WithSessionStore(store chat.SessionStore) chat.SessionManagerOption {
	return chat.WithSessionStore(store)
}

// WithCleanupInterval 设置清理间隔的选项
func (ab *AgentBridge) WithCleanupInterval(interval time.Duration) chat.SessionManagerOption {
	return chat.WithCleanupInterval(interval)
}

// WithSTMUserID 设置短期记忆用户 ID 的选项
func (ab *AgentBridge) WithSTMUserID(userID string) memory.ShortTermOption {
	return memory.WithSTMUserID(userID)
}

// WithSessionIDOpt 设置短期记忆会话 ID 的选项
func (ab *AgentBridge) WithSessionIDOpt(sessionID string) memory.ShortTermOption {
	return memory.WithSessionID(sessionID)
}

// WithOnEvict 设置短期记忆驱逐回调的选项
func (ab *AgentBridge) WithOnEvict(onEvict func([]*memory.Message)) memory.ShortTermOption {
	return memory.WithOnEvict(onEvict)
}

// WithConfigConfig 设置长期记忆配置的选项
func (ab *AgentBridge) WithConfigConfig(cfg *memory.Config) memory.LongTermOption {
	return memory.WithConfigConfig(cfg)
}

// DefaultMemoryStorageConfig 返回默认内存存储配置
func (ab *AgentBridge) DefaultMemoryStorageConfig() *memory.MemoryStorageConfig {
	return memory.DefaultMemoryStorageConfig()
}

// ========== Bridge Singleton 集成 ==========

// ResetBridgeInstance 重置桥接实例
func (ab *AgentBridge) ResetBridgeInstance() {
	ResetBridge()
}

// ========== Agent 日志方法集成 ==========

// LogDebug 记录调试日志
func (ab *AgentBridge) LogDebug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

// LogInfo 记录信息日志
func (ab *AgentBridge) LogInfo(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

// LogError 记录错误日志
func (ab *AgentBridge) LogError(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

// ========== Plugin BasePlugin 方法集成 ==========

// SetPluginMetadata 设置插件元数据
func (ab *AgentBridge) SetPluginMetadata(p plugin.Plugin, meta *plugin.Metadata) {
	if bp, ok := p.(*plugin.BasePlugin); ok {
		bp.SetMetadata(meta)
	}
}

// LoadPluginConfig 加载插件配置
func (ab *AgentBridge) LoadPluginConfig(p plugin.Plugin, globalConfigDir string) (map[string]any, error) {
	if bp, ok := p.(*plugin.BasePlugin); ok {
		return bp.LoadConfig(globalConfigDir)
	}
	return nil, nil
}

// SavePluginConfig 保存插件配置
func (ab *AgentBridge) SavePluginConfig(p plugin.Plugin, globalConfigDir string) error {
	if bp, ok := p.(*plugin.BasePlugin); ok {
		return bp.SaveConfig(globalConfigDir)
	}
	return nil
}

// ========== Voice Xunfei 工具函数集成 ==========

// Int16ToBytes 将 int16 转换为字节
func (ab *AgentBridge) Int16ToBytes(n int16) []byte {
	return []byte{byte(n), byte(n >> 8)}
}

// BytesToInt16 将字节转换为 int16
func (ab *AgentBridge) BytesToInt16(b []byte) int16 {
	if len(b) < 2 {
		return 0
	}
	return int16(b[0]) | int16(b[1])<<8
}

// ========== LLM 工厂方法集成 ==========

// NewModelWithProvider 使用指定提供商创建模型
func (ab *AgentBridge) NewModelWithProvider(provider string, cfg llm.ModelConfig) (llm.Model, error) {
	return llm.NewModelWithProvider(provider, cfg)
}

// RegisterLLMProvider 注册自定义提供商
func (ab *AgentBridge) RegisterLLMProvider(name, baseURL string) {
	llm.RegisterProvider(name, baseURL)
}

// GetProviderBaseURL 获取提供商基础 URL
func (ab *AgentBridge) GetProviderBaseURL(provider string) string {
	return llm.GetProviderBaseURL(provider)
}

// ListLLMProviders 列出所有提供商
func (ab *AgentBridge) ListLLMProviders() []string {
	return llm.ListProviders()
}

// GetDashScopeModelInfo 获取 DashScope 模型信息
func (ab *AgentBridge) GetDashScopeModelInfo(model string) *llm.ModelInfo {
	return llm.GetDashScopeModelInfo(model)
}

// ListDashScopeModels 列出 DashScope 模型
func (ab *AgentBridge) ListDashScopeModels() []llm.ModelInfo {
	return llm.ListDashScopeModels()
}

// GetLinkAIModelInfo 获取 LinkAI 模型信息
func (ab *AgentBridge) GetLinkAIModelInfo(model string) *llm.ModelInfo {
	return llm.GetLinkAIModelInfo(model)
}

// ListLinkAIModels 列出 LinkAI 模型
func (ab *AgentBridge) ListLinkAIModels() []llm.ModelInfo {
	return llm.ListLinkAIModels()
}

// GetModelScopeModelInfo 获取 ModelScope 模型信息
func (ab *AgentBridge) GetModelScopeModelInfo(model string) *llm.ModelInfo {
	return llm.GetModelScopeModelInfo(model)
}

// ListModelScopeModels 列出 ModelScope 模型
func (ab *AgentBridge) ListModelScopeModels() []llm.ModelInfo {
	return llm.ListModelScopeModels()
}

// GetQwenModelInfo 获取 Qwen 模型信息
func (ab *AgentBridge) GetQwenModelInfo(model string) *llm.ModelInfo {
	return llm.GetQwenModelInfo(model)
}

// ListQwenModels 列出 Qwen 模型
func (ab *AgentBridge) ListQwenModels() []llm.ModelInfo {
	return llm.ListQwenModels()
}

// ListZhipuModels 列出智谱模型
func (ab *AgentBridge) ListZhipuModels() []llm.ModelInfo {
	return llm.ListZhipuModels()
}

// ========== Translate Baidu 集成 ==========

// NewBaiduTranslator 创建百度翻译器
func (ab *AgentBridge) NewBaiduTranslator(appID, appKey string) (translate.Translator, error) {
	return baidutrans.NewBaiduTranslator(baidutrans.Config{
		AppID:  appID,
		AppKey: appKey,
	})
}
