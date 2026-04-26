// Package linkai 提供 LinkAI 集成插件，支持知识库、Midjourney绘画、文档总结等功能。
// 本文件包含文档总结相关的类型定义、API 客户端方法、SummaryService 及插件处理器。
package linkai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/plugin"
	"github.com/bstr9/simpleclaw/pkg/types"
)

const (
	// API 路径 - 总结相关
	apiSummaryFile       = "/v1/summary/file"
	apiSummaryURL        = "/v1/summary/url"
	apiSummaryChat       = "/v1/summary/chat"
	apiChat              = "/v1/chat"
	defaultLinkAIBaseURL = "https://api.link-ai.tech"
)

// SummaryConfig 文档总结配置
type SummaryConfig struct {
	// Enabled 是否启用总结功能
	Enabled bool `json:"enabled"`
	// GroupEnabled 群聊是否启用
	GroupEnabled bool `json:"group_enabled"`
	// MaxFileSize 最大文件大小(KB)
	MaxFileSize int `json:"max_file_size"`
	// Type 支持的文件类型
	Type []string `json:"type"`
}

// SummaryFileResponse 文件总结响应
type SummaryFileResponse struct {
	Code int `json:"code"`
	Data struct {
		Summary   string `json:"summary"`
		SummaryID string `json:"summary_id"`
		FileID    string `json:"file_id"`
	} `json:"data"`
	Message string `json:"message"`
}

// SummaryURLResponse URL 总结响应
type SummaryURLResponse struct {
	Code int `json:"code"`
	Data struct {
		Summary   string `json:"summary"`
		SummaryID string `json:"summary_id"`
	} `json:"data"`
	Message string `json:"message"`
}

// SummaryChatResponse 总结对话响应
type SummaryChatResponse struct {
	Code int `json:"code"`
	Data struct {
		Questions string `json:"questions"`
		FileID    string `json:"file_id"`
	} `json:"data"`
	Message string `json:"message"`
}

// SummaryFile 调用文件总结 API
func (c *LinkAIClient) SummaryFile(filePath, appCode, sessionID string) (*SummaryFileResponse, error) {
	url := c.baseURL + apiSummaryFile

	// 创建 multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 添加文件
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	// 添加其他字段
	if appCode != "" {
		writer.WriteField("app_code", appCode)
	}
	if sessionID != "" {
		writer.WriteField("session_id", sessionID)
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	// 创建请求
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set(common.HeaderContentType, writer.FormDataContentType())
	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 解析响应
	var result SummaryFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SummaryURL 调用 URL 总结 API
func (c *LinkAIClient) SummaryURL(urlStr, appCode string) (*SummaryURLResponse, error) {
	url := c.baseURL + apiSummaryURL

	body := map[string]string{
		"url":      urlStr,
		"app_code": appCode,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SummaryURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SummaryChat 调用总结对话 API
func (c *LinkAIClient) SummaryChat(summaryID string) (*SummaryChatResponse, error) {
	url := c.baseURL + apiSummaryChat

	body := map[string]string{
		"summary_id": summaryID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SummaryChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Chat 调用对话 API（带 file_id）
func (c *LinkAIClient) Chat(query, fileID string) (string, error) {
	url := c.baseURL + apiChat

	body := map[string]any{
		"query":   query,
		"file_id": fileID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Data    string `json:"data"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Code != 200 {
		return "", fmt.Errorf(errLinkAI, result.Message)
	}

	return result.Data, nil
}

// ============ 总结服务 ============

// SummaryService 总结服务
type SummaryService struct {
	client *LinkAIClient
	config *SummaryConfig
}

// NewSummaryService 创建总结服务
func NewSummaryService(client *LinkAIClient, config *SummaryConfig) *SummaryService {
	return &SummaryService{
		client: client,
		config: config,
	}
}

// SummaryResult 总结结果
type SummaryResult struct {
	Summary   string
	SummaryID string
	FileID    string
}

// ChatResult 对话结果
type ChatResult struct {
	Questions string
	FileID    string
}

// SummaryFile 文件总结
func (s *SummaryService) SummaryFile(filePath, appCode, sessionID string) (*SummaryResult, error) {
	resp, err := s.client.SummaryFile(filePath, appCode, sessionID)
	if err != nil {
		return nil, err
	}

	if resp.Code != 200 {
		return nil, fmt.Errorf(errLinkAI, resp.Message)
	}

	return &SummaryResult{
		Summary:   resp.Data.Summary,
		SummaryID: resp.Data.SummaryID,
		FileID:    resp.Data.FileID,
	}, nil
}

// SummaryURL URL 总结
func (s *SummaryService) SummaryURL(url, appCode string) (*SummaryResult, error) {
	resp, err := s.client.SummaryURL(url, appCode)
	if err != nil {
		return nil, err
	}

	if resp.Code != 200 {
		return nil, fmt.Errorf(errLinkAI, resp.Message)
	}

	return &SummaryResult{
		Summary:   resp.Data.Summary,
		SummaryID: resp.Data.SummaryID,
	}, nil
}

// SummaryChat 总结对话
func (s *SummaryService) SummaryChat(summaryID string) (*ChatResult, error) {
	resp, err := s.client.SummaryChat(summaryID)
	if err != nil {
		return nil, err
	}

	if resp.Code != 200 {
		return nil, fmt.Errorf(errLinkAI, resp.Message)
	}

	return &ChatResult{
		Questions: resp.Data.Questions,
		FileID:    resp.Data.FileID,
	}, nil
}

// CheckFile 检查文件是否符合总结要求
func (s *SummaryService) CheckFile(filePath string) bool {
	// 检查文件大小
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	fileSizeKB := int(fileInfo.Size() / 1024)
	maxSize := s.config.MaxFileSize
	if maxSize == 0 {
		maxSize = 5000 // 默认 5MB
	}
	if fileSizeKB > maxSize || fileSizeKB > 15000 {
		return false
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(filePath))
	ext = strings.TrimPrefix(ext, ".")
	supportedTypes := []string{"txt", "csv", "docx", "pdf", "md", "jpg", "jpeg", "png", "gif", "webp"}
	for _, t := range supportedTypes {
		if t == ext {
			return true
		}
	}

	return false
}

// CheckURL 检查 URL 是否支持总结
func (s *SummaryService) CheckURL(url string) bool {
	if url == "" {
		return false
	}

	// 支持的 URL 前缀
	supportedPrefixes := []string{
		"http://mp.weixin.qq.com",
		"https://mp.weixin.qq.com",
	}

	// 黑名单 URL 前缀
	blacklistPrefixes := []string{
		"https://mp.weixin.qq.com/mp/waerrpage",
	}

	url = strings.TrimSpace(url)

	// 检查黑名单
	for _, prefix := range blacklistPrefixes {
		if strings.HasPrefix(url, prefix) {
			return false
		}
	}

	// 检查支持列表
	for _, prefix := range supportedPrefixes {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}

	return false
}

// ============ Summary 插件处理器 ============

// handleSummaryFeature 处理总结对话功能，返回是否已处理。
func (p *LinkAIPlugin) handleSummaryFeature(ec *plugin.EventContext, content string) bool {
	userID := p.findUserID(ec)

	// 开启对话
	if content == cmdStartChat {
		sumID := p.userMap.Get(userID + sumIDFlag)
		if sumID != "" {
			p.openSummaryChat(ec, sumID.(string))
			return true
		}
	}

	// 退出对话
	if content == cmdExitChat {
		fileID := p.userMap.Get(userID + fileIDFlag)
		if fileID != nil && fileID != "" {
			p.userMap.Delete(userID + fileIDFlag)
			reply := types.NewInfoReply("对话已退出")
			ec.Set("reply", reply)
			ec.BreakPass("linkai")
			return true
		}
	}

	// 总结对话
	fileID := p.userMap.Get(userID + fileIDFlag)
	if fileID != nil && fileID != "" {
		p.handleSummaryChat(ec, content, fileID.(string))
		return true
	}

	// 检查是否是 URL 需要总结
	if p.summary.CheckURL(content) {
		p.handleURLSummary(ec, content)
		return true
	}

	return false
}

// handleImageMessage 处理图片消息 - 实现图片总结功能
func (p *LinkAIPlugin) handleImageMessage(ec *plugin.EventContext) error {
	// 检查是否启用总结功能
	if !p.isSummaryEnabled(ec) {
		return nil
	}

	// 获取图片路径
	imagePath, ok := ec.GetString("content")
	if !ok || imagePath == "" {
		return nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return nil
	}

	// 检查文件大小和类型
	if !p.summary.CheckFile(imagePath) {
		return nil
	}

	// 获取应用编码
	appCode := p.fetchAppCode(ec)

	// 获取会话 ID
	sessionID := p.getSessionID(ec)

	result, err := p.summary.SummaryFile(imagePath, appCode, sessionID)
	if err != nil {
		return nil
	}

	if result == nil || result.Summary == "" {
		return nil
	}

	// 保存 summary_id 用于后续对话
	userID := p.findUserID(ec)
	if userID != "" && result.SummaryID != "" {
		p.userMap.Set(userID+sumIDFlag, result.SummaryID)
	}

	// 返回总结结果
	reply := types.NewTextReply(result.Summary)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")

	return nil
}

// handleFileMessage 处理文件消息 - 实现文件总结功能
func (p *LinkAIPlugin) handleFileMessage(ec *plugin.EventContext) error {
	// 检查是否启用总结功能
	if !p.isSummaryEnabled(ec) {
		return nil
	}

	// 获取文件路径
	filePath, ok := ec.GetString("content")
	if !ok || filePath == "" {
		return nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}

	// 检查文件大小和类型
	if !p.summary.CheckFile(filePath) {
		return nil
	}

	// 发送处理中提示
	p.sendInfoReply(ec, msgSummaryGenerating)

	// 获取应用编码
	appCode := p.fetchAppCode(ec)

	// 获取会话 ID
	sessionID := p.getSessionID(ec)

	// 调用文件总结 API
	result, err := p.summary.SummaryFile(filePath, appCode, sessionID)
	if err != nil {
		reply := types.NewErrorReply("因为神秘力量无法获取内容，请稍后再试吧")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	if result == nil || result.Summary == "" {
		reply := types.NewErrorReply("无法生成文件摘要")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	// 保存 summary_id 用于后续对话
	userID := p.findUserID(ec)
	if userID != "" && result.SummaryID != "" {
		p.userMap.Set(userID+sumIDFlag, result.SummaryID)
	}

	// 返回总结结果
	summaryText := result.Summary + fmt.Sprintf(msgStartChatHint, "文件")
	reply := types.NewTextReply(summaryText)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")

	// 清理临时文件
	os.Remove(filePath)

	return nil
}

// handleSharingMessage 处理分享消息 - 实现链接总结功能
func (p *LinkAIPlugin) handleSharingMessage(ec *plugin.EventContext) error {
	// 检查是否启用总结功能
	if !p.isSummaryEnabled(ec) {
		return nil
	}

	// 获取分享 URL
	url, ok := ec.GetString("content")
	if !ok || url == "" {
		return nil
	}

	return p.handleURLSummary(ec, url)
}

// handleURLSummary 处理 URL 总结
func (p *LinkAIPlugin) handleURLSummary(ec *plugin.EventContext, url string) error {
	// 解码 URL
	url = html.UnescapeString(url)

	// 检查 URL 是否支持
	if !p.summary.CheckURL(url) {
		return nil
	}

	// 发送处理中提示
	p.sendInfoReply(ec, msgSummaryGenerating)

	// 获取应用编码
	appCode := p.fetchAppCode(ec)

	// 调用 URL 总结 API
	result, err := p.summary.SummaryURL(url, appCode)
	if err != nil {
		reply := types.NewErrorReply("因为神秘力量无法获取文章内容，请稍后再试吧~")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	if result == nil || result.Summary == "" {
		reply := types.NewErrorReply("无法生成文章摘要")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	// 保存 summary_id 用于后续对话
	userID := p.findUserID(ec)
	if userID != "" && result.SummaryID != "" {
		p.userMap.Set(userID+sumIDFlag, result.SummaryID)
	}

	// 返回总结结果
	summaryText := result.Summary + fmt.Sprintf(msgStartChatHint, "文章")
	reply := types.NewTextReply(summaryText)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")

	return nil
}

// openSummaryChat 开启总结对话
func (p *LinkAIPlugin) openSummaryChat(ec *plugin.EventContext, sumID string) error {
	p.sendInfoReply(ec, "正在为你开启对话，请稍后")

	result, err := p.summary.SummaryChat(sumID)
	if err != nil || result == nil {
		reply := types.NewErrorReply("开启对话失败，请稍后再试吧")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	// 保存 file_id 用于后续对话
	userID := p.findUserID(ec)
	if userID != "" && result.FileID != "" {
		p.userMap.Set(userID+fileIDFlag, result.FileID)
	}

	// 返回提示信息
	helpText := "💡你可以问我关于这篇文章的任何问题，例如：\n\n" + result.Questions + "\n\n发送 \"退出对话\" 可以关闭与文章的对话"
	reply := types.NewTextReply(helpText)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")

	return nil
}

// handleSummaryChat 处理总结对话
func (p *LinkAIPlugin) handleSummaryChat(ec *plugin.EventContext, query, fileID string) error {
	// 调用 LinkAI 对话 API
	resp, err := p.client.Chat(query, fileID)
	if err != nil {
		reply := types.NewErrorReply("对话失败: " + err.Error())
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	reply := types.NewTextReply(resp)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")

	return nil
}

// handleAdminSumCmd 处理 linkai sum 命令
func (p *LinkAIPlugin) handleAdminSumCmd(ec *plugin.EventContext, parts []string) error {
	isAdmin, _ := ec.GetBool("is_admin")

	if len(parts) < 3 {
		reply := types.NewErrorReply(msgOpenOrCloseRequired)
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}
	if !isAdmin {
		reply := types.NewErrorReply(errAdminReq)
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}
	action := actionOpen
	if parts[2] == "close" {
		action = actionClose
	}
	p.mu.Lock()
	p.config.Summary.Enabled = parts[2] == "open"
	p.mu.Unlock()
	reply := types.NewInfoReply("文章总结功能" + action)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")
	return nil
}

// isSummaryEnabled 检查总结功能是否启用
func (p *LinkAIPlugin) isSummaryEnabled(ec *plugin.EventContext) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.config.Summary.Enabled {
		return false
	}

	isGroup, _ := ec.GetBool("is_group")
	if isGroup && !p.config.Summary.GroupEnabled {
		return false
	}

	return true
}
