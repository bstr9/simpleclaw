// Package wechatcom 提供企业微信 API 客户端实现。
// 该文件处理 API 调用、令牌管理和消息发送。
package wechatcom

import (
	"bytes"
	"github.com/bstr9/simpleclaw/pkg/common"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

const (
	APIBaseURL = "https://qyapi.weixin.qq.com/cgi-bin"

	apiGetAccessToken = "/gettoken"
	apiMessageSend    = "/message/send"
	apiMediaUpload    = "/media/upload"
	apiMediaGet       = "/media/get"

	errAPIFormat       = "API error: code=%d, msg=%s"
	errDecodeFormat    = "failed to decode response: %w"
	errGetTokenFormat  = "failed to get access token: %w"
	errMarshalFormat   = "failed to marshal message: %w"
	errSendMsgFormat   = "failed to send message: %w"
	errOpenFileFormat  = "failed to open file: %w"
	errCreateReqFormat = "failed to create request: %w"
	errUploadFormat    = "failed to upload media: %w"

	// Default timeouts
	defaultAPITimeout = 30 * time.Second
)

// WechatClient 实现企业微信 API 客户端
type WechatClient struct {
	corpID string
	secret string
	client *http.Client

	// Token management
	tokenMu        sync.RWMutex
	accessToken    string
	tokenExpiresAt time.Time

	// Stop channel for refresh goroutine
	stopChan chan struct{}
}

// AccessTokenResponse 表示 gettoken API 的响应
type AccessTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// MessageResponse 表示消息发送 API 的响应
type MessageResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	MsgID       string `json:"msgid"`
	InvalidUser string `json:"invaliduser"`
}

// MediaUploadResponse 表示媒体上传 API 的响应
type MediaUploadResponse struct {
	ErrCode   int    `json:"errcode"`
	ErrMsg    string `json:"errmsg"`
	Type      string `json:"type"`
	MediaID   string `json:"media_id"`
	CreatedAt int64  `json:"created_at"`
}

// ErrorResponse 表示 API 错误
type ErrorResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// NewWechatClient 创建新的企业微信 API 客户端
func NewWechatClient(corpID, secret string) *WechatClient {
	c := &WechatClient{
		corpID: corpID,
		secret: secret,
		client: &http.Client{
			Timeout: defaultAPITimeout,
		},
		stopChan: make(chan struct{}),
	}

	// Start token refresh goroutine
	go c.tokenRefreshLoop()

	return c
}

// Stop 停止客户端和刷新协程
func (c *WechatClient) Stop() {
	close(c.stopChan)
}

// tokenRefreshLoop 定期刷新访问令牌
func (c *WechatClient) tokenRefreshLoop() {
	ticker := time.NewTicker(TokenRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.checkAndRefreshToken()
		}
	}
}

// checkAndRefreshToken 检查令牌是否需要刷新，并在需要时刷新
func (c *WechatClient) checkAndRefreshToken() {
	c.tokenMu.RLock()
	expiresAt := c.tokenExpiresAt
	c.tokenMu.RUnlock()

	// Refresh token 10 minutes before expiration
	if time.Until(expiresAt) < TokenRefreshAhead {
		if err := c.refreshToken(); err != nil {
			logger.Warn("[Wechatcom] Failed to refresh token", zap.Error(err))
		}
	}
}

// refreshToken 从微信 API 获取新的访问令牌
func (c *WechatClient) refreshToken() error {
	url := fmt.Sprintf("%s%s?corpid=%s&corpsecret=%s",
		APIBaseURL, apiGetAccessToken, c.corpID, c.secret)

	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf(errGetTokenFormat, err)
	}
	defer resp.Body.Close()

	var result AccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf(errDecodeFormat, err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf(errAPIFormat, result.ErrCode, result.ErrMsg)
	}

	c.tokenMu.Lock()
	c.accessToken = result.AccessToken
	c.tokenExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	c.tokenMu.Unlock()

	logger.Debug("[Wechatcom] Access token refreshed",
		zap.Int("expires_in", result.ExpiresIn))

	return nil
}

// GetAccessToken 返回当前访问令牌，必要时刷新
func (c *WechatClient) GetAccessToken() (string, error) {
	c.tokenMu.RLock()
	token := c.accessToken
	expiresAt := c.tokenExpiresAt
	c.tokenMu.RUnlock()

	// Check if token is valid (with 1 minute buffer)
	if token != "" && time.Until(expiresAt) > time.Minute {
		return token, nil
	}

	// Need to refresh
	if err := c.refreshToken(); err != nil {
		return "", err
	}

	c.tokenMu.RLock()
	token = c.accessToken
	c.tokenMu.RUnlock()

	return token, nil
}

// SendText 发送文本消息
func (c *WechatClient) SendText(agentID int, toUser, content string) error {
	token, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	msg := map[string]any{
		"touser":  toUser,
		"msgtype": "text",
		"agentid": agentID,
		"text": map[string]string{
			"content": content,
		},
		"safe": 0,
	}

	return c.sendMessage(token, msg)
}

// SendImage 发送图片消息
func (c *WechatClient) SendImage(agentID int, toUser, mediaID string) error {
	token, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	msg := map[string]any{
		"touser":  toUser,
		"msgtype": "image",
		"agentid": agentID,
		"image": map[string]string{
			"media_id": mediaID,
		},
	}

	return c.sendMessage(token, msg)
}

// SendVoice 发送语音消息
func (c *WechatClient) SendVoice(agentID int, toUser, mediaID string) error {
	token, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	msg := map[string]any{
		"touser":  toUser,
		"msgtype": "voice",
		"agentid": agentID,
		"voice": map[string]string{
			"media_id": mediaID,
		},
	}

	return c.sendMessage(token, msg)
}

// SendFile 发送文件消息
func (c *WechatClient) SendFile(agentID int, toUser, mediaID string) error {
	token, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	msg := map[string]any{
		"touser":  toUser,
		"msgtype": "file",
		"agentid": agentID,
		"file": map[string]string{
			"media_id": mediaID,
		},
	}

	return c.sendMessage(token, msg)
}

// sendMessage 通过微信 API 发送消息
func (c *WechatClient) sendMessage(token string, msg map[string]any) error {
	url := fmt.Sprintf("%s%s?access_token=%s", APIBaseURL, apiMessageSend, token)

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf(errMarshalFormat, err)
	}

	resp, err := c.client.Post(url, common.ContentTypeJSON, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf(errSendMsgFormat, err)
	}
	defer resp.Body.Close()

	var result MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf(errDecodeFormat, err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf(errAPIFormat, result.ErrCode, result.ErrMsg)
	}

	return nil
}

// UploadMedia 上传媒体文件到微信
func (c *WechatClient) UploadMedia(mediaType, filePath string) (string, error) {
	token, err := c.GetAccessToken()
	if err != nil {
		return "", err
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf(errOpenFileFormat, err)
	}
	defer file.Close()

	// 创建 multipart 表单
	var buf bytes.Buffer
	writer := newMultipartWriter(&buf, file, "media", filePath)

	url := fmt.Sprintf("%s%s?access_token=%s&type=%s",
		APIBaseURL, apiMediaUpload, token, mediaType)

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return "", fmt.Errorf(errCreateReqFormat, err)
	}
	req.Header.Set(common.HeaderContentType, writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf(errUploadFormat, err)
	}
	defer resp.Body.Close()

	var result MediaUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf(errDecodeFormat, err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf(errAPIFormat, result.ErrCode, result.ErrMsg)
	}

	return result.MediaID, nil
}

// DownloadMedia 从微信下载媒体文件
func (c *WechatClient) DownloadMedia(mediaID string) (*http.Response, error) {
	token, err := c.GetAccessToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s%s?access_token=%s&media_id=%s",
		APIBaseURL, apiMediaGet, token, mediaID)

	return c.client.Get(url)
}

// multipartWriter 辅助创建 multipart 表单数据
type multipartWriter struct {
	*bytes.Buffer
	boundary string
}

func newMultipartWriter(buf *bytes.Buffer, file io.Reader, fieldName, filename string) *multipartWriter {
	boundary := fmt.Sprintf("----WechatFormBoundary%d", time.Now().UnixNano())
	w := &multipartWriter{
		Buffer:   buf,
		boundary: boundary,
	}

	// Write form header
	fmt.Fprintf(buf, "--%s\r\n", boundary)
	fmt.Fprintf(buf, "Content-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n", fieldName, filename)
	fmt.Fprintf(buf, "Content-Type: application/octet-stream\r\n\r\n")

	// Write file content
	io.Copy(buf, file)

	// Write form footer
	fmt.Fprintf(buf, "\r\n--%s--\r\n", boundary)

	return w
}

func (w *multipartWriter) FormDataContentType() string {
	return fmt.Sprintf("multipart/form-data; boundary=%s", w.boundary)
}
