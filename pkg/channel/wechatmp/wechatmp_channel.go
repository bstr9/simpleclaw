package wechatmp

import (
	"bytes"
	"github.com/bstr9/simpleclaw/pkg/common"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	"go.uber.org/zap"
)

const errWechatAPIFormat = "wechat API error: code=%d, msg=%s"

// Config 定义微信公众号渠道配置
type Config struct {
	// AppID 是微信公众号的 AppID
	AppID string `json:"app_id"`
	// AppSecret 是微信公众号的 AppSecret
	AppSecret string `json:"app_secret"`
	// Token 是在公众号后台设置的用于签名验证的令牌
	Token string `json:"token"`
	// EncodingAESKey 是消息加解密密钥（可选）
	EncodingAESKey string `json:"encoding_aes_key"`
	// Port 是 HTTP 服务器端口
	Port int `json:"port"`
	// Host 是 HTTP 服务器主机地址
	Host string `json:"host"`
	// PassiveReply 决定是否使用被动回复模式
	// 如果为 true，响应在回调请求中发送
	// 如果为 false，响应通过客服 API 发送
	PassiveReply bool `json:"passive_reply"`
	// SubscribeMsg 是用户关注时发送的消息
	SubscribeMsg string `json:"subscribe_msg"`
}

// WechatmpChannel 实现微信公众号渠道
type WechatmpChannel struct {
	*channel.BaseChannel

	config  *Config
	client  *WechatMPClient
	crypto  *Crypto
	handler *Handler
	server  *http.Server

	// 被动回复模式缓存
	cacheMap   map[string][]cachedReply
	cacheMu    sync.RWMutex
	runningSet map[string]bool
	runningMu  sync.RWMutex
	requestCnt map[string]int
	reqCntMu   sync.RWMutex

	// 消息处理器回调
	messageHandler MessageHandlerFunc

	// 关注消息
	subscribeMsg string

	// 控制
	startupOnce sync.Once
	stopOnce    sync.Once
}

// cachedReply 表示被动回复模式的缓存回复
type cachedReply struct {
	replyType string
	content   string
}

// MessageHandlerFunc 是处理消息的回调函数类型
type MessageHandlerFunc func(msg *WechatMessage, ctx *types.Context) (*types.Reply, error)

// NewWechatmpChannel 创建新的微信公众号渠道实例
func NewWechatmpChannel(cfg *Config) *WechatmpChannel {
	if cfg == nil {
		cfg = &Config{
			Port:         8080,
			Host:         "0.0.0.0",
			PassiveReply: true,
		}
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}

	ch := &WechatmpChannel{
		BaseChannel:  channel.NewBaseChannel(channel.ChannelWechatMP),
		config:       cfg,
		cacheMap:     make(map[string][]cachedReply),
		runningSet:   make(map[string]bool),
		requestCnt:   make(map[string]int),
		subscribeMsg: cfg.SubscribeMsg,
	}

	// 设置不支持的回复类型
	ch.SetNotSupportTypes([]types.ReplyType{})

	return ch
}

// SetMessageHandler 设置消息处理器回调
func (w *WechatmpChannel) SetMessageHandler(handler any) {
	if h, ok := handler.(MessageHandlerFunc); ok {
		w.messageHandler = h
	}
}

// Startup 启动微信公众号渠道
func (w *WechatmpChannel) Startup(ctx context.Context) error {
	var startErr error
	w.startupOnce.Do(func() {
		// 初始化微信客户端
		w.client = NewWechatMPClient(w.config.AppID, w.config.AppSecret)

		// 如果提供了 AES 密钥则初始化加解密
		if w.config.EncodingAESKey != "" {
			crypto, err := NewCrypto(w.config.Token, w.config.EncodingAESKey, w.config.AppID)
			if err != nil {
				startErr = fmt.Errorf("failed to initialize crypto: %w", err)
				w.ReportStartupError(startErr)
				return
			}
			w.crypto = crypto
		}

		// 创建 HTTP 处理器
		w.handler = NewHandler(w, w.config.Token, w.crypto)
		w.handler.SetMessageProcessor(&defaultMessageProcessor{channel: w})

		// 创建 HTTP 服务器
		mux := http.NewServeMux()
		mux.Handle("/wx", w.handler)

		addr := fmt.Sprintf("%s:%d", w.config.Host, w.config.Port)
		w.server = &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		// 在后台启动 HTTP 服务器
		go func() {
			logger.Info("[wechatmp] Starting HTTP server", zap.String("addr", addr))
			if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("[wechatmp] HTTP server error", zap.Error(err))
				w.ReportStartupError(err)
			}
		}()

		w.ReportStartupSuccess()
		logger.Info("[wechatmp] Channel started successfully",
			zap.String("addr", addr),
			zap.Bool("passive_reply", w.config.PassiveReply))
	})

	return startErr
}

// Stop 停止微信公众号渠道
func (w *WechatmpChannel) Stop() error {
	var err error
	w.stopOnce.Do(func() {
		if w.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if shutdownErr := w.server.Shutdown(ctx); shutdownErr != nil {
				logger.Error("[wechatmp] Error shutting down HTTP server", zap.Error(shutdownErr))
				err = shutdownErr
			} else {
				logger.Info("[wechatmp] HTTP server stopped gracefully")
			}
		}

		// 清除缓存
		w.cacheMu.Lock()
		w.cacheMap = make(map[string][]cachedReply)
		w.cacheMu.Unlock()

		w.runningMu.Lock()
		w.runningSet = make(map[string]bool)
		w.runningMu.Unlock()

		w.reqCntMu.Lock()
		w.requestCnt = make(map[string]int)
		w.reqCntMu.Unlock()

		w.SetStarted(false)
	})

	return err
}

// Send 发送回复消息
func (w *WechatmpChannel) Send(reply *types.Reply, ctx *types.Context) error {
	if reply == nil {
		return nil
	}

	// 从上下文获取接收者
	receiverI, ok := ctx.Get("receiver")
	if !ok {
		return fmt.Errorf("no receiver in context")
	}
	receiver, ok := receiverI.(string)
	if !ok {
		return fmt.Errorf("receiver is not a string")
	}

	if w.config.PassiveReply {
		return w.sendPassiveReply(reply, receiver, ctx)
	}
	return w.sendActiveReply(reply, receiver, ctx)
}

// sendPassiveReply 缓存被动回复模式的回复
func (w *WechatmpChannel) sendPassiveReply(reply *types.Reply, receiver string, ctx *types.Context) error {
	content := reply.StringContent()

	switch reply.Type {
	case types.ReplyText, types.ReplyInfo, types.ReplyError:
		// 移除 Markdown 符号以兼容微信
		content = removeMarkdownSymbols(content)
		w.cacheReply(receiver, "text", content)

	case types.ReplyVoice:
		// 语音需要上传获取 media_id
		// 在处理器发送时处理
		w.cacheReply(receiver, "voice", content)

	case types.ReplyImage, types.ReplyImageURL:
		// 图片需要上传并缓存 media_id
		mediaID, err := w.uploadMedia(reply, ctx, "image")
		if err != nil {
			logger.Error("[wechatmp] Failed to upload image", zap.Error(err))
			return err
		}
		w.cacheReply(receiver, "image", mediaID)

	case types.ReplyVideo, types.ReplyVideoURL:
		mediaID, err := w.uploadMedia(reply, ctx, "video")
		if err != nil {
			logger.Error("[wechatmp] Failed to upload video", zap.Error(err))
			return err
		}
		w.cacheReply(receiver, "video", mediaID)

	default:
		// 默认为文本
		w.cacheReply(receiver, "text", content)
	}

	logger.Debug("[wechatmp] Reply cached",
		zap.String("receiver", receiver),
		zap.String("type", reply.Type.String()))

	return nil
}

// sendActiveReply 通过客服 API 发送回复
func (w *WechatmpChannel) sendActiveReply(reply *types.Reply, receiver string, ctx *types.Context) error {
	content := reply.StringContent()

	switch reply.Type {
	case types.ReplyText, types.ReplyInfo, types.ReplyError:
		// 分割长文本
		texts := splitStringByUTF8Length(content, MaxUTF8Len)
		for i, text := range texts {
			if i > 0 {
				time.Sleep(500 * time.Millisecond) // 避免频率限制
			}
			if err := w.client.SendText(receiver, text); err != nil {
				return err
			}
		}

	case types.ReplyVoice:
		// 上传语音并发送
		mediaID, err := w.uploadMedia(reply, ctx, "voice")
		if err != nil {
			return err
		}
		return w.client.SendVoice(receiver, mediaID)

	case types.ReplyImage, types.ReplyImageURL:
		mediaID, err := w.uploadMedia(reply, ctx, "image")
		if err != nil {
			return err
		}
		return w.client.SendImage(receiver, mediaID)

	case types.ReplyVideo, types.ReplyVideoURL:
		mediaID, err := w.uploadMedia(reply, ctx, "video")
		if err != nil {
			return err
		}
		return w.client.SendVideo(receiver, mediaID, "", "")

	default:
		// 默认为文本
		return w.client.SendText(receiver, content)
	}

	return nil
}

// cacheReply 缓存被动回复模式的回复
func (w *WechatmpChannel) cacheReply(receiver, replyType, content string) {
	w.cacheMu.Lock()
	defer w.cacheMu.Unlock()

	w.cacheMap[receiver] = append(w.cacheMap[receiver], cachedReply{
		replyType: replyType,
		content:   content,
	})
}

// uploadMedia 上传媒体文件到微信并返回 media_id
func (w *WechatmpChannel) uploadMedia(reply *types.Reply, ctx *types.Context, mediaType string) (string, error) {
	// 这是一个简化实现
	// 生产环境中需要读取文件并上传到微信 API
	content := reply.StringContent()

	// 对于基于 URL 的媒体，先下载
	if reply.Type == types.ReplyImageURL || reply.Type == types.ReplyVideoURL {
		// 从 URL 下载
		resp, err := http.Get(content)
		if err != nil {
			return "", fmt.Errorf("failed to download media: %w", err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read media data: %w", err)
		}

		return w.client.UploadMedia(mediaType, "media."+mediaType, data)
	}

	// 对于本地文件，读取并上传
	// 需要处理文件路径
	return "", fmt.Errorf("local file upload not implemented")
}

// HandleMessage 处理收到的消息（由处理器调用）
func (w *WechatmpChannel) HandleMessage(msg *WechatMessage) (*types.Reply, error) {
	if w.messageHandler == nil {
		return nil, nil
	}

	ctx := types.NewContext(msg.ToContextType(), msg.GetContent())
	ctx.Set("receiver", msg.FromUserName)
	ctx.Set("sender", msg.ToUserName)
	ctx.Set("msg_id", msg.GetMessageID())
	ctx.Set("raw_message", msg)

	return w.messageHandler(msg, ctx)
}

// GetSubscribeMsg 返回订阅消息。
func (w *WechatmpChannel) GetSubscribeMsg() string {
	return w.subscribeMsg
}

// defaultMessageProcessor 实现 MessageProcessor 接口
type defaultMessageProcessor struct {
	channel *WechatmpChannel
}

// ProcessMessage 处理微信消息（实现 MessageProcessor 接口）。
func (p *defaultMessageProcessor) ProcessMessage(msg *WechatMessage) (*types.Reply, error) {
	return p.channel.HandleMessage(msg)
}

// WechatMPClient 提供微信公众号 API 客户端
type WechatMPClient struct {
	appID       string
	appSecret   string
	accessToken string
	expiresAt   time.Time
	mu          sync.RWMutex
}

// NewWechatMPClient 创建新的微信公众号 API 客户端
func NewWechatMPClient(appID, appSecret string) *WechatMPClient {
	return &WechatMPClient{
		appID:     appID,
		appSecret: appSecret,
	}
}

// GetAccessToken 获取访问令牌，必要时刷新
func (c *WechatMPClient) GetAccessToken() (string, error) {
	c.mu.RLock()
	if c.accessToken != "" && time.Now().Before(c.expiresAt.Add(-60*time.Second)) {
		token := c.accessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	return c.refreshAccessToken()
}

// refreshAccessToken 刷新访问令牌
func (c *WechatMPClient) refreshAccessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 获取锁后双重检查
	if c.accessToken != "" && time.Now().Before(c.expiresAt.Add(-60*time.Second)) {
		return c.accessToken, nil
	}

	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		c.appID, c.appSecret)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf(errWechatAPIFormat, result.ErrCode, result.ErrMsg)
	}

	c.accessToken = result.AccessToken
	c.expiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	return c.accessToken, nil
}

// SendText 通过客服 API 发送文本消息
func (c *WechatMPClient) SendText(toUser, content string) error {
	return c.sendMessage(toUser, "text", map[string]string{"content": content})
}

// SendImage 通过客服 API 发送图片消息
func (c *WechatMPClient) SendImage(toUser, mediaID string) error {
	return c.sendMessage(toUser, "image", map[string]string{"media_id": mediaID})
}

// SendVoice 通过客服 API 发送语音消息
func (c *WechatMPClient) SendVoice(toUser, mediaID string) error {
	return c.sendMessage(toUser, "voice", map[string]string{"media_id": mediaID})
}

// SendVideo 通过客服 API 发送视频消息
func (c *WechatMPClient) SendVideo(toUser, mediaID, title, description string) error {
	return c.sendMessage(toUser, "video", map[string]string{
		"media_id":    mediaID,
		"title":       title,
		"description": description,
	})
}

// sendMessage 通过客服 API 发送消息
func (c *WechatMPClient) sendMessage(toUser, msgType string, data map[string]string) error {
	token, err := c.GetAccessToken()
	if err != nil {
		return err
	}

	msg := map[string]any{
		"touser":  toUser,
		"msgtype": msgType,
		msgType:   data,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/message/custom/send?access_token=%s", token)
	resp, err := http.Post(apiURL, common.ContentTypeJSON, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.ErrCode != 0 {
		return fmt.Errorf(errWechatAPIFormat, result.ErrCode, result.ErrMsg)
	}

	return nil
}

// UploadMedia 上传媒体文件到微信
func (c *WechatMPClient) UploadMedia(mediaType, filename string, data []byte) (string, error) {
	token, err := c.GetAccessToken()
	if err != nil {
		return "", err
	}

	// 构建 multipart 表单
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加文件
	part, err := writer.CreateFormFile("media", filename)
	if err != nil {
		return "", err
	}
	part.Write(data)

	if err := writer.Close(); err != nil {
		return "", err
	}

	// 发送请求
	apiURL := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/media/upload?access_token=%s&type=%s",
		token, mediaType)

	resp, err := http.Post(apiURL, writer.FormDataContentType(), body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		MediaID string `json:"media_id"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf(errWechatAPIFormat, result.ErrCode, result.ErrMsg)
	}

	return result.MediaID, nil
}

// DownloadMedia 从微信下载媒体文件
func (c *WechatMPClient) DownloadMedia(mediaID string) ([]byte, error) {
	token, err := c.GetAccessToken()
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/media/get?access_token=%s&media_id=%s",
		token, mediaID)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// 辅助函数

// splitStringByUTF8Length 按 UTF-8 长度分割字符串
func splitStringByUTF8Length(s string, maxLen int) []string {
	if len(s) <= maxLen {
		return []string{s}
	}

	var result []string
	var current string

	for _, r := range s {
		if len(current)+len(string(r)) > maxLen {
			result = append(result, current)
			current = string(r)
		} else {
			current += string(r)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

// removeMarkdownSymbols 移除 Markdown 格式符号
func removeMarkdownSymbols(s string) string {
	// 移除常见的 Markdown 符号
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "~~", "")
	s = strings.ReplaceAll(s, "`", "")
	return s
}
