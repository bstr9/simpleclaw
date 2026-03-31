package bridge

import (
	"context"
	"strings"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/llm"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
)

type BotType string

const (
	BotTypeOpenAI   BotType = "openai"
	BotTypeClaude   BotType = "claude"
	BotTypeGemini   BotType = "gemini"
	BotTypeDeepSeek BotType = "deepseek"
	BotTypeGLM      BotType = "glm"
	BotTypeQwen     BotType = "qwen"
	BotTypeMinimax  BotType = "minimax"
	BotTypeMoonshot BotType = "moonshot"
	BotTypeLinkAI   BotType = "linkai"
	BotTypeBaidu    BotType = "baidu"
	BotTypeXunfei   BotType = "xunfei"
)

type Bridge struct {
	mu            sync.RWMutex
	botTypes      map[string]BotType
	bots          map[BotType]llm.Model
	cfg           *config.Config
	agentBridge   *AgentBridge
	agentBridgeMu sync.Once
}

func newBridge() *Bridge {
	b := &Bridge{
		botTypes: make(map[string]BotType),
		bots:     make(map[BotType]llm.Model),
		cfg:      config.Get(),
	}
	b.initBotTypes()
	return b
}

func (b *Bridge) initBotTypes() {
	b.botTypes = map[string]BotType{
		"text-davinci-003": BotTypeOpenAI,
	}

	model := strings.ToLower(b.cfg.Model)

	switch {
	case b.cfg.UseLinkAI && b.cfg.LinkAIAPIKey != "":
		b.botTypes["chat"] = BotTypeLinkAI
	case b.cfg.BotType != "":
		b.botTypes["chat"] = BotType(b.cfg.BotType)
	case strings.HasPrefix(model, "claude"):
		b.botTypes["chat"] = BotTypeClaude
	case strings.HasPrefix(model, "gemini"):
		b.botTypes["chat"] = BotTypeGemini
	case strings.HasPrefix(model, "deepseek"):
		b.botTypes["chat"] = BotTypeDeepSeek
	case strings.HasPrefix(model, "glm"):
		b.botTypes["chat"] = BotTypeGLM
	case strings.HasPrefix(model, "qwen") || strings.HasPrefix(model, "qwq") || strings.HasPrefix(model, "qvq"):
		b.botTypes["chat"] = BotTypeQwen
	case strings.HasPrefix(model, "minimax") || model == "abab6.5-chat":
		b.botTypes["chat"] = BotTypeMinimax
	case strings.HasPrefix(model, "moonshot") || strings.HasPrefix(model, "kimi"):
		b.botTypes["chat"] = BotTypeMoonshot
	case model == "wenxin" || model == "wenxin-4":
		b.botTypes["chat"] = BotTypeBaidu
	case model == "xunfei":
		b.botTypes["chat"] = BotTypeXunfei
	default:
		b.botTypes["chat"] = BotTypeOpenAI
	}
}

func (b *Bridge) GetBotType(typename string) BotType {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.botTypes[typename]
}

func (b *Bridge) FetchReplyContent(query string, ctx *types.Context) (*types.Reply, error) {
	if b.cfg.Agent {
		return b.FetchAgentReply(query, ctx, nil)
	}

	botType := b.GetBotType("chat")
	model, err := b.GetBot(botType)
	if err != nil {
		logger.Errorf("Failed to get bot: %v", err)
		return types.NewErrorReply(err.Error()), err
	}

	messages := []llm.Message{
		{Role: llm.RoleUser, Content: query},
	}

	resp, err := model.Call(context.Background(), messages)
	if err != nil {
		return types.NewErrorReply(err.Error()), err
	}
	return types.NewTextReply(resp.Content), nil
}

func (b *Bridge) FetchAgentReply(query string, ctx *types.Context, onEvent func(event map[string]any)) (*types.Reply, error) {
	logger.Infof("[Bridge] Agent mode: %s", truncate(query, 50))

	b.agentBridgeMu.Do(func() {
		b.agentBridge = NewAgentBridge(b)
	})

	if b.agentBridge == nil {
		return types.NewErrorReply("agent bridge not initialized"), nil
	}

	return b.agentBridge.AgentReply(context.Background(), query, ctx, onEvent)
}

func (b *Bridge) GetBot(botType BotType) (llm.Model, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if model, ok := b.bots[botType]; ok {
		return model, nil
	}

	model, err := b.createBot(botType)
	if err != nil {
		return nil, err
	}

	b.bots[botType] = model
	return model, nil
}

func (b *Bridge) createBot(botType BotType) (llm.Model, error) {
	modelName := b.cfg.ModelName
	if modelName == "" {
		modelName = b.cfg.Model
	}

	cfg := llm.ModelConfig{
		Model:     modelName,
		ModelName: modelName,
	}

	switch botType {
	case BotTypeClaude:
		cfg.APIKey = b.cfg.ClaudeAPIKey
		cfg.APIBase = b.cfg.ClaudeAPIBase
		cfg.Provider = llm.ProviderAnthropic
	case BotTypeGemini:
		cfg.APIKey = b.cfg.GeminiAPIKey
		cfg.APIBase = b.cfg.GeminiAPIBase
		cfg.Provider = llm.ProviderGemini
	case BotTypeGLM:
		cfg.APIKey = b.cfg.ZhipuAIAPIKey
		cfg.APIBase = b.cfg.ZhipuAIAPIBase
		cfg.Provider = llm.ProviderZhipu
	case BotTypeMinimax:
		cfg.APIKey = b.cfg.MinimaxAPIKey
		cfg.APIBase = b.cfg.MinimaxBaseURL
		cfg.Provider = llm.ProviderMiniMax
	case BotTypeLinkAI:
		cfg.APIKey = b.cfg.LinkAIAPIKey
		cfg.APIBase = b.cfg.LinkAIAPIBase
		cfg.Provider = llm.ProviderLinkAI
	case BotTypeXunfei:
		cfg.Extra = map[string]any{
			"app_id":     b.cfg.XunfeiAppID,
			"api_key":    b.cfg.XunfeiAPIKey,
			"api_secret": b.cfg.XunfeiAPISecret,
		}
		cfg.Provider = llm.ProviderXunfei
	case BotTypeDeepSeek:
		cfg.APIKey = b.cfg.OpenAIAPIKey
		cfg.APIBase = b.cfg.OpenAIAPIBase
		cfg.Provider = llm.ProviderDeepSeek
	case BotTypeMoonshot:
		cfg.APIKey = b.cfg.OpenAIAPIKey
		cfg.APIBase = b.cfg.OpenAIAPIBase
		cfg.Provider = llm.ProviderMoonshot
	case BotTypeQwen:
		cfg.APIKey = b.cfg.OpenAIAPIKey
		cfg.APIBase = b.cfg.OpenAIAPIBase
		cfg.Provider = llm.ProviderQwen
	default:
		cfg.APIKey = b.cfg.OpenAIAPIKey
		cfg.APIBase = b.cfg.OpenAIAPIBase
		cfg.Provider = llm.ProviderOpenAI
	}

	return llm.NewModel(cfg)
}

func (b *Bridge) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.bots = make(map[BotType]llm.Model)
	b.initBotTypes()
}

// GetAgentBridge 获取 Agent 桥接器实例
func (b *Bridge) GetAgentBridge() *AgentBridge {
	b.agentBridgeMu.Do(func() {
		b.agentBridge = NewAgentBridge(b)
	})
	return b.agentBridge
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
