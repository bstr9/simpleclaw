// Package tools 提供代理内置工具实现
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

// LsTool 列出目录内容。
type LsTool struct {
	workingDir string
}

// NewLsTool 创建新的 LsTool 实例。
func NewLsTool(workingDir string) *LsTool {
	return &LsTool{workingDir: workingDir}
}

// Name 返回工具名称。
func (t *LsTool) Name() string {
	return "ls"
}

// Description 返回工具描述。
func (t *LsTool) Description() string {
	return "列出目录内容。返回按字母顺序排序的条目，目录以 '/' 结尾。"
}

// Parameters 返回参数 JSON Schema。
func (t *LsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "要列出的目录路径（默认为当前目录）",
			},
		},
		"required": []string{},
	}
}

// Stage 返回工具执行阶段。
func (t *LsTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 执行目录列表。
func (t *LsTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	path, _ := params["path"].(string)
	if path == "" {
		path = "."
	}

	absolutePath := t.resolvePath(path)

	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return agent.NewErrorToolResult(fmt.Errorf("路径不存在: %s", path)), nil
		}
		return agent.NewErrorToolResult(fmt.Errorf("无法访问路径: %w", err)), nil
	}

	if !info.IsDir() {
		return agent.NewErrorToolResult(fmt.Errorf("不是目录: %s", path)), nil
	}

	entries, err := os.ReadDir(absolutePath)
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("读取目录失败: %w", err)), nil
	}

	var results []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		results = append(results, name)
	}

	if len(results) == 0 {
		return agent.NewToolResult(map[string]any{
			"message": "(空目录)",
			"entries": []string{},
		}), nil
	}

	output := strings.Join(results, "\n")
	return agent.NewToolResult(map[string]any{
		"output":      output,
		"entry_count": len(results),
	}), nil
}

func (t *LsTool) resolvePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	if filepath.IsAbs(path) {
		return path
	}
	if t.workingDir != "" {
		return filepath.Join(t.workingDir, path)
	}
	abs, _ := filepath.Abs(path)
	return abs
}
