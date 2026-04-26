// Package bridge 提供消息处理的核心路由层
// interfaces.go 定义领域接口，遵循接口隔离原则（ISP）
package bridge

import (
	"context"

	"github.com/bstr9/simpleclaw/pkg/agent/memory"
	"github.com/bstr9/simpleclaw/pkg/llm"
	"github.com/bstr9/simpleclaw/pkg/plugin"
	"github.com/bstr9/simpleclaw/pkg/voice"
)

// VoiceProvider 语音引擎接口
type VoiceProvider interface {
	// HasVoiceEngine 检查是否配置了语音引擎
	HasVoiceEngine() bool
	// TextToSpeech 文本转语音
	TextToSpeech(ctx context.Context, text string) ([]byte, error)
	// SpeechToText 语音转文本
	SpeechToText(ctx context.Context, audio []byte) (string, error)
	// ListVoiceEngines 列出所有已注册的语音引擎
	ListVoiceEngines() []voice.EngineType
}

// TranslatorProvider 翻译服务接口
type TranslatorProvider interface {
	// HasTranslator 检查是否配置了翻译器
	HasTranslator() bool
	// Translate 翻译文本
	Translate(text, from, to string) (string, error)
	// ListTranslators 列出所有已注册的翻译器
	ListTranslators() []string
}

// MemoryProvider 记忆管理接口
type MemoryProvider interface {
	// GetMemoryManager 获取内存管理器
	GetMemoryManager() *memory.Manager
	// AddMemory 添加长期记忆
	AddMemory(ctx context.Context, content, userID string, scope memory.MemoryScope) error
	// SearchMemory 搜索记忆
	SearchMemory(ctx context.Context, query string, limit int) ([]*memory.SearchResult, error)
	// GetMemoryStats 获取内存统计信息
	GetMemoryStats(ctx context.Context) map[string]any
}

// PluginProvider 插件管理接口
type PluginProvider interface {
	// ListPlugins 列出所有插件
	ListPlugins() map[string]*plugin.Metadata
}

// EmbeddingProvider 向量嵌入接口
type EmbeddingProvider interface {
	// HasEmbedder 检查是否配置了嵌入器
	HasEmbedder() bool
	// Embed 生成文本的嵌入向量
	Embed(ctx context.Context, text string) ([]float64, error)
	// EmbedBatch 批量生成文本的嵌入向量
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
	// GetEmbeddingDimensions 获取嵌入向量维度
	GetEmbeddingDimensions() int
}

// SessionProvider 会话管理接口
type SessionProvider interface {
	// SessionCount 返回活跃会话数量
	SessionCount() int
	// ClearSession 清除指定会话
	ClearSession(sessionID string)
	// GetSessionHistory 获取会话的消息历史
	GetSessionHistory(sessionID string) []llm.Message
}

// ChannelBridge 渠道桥接器接口
// 组合所有领域接口，供渠道使用
type ChannelBridge interface {
	VoiceProvider
	TranslatorProvider
	MemoryProvider
	PluginProvider
	EmbeddingProvider
	SessionProvider
}
