package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	"github.com/coder/websocket"
	"go.uber.org/zap"
)

const (
	apiOpenConnection    = "https://api.dingtalk.com/v1.0/gateway/connections/open"
	apiAccessToken       = "https://api.dingtalk.com/v1.0/oauth2/accessToken"
	apiSendGroupMessage  = "https://api.dingtalk.com/v1.0/robot/groupMessages/send"
	apiSendSingleMessage = "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend"
	apiMediaUpload       = "https://oapi.dingtalk.com/media/upload"
	apiDownloadFile      = "https://api.dingtalk.com/v1.0/robot/messageFiles/download"

	// 超时设置
	defaultTimeout    = 10 * time.Second
	uploadTimeout     = 60 * time.Second
	reconnectInterval = 3 * time.Second
	staleMessageAge   = 60 * time.Second

	// 热重载消息年龄检查
	staleMsgThreshold = 60 * time.Second

	// WebSocket 关闭原因
	closeReasonSessionEnded = "session ended"
)

// Config 定义钉钉渠道配置
type Config struct {
	ClientID     string `json:"client_id" mapstructure:"dingtalk_client_id"`
	ClientSecret string `json:"client_secret" mapstructure:"dingtalk_client_secret"`
	RobotCode    string `json:"robot_code" mapstructure:"dingtalk_robot_code"`
	Workspace    string `json:"workspace" mapstructure:"agent_workspace"`
	CardEnabled  bool   `json:"card_enabled" mapstructure:"dingtalk_card_enabled"`
}

// DingtalkChannel 实现钉钉渠道，支持 Stream 模式
type DingtalkChannel struct {
	*channel.BaseChannel

	config *Config

	// 访问令牌管理
	accessToken    string
	tokenExpiresAt time.Time
	tokenMu        sync.RWMutex

	// Stream 连接
	conn      *websocket.Conn
	running   bool
	runningMu sync.RWMutex
	stopChan  chan struct{}

	// 消息去重
	receivedMsgs map[string]time.Time
	msgsMu       sync.RWMutex

	// 机器人代码缓存
	robotCode string

	// 消息处理器
	messageHandler MessageHandler
}

// MessageHandler 处理传入消息
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *DingTalkMessage) (*types.Reply, error)
}

// NewDingtalkChannel 创建新的钉钉渠道实例
func NewDingtalkChannel(cfg *Config) *DingtalkChannel {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.Workspace == "" {
		cfg.Workspace = "~/cow"
	}

	return &DingtalkChannel{
		BaseChannel:  channel.NewBaseChannel("dingtalk"),
		config:       cfg,
		stopChan:     make(chan struct{}),
		receivedMsgs: make(map[string]time.Time),
	}
}

// SetMessageHandler 设置消息处理器
func (d *DingtalkChannel) SetMessageHandler(handler any) {
	if h, ok := handler.(MessageHandler); ok {
		d.messageHandler = h
	}
}

// Startup 启动钉钉 Stream 连接
func (d *DingtalkChannel) Startup(ctx context.Context) error {
	if d.config.ClientID == "" || d.config.ClientSecret == "" {
		return fmt.Errorf("dingtalk client_id and client_secret are required")
	}

	d.runningMu.Lock()
	d.running = true
	d.runningMu.Unlock()

	go d.runStreamLoop(ctx)

	logger.Info("[DingTalk] Channel started, connecting to stream...")
	return nil
}

// Stop 停止钉钉渠道
func (d *DingtalkChannel) Stop() error {
	d.runningMu.Lock()
	d.running = false
	d.runningMu.Unlock()

	close(d.stopChan)

	if d.conn != nil {
		d.conn.Close(websocket.StatusNormalClosure, "channel stopped")
	}

	d.SetStarted(false)
	logger.Info("[DingTalk] Channel stopped")
	return nil
}

// Send 发送回复消息
func (d *DingtalkChannel) Send(reply *types.Reply, ctx *types.Context) error {
	if reply == nil {
		return nil
	}

	// 从上下文获取接收者和消息信息
	receiverI, hasReceiver := ctx.Get("receiver")
	if !hasReceiver {
		return fmt.Errorf("no receiver in context")
	}
	receiver, ok := receiverI.(string)
	if !ok {
		return fmt.Errorf("receiver is not a string")
	}

	isGroup, _ := ctx.GetBool("isgroup")
	robotCode := d.robotCode
	if rc, ok := ctx.GetString("robot_code"); ok && rc != "" {
		robotCode = rc
	}

	// 处理不同的回复类型
	switch reply.Type {
	case types.ReplyText:
		return d.sendTextMessage(receiver, reply.StringContent(), robotCode, isGroup)
	case types.ReplyImageURL:
		return d.sendImageFromURL(receiver, reply.StringContent(), robotCode, isGroup)
	case types.ReplyImage:
		return d.sendImageFromPath(receiver, reply.StringContent(), robotCode, isGroup)
	case types.ReplyFile:
		return d.sendFileMessage(receiver, reply.StringContent(), robotCode, isGroup)
	default:
		// 默认发送文本消息
		return d.sendTextMessage(receiver, reply.StringContent(), robotCode, isGroup)
	}
}

// runStreamLoop 运行主要的 Stream 连接循环
func (d *DingtalkChannel) runStreamLoop(ctx context.Context) {
	firstConnect := true

	for d.isRunning() {
		// 打开连接
		conn, ticket, err := d.openConnection()
		if err != nil {
			if firstConnect {
				logger.Error("[DingTalk] Failed to open connection", zap.Error(err))
				d.ReportStartupError(err)
				firstConnect = false
			} else {
				logger.Warn("[DingTalk] Connection failed, retrying...", zap.Error(err))
			}

			// 重试前等待
			select {
			case <-time.After(reconnectInterval):
				continue
			case <-d.stopChan:
				return
			}
		}

		if firstConnect {
			logger.Info("[DingTalk] Connected to DingTalk stream")
			d.ReportStartupSuccess()
			firstConnect = false
		} else {
			logger.Info("[DingTalk] Reconnected to DingTalk stream")
		}

		// 运行 WebSocket 会话
		err = d.runWebSocketSession(ctx, conn, ticket)
		if err != nil {
			logger.Warn("[DingTalk] WebSocket session error", zap.Error(err))
		}

		// 清理连接
		if d.conn != nil {
			d.conn.Close(websocket.StatusNormalClosure, closeReasonSessionEnded)
			d.conn = nil
		}
	}
}

// openConnection 打开 Stream 连接并返回端点和票据
func (d *DingtalkChannel) openConnection() (endpoint, ticket string, err error) {
	payload := map[string]any{
		"clientId":     d.config.ClientID,
		"clientSecret": d.config.ClientSecret,
		"subscriptions": []map[string]string{
			{"type": "CALLBACK", "topic": "chatbot_message"},
		},
		"ua":      "simpleclaw-go",
		"localIp": "",
	}

	resp, err := d.doPost(apiOpenConnection, payload, nil)
	if err != nil {
		return "", "", fmt.Errorf("open connection request failed: %w", err)
	}

	endpoint, _ = resp["endpoint"].(string)
	ticket, _ = resp["ticket"].(string)

	if endpoint == "" || ticket == "" {
		return "", "", fmt.Errorf("invalid connection response: missing endpoint or ticket")
	}

	return endpoint, ticket, nil
}

// runWebSocketSession 运行 WebSocket 会话
func (d *DingtalkChannel) runWebSocketSession(ctx context.Context, endpoint, ticket string) error {
	uri := fmt.Sprintf("%s?ticket=%s", endpoint, url.QueryEscape(ticket))

	conn, _, err := websocket.Dial(ctx, uri, nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}
	d.conn = conn

	defer conn.Close(websocket.StatusNormalClosure, closeReasonSessionEnded)

	for d.isRunning() {
		_, msg, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return nil
			}
			return fmt.Errorf("websocket read error: %w", err)
		}

		// 处理消息
		go d.handleStreamMessage(ctx, msg)
	}

	return nil
}

// handleStreamMessage 处理传入的 Stream 消息
func (d *DingtalkChannel) handleStreamMessage(ctx context.Context, data []byte) {
	var callback StreamCallback
	if err := json.Unmarshal(data, &callback); err != nil {
		logger.Error("[DingTalk] Failed to parse callback message", zap.Error(err))
		return
	}

	var incoming IncomingMessage
	if err := json.Unmarshal(callback.Data, &incoming); err != nil {
		logger.Error("[DingTalk] Failed to parse incoming message", zap.Error(err))
		return
	}

	if !d.validateMessage(&incoming) {
		return
	}

	msg := NewDingTalkMessage(&incoming)

	logger.Debug("[DingTalk] Received message",
		zap.String("msg_id", incoming.MessageID),
		zap.String("type", string(incoming.MessageType)),
		zap.Bool("is_group", msg.IsGroup()))

	d.processAndReply(ctx, msg, &incoming)
}

// validateMessage 验证消息是否有效
func (d *DingtalkChannel) validateMessage(incoming *IncomingMessage) bool {
	if incoming.RobotCode != "" {
		d.robotCode = incoming.RobotCode
	}

	if incoming.CreateAt > 0 {
		msgTime := time.UnixMilli(incoming.CreateAt)
		if time.Since(msgTime) > staleMsgThreshold {
			logger.Debug("[DingTalk] Skipping stale message",
				zap.String("msg_id", incoming.MessageID),
				zap.Duration("age", time.Since(msgTime)))
			return false
		}
	}

	if d.isDuplicateMessage(incoming.MessageID) {
		logger.Debug("[DingTalk] Skipping duplicate message",
			zap.String("msg_id", incoming.MessageID))
		return false
	}

	return true
}

// processAndReply 处理消息并发送回复
func (d *DingtalkChannel) processAndReply(ctx context.Context, msg *DingTalkMessage, incoming *IncomingMessage) {
	if d.messageHandler == nil {
		return
	}

	reply, err := d.messageHandler.HandleMessage(ctx, msg)
	if err != nil {
		logger.Error("[DingTalk] Message handler error", zap.Error(err))
		return
	}

	if reply == nil {
		return
	}

	replyCtx := types.NewContext(types.ContextText, reply.StringContent())
	replyCtx.Set("receiver", incoming.SenderStaffID)
	replyCtx.Set("isgroup", msg.IsGroup())
	replyCtx.Set("robot_code", incoming.RobotCode)

	if msg.IsGroup() {
		replyCtx.Set("receiver", incoming.ConversationID)
	}

	if err := d.Send(reply, replyCtx); err != nil {
		logger.Error("[DingTalk] Failed to send reply", zap.Error(err))
	}
}

// isDuplicateMessage 检查消息是否已处理过
func (d *DingtalkChannel) isDuplicateMessage(msgID string) bool {
	d.msgsMu.Lock()
	defer d.msgsMu.Unlock()

	if _, exists := d.receivedMsgs[msgID]; exists {
		return true
	}

	// 清理旧条目
	now := time.Now()
	for id, t := range d.receivedMsgs {
		if now.Sub(t) > 5*time.Minute {
			delete(d.receivedMsgs, id)
		}
	}

	d.receivedMsgs[msgID] = now
	return false
}

// isRunning 检查渠道是否正在运行
func (d *DingtalkChannel) isRunning() bool {
	d.runningMu.RLock()
	defer d.runningMu.RUnlock()
	return d.running
}

// getAccessToken 获取或刷新访问令牌
func (d *DingtalkChannel) getAccessToken() (string, error) {
	d.tokenMu.RLock()
	if d.accessToken != "" && time.Now().Before(d.tokenExpiresAt) {
		token := d.accessToken
		d.tokenMu.RUnlock()
		return token, nil
	}
	d.tokenMu.RUnlock()

	d.tokenMu.Lock()
	defer d.tokenMu.Unlock()

	// 双重检查
	if d.accessToken != "" && time.Now().Before(d.tokenExpiresAt) {
		return d.accessToken, nil
	}

	// 请求新令牌
	payload := map[string]string{
		"appKey":    d.config.ClientID,
		"appSecret": d.config.ClientSecret,
	}

	resp, err := d.doPost(apiAccessToken, payload, nil)
	if err != nil {
		return "", fmt.Errorf("get access token failed: %w", err)
	}

	token, _ := resp["accessToken"].(string)
	expireIn, _ := resp["expireIn"].(float64)

	if token == "" {
		return "", fmt.Errorf("access token not found in response")
	}

	d.accessToken = token
	d.tokenExpiresAt = time.Now().Add(time.Duration(expireIn-300) * time.Second)

	logger.Debug("[DingTalk] Access token refreshed")
	return token, nil
}

// sendTextMessage 发送文本消息
func (d *DingtalkChannel) sendTextMessage(receiver, content, robotCode string, isGroup bool) error {
	if robotCode == "" {
		robotCode = d.robotCode
	}
	if robotCode == "" {
		return fmt.Errorf("robot_code is required")
	}

	accessToken, err := d.getAccessToken()
	if err != nil {
		return fmt.Errorf(common.ErrGetAccessToken, err)
	}

	param := TextMessageParam{Content: content}
	paramBytes, _ := json.Marshal(param)

	payload := map[string]any{
		"msgKey":    "sampleText",
		"msgParam":  string(paramBytes),
		"robotCode": robotCode,
	}

	if isGroup {
		payload["openConversationId"] = receiver
		return d.doSendWithToken(apiSendGroupMessage, payload, accessToken)
	}

	payload["userIds"] = []string{receiver}
	return d.doSendWithToken(apiSendSingleMessage, payload, accessToken)
}

// sendImageFromURL 从 URL 发送图片
func (d *DingtalkChannel) sendImageFromURL(receiver, imageURL, robotCode string, isGroup bool) error {
	// 先下载图片
	tmpPath, err := d.downloadFile(imageURL)
	if err != nil {
		return fmt.Errorf("download image: %w", err)
	}
	defer os.Remove(tmpPath)

	return d.sendImageFromPath(receiver, tmpPath, robotCode, isGroup)
}

// sendImageFromPath 从本地路径发送图片
func (d *DingtalkChannel) sendImageFromPath(receiver, imagePath, robotCode string, isGroup bool) error {
	if robotCode == "" {
		robotCode = d.robotCode
	}

	// 上传媒体文件
	mediaID, err := d.uploadMedia(imagePath, "image")
	if err != nil {
		return fmt.Errorf("upload image: %w", err)
	}

	accessToken, err := d.getAccessToken()
	if err != nil {
		return fmt.Errorf(common.ErrGetAccessToken, err)
	}

	param := ImageMessageParam{MediaID: mediaID}
	paramBytes, _ := json.Marshal(param)

	payload := map[string]any{
		"msgKey":    "sampleImageMsg",
		"msgParam":  string(paramBytes),
		"robotCode": robotCode,
	}

	if isGroup {
		payload["openConversationId"] = receiver
		return d.doSendWithToken(apiSendGroupMessage, payload, accessToken)
	}

	payload["userIds"] = []string{receiver}
	return d.doSendWithToken(apiSendSingleMessage, payload, accessToken)
}

// sendFileMessage 发送文件
func (d *DingtalkChannel) sendFileMessage(receiver, filePath, robotCode string, isGroup bool) error {
	if robotCode == "" {
		robotCode = d.robotCode
	}

	filePath = stripFilePrefix(filePath)
	mediaType := determineMediaType(filePath)

	mediaID, err := d.uploadMedia(filePath, mediaType)
	if err != nil {
		return fmt.Errorf("upload file: %w", err)
	}

	accessToken, err := d.getAccessToken()
	if err != nil {
		return fmt.Errorf(common.ErrGetAccessToken, err)
	}

	msgKey, param := buildFileMessageParams(mediaID, filePath, mediaType)
	paramBytes, _ := json.Marshal(param)

	payload := map[string]any{
		"msgKey":    msgKey,
		"msgParam":  string(paramBytes),
		"robotCode": robotCode,
	}

	return d.sendPayloadByType(receiver, payload, accessToken, isGroup)
}

// stripFilePrefix 移除 file:// 前缀
func stripFilePrefix(path string) string {
	if strings.HasPrefix(path, "file://") {
		return path[7:]
	}
	return path
}

// determineMediaType 根据文件扩展名确定媒体类型
func determineMediaType(filePath string) string {
	lowerPath := strings.ToLower(filePath)
	videoExts := []string{".mp4", ".mov", ".avi"}
	for _, ext := range videoExts {
		if strings.HasSuffix(lowerPath, ext) {
			return "video"
		}
	}
	return "file"
}

// buildFileMessageParams 构建文件消息参数
func buildFileMessageParams(mediaID, filePath, mediaType string) (string, map[string]any) {
	fileName := filepath.Base(filePath)
	ext := filepath.Ext(fileName)

	if mediaType == "video" {
		return "sampleVideo", map[string]any{
			"videoMediaId": mediaID,
			"videoType":    strings.TrimPrefix(ext, "."),
			"duration":     "30",
			"height":       "400",
			"width":        "600",
		}
	}

	return "sampleFile", map[string]any{
		"mediaId":  mediaID,
		"fileName": fileName,
		"fileType": strings.TrimPrefix(ext, "."),
	}
}

// sendPayloadByType 根据消息类型发送载荷
func (d *DingtalkChannel) sendPayloadByType(receiver string, payload map[string]any, accessToken string, isGroup bool) error {
	if isGroup {
		payload["openConversationId"] = receiver
		return d.doSendWithToken(apiSendGroupMessage, payload, accessToken)
	}

	payload["userIds"] = []string{receiver}
	return d.doSendWithToken(apiSendSingleMessage, payload, accessToken)
}

// uploadMedia 上传媒体文件到钉钉
func (d *DingtalkChannel) uploadMedia(filePath, mediaType string) (string, error) {
	accessToken, err := d.getAccessToken()
	if err != nil {
		return "", err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	uploadURL := fmt.Sprintf("%s?access_token=%s&type=%s", apiMediaUpload, accessToken, mediaType)

	var body bytes.Buffer
	writer := createMultipartWriter(&body, "media", filepath.Base(filePath), file)

	req, err := http.NewRequest("POST", uploadURL, &body)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: uploadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if errCode, ok := result["errcode"].(float64); ok && errCode != 0 {
		return "", fmt.Errorf("upload failed: %v", result["errmsg"])
	}

	mediaID, _ := result["media_id"].(string)
	return mediaID, nil
}

// multipartWriter 封装 multipart form 写入器
type multipartWriter struct {
	*bytes.Buffer
	contentType string
}

// FormDataContentType 返回 Content-Type
func (w *multipartWriter) FormDataContentType() string {
	return w.contentType
}

// createMultipartWriter 创建 multipart form 数据
func createMultipartWriter(body *bytes.Buffer, fieldName, fileName string, file *os.File) *multipartWriter {
	boundary := fmt.Sprintf("----WebKitFormBoundary%016x", time.Now().UnixNano())
	contentType := fmt.Sprintf("multipart/form-data; boundary=%s", boundary)

	body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	body.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n", fieldName, fileName))
	body.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	io.Copy(body, file)
	body.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary))

	return &multipartWriter{
		Buffer:      body,
		contentType: contentType,
	}
}

// downloadFile 从 URL 下载文件
func (d *DingtalkChannel) downloadFile(fileURL string) (string, error) {
	client := &http.Client{Timeout: uploadTimeout}
	resp, err := client.Get(fileURL)
	if err != nil {
		return "", fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	// 创建临时文件
	tmpDir := d.getTmpDir()
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("create tmp dir: %w", err)
	}

	fileName := filepath.Base(fileURL)
	if idx := strings.Index(fileName, "?"); idx > 0 {
		fileName = fileName[:idx]
	}
	if fileName == "" {
		fileName = fmt.Sprintf("download_%d", time.Now().Unix())
	}

	tmpPath := filepath.Join(tmpDir, fileName)
	file, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("write file: %w", err)
	}

	return tmpPath, nil
}

// getTmpDir 返回临时目录路径
func (d *DingtalkChannel) getTmpDir() string {
	workspace := d.config.Workspace
	if strings.HasPrefix(workspace, "~") {
		home, _ := os.UserHomeDir()
		workspace = filepath.Join(home, workspace[1:])
	}
	return filepath.Join(workspace, "tmp")
}

// doPost 执行 HTTP POST 请求
func (d *DingtalkChannel) doPost(url string, payload any, headers map[string]string) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: defaultTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed: status %d, %v", resp.StatusCode, result)
	}

	return result, nil
}

// doSendWithToken 使用访问令牌发送消息
func (d *DingtalkChannel) doSendWithToken(url string, payload map[string]any, accessToken string) error {
	headers := map[string]string{
		"x-acs-dingtalk-access-token": accessToken,
	}
	_, err := d.doPost(url, payload, headers)
	return err
}
