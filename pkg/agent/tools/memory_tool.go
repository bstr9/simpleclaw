// Package tools 提供内置工具实现
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/common"
)

// memoryTool 常量
const (
	// memoryDirPattern 记忆文件目录模式
	memoryDirPattern = "memory/YYYY-MM-DD.md"
	// memoryStoreHint 记忆存储提示信息
	memoryStoreHint = "可以通过写入 %s 或 " + memoryDirPattern + " 文件来存储记忆"
	// defaultMaxResults 默认最大搜索结果数
	defaultMaxResults = 10
	// snippetContextChars 片段上下文字符数
	snippetContextChars = 50
	// snippetExtraChars 片段额外字符数
	snippetExtraChars = 150
)

// MemoryTool 记忆工具，用于搜索和读取记忆文件
type MemoryTool struct {
	workingDir string
}

// NewMemoryTool 创建记忆工具实例
func NewMemoryTool(workingDir string) *MemoryTool {
	return &MemoryTool{workingDir: workingDir}
}

// Name 返回工具名称
func (t *MemoryTool) Name() string {
	return "memory"
}

// Description 返回工具描述
func (t *MemoryTool) Description() string {
	return "搜索和读取记忆文件。使用语义和关键词搜索来回忆过去的对话、偏好和知识。"
}

// Parameters 返回参数 JSON Schema
func (t *MemoryTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "操作类型: search(搜索记忆), get(读取记忆文件)",
				"enum":        []string{"search", "get"},
			},
			"query": map[string]any{
				"type":        "string",
				"description": "搜索查询（用于 search 操作）",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "记忆文件路径（用于 get 操作）",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "起始行号（可选，默认1）",
			},
			"num_lines": map[string]any{
				"type":        "integer",
				"description": "读取行数（可选）",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"description": "最大返回结果数（可选，默认10）",
			},
		},
		"required": []string{"action"},
	}
}

// Stage 返回工具执行阶段
func (t *MemoryTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 执行工具
func (t *MemoryTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return agent.NewErrorToolResult(fmt.Errorf("action 参数是必需的")), nil
	}

	switch action {
	case "search":
		return t.handleSearch(params)
	case "get":
		return t.handleGet(params)
	default:
		return agent.NewErrorToolResult(fmt.Errorf("不支持的操作: %s", action)), nil
	}
}

func (t *MemoryTool) handleSearch(params map[string]any) (*agent.ToolResult, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return agent.NewErrorToolResult(fmt.Errorf("search 操作需要 query 参数")), nil
	}

	maxResults := defaultMaxResults
	if mr, ok := params["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
	}

	memoryDir := t.getMemoryDir()
	if _, err := os.Stat(memoryDir); os.IsNotExist(err) {
		return agent.NewToolResult(map[string]any{
			"message": fmt.Sprintf(memoryStoreHint, common.MemoryFileName),
			"results": []any{},
		}), nil
	}

	results := t.searchInMemory(query, memoryDir, maxResults)

	if len(results) == 0 {
		return agent.NewToolResult(map[string]any{
			"message": fmt.Sprintf("未找到与 '%s' 相关的记忆", query),
			"results": []any{},
			"note":    fmt.Sprintf(memoryStoreHint, common.MemoryFileName),
		}), nil
	}

	return agent.NewToolResult(map[string]any{
		"message": fmt.Sprintf("找到 %d 条相关记忆", len(results)),
		"results": results,
	}), nil
}

func (t *MemoryTool) handleGet(params map[string]any) (*agent.ToolResult, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return agent.NewErrorToolResult(fmt.Errorf("get 操作需要 path 参数")), nil
	}

	startLine := 1
	if sl, ok := params["start_line"].(float64); ok && sl > 0 {
		startLine = int(sl)
	}

	var numLines int
	if nl, ok := params["num_lines"].(float64); ok && nl > 0 {
		numLines = int(nl)
	}

	// 自动添加 memory/ 前缀
	if !strings.HasPrefix(path, "memory/") && !filepath.IsAbs(path) && path != common.MemoryFileName {
		path = filepath.Join("memory", path)
	}

	filePath := t.resolvePath(path)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("无法读取记忆文件: %w", err)), nil
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	startIdx := max(0, startLine-1)
	if startIdx >= totalLines {
		return agent.NewErrorToolResult(fmt.Errorf("起始行号超出文件范围")), nil
	}

	endIdx := totalLines
	if numLines > 0 && startIdx+numLines < totalLines {
		endIdx = startIdx + numLines
	}

	selectedLines := lines[startIdx:endIdx]
	content := strings.Join(selectedLines, "\n")

	return agent.NewToolResult(map[string]any{
		"path":        path,
		"content":     content,
		"total_lines": totalLines,
		"start_line":  startLine,
		"end_line":    startLine + len(selectedLines) - 1,
	}), nil
}

func (t *MemoryTool) getMemoryDir() string {
	return filepath.Join(t.workingDir, "memory")
}

func (t *MemoryTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return filepath.Join(t.workingDir, path)
}

func (t *MemoryTool) searchInMemory(query string, memoryDir string, maxResults int) []map[string]any {
	var results []map[string]any
	queryLower := strings.ToLower(query)

	filepath.Walk(memoryDir, func(path string, info os.FileInfo, err error) error {
		if t.shouldSkipEntry(err, info) {
			return nil
		}
		if t.tryAppendResult(&results, path, query, queryLower, maxResults) {
			return filepath.SkipAll
		}
		return nil
	})

	results = t.appendRootMemoryFile(results, query, queryLower, maxResults)

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

// shouldSkipEntry 检查是否应该跳过目录条目
func (t *MemoryTool) shouldSkipEntry(err error, info os.FileInfo) bool {
	return err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".md")
}

// tryAppendResult 尝试将搜索结果添加到列表，返回是否已达到最大结果数
func (t *MemoryTool) tryAppendResult(results *[]map[string]any, path, query, queryLower string, maxResults int) bool {
	result := t.searchInFile(path, query, queryLower)
	if result != nil {
		*results = append(*results, result)
		return len(*results) >= maxResults
	}
	return false
}

func (t *MemoryTool) searchInFile(path, query, queryLower string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	content := string(data)
	if !strings.Contains(strings.ToLower(content), queryLower) {
		return nil
	}

	relPath, _ := filepath.Rel(t.workingDir, path)
	snippet := t.extractSnippet(content, query, 200)

	return map[string]any{
		"path":    relPath,
		"snippet": snippet,
	}
}

func (t *MemoryTool) appendRootMemoryFile(results []map[string]any, query, queryLower string, maxResults int) []map[string]any {
	if len(results) >= maxResults {
		return results
	}

	memoryFile := filepath.Join(t.workingDir, common.MemoryFileName)
	data, err := os.ReadFile(memoryFile)
	if err != nil {
		return results
	}

	content := string(data)
	if !strings.Contains(strings.ToLower(content), queryLower) {
		return results
	}

	snippet := t.extractSnippet(content, query, 200)
	return append([]map[string]any{{
		"path":    common.MemoryFileName,
		"snippet": snippet,
	}}, results...)
}

func (t *MemoryTool) extractSnippet(content, query string, maxLen int) string {
	contentLower := strings.ToLower(content)
	queryLower := strings.ToLower(query)
	idx := strings.Index(contentLower, queryLower)

	if idx == -1 {
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}

	start := max(0, idx-snippetContextChars)
	end := min(len(content), idx+len(query)+snippetExtraChars)

	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}

	return snippet
}
