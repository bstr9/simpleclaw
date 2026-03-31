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

type WechatMPTool struct {
	appID     string
	appSecret string
}

func NewWechatMPTool(appID, appSecret string) *WechatMPTool {
	return &WechatMPTool{
		appID:     appID,
		appSecret: appSecret,
	}
}

func (t *WechatMPTool) Name() string {
	return "wechatmp"
}

func (t *WechatMPTool) Description() string {
	return `微信公众号操作工具。支持发送模板消息、获取用户信息等操作。

操作类型 (action):
- send_template: 发送模板消息
- get_user: 获取用户信息
- get_menu: 获取自定义菜单

参数说明:
- action: 操作类型
- open_id: 用户OpenID
- template_id: 模板ID
- data: 模板数据`
}

func (t *WechatMPTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"send_template", "get_user", "get_menu"},
				"description": "操作类型",
			},
			"open_id": map[string]any{
				"type":        "string",
				"description": "用户OpenID",
			},
			"template_id": map[string]any{
				"type":        "string",
				"description": "模板ID",
			},
			"data": map[string]any{
				"type":        "object",
				"description": "模板数据",
			},
		},
		"required": []string{"action"},
	}
}

func (t *WechatMPTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

func (t *WechatMPTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing action parameter")), nil
	}

	token, err := t.getAccessToken()
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("get access token failed: %w", err)), nil
	}

	switch action {
	case "send_template":
		return t.sendTemplate(params, token)
	case "get_user":
		return t.getUser(params, token)
	case "get_menu":
		return t.getMenu(token)
	default:
		return agent.NewErrorToolResult(fmt.Errorf("unknown action: %s", action)), nil
	}
}

func (t *WechatMPTool) sendTemplate(params map[string]any, token string) (*agent.ToolResult, error) {
	openID, _ := params["open_id"].(string)
	templateID, _ := params["template_id"].(string)
	data, _ := params["data"].(map[string]any)

	if openID == "" || templateID == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing open_id or template_id")), nil
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/template/send?access_token=%s", token)

	body := map[string]any{
		"touser":      openID,
		"template_id": templateID,
		"data":        data,
	}

	resp, err := t.request("POST", url, body)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

func (t *WechatMPTool) getUser(params map[string]any, token string) (*agent.ToolResult, error) {
	openID, _ := params["open_id"].(string)
	if openID == "" {
		return agent.NewErrorToolResult(fmt.Errorf("missing open_id")), nil
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/user/info?access_token=%s&openid=%s", token, openID)

	resp, err := t.request("GET", url, nil)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

func (t *WechatMPTool) getMenu(token string) (*agent.ToolResult, error) {
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/menu/get?access_token=%s", token)

	resp, err := t.request("GET", url, nil)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(resp), nil
}

func (t *WechatMPTool) getAccessToken() (string, error) {
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s", t.appID, t.appSecret)

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

func (t *WechatMPTool) request(method, url string, body any) (string, error) {
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
