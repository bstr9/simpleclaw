// Package tools 提供代理内置工具实现
package tools

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

const (
	fetchUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"
	fetchAccept    = "text/html,application/xhtml+xml,application/xml;q=0.9,text/markdown,*/*;q=0.8"
	maxFetchSize   = 32000
)

// WebFetchTool 网页获取工具
type WebFetchTool struct {
	workingDir string
	timeout    time.Duration
	maxChars   int
}

// WebFetchOption 网页获取工具配置选项
type WebFetchOption func(*WebFetchTool)

// WithFetchTimeout 设置超时时间
func WithFetchTimeout(timeout time.Duration) WebFetchOption {
	return func(t *WebFetchTool) {
		t.timeout = timeout
	}
}

// WithFetchMaxChars 设置最大字符数
func WithFetchMaxChars(maxChars int) WebFetchOption {
	return func(t *WebFetchTool) {
		t.maxChars = maxChars
	}
}

// NewWebFetchTool 创建网页获取工具实例
func NewWebFetchTool(workingDir string, opts ...WebFetchOption) *WebFetchTool {
	t := &WebFetchTool{
		workingDir: workingDir,
		timeout:    60 * time.Second,
		maxChars:   16000,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Name 返回工具名称
func (t *WebFetchTool) Name() string {
	return "web_fetch"
}

// Description 返回工具描述
func (t *WebFetchTool) Description() string {
	return "Fetch content from a URL and optionally save to a file."
}

// Parameters 返回工具参数的 JSON Schema
func (t *WebFetchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to fetch.",
			},
			"output_file": map[string]any{
				"type":        "string",
				"description": "Optional file path to save the content.",
			},
		},
		"required": []string{"url"},
	}
}

// Stage 返回工具执行阶段
func (t *WebFetchTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 获取网页内容
func (t *WebFetchTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	rawURL, ok := params["url"].(string)
	if !ok {
		return agent.NewErrorToolResult(fmt.Errorf("url parameter is required")), nil
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return agent.NewErrorToolResult(fmt.Errorf("only http and https URLs are supported")), nil
	}

	client := &http.Client{Timeout: t.timeout}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	req.Header.Set("User-Agent", fetchUserAgent)
	req.Header.Set("Accept", fetchAccept)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchSize))
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	outputFile, _ := params["output_file"].(string)
	if outputFile != "" {
		if t.workingDir != "" && !filepath.IsAbs(outputFile) {
			outputFile = filepath.Join(t.workingDir, outputFile)
		}

		if err := os.WriteFile(outputFile, body, 0644); err != nil {
			return agent.NewErrorToolResult(err), nil
		}

		return agent.NewToolResult(fmt.Sprintf("Successfully fetched %s and saved to %s (%d bytes)", rawURL, outputFile, len(body))), nil
	}

	content := extractReadableContent(string(body))

	if len(content) > t.maxChars {
		content = content[:t.maxChars] + "\n... (truncated)"
	}

	return agent.NewToolResult(content), nil
}

// extractReadableContent 从 HTML 中提取可读内容
func extractReadableContent(html string) string {
	html = removeTags(html, "script")
	html = removeTags(html, "style")
	html = removeTags(html, "nav")
	html = removeTags(html, "header")
	html = removeTags(html, "footer")
	html = removeTags(html, "aside")

	if bodyStart := strings.Index(html, "<body"); bodyStart != -1 {
		if bodyEnd := strings.Index(html[bodyStart:], ">"); bodyEnd != -1 {
			bodyStart += bodyEnd + 1
			if bodyClose := strings.Index(html[bodyStart:], "</body>"); bodyClose != -1 {
				html = html[bodyStart : bodyStart+bodyClose]
			}
		}
	}

	html = extractMainContent(html)
	text := stripHTMLTags(html)
	text = cleanWhitespace(text)
	text = decodeHTMLEntities(text)

	return text
}

// removeTags 移除指定标签及其内容
func removeTags(html, tag string) string {
	pattern := fmt.Sprintf(`<%s[^>]*>[\s\S]*?</%s>`, tag, tag)
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(html, "")
}

// extractMainContent 提取主要内容区域
func extractMainContent(html string) string {
	patterns := []string{
		`<article[^>]*>([\s\S]*?)</article>`,
		`<main[^>]*>([\s\S]*?)</main>`,
		`<div[^>]*class="[^"]*content[^"]*"[^>]*>([\s\S]*?)</div>`,
		`<div[^>]*class="[^"]*article[^"]*"[^>]*>([\s\S]*?)</div>`,
		`<div[^>]*id="[^"]*content[^"]*"[^>]*>([\s\S]*?)</div>`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(html); len(match) > 1 {
			content := match[1]
			if len(content) > 500 {
				return content
			}
		}
	}

	return html
}

// stripHTMLTags 移除所有 HTML 标签并保留结构
func stripHTMLTags(html string) string {
	html = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`<p[^>]*>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`</p>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`<li[^>]*>`).ReplaceAllString(html, "\n- ")
	html = regexp.MustCompile(`<h[1-6][^>]*>`).ReplaceAllString(html, "\n\n## ")
	html = regexp.MustCompile(`</h[1-6]>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(html, " ")

	return html
}

// cleanWhitespace 清理多余空白
func cleanWhitespace(text string) string {
	text = regexp.MustCompile(`[ \t]+`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
