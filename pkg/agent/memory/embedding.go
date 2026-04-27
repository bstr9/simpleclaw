// Package memory 提供代理记忆管理功能。
// embedding.go 实现向量嵌入接口和多种嵌入提供者。
package memory

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// EmbeddingProvider 定义了嵌入提供者的接口。
type EmbeddingProvider interface {
	// Embed 为单个文本生成嵌入向量。
	Embed(ctx context.Context, text string) ([]float64, error)
	// EmbedBatch 为多个文本批量生成嵌入向量。
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
	// Dimensions 返回嵌入向量的维度。
	Dimensions() int
	// Model 返回使用的模型名称。
	Model() string
}

// OpenAIEmbeddingProvider 使用 OpenAI API 生成嵌入向量。
type OpenAIEmbeddingProvider struct {
	apiKey       string            // API 密钥
	apiBase      string            // API 基础 URL
	model        string            // 模型名称
	dimensions   int               // 向量维度
	httpClient   *http.Client      // HTTP 客户端
	extraHeaders map[string]string // 额外的请求头
}

// OpenAIEmbeddingOption 是 OpenAIEmbeddingProvider 的函数式选项。
type OpenAIEmbeddingOption func(*OpenAIEmbeddingProvider)

// WithAPIBase 设置 API 基础 URL。
func WithAPIBase(apiBase string) OpenAIEmbeddingOption {
	return func(p *OpenAIEmbeddingProvider) {
		if apiBase != "" {
			p.apiBase = strings.TrimSuffix(apiBase, "/")
		}
	}
}

// WithExtraHeaders 设置额外的请求头。
func WithExtraHeaders(headers map[string]string) OpenAIEmbeddingOption {
	return func(p *OpenAIEmbeddingProvider) {
		p.extraHeaders = headers
	}
}

// WithHTTPClient 设置自定义 HTTP 客户端。
func WithHTTPClient(client *http.Client) OpenAIEmbeddingOption {
	return func(p *OpenAIEmbeddingProvider) {
		p.httpClient = client
	}
}

// WithTimeout 设置请求超时时间。
func WithTimeout(timeout time.Duration) OpenAIEmbeddingOption {
	return func(p *OpenAIEmbeddingProvider) {
		p.httpClient = &http.Client{
			Timeout: timeout,
		}
	}
}

// NewOpenAIEmbeddingProvider 创建一个新的 OpenAI 嵌入提供者。
// model: 模型名称，如 "text-embedding-3-small" 或 "text-embedding-3-large"
// apiKey: OpenAI API 密钥
func NewOpenAIEmbeddingProvider(model, apiKey string, opts ...OpenAIEmbeddingOption) (*OpenAIEmbeddingProvider, error) {
	if apiKey == "" || apiKey == "YOUR API KEY" || apiKey == "YOUR_API_KEY" {
		return nil, fmt.Errorf("OpenAI API 密钥未配置")
	}

	if model == "" {
		model = "text-embedding-3-small"
	}

	// 根据模型设置默认维度
	dimensions := 1536 // text-embedding-3-small
	if strings.Contains(model, "large") {
		dimensions = 3072 // text-embedding-3-large
	}

	provider := &OpenAIEmbeddingProvider{
		apiKey:     apiKey,
		apiBase:    "https://api.openai.com/v1",
		model:      model,
		dimensions: dimensions,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		extraHeaders: make(map[string]string),
	}

	for _, opt := range opts {
		opt(provider)
	}

	return provider, nil
}

// EmbeddingRequest 表示 OpenAI 嵌入 API 的请求体。
type EmbeddingRequest struct {
	Input any `json:"input"` // 可以是字符串或字符串数组
	Model string      `json:"model"`
}

// EmbeddingResponse 表示 OpenAI 嵌入 API 的响应体。
type EmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Embed 为单个文本生成嵌入向量。
func (p *OpenAIEmbeddingProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("未返回嵌入向量")
	}
	return embeddings[0], nil
}

// EmbedBatch 为多个文本批量生成嵌入向量。
func (p *OpenAIEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// 构建请求
	reqBody := EmbeddingRequest{
		Input: texts,
		Model: p.model,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 创建 HTTP 请求
	url := p.apiBase + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Authorization", common.AuthPrefixBearer+p.apiKey)
	for k, v := range p.extraHeaders {
		req.Header.Set(k, v)
	}

	// 发送请求
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var embeddingResp EmbeddingResponse
	if err := json.Unmarshal(body, &embeddingResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查错误
	if embeddingResp.Error != nil {
		return nil, fmt.Errorf("API 错误: %s", embeddingResp.Error.Message)
	}

	// 提取嵌入向量
	embeddings := make([][]float64, len(embeddingResp.Data))
	for _, data := range embeddingResp.Data {
		embeddings[data.Index] = data.Embedding
	}

	return embeddings, nil
}

// Dimensions 返回嵌入向量的维度。
func (p *OpenAIEmbeddingProvider) Dimensions() int {
	return p.dimensions
}

// Model 返回使用的模型名称。
func (p *OpenAIEmbeddingProvider) Model() string {
	return p.model
}

// EmbeddingCache 提供嵌入向量的内存缓存。
type EmbeddingCache struct {
	mu      sync.RWMutex
	cache   map[string][]float64
	maxSize int // 最大缓存条目数
}

// NewEmbeddingCache 创建一个新的嵌入缓存。
func NewEmbeddingCache(maxSize int) *EmbeddingCache {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &EmbeddingCache{
		cache:   make(map[string][]float64),
		maxSize: maxSize,
	}
}

// Get 从缓存中获取嵌入向量。
func (c *EmbeddingCache) Get(text string, provider, model string) ([]float64, bool) {
	key := c.computeKey(text, provider, model)
	c.mu.RLock()
	defer c.mu.RUnlock()
	embedding, ok := c.cache[key]
	return embedding, ok
}

// Put 将嵌入向量存入缓存。
func (c *EmbeddingCache) Put(text string, provider, model string, embedding []float64) {
	key := c.computeKey(text, provider, model)
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果缓存已满，删除部分旧条目
	if len(c.cache) >= c.maxSize {
		// 简单策略：删除一半
		count := 0
		for k := range c.cache {
			delete(c.cache, k)
			count++
			if count >= c.maxSize/2 {
				break
			}
		}
	}

	c.cache[key] = embedding
}

// computeKey 计算缓存键。
func (c *EmbeddingCache) computeKey(text, provider, model string) string {
	content := fmt.Sprintf("%s:%s:%s", provider, model, text)
	hash := md5.Sum([]byte(content))
	return hex.EncodeToString(hash[:])
}

// Clear 清空缓存。
func (c *EmbeddingCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string][]float64)
}

// Size 返回缓存大小。
func (c *EmbeddingCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// CachedEmbedder 包装 Embedder 提供缓存功能。
type CachedEmbedder struct {
	provider     EmbeddingProvider
	cache        *EmbeddingCache
	providerName string
}

// NewCachedEmbedder 创建一个带缓存的嵌入器。
func NewCachedEmbedder(provider EmbeddingProvider, cache *EmbeddingCache, providerName string) *CachedEmbedder {
	return &CachedEmbedder{
		provider:     provider,
		cache:        cache,
		providerName: providerName,
	}
}

// Embed 为单个文本生成嵌入向量（带缓存）。
func (e *CachedEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	model := e.provider.Model()

	// 尝试从缓存获取
	if embedding, ok := e.cache.Get(text, e.providerName, model); ok {
		return embedding, nil
	}

	// 调用提供者生成嵌入
	embedding, err := e.provider.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	// 存入缓存
	e.cache.Put(text, e.providerName, model, embedding)

	return embedding, nil
}

// EmbedBatch 批量生成嵌入向量（带缓存）。
func (e *CachedEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	model := e.provider.Model()
	embeddings := make([][]float64, len(texts))
	var uncachedTexts []string
	var uncachedIndices []int

	// 检查缓存
	for i, text := range texts {
		if embedding, ok := e.cache.Get(text, e.providerName, model); ok {
			embeddings[i] = embedding
		} else {
			uncachedTexts = append(uncachedTexts, text)
			uncachedIndices = append(uncachedIndices, i)
		}
	}

	// 如果全部命中缓存
	if len(uncachedTexts) == 0 {
		return embeddings, nil
	}

	// 调用提供者生成未缓存的嵌入
	newEmbeddings, err := e.provider.EmbedBatch(ctx, uncachedTexts)
	if err != nil {
		return nil, err
	}

	// 填充结果并存入缓存
	for idx, embedding := range newEmbeddings {
		origIdx := uncachedIndices[idx]
		embeddings[origIdx] = embedding
		e.cache.Put(texts[origIdx], e.providerName, model, embedding)
	}

	return embeddings, nil
}

// Dimensions 返回嵌入向量的维度。
func (e *CachedEmbedder) Dimensions() int {
	return e.provider.Dimensions()
}

// Model 返回使用的模型名称。
func (e *CachedEmbedder) Model() string {
	return e.provider.Model()
}

// MockEmbedder 用于测试的模拟嵌入器。
type MockEmbedder struct {
	dimensions int
}

// NewMockEmbedder 创建一个模拟嵌入器。
func NewMockEmbedder(dimensions int) *MockEmbedder {
	if dimensions <= 0 {
		dimensions = 1536
	}
	return &MockEmbedder{dimensions: dimensions}
}

// Embed 生成模拟的嵌入向量。
func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	embedding := make([]float64, m.dimensions)
	// 使用文本哈希生成伪随机嵌入
	hash := md5.Sum([]byte(text))
	for i := range embedding {
		embedding[i] = float64(hash[i%16]) / 255.0
	}
	return embedding, nil
}

// EmbedBatch 批量生成模拟嵌入向量。
func (m *MockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	embeddings := make([][]float64, len(texts))
	for i, text := range texts {
		embedding, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

// Dimensions 返回嵌入向量的维度。
func (m *MockEmbedder) Dimensions() int {
	return m.dimensions
}

// Model 返回模拟模型名称。
func (m *MockEmbedder) Model() string {
	return "mock-embedding"
}

// CreateEmbeddingProvider 创建嵌入提供者的工厂函数。
func CreateEmbeddingProvider(providerType, model, apiKey, apiBase string, extraHeaders map[string]string) (EmbeddingProvider, error) {
	switch strings.ToLower(providerType) {
	case "openai", "linkai":
		opts := []OpenAIEmbeddingOption{}
		if apiBase != "" {
			opts = append(opts, WithAPIBase(apiBase))
		}
		if extraHeaders != nil {
			opts = append(opts, WithExtraHeaders(extraHeaders))
		}
		return NewOpenAIEmbeddingProvider(model, apiKey, opts...)
	case "mock":
		dim := 1536
		if strings.Contains(model, "large") {
			dim = 3072
		}
		return NewMockEmbedder(dim), nil
	default:
		return nil, fmt.Errorf("不支持的嵌入提供者类型: %s", providerType)
	}
}
