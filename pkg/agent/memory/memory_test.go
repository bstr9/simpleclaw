package memory

import (
	"context"
	"testing"
	"time"
)

func newTestMessage(role Role, content string) *Message {
	return &Message{
		ID:        generateID(content),
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]any),
	}
}

func TestMemoryScope(t *testing.T) {
	tests := []struct {
		scope    MemoryScope
		expected string
	}{
		{ScopeShared, "shared"},
		{ScopeUser, "user"},
		{ScopeSession, "session"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.scope) != tt.expected {
				t.Errorf("期望 %s，实际为 %s", tt.expected, tt.scope)
			}
		})
	}
}

func TestMemorySource(t *testing.T) {
	tests := []struct {
		source   MemorySource
		expected string
	}{
		{SourceMemory, "memory"},
		{SourceSession, "session"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.source) != tt.expected {
				t.Errorf("期望 %s，实际为 %s", tt.expected, tt.source)
			}
		})
	}
}

func TestRole(t *testing.T) {
	tests := []struct {
		role     Role
		expected string
	}{
		{RoleUser, "user"},
		{RoleAssistant, "assistant"},
		{RoleSystem, "system"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.role) != tt.expected {
				t.Errorf("期望 %s，实际为 %s", tt.expected, tt.role)
			}
		})
	}
}

func TestMessage(t *testing.T) {
	tests := []struct {
		name    string
		role    Role
		content string
	}{
		{"用户消息", RoleUser, "你好"},
		{"助手消息", RoleAssistant, "你好，有什么可以帮助你的？"},
		{"系统消息", RoleSystem, "你是一个助手"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := newTestMessage(tt.role, tt.content)

			if msg.Role != tt.role {
				t.Errorf("期望角色为 %s，实际为 %s", tt.role, msg.Role)
			}
			if msg.Content != tt.content {
				t.Errorf("期望内容为 %s，实际为 %s", tt.content, msg.Content)
			}
			if msg.ID == "" {
				t.Error("期望 ID 不为空")
			}
			if msg.CreatedAt.IsZero() {
				t.Error("期望 CreatedAt 不为零值")
			}
			if msg.Metadata == nil {
				t.Error("期望 Metadata 初始化为空 map")
			}
		})
	}
}

func TestMessageMetadata(t *testing.T) {
	msg := newTestMessage(RoleUser, "测试消息")

	msg.Metadata["key"] = "value"
	msg.Metadata["number"] = 42

	if msg.Metadata["key"] != "value" {
		t.Error("期望设置 key 为 value")
	}
	if msg.Metadata["number"] != 42 {
		t.Error("期望设置 number 为 42")
	}
}

func TestMemoryChunk(t *testing.T) {
	chunk := &MemoryChunk{
		ID:        "chunk-001",
		UserID:    "user-001",
		Scope:     ScopeUser,
		Source:    SourceSession,
		Path:      "/path/to/file.md",
		StartLine: 1,
		EndLine:   10,
		Text:      "这是测试文本内容",
		Hash:      ComputeHash("这是测试文本内容"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if chunk.ID != "chunk-001" {
		t.Errorf("期望 ID 为 chunk-001")
	}
	if chunk.Scope != ScopeUser {
		t.Errorf("期望 Scope 为 ScopeUser")
	}
	if chunk.Hash == "" {
		t.Error("期望 Hash 不为空")
	}
}

func TestSearchResult(t *testing.T) {
	result := &SearchResult{
		Path:      "/path/to/file.md",
		StartLine: 1,
		EndLine:   10,
		Score:     0.95,
		Snippet:   "匹配的文本片段...",
		Source:    SourceMemory,
		UserID:    "user-001",
	}

	if result.Score < 0 || result.Score > 1 {
		t.Errorf("Score 应该在 0-1 之间，实际为 %f", result.Score)
	}
	if result.Path == "" {
		t.Error("期望 Path 不为空")
	}
}

func TestSearchOptions(t *testing.T) {
	tests := []struct {
		name  string
		opts  *SearchOptions
		check func(t *testing.T, opts *SearchOptions)
	}{
		{
			name: "默认选项",
			opts: DefaultSearchOptions(),
			check: func(t *testing.T, opts *SearchOptions) {
				if opts.MaxResults != 10 {
					t.Errorf("期望 MaxResults 为 10，实际为 %d", opts.MaxResults)
				}
				if opts.MinScore != 0.1 {
					t.Errorf("期望 MinScore 为 0.1，实际为 %f", opts.MinScore)
				}
				if !opts.IncludeShared {
					t.Error("期望 IncludeShared 为 true")
				}
			},
		},
		{
			name: "自定义选项",
			opts: &SearchOptions{
				UserID:        "user-001",
				Scopes:        []MemoryScope{ScopeUser},
				MaxResults:    20,
				MinScore:      0.5,
				IncludeShared: false,
			},
			check: func(t *testing.T, opts *SearchOptions) {
				if opts.UserID != "user-001" {
					t.Errorf("期望 UserID 为 user-001")
				}
				if opts.MaxResults != 20 {
					t.Errorf("期望 MaxResults 为 20")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, tt.opts)
		})
	}
}

func TestConfig(t *testing.T) {
	tests := []struct {
		name  string
		cfg   *Config
		check func(t *testing.T, cfg *Config)
	}{
		{
			name: "默认配置",
			cfg:  DefaultConfig(),
			check: func(t *testing.T, cfg *Config) {
				if cfg.WorkspaceRoot != "./workspace" {
					t.Errorf("期望 WorkspaceRoot 为 ./workspace")
				}
				if cfg.EmbeddingDim != 1536 {
					t.Errorf("期望 EmbeddingDim 为 1536")
				}
				if cfg.ChunkMaxTokens != 500 {
					t.Errorf("期望 ChunkMaxTokens 为 500")
				}
				if cfg.MaxResults != 10 {
					t.Errorf("期望 MaxResults 为 10")
				}
				if cfg.EnableAutoSync != true {
					t.Error("期望 EnableAutoSync 为 true")
				}
			},
		},
		{
			name: "自定义配置",
			cfg: &Config{
				WorkspaceRoot:      "/custom/workspace",
				EmbeddingModel:     "custom-model",
				EmbeddingDim:       768,
				ChunkMaxTokens:     1000,
				ChunkOverlapTokens: 100,
				MaxResults:         20,
				MinScore:           0.2,
				VectorWeight:       0.8,
				KeywordWeight:      0.2,
			},
			check: func(t *testing.T, cfg *Config) {
				if cfg.WorkspaceRoot != "/custom/workspace" {
					t.Errorf("期望 WorkspaceRoot 为 /custom/workspace")
				}
				if cfg.EmbeddingDim != 768 {
					t.Errorf("期望 EmbeddingDim 为 768")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, tt.cfg)
		})
	}
}

type mockMemory struct {
	messages []*Message
}

func (m *mockMemory) Add(ctx context.Context, msg *Message) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockMemory) Get(ctx context.Context, query string, limit int) ([]*Message, error) {
	if limit > len(m.messages) {
		limit = len(m.messages)
	}
	return m.messages[:limit], nil
}

func (m *mockMemory) Clear(ctx context.Context) error {
	m.messages = nil
	return nil
}

func (m *mockMemory) Summarize(ctx context.Context) (string, error) {
	return "摘要内容", nil
}

func (m *mockMemory) Close() error {
	return nil
}

func TestMemoryInterface(t *testing.T) {
	ctx := context.Background()
	mem := &mockMemory{messages: make([]*Message, 0)}

	msg1 := newTestMessage(RoleUser, "第一条消息")
	msg2 := newTestMessage(RoleAssistant, "第二条消息")

	if err := mem.Add(ctx, msg1); err != nil {
		t.Errorf("Add 失败: %v", err)
	}
	if err := mem.Add(ctx, msg2); err != nil {
		t.Errorf("Add 失败: %v", err)
	}

	msgs, err := mem.Get(ctx, "查询", 10)
	if err != nil {
		t.Errorf("Get 失败: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("期望 2 条消息，实际为 %d", len(msgs))
	}

	summary, err := mem.Summarize(ctx)
	if err != nil {
		t.Errorf("Summarize 失败: %v", err)
	}
	if summary != "摘要内容" {
		t.Errorf("期望摘要为 '摘要内容'，实际为 '%s'", summary)
	}

	if err := mem.Clear(ctx); err != nil {
		t.Errorf("Clear 失败: %v", err)
	}
	if len(mem.messages) != 0 {
		t.Error("期望 Clear 后消息为空")
	}

	if err := mem.Close(); err != nil {
		t.Errorf("Close 失败: %v", err)
	}
}

type mockSearcher struct{}

func (s *mockSearcher) Search(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error) {
	return []*SearchResult{
		{Path: "/path/1", Score: 0.9, Snippet: "结果1"},
		{Path: "/path/2", Score: 0.7, Snippet: "结果2"},
	}, nil
}

func (s *mockSearcher) SearchVector(ctx context.Context, embedding []float64, opts *SearchOptions) ([]*SearchResult, error) {
	return []*SearchResult{
		{Path: "/path/1", Score: 0.95},
	}, nil
}

func (s *mockSearcher) SearchKeyword(ctx context.Context, query string, opts *SearchOptions) ([]*SearchResult, error) {
	return []*SearchResult{
		{Path: "/path/1", Score: 1.0, Snippet: "精确匹配"},
	}, nil
}

func TestSearcherInterface(t *testing.T) {
	ctx := context.Background()
	searcher := &mockSearcher{}

	results, err := searcher.Search(ctx, "测试查询", DefaultSearchOptions())
	if err != nil {
		t.Errorf("Search 失败: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("期望 2 个结果，实际为 %d", len(results))
	}

	embedding := []float64{0.1, 0.2, 0.3, 0.4}
	vecResults, err := searcher.SearchVector(ctx, embedding, DefaultSearchOptions())
	if err != nil {
		t.Errorf("SearchVector 失败: %v", err)
	}
	if len(vecResults) != 1 {
		t.Errorf("期望 1 个向量搜索结果")
	}

	kwResults, err := searcher.SearchKeyword(ctx, "关键词", DefaultSearchOptions())
	if err != nil {
		t.Errorf("SearchKeyword 失败: %v", err)
	}
	if len(kwResults) != 1 {
		t.Errorf("期望 1 个关键词搜索结果")
	}
}

type mockEmbedder struct {
	dim int
}

func (e *mockEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	return make([]float64, e.dim), nil
}

func (e *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = make([]float64, e.dim)
	}
	return result, nil
}

func (e *mockEmbedder) Dimensions() int {
	return e.dim
}

func TestEmbedderInterface(t *testing.T) {
	ctx := context.Background()
	embedder := &mockEmbedder{dim: 768}

	emb, err := embedder.Embed(ctx, "测试文本")
	if err != nil {
		t.Errorf("Embed 失败: %v", err)
	}
	if len(emb) != 768 {
		t.Errorf("期望嵌入维度为 768，实际为 %d", len(emb))
	}

	batch, err := embedder.EmbedBatch(ctx, []string{"文本1", "文本2", "文本3"})
	if err != nil {
		t.Errorf("EmbedBatch 失败: %v", err)
	}
	if len(batch) != 3 {
		t.Errorf("期望 3 个嵌入，实际为 %d", len(batch))
	}

	if embedder.Dimensions() != 768 {
		t.Errorf("期望维度为 768")
	}
}

type mockSummarizer struct{}

func (s *mockSummarizer) Summarize(ctx context.Context, messages []*Message) (string, error) {
	return "生成的摘要", nil
}

func (s *mockSummarizer) Flush(ctx context.Context, messages []*Message, userID string, reason string) error {
	return nil
}

func TestSummarizerInterface(t *testing.T) {
	ctx := context.Background()
	summarizer := &mockSummarizer{}

	messages := []*Message{
		newTestMessage(RoleUser, "消息1"),
		newTestMessage(RoleAssistant, "回复1"),
	}

	summary, err := summarizer.Summarize(ctx, messages)
	if err != nil {
		t.Errorf("Summarize 失败: %v", err)
	}
	if summary != "生成的摘要" {
		t.Errorf("期望摘要为 '生成的摘要'")
	}

	err = summarizer.Flush(ctx, messages, "user-001", "会话结束")
	if err != nil {
		t.Errorf("Flush 失败: %v", err)
	}
}

func TestComputeHash(t *testing.T) {
	content := "测试内容"
	hash1 := ComputeHash(content)
	hash2 := ComputeHash(content)

	if hash1 != hash2 {
		t.Error("相同内容应该产生相同的哈希")
	}
	if len(hash1) != 64 {
		t.Errorf("SHA256 哈希长度应为 64，实际为 %d", len(hash1))
	}

	differentHash := ComputeHash("不同的内容")
	if hash1 == differentHash {
		t.Error("不同内容应该产生不同的哈希")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID("内容")

	// 确保时间戳不同
	time.Sleep(time.Millisecond)

	id2 := generateID("内容")

	if id1 == id2 {
		t.Error("相同内容在不同时间应该产生不同的 ID（由于时间戳）")
	}
	if len(id1) != 16 {
		t.Errorf("期望 ID 长度为 16，实际为 %d", len(id1))
	}
}

func TestToJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"字符串", "hello", `"hello"`},
		{"数字", 42, `42`},
		{"布尔", true, `true`},
		{"切片", []string{"a", "b"}, `["a","b"]`},
		{"map", map[string]string{"key": "value"}, `{"key":"value"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToJSON(tt.input)
			if result != tt.expected {
				t.Errorf("ToJSON(%v) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFromJSON(t *testing.T) {
	t.Run("有效 JSON", func(t *testing.T) {
		var result map[string]string
		err := FromJSON(`{"key":"value"}`, &result)
		if err != nil {
			t.Errorf("FromJSON 失败: %v", err)
		}
		if result["key"] != "value" {
			t.Errorf("期望 key 为 value，实际为 %s", result["key"])
		}
	})

	t.Run("无效 JSON", func(t *testing.T) {
		var result map[string]string
		err := FromJSON(`invalid json`, &result)
		if err == nil {
			t.Error("期望解析无效 JSON 时返回错误")
		}
	})
}
