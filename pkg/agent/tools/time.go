// Package tools 提供代理内置工具实现
package tools

import (
	"fmt"
	"time"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

// TimeTool 时间工具
type TimeTool struct{}

// NewTimeTool 创建时间工具实例
func NewTimeTool() *TimeTool {
	return &TimeTool{}
}

// Name 返回工具名称
func (t *TimeTool) Name() string {
	return "time"
}

// Description 返回工具描述
func (t *TimeTool) Description() string {
	return "Get the current date and time."
}

// Parameters 返回工具参数的 JSON Schema
func (t *TimeTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

// Stage 返回工具执行阶段
func (t *TimeTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 获取当前时间
func (t *TimeTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	now := time.Now()
	result := fmt.Sprintf("Current time: %s\nDate: %s\nUnix timestamp: %d",
		now.Format(time.RFC1123),
		now.Format("2006-01-02"),
		now.Unix(),
	)
	return agent.NewToolResult(result), nil
}
