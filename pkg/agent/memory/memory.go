// Package memory 提供 Agent 记忆管理，支持短期和长期存储。
// 支持对话历史持久化、向量语义搜索和记忆摘要功能。
package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// MemoryScope 定义记忆条目的可见性作用域。
type MemoryScope string

const (
	// ScopeShared 表示全局共享记忆，所有 Agent 可访问。
	ScopeShared MemoryScope = "shared"
	// ScopeUser 表示用户专属记忆，仅该用户的会话可访问。
	ScopeUser MemoryScope = "user"
	// ScopeSession 表示会话专属记忆，临时且隔离。
	ScopeSession MemoryScope = "session"
)

// MemorySource 指示记忆内容的来源。
type MemorySource string

const (
	// SourceMemory 表示来自持久化记忆文件的内容。
	SourceMemory MemorySource = "memory"
	// SourceSession 表示来自对话会话的内容。
	SourceSession MemorySource = "session"
)

// Role 表示对话中消息发送者的角色。
type Role string

const (
	// RoleUser 表示用户发送的消息。
	RoleUser Role = "user"
	// RoleAssistant 表示助手发送的消息。
	RoleAssistant Role = "assistant"
	// RoleSystem 表示系统消息。
	RoleSystem Role = "system"
)

// Message 表示对话历史中的单条消息。
type Message struct {
	// ID 是消息的唯一标识符。
	ID string `json:"id"`
	// Role 指示消息发送者。
	Role Role `json:"role"`
	// Content 是消息的文本内容。
	Content string `json:"content"`
	// CreatedAt 是消息创建的时间戳。
	CreatedAt time.Time `json:"created_at"`
	// Metadata 包含可选的附加信息。
	Metadata map[string]any `json:"metadata,omitempty"`
}

// MemoryChunk 表示用于索引的记忆内容分块。
type MemoryChunk struct {
	// ID 是分块的唯一标识符。
	ID string `json:"id"`
	// UserID 是用户作用域记忆的可选用户标识符。
	UserID string `json:"user_id,omitempty"`
	// Scope 定义可见性作用域。
	Scope MemoryScope `json:"scope"`
	// Source 指示内容来源。
	Source MemorySource `json:"source"`
	// Path 是内容存储的文件路径。
	Path string `json:"path"`
	// StartLine 是源文件中的起始行号。
	StartLine int `json:"start_line"`
	// EndLine 是源文件中的结束行号。
	EndLine int `json:"end_line"`
	// Text 是分块的实际内容。
	Text string `json:"text"`
	// Embedding 是文本的向量表示（可选）。
	Embedding []float64 `json:"embedding,omitempty"`
	// Hash 是文本内容的 SHA256 哈希值。
	Hash string `json:"hash"`
	// Metadata 包含可选的附加信息。
	Metadata map[string]any `json:"metadata,omitempty"`
	// CreatedAt 是分块创建的时间戳。
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 是分块最后更新的时间戳。
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchResult 表示记忆搜索操作的结果。
type SearchResult struct {
	// Path 是内容存储的文件路径。
	Path string `json:"path"`
	// StartLine 是起始行号。
	StartLine int `json:"start_line"`
	// EndLine 是结束行号。
	EndLine int `json:"end_line"`
	// Score 是相关性分数（向量搜索为 0-1，关键词搜索为 BM25 排名）。
	Score float64 `json:"score"`
	// Snippet 是内容的截断预览。
	Snippet string `json:"snippet"`
	// Source 指示内容来源。
	Source MemorySource `json:"source"`
	// UserID 是可选的用户标识符。
	UserID string `json:"user_id,omitempty"`
}

// SearchOptions 包含记忆搜索操作的选项。
type SearchOptions struct {
	// UserID 过滤结果为用户作用域记忆。
	UserID string
	// Scopes 限制搜索到特定作用域。
	Scopes []MemoryScope
	// MaxResults 限制结果数量。
	MaxResults int
	// MinScore 设置最低相关性阈值。
	MinScore float64
	// IncludeShared 在结果中包含共享记忆。
	IncludeShared bool
}

// DefaultSearchOptions 返回默认搜索选项。
func DefaultSearchOptions() *SearchOptions {
	return &SearchOptions{
		Scopes:        []MemoryScope{ScopeShared, ScopeUser},
		MaxResults:    10,
		MinScore:      0.1,
		IncludeShared: true,
	}
}

// Config 包含记忆操作的配置。
type Config struct {
	// WorkspaceRoot 是记忆存储的根目录。
	WorkspaceRoot string `json:"workspace_root"`
	// EmbeddingModel 是用于嵌入的模型（如 "text-embedding-3-small"）。
	EmbeddingModel string `json:"embedding_model"`
	// EmbeddingDim 是嵌入向量的维度。
	EmbeddingDim int `json:"embedding_dim"`
	// ChunkMaxTokens 是每个分块的最大 token 数。
	ChunkMaxTokens int `json:"chunk_max_tokens"`
	// ChunkOverlapTokens 是分块之间的重叠 token 数。
	ChunkOverlapTokens int `json:"chunk_overlap_tokens"`
	// MaxResults 是默认的最大搜索结果数。
	MaxResults int `json:"max_results"`
	// MinScore 是默认的最低相关性阈值。
	MinScore float64 `json:"min_score"`
	// VectorWeight 是混合搜索中向量搜索的权重。
	VectorWeight float64 `json:"vector_weight"`
	// KeywordWeight 是混合搜索中关键词搜索的权重。
	KeywordWeight float64 `json:"keyword_weight"`
	// EnableAutoSync 启用自动同步。
	EnableAutoSync bool `json:"enable_auto_sync"`
	// SyncOnSearch 在搜索操作前触发同步。
	SyncOnSearch bool `json:"sync_on_search"`
	// MaxAgeDays 是清理前的消息最大保留天数。
	MaxAgeDays int `json:"max_age_days"`
}

// DefaultConfig 返回默认记忆配置。
func DefaultConfig() *Config {
	return &Config{
		WorkspaceRoot:      "./workspace",
		EmbeddingModel:     "text-embedding-3-small",
		EmbeddingDim:       1536,
		ChunkMaxTokens:     500,
		ChunkOverlapTokens: 50,
		MaxResults:         10,
		MinScore:           0.1,
		VectorWeight:       0.7,
		KeywordWeight:      0.3,
		EnableAutoSync:     true,
		SyncOnSearch:       true,
		MaxAgeDays:         30,
	}
}

// Memory 定义记忆存储操作的接口。
type Memory interface {
	// Add 将新消息存储到记忆中。
	Add(ctx context.Context, msg *Message) error

	// Get 根据查询从记忆中检索消息。
	Get(ctx context.Context, query string, limit int) ([]*Message, error)

	// Clear 清除记忆中的所有消息。
	Clear(ctx context.Context) error

	// Summarize 生成已存储记忆的摘要。
	Summarize(ctx context.Context) (string, error)

	// Close 释放记忆实例持有的资源。
	Close() error
}

// Searcher 定义记忆搜索操作的接口。
type Searcher interface {
	// Search 执行搜索操作并返回匹配结果。
	Search(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error)

	// SearchVector 执行向量相似度搜索。
	SearchVector(ctx context.Context, embedding []float64, opts *SearchOptions) ([]*SearchResult, error)

	// SearchKeyword 执行关键词搜索。
	SearchKeyword(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error)
}

// Embedder 定义嵌入生成的接口。
type Embedder interface {
	// Embed 为给定文本生成嵌入向量。
	Embed(ctx context.Context, text string) ([]float64, error)

	// EmbedBatch 为多个文本生成嵌入。
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)

	// Dimensions 返回嵌入向量的维度。
	Dimensions() int
}

// Summarizer 定义记忆摘要的接口。
type Summarizer interface {
	// Summarize 从给定消息生成摘要。
	Summarize(ctx context.Context, messages []*Message) (string, error)

	// Flush 将摘要内容写入持久化存储。
	Flush(ctx context.Context, messages []*Message, userID string, reason string) error
}

// generateID 基于内容哈希创建唯一 ID。
func generateID(content string) string {
	hash := sha256.Sum256([]byte(content + time.Now().String()))
	return hex.EncodeToString(hash[:])[:16]
}

// ComputeHash 计算内容的 SHA256 哈希值。
func ComputeHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// ToJSON 将任意值转换为 JSON 字符串。
func ToJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

// FromJSON 将 JSON 字符串解析到目标值。
func FromJSON(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}
