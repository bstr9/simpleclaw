// feishu 包提供飞书渠道实现。
// feishu_channel.go 定义飞书渠道主要结构体和方法。
package feishu

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/bstr9/simpleclaw/pkg/common"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	"go.uber.org/zap"
)

const (
	defaultPort            = 9891
	tokenExpireTime        = 2 * time.Hour
	messageCacheExpireTime = 7 * time.Hour
	maxStaleMessageAge     = 60 * time.Second

	errCreateRequest    = "failed to create request: %w"
	errParseResponse    = "failed to parse response: %w"
	errUploadRequest    = "failed to upload request: %w"
	errUploadFailed     = "upload failed: code=%d, msg=%s"
	errGetAccessToken   = "failed to get access token: %w"
	errMarshalRequest   = "failed to marshal request: %w"
	errRequestFailed    = "request failed: %w"
	errFeishuAPI        = "feishu api error: code=%d"
	errFeishuAPIWithMsg = "feishu api error: code=%d, msg=%s"
	prefixFile          = "file://"

	// 飞书 API 基础 URL
	feishuAPIBase = "https://open.feishu.cn/open-apis"

	// 消息类型
	msgTypeText = "text"
	msgTypePost = "post"

	// ID 类型
	idTypeOpenID = "open_id"
	idTypeChatID = "chat_id"

	// 表单字段名
	formFieldFileType = "file_type"
	formFieldFileName = "file_name"

	apiPathMessages = "/im/v1/messages?receive_id_type=%s"
)

// Config 定义飞书渠道配置
type Config struct {
	AppID              string `json:"app_id"`
	AppSecret          string `json:"app_secret"`
	VerificationToken  string `json:"verification_token"`
	EncryptKey         string `json:"encrypt_key"`
	Port               int    `json:"port"`
	EventMode          string `json:"event_mode"` // "webhook" or "websocket"
	BotName            string `json:"bot_name"`
	GroupSharedSession bool   `json:"group_shared_session"`
	StreamOutput       bool   `json:"stream_output"` // 是否启用流式输出
}

// FeishuChannel 实现飞书渠道接口
type FeishuChannel struct {
	*channel.BaseChannel

	config  *Config
	server  *http.Server
	handler *HTTPHandler

	// WebSocket 客户端
	wsClient *WSClient

	// 访问令牌管理
	accessToken   string
	tokenExpireAt time.Time
	tokenMu       sync.RWMutex

	// 机器人信息
	botOpenID string

	// 消息去重缓存
	processedMsgs map[string]time.Time
	msgCacheMu    sync.RWMutex

	// 停止控制
	stopOnce sync.Once
	stopCh   chan struct{}

	// 消息处理器回调
	messageHandler FeishuMessageProcessor

	// Pair 管理器 (可选)
	pairManager PairManager
}

// PairManager 配对管理器接口
type PairManager interface {
	CheckSessionPair(sessionID, userID, channelType string) (*PairCheckResult, error)
	StartPair(sessionID, userID, channelType string) (*PairResult, error)
}

// PairCheckResult 配对检查结果
type PairCheckResult struct {
	Paired  bool
	Status  string
	AuthURL string
}

// PairResult 配对结果
type PairResult struct {
	Success bool
	AuthURL string
	Message string
}

// FeishuMessageProcessor 飞书消息处理器接口
type FeishuMessageProcessor interface {
	ProcessMessage(ctx context.Context, msg *FeishuMessage) (*types.Reply, error)
}

// MessageHandlerFunc 消息处理函数类型
type MessageHandlerFunc func(ctx context.Context, msg *FeishuMessage) (*types.Reply, error)

// ProcessMessage 实现 FeishuMessageProcessor 接口
func (f MessageHandlerFunc) ProcessMessage(ctx context.Context, msg *FeishuMessage) (*types.Reply, error) {
	return f(ctx, msg)
}

// feishuHandlerAdapter 适配通用 ChatMessage 处理器到飞书专用处理器
type feishuHandlerAdapter struct {
	handler interface {
		HandleMessage(ctx context.Context, msg types.ChatMessage) (*types.Reply, error)
	}
}

func (a *feishuHandlerAdapter) ProcessMessage(ctx context.Context, msg *FeishuMessage) (*types.Reply, error) {
	return a.handler.HandleMessage(ctx, msg)
}

// NewFeishuChannel 创建新的飞书渠道实例
func NewFeishuChannel(cfg *Config) *FeishuChannel {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	if cfg.EventMode == "" {
		cfg.EventMode = "webhook"
	}

	return &FeishuChannel{
		BaseChannel:   channel.NewBaseChannel("feishu"),
		config:        cfg,
		processedMsgs: make(map[string]time.Time),
		stopCh:        make(chan struct{}),
	}
}

// SetMessageHandler 设置消息处理器
func (f *FeishuChannel) SetMessageHandler(handler any) {
	switch h := handler.(type) {
	case FeishuMessageProcessor:
		f.messageHandler = h
	case interface {
		HandleMessage(ctx context.Context, msg types.ChatMessage) (*types.Reply, error)
	}:
		f.messageHandler = &feishuHandlerAdapter{handler: h}
	}
}

// SetMessageHandlerFunc 以函数形式设置消息处理器
func (f *FeishuChannel) SetMessageHandlerFunc(handler func(ctx context.Context, msg *FeishuMessage) (*types.Reply, error)) {
	f.messageHandler = MessageHandlerFunc(handler)
}

// SetPairManager 设置配对管理器
func (f *FeishuChannel) SetPairManager(pm PairManager) {
	f.pairManager = pm
}

// Startup 启动飞书渠道
func (f *FeishuChannel) Startup(ctx context.Context) error {
	logger.Info("[Feishu] Starting channel",
		zap.String("app_id", f.config.AppID),
		zap.String("event_mode", f.config.EventMode),
		zap.Int("port", f.config.Port))

	// 设置不支持的回复类型
	f.SetNotSupportTypes([]types.ReplyType{
		types.ReplyVoice,
	})

	// 获取机器人信息
	if err := f.fetchBotInfo(); err != nil {
		logger.Warn("[Feishu] Failed to fetch bot info", zap.Error(err))
	}

	// 启动消息清理协程
	go f.cleanupMessageCache()

	// 根据事件模式启动
	switch f.config.EventMode {
	case "websocket":
		return f.startWebSocketClient(ctx)
	default:
		return f.startWebhookServer(ctx)
	}
}

// startWebSocketClient 启动 WebSocket 长连接模式
func (f *FeishuChannel) startWebSocketClient(ctx context.Context) error {
	f.wsClient = NewWSClient(f.config.AppID, f.config.AppSecret, f)

	if err := f.wsClient.Start(ctx); err != nil {
		logger.Error("[Feishu] WebSocket 启动失败", zap.Error(err))
		return err
	}

	f.ReportStartupSuccess()
	logger.Info("[Feishu] WebSocket 连接已启动")
	return nil
}

// startWebhookServer 启动 Webhook 模式的 HTTP 服务器
func (f *FeishuChannel) startWebhookServer(ctx context.Context) error {
	f.handler = NewHTTPHandler(f)

	mux := http.NewServeMux()
	mux.Handle("/", f.handler)

	addr := fmt.Sprintf(":%d", f.config.Port)
	f.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("[Feishu] HTTP server starting", zap.String("addr", addr))
		if err := f.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("[Feishu] HTTP server error", zap.Error(err))
			f.ReportStartupError(err)
		}
	}()

	f.ReportStartupSuccess()
	logger.Info("[Feishu] Channel started successfully", zap.String("addr", addr))
	return nil
}

// Stop 停止飞书渠道
func (f *FeishuChannel) Stop() error {
	var err error
	f.stopOnce.Do(func() {
		close(f.stopCh)

		// 停止 WebSocket 客户端
		if f.wsClient != nil {
			if wsErr := f.wsClient.Stop(); wsErr != nil {
				logger.Error("[Feishu] Error stopping WebSocket client", zap.Error(wsErr))
				err = wsErr
			}
		}

		// 停止 HTTP 服务器
		if f.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if shutdownErr := f.server.Shutdown(ctx); shutdownErr != nil {
				logger.Error("[Feishu] Error shutting down HTTP server", zap.Error(shutdownErr))
				err = shutdownErr
			}
		}

		f.SetStarted(false)
		logger.Info("[Feishu] Channel stopped")
	})
	return err
}

// Send 发送回复到飞书
func (f *FeishuChannel) Send(reply *types.Reply, ctx *types.Context) error {
	if reply == nil {
		return fmt.Errorf("reply is nil")
	}

	accessToken, err := f.getAccessToken()
	if err != nil {
		return fmt.Errorf(errGetAccessToken, err)
	}

	// 从上下文获取消息信息
	msgI, hasMsg := ctx.Get("msg")
	var msg *FeishuMessage
	if hasMsg {
		msg, _ = msgI.(*FeishuMessage)
	}

	// 确定接收者 ID 类型和目标
	receiveIDType, _ := ctx.GetString("receive_id_type")
	if receiveIDType == "" {
		receiveIDType = idTypeOpenID
	}
	receiver, _ := ctx.GetString("receiver")

	// 准备消息内容
	msgType, content, err := f.prepareMessageContent(reply, accessToken)
	if err != nil {
		return fmt.Errorf("failed to prepare message content: %w", err)
	}

	// 检查是否可以回复现有消息
	if msg != nil && msg.MsgID != "" && msg.IsGroupChat {
		return f.replyToMessage(msg.MsgID, msgType, content, accessToken)
	}

	// 发送新消息
	return f.sendNewMessage(receiver, receiveIDType, msgType, content, accessToken)
}

// SendTypingReaction 发送 typing 表情，防止飞书超时重发
func (f *FeishuChannel) SendTypingReaction(msgID string) (reactionID string, err error) {
	accessToken, err := f.getAccessToken()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/im/v1/messages/%s/reactions", feishuAPIBase, msgID)

	data := map[string]any{
		"reaction_type": map[string]string{
			"emoji_type": "Typing",
		},
	}

	body, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+accessToken)
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int `json:"code"`
		Data struct {
			ReactionID string `json:"reaction_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if result.Code != 0 {
		logger.Debug("[Feishu] Typing reaction failed", zap.Int("code", result.Code))
	}

	return result.Data.ReactionID, nil
}

// RemoveTypingReaction 移除 typing 表情
func (f *FeishuChannel) RemoveTypingReaction(msgID, reactionID string) error {
	if reactionID == "" {
		return nil
	}

	accessToken, err := f.getAccessToken()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/im/v1/messages/%s/reactions/%s", feishuAPIBase, msgID, reactionID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+accessToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return err
	}

	if result.Code != 0 {
		logger.Debug("[Feishu] Remove typing reaction failed", zap.Int("code", result.Code))
	}

	return nil
}

// prepareMessageContent 根据回复类型准备消息内容
func (f *FeishuChannel) prepareMessageContent(reply *types.Reply, accessToken string) (string, string, error) {
	switch reply.Type {
	case types.ReplyText:
		content := reply.StringContent()
		// 使用 post 类型支持 Markdown 渲染
		return msgTypePost, buildPostContent(content), nil

	case types.ReplyImage, types.ReplyImageURL:
		imageKey, err := f.uploadImage(reply.StringContent(), accessToken)
		if err != nil {
			return "", "", fmt.Errorf("failed to upload image: %w", err)
		}
		return "image", fmt.Sprintf(`{"image_key":%q}`, imageKey), nil

	case types.ReplyFile:
		fileKey, err := f.uploadFile(reply.StringContent(), accessToken)
		if err != nil {
			return "", "", fmt.Errorf("failed to upload file: %w", err)
		}
		return "file", fmt.Sprintf(`{"file_key":%q}`, fileKey), nil

	case types.ReplyVideo, types.ReplyVideoURL:
		uploadData, err := f.uploadVideo(reply.StringContent(), accessToken)
		if err != nil {
			return "", "", fmt.Errorf("failed to upload video: %w", err)
		}
		contentJSON, _ := json.Marshal(uploadData)
		return "media", string(contentJSON), nil

	case types.ReplyCard:
		cardJSON, err := json.Marshal(reply.Content)
		if err != nil {
			return "", "", fmt.Errorf("failed to marshal card content: %w", err)
		}
		return "interactive", string(cardJSON), nil

	default:
		return msgTypePost, buildPostContent(reply.StringContent()), nil
	}
}

// replyToMessage 回复现有消息
func (f *FeishuChannel) replyToMessage(msgID, msgType, content, accessToken string) error {
	url := fmt.Sprintf("%s/im/v1/messages/%s/reply", feishuAPIBase, msgID)

	data := map[string]string{
		"msg_type": msgType,
		"content":  content,
	}

	return f.sendFeishuRequest(url, data, accessToken)
}

// sendNewMessage 发送新消息
func (f *FeishuChannel) sendNewMessage(receiveID, receiveIDType, msgType, content, accessToken string) error {
	url := fmt.Sprintf(feishuAPIBase+apiPathMessages, receiveIDType)

	data := map[string]string{
		"receive_id": receiveID,
		"msg_type":   msgType,
		"content":    content,
	}

	return f.sendFeishuRequest(url, data, accessToken)
}

// sendFeishuRequest 发送请求到飞书 API
func (f *FeishuChannel) sendFeishuRequest(url string, data map[string]string, accessToken string) error {
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf(errMarshalRequest, err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf(errCreateRequest, err)
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+accessToken)
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf(errRequestFailed, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf(errParseResponse, err)
	}

	if result.Code != 0 {
		return fmt.Errorf(errFeishuAPIWithMsg, result.Code, result.Msg)
	}

	logger.Debug("[Feishu] Message sent successfully")
	return nil
}

// SendStreamMessage 发送流式消息
func (f *FeishuChannel) SendStreamMessage(feishuMsg *FeishuMessage, content string, ctx *types.Context) (string, error) {
	accessToken, err := f.getAccessToken()
	if err != nil {
		return "", fmt.Errorf(errGetAccessToken, err)
	}

	receiveIDType := idTypeOpenID
	receiver := feishuMsg.SenderOpenID

	if feishuMsg.IsGroupChat {
		receiveIDType = idTypeChatID
		receiver = feishuMsg.ChatID
	}

	url := fmt.Sprintf(feishuAPIBase+apiPathMessages, receiveIDType)

	data := map[string]string{
		"receive_id": receiver,
		"msg_type":   msgTypePost,
		"content":    buildPostContent(content),
	}

	body, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf(errMarshalRequest, err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf(errCreateRequest, err)
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+accessToken)
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf(errRequestFailed, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int `json:"code"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf(errParseResponse, err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf(errFeishuAPI, result.Code)
	}

	return result.Data.MessageID, nil
}

func (f *FeishuChannel) UpdateStreamMessage(msgID, content string) error {
	accessToken, err := f.getAccessToken()
	if err != nil {
		return fmt.Errorf(errGetAccessToken, err)
	}

	url := fmt.Sprintf("%s/im/v1/messages/%s", feishuAPIBase, msgID)

	data := map[string]string{
		"msg_type": msgTypePost,
		"content":  buildPostContent(content),
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf(errMarshalRequest, err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf(errCreateRequest, err)
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+accessToken)
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf(errRequestFailed, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf(errParseResponse, err)
	}

	if result.Code != 0 {
		return fmt.Errorf(errFeishuAPI, result.Code)
	}

	return nil
}

// SendStreamCard 发送流式卡片消息
func (f *FeishuChannel) SendStreamCard(feishuMsg *FeishuMessage, content string, ctx map[string]any) (string, error) {
	accessToken, err := f.getAccessToken()
	if err != nil {
		return "", fmt.Errorf(errGetAccessToken, err)
	}

	receiveIDType := idTypeOpenID
	receiver := feishuMsg.SenderOpenID

	if feishuMsg.IsGroupChat {
		receiveIDType = idTypeChatID
		receiver = feishuMsg.ChatID
	}

	url := fmt.Sprintf(feishuAPIBase+apiPathMessages, receiveIDType)

	data := map[string]string{
		"receive_id": receiver,
		"msg_type":   "interactive",
		"content":    buildStreamingCard(content, false),
	}

	body, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf(errMarshalRequest, err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf(errCreateRequest, err)
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+accessToken)
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf(errRequestFailed, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int `json:"code"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf(errParseResponse, err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf(errFeishuAPI, result.Code)
	}

	return result.Data.MessageID, nil
}

// UpdateStreamCard 更新流式卡片消息
func (f *FeishuChannel) UpdateStreamCard(msgID, content string, isComplete bool) error {
	accessToken, err := f.getAccessToken()
	if err != nil {
		return fmt.Errorf(errGetAccessToken, err)
	}

	url := fmt.Sprintf("%s/im/v1/messages/%s", feishuAPIBase, msgID)

	data := map[string]string{
		"msg_type": "interactive",
		"content":  buildStreamingCard(content, isComplete),
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf(errMarshalRequest, err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf(errCreateRequest, err)
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+accessToken)
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf(errRequestFailed, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf(errParseResponse, err)
	}

	if result.Code != 0 {
		return fmt.Errorf(errFeishuAPI, result.Code)
	}

	return nil
}

// getAccessToken 获取或刷新访问令牌
func (f *FeishuChannel) getAccessToken() (string, error) {
	f.tokenMu.RLock()
	if f.accessToken != "" && time.Now().Before(f.tokenExpireAt) {
		token := f.accessToken
		f.tokenMu.RUnlock()
		return token, nil
	}
	f.tokenMu.RUnlock()

	f.tokenMu.Lock()
	defer f.tokenMu.Unlock()

	// 双重检查
	if f.accessToken != "" && time.Now().Before(f.tokenExpireAt) {
		return f.accessToken, nil
	}

	url := feishuAPIBase + "/auth/v3/tenant_access_token/internal/"

	data := map[string]string{
		"app_id":     f.config.AppID,
		"app_secret": f.config.AppSecret,
	}

	body, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf("token api error: code=%d, msg=%s", result.Code, result.Msg)
	}

	f.accessToken = result.TenantAccessToken
	f.tokenExpireAt = time.Now().Add(time.Duration(result.Expire) * time.Second)

	logger.Debug("[Feishu] Access token refreshed")
	return f.accessToken, nil
}

// uploadImage 上传图片到飞书并返回图片 key
func (f *FeishuChannel) uploadImage(imagePath, accessToken string) (string, error) {
	// 处理 URL 和本地文件
	var reader io.Reader
	var fileName string

	if strings.HasPrefix(imagePath, "http://") || strings.HasPrefix(imagePath, "https://") {
		// 从 URL 下载
		resp, err := http.Get(imagePath)
		if err != nil {
			return "", fmt.Errorf("failed to download image: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
		}

		reader = resp.Body
		fileName = filepath.Base(imagePath)
	} else {
		// 本地文件
		imagePath = strings.TrimPrefix(imagePath, prefixFile)

		file, err := os.Open(imagePath)
		if err != nil {
			return "", fmt.Errorf("failed to open image: %w", err)
		}
		defer file.Close()

		reader = file
		fileName = filepath.Base(imagePath)
	}

	// 创建 multipart 表单
	body := &bytes.Buffer{}
	writer := createMultipartWriter(body, "image", fileName, reader)

	url := feishuAPIBase + "/im/v1/images"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", fmt.Errorf(errCreateRequest, err)
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+accessToken)
	req.Header.Set(common.HeaderContentType, writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf(errUploadRequest, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ImageKey string `json:"image_key"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf(errParseResponse, err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf(errUploadFailed, result.Code, result.Msg)
	}

	return result.Data.ImageKey, nil
}

// uploadFile 上传文件到飞书并返回文件 key
func (f *FeishuChannel) uploadFile(filePath, accessToken string) (string, error) {
	filePath = strings.TrimPrefix(filePath, prefixFile)

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileName := filepath.Base(filePath)

	// 确定文件类型
	fileType := getFileType(fileName)

	// 创建 multipart 表单
	body := &bytes.Buffer{}
	writer := createMultipartWriterWithField(body, "file", fileName, file, map[string]string{
		formFieldFileType: fileType,
		formFieldFileName: fileName,
	})

	url := feishuAPIBase + "/im/v1/files"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", fmt.Errorf(errCreateRequest, err)
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+accessToken)
	req.Header.Set(common.HeaderContentType, writer.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf(errUploadRequest, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FileKey string `json:"file_key"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf(errParseResponse, err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf(errUploadFailed, result.Code, result.Msg)
	}

	return result.Data.FileKey, nil
}

// uploadVideo 上传视频到飞书并返回上传数据
func (f *FeishuChannel) uploadVideo(videoPath, accessToken string) (map[string]any, error) {
	videoPath = strings.TrimPrefix(videoPath, prefixFile)

	file, err := os.Open(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open video: %w", err)
	}
	defer file.Close()

	fileName := filepath.Base(videoPath)

	// 创建 multipart 表单
	body := &bytes.Buffer{}
	writer := createMultipartWriterWithField(body, "file", fileName, file, map[string]string{
		formFieldFileType: "mp4",
		formFieldFileName: fileName,
	})

	url := feishuAPIBase + "/im/v1/files"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf(errCreateRequest, err)
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+accessToken)
	req.Header.Set(common.HeaderContentType, writer.FormDataContentType())

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(errUploadRequest, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FileKey string `json:"file_key"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf(errParseResponse, err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf(errUploadFailed, result.Code, result.Msg)
	}

	return map[string]any{
		"file_key": result.Data.FileKey,
		"duration": 0, // 时长需要用 ffprobe 计算
	}, nil
}

// handleMessage 处理接收到的飞书消息
func (f *FeishuChannel) handleMessage(msg *FeishuMessage) error {
	if f.messageHandler == nil {
		logger.Warn("[Feishu] No message handler set")
		return nil
	}

	// 创建上下文
	ctx := types.NewContext(msg.CtxType, msg.Content)

	// 将消息添加到上下文
	ctx.Set("msg", msg)
	ctx.Set("isgroup", msg.IsGroupChat)
	ctx.Set("receiver", f.getReceiver(msg))
	ctx.Set("receive_id_type", msg.ReceiveIDType)
	ctx.Set("session_id", msg.GetSessionID(f.config.GroupSharedSession))

	// 调用消息处理器
	reply, err := f.messageHandler.ProcessMessage(context.Background(), msg)
	if err != nil {
		logger.Error("[Feishu] Message handler error", zap.Error(err))
		return err
	}

	// 发送回复
	if reply != nil {
		return f.Send(reply, ctx)
	}

	return nil
}

// getReceiver 返回回复的接收者 ID
func (f *FeishuChannel) getReceiver(msg *FeishuMessage) string {
	if msg.IsGroupChat {
		return msg.ChatID
	}
	return msg.OpenID
}

// fetchBotInfo 从飞书获取机器人信息
func (f *FeishuChannel) fetchBotInfo() error {
	accessToken, err := f.getAccessToken()
	if err != nil {
		return err
	}

	url := feishuAPIBase + "/bot/v3/info/"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Bot  struct {
			OpenID string `json:"open_id"`
			Name   string `json:"app_name"`
		} `json:"bot"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	if result.Code != 0 {
		return fmt.Errorf("bot info api error: code=%d", result.Code)
	}

	f.botOpenID = result.Bot.OpenID
	logger.Info("[Feishu] Bot info fetched", zap.String("open_id", f.botOpenID))
	return nil
}

// getBotOpenID 返回机器人的 OpenID
func (f *FeishuChannel) getBotOpenID() string {
	return f.botOpenID
}

// isMessageProcessed 检查消息是否已处理
func (f *FeishuChannel) isMessageProcessed(msgID string) bool {
	f.msgCacheMu.RLock()
	defer f.msgCacheMu.RUnlock()
	_, exists := f.processedMsgs[msgID]
	return exists
}

// markMessageProcessed 标记消息为已处理
func (f *FeishuChannel) markMessageProcessed(msgID string) {
	f.msgCacheMu.Lock()
	defer f.msgCacheMu.Unlock()
	f.processedMsgs[msgID] = time.Now()
}

// cleanupMessageCache 定期清理旧的消息 ID
func (f *FeishuChannel) cleanupMessageCache() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-f.stopCh:
			return
		case <-ticker.C:
			f.msgCacheMu.Lock()
			now := time.Now()
			for msgID, t := range f.processedMsgs {
				if now.Sub(t) > messageCacheExpireTime {
					delete(f.processedMsgs, msgID)
				}
			}
			f.msgCacheMu.Unlock()
		}
	}
}

// HandleEvent 直接处理飞书事件（供外部调用）
func (f *FeishuChannel) HandleEvent(event *FeishuEvent) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}

	// 获取访问令牌
	accessToken, err := f.getAccessToken()
	if err != nil {
		return fmt.Errorf(errGetAccessToken, err)
	}

	// 解析消息
	msg, err := NewFeishuMessage(event, accessToken)
	if err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}

	return f.handleMessage(msg)
}

// DecryptEvent 解密加密的飞书事件
func (f *FeishuChannel) DecryptEvent(encryptedData string) ([]byte, error) {
	if f.config.EncryptKey == "" {
		return nil, fmt.Errorf("no encrypt key configured")
	}

	// Base64 解码
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// 创建密码器
	key := sha256.Sum256([]byte(f.config.EncryptKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// CBC 解密
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 移除填充
	plaintext = pkcs7Unpad(plaintext)

	return plaintext, nil
}

// pkcs7Unpad 移除 PKCS7 填充
func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padding := int(data[len(data)-1])
	if padding > len(data) {
		return data
	}
	return data[:len(data)-padding]
}

// 辅助函数

// createMultipartWriter 创建文件上传的 multipart writer
func createMultipartWriter(body *bytes.Buffer, fieldName, fileName string, reader io.Reader) *multipart.Writer {
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(fieldName, fileName)
	io.Copy(part, reader)
	writer.WriteField("image_type", "message")
	writer.Close()
	return writer
}

// createMultipartWriterWithField 创建带额外字段的 multipart writer
func createMultipartWriterWithField(body *bytes.Buffer, fieldName, fileName string, reader io.Reader, fields map[string]string) *multipart.Writer {
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(fieldName, fileName)
	io.Copy(part, reader)
	for k, v := range fields {
		writer.WriteField(k, v)
	}
	writer.Close()
	return writer
}

// getFileType 根据文件扩展名返回飞书文件类型
func getFileType(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	typeMap := map[string]string{
		".opus": "opus",
		".mp4":  "mp4",
		".pdf":  "pdf",
		".doc":  "doc",
		".docx": "doc",
		".xls":  "xls",
		".xlsx": "xls",
		".ppt":  "ppt",
		".pptx": "ppt",
	}
	if t, ok := typeMap[ext]; ok {
		return t
	}
	return "stream"
}

// buildPostContent 构建飞书 post 类型消息内容，支持 Markdown 渲染
func buildPostContent(text string) string {
	content := map[string]any{
		"zh_cn": map[string]any{
			"content": [][]map[string]string{
				{{"tag": "md", "text": text}},
			},
		},
	}
	data, _ := json.Marshal(content)
	return string(data)
}
