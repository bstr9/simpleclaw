// Package tools 提供代理内置工具实现
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

const (
	// maxTextFileSize 文本读取的最大文件大小 (50MB)
	maxTextFileSize = 50 * 1024 * 1024
	// maxContentChars 模型上下文的最大内容字符数
	maxContentChars = 20 * 1024
)

// ReadTool 读取文件内容的工具
type ReadTool struct {
	workingDir string
}

// NewReadTool 创建新的 ReadTool 实例
func NewReadTool(workingDir string) *ReadTool {
	return &ReadTool{workingDir: workingDir}
}

// Name 返回工具名称
func (t *ReadTool) Name() string {
	return "read"
}

// Description 返回工具描述
func (t *ReadTool) Description() string {
	return "Read or inspect file contents. For text files, returns content (truncated to 2000 lines or 20KB). Supports offset and limit for large files. Use negative offset to read from end."
}

// Parameters 返回工具参数的 JSON Schema
func (t *ReadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to read. Relative paths are based on workspace directory. For files outside workspace, use absolute paths starting with ~ or /.",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Line number to start reading from (1-indexed, optional). Use negative values to read from end (e.g. -20 for last 20 lines).",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of lines to read (optional).",
			},
		},
		"required": []string{"path"},
	}
}

// Stage 返回工具执行阶段
func (t *ReadTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 读取文件并返回内容
func (t *ReadTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return agent.NewErrorToolResult(fmt.Errorf("path parameter is required")), nil
	}

	// 解析路径
	absolutePath := t.resolvePath(path)

	// 检查文件是否存在
	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return agent.NewErrorToolResult(fmt.Errorf("file not found: %s (resolved to: %s)", path, absolutePath)), nil
		}
		return agent.NewErrorToolResult(fmt.Errorf("cannot access file: %w", err)), nil
	}

	// 检查是否为目录
	if info.IsDir() {
		return agent.NewErrorToolResult(fmt.Errorf("path is a directory, use 'ls' tool instead: %s", path)), nil
	}

	// 检查文件大小
	if info.Size() > maxTextFileSize {
		return agent.NewToolResult(map[string]any{
			"type":    "file_too_large",
			"message": fmt.Sprintf("File too large (%s). Maximum size is 50MB.", formatSize(info.Size())),
			"path":    absolutePath,
			"size":    info.Size(),
		}), nil
	}

	// 读取文件内容
	content, err := os.ReadFile(absolutePath)
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("failed to read file: %w", err)), nil
	}

	// 解析 offset 和 limit 参数
	offsetFloat, _ := params["offset"].(float64)
	limitFloat, _ := params["limit"].(float64)
	offset := int(offsetFloat)
	limit := int(limitFloat)

	// 处理文本内容
	result := t.processTextContent(content, offset, limit, path)
	return agent.NewToolResult(result), nil
}

func (t *ReadTool) resolvePath(path string) string {
	// 展开 ~ 到用户主目录
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.ToSlash(filepath.Join(home, path[2:]))
	}
	// Unix 风格绝对路径（以 / 开头）
	if strings.HasPrefix(path, "/") {
		return filepath.ToSlash(path)
	}
	// Windows 绝对路径
	if filepath.IsAbs(path) {
		return filepath.ToSlash(path)
	}
	// 相对路径
	if t.workingDir != "" {
		return filepath.ToSlash(filepath.Join(t.workingDir, path))
	}
	abs, _ := filepath.Abs(path)
	return filepath.ToSlash(abs)
}

func (t *ReadTool) processTextContent(content []byte, offset, limit int, _ string) map[string]any {
	// 尝试解码为 UTF-8
	text := string(content)
	lines := strings.Split(text, "\n")
	totalLines := len(lines)

	// 处理偏移量
	startLine := 0
	if offset < 0 {
		// 负偏移量：从末尾读取
		startLine = maxInt(0, totalLines+offset)
	} else if offset > 0 {
		// 正偏移量：从开头读取（1-indexed）
		startLine = maxInt(0, offset-1)
		if startLine >= totalLines {
			return map[string]any{
				"error": fmt.Sprintf("Offset %d is beyond end of file (%d lines total)", offset, totalLines),
			}
		}
	}

	// 获取选中的行
	endLine := totalLines
	if limit > 0 {
		endLine = minInt(startLine+limit, totalLines)
	}

	selectedLines := lines[startLine:endLine]
	selectedText := strings.Join(selectedLines, "\n")

	// 如果内容过长则截断
	truncated := false
	if len(selectedText) > maxContentChars {
		selectedText = selectedText[:maxContentChars]
		truncated = true
	}

	// 格式化输出
	output := selectedText
	startLineDisplay := startLine + 1
	endLineDisplay := startLine + len(selectedLines)

	if truncated {
		output += fmt.Sprintf("\n\n[Content truncated. Showing lines %d-%d of %d. Use offset/limit to read more.]", startLineDisplay, endLineDisplay, totalLines)
	} else if limit > 0 && endLine < totalLines {
		remaining := totalLines - endLine
		output += fmt.Sprintf("\n\n[%d more lines in file. Use offset=%d to continue.]", remaining, endLineDisplay+1)
	}

	return map[string]any{
		"content":     output,
		"total_lines": totalLines,
		"start_line":  startLineDisplay,
		"end_line":    endLineDisplay,
	}
}

// minInt 返回两个整数中的较小值
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// maxInt 返回两个整数中的较大值
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
