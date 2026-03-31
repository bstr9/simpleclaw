// Package tools 提供内置工具实现
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

var (
	imageExtensions    = map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".gif": "image/gif", ".webp": "image/webp", ".bmp": "image/bmp", ".svg": "image/svg+xml"}
	videoExtensions    = map[string]string{".mp4": "video/mp4", ".avi": "video/x-msvideo", ".mov": "video/quicktime", ".mkv": "video/x-matroska", ".webm": "video/webm", ".flv": "video/x-flv"}
	audioExtensions    = map[string]string{".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg", ".m4a": "audio/mp4", ".flac": "audio/flac", ".aac": "audio/aac"}
	documentExtensions = map[string]string{".pdf": "application/pdf", ".doc": "application/msword", ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".xls": "application/vnd.ms-excel", ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".ppt": "application/vnd.ms-powerpoint", ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation", ".txt": "text/plain", ".md": "text/markdown"}
)

// SendTool 发送文件工具
type SendTool struct {
	workingDir string
}

// NewSendTool 创建发送文件工具实例
func NewSendTool(workingDir string) *SendTool {
	return &SendTool{workingDir: workingDir}
}

// Name 返回工具名称
func (t *SendTool) Name() string {
	return "send"
}

// Description 返回工具描述
func (t *SendTool) Description() string {
	return "发送本地文件（图片、视频、音频、文档）给用户。仅用于本地文件路径，URL应直接在文本回复中包含。"
}

// Parameters 返回参数 JSON Schema
func (t *SendTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "要发送的本地文件路径。必须是绝对路径或相对于工作目录的路径。不要传递URL。",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "附带的消息（可选）",
			},
		},
		"required": []string{"path"},
	}
}

// Stage 返回工具执行阶段
func (t *SendTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 执行工具
func (t *SendTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	path, _ := params["path"].(string)
	message, _ := params["message"].(string)

	if path == "" {
		return agent.NewErrorToolResult(fmt.Errorf("path 参数是必需的")), nil
	}

	absolutePath := t.resolvePath(path)

	if _, err := os.Stat(absolutePath); os.IsNotExist(err) {
		return agent.NewErrorToolResult(fmt.Errorf("文件不存在: %s", path)), nil
	}

	info, err := os.Stat(absolutePath)
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("无法访问文件: %w", err)), nil
	}

	ext := strings.ToLower(filepath.Ext(absolutePath))
	fileName := filepath.Base(absolutePath)
	fileSize := info.Size()

	fileType, mimeType := t.getFileType(ext)

	result := map[string]any{
		"type":           "file_to_send",
		"file_type":      fileType,
		"path":           absolutePath,
		"file_name":      fileName,
		"mime_type":      mimeType,
		"size":           fileSize,
		"size_formatted": formatSize(fileSize),
	}

	if message != "" {
		result["message"] = message
	} else {
		result["message"] = fmt.Sprintf("正在发送 %s", fileName)
	}

	return agent.NewToolResult(result), nil
}

func (t *SendTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return filepath.Join(t.workingDir, path)
}

func (t *SendTool) getFileType(ext string) (string, string) {
	if mimeType, ok := imageExtensions[ext]; ok {
		return "image", mimeType
	}
	if mimeType, ok := videoExtensions[ext]; ok {
		return "video", mimeType
	}
	if mimeType, ok := audioExtensions[ext]; ok {
		return "audio", mimeType
	}
	if mimeType, ok := documentExtensions[ext]; ok {
		return "document", mimeType
	}
	return "file", "application/octet-stream"
}
