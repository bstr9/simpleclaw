// Package tools 提供代理内置工具实现
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

// EditTool 通过精确文本替换编辑文件的工具
type EditTool struct {
	workingDir string
}

// NewEditTool 创建新的 EditTool 实例
func NewEditTool(workingDir string) *EditTool {
	return &EditTool{workingDir: workingDir}
}

// Name 返回工具名称
func (t *EditTool) Name() string {
	return "edit"
}

// Description 返回工具描述
func (t *EditTool) Description() string {
	return "Edit a file by replacing exact text, or append to end if oldText is empty. For append: use empty oldText. For replace: oldText must match exactly (including whitespace)."
}

// Parameters 返回工具参数的 JSON Schema
func (t *EditTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to edit (relative or absolute)",
			},
			"oldText": map[string]any{
				"type":        "string",
				"description": "Text to find and replace. Use empty string to append to end of file. For replacement: must match exactly including whitespace.",
			},
			"newText": map[string]any{
				"type":        "string",
				"description": "New text to replace the old text with",
			},
		},
		"required": []string{"path", "oldText", "newText"},
	}
}

// Stage 返回工具执行阶段
func (t *EditTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 编辑文件
func (t *EditTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return agent.NewErrorToolResult(fmt.Errorf("path parameter is required")), nil
	}

	oldText, newText := t.parseEditParams(params)

	absolutePath := t.resolvePath(path)
	if _, err := os.Stat(absolutePath); os.IsNotExist(err) {
		return agent.NewErrorToolResult(fmt.Errorf("file not found: %s", path)), nil
	}

	content, err := os.ReadFile(absolutePath)
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("failed to read file: %w", err)), nil
	}

	text := string(content)
	newContent, err := t.applyEdit(text, oldText, newText, path)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	if text == newContent {
		return agent.NewErrorToolResult(fmt.Errorf("no changes made to %s - replacement produced identical content", path)), nil
	}

	if err := os.WriteFile(absolutePath, []byte(newContent), 0644); err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("failed to write file: %w", err)), nil
	}

	return agent.NewToolResult(map[string]any{
		"message": fmt.Sprintf("Successfully replaced text in %s", path),
		"path":    path,
	}), nil
}

// parseEditParams 解析编辑参数
func (t *EditTool) parseEditParams(params map[string]any) (oldText, newText string) {
	oldText, _ = params["oldText"].(string)
	if oldText == "" {
		oldText, _ = params["old_text"].(string)
	}
	newText, _ = params["newText"].(string)
	if newText == "" {
		newText, _ = params["new_text"].(string)
	}
	return
}

// applyEdit 应用编辑操作
func (t *EditTool) applyEdit(text, oldText, newText, path string) (string, error) {
	if oldText == "" {
		return t.appendText(text, newText), nil
	}
	return t.replaceText(text, oldText, newText, path)
}

// appendText 追加文本到文件末尾
func (t *EditTool) appendText(text, newText string) string {
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		return text + "\n" + newText
	}
	return text + newText
}

// replaceText 替换文本
func (t *EditTool) replaceText(text, oldText, newText, path string) (string, error) {
	count := strings.Count(text, oldText)
	if count == 0 {
		return "", fmt.Errorf("could not find the exact text in %s. The old text must match exactly including all whitespace and newlines", path)
	}
	if count > 1 {
		return "", fmt.Errorf("found %d occurrences of the text in %s. The text must be unique. Please provide more context to make it unique", count, path)
	}
	return strings.Replace(text, oldText, newText, 1), nil
}

func (t *EditTool) resolvePath(path string) string {
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
