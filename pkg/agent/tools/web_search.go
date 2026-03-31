// Package tools 提供代理内置工具实现
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/common"
)

// WebSearchTool 网络搜索工具
type WebSearchTool struct {
	provider  string
	baseURL   string
	apiKey    string
	timeout   time.Duration
	userAgent string
}

// WebSearchOption 网络搜索工具配置选项
type WebSearchOption func(*WebSearchTool)

// WithSearchProvider 设置搜索提供商
func WithSearchProvider(provider string) WebSearchOption {
	return func(t *WebSearchTool) {
		t.provider = provider
	}
}

// WithSearchBaseURL 设置搜索基础 URL
func WithSearchBaseURL(baseURL string) WebSearchOption {
	return func(t *WebSearchTool) {
		t.baseURL = baseURL
	}
}

// WithSearchAPIKey 设置搜索 API 密钥
func WithSearchAPIKey(apiKey string) WebSearchOption {
	return func(t *WebSearchTool) {
		t.apiKey = apiKey
	}
}

// WithSearchTimeout 设置搜索超时时间
func WithSearchTimeout(timeout time.Duration) WebSearchOption {
	return func(t *WebSearchTool) {
		t.timeout = timeout
	}
}

// NewWebSearchTool 创建网络搜索工具实例
func NewWebSearchTool(opts ...WebSearchOption) *WebSearchTool {
	t := &WebSearchTool{
		provider:  "duckduckgo",
		timeout:   30 * time.Second,
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Name 返回工具名称
func (t *WebSearchTool) Name() string {
	return "web_search"
}

// Description 返回工具描述
func (t *WebSearchTool) Description() string {
	return "搜索网络信息。支持 DuckDuckGo（默认，免费）和 SearXNG（自托管）。"
}

// Parameters 返回工具参数的 JSON Schema
func (t *WebSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "搜索查询字符串",
			},
			"num_results": map[string]any{
				"type":        "integer",
				"description": "返回结果数量，默认为 5",
			},
		},
		"required": []string{"query"},
	}
}

// Stage 返回工具执行阶段
func (t *WebSearchTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 执行搜索
func (t *WebSearchTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	query, ok := params["query"].(string)
	if !ok {
		return agent.NewErrorToolResult(fmt.Errorf("query 参数是必需的")), nil
	}

	numResults := 5
	if n, ok := params["num_results"].(float64); ok {
		numResults = int(n)
	}

	var results []SearchResult
	var err error

	switch t.provider {
	case "searxng":
		results, err = t.searchSearXNG(query, numResults)
	default:
		results, err = t.searchDuckDuckGo(query, numResults)
	}

	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	if len(results) == 0 {
		return agent.NewToolResult(map[string]any{
			"query":   query,
			"count":   0,
			"results": []SearchResult{},
			"message": "未找到结果",
		}), nil
	}

	return agent.NewToolResult(map[string]any{
		"query":   query,
		"count":   len(results),
		"results": results,
	}), nil
}

// SearchResult 搜索结果
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

const duckDuckGoHTMLEndpoint = "https://html.duckduckgo.com/html"

// searchDuckDuckGo 使用 DuckDuckGo 搜索
func (t *WebSearchTool) searchDuckDuckGo(query string, maxResults int) ([]SearchResult, error) {
	client := &http.Client{Timeout: t.timeout}

	searchURL := fmt.Sprintf("%s/?q=%s", duckDuckGoHTMLEndpoint, url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", t.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	html := string(body)

	if isBotChallenge(html) {
		return nil, fmt.Errorf("DuckDuckGo 返回了机器人检测页面")
	}

	return parseDuckDuckGoHTML(html, maxResults), nil
}

// isBotChallenge 检测是否触发了机器人验证
func isBotChallenge(html string) bool {
	if strings.Contains(html, `class="result__a"`) {
		return false
	}
	return strings.Contains(html, "g-recaptcha") ||
		strings.Contains(html, "are you a human") ||
		strings.Contains(html, `id="challenge-form"`) ||
		strings.Contains(html, `name="challenge"`)
}

// parseDuckDuckGoHTML 解析 DuckDuckGo HTML 搜索结果
func parseDuckDuckGoHTML(html string, maxResults int) []SearchResult {
	var results []SearchResult

	resultPattern := `<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]*)"[^>]*>([^<]*)</a>`
	re := regexp.MustCompile(resultPattern)

	matches := re.FindAllStringSubmatch(html, -1)

	for i, match := range matches {
		if i >= maxResults {
			break
		}

		if len(match) < 3 {
			continue
		}

		rawURL := match[1]
		title := decodeHTMLEntities(strings.TrimSpace(match[2]))

		decodedURL := decodeDuckDuckGoHTMLURL(rawURL)
		if decodedURL == "" || strings.HasPrefix(decodedURL, "/") {
			continue
		}

		result := SearchResult{
			Title:   title,
			URL:     decodedURL,
			Snippet: extractSnippet(html, match[0]),
		}

		results = append(results, result)
	}

	return results
}

// decodeDuckDuckGoHTMLURL 解码 DuckDuckGo HTML 版本的 URL
func decodeDuckDuckGoHTMLURL(rawURL string) string {
	if strings.Contains(rawURL, "uddg=") {
		parsed, err := url.Parse(rawURL)
		if err == nil {
			if parsed.Scheme == "" && strings.HasPrefix(rawURL, "//") {
				parsed, err = url.Parse("https:" + rawURL)
			}
			if err == nil {
				uddg := parsed.Query().Get("uddg")
				if uddg != "" {
					return uddg
				}
			}
		}
	}

	return rawURL
}

// extractSnippet 从搜索结果后面提取摘要
func extractSnippet(html string, resultLink string) string {
	idx := strings.Index(html, resultLink)
	if idx == -1 {
		return ""
	}

	trailing := html[idx+len(resultLink):]

	nextIdx := strings.Index(trailing, `class="result__a"`)
	if nextIdx > 0 {
		trailing = trailing[:nextIdx]
	}

	snippetPattern := `<a[^>]*class="[^"]*result__snippet[^"]*"[^>]*>([^<]*)</a>`
	re := regexp.MustCompile(snippetPattern)
	match := re.FindStringSubmatch(trailing)
	if len(match) > 1 {
		return decodeHTMLEntities(strings.TrimSpace(match[1]))
	}

	return ""
}

// decodeHTMLEntities 解码 HTML 实体
func decodeHTMLEntities(text string) string {
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", `"`)
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&apos;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&ndash;", "-")
	text = strings.ReplaceAll(text, "&mdash;", "--")
	text = strings.ReplaceAll(text, "&hellip;", "...")
	return text
}

// parseDuckDuckGoLite 解析 DuckDuckGo Lite 搜索结果 (保留兼容)
func parseDuckDuckGoLite(html string, maxResults int) []SearchResult {
	var results []SearchResult

	lines := strings.Split(html, "\n")

	for i := 0; i < len(lines) && len(results) < maxResults; i++ {
		line := lines[i]

		// 支持单引号和双引号
		if strings.Contains(line, `class='result-link'`) || strings.Contains(line, `class="result-link"`) {
			if result := parseResultLinkLine(line); result != nil {
				results = append(results, *result)
			}
		}

		// 支持 snippet 解析
		if strings.Contains(line, `class='result-snippet'`) || strings.Contains(line, `class="result-snippet"`) {
			updateSnippet(results, lines, i)
		}
	}

	return results
}

// parseResultLinkLine 解析结果链接行
func parseResultLinkLine(line string) *SearchResult {
	title := extractTitle(line)
	if title == "" {
		return nil
	}

	rawURL := extractURL(line)
	if rawURL == "" {
		return nil
	}

	// 解码 DuckDuckGo 重定向 URL
	url := decodeDuckDuckGoURL(rawURL)
	if url == "" || strings.HasPrefix(url, "/") {
		return nil
	}

	return &SearchResult{
		Title:   title,
		URL:     url,
		Snippet: "",
	}
}

// decodeDuckDuckGoURL 解码 DuckDuckGo 重定向 URL
func decodeDuckDuckGoURL(rawURL string) string {
	// DuckDuckGo Lite 使用格式: //duckduckgo.com/l/?uddg=ENCODED_URL&rut=...
	if strings.Contains(rawURL, "uddg=") {
		// 提取 uddg 参数
		uddgStart := strings.Index(rawURL, "uddg=")
		if uddgStart == -1 {
			return rawURL
		}

		encodedURL := rawURL[uddgStart+5:]

		// 找到下一个 & 或结束
		if ampIdx := strings.Index(encodedURL, "&"); ampIdx != -1 {
			encodedURL = encodedURL[:ampIdx]
		}

		// URL 解码
		decoded, err := url.QueryUnescape(encodedURL)
		if err == nil {
			return decoded
		}
	}

	// 如果不是重定向链接，直接返回
	return rawURL
}

// extractTitle 从行中提取标题
func extractTitle(line string) string {
	titleStart := strings.Index(line, `>`)
	if titleStart == -1 {
		return ""
	}
	titleEnd := strings.Index(line[titleStart+1:], `<`)
	if titleEnd == -1 {
		return ""
	}
	return strings.TrimSpace(line[titleStart+1 : titleStart+1+titleEnd])
}

// extractURL 从行中提取 URL
func extractURL(line string) string {
	hrefStart := strings.Index(line, `href="`)
	if hrefStart == -1 {
		return ""
	}
	hrefStart += 6
	hrefEnd := strings.Index(line[hrefStart:], `"`)
	if hrefEnd == -1 {
		return ""
	}
	return line[hrefStart : hrefStart+hrefEnd]
}

// updateSnippet 更新最后一个结果的摘要
func updateSnippet(results []SearchResult, lines []string, currentIndex int) {
	if len(results) == 0 {
		return
	}
	nextLine := ""
	if currentIndex+1 < len(lines) {
		nextLine = lines[currentIndex+1]
	}
	results[len(results)-1].Snippet = strings.TrimSpace(stripTags(nextLine))
}

// stripTags 移除 HTML 标签
func stripTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// searchSearXNG 使用 SearXNG 搜索
func (t *WebSearchTool) searchSearXNG(query string, maxResults int) ([]SearchResult, error) {
	baseURL := t.baseURL
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	client := &http.Client{Timeout: t.timeout}

	searchURL := fmt.Sprintf("%s/search?q=%s&format=json", strings.TrimSuffix(baseURL, "/"), url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", t.userAgent)
	if t.apiKey != "" {
		req.Header.Set("Authorization", common.AuthPrefixBearer+t.apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searxResp SearXNGResponse
	if err := json.Unmarshal(body, &searxResp); err != nil {
		return nil, fmt.Errorf("解析 SearXNG 响应失败: %w", err)
	}

	var results []SearchResult
	for i, r := range searxResp.Results {
		if i >= maxResults {
			break
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return results, nil
}

// SearXNGResponse SearXNG 搜索响应
type SearXNGResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

// Search 执行搜索（支持上下文取消）
func (t *WebSearchTool) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	done := make(chan struct {
		results []SearchResult
		err     error
	})

	go func() {
		var results []SearchResult
		var err error

		switch t.provider {
		case "searxng":
			results, err = t.searchSearXNG(query, maxResults)
		default:
			results, err = t.searchDuckDuckGo(query, maxResults)
		}

		done <- struct {
			results []SearchResult
			err     error
		}{results, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-done:
		return result.results, result.err
	}
}
