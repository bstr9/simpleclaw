// Package tools 提供内置工具实现
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

// APIKeyRegistry 常见环境变量及其描述
var APIKeyRegistry = map[string]string{
	"OPENAI_API_KEY":  "OpenAI API 密钥 (用于GPT模型、Embedding模型)",
	"OPENAI_API_BASE": "OpenAI API 基础URL",
	"GEMINI_API_KEY":  "Google Gemini API 密钥",
	"CLAUDE_API_KEY":  "Claude API 密钥 (用于Claude模型)",
	"LINKAI_API_KEY":  "LinkAI智能体平台 API 密钥，支持多种模型切换",
	"BOCHA_API_KEY":   "博查 AI 搜索 API 密钥",
}

// EnvConfigTool 环境配置工具，用于管理API密钥和环境变量
type EnvConfigTool struct {
	envDir  string
	envPath string
}

// NewEnvConfigTool 创建环境配置工具实例
func NewEnvConfigTool() *EnvConfigTool {
	homeDir, _ := os.UserHomeDir()
	envDir := filepath.Join(homeDir, ".cow")
	return &EnvConfigTool{
		envDir:  envDir,
		envPath: filepath.Join(envDir, ".env"),
	}
}

// Name 返回工具名称
func (t *EnvConfigTool) Name() string {
	return "env_config"
}

// Description 返回工具描述
func (t *EnvConfigTool) Description() string {
	return "管理 API 密钥和环境变量配置。支持设置、获取、列出、删除环境变量。配置会自动热加载生效。"
}

// Parameters 返回参数 JSON Schema
func (t *EnvConfigTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "操作类型: set(设置), get(获取), list(列出), delete(删除)",
				"enum":        []string{"set", "get", "list", "delete"},
			},
			"key": map[string]any{
				"type":        "string",
				"description": "环境变量名称（如 OPENAI_API_KEY, CLAUDE_API_KEY 等）",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "要设置的值（用于 set 操作）",
			},
		},
		"required": []string{"action"},
	}
}

// Stage 返回工具执行阶段
func (t *EnvConfigTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 执行工具
func (t *EnvConfigTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return agent.NewErrorToolResult(fmt.Errorf("action 参数是必需的")), nil
	}

	t.ensureEnvFile()

	switch action {
	case "set":
		return t.handleSet(params)
	case "get":
		return t.handleGet(params)
	case "list":
		return t.handleList()
	case "delete":
		return t.handleDelete(params)
	default:
		return agent.NewErrorToolResult(fmt.Errorf("不支持的操作: %s", action)), nil
	}
}

func (t *EnvConfigTool) ensureEnvFile() {
	os.MkdirAll(t.envDir, 0755)
	if _, err := os.Stat(t.envPath); os.IsNotExist(err) {
		os.WriteFile(t.envPath, []byte("# Environment variables for agent\n"), 0644)
	}
}

func (t *EnvConfigTool) handleSet(params map[string]any) (*agent.ToolResult, error) {
	key, _ := params["key"].(string)
	value, _ := params["value"].(string)

	if key == "" || value == "" {
		return agent.NewErrorToolResult(fmt.Errorf("set 操作需要 key 和 value 参数")), nil
	}

	envVars := t.readEnvFile()
	envVars[key] = value
	t.writeEnvFile(envVars)
	os.Setenv(key, value)

	return agent.NewToolResult(map[string]any{
		"message": fmt.Sprintf("成功设置 %s", key),
		"key":     key,
		"value":   t.maskValue(value),
		"note":    "配置已保存并立即生效",
	}), nil
}

func (t *EnvConfigTool) handleGet(params map[string]any) (*agent.ToolResult, error) {
	key, _ := params["key"].(string)
	if key == "" {
		return agent.NewErrorToolResult(fmt.Errorf("get 操作需要 key 参数")), nil
	}

	envVars := t.readEnvFile()
	value, exists := envVars[key]
	if !exists {
		value = os.Getenv(key)
		if value != "" {
			exists = true
		}
	}

	description := APIKeyRegistry[key]
	if description == "" {
		description = "未知用途的环境变量"
	}

	if exists {
		return agent.NewToolResult(map[string]any{
			"key":         key,
			"value":       t.maskValue(value),
			"description": description,
			"exists":      true,
		}), nil
	}

	return agent.NewToolResult(map[string]any{
		"key":         key,
		"description": description,
		"exists":      false,
		"message":     fmt.Sprintf("环境变量 '%s' 未设置", key),
	}), nil
}

func (t *EnvConfigTool) handleList() (*agent.ToolResult, error) {
	envVars := t.readEnvFile()

	if len(envVars) == 0 {
		return agent.NewToolResult(map[string]any{
			"message":   "暂无已配置的环境变量",
			"variables": map[string]any{},
			"note":      "可以通过 env_config(action='set', key='KEY_NAME', value='your-key') 来配置",
		}), nil
	}

	variables := make(map[string]any)
	for key, value := range envVars {
		description := APIKeyRegistry[key]
		if description == "" {
			description = "未知用途的环境变量"
		}
		variables[key] = map[string]any{
			"value":       t.maskValue(value),
			"description": description,
		}
	}

	return agent.NewToolResult(map[string]any{
		"message":   fmt.Sprintf("发现 %d 个环境变量", len(envVars)),
		"variables": variables,
	}), nil
}

func (t *EnvConfigTool) handleDelete(params map[string]any) (*agent.ToolResult, error) {
	key, _ := params["key"].(string)
	if key == "" {
		return agent.NewErrorToolResult(fmt.Errorf("delete 操作需要 key 参数")), nil
	}

	envVars := t.readEnvFile()
	if _, exists := envVars[key]; !exists {
		return agent.NewToolResult(map[string]any{
			"message": fmt.Sprintf("环境变量 '%s' 未设置", key),
			"key":     key,
		}), nil
	}

	delete(envVars, key)
	t.writeEnvFile(envVars)
	os.Unsetenv(key)

	return agent.NewToolResult(map[string]any{
		"message": fmt.Sprintf("成功删除 %s", key),
		"key":     key,
		"note":    "配置已立即生效",
	}), nil
}

func (t *EnvConfigTool) readEnvFile() map[string]string {
	envVars := make(map[string]string)
	data, err := os.ReadFile(t.envPath)
	if err != nil {
		return envVars
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			envVars[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return envVars
}

func (t *EnvConfigTool) writeEnvFile(envVars map[string]string) {
	var lines []string
	lines = append(lines, "# Environment variables for agent")
	lines = append(lines, "# Auto-managed by env_config tool")
	lines = append(lines, "")

	var keys []string
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, envVars[k]))
	}

	os.WriteFile(t.envPath, []byte(strings.Join(lines, "\n")), 0644)
}

func (t *EnvConfigTool) maskValue(value string) string {
	if len(value) <= 10 {
		return "***"
	}
	return value[:6] + "***" + value[len(value)-4:]
}
