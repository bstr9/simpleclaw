// Package tools 提供代理内置工具实现
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

// WriteTool 写入内容到文件的工具
type WriteTool struct {
	workingDir string
}

// NewWriteTool 创建新的 WriteTool 实例
func NewWriteTool(workingDir string) *WriteTool {
	return &WriteTool{workingDir: workingDir}
}

// Name 返回工具名称
func (t *WriteTool) Name() string {
	return "write"
}

// Description 返回工具描述
func (t *WriteTool) Description() string {
	return "Write content to a file. Creates the file if it does not exist, overwrites if it does. Automatically creates parent directories. IMPORTANT: Single write should not exceed 10KB. For large files, create a skeleton first, then use edit to add content in chunks."
}

// Parameters 返回工具参数的 JSON Schema
func (t *WriteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to write (relative or absolute)",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Content to write to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

// Stage 返回工具执行阶段
func (t *WriteTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 写入内容到文件
func (t *WriteTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	path, _ := params["path"].(string)
	content, _ := params["content"].(string)

	if path == "" {
		return agent.NewErrorToolResult(fmt.Errorf("path parameter is required")), nil
	}

	// 解析路径
	absolutePath := t.resolvePath(path)

	// 如需则创建父目录
	parentDir := filepath.Dir(absolutePath)
	if parentDir != "" && parentDir != "." {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return agent.NewErrorToolResult(fmt.Errorf("failed to create parent directory: %w", err)), nil
		}
	}

	// 写入文件
	if err := os.WriteFile(absolutePath, []byte(content), 0644); err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("failed to write file: %w", err)), nil
	}

	bytesWritten := len(content)
	return agent.NewToolResult(map[string]any{
		"message":       fmt.Sprintf("Successfully wrote %d bytes to %s", bytesWritten, path),
		"path":          path,
		"bytes_written": bytesWritten,
	}), nil
}

func (t *WriteTool) resolvePath(path string) string {
	// 展开 ~ 到用户主目录
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	// 绝对路径
	if filepath.IsAbs(path) {
		return path
	}
	// 相对路径
	if t.workingDir != "" {
		return filepath.Join(t.workingDir, path)
	}
	abs, _ := filepath.Abs(path)
	return abs
}
