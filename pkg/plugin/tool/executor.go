// Package tool 提供工具调用插件，支持多种工具来增强 AI 机器人的能力。
// 该文件包含工具执行逻辑，负责工具的调用、LLM 智能选择和顺序尝试。
package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/llm"
)

// 工具常量
const (
	// 重复字符串提取 (SonarQube go:S1192)
	errMsgNoToolHandle   = "暂无工具能够处理该请求"
	errMsgAllToolsFailed = "所有工具都无法处理该请求: %w"
)

// executeTool 执行工具调用。
func (p *ToolPlugin) executeTool(toolName, query string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 如果指定了特定工具
	if toolName != "" {
		tool, ok := p.tools[toolName]
		if !ok {
			return "", fmt.Errorf("工具 '%s' 未找到", toolName)
		}

		toolConfig := p.getToolConfig(toolName)
		return tool.Run(query, toolConfig)
	}

	// 使用 LLM 智能选择工具
	if p.llmModel != nil {
		return p.selectToolWithLLM(query)
	}

	// 如果没有 LLM，按顺序尝试每个工具
	var lastErr error
	for _, name := range p.getLoadedToolNames() {
		tool, ok := p.tools[name]
		if !ok {
			continue
		}

		toolConfig := p.getToolConfig(name)
		result, err := tool.Run(query, toolConfig)
		if err != nil {
			lastErr = err
			continue
		}

		if result != "" {
			return result, nil
		}
	}

	if lastErr != nil {
		return "", fmt.Errorf(errMsgAllToolsFailed, lastErr)
	}

	return errMsgNoToolHandle, nil
}

// selectToolWithLLM 使用 LLM 智能选择工具。
func (p *ToolPlugin) selectToolWithLLM(query string) (string, error) {
	toolNames := p.getLoadedToolNames()
	toolDescriptions := p.buildToolDescriptions(toolNames)

	systemPrompt := p.buildSystemPrompt(toolDescriptions)

	selectedTool, err := p.callLLMForToolSelection(query, systemPrompt)
	if err != nil {
		return "", err
	}

	if result, ok := p.tryToolByName(toolNames, selectedTool, query); ok {
		return result, nil
	}

	return p.tryAllTools(toolNames, query)
}

// buildToolDescriptions 构建工具描述列表。
func (p *ToolPlugin) buildToolDescriptions(toolNames []string) []string {
	descriptions := make([]string, 0, len(toolNames))
	for _, name := range toolNames {
		if tool, ok := p.tools[name]; ok {
			descriptions = append(descriptions, fmt.Sprintf("- %s: %s", name, tool.Description()))
		}
	}
	return descriptions
}

// buildSystemPrompt 构建系统提示词。
func (p *ToolPlugin) buildSystemPrompt(toolDescriptions []string) string {
	return `你是一个工具选择助手。根据用户的查询，选择最合适的工具来处理请求。
可用的工具有：
` + strings.Join(toolDescriptions, "\n") + `

请直接回复工具名称，不要包含其他内容。如果不确定使用哪个工具，回复 "unknown"。`
}

// callLLMForToolSelection 调用 LLM 获取工具选择。
func (p *ToolPlugin) callLLMForToolSelection(query, systemPrompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: query},
	}

	opts := []llm.Option{
		llm.WithTemperature(0),
		llm.WithMaxTokens(50),
	}

	resp, err := p.llmModel.Call(ctx, messages, opts...)
	if err != nil {
		return "", fmt.Errorf("LLM 调用失败: %w", err)
	}

	selectedTool := strings.TrimSpace(strings.ToLower(resp.Content))
	return strings.Trim(selectedTool, "\"'`"), nil
}

// tryToolByName 按名称尝试指定工具。
func (p *ToolPlugin) tryToolByName(toolNames []string, selectedTool, query string) (string, bool) {
	for _, name := range toolNames {
		if strings.ToLower(name) != selectedTool {
			continue
		}

		tool, ok := p.tools[name]
		if !ok {
			continue
		}

		toolConfig := p.getToolConfig(name)
		result, err := tool.Run(query, toolConfig)
		if err != nil {
			return "", false
		}
		return result, true
	}
	return "", false
}

// tryAllTools 按顺序尝试所有工具。
func (p *ToolPlugin) tryAllTools(toolNames []string, query string) (string, error) {
	var lastErr error
	for _, name := range toolNames {
		tool, ok := p.tools[name]
		if !ok {
			continue
		}

		toolConfig := p.getToolConfig(name)
		result, err := tool.Run(query, toolConfig)
		if err != nil {
			lastErr = err
			continue
		}

		if result != "" {
			return result, nil
		}
	}

	if lastErr != nil {
		return "", fmt.Errorf(errMsgAllToolsFailed, lastErr)
	}

	return errMsgNoToolHandle, nil
}

// getToolConfig 获取工具配置。
func (p *ToolPlugin) getToolConfig(toolName string) map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 合并全局配置和工具特定配置
	config := make(map[string]any)

	// 添加全局 kwargs
	for k, v := range p.config.Kwargs {
		config[k] = v
	}

	// 添加全局配置
	config["llm_api_key"] = p.config.LLMAPIKey
	config["llm_api_base"] = p.config.LLMAPIBase
	config["bing_subscription_key"] = p.config.BingSubscriptionKey
	config["bing_search_url"] = p.config.BingSearchURL
	config["google_api_key"] = p.config.GoogleAPIKey
	config["google_cse_id"] = p.config.GoogleCSEID
	config["request_timeout"] = p.config.RequestTimeout
	config["debug"] = p.config.Debug

	// 添加工具特定配置
	if tc, ok := p.config.ToolConfigs[toolName]; ok {
		for k, v := range tc.Config {
			config[k] = v
		}
	}

	return config
}
