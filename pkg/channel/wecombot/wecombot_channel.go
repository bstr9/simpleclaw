// Package wecombot 提供企业微信机器人渠道实现。
// wecombot_channel.go 实现企业微信机器人渠道，通过 WebSocket 长连接接收和发送消息。
package wecombot

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	"github.com/coder/websocket"
	"go.uber.org/zap"
)

const (
	// WebSocket 服务地址
	wsURL = "wss://openws.work.weixin.qq.com"

	// 心跳间隔
	heartbeatInterval = 30 * time.Second

	// 媒体上传分块大小（512KB）
	mediaChunkSize = 512 * 1024

	// 重连间隔
	reconnectInterval = 5 * time.Second

	// 消息过期时间
	messageExpireTime = 7 * time.Hour

	// 上传超时
	uploadTimeout = 30 * time.Second
)

// Config 企业微信机器人渠道配置
type Config struct {
	BotID     string `json:"bot_id" mapstructure:"wecom_bot_id"`
	BotSecret string `json:"bot_secret" mapstructure:"wecom_bot_secret"`
	Workspace string `json:"workspace" mapstructure:"agent_workspace"`
}

// WecomBotChannel 企业微信机器人渠道
type WecomBotChannel struct {
	*channel.BaseChannel

	config *Config

	// WebSocket 连接
	conn    *websocket.Conn
	running bool
	connMu  sync.RWMutex
	stopCh  chan struct{}

	// 消息去重
	receivedMsgs map[string]time.Time
	msgsMu       sync.RWMutex

	// 请求响应映射
	pendingRequests map[string]chan *WSMessage
	pendingMu       sync.Mutex

	// 流式消息状态
	streamStates map[string]*streamState
	streamMu     sync.Mutex

	// 消息处理器
	messageHandler MessageHandler

	// 临时目录
	tmpDir string
}

// streamState 流式消息状态
type streamState struct {
	streamID string
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *WecomBotMessage) (*types.Reply, error)
}

// MessageHandlerFunc 消息处理函数类型
type MessageHandlerFunc func(ctx context.Context, msg *WecomBotMessage) (*types.Reply, error)

// HandleMessage 实现 MessageHandler 接口
func (f MessageHandlerFunc) HandleMessage(ctx context.Context, msg *WecomBotMessage) (*types.Reply, error) {
	return f(ctx, msg)
}

// NewWecomBotChannel 创建企业微信机器人渠道实例
func NewWecomBotChannel(cfg *Config) *WecomBotChannel {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.Workspace == "" {
		cfg.Workspace = "~/cow"
	}

	// 展开工作目录路径
	workspace := cfg.Workspace
	if strings.HasPrefix(workspace, "~") {
		home, _ := os.UserHomeDir()
		workspace = filepath.Join(home, workspace[1:])
	}

	tmpDir := filepath.Join(workspace, "tmp")

	return &WecomBotChannel{
		BaseChannel:     channel.NewBaseChannel(channel.ChannelWecomBot),
		config:          cfg,
		stopCh:          make(chan struct{}),
		receivedMsgs:    make(map[string]time.Time),
		pendingRequests: make(map[string]chan *WSMessage),
		streamStates:    make(map[string]*streamState),
		tmpDir:          tmpDir,
	}
}

// SetMessageHandler 设置消息处理器
func (c *WecomBotChannel) SetMessageHandler(handler any) {
	if h, ok := handler.(MessageHandler); ok {
		c.messageHandler = h
	}
}

// SetMessageHandlerFunc 设置消息处理函数
func (c *WecomBotChannel) SetMessageHandlerFunc(handler func(ctx context.Context, msg *WecomBotMessage) (*types.Reply, error)) {
	c.messageHandler = MessageHandlerFunc(handler)
}

// Startup 启动渠道
func (c *WecomBotChannel) Startup(ctx context.Context) error {
	if c.config.BotID == "" || c.config.BotSecret == "" {
		return fmt.Errorf("wecom_bot_id 和 wecom_bot_secret 是必需的")
	}

	c.connMu.Lock()
	c.running = true
	c.connMu.Unlock()

	go c.runWSLoop(ctx)

	logger.Info("[WecomBot] 渠道启动中，正在连接 WebSocket...")
	return nil
}

// Stop 停止渠道
func (c *WecomBotChannel) Stop() error {
	c.connMu.Lock()
	c.running = false
	c.connMu.Unlock()

	close(c.stopCh)

	if c.conn != nil {
		c.conn.Close(websocket.StatusNormalClosure, "渠道已停止")
	}

	c.SetStarted(false)
	logger.Info("[WecomBot] 渠道已停止")
	return nil
}

// Send 发送消息
func (c *WecomBotChannel) Send(reply *types.Reply, ctx *types.Context) error {
	if reply == nil {
		return nil
	}

	// 从上下文获取接收者信息
	receiverI, hasReceiver := ctx.Get("receiver")
	if !hasReceiver {
		return fmt.Errorf("上下文中没有接收者信息")
	}
	receiver, ok := receiverI.(string)
	if !ok {
		return fmt.Errorf("接收者类型错误")
	}

	isGroup, _ := ctx.GetBool("isgroup")

	// 获取请求ID（用于响应消息）
	reqIDI, hasReqID := ctx.Get("req_id")
	var reqID string
	if hasReqID {
		reqID, _ = reqIDI.(string)
	}

	// 根据回复类型发送消息
	switch reply.Type {
	case types.ReplyText:
		return c.sendText(reply.StringContent(), receiver, isGroup, reqID)
	case types.ReplyImage, types.ReplyImageURL:
		return c.sendImage(reply.StringContent(), receiver, isGroup, reqID)
	case types.ReplyFile:
		return c.sendFile(reply.StringContent(), receiver, isGroup, reqID)
	case types.ReplyVideo, types.ReplyVideoURL:
		return c.sendFile(reply.StringContent(), receiver, isGroup, reqID)
	default:
		return c.sendText(reply.StringContent(), receiver, isGroup, reqID)
	}
}

// runWSLoop 运行 WebSocket 连接循环
func (c *WecomBotChannel) runWSLoop(ctx context.Context) {
	firstConnect := true

	for c.isRunning() {
		// 建立 WebSocket 连接
		conn, err := c.connect(ctx)
		if err != nil {
			if firstConnect {
				logger.Error("[WecomBot] 连接失败", zap.Error(err))
				c.ReportStartupError(err)
				firstConnect = false
			} else {
				logger.Warn("[WecomBot] 连接失败，准备重连...", zap.Error(err))
			}

			select {
			case <-time.After(reconnectInterval):
				continue
			case <-c.stopCh:
				return
			}
		}

		if firstConnect {
			logger.Info("[WecomBot] 已连接到企业微信 WebSocket")
			c.ReportStartupSuccess()
			firstConnect = false
		} else {
			logger.Info("[WecomBot] 已重新连接")
		}

		// 发送订阅请求
		if err := c.subscribe(); err != nil {
			logger.Error("[WecomBot] 订阅失败", zap.Error(err))
			conn.Close(websocket.StatusInternalError, "订阅失败")
			continue
		}

		// 运行会话
		c.runSession(ctx, conn)

		// 清理连接
		if c.conn != nil {
			c.conn.Close(websocket.StatusNormalClosure, "会话结束")
			c.conn = nil
		}
	}
}

// connect 建立 WebSocket 连接
func (c *WecomBotChannel) connect(ctx context.Context) (*websocket.Conn, error) {
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket 连接失败: %w", err)
	}
	c.conn = conn
	return conn, nil
}

// subscribe 发送订阅请求
func (c *WecomBotChannel) subscribe() error {
	reqID := c.genReqID()
	msg := &OutgoingMessage{
		Cmd: "aibot_subscribe",
		Headers: &MessageHeaders{
			ReqID: reqID,
		},
	}

	body := map[string]string{
		"bot_id": c.config.BotID,
		"secret": c.config.BotSecret,
	}
	bodyBytes, _ := json.Marshal(body)
	msg.Body = bodyBytes

	return c.wsSend(msg)
}

// runSession 运行 WebSocket 会话
func (c *WecomBotChannel) runSession(ctx context.Context, conn *websocket.Conn) {
	// 启动心跳
	go c.runHeartbeat(ctx)

	for c.isRunning() {
		_, data, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return
			}
			logger.Warn("[WecomBot] 读取消息失败", zap.Error(err))
			return
		}

		// 处理消息
		go c.handleWSMessage(ctx, data)
	}
}

// runHeartbeat 运行心跳
func (c *WecomBotChannel) runHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !c.isRunning() {
				return
			}
			msg := &OutgoingMessage{
				Cmd: "ping",
				Headers: &MessageHeaders{
					ReqID: c.genReqID(),
				},
			}
			if err := c.wsSend(msg); err != nil {
				logger.Warn("[WecomBot] 发送心跳失败", zap.Error(err))
			}
		case <-c.stopCh:
			return
		}
	}
}

// handleWSMessage 处理 WebSocket 消息
func (c *WecomBotChannel) handleWSMessage(ctx context.Context, data []byte) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Error("[WecomBot] 解析消息失败", zap.Error(err))
		return
	}

	reqID := ""
	if msg.Headers != nil {
		reqID = msg.Headers.ReqID
	}

	// 检查是否为待响应的请求
	if reqID != "" {
		c.pendingMu.Lock()
		if ch, ok := c.pendingRequests[reqID]; ok {
			ch <- &msg
			c.pendingMu.Unlock()
			return
		}
		c.pendingMu.Unlock()
	}

	// 订阅响应
	if msg.ErrCode != 0 && msg.Cmd == "" {
		if msg.ErrCode == 0 {
			logger.Info("[WecomBot] ✅ 订阅成功")
		} else {
			logger.Error("[WecomBot] 订阅失败",
				zap.Int("errcode", msg.ErrCode),
				zap.String("errmsg", msg.ErrMsg))
			c.ReportStartupError(fmt.Errorf("订阅失败: %s", msg.ErrMsg))
		}
		return
	}

	// 消息回调
	switch msg.Cmd {
	case "aibot_msg_callback":
		c.handleMsgCallback(ctx, &msg)
	case "aibot_event_callback":
		c.handleEventCallback(&msg)
	}
}

// handleMsgCallback 处理消息回调
func (c *WecomBotChannel) handleMsgCallback(ctx context.Context, wsMsg *WSMessage) {
	var body IncomingMessage
	if err := json.Unmarshal(wsMsg.Body, &body); err != nil {
		logger.Error("[WecomBot] 解析消息体失败", zap.Error(err))
		return
	}

	msgID := body.MsgID
	reqID := c.extractReqID(wsMsg)

	// 消息去重
	if c.isDuplicate(msgID) {
		logger.Debug("[WecomBot] 跳过重复消息", zap.String("msg_id", msgID))
		return
	}
	c.markReceived(msgID)

	// 创建消息实例
	msg, err := NewWecomBotMessage(&body, c.tmpDir)
	if err != nil {
		c.handleMsgCreateError(err, body.MsgType)
		return
	}

	msg.SetReqID(reqID)

	// 准备消息（下载媒体文件等）
	if err := msg.Prepare(); err != nil {
		logger.Warn("[WecomBot] 准备消息失败", zap.Error(err))
	}

	// 处理并发送回复
	c.processAndReply(ctx, msg, reqID)
}

// extractReqID 从消息中提取请求ID
func (c *WecomBotChannel) extractReqID(wsMsg *WSMessage) string {
	if wsMsg.Headers != nil {
		return wsMsg.Headers.ReqID
	}
	return ""
}

// handleMsgCreateError 处理消息创建错误
func (c *WecomBotChannel) handleMsgCreateError(err error, msgType string) {
	if strings.Contains(err.Error(), "不支持的消息类型") {
		logger.Debug("[WecomBot] 跳过不支持的消息类型",
			zap.String("msg_type", msgType))
	} else {
		logger.Error("[WecomBot] 创建消息失败", zap.Error(err))
	}
}

// processAndReply 处理消息并发送回复
func (c *WecomBotChannel) processAndReply(ctx context.Context, msg *WecomBotMessage, reqID string) {
	if c.messageHandler == nil {
		return
	}

	reply, err := c.messageHandler.HandleMessage(ctx, msg)
	if err != nil {
		logger.Error("[WecomBot] 消息处理错误", zap.Error(err))
		return
	}

	if reply == nil {
		return
	}

	c.sendReply(reply, msg, reqID)
}

// sendReply 发送回复消息
func (c *WecomBotChannel) sendReply(reply *types.Reply, msg *WecomBotMessage, reqID string) {
	replyCtx := types.NewContext(msg.Context.Type, reply.StringContent())
	replyCtx.Set("receiver", msg.GetChatID())
	replyCtx.Set("isgroup", msg.IsGroup())
	replyCtx.Set("req_id", reqID)

	if err := c.Send(reply, replyCtx); err != nil {
		logger.Error("[WecomBot] 发送回复失败", zap.Error(err))
	}
}

// handleEventCallback 处理事件回调
func (c *WecomBotChannel) handleEventCallback(wsMsg *WSMessage) {
	var body EventCallbackBody
	if err := json.Unmarshal(wsMsg.Body, &body); err != nil {
		logger.Error("[WecomBot] 解析事件体失败", zap.Error(err))
		return
	}

	eventType := body.Event.EventType
	switch eventType {
	case "enter_chat":
		logger.Info("[WecomBot] 用户进入聊天",
			zap.String("user_id", body.From.UserID))
	case "disconnected_event":
		logger.Warn("[WecomBot] 收到断开事件，可能有其他连接接管")
	default:
		logger.Debug("[WecomBot] 事件", zap.String("type", eventType))
	}
}

// sendText 发送文本消息
func (c *WecomBotChannel) sendText(content, receiver string, isGroup bool, reqID string) error {
	// 如果有 reqID，使用流式消息响应
	if reqID != "" {
		streamID := c.getOrCreateStreamID(reqID)
		body := StreamMessageBody{
			MsgType: "stream",
		}
		body.Stream.ID = streamID
		body.Stream.Finish = true
		body.Stream.Content = content

		bodyBytes, _ := json.Marshal(body)
		msg := &OutgoingMessage{
			Cmd: "aibot_respond_msg",
			Headers: &MessageHeaders{
				ReqID: reqID,
			},
			Body: bodyBytes,
		}
		return c.wsSend(msg)
	}

	// 主动发送消息
	body := map[string]any{
		"chatid":    receiver,
		"chat_type": 2,
		"msgtype":   "markdown",
		"markdown":  map[string]string{"content": content},
	}
	if !isGroup {
		body["chat_type"] = 1
	}

	bodyBytes, _ := json.Marshal(body)
	msg := &OutgoingMessage{
		Cmd: "aibot_send_msg",
		Headers: &MessageHeaders{
			ReqID: c.genReqID(),
		},
		Body: bodyBytes,
	}
	return c.wsSend(msg)
}

// prepareLocalFile 准备本地文件路径，处理 URL 下载
func (c *WecomBotChannel) prepareLocalFile(path string) (string, func(), error) {
	// 默认空清理函数，当文件为本地路径时无需清理
	cleanup := func() { /* 本地文件无需清理 */ }
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		tmpPath, err := c.downloadFile(path)
		if err != nil {
			return "", cleanup, err
		}
		cleanup = func() { os.Remove(tmpPath) }
		path = tmpPath
	}
	path = strings.TrimPrefix(path, "file://")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", cleanup, fmt.Errorf("文件不存在: %s", path)
	}
	return path, cleanup, nil
}

// getChatType 获取聊天类型数值
func getChatType(isGroup bool) int {
	if isGroup {
		return 2
	}
	return 1
}

// sendImage 发送图片消息
func (c *WecomBotChannel) sendImage(imagePath, receiver string, isGroup bool, reqID string) error {
	localPath, cleanup, err := c.prepareLocalFile(imagePath)
	if err != nil {
		return fmt.Errorf("下载图片失败: %w", err)
	}
	defer cleanup()

	mediaID, err := c.uploadMedia(localPath, "image")
	if err != nil {
		return fmt.Errorf("上传图片失败: %w", err)
	}

	if reqID != "" {
		return c.sendImageResponse(reqID, mediaID)
	}
	return c.sendImageMessage(receiver, isGroup, mediaID)
}

// sendImageResponse 发送图片响应消息
func (c *WecomBotChannel) sendImageResponse(reqID, mediaID string) error {
	body := ImageMessageBody{MsgType: "image"}
	body.Image.MediaID = mediaID
	bodyBytes, _ := json.Marshal(body)
	return c.wsSend(&OutgoingMessage{
		Cmd:     "aibot_respond_msg",
		Headers: &MessageHeaders{ReqID: reqID},
		Body:    bodyBytes,
	})
}

// sendImageMessage 主动发送图片消息
func (c *WecomBotChannel) sendImageMessage(receiver string, isGroup bool, mediaID string) error {
	body := map[string]any{
		"chatid":    receiver,
		"chat_type": getChatType(isGroup),
		"msgtype":   "image",
		"image":     map[string]string{"media_id": mediaID},
	}
	bodyBytes, _ := json.Marshal(body)
	return c.wsSend(&OutgoingMessage{
		Cmd:     "aibot_send_msg",
		Headers: &MessageHeaders{ReqID: c.genReqID()},
		Body:    bodyBytes,
	})
}

// sendFile 发送文件消息
func (c *WecomBotChannel) sendFile(filePath, receiver string, isGroup bool, reqID string) error {
	localPath, cleanup, err := c.prepareLocalFile(filePath)
	if err != nil {
		return fmt.Errorf("下载文件失败: %w", err)
	}
	defer cleanup()

	mediaType := getMediaType(filePath)
	mediaID, err := c.uploadMedia(localPath, mediaType)
	if err != nil {
		return fmt.Errorf("上传文件失败: %w", err)
	}

	if reqID != "" {
		return c.sendFileResponse(reqID, mediaID, mediaType)
	}
	return c.sendFileMessage(receiver, isGroup, mediaID, mediaType)
}

// getMediaType 根据文件扩展名确定媒体类型
func getMediaType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp4", ".mov", ".avi":
		return "video"
	default:
		return "file"
	}
}

// sendFileResponse 发送文件响应消息
func (c *WecomBotChannel) sendFileResponse(reqID, mediaID, mediaType string) error {
	body := FileMessageBody{MsgType: mediaType}
	body.File.MediaID = mediaID
	bodyBytes, _ := json.Marshal(body)
	return c.wsSend(&OutgoingMessage{
		Cmd:     "aibot_respond_msg",
		Headers: &MessageHeaders{ReqID: reqID},
		Body:    bodyBytes,
	})
}

// sendFileMessage 主动发送文件消息
func (c *WecomBotChannel) sendFileMessage(receiver string, isGroup bool, mediaID, mediaType string) error {
	body := map[string]any{
		"chatid":    receiver,
		"chat_type": getChatType(isGroup),
		"msgtype":   mediaType,
		mediaType:   map[string]string{"media_id": mediaID},
	}
	bodyBytes, _ := json.Marshal(body)
	return c.wsSend(&OutgoingMessage{
		Cmd:     "aibot_send_msg",
		Headers: &MessageHeaders{ReqID: c.genReqID()},
		Body:    bodyBytes,
	})
}

// uploadMedia 上传媒体文件
func (c *WecomBotChannel) uploadMedia(filePath, mediaType string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("获取文件信息失败: %w", err)
	}
	fileSize := stat.Size()
	fileName := stat.Name()

	if fileSize < 5 {
		return "", fmt.Errorf("文件太小")
	}

	totalChunks := c.calculateChunks(fileSize)
	if totalChunks > 100 {
		return "", fmt.Errorf("分块数超过限制")
	}

	md5Hex, err := c.computeFileMD5(file)
	if err != nil {
		return "", err
	}
	file.Seek(0, 0)

	uploadID, err := c.initUpload(mediaType, fileName, fileSize, totalChunks, md5Hex)
	if err != nil {
		return "", err
	}

	if err := c.uploadChunks(file, uploadID, totalChunks); err != nil {
		return "", err
	}

	return c.finishUpload(uploadID)
}

// calculateChunks 计算分块数
func (c *WecomBotChannel) calculateChunks(fileSize int64) int {
	totalChunks := int(fileSize / mediaChunkSize)
	if fileSize%mediaChunkSize != 0 {
		totalChunks++
	}
	return totalChunks
}

// computeFileMD5 计算文件 MD5
func (c *WecomBotChannel) computeFileMD5(file *os.File) (string, error) {
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("计算 MD5 失败: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// initUpload 初始化上传
func (c *WecomBotChannel) initUpload(mediaType, fileName string, fileSize int64, totalChunks int, md5Hex string) (string, error) {
	initReqID := c.genReqID()
	initBody := UploadInitBody{
		Type:        mediaType,
		Filename:    fileName,
		TotalSize:   fileSize,
		TotalChunks: totalChunks,
		MD5:         md5Hex,
	}
	initBodyBytes, _ := json.Marshal(initBody)

	initMsg := &OutgoingMessage{
		Cmd: "aibot_upload_media_init",
		Headers: &MessageHeaders{
			ReqID: initReqID,
		},
		Body: initBodyBytes,
	}

	initResp, err := c.wsSendAndWait(initMsg, uploadTimeout)
	if err != nil {
		return "", fmt.Errorf("初始化上传失败: %w", err)
	}
	if initResp.ErrCode != 0 {
		return "", fmt.Errorf("初始化上传失败: errcode=%d", initResp.ErrCode)
	}

	var initRespBody struct {
		UploadID string `json:"upload_id"`
	}
	if err := json.Unmarshal(initResp.Body, &initRespBody); err != nil {
		return "", fmt.Errorf("解析上传初始化响应失败: %w", err)
	}
	if initRespBody.UploadID == "" {
		return "", fmt.Errorf("获取 upload_id 失败")
	}
	return initRespBody.UploadID, nil
}

// uploadChunks 上传分块
func (c *WecomBotChannel) uploadChunks(file *os.File, uploadID string, totalChunks int) error {
	chunk := make([]byte, mediaChunkSize)
	for idx := 0; idx < totalChunks; idx++ {
		n, err := file.Read(chunk)
		if err != nil && err != io.EOF {
			return fmt.Errorf("读取文件失败: %w", err)
		}

		b64Data := base64.StdEncoding.EncodeToString(chunk[:n])
		chunkBody := UploadChunkBody{
			UploadID:   uploadID,
			ChunkIndex: idx,
			Base64Data: b64Data,
		}
		chunkBodyBytes, _ := json.Marshal(chunkBody)

		chunkMsg := &OutgoingMessage{
			Cmd: "aibot_upload_media_chunk",
			Headers: &MessageHeaders{
				ReqID: c.genReqID(),
			},
			Body: chunkBodyBytes,
		}

		chunkResp, err := c.wsSendAndWait(chunkMsg, uploadTimeout)
		if err != nil {
			return fmt.Errorf("上传分块 %d 失败: %w", idx, err)
		}
		if chunkResp.ErrCode != 0 {
			return fmt.Errorf("上传分块 %d 失败: errcode=%d", idx, chunkResp.ErrCode)
		}
	}
	return nil
}

// finishUpload 完成上传
func (c *WecomBotChannel) finishUpload(uploadID string) (string, error) {
	finishMsg := &OutgoingMessage{
		Cmd: "aibot_upload_media_finish",
		Headers: &MessageHeaders{
			ReqID: c.genReqID(),
		},
		Body: json.RawMessage(fmt.Sprintf(`{"upload_id":"%s"}`, uploadID)),
	}

	finishResp, err := c.wsSendAndWait(finishMsg, uploadTimeout)
	if err != nil {
		return "", fmt.Errorf("完成上传失败: %w", err)
	}
	if finishResp.ErrCode != 0 {
		return "", fmt.Errorf("完成上传失败: errcode=%d", finishResp.ErrCode)
	}

	var finishRespBody struct {
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(finishResp.Body, &finishRespBody); err != nil {
		return "", fmt.Errorf("解析完成上传响应失败: %w", err)
	}

	if finishRespBody.MediaID == "" {
		return "", fmt.Errorf("获取 media_id 失败")
	}

	logger.Info("[WecomBot] 媒体文件上传成功",
		zap.String("media_id", finishRespBody.MediaID))

	return finishRespBody.MediaID, nil
}

// wsSend 发送 WebSocket 消息
func (c *WecomBotChannel) wsSend(msg *OutgoingMessage) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return fmt.Errorf("WebSocket 连接已断开")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	return conn.Write(context.Background(), websocket.MessageText, data)
}

// wsSendAndWait 发送消息并等待响应
func (c *WecomBotChannel) wsSendAndWait(msg *OutgoingMessage, timeout time.Duration) (*WSMessage, error) {
	reqID := ""
	if msg.Headers != nil {
		reqID = msg.Headers.ReqID
	}

	// 创建响应通道
	ch := make(chan *WSMessage, 1)
	c.pendingMu.Lock()
	c.pendingRequests[reqID] = ch
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pendingRequests, reqID)
		c.pendingMu.Unlock()
	}()

	// 发送消息
	if err := c.wsSend(msg); err != nil {
		return nil, err
	}

	// 等待响应
	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("等待响应超时")
	}
}

// isRunning 检查是否正在运行
func (c *WecomBotChannel) isRunning() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.running
}

// isDuplicate 检查是否为重复消息
func (c *WecomBotChannel) isDuplicate(msgID string) bool {
	c.msgsMu.RLock()
	defer c.msgsMu.RUnlock()
	_, exists := c.receivedMsgs[msgID]
	return exists
}

// markReceived 标记消息已接收
func (c *WecomBotChannel) markReceived(msgID string) {
	c.msgsMu.Lock()
	defer c.msgsMu.Unlock()
	c.receivedMsgs[msgID] = time.Now()

	// 清理过期消息
	now := time.Now()
	for id, t := range c.receivedMsgs {
		if now.Sub(t) > messageExpireTime {
			delete(c.receivedMsgs, id)
		}
	}
}

// getOrCreateStreamID 获取或创建流式消息ID
func (c *WecomBotChannel) getOrCreateStreamID(reqID string) string {
	c.streamMu.Lock()
	defer c.streamMu.Unlock()

	if state, ok := c.streamStates[reqID]; ok {
		return state.streamID
	}

	state := &streamState{
		streamID: c.genReqID(),
	}
	c.streamStates[reqID] = state
	return state.streamID
}

// genReqID 生成请求ID
func (c *WecomBotChannel) genReqID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())[:16]
}

// downloadFile 下载文件
func (c *WecomBotChannel) downloadFile(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	// 确保临时目录存在
	if err := os.MkdirAll(c.tmpDir, 0755); err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 生成临时文件名
	fileName := filepath.Base(url)
	if idx := strings.Index(fileName, "?"); idx > 0 {
		fileName = fileName[:idx]
	}
	if fileName == "" {
		fileName = fmt.Sprintf("download_%d", time.Now().Unix())
	}

	tmpPath := filepath.Join(c.tmpDir, fileName)
	file, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return tmpPath, nil
}
