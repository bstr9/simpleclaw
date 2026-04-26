// Package tools 提供飞书扩展的工具实现。
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/config"
)

// LarkCLITool lark-cli 命令行工具封装。
// 提供对飞书开放平台 API 的完整访问能力。
type LarkCLITool struct {
	timeout    time.Duration
	workingDir string
	installed  bool
	appID      string
	appSecret  string
	cliPath    string
}

// NewLarkCLITool 创建 lark-cli 工具实例。
func NewLarkCLITool(opts ...LarkCLIToolOption) *LarkCLITool {
	t := &LarkCLITool{
		timeout:    60 * time.Second,
		workingDir: "",
		installed:  checkLarkCLIInstalled(),
		cliPath:    defaultCLIPath(),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// defaultCLIPath 返回默认 lark-cli 可执行文件路径。
func defaultCLIPath() string {
	if path := config.Get().LarkCLIPath; path != "" {
		return path
	}
	return "lark-cli"
}

// WithAppCredentials 设置应用凭证（与 channel 共享配置）。
func WithAppCredentials(appID, appSecret string) LarkCLIToolOption {
	return func(t *LarkCLITool) {
		t.appID = appID
		t.appSecret = appSecret
	}
}

// WithCLIPath 设置 lark-cli 可执行文件路径。
func WithCLIPath(path string) LarkCLIToolOption {
	return func(t *LarkCLITool) {
		if path != "" {
			t.cliPath = path
		}
	}
}

// checkLarkCLIInstalled 检测 lark-cli 是否已安装。
func checkLarkCLIInstalled() bool {
	_, err := exec.LookPath(defaultCLIPath())
	return err == nil
}

// checkSkillsInstalled 检测 lark-cli skills 是否已安装。
func checkSkillsInstalled() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	skillsDir := filepath.Join(homeDir, ".agents", "skills")
	if _, err := os.Stat(skillsDir); err != nil {
		return false
	}
	// 检查是否有 lark-shared（基础技能）
	larkShared := filepath.Join(skillsDir, "lark-shared", "SKILL.md")
	if _, err := os.Stat(larkShared); err != nil {
		return false
	}
	return true
}

// InstallNodeJS 自动安装 Node.js（包含 npm 和 npx）。
func InstallNodeJS() error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// macOS: 使用 Homebrew
		if _, err := exec.LookPath("brew"); err == nil {
			cmd = exec.CommandContext(ctx, "brew", "install", "node")
		} else {
			return fmt.Errorf("homebrew 未安装，请先安装 Homebrew: /bin/bash -c \"$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"")
		}
	case "linux":
		// Linux: 检测包管理器
		if _, err := exec.LookPath("apt"); err == nil {
			cmd = exec.CommandContext(ctx, "apt", "install", "-y", "nodejs", "npm")
		} else if _, err := exec.LookPath("yum"); err == nil {
			cmd = exec.CommandContext(ctx, "yum", "install", "-y", "nodejs", "npm")
		} else if _, err := exec.LookPath("dnf"); err == nil {
			cmd = exec.CommandContext(ctx, "dnf", "install", "-y", "nodejs", "npm")
		} else if _, err := exec.LookPath("pacman"); err == nil {
			cmd = exec.CommandContext(ctx, "pacman", "-S", "--noconfirm", "nodejs", "npm")
		} else {
			return fmt.Errorf("不支持的 Linux 发行版，请手动安装 Node.js")
		}
	case "windows":
		// Windows: 使用 winget 或 chocolatey
		if _, err := exec.LookPath("winget"); err == nil {
			cmd = exec.CommandContext(ctx, "winget", "install", "OpenJS.NodeJS.LTS")
		} else if _, err := exec.LookPath("choco"); err == nil {
			cmd = exec.CommandContext(ctx, "choco", "install", "-y", "nodejs")
		} else {
			return fmt.Errorf("请安装 winget 或 chocolatey，或手动从 https://nodejs.org/ 下载安装")
		}
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("安装失败: %w\n%s", err, string(output))
	}

	return nil
}

// InstallLarkCLI 安装 lark-cli。
func InstallLarkCLI() error {
	// 检测 npm 是否可用
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm 未安装，请先安装 Node.js")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npm", "install", "-g", "@larksuite/cli")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("安装 lark-cli 失败: %w\n%s", err, string(output))
	}

	return nil
}

// InstallLarkCLISkills 安装 lark-cli skills。
func InstallLarkCLISkills() error {
	if _, err := exec.LookPath("npx"); err != nil {
		return fmt.Errorf("npx 未安装，请先安装 Node.js")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npx", "skills", "add", "larksuite/cli", "-g", "-y")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("安装 lark-cli skills 失败: %w\n%s", err, string(output))
	}

	return nil
}

// UpdateLarkCLI 更新 lark-cli 到最新版本。
func UpdateLarkCLI() error {
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm 未安装，请先安装 Node.js")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npm", "update", "-g", "@larksuite/cli")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("更新 lark-cli 失败: %w\n%s", err, string(output))
	}

	return nil
}

// UpdateLarkCLISkills 更新 lark-cli skills 到最新版本。
func UpdateLarkCLISkills() error {
	if _, err := exec.LookPath("npx"); err != nil {
		return fmt.Errorf("npx 未安装，请先安装 Node.js")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npx", "skills", "add", "larksuite/cli", "-g", "-y")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("更新 lark-cli skills 失败: %w\n%s", err, string(output))
	}

	return nil
}

// CheckUpdate 检查是否有新版本。
func CheckUpdate() (map[string]string, error) {
	result := map[string]string{
		"lark_cli_current": "unknown",
		"lark_cli_latest":  "unknown",
		"skills_current":   "unknown",
		"update_available": "false",
	}

	// 获取当前 lark-cli 版本
	if checkLarkCLIInstalled() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, defaultCLIPath(), "version")
		output, err := cmd.Output()
		if err == nil {
			result["lark_cli_current"] = strings.TrimSpace(string(output))
		}
	}

	// 获取 npm 上的最新版本
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "npm", "view", "@larksuite/cli", "version")
	output, err := cmd.Output()
	if err == nil {
		result["lark_cli_latest"] = strings.TrimSpace(string(output))
	}

	// 比较版本
	if result["lark_cli_current"] != "unknown" && result["lark_cli_latest"] != "unknown" {
		if result["lark_cli_current"] != result["lark_cli_latest"] {
			result["update_available"] = "true"
		}
	}

	return result, nil
}

// IsInstalled 返回 lark-cli 是否已安装。
func (t *LarkCLITool) IsInstalled() bool {
	return t.installed
}

// Status 返回安装状态信息。
func (t *LarkCLITool) Status() map[string]bool {
	return map[string]bool{
		"npm_installed":         checkNPMInstalled(),
		"lark_cli_installed":    checkLarkCLIInstalled(),
		"lark_skills_installed": checkSkillsInstalled(),
	}
}

// checkNPMInstalled 检测 npm 是否可用。
func checkNPMInstalled() bool {
	_, err := exec.LookPath("npm")
	return err == nil
}

// StatusDetail 返回详细状态信息。
func (t *LarkCLITool) StatusDetail() map[string]string {
	status := map[string]string{}

	// npm 状态
	if checkNPMInstalled() {
		status["npm"] = "已安装"
		// 获取 npm 版本
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "npm", "--version")
		output, err := cmd.Output()
		if err == nil {
			status["npm_version"] = strings.TrimSpace(string(output))
		}
	} else {
		status["npm"] = "未安装"
		status["npm_hint"] = "请安装 Node.js: https://nodejs.org/"
	}

	// lark-cli 状态
	if checkLarkCLIInstalled() {
		status["lark_cli"] = "已安装"
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, defaultCLIPath(), "version")
		output, err := cmd.Output()
		if err == nil {
			status["lark_cli_version"] = strings.TrimSpace(string(output))
		}
	} else {
		status["lark_cli"] = "未安装"
		if checkNPMInstalled() {
			status["lark_cli_hint"] = "请执行 lark_cli 工具的 install 操作"
		}
	}

	// skills 状态
	if checkSkillsInstalled() {
		status["lark_skills"] = "已安装"
	} else {
		status["lark_skills"] = "未安装"
		if checkNPMInstalled() {
			status["lark_skills_hint"] = "请执行 lark_cli 工具的 install 操作"
		}
	}

	return status
}

// LarkCLIToolOption lark-cli 工具配置选项。
type LarkCLIToolOption func(*LarkCLITool)

// Name 返回工具名称。
func (t *LarkCLITool) Name() string {
	return "lark_cli"
}

// Description 返回工具描述。
func (t *LarkCLITool) Description() string {
	return `飞书/Lark CLI 工具封装。通过 lark-cli 访问飞书开放平台 API。

特殊操作 (action):
- install: 自动安装 lark-cli 和 skills（需要 npm）
- update: 更新到最新版本
- check-update: 检查是否有新版本
- status: 查看安装状态

支持的业务域:
- docs: 云文档操作（创建、读取、更新、搜索）
- sheets: 电子表格
- base: 多维表格
- calendar: 日历日程管理
- im: 即时通讯，消息收发
- drive: 云空间文件管理
- task: 任务管理
- contact: 通讯录
- mail: 邮箱
- wiki: 知识库
- vc: 视频会议
- event: 事件订阅

飞书文档操作示例:
- 创建文档: {"command": "docs +create --title \"文档标题\" --markdown \"# 内容\n正文\""}
- 追加内容: {"command": "docs +update --doc \"TOKEN\" --mode append --markdown \"新内容\""}
- 读取文档: {"command": "docs +fetch --doc \"TOKEN\""}
- 搜索文档: {"command": "docs +search --keyword \"关键词\""}

身份说明:
- 默认使用 bot 身份（应用权限）
- 访问用户私有资源时使用 user 身份: {"command": "calendar events list", "as": "user"}
- 注意: 使用 user 身份需要用户先授权 (lark-cli auth login)

其他常用示例:
- 安装: {"action": "install"}
- 更新: {"action": "update"}
- 查看今日日程: {"command": "calendar +agenda"}
- 发送消息: {"command": "im +messages-send --chat-id oc_xxx --text 'Hello'"}`
}

// Parameters 返回参数 JSON Schema。
func (t *LarkCLITool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"install", "update", "check-update", "status"},
				"description": "特殊操作: install(安装), update(更新), check-update(检查更新), status(状态)",
			},
			"command": map[string]any{
				"type":        "string",
				"description": "lark-cli 命令（不含 lark-cli 前缀）",
			},
			"params": map[string]any{
				"type":        "object",
				"description": "URL 查询参数 (JSON 对象)",
			},
			"data": map[string]any{
				"type":        "object",
				"description": "请求体数据 (JSON 对象)",
			},
			"format": map[string]any{
				"type":        "string",
				"enum":        []string{"json", "pretty", "table"},
				"description": "输出格式，默认 json",
			},
			"page_all": map[string]any{
				"type":        "boolean",
				"description": "是否自动分页获取所有结果",
			},
			"dry_run": map[string]any{
				"type":        "boolean",
				"description": "预览请求而不实际执行",
			},
			"as": map[string]any{
				"type":        "string",
				"enum":        []string{"user", "bot"},
				"description": "身份类型：user 或 bot",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "超时时间（秒）",
			},
		},
	}
}

// Stage 返回工具执行阶段。
func (t *LarkCLITool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 执行工具。
func (t *LarkCLITool) Execute(params map[string]any) (*agent.ToolResult, error) {
	// 处理特殊操作
	if action, ok := params["action"].(string); ok && action != "" {
		switch action {
		case "install":
			return t.doInstall()
		case "update":
			return t.doUpdate()
		case "check-update":
			return t.doCheckUpdate()
		case "status":
			return t.doStatus()
		default:
			return agent.NewErrorToolResult(fmt.Errorf("未知操作: %s", action)), nil
		}
	}

	// 处理 lark-cli 命令
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return agent.NewErrorToolResult(fmt.Errorf("command 参数是必需的")), nil
	}

	// 检测是否已安装
	if !t.installed {
		t.installed = checkLarkCLIInstalled()
		if !t.installed {
			return agent.NewErrorToolResult(fmt.Errorf("lark-cli 未安装，请先执行 install 操作")), nil
		}
	}

	// 构建完整命令
	args := buildLarkCLIArgs(params, command)

	// 执行命令
	timeout := t.timeout
	if timeoutSec, ok := params["timeout"].(float64); ok && timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}

	output, err := t.runCommand(args, timeout)
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("命令执行失败: %w\n命令: lark-cli %v", err, args)), nil
	}

	return agent.NewToolResult(output), nil
}

// doInstall 执行安装操作。
func (t *LarkCLITool) doInstall() (*agent.ToolResult, error) {
	var messages []string

	// 检查并安装 npm
	if !checkNPMInstalled() {
		messages = append(messages, "npm 未安装，正在自动安装 Node.js...")
		if err := InstallNodeJS(); err != nil {
			return agent.NewErrorToolResult(fmt.Errorf("自动安装 Node.js 失败: %w\n请手动安装: https://nodejs.org/", err)), nil
		}
		messages = append(messages, "Node.js 安装成功")
	}

	cliInstalled := checkLarkCLIInstalled()
	skillsInstalled := checkSkillsInstalled()

	if cliInstalled && skillsInstalled {
		if len(messages) > 0 {
			return agent.NewToolResult(strings.Join(messages, "\n") + "\nlark-cli 和 skills 已安装"), nil
		}
		return agent.NewToolResult("lark-cli 和 skills 已安装"), nil
	}

	if !cliInstalled {
		messages = append(messages, "正在安装 lark-cli...")
		if err := InstallLarkCLI(); err != nil {
			return agent.NewErrorToolResult(err), nil
		}
		messages = append(messages, "lark-cli 安装成功")
		t.installed = true
	}

	if !skillsInstalled {
		messages = append(messages, "正在安装 lark-cli skills...")
		if err := InstallLarkCLISkills(); err != nil {
			return agent.NewErrorToolResult(err), nil
		}
		messages = append(messages, "lark-cli skills 安装成功")
	}

	return agent.NewToolResult(strings.Join(messages, "\n")), nil
}

// doUpdate 执行更新操作。
func (t *LarkCLITool) doUpdate() (*agent.ToolResult, error) {
	var messages []string

	if !checkLarkCLIInstalled() {
		return agent.NewErrorToolResult(fmt.Errorf("lark-cli 未安装，请先执行 install 操作")), nil
	}

	messages = append(messages, "正在更新 lark-cli...")
	if err := UpdateLarkCLI(); err != nil {
		return agent.NewErrorToolResult(err), nil
	}
	messages = append(messages, "lark-cli 更新成功")

	messages = append(messages, "正在更新 lark-cli skills...")
	if err := UpdateLarkCLISkills(); err != nil {
		return agent.NewErrorToolResult(err), nil
	}
	messages = append(messages, "lark-cli skills 更新成功")

	messages = append(messages, "\n提示: 更新后请重启 Agent 以加载最新版本")

	return agent.NewToolResult(strings.Join(messages, "\n")), nil
}

// doCheckUpdate 检查更新。
func (t *LarkCLITool) doCheckUpdate() (*agent.ToolResult, error) {
	info, err := CheckUpdate()
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("当前版本: %s", info["lark_cli_current"]))
	lines = append(lines, fmt.Sprintf("最新版本: %s", info["lark_cli_latest"]))

	if info["update_available"] == "true" {
		lines = append(lines, "\n有新版本可用，请执行 update 操作进行更新")
	} else {
		lines = append(lines, "\n已是最新版本")
	}

	return agent.NewToolResult(strings.Join(lines, "\n")), nil
}

// doStatus 返回安装状态。
func (t *LarkCLITool) doStatus() (*agent.ToolResult, error) {
	detail := t.StatusDetail()
	var lines []string

	lines = append(lines, fmt.Sprintf("npm: %s", detail["npm"]))
	if detail["npm_version"] != "" {
		lines = append(lines, fmt.Sprintf("  版本: %s", detail["npm_version"]))
	}
	if detail["npm_hint"] != "" {
		lines = append(lines, fmt.Sprintf("  提示: %s", detail["npm_hint"]))
	}

	lines = append(lines, fmt.Sprintf("lark-cli: %s", detail["lark_cli"]))
	if detail["lark_cli_version"] != "" {
		lines = append(lines, fmt.Sprintf("  版本: %s", detail["lark_cli_version"]))
	}
	if detail["lark_cli_hint"] != "" {
		lines = append(lines, fmt.Sprintf("  提示: %s", detail["lark_cli_hint"]))
	}

	lines = append(lines, fmt.Sprintf("lark-skills: %s", detail["lark_skills"]))
	if detail["lark_skills_hint"] != "" {
		lines = append(lines, fmt.Sprintf("  提示: %s", detail["lark_skills_hint"]))
	}

	return agent.NewToolResult(strings.Join(lines, "\n")), nil
}

// buildLarkCLIArgs 构建 lark-cli 命令参数。
func buildLarkCLIArgs(params map[string]any, command string) []string {
	args := []string{}

	// 使用正确的命令行解析（支持引号）
	parts := parseCommand(command)
	args = append(args, parts...)

	// 添加 params
	if p, ok := params["params"].(map[string]any); ok && len(p) > 0 {
		data, _ := json.Marshal(p)
		args = append(args, "--params", string(data))
	}

	// 添加 data
	if d, ok := params["data"].(map[string]any); ok && len(d) > 0 {
		data, _ := json.Marshal(d)
		args = append(args, "--data", string(data))
	}

	// 添加 format
	if format, ok := params["format"].(string); ok && format != "" {
		args = append(args, "--format", format)
	}

	// 添加 page_all
	if pageAll, ok := params["page_all"].(bool); ok && pageAll {
		args = append(args, "--page-all")
	}

	// 添加 dry_run
	if dryRun, ok := params["dry_run"].(bool); ok && dryRun {
		args = append(args, "--dry-run")
	}

	// 添加身份
	if as, ok := params["as"].(string); ok && as != "" {
		args = append(args, "--as", as)
	}

	return args
}

// parseCommand 解析命令字符串，正确处理引号。
func parseCommand(command string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for i, ch := range command {
		switch {
		case ch == '"' || ch == '\'':
			if !inQuote {
				inQuote = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuote = false
				quoteChar = 0
			} else {
				current.WriteRune(ch)
			}
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}

		// 处理转义字符
		_ = i
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// runCommand 执行 lark-cli 命令。
func (t *LarkCLITool) runCommand(args []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.cliPath, args...)

	if t.workingDir != "" {
		cmd.Dir = t.workingDir
	}

	if t.appID != "" && t.appSecret != "" {
		cmd.Env = append(os.Environ(),
			"LARK_APP_ID="+t.appID,
			"LARK_APP_SECRET="+t.appSecret,
		)
	}

	forceUser := false
	for _, arg := range args {
		if arg == "--as" || arg == "user" {
			if arg == "user" {
				forceUser = true
			}
		}
	}

	if forceUser {
		cmd.Args = append([]string{t.cliPath, "--as", "user"}, args[1:]...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("命令执行超时")
		}
		if stderr.Len() > 0 {
			return "", fmt.Errorf("命令执行失败: %s", stderr.String())
		}
		return "", fmt.Errorf("命令执行失败: %w", err)
	}

	result := stdout.String()
	if stderr.Len() > 0 {
		result += "\n[stderr]\n" + stderr.String()
	}

	return result, nil
}
