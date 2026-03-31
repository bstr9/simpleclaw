// Package tools 提供内置工具实现
package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

// BashTool 执行 shell 命令的工具
type BashTool struct {
	timeout    time.Duration
	workingDir string
	allowList  []string
	denyList   []string
}

// BashToolOption Bash 工具配置选项
type BashToolOption func(*BashTool)

// WithBashTimeout 设置命令超时时间
func WithBashTimeout(timeout time.Duration) BashToolOption {
	return func(t *BashTool) {
		t.timeout = timeout
	}
}

// WithBashWorkingDir 设置工作目录
func WithBashWorkingDir(dir string) BashToolOption {
	return func(t *BashTool) {
		t.workingDir = dir
	}
}

// WithBashAllowList 设置允许的命令列表
func WithBashAllowList(cmds []string) BashToolOption {
	return func(t *BashTool) {
		t.allowList = cmds
	}
}

// WithBashDenyList 设置禁止的命令列表
func WithBashDenyList(cmds []string) BashToolOption {
	return func(t *BashTool) {
		t.denyList = cmds
	}
}

// NewBashTool 创建 Bash 工具实例
func NewBashTool(opts ...BashToolOption) *BashTool {
	t := &BashTool{
		timeout:    30 * time.Second,
		workingDir: "",
		allowList:  []string{},
		denyList:   []string{"rm", "dd", "mkfs", "fdisk", "shutdown", "reboot"},
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Name 返回工具名称
func (t *BashTool) Name() string {
	return "bash"
}

// Description 返回工具描述
func (t *BashTool) Description() string {
	return "执行 Bash shell 命令并返回输出。用于运行系统命令、脚本或其他 shell 操作。"
}

// Parameters 返回参数 JSON Schema
func (t *BashTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "要执行的 bash 命令",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "超时时间（秒），可选，默认 30",
			},
		},
		"required": []string{"command"},
	}
}

// Stage 返回工具执行阶段
func (t *BashTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 执行工具
func (t *BashTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	command, ok := params["command"].(string)
	if !ok {
		return agent.NewErrorToolResult(fmt.Errorf("command 参数是必需的且必须是字符串")), nil
	}

	// 安全检查
	if err := t.checkCommand(command); err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	// 获取超时时间
	timeout := t.timeout
	if timeoutSec, ok := params["timeout"].(float64); ok && timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}

	// 执行命令
	output, err := t.runCommand(command, timeout)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(output), nil
}

// checkCommand 检查命令是否允许执行
func (t *BashTool) checkCommand(command string) error {
	// 如果有允许列表，只允许列表中的命令
	if len(t.allowList) > 0 {
		allowed := false
		for _, cmd := range t.allowList {
			if containsCommand(command, cmd) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("命令不在允许列表中")
		}
	}

	// 检查禁止列表
	for _, cmd := range t.denyList {
		if containsCommand(command, cmd) {
			return fmt.Errorf("命令 '%s' 不被允许", cmd)
		}
	}

	return nil
}

// containsCommand 检查命令是否包含指定字符串
func containsCommand(command, target string) bool {
	return len(command) >= len(target) &&
		(command == target ||
			(len(command) > len(target) && command[:len(target)+1] == target+" "))
}

// runCommand 执行命令
func (t *BashTool) runCommand(command string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("命令执行超时 (%v)", timeout)
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n[stderr]: " + stderr.String()
	}

	if err != nil {
		return output, fmt.Errorf("命令执行失败: %w", err)
	}

	return output, nil
}
