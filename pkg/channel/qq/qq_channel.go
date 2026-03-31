// Package qq 提供 QQ Bot 渠道的 WebSocket 实现。
// qq_channel.go 实现 QQ Bot 的 WebSocket 连接和消息处理。
package qq

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

// WebSocket 操作码常量
const (
	OpDispatch       = 0  // 分发事件
	OpHeartbeat      = 1  // 心跳
	OpIdentify       = 2  // 鉴权
	OpResume         = 6  // 恢复连接
	OpReconnect      = 7  // 重连
	OpInvalidSession = 9  // 无效会话
	OpHello          = 10 // Hello
	OpHeartbeatAck   = 11 // 心跳响应
)

// QQ 授权前缀
const qqAuthPrefix = "QQBot "

// 日志前缀
const logPrefix = "[QQ]"

// 富媒体文件类型常量
const (
	QQFileTypeImage = 1 // 图片
	QQFileTypeVideo = 2 // 视频
	QQFileTypeVoice = 3 // 语音
	QQFileTypeFile  = 4 // 文件
)

// 默认意图: 群聊和私聊事件(1<<25) | 公域频道消息(1<<30)
const DefaultIntents = (1 << 25) | (1 << 30)

// API 端点
const (
	QQAPIBase   = "https://api.sgroup.qq.com"
	TokenAPIURL = "https://bots.qq.com/app/getAppAccessToken"
)

// 可恢复的关闭码
var resumableCloseCodes = map[websocket.StatusCode]bool{
	4008: true,
	4009: true,
}

// Config QQ 渠道配置
type Config struct {
	AppID     string `json:"app_id"`     // 应用 ID
	AppSecret string `json:"app_secret"` // 应用密钥
	TmpDir    string `json:"tmp_dir"`    // 临时文件目录
}

// QQChannel 通过 WebSocket 实现 QQ Bot 渠道
type QQChannel struct {
	*channel.BaseChannel

	config *Config

	// 访问令牌管理
	accessToken    string
	tokenExpiresAt time.Time
	tokenMu        sync.RWMutex

	// WebSocket 连接
	wsConn       *websocket.Conn
	wsURL        string
	sessionID    string
	lastSeq      int
	heartbeatInt time.Duration
	canResume    bool
	connected    bool

	// 控制
	stopChan chan struct{}
	wg       sync.WaitGroup

	// 消息去重
	receivedMsgs *expiredMap

	// 消息序列号计数器
	msgSeqCounter map[string]int
	msgSeqMu      sync.Mutex

	// HTTP 客户端
	httpClient *http.Client

	// 媒体文件临时目录
	tmpDir string

	// 消息处理器
	messageHandler MessageHandler
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *QQMessage) (*types.Reply, error)
}

// MessageHandlerFunc 消息处理函数类型
type MessageHandlerFunc func(ctx context.Context, msg *QQMessage) (*types.Reply, error)

// HandleMessage 实现 MessageHandler 接口
func (f MessageHandlerFunc) HandleMessage(ctx context.Context, msg *QQMessage) (*types.Reply, error) {
	return f(ctx, msg)
}

// SetMessageHandler 设置消息处理器
func (q *QQChannel) SetMessageHandler(handler any) {
	if h, ok := handler.(MessageHandler); ok {
		q.messageHandler = h
	}
}

// SetMessageHandlerFunc 设置消息处理函数
func (q *QQChannel) SetMessageHandlerFunc(handler func(ctx context.Context, msg *QQMessage) (*types.Reply, error)) {
	q.messageHandler = MessageHandlerFunc(handler)
}

// NewQQChannel 创建新的 QQ 渠道实例
func NewQQChannel() *QQChannel {
	return &QQChannel{
		BaseChannel:   channel.NewBaseChannel("qq"),
		stopChan:      make(chan struct{}),
		receivedMsgs:  newExpiredMap(7*time.Hour + 6*time.Minute),
		msgSeqCounter: make(map[string]int),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Startup 启动 QQ 渠道
func (q *QQChannel) Startup(ctx context.Context) error {
	// 初始化配置
	q.initConfig()

	// 设置临时目录
	q.setupTmpDir()

	// 验证凭证
	if q.config.AppID == "" || q.config.AppSecret == "" {
		err := fmt.Errorf("qq_app_id and qq_app_secret are required")
		logger.Error(logPrefix+" Configuration error", zap.Error(err))
		q.ReportStartupError(err)
		return err
	}

	// 获取初始访问令牌
	if err := q.refreshAccessToken(); err != nil {
		logger.Error(logPrefix+" Failed to get access token", zap.Error(err))
		q.ReportStartupError(err)
		return err
	}

	// 获取 WebSocket 网关地址
	wsURL, err := q.getGatewayURL()
	if err != nil {
		logger.Error(logPrefix+" Failed to get gateway URL", zap.Error(err))
		q.ReportStartupError(err)
		return err
	}
	q.wsURL = wsURL

	// 启动 WebSocket 连接
	go q.wsLoop()

	return nil
}

// Stop 停止 QQ 渠道
func (q *QQChannel) Stop() error {
	logger.Info(logPrefix + " Stop() called")
	close(q.stopChan)
	q.wg.Wait()

	if q.wsConn != nil {
		q.wsConn.Close(websocket.StatusNormalClosure, "channel stopping")
	}
	q.connected = false
	q.SetStarted(false)

	return q.BaseChannel.Stop()
}

// Send 发送回复消息给用户
func (q *QQChannel) Send(reply *types.Reply, ctx *types.Context) error {
	msg, _ := ctx.Get("msg")
	qqMsg := extractQQMessage(msg)

	receiver := q.getReceiver(ctx, qqMsg)
	if receiver == "" {
		logger.Error(logPrefix + " No receiver found in context")
		return fmt.Errorf("no receiver in context")
	}

	eventType, msgID := getMsgInfo(msg, qqMsg)

	return q.sendByType(reply, qqMsg, eventType, msgID)
}

// getReceiver 获取消息接收者
func (q *QQChannel) getReceiver(ctx *types.Context, qqMsg *QQMessage) string {
	receiver, _ := ctx.GetString("receiver")
	if receiver != "" {
		return receiver
	}
	if qqMsg != nil {
		return qqMsg.OtherUserID
	}
	return ""
}

// extractQQMessage 从接口中提取 QQMessage
func extractQQMessage(msg any) *QQMessage {
	if msg == nil {
		return nil
	}
	qqMsg, ok := msg.(*QQMessage)
	if !ok {
		return nil
	}
	return qqMsg
}

// getMsgInfo 获取事件类型和消息ID
func getMsgInfo(msg any, qqMsg *QQMessage) (eventType, msgID string) {
	if qqMsg != nil {
		return qqMsg.EventType, qqMsg.MsgID
	}
	return "", ""
}

// sendByType 根据回复类型发送消息
func (q *QQChannel) sendByType(reply *types.Reply, qqMsg *QQMessage, eventType, msgID string) error {
	content := reply.StringContent()
	switch reply.Type {
	case types.ReplyText, types.ReplyText_:
		return q.sendText(content, qqMsg, eventType, msgID)
	case types.ReplyImage, types.ReplyImageURL:
		return q.sendImage(content, qqMsg, eventType, msgID)
	case types.ReplyFile:
		return q.sendFile(content, qqMsg, eventType, msgID)
	case types.ReplyVideo, types.ReplyVideoURL:
		return q.sendMedia(content, qqMsg, eventType, msgID, QQFileTypeVideo)
	default:
		logger.Warn(logPrefix+" Unsupported reply type, fallback to text",
			zap.String("type", reply.Type.String()))
		return q.sendText(content, qqMsg, eventType, msgID)
	}
}

// initConfig 使用默认值初始化配置
func (q *QQChannel) initConfig() {
	if q.config == nil {
		q.config = &Config{}
	}
}

// setupTmpDir 创建媒体文件临时目录
func (q *QQChannel) setupTmpDir() {
	if q.config.TmpDir != "" {
		q.tmpDir = q.config.TmpDir
	} else {
		home, _ := os.UserHomeDir()
		q.tmpDir = filepath.Join(home, "cow", "tmp")
	}
	os.MkdirAll(q.tmpDir, 0755)
}

// refreshAccessToken 刷新访问令牌
func (q *QQChannel) refreshAccessToken() error {
	body := map[string]string{
		"appId":        q.config.AppID,
		"clientSecret": q.config.AppSecret,
	}

	jsonBody, _ := json.Marshal(body)
	resp, err := q.httpClient.Post(TokenAPIURL, common.ContentTypeJSON, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("token decode failed: %w", err)
	}

	if result.AccessToken == "" {
		return fmt.Errorf("empty access token received")
	}

	q.tokenMu.Lock()
	q.accessToken = result.AccessToken
	q.tokenExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	q.tokenMu.Unlock()

	logger.Debug(logPrefix+" Access token refreshed", zap.Int("expires_in", result.ExpiresIn))
	return nil
}

// getAccessToken 返回当前访问令牌，如需要则刷新
func (q *QQChannel) getAccessToken() string {
	q.tokenMu.RLock()
	if time.Now().Before(q.tokenExpiresAt) {
		token := q.accessToken
		q.tokenMu.RUnlock()
		return token
	}
	q.tokenMu.RUnlock()

	q.refreshAccessToken()
	q.tokenMu.RLock()
	defer q.tokenMu.RUnlock()
	return q.accessToken
}

// getAuthHeaders 返回 API 请求的授权头
func (q *QQChannel) getAuthHeaders() map[string]string {
	return map[string]string{
		"Authorization":          qqAuthPrefix + q.getAccessToken(),
		common.HeaderContentType: common.ContentTypeJSON,
	}
}

// getGatewayURL 获取 WebSocket 网关地址
func (q *QQChannel) getGatewayURL() (string, error) {
	req, _ := http.NewRequest(http.MethodGet, QQAPIBase+"/gateway", nil)
	for k, v := range q.getAuthHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.URL == "" {
		return "", fmt.Errorf("empty gateway URL")
	}

	logger.Debug(logPrefix+" Gateway URL", zap.String("url", result.URL))
	return result.URL, nil
}

// wsLoop 管理 WebSocket 连接生命周期
func (q *QQChannel) wsLoop() {
	q.wg.Add(1)
	defer q.wg.Done()

	for {
		select {
		case <-q.stopChan:
			return
		default:
		}

		if err := q.connectWebSocket(); err != nil {
			logger.Error(logPrefix+" WebSocket connection error", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		// 连接关闭，尝试重连
		select {
		case <-q.stopChan:
			return
		default:
		}

		if q.canResume && q.sessionID != "" {
			logger.Info(logPrefix + " Will attempt resume in 3s...")
			time.Sleep(3 * time.Second)
		} else {
			logger.Info(logPrefix + " Will reconnect in 5s...")
			time.Sleep(5 * time.Second)
		}
	}
}

// connectWebSocket 建立 WebSocket 连接并处理消息
func (q *QQChannel) connectWebSocket() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, q.wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{},
	})
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	q.wsConn = conn
	q.canResume = false

	defer func() {
		conn.Close(websocket.StatusNormalClosure, "connection closing")
		q.connected = false
	}()

	heartbeatStop := make(chan struct{})
	var heartbeatWg sync.WaitGroup

	return q.readMessages(ctx, conn, heartbeatStop, &heartbeatWg)
}

// readMessages 读取并处理 WebSocket 消息
func (q *QQChannel) readMessages(ctx context.Context, conn *websocket.Conn, heartbeatStop chan struct{}, heartbeatWg *sync.WaitGroup) error {
	for {
		select {
		case <-q.stopChan:
			close(heartbeatStop)
			heartbeatWg.Wait()
			return nil
		default:
		}

		_, message, err := conn.Read(ctx)
		if err != nil {
			return q.handleReadError(err, heartbeatStop, heartbeatWg)
		}

		var data wsMessage
		if err := json.Unmarshal(message, &data); err != nil {
			logger.Error(logPrefix+" Failed to parse message", zap.Error(err))
			continue
		}

		if data.S != 0 {
			q.lastSeq = data.S
		}

		q.handleWSMessage(&data, heartbeatStop, heartbeatWg)
	}
}

// handleReadError 处理读取错误
func (q *QQChannel) handleReadError(err error, heartbeatStop chan struct{}, heartbeatWg *sync.WaitGroup) error {
	logger.Error(logPrefix+" WebSocket read error", zap.Error(err))
	close(heartbeatStop)
	heartbeatWg.Wait()

	var closeErr websocket.CloseError
	if errors.As(err, &closeErr) {
		if resumableCloseCodes[closeErr.Code] {
			q.canResume = true
			logger.Info(logPrefix+" Resumable close code detected, will attempt resume",
				zap.Int("code", int(closeErr.Code)))
		} else {
			q.canResume = false
			q.sessionID = ""
		}
	}
	return err
}

// wsMessage WebSocket 消息结构
type wsMessage struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	T  string          `json:"t"`
	S  int             `json:"s"`
}

// handleWSMessage 处理收到的 WebSocket 消息
func (q *QQChannel) handleWSMessage(msg *wsMessage, heartbeatStop chan struct{}, heartbeatWg *sync.WaitGroup) {
	switch msg.Op {
	case OpHello:
		var helloData struct {
			HeartbeatInterval int `json:"heartbeat_interval"`
		}
		json.Unmarshal(msg.D, &helloData)

		q.heartbeatInt = time.Duration(helloData.HeartbeatInterval) * time.Millisecond
		logger.Debug(logPrefix+" Received Hello", zap.Duration("heartbeat_interval", q.heartbeatInt))

		if q.canResume && q.sessionID != "" {
			q.sendResume()
		} else {
			q.sendIdentify()
		}

		// 开始心跳
		heartbeatWg.Add(1)
		go func() {
			defer heartbeatWg.Done()
			q.heartbeatLoop(heartbeatStop)
		}()

	case OpHeartbeatAck:
		// 心跳响应确认

	case OpHeartbeat:
		// 服务器请求心跳
		q.wsSend(map[string]any{
			"op": OpHeartbeat,
			"d":  q.lastSeq,
		})

	case OpReconnect:
		logger.Warn(logPrefix + " Server requested reconnect")
		q.canResume = true
		q.wsConn.Close(websocket.StatusNormalClosure, "reconnecting")

	case OpInvalidSession:
		logger.Warn(logPrefix + " Invalid session, re-identifying...")
		q.sessionID = ""
		q.canResume = false
		time.Sleep(2 * time.Second)
		q.sendIdentify()

	case OpDispatch:
		q.handleDispatch(msg)
	}
}

// handleDispatch 处理分发事件
func (q *QQChannel) handleDispatch(msg *wsMessage) {
	switch msg.T {
	case "READY":
		var readyData struct {
			SessionID string `json:"session_id"`
			User      struct {
				Username string `json:"username"`
			} `json:"user"`
		}
		json.Unmarshal(msg.D, &readyData)

		q.sessionID = readyData.SessionID
		q.connected = true
		q.canResume = false
		q.ReportStartupSuccess()

		logger.Info(logPrefix+" Connected successfully",
			zap.String("bot", readyData.User.Username))

	case "RESUMED":
		q.connected = true
		q.canResume = false
		q.ReportStartupSuccess()
		logger.Info(logPrefix + " Session resumed successfully")

	case EventTypeGroupAtMessage, EventTypeC2CMessage, EventTypeATMessage, EventTypeDirectMessage:
		q.handleMessageEvent(msg.D, msg.T)

	case EventTypeGroupAddRobot, EventTypeFriendAdd:
		logger.Info(logPrefix+" Event received", zap.String("type", msg.T))

	default:
		logger.Debug(logPrefix+" Dispatch event", zap.String("type", msg.T))
	}
}

// handleMessageEvent 处理消息事件
func (q *QQChannel) handleMessageEvent(data json.RawMessage, eventType string) {
	var eventData map[string]any
	if err := json.Unmarshal(data, &eventData); err != nil {
		logger.Error(logPrefix+" Failed to parse message event", zap.Error(err))
		return
	}

	msgID := getStringOr(eventData, "id", "")
	if q.receivedMsgs.Exists(msgID) {
		logger.Debug(logPrefix+" Duplicate message filtered", zap.String("msg_id", msgID))
		return
	}
	q.receivedMsgs.Set(msgID, true)

	// 解析消息
	qqMsg, err := NewQQMessage(eventData, eventType, q.tmpDir)
	if err != nil {
		logger.Warn(logPrefix+" Failed to create message", zap.Error(err))
		return
	}

	logger.Info(logPrefix+" Received message",
		zap.String("from", qqMsg.FromUserID),
		zap.String("type", eventType),
		zap.String("content", truncateString(qqMsg.Content, 50)))

	// 调用消息处理器
	if q.messageHandler != nil {
		ctx := context.Background()
		reply, err := q.messageHandler.HandleMessage(ctx, qqMsg)
		if err != nil {
			logger.Error(logPrefix+" 消息处理错误", zap.Error(err))
			return
		}
		if reply != nil {
			// 创建上下文
			replyCtx := types.NewContext(types.ContextText, qqMsg.Content)
			replyCtx.Set("receiver", qqMsg.OtherUserID)
			replyCtx.Set("msg", qqMsg)

			if qqMsg.IsGroupMessage {
				replyCtx.Set("isgroup", true)
				replyCtx.Set("group_id", qqMsg.GroupID)
			}

			if err := q.Send(reply, replyCtx); err != nil {
				logger.Error(logPrefix+" 发送回复失败", zap.Error(err))
			}
		}
	}
}

// heartbeatLoop 发送周期性心跳
func (q *QQChannel) heartbeatLoop(stop chan struct{}) {
	ticker := time.NewTicker(q.heartbeatInt)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if !q.connected {
				continue
			}
			q.wsSend(map[string]any{
				"op": OpHeartbeat,
				"d":  q.lastSeq,
			})
		}
	}
}

// sendIdentify 发送鉴权载荷
func (q *QQChannel) sendIdentify() {
	q.wsSend(map[string]any{
		"op": OpIdentify,
		"d": map[string]any{
			"token":   qqAuthPrefix + q.getAccessToken(),
			"intents": DefaultIntents,
			"shard":   []int{0, 1},
			"properties": map[string]string{
				"$os":      "linux",
				"$browser": "simpleclaw",
				"$device":  "simpleclaw",
			},
		},
	})
	logger.Debug(logPrefix + " Identify sent")
}

// sendResume 发送恢复连接载荷
func (q *QQChannel) sendResume() {
	q.wsSend(map[string]any{
		"op": OpResume,
		"d": map[string]any{
			"token":      qqAuthPrefix + q.getAccessToken(),
			"session_id": q.sessionID,
			"seq":        q.lastSeq,
		},
	})
	logger.Debug(logPrefix+" Resume sent", zap.String("session_id", q.sessionID))
}

// wsSend 通过 WebSocket 发送消息
func (q *QQChannel) wsSend(data any) {
	if q.wsConn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Error(logPrefix+" Failed to marshal WebSocket message", zap.Error(err))
		return
	}
	q.wsConn.Write(ctx, websocket.MessageText, jsonData)
}

// 消息发送方法

func (q *QQChannel) sendText(content string, msg *QQMessage, eventType, msgID string) error {
	url, body, err := q.buildMsgURLAndBody(msg, eventType, msgID)
	if err != nil {
		return err
	}
	body["content"] = content
	body["msg_type"] = 0

	return q.postAPI(url, body)
}

func (q *QQChannel) sendImage(pathOrURL string, msg *QQMessage, eventType, msgID string) error {
	if eventType != EventTypeGroupAtMessage && eventType != EventTypeC2CMessage {
		return q.sendText(pathOrURL, msg, eventType, msgID)
	}

	fileInfo, err := q.uploadRichMedia(pathOrURL, QQFileTypeImage, msg, eventType)
	if err != nil {
		q.sendText("[Image upload failed]", msg, eventType, msgID)
		return err
	}

	return q.sendMediaMessage(fileInfo, msg, eventType, msgID)
}

func (q *QQChannel) sendFile(pathOrURL string, msg *QQMessage, eventType, msgID string) error {
	if eventType != EventTypeGroupAtMessage && eventType != EventTypeC2CMessage {
		return q.sendText(pathOrURL, msg, eventType, msgID)
	}

	fileInfo, err := q.uploadRichMedia(pathOrURL, QQFileTypeFile, msg, eventType)
	if err != nil {
		q.sendText("[File upload failed]", msg, eventType, msgID)
		return err
	}

	return q.sendMediaMessage(fileInfo, msg, eventType, msgID)
}

func (q *QQChannel) sendMedia(pathOrURL string, msg *QQMessage, eventType, msgID string, fileType int) error {
	if eventType != EventTypeGroupAtMessage && eventType != EventTypeC2CMessage {
		return q.sendText(pathOrURL, msg, eventType, msgID)
	}

	fileInfo, err := q.uploadRichMedia(pathOrURL, fileType, msg, eventType)
	if err != nil {
		return err
	}

	return q.sendMediaMessage(fileInfo, msg, eventType, msgID)
}

// buildMsgURLAndBody 构建发送消息的 API URL 和基础请求体
func (q *QQChannel) buildMsgURLAndBody(msg *QQMessage, eventType, msgID string) (string, map[string]any, error) {
	if msg == nil {
		return "", nil, fmt.Errorf("no message context")
	}

	var url string
	body := make(map[string]any)

	switch eventType {
	case EventTypeGroupAtMessage:
		groupOpenID := msg.GroupOpenID
		url = fmt.Sprintf("%s/v2/groups/%s/messages", QQAPIBase, groupOpenID)
		body["msg_id"] = msgID
		body["msg_seq"] = q.getNextMsgSeq(msgID)

	case EventTypeC2CMessage:
		userOpenID := msg.UserOpenID
		if userOpenID == "" {
			userOpenID = msg.FromUserID
		}
		url = fmt.Sprintf("%s/v2/users/%s/messages", QQAPIBase, userOpenID)
		body["msg_id"] = msgID
		body["msg_seq"] = q.getNextMsgSeq(msgID)

	case EventTypeATMessage:
		channelID := msg.OtherUserID
		url = fmt.Sprintf("%s/channels/%s/messages", QQAPIBase, channelID)
		body["msg_id"] = msgID

	case EventTypeDirectMessage:
		guildID := ""
		url = fmt.Sprintf("%s/dms/%s/messages", QQAPIBase, guildID)
		body["msg_id"] = msgID

	default:
		return "", nil, fmt.Errorf("unsupported event type: %s", eventType)
	}

	return url, body, nil
}

// getNextMsgSeq 获取下一个消息序列号
func (q *QQChannel) getNextMsgSeq(msgID string) int {
	q.msgSeqMu.Lock()
	defer q.msgSeqMu.Unlock()
	seq := q.msgSeqCounter[msgID] + 1
	q.msgSeqCounter[msgID] = seq
	return seq
}

// postAPI 发送 QQ API 请求
func (q *QQChannel) postAPI(url string, body map[string]any) error {
	jsonBody, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	for k, v := range q.getAuthHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Info(logPrefix + " Message sent successfully")
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
}

// uploadRichMedia 上传媒体文件到 QQ 并返回 file_info
func (q *QQChannel) uploadRichMedia(pathOrURL string, fileType int, msg *QQMessage, eventType string) (string, error) {
	var uploadURL string

	switch eventType {
	case EventTypeGroupAtMessage:
		uploadURL = fmt.Sprintf("%s/v2/groups/%s/files", QQAPIBase, msg.GroupOpenID)
	case EventTypeC2CMessage:
		userOpenID := msg.UserOpenID
		if userOpenID == "" {
			userOpenID = msg.FromUserID
		}
		uploadURL = fmt.Sprintf("%s/v2/users/%s/files", QQAPIBase, userOpenID)
	default:
		return "", fmt.Errorf("rich media upload not supported for event type: %s", eventType)
	}

	// 准备上传请求体
	uploadBody := map[string]any{
		"file_type":    fileType,
		"srv_send_msg": false,
	}

	// 检查是 URL 还是本地文件
	pathOrURL = strings.TrimPrefix(pathOrURL, "file://")

	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		uploadBody["url"] = pathOrURL
	} else {
		// 读取本地文件并编码为 base64
		data, err := os.ReadFile(pathOrURL)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		uploadBody["file_data"] = base64.StdEncoding.EncodeToString(data)
	}

	// 上传
	jsonBody, _ := json.Marshal(uploadBody)
	req, _ := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(jsonBody))
	for k, v := range q.getAuthHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		FileInfo string `json:"file_info"`
		FileUUID string `json:"file_uuid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.FileInfo == "" {
		return "", fmt.Errorf("no file_info in response")
	}

	logger.Info(logPrefix+" Rich media uploaded",
		zap.Int("file_type", fileType),
		zap.String("file_uuid", result.FileUUID))

	return result.FileInfo, nil
}

// sendMediaMessage 使用 file_info 发送媒体消息
func (q *QQChannel) sendMediaMessage(fileInfo string, msg *QQMessage, eventType, msgID string) error {
	url, body, err := q.buildMsgURLAndBody(msg, eventType, msgID)
	if err != nil {
		return err
	}

	body["msg_type"] = 7
	body["media"] = map[string]string{
		"file_info": fileInfo,
	}

	return q.postAPI(url, body)
}

// 辅助函数

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// expiredMap 提供带过期时间的简单映射
type expiredMap struct {
	mu    sync.RWMutex
	items map[string]*expiredItem
	ttl   time.Duration
}

type expiredItem struct {
	value     any
	expiresAt time.Time
}

func newExpiredMap(ttl time.Duration) *expiredMap {
	m := &expiredMap{
		items: make(map[string]*expiredItem),
		ttl:   ttl,
	}
	go m.cleanup()
	return m
}

func (m *expiredMap) Set(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = &expiredItem{
		value:     value,
		expiresAt: time.Now().Add(m.ttl),
	}
}

func (m *expiredMap) Get(key string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.value, true
}

func (m *expiredMap) Exists(key string) bool {
	_, ok := m.Get(key)
	return ok
}

func (m *expiredMap) cleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for k, v := range m.items {
			if now.After(v.expiresAt) {
				delete(m.items, k)
			}
		}
		m.mu.Unlock()
	}
}
