// Package tools 提供内置工具实现
package tools

import (
	"fmt"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

// BrowserTool 浏览器工具，用于网页交互和自动化
type BrowserTool struct {
	headless bool
	timeout  int
}

// BrowserToolOption 浏览器工具配置选项
type BrowserToolOption func(*BrowserTool)

// WithBrowserHeadless 设置是否使用无头模式
func WithBrowserHeadless(headless bool) BrowserToolOption {
	return func(t *BrowserTool) {
		t.headless = headless
	}
}

// WithBrowserTimeout 设置超时时间（秒）
func WithBrowserTimeout(timeout int) BrowserToolOption {
	return func(t *BrowserTool) {
		t.timeout = timeout
	}
}

// NewBrowserTool 创建浏览器工具实例
func NewBrowserTool(opts ...BrowserToolOption) *BrowserTool {
	t := &BrowserTool{
		headless: true,
		timeout:  60,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Name 返回工具名称
func (t *BrowserTool) Name() string {
	return "browser"
}

// Description 返回工具描述
func (t *BrowserTool) Description() string {
	return "浏览器自动化工具，用于网页交互、截图、提取内容等操作。支持导航、点击、输入、截图、获取内容等功能。"
}

// Parameters 返回参数 JSON Schema
func (t *BrowserTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "操作类型: navigate(导航到URL), click(点击元素), type(输入文本), screenshot(截图), content(获取页面内容), wait(等待元素)",
				"enum":        []string{"navigate", "click", "type", "screenshot", "content", "wait"},
			},
			"url": map[string]any{
				"type":        "string",
				"description": "要导航的URL（用于navigate操作）",
			},
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS选择器（用于click、type、wait操作）",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "要输入的文本（用于type操作）",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "操作超时时间（秒，可选）",
			},
		},
		"required": []string{"action"},
	}
}

// Stage 返回工具执行阶段
func (t *BrowserTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 执行工具
func (t *BrowserTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return agent.NewErrorToolResult(fmt.Errorf("action 参数是必需的")), nil
	}

	// 浏览器工具需要外部浏览器服务支持
	// 这里返回一个占位结果，实际实现需要集成 Playwright 或类似服务
	switch action {
	case "navigate":
		url, _ := params["url"].(string)
		if url == "" {
			return agent.NewErrorToolResult(fmt.Errorf("navigate 操作需要 url 参数")), nil
		}
		return agent.NewToolResult(map[string]any{
			"message": "浏览器导航功能需要集成外部浏览器服务（如 Playwright）",
			"action":  action,
			"url":     url,
			"hint":    "请确保系统已配置浏览器服务",
		}), nil

	case "click":
		selector, _ := params["selector"].(string)
		if selector == "" {
			return agent.NewErrorToolResult(fmt.Errorf("click 操作需要 selector 参数")), nil
		}
		return agent.NewToolResult(map[string]any{
			"message":  "浏览器点击功能需要集成外部浏览器服务",
			"action":   action,
			"selector": selector,
		}), nil

	case "type":
		selector, _ := params["selector"].(string)
		text, _ := params["text"].(string)
		if selector == "" {
			return agent.NewErrorToolResult(fmt.Errorf("type 操作需要 selector 参数")), nil
		}
		return agent.NewToolResult(map[string]any{
			"message":  "浏览器输入功能需要集成外部浏览器服务",
			"action":   action,
			"selector": selector,
			"text":     text,
		}), nil

	case "screenshot":
		return agent.NewToolResult(map[string]any{
			"message": "浏览器截图功能需要集成外部浏览器服务",
			"action":  action,
			"hint":    "截图将保存到工作目录",
		}), nil

	case "content":
		return agent.NewToolResult(map[string]any{
			"message": "获取页面内容功能需要集成外部浏览器服务",
			"action":  action,
		}), nil

	case "wait":
		selector, _ := params["selector"].(string)
		return agent.NewToolResult(map[string]any{
			"message":  "等待元素功能需要集成外部浏览器服务",
			"action":   action,
			"selector": selector,
		}), nil

	default:
		return agent.NewErrorToolResult(fmt.Errorf("不支持的操作: %s", action)), nil
	}
}
