// Package tools 提供飞书扩展的工具实现。
package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

// FeishuDocTool 飞书文档工具。
type FeishuDocTool struct {
	appID     string
	appSecret string
}

// NewFeishuDocTool 创建飞书文档工具。
func NewFeishuDocTool(appID, appSecret string) *FeishuDocTool {
	return &FeishuDocTool{
		appID:     appID,
		appSecret: appSecret,
	}
}

// Name 返回工具名称。
func (t *FeishuDocTool) Name() string {
	return "feishu_doc"
}

// Description 返回工具描述。
func (t *FeishuDocTool) Description() string {
	return `飞书文档操作工具。支持读取、写入、创建文档等操作。

操作类型 (action):
- read: 读取文档内容
- write: 写入/替换文档内容（Markdown格式）
- append: 追加内容到文档末尾
- create: 创建新文档
- list_blocks: 列出文档所有块（用于读取表格、图片等结构化内容）

参数说明:
- doc_token: 文档token（从URL中提取，如 https://xxx.feishu.cn/docx/ABC123 的 token 为 ABC123）
- action: 操作类型
- content: 写入内容（Markdown格式）
- title: 文档标题（创建时使用）

示例:
读取文档: {"action": "read", "doc_token": "ABC123"}
写入文档: {"action": "write", "doc_token": "ABC123", "content": "# 标题\n内容"}
创建文档: {"action": "create", "title": "新文档"}`
}

// Parameters 返回工具参数定义。
func (t *FeishuDocTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"read", "write", "append", "create", "list_blocks"},
				"description": "操作类型",
			},
			"doc_token": map[string]any{
				"type":        "string",
				"description": "文档token",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "写入内容（Markdown格式）",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "文档标题（创建时使用）",
			},
		},
		"required": []string{"action"},
	}
}

// Stage 返回工具执行阶段。
func (t *FeishuDocTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 执行工具。
func (t *FeishuDocTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing action parameter")), nil
	}

	token, err := t.getAccessToken()
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("get access token failed: %w", err)), nil
	}

	switch action {
	case "read":
		return t.doRead(params, token)
	case "write":
		return t.doWrite(params, token)
	case "append":
		return t.doAppend(params, token)
	case "create":
		return t.doCreate(params, token)
	case "list_blocks":
		return t.doListBlocks(params, token)
	default:
		return agent.NewErrorToolResult(fmt.Errorf("unknown action: %s", action)), nil
	}
}

// doRead 读取文档。
func (t *FeishuDocTool) doRead(params map[string]any, token string) (*agent.ToolResult, error) {
	docToken, _ := params["doc_token"].(string)
	if docToken == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing doc_token")), nil
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/docx/v1/documents/%s?document_revision_id=-1", docToken)

	resp, err := t.request("GET", url, token, nil)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

// doWrite 写入文档。
func (t *FeishuDocTool) doWrite(params map[string]any, token string) (*agent.ToolResult, error) {
	docToken, _ := params["doc_token"].(string)
	if docToken == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing doc_token")), nil
	}

	content, _ := params["content"].(string)
	if content == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing content")), nil
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/docx/v1/documents/%s/blocks/batch_update", docToken)

	body := map[string]any{
		"requests": []map[string]any{
			{
				"request_type": "InsertTextRequest",
				"insert_text": map[string]any{
					"text": content,
				},
			},
		},
	}

	resp, err := t.request("POST", url, token, body)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

// doAppend 追加内容。
func (t *FeishuDocTool) doAppend(params map[string]any, token string) (*agent.ToolResult, error) {
	docToken, _ := params["doc_token"].(string)
	if docToken == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing doc_token")), nil
	}

	content, _ := params["content"].(string)
	if content == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing content")), nil
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/docx/v1/documents/%s/blocks/batch_update", docToken)

	body := map[string]any{
		"requests": []map[string]any{
			{
				"request_type": "InsertTextRequest",
				"insert_text": map[string]any{
					"text":      content,
					"append_at": map[string]any{"location": "end"},
				},
			},
		},
	}

	resp, err := t.request("POST", url, token, body)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

// doCreate 创建文档。
func (t *FeishuDocTool) doCreate(params map[string]any, token string) (*agent.ToolResult, error) {
	title, _ := params["title"].(string)
	if title == "" {
		title = "新文档"
	}

	url := "https://open.feishu.cn/open-apis/docx/v1/documents"

	body := map[string]any{
		"document": map[string]any{
			"title": title,
		},
	}

	resp, err := t.request("POST", url, token, body)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

// doListBlocks 列出块。
func (t *FeishuDocTool) doListBlocks(params map[string]any, token string) (*agent.ToolResult, error) {
	docToken, _ := params["doc_token"].(string)
	if docToken == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing doc_token")), nil
	}

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/docx/v1/documents/%s/blocks?document_revision_id=-1&page_size=50", docToken)

	resp, err := t.request("GET", url, token, nil)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

// getAccessToken 获取访问令牌。
func (t *FeishuDocTool) getAccessToken() (string, error) {
	url := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal/"

	body := map[string]string{
		"app_id":     t.appID,
		"app_secret": t.appSecret,
	}

	resp, err := t.request("POST", url, "", body)
	if err != nil {
		return "", err
	}

	var result struct {
		Code              int    `json:"code"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		return "", fmt.Errorf("auth failed: code=%d", result.Code)
	}

	return result.TenantAccessToken, nil
}

// request 发送请求。
func (t *FeishuDocTool) request(method, url, token string, body any) (string, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return "", err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
