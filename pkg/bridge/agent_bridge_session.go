// Package bridge 提供消息处理的核心路由层
// agent_bridge_session.go 会话管理相关方法
package bridge

import (
	"context"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/agent/chat"
	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/llm"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

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
	return len(ab.agents) + common.BoolToInt(ab.defaultAgent != nil)
}
