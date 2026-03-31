// Package tools 提供内置工具实现
package tools

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

const (
	visionDefaultModel   = "gpt-4o-mini"
	visionDefaultTimeout = 60
	visionMaxTokens      = 1000
)

var supportedImageExtensions = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// VisionTool 视觉/图像理解工具
type VisionTool struct {
	apiKey  string
	apiBase string
	model   string
	timeout time.Duration
}

// VisionToolOption 视觉工具配置选项
type VisionToolOption func(*VisionTool)

// WithVisionAPIKey 设置 API 密钥
func WithVisionAPIKey(key string) VisionToolOption {
	return func(t *VisionTool) {
		t.apiKey = key
	}
}

// WithVisionAPIBase 设置 API 基础 URL
func WithVisionAPIBase(base string) VisionToolOption {
	return func(t *VisionTool) {
		t.apiBase = base
	}
}

// WithVisionModel 设置模型
func WithVisionModel(model string) VisionToolOption {
	return func(t *VisionTool) {
		t.model = model
	}
}

// WithVisionTimeout 设置超时时间
func WithVisionTimeout(timeout time.Duration) VisionToolOption {
	return func(t *VisionTool) {
		t.timeout = timeout
	}
}

// NewVisionTool 创建视觉工具实例
func NewVisionTool(opts ...VisionToolOption) *VisionTool {
	t := &VisionTool{
		apiKey:  os.Getenv("OPENAI_API_KEY"),
		apiBase: os.Getenv("OPENAI_API_BASE"),
		model:   visionDefaultModel,
		timeout: visionDefaultTimeout * time.Second,
	}

	if t.apiBase == "" {
		t.apiBase = "https://api.openai.com/v1"
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Name 返回工具名称
func (t *VisionTool) Name() string {
	return "vision"
}

// Description 返回工具描述
func (t *VisionTool) Description() string {
	return "使用视觉API分析本地图片或图片URL。可以描述内容、提取文本、识别对象、颜色等。需要配置OPENAI_API_KEY。"
}

// Parameters 返回参数 JSON Schema
func (t *VisionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"image": map[string]any{
				"type":        "string",
				"description": "要分析的本地文件路径或HTTP(S) URL",
			},
			"question": map[string]any{
				"type":        "string",
				"description": "对图片的问题",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "视觉模型（可选，默认gpt-4o-mini）",
			},
		},
		"required": []string{"image", "question"},
	}
}

// Stage 返回工具执行阶段
func (t *VisionTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 执行工具
func (t *VisionTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	image, _ := params["image"].(string)
	question, _ := params["question"].(string)
	model, _ := params["model"].(string)

	if image == "" {
		return agent.NewErrorToolResult(fmt.Errorf("image 参数是必需的")), nil
	}

	if question == "" {
		return agent.NewErrorToolResult(fmt.Errorf("question 参数是必需的")), nil
	}

	if t.apiKey == "" {
		return agent.NewErrorToolResult(fmt.Errorf("未配置 OPENAI_API_KEY，请使用 env_config 工具配置")), nil
	}

	if model == "" {
		model = t.model
	}

	imageContent, err := t.buildImageContent(image)
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("无法处理图片: %w", err)), nil
	}

	result, err := t.callAPI(model, question, imageContent)
	if err != nil {
		return agent.NewErrorToolResult(err), nil
	}

	return agent.NewToolResult(result), nil
}

func (t *VisionTool) buildImageContent(image string) (map[string]any, error) {
	if strings.HasPrefix(image, "http://") || strings.HasPrefix(image, "https://") {
		return map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": image},
		}, nil
	}

	if _, err := os.Stat(image); os.IsNotExist(err) {
		return nil, fmt.Errorf("图片文件不存在: %s", image)
	}

	ext := strings.ToLower(filepath.Ext(image))
	mimeType, ok := supportedImageExtensions[ext]
	if !ok {
		return nil, fmt.Errorf("不支持的图片格式: %s（支持: jpg, jpeg, png, gif, webp）", ext)
	}

	data, err := os.ReadFile(image)
	if err != nil {
		return nil, fmt.Errorf("无法读取图片: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	return map[string]any{
		"type":      "image_url",
		"image_url": map[string]string{"url": dataURL},
	}, nil
}

func (t *VisionTool) callAPI(model, question string, imageContent map[string]any) (map[string]any, error) {
	payload := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": question},
					imageContent,
				},
			},
		},
		"max_tokens": visionMaxTokens,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("无法构建请求: %w", err)
	}

	client := &http.Client{Timeout: t.timeout}
	apiURL := strings.TrimSuffix(t.apiBase, "/") + "/chat/completions"

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("无法创建请求: %w", err)
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+t.apiKey)
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("无法读取响应: %w", err)
	}

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("API 密钥无效")
	}
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("API 请求频率超限，请稍后重试")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API 返回错误 (HTTP %d): %s", resp.StatusCode, string(respBody)[:min(200, len(respBody))])
	}

	var result struct {
		Error   *struct{ Message string } `json:"error"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("无法解析响应: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("API 错误: %s", result.Error.Message)
	}

	content := ""
	if len(result.Choices) > 0 {
		content = result.Choices[0].Message.Content
	}

	return map[string]any{
		"model":   model,
		"content": content,
		"usage": map[string]int{
			"prompt_tokens":     result.Usage.PromptTokens,
			"completion_tokens": result.Usage.CompletionTokens,
			"total_tokens":      result.Usage.TotalTokens,
		},
	}, nil
}
