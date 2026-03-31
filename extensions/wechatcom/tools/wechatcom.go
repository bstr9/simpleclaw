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

type WechatComTool struct {
	corpID    string
	appSecret string
}

func NewWechatComTool(corpID, appSecret string) *WechatComTool {
	return &WechatComTool{
		corpID:    corpID,
		appSecret: appSecret,
	}
}

func (t *WechatComTool) Name() string {
	return "wechatcom"
}

func (t *WechatComTool) Description() string {
	return `企业微信操作工具。支持发送消息、获取用户信息等操作。

操作类型 (action):
- send_message: 发送应用消息
- get_user: 获取用户信息
- get_department: 获取部门列表

参数说明:
- action: 操作类型
- user_id: 用户ID
- message: 消息内容
- department_id: 部门ID`
}

func (t *WechatComTool) Parameters() map[string]any {
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

func (t *WechatComTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

func (t *WechatComTool) Execute(params map[string]any) (*agent.ToolResult, error) {
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

func (t *WechatComTool) sendMessage(params map[string]any, token string) (*agent.ToolResult, error) {
	userID, _ := params["user_id"].(string)
	message, _ := params["message"].(string)
	if userID == "" || message == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing user_id or message")), nil
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)

	body := map[string]any{
		"touser":  userID,
		"msgtype": "text",
		"agentid": 0,
		"text": map[string]any{
			"content": message,
		},
	}

	resp, err := t.request("POST", url, body)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

func (t *WechatComTool) getUser(params map[string]any, token string) (*agent.ToolResult, error) {
	userID, _ := params["user_id"].(string)
	if userID == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing user_id")), nil
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/user/get?access_token=%s&userid=%s", token, userID)

	resp, err := t.request("GET", url, nil)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

func (t *WechatComTool) getDepartment(params map[string]any, token string) (*agent.ToolResult, error) {
	deptID := "1"
	if id, ok := params["department_id"].(string); ok && id != "" {
		deptID = id
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/department/list?access_token=%s&id=%s", token, deptID)

	resp, err := t.request("GET", url, nil)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

func (t *WechatComTool) getAccessToken() (string, error) {
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", t.corpID, t.appSecret)

	resp, err := t.request("GET", url, nil)
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

func (t *WechatComTool) request(method, url string, body any) (string, error) {
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
