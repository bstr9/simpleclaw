// Package bridge 提供消息处理的核心路由层
// agent_bridge.go 集成 Agent 系统与桥接层
package bridge

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/agent/chat"
	"github.com/bstr9/simpleclaw/pkg/agent/memory"
	"github.com/bstr9/simpleclaw/pkg/agent/skills"
	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/extension/registry"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/plugin"
	"github.com/bstr9/simpleclaw/pkg/translate"
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

	// 添加全局 skills 目录 (~/.agents/skills/)，支持 npx skills 安装的技能
	if homeDir, err := os.UserHomeDir(); err == nil {
		globalSkillsDir := filepath.Join(homeDir, ".agents", "skills")
		if _, err := os.Stat(globalSkillsDir); err == nil {
			skillDirs = append(skillDirs, globalSkillsDir)
		}
	}

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
