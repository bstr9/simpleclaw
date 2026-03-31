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

type DingtalkTool struct {
	appKey    string
	appSecret string
}

func NewDingtalkTool(appKey, appSecret string) *DingtalkTool {
	return &DingtalkTool{
		appKey:    appKey,
		appSecret: appSecret,
	}
}

func (t *DingtalkTool) Name() string {
	return "dingtalk"
}

func (t *DingtalkTool) Description() string {
	return `钉钉操作工具。支持发送消息、获取用户信息等操作。

操作类型 (action):
- send_message: 发送工作通知消息
- get_user: 获取用户详细信息
- get_department: 获取部门列表

参数说明:
- action: 操作类型
- user_id: 用户ID（发送消息、获取用户时使用）
- message: 消息内容（发送消息时使用）
- department_id: 部门ID（获取部门时使用）`
}

func (t *DingtalkTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"send_message", "get_user", "get_department"},
				"description": "操作类型",
			},
			"user_id": map[string]any{
				"type":        "string",
				"description": "用户ID",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "消息内容",
			},
			"department_id": map[string]any{
				"type":        "string",
				"description": "部门ID",
			},
		},
		"required": []string{"action"},
	}
}

func (t *DingtalkTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

func (t *DingtalkTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing action parameter")), nil
	}

	token, err := t.getAccessToken()
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("get access token failed: %w", err)), nil
	}

	switch action {
	case "send_message":
		return t.sendMessage(params, token)
	case "get_user":
		return t.getUser(params, token)
	case "get_department":
		return t.getDepartment(params, token)
	default:
		return agent.NewErrorToolResult(fmt.Errorf("unknown action: %s", action)), nil
	}
}

func (t *DingtalkTool) sendMessage(params map[string]any, token string) (*agent.ToolResult, error) {
	userID, _ := params["user_id"].(string)
	message, _ := params["message"].(string)
	if userID == "" || message == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing user_id or message")), nil
	}

	url := fmt.Sprintf("https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2?access_token=%s", token)

	body := map[string]any{
		"agent_id":    0,
		"userid_list": userID,
		"msg": map[string]any{
			"msgtype": "text",
			"text": map[string]any{
				"content": message,
			},
		},
	}

	resp, err := t.request("POST", url, token, body)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

func (t *DingtalkTool) getUser(params map[string]any, token string) (*agent.ToolResult, error) {
	userID, _ := params["user_id"].(string)
	if userID == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing user_id")), nil
	}

	url := fmt.Sprintf("https://oapi.dingtalk.com/topapi/v2/user/get?access_token=%s", token)

	body := map[string]any{
		"userid": userID,
	}

	resp, err := t.request("POST", url, token, body)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

func (t *DingtalkTool) getDepartment(params map[string]any, token string) (*agent.ToolResult, error) {
	deptID := "1"
	if id, ok := params["department_id"].(string); ok && id != "" {
		deptID = id
	}

	url := fmt.Sprintf("https://oapi.dingtalk.com/topapi/v2/department/listsub?access_token=%s", token)

	body := map[string]any{
		"dept_id": deptID,
	}

	resp, err := t.request("POST", url, token, body)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

func (t *DingtalkTool) getAccessToken() (string, error) {
	url := "https://oapi.dingtalk.com/gettoken"

	body := map[string]string{
		"appkey":    t.appKey,
		"appsecret": t.appSecret,
	}

	resp, err := t.request("POST", url, "", body)
	if err != nil {
		return "", err
	}

	var result struct {
		Errcode     int    `json:"errcode"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return "", err
	}

	if result.Errcode != 0 {
		return "", fmt.Errorf("auth failed: code=%d", result.Errcode)
	}

	return result.AccessToken, nil
}

func (t *DingtalkTool) request(method, url, token string, body any) (string, error) {
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
