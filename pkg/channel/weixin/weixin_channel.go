// Package weixin 提供微信个人号渠道实现
// 本包实现了 ilink bot 协议用于微信通信
package weixin

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
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
	"go.uber.org/zap"
)

const (
	// 默认 API 端点
	defaultBaseURL    = "https://ilinkai.weixin.qq.com"
	defaultCDNBaseURL = "https://novac2c.cdn.weixin.qq.com/c2c"

	// 超时设置
	defaultLongPollTimeout = 35 * time.Second
	defaultAPITimeout      = 15 * time.Second
	qrLoginTimeout         = 480 * time.Second
	maxQRRefreshes         = 10

	// 错误码
	sessionExpiredErrCode = -14

	// 重试设置
	maxConsecutiveFailures = 3
	backoffDelay           = 30 * time.Second
	retryDelay             = 2 * time.Second

	// 消息限制
	textChunkLimit = 4000

	// 消息类型
	messageTypeUser = 1 // 用户消息
	messageTypeBot  = 2 // 机器人消息

	// 消息项类型
	itemText  = 1
	itemImage = 2
	itemVoice = 3
	itemFile  = 4
	itemVideo = 5

	// CDN 媒体类型
	mediaTypeImage = 1
	mediaTypeVideo = 2
	mediaTypeFile  = 3

	// 日志前缀
	logPrefix = "[Weixin]"
)

// LoginStatus 表示当前登录状态
type LoginStatus string

const (
	LoginStatusIdle     LoginStatus = "idle"
	LoginStatusWaiting  LoginStatus = "waiting_scan"
	LoginStatusScanned  LoginStatus = "scanned"
	LoginStatusLoggedIn LoginStatus = "logged_in"
)

// Config 微信渠道配置
type Config struct {
	BaseURL         string `json:"base_url"`
	CDNBaseURL      string `json:"cdn_base_url"`
	Token           string `json:"token"`
	CredentialsPath string `json:"credentials_path"`
}

// Credentials 存储的登录凭证
type Credentials struct {
	Token   string `json:"token"`
	BaseURL string `json:"base_url"`
	BotID   string `json:"bot_id"`
	UserID  string `json:"user_id"`
}

// WeixinChannel 实现微信个人号渠道
type WeixinChannel struct {
	*channel.BaseChannel

	config *Config
	api    *weixinAPI
	client *http.Client

	// 登录状态
	loginStatus  LoginStatus
	currentQRURL string
	credentials  *Credentials

	// 轮询状态
	stopChan      chan struct{}
	pollWg        sync.WaitGroup
	getUpdatesBuf string

	// 消息发送的上下文令牌
	contextTokens map[string]string
	contextMu     sync.RWMutex

	// 消息去重
	receivedMsgs *expiredMap

	// 媒体文件临时目录
	tmpDir string

	// 消息处理器
	messageHandler MessageHandler
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *WeixinMessage) (*types.Reply, error)
}

// MessageHandlerFunc 消息处理函数类型
type MessageHandlerFunc func(ctx context.Context, msg *WeixinMessage) (*types.Reply, error)

// HandleMessage 实现 MessageHandler 接口
func (f MessageHandlerFunc) HandleMessage(ctx context.Context, msg *WeixinMessage) (*types.Reply, error) {
	return f(ctx, msg)
}

// SetMessageHandler 设置消息处理器
func (w *WeixinChannel) SetMessageHandler(handler any) {
	if h, ok := handler.(MessageHandler); ok {
		w.messageHandler = h
	}
}

// SetMessageHandlerFunc 设置消息处理函数
func (w *WeixinChannel) SetMessageHandlerFunc(handler func(ctx context.Context, msg *WeixinMessage) (*types.Reply, error)) {
	w.messageHandler = MessageHandlerFunc(handler)
}

// NewWeixinChannel 创建新的微信渠道实例
func NewWeixinChannel() *WeixinChannel {
	return &WeixinChannel{
		BaseChannel:   channel.NewBaseChannel("weixin"),
		loginStatus:   LoginStatusIdle,
		stopChan:      make(chan struct{}),
		contextTokens: make(map[string]string),
		receivedMsgs:  newExpiredMap(7*time.Hour + 6*time.Minute),
	}
}

// Startup 启动微信渠道
func (w *WeixinChannel) Startup(ctx context.Context) error {
	// 初始化配置
	w.initConfig()

	// 设置 HTTP 客户端
	w.client = &http.Client{
		Timeout: defaultAPITimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}

	// 设置临时目录
	w.setupTmpDir()

	// 加载凭证或执行二维码登录
	token, baseURL := w.loadCredentials()
	if token == "" {
		var err error
		token, baseURL, err = w.performQRLogin(ctx)
		if err != nil {
			return fmt.Errorf("QR login failed: %w", err)
		}
	}

	// 初始化 API 客户端
	w.api = newWeixinAPI(baseURL, token, w.config.CDNBaseURL, w.client)
	w.loginStatus = LoginStatusLoggedIn

	logger.Info(logPrefix+" Channel started successfully",
		zap.String("credentials_path", w.config.CredentialsPath))

	w.ReportStartupSuccess()

	// 启动轮询循环
	go w.pollLoop()

	return nil
}

// Stop 停止微信渠道
func (w *WeixinChannel) Stop() error {
	logger.Info(logPrefix + " Stop() called")
	close(w.stopChan)
	w.pollWg.Wait()
	w.SetStarted(false)
	return w.BaseChannel.Stop()
}

// Send 发送回复消息给用户
func (w *WeixinChannel) Send(reply *types.Reply, ctx *types.Context) error {
	receiver, _ := ctx.GetString("receiver")
	if receiver == "" {
		// 尝试从消息中获取
		if msg, ok := ctx.Get("msg"); ok {
			if wxMsg, ok := msg.(*WeixinMessage); ok {
				receiver = wxMsg.GetFromUserID()
			}
		}
	}

	if receiver == "" {
		logger.Error(logPrefix + " No receiver found in context")
		return fmt.Errorf("no receiver in context")
	}

	contextToken := w.getContextToken(receiver, ctx)
	if contextToken == "" {
		logger.Error(logPrefix+" No context_token for receiver",
			zap.String("receiver", receiver))
		return fmt.Errorf("no context_token for receiver")
	}

	switch reply.Type {
	case types.ReplyText, types.ReplyText_:
		return w.sendText(reply.StringContent(), receiver, contextToken)
	case types.ReplyImage, types.ReplyImageURL:
		return w.sendImage(reply.StringContent(), receiver, contextToken)
	case types.ReplyFile:
		return w.sendFile(reply.StringContent(), receiver, contextToken)
	case types.ReplyVideo, types.ReplyVideoURL:
		return w.sendVideo(reply.StringContent(), receiver, contextToken)
	default:
		logger.Warn(logPrefix+" Unsupported reply type, fallback to text",
			zap.String("type", reply.Type.String()))
		return w.sendText(reply.StringContent(), receiver, contextToken)
	}
}

// GetLoginStatus 返回当前登录状态
func (w *WeixinChannel) GetLoginStatus() LoginStatus {
	return w.loginStatus
}

// GetLoginStatusString 返回当前登录状态的字符串表示
// 预留接口：供未来微信登录状态监控功能使用
func (w *WeixinChannel) GetLoginStatusString() string {
	return string(w.loginStatus)
}

// GetCurrentQRURL 返回当前二维码 URL
// 预留接口：供未来微信扫码登录引导功能使用
func (w *WeixinChannel) GetCurrentQRURL() string {
	return w.currentQRURL
}

// initConfig 使用默认值初始化配置
func (w *WeixinChannel) initConfig() {
	if w.config == nil {
		w.config = &Config{}
	}
	if w.config.BaseURL == "" {
		w.config.BaseURL = defaultBaseURL
	}
	if w.config.CDNBaseURL == "" {
		w.config.CDNBaseURL = defaultCDNBaseURL
	}
	if w.config.CredentialsPath == "" {
		home, _ := os.UserHomeDir()
		w.config.CredentialsPath = filepath.Join(home, ".weixin_cow_credentials.json")
	}
}

// setupTmpDir 创建媒体文件临时目录
func (w *WeixinChannel) setupTmpDir() {
	home, _ := os.UserHomeDir()
	w.tmpDir = filepath.Join(home, "cow", "tmp")
	os.MkdirAll(w.tmpDir, 0755)
}

// loadCredentials 从文件加载存储的凭证
func (w *WeixinChannel) loadCredentials() (token, baseURL string) {
	data, err := os.ReadFile(w.config.CredentialsPath)
	if err != nil {
		return "", ""
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		logger.Warn(logPrefix+" Failed to parse credentials file", zap.Error(err))
		return "", ""
	}

	w.credentials = &creds
	if creds.BaseURL != "" {
		return creds.Token, creds.BaseURL
	}
	return creds.Token, w.config.BaseURL
}

// saveCredentials 保存凭证到文件
func (w *WeixinChannel) saveCredentials(creds *Credentials) error {
	w.credentials = creds
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(w.config.CredentialsPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	if err := os.WriteFile(w.config.CredentialsPath, data, 0600); err != nil {
		return err
	}

	logger.Info(logPrefix+" Credentials saved", zap.String("path", w.config.CredentialsPath))
	return nil
}

// performQRLogin 执行交互式二维码登录
func (w *WeixinChannel) performQRLogin(ctx context.Context) (token, baseURL string, err error) {
	logger.Info(logPrefix + " Starting QR login...")
	w.loginStatus = LoginStatusWaiting

	api := newWeixinAPI(w.config.BaseURL, "", w.config.CDNBaseURL, w.client)

	qrcode, _, err := w.fetchInitialQRCode(ctx, api)
	if err != nil {
		return "", "", err
	}

	deadline := time.Now().Add(qrLoginTimeout)
	refreshCount := 0
	scannedPrinted := false

	for {
		if w.isLoginCancelled(ctx) {
			logger.Info(logPrefix + " QR login cancelled")
			return "", "", fmt.Errorf("login cancelled")
		}

		if time.Now().After(deadline) {
			logger.Warn(logPrefix + " QR login timed out")
			return "", "", fmt.Errorf("QR login timed out")
		}

		statusResp, err := api.pollQRStatus(ctx, qrcode)
		if err != nil {
			logger.Error(logPrefix+" QR status poll error", zap.Error(err))
			return "", "", err
		}

		action, newQRCode, _ := w.handleQRStatus(statusResp, &refreshCount, &scannedPrinted, api, ctx)
		switch action {
		case qrActionContinue:
			time.Sleep(1 * time.Second)
			continue
		case qrActionRefresh:
			qrcode = newQRCode
			time.Sleep(1 * time.Second)
			continue
		case qrActionSuccess:
			return statusResp.BotToken, newQRCode, nil
		case qrActionError:
			return "", "", fmt.Errorf("%s", newQRCode)
		}
	}
}

type qrAction int

const (
	qrActionContinue qrAction = iota
	qrActionRefresh
	qrActionSuccess
	qrActionError
)

// fetchInitialQRCode 获取初始二维码
func (w *WeixinChannel) fetchInitialQRCode(ctx context.Context, api *weixinAPI) (qrcode, qrURL string, err error) {
	qrResp, err := api.fetchQRCode(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch QR code: %w", err)
	}

	if qrResp.QRCode == "" {
		return "", "", fmt.Errorf("no QR code returned from server")
	}

	w.currentQRURL = qrResp.QRImageContent
	logger.Info(logPrefix+" QR code generated", zap.String("url", qrResp.QRImageContent))
	w.printQRCode(qrResp.QRImageContent)

	return qrResp.QRCode, qrResp.QRImageContent, nil
}

// isLoginCancelled 检查登录是否被取消
func (w *WeixinChannel) isLoginCancelled(ctx context.Context) bool {
	select {
	case <-w.stopChan:
		return true
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// handleQRStatus 处理二维码状态并返回相应动作
func (w *WeixinChannel) handleQRStatus(statusResp *qrStatusResponse, refreshCount *int, scannedPrinted *bool, api *weixinAPI, ctx context.Context) (action qrAction, data1, data2 string) {
	switch statusResp.Status {
	case "wait":
		return qrActionContinue, "", ""
	case "scaned":
		w.loginStatus = LoginStatusScanned
		if !*scannedPrinted {
			logger.Info(logPrefix + " QR code scanned, waiting for confirmation...")
			*scannedPrinted = true
		}
		return qrActionContinue, "", ""
	case "expired":
		return w.handleQRExpired(refreshCount, scannedPrinted, api, ctx)
	case "confirmed":
		return w.handleQRConfirmed(statusResp)
	}
	return qrActionContinue, "", ""
}

// handleQRExpired 处理二维码过期
func (w *WeixinChannel) handleQRExpired(refreshCount *int, scannedPrinted *bool, api *weixinAPI, ctx context.Context) (action qrAction, qrcode, qrURL string) {
	*refreshCount++
	if *refreshCount >= maxQRRefreshes {
		return qrActionError, fmt.Sprintf("QR code expired after %d refreshes", maxQRRefreshes), ""
	}
	logger.Info(logPrefix+" QR code expired, refreshing...",
		zap.Int("attempt", *refreshCount))

	qrResp, err := api.fetchQRCode(ctx)
	if err != nil {
		return qrActionError, err.Error(), ""
	}

	w.currentQRURL = qrResp.QRImageContent
	w.printQRCode(qrResp.QRImageContent)
	*scannedPrinted = false

	return qrActionRefresh, qrResp.QRCode, qrResp.QRImageContent
}

// handleQRConfirmed 处理登录确认
func (w *WeixinChannel) handleQRConfirmed(statusResp *qrStatusResponse) (action qrAction, baseURL, _ string) {
	if statusResp.BotToken == "" || statusResp.BotID == "" {
		return qrActionError, "login confirmed but missing token/bot_id", ""
	}

	w.currentQRURL = ""
	w.loginStatus = LoginStatusLoggedIn
	logger.Info(logPrefix+" Login successful", zap.String("bot_id", statusResp.BotID))

	resultBaseURL := statusResp.BaseURL
	if resultBaseURL == "" {
		resultBaseURL = w.config.BaseURL
	}

	creds := &Credentials{
		Token:   statusResp.BotToken,
		BaseURL: resultBaseURL,
		BotID:   statusResp.BotID,
		UserID:  statusResp.UserID,
	}
	w.saveCredentials(creds)

	return qrActionSuccess, resultBaseURL, ""
}

// printQRCode 打印二维码到终端
func (w *WeixinChannel) printQRCode(qrURL string) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  Please scan QR code with WeChat (expires in ~2 minutes)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  QR Code URL: %s\n\n", qrURL)
}

// pollLoop 运行主长轮询循环
func (w *WeixinChannel) pollLoop() {
	w.pollWg.Add(1)
	defer w.pollWg.Done()

	logger.Info(logPrefix + " Starting long-poll loop")
	consecutiveFailures := 0

	for {
		if w.isStopped() {
			logger.Info(logPrefix + " Long-poll loop ended")
			return
		}

		resp, err := w.api.getUpdates(w.getUpdatesBuf)
		if err != nil {
			consecutiveFailures = w.handlePollError(err, consecutiveFailures)
			continue
		}

		if w.isSessionExpired(resp) {
			consecutiveFailures = w.handleSessionExpired(consecutiveFailures)
			continue
		}

		if w.hasAPIError(resp) {
			consecutiveFailures = w.handleAPIError(resp, consecutiveFailures)
			continue
		}

		consecutiveFailures = 0
		w.updateSyncCursor(resp)

		for _, rawMsg := range resp.Messages {
			w.processMessage(rawMsg)
		}
	}
}

// isStopped 检查是否已停止
func (w *WeixinChannel) isStopped() bool {
	select {
	case <-w.stopChan:
		return true
	default:
		return false
	}
}

// handlePollError 处理轮询错误
func (w *WeixinChannel) handlePollError(err error, consecutiveFailures int) int {
	consecutiveFailures++
	logger.Error(logPrefix+" getUpdates error",
		zap.Error(err),
		zap.Int("consecutive_failures", consecutiveFailures))

	w.applyRetryDelay(consecutiveFailures)
	return consecutiveFailures
}

// isSessionExpired 检查会话是否过期
func (w *WeixinChannel) isSessionExpired(resp *updatesResponse) bool {
	return resp.ErrCode == sessionExpiredErrCode || resp.Ret == sessionExpiredErrCode
}

// handleSessionExpired 处理会话过期
func (w *WeixinChannel) handleSessionExpired(consecutiveFailures int) int {
	logger.Error(logPrefix + " Session expired, attempting re-login...")
	if w.relogin() {
		logger.Info(logPrefix + " Re-login successful, resuming poll")
		w.getUpdatesBuf = ""
		return 0
	}
	logger.Error(logPrefix + " Re-login failed, will retry in 5 minutes")
	time.Sleep(5 * time.Minute)
	return consecutiveFailures
}

// hasAPIError 检查是否有 API 错误
func (w *WeixinChannel) hasAPIError(resp *updatesResponse) bool {
	return resp.Ret != 0 || resp.ErrCode != 0
}

// handleAPIError 处理 API 错误
func (w *WeixinChannel) handleAPIError(resp *updatesResponse, consecutiveFailures int) int {
	consecutiveFailures++
	logger.Error(logPrefix+" getUpdates API error",
		zap.Int("ret", resp.Ret),
		zap.Int("errcode", resp.ErrCode),
		zap.String("errmsg", resp.ErrMsg))
	w.applyRetryDelay(consecutiveFailures)
	return consecutiveFailures
}

// applyRetryDelay 应用重试延迟
func (w *WeixinChannel) applyRetryDelay(consecutiveFailures int) {
	if consecutiveFailures >= maxConsecutiveFailures {
		time.Sleep(backoffDelay)
	} else {
		time.Sleep(retryDelay)
	}
}

// updateSyncCursor 更新同步游标
func (w *WeixinChannel) updateSyncCursor(resp *updatesResponse) {
	if resp.GetUpdatesBuf != "" {
		w.getUpdatesBuf = resp.GetUpdatesBuf
	}
}

// relogin 尝试在会话过期后重新登录
func (w *WeixinChannel) relogin() bool {
	// 删除凭证文件
	os.Remove(w.config.CredentialsPath)

	w.loginStatus = LoginStatusWaiting
	ctx, cancel := context.WithTimeout(context.Background(), qrLoginTimeout)
	defer cancel()

	token, baseURL, err := w.performQRLogin(ctx)
	if err != nil {
		w.loginStatus = LoginStatusIdle
		return false
	}

	w.api = newWeixinAPI(baseURL, token, w.config.CDNBaseURL, w.client)
	w.loginStatus = LoginStatusLoggedIn

	// 清除上下文令牌
	w.contextMu.Lock()
	w.contextTokens = make(map[string]string)
	w.contextMu.Unlock()

	return true
}

// processMessage 处理单条传入消息
func (w *WeixinChannel) processMessage(rawMsg map[string]any) {
	// 只处理用户消息 (type=1)
	msgType, _ := rawMsg["message_type"].(float64)
	if int(msgType) != messageTypeUser {
		return
	}

	// 去重
	msgID := fmt.Sprintf("%v", rawMsg["message_id"])
	if msgID == "" || msgID == "0" {
		msgID = fmt.Sprintf("%v", rawMsg["seq"])
	}
	if w.receivedMsgs.Exists(msgID) {
		return
	}
	w.receivedMsgs.Set(msgID, true)

	fromUser, _ := rawMsg["from_user_id"].(string)
	contextToken, _ := rawMsg["context_token"].(string)

	// 存储上下文令牌
	if contextToken != "" && fromUser != "" {
		w.contextMu.Lock()
		w.contextTokens[fromUser] = contextToken
		w.contextMu.Unlock()
	}

	// 解析消息
	wxMsg := parseWeixinMessage(rawMsg, w.config.CDNBaseURL, w.tmpDir)
	if wxMsg == nil {
		return
	}

	logger.Info(logPrefix+" Received message",
		zap.String("from", fromUser),
		zap.String("type", wxMsg.ctype.String()),
		zap.String("content", truncateString(wxMsg.content, 50)))

	// 调用消息处理器
	if w.messageHandler != nil {
		ctx := context.Background()
		reply, err := w.messageHandler.HandleMessage(ctx, wxMsg)
		if err != nil {
			logger.Error(logPrefix+" 消息处理错误", zap.Error(err))
			return
		}
		if reply != nil {
			// 创建上下文
			replyCtx := types.NewContext(wxMsg.ctype, wxMsg.content)
			replyCtx.Set("receiver", fromUser)
			replyCtx.Set("msg", wxMsg)

			if err := w.Send(reply, replyCtx); err != nil {
				logger.Error(logPrefix+" 发送回复失败", zap.Error(err))
			}
		}
	}
}

// getContextToken 获取接收者的上下文令牌
func (w *WeixinChannel) getContextToken(receiver string, ctx *types.Context) string {
	// 首先检查消息是否有上下文令牌
	if msg, ok := ctx.Get("msg"); ok {
		if wxMsg, ok := msg.(*WeixinMessage); ok && wxMsg.contextToken != "" {
			return wxMsg.contextToken
		}
	}

	w.contextMu.RLock()
	defer w.contextMu.RUnlock()
	return w.contextTokens[receiver]
}

// 消息发送方法

func (w *WeixinChannel) sendText(text, receiver, contextToken string) error {
	if len(text) <= textChunkLimit {
		return w.api.sendText(receiver, text, contextToken)
	}

	// 分割长文本
	chunks := splitText(text, textChunkLimit)
	for i, chunk := range chunks {
		if err := w.api.sendText(receiver, chunk, contextToken); err != nil {
			return fmt.Errorf("failed to send chunk %d/%d: %w", i+1, len(chunks), err)
		}
		if i < len(chunks)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

func (w *WeixinChannel) sendImage(pathOrURL, receiver, contextToken string) error {
	localPath, err := w.resolveMediaPath(pathOrURL)
	if err != nil {
		w.sendText("[Image send failed: file not found]", receiver, contextToken)
		return err
	}

	result, err := uploadMediaToCDN(w.api, localPath, receiver, mediaTypeImage)
	if err != nil {
		w.sendText("[Image send failed]", receiver, contextToken)
		return err
	}

	return w.api.sendImageItem(receiver, contextToken, result.EncryptQueryParam, result.AESKeyB64, result.CiphertextSize)
}

func (w *WeixinChannel) sendFile(pathOrURL, receiver, contextToken string) error {
	localPath, err := w.resolveMediaPath(pathOrURL)
	if err != nil {
		w.sendText("[File send failed: file not found]", receiver, contextToken)
		return err
	}

	result, err := uploadMediaToCDN(w.api, localPath, receiver, mediaTypeFile)
	if err != nil {
		w.sendText("[File send failed]", receiver, contextToken)
		return err
	}

	fileName := filepath.Base(localPath)
	return w.api.sendFileItem(receiver, contextToken, result.EncryptQueryParam, result.AESKeyB64, fileName, result.RawSize)
}

func (w *WeixinChannel) sendVideo(pathOrURL, receiver, contextToken string) error {
	localPath, err := w.resolveMediaPath(pathOrURL)
	if err != nil {
		w.sendText("[Video send failed: file not found]", receiver, contextToken)
		return err
	}

	result, err := uploadMediaToCDN(w.api, localPath, receiver, mediaTypeVideo)
	if err != nil {
		w.sendText("[Video send failed]", receiver, contextToken)
		return err
	}

	return w.api.sendVideoItem(receiver, contextToken, result.EncryptQueryParam, result.AESKeyB64, result.CiphertextSize)
}

// resolveMediaPath 将文件路径或 URL 解析为本地路径，如需要则下载
func (w *WeixinChannel) resolveMediaPath(pathOrURL string) (string, error) {
	if pathOrURL == "" {
		return "", fmt.Errorf("empty path")
	}

	localPath := strings.TrimPrefix(pathOrURL, "file://")

	if strings.HasPrefix(localPath, "http://") || strings.HasPrefix(localPath, "https://") {
		return w.downloadMedia(localPath)
	}

	if _, err := os.Stat(localPath); err != nil {
		return "", fmt.Errorf("file not found: %s", localPath)
	}

	return localPath, nil
}

// downloadMedia 从 URL 下载文件到临时目录
func (w *WeixinChannel) downloadMedia(urlStr string) (string, error) {
	resp, err := w.client.Get(urlStr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	// 根据内容类型确定扩展名
	ext := ".bin"
	ct := resp.Header.Get(common.HeaderContentType)
	switch {
	case strings.Contains(ct, "jpeg") || strings.Contains(ct, "jpg"):
		ext = ".jpg"
	case strings.Contains(ct, "png"):
		ext = ".png"
	case strings.Contains(ct, "gif"):
		ext = ".gif"
	case strings.Contains(ct, "webp"):
		ext = ".webp"
	case strings.Contains(ct, "mp4"):
		ext = ".mp4"
	case strings.Contains(ct, "pdf"):
		ext = ".pdf"
	}

	// 生成随机文件名
	randBytes := make([]byte, 4)
	rand.Read(randBytes)
	filename := fmt.Sprintf("wx_media_%s%s", hex.EncodeToString(randBytes), ext)
	savePath := filepath.Join(w.tmpDir, filename)

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return "", err
	}

	return savePath, nil
}

// 辅助函数

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func splitText(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= limit {
			chunks = append(chunks, text)
			break
		}

		// 尝试在段落处断开
		cut := strings.LastIndex(text[:limit], "\n\n")
		if cut <= 0 {
			cut = strings.LastIndex(text[:limit], "\n")
		}
		if cut <= 0 {
			cut = limit
		}

		chunks = append(chunks, text[:cut])
		text = strings.TrimLeft(text[cut:], "\n")
	}

	return chunks
}

// expiredMap 提供带过期时间的简单映射（简化实现）
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
	// 启动清理协程
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

// weixinAPI 实现 ilink bot HTTP API
type weixinAPI struct {
	baseURL    string
	token      string
	cdnBaseURL string
	client     *http.Client
}

func newWeixinAPI(baseURL, token, cdnBaseURL string, client *http.Client) *weixinAPI {
	if cdnBaseURL == "" {
		cdnBaseURL = defaultCDNBaseURL
	}
	return &weixinAPI{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		token:      token,
		cdnBaseURL: cdnBaseURL,
		client:     client,
	}
}

func (a *weixinAPI) buildHeaders() map[string]string {
	// 生成随机微信 UIN
	val := make([]byte, 4)
	rand.Read(val)
	uinVal := uint32(val[0])<<24 | uint32(val[1])<<16 | uint32(val[2])<<8 | uint32(val[3])
	uin := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%d", uinVal))

	headers := map[string]string{
		common.HeaderContentType: common.ContentTypeJSON,
		"AuthorizationType":      "ilink_bot_token",
		"X-WECHAT-UIN":           uin,
	}
	if a.token != "" {
		headers["Authorization"] = common.AuthPrefixBearer + a.token
	}
	return headers
}

type qrCodeResponse struct {
	QRCode         string `json:"qrcode"`
	QRImageContent string `json:"qrcode_img_content"`
}

type qrStatusResponse struct {
	Status   string `json:"status"`
	BotToken string `json:"bot_token"`
	BotID    string `json:"ilink_bot_id"`
	BaseURL  string `json:"baseurl"`
	UserID   string `json:"ilink_user_id"`
}

type updatesResponse struct {
	Ret           int              `json:"ret"`
	ErrCode       int              `json:"errcode"`
	ErrMsg        string           `json:"errmsg"`
	GetUpdatesBuf string           `json:"get_updates_buf"`
	Messages      []map[string]any `json:"msgs"`
}

type uploadURLResponse struct {
	UploadParam string `json:"upload_param"`
}

func (a *weixinAPI) fetchQRCode(ctx context.Context) (*qrCodeResponse, error) {
	url := fmt.Sprintf("%s/ilink/bot/get_bot_qrcode?bot_type=3", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result qrCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (a *weixinAPI) pollQRStatus(ctx context.Context, qrcode string) (*qrStatusResponse, error) {
	escaped := url.QueryEscape(qrcode)
	urlStr := fmt.Sprintf("%s/ilink/bot/get_qrcode_status?qrcode=%s", a.baseURL, escaped)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("iLink-App-ClientVersion", "1")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result qrStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (a *weixinAPI) getUpdates(buf string) (*updatesResponse, error) {
	body := map[string]any{"get_updates_buf": buf}
	var result updatesResponse
	if err := a.post("ilink/bot/getupdates", body, &result, int(defaultLongPollTimeout.Seconds())+5); err != nil {
		return nil, err
	}
	return &result, nil
}

func (a *weixinAPI) sendText(to, text, contextToken string) error {
	body := map[string]any{
		"msg": map[string]any{
			"from_user_id":  "",
			"to_user_id":    to,
			"client_id":     generateClientID(),
			"message_type":  messageTypeBot,
			"message_state": 2, // FINISH
			"item_list": []map[string]any{
				{"type": itemText, "text_item": map[string]any{"text": text}},
			},
			"context_token": contextToken,
		},
	}
	return a.post("ilink/bot/sendmessage", body, nil, 0)
}

func (a *weixinAPI) sendImageItem(to, contextToken, encryptParam, aesKeyB64 string, ciphertextSize int) error {
	items := []map[string]any{
		{
			"type": itemImage,
			"image_item": map[string]any{
				"media": map[string]any{
					"encrypt_query_param": encryptParam,
					"aes_key":             aesKeyB64,
					"encrypt_type":        1,
				},
				"mid_size": ciphertextSize,
			},
		},
	}
	return a.sendItems(to, contextToken, items)
}

func (a *weixinAPI) sendFileItem(to, contextToken, encryptParam, aesKeyB64, fileName string, fileSize int) error {
	items := []map[string]any{
		{
			"type": itemFile,
			"file_item": map[string]any{
				"media": map[string]any{
					"encrypt_query_param": encryptParam,
					"aes_key":             aesKeyB64,
					"encrypt_type":        1,
				},
				"file_name": fileName,
				"len":       fmt.Sprintf("%d", fileSize),
			},
		},
	}
	return a.sendItems(to, contextToken, items)
}

func (a *weixinAPI) sendVideoItem(to, contextToken, encryptParam, aesKeyB64 string, ciphertextSize int) error {
	items := []map[string]any{
		{
			"type": itemVideo,
			"video_item": map[string]any{
				"media": map[string]any{
					"encrypt_query_param": encryptParam,
					"aes_key":             aesKeyB64,
					"encrypt_type":        1,
				},
				"video_size": ciphertextSize,
			},
		},
	}
	return a.sendItems(to, contextToken, items)
}

func (a *weixinAPI) sendItems(to, contextToken string, items []map[string]any) error {
	body := map[string]any{
		"msg": map[string]any{
			"from_user_id":  "",
			"to_user_id":    to,
			"client_id":     generateClientID(),
			"message_type":  messageTypeBot,
			"message_state": 2,
			"item_list":     items,
			"context_token": contextToken,
		},
	}
	return a.post("ilink/bot/sendmessage", body, nil, 0)
}

func (a *weixinAPI) getUploadURL(fileKey string, mediaType int, toUserID string, rawSize int, rawMD5 string, fileSize int, aesKey string) (*uploadURLResponse, error) {
	body := map[string]any{
		"filekey":       fileKey,
		"media_type":    mediaType,
		"to_user_id":    toUserID,
		"rawsize":       rawSize,
		"rawfilemd5":    rawMD5,
		"filesize":      fileSize,
		"aeskey":        aesKey,
		"no_need_thumb": true,
	}
	var result uploadURLResponse
	if err := a.post("ilink/bot/getuploadurl", body, &result, 0); err != nil {
		return nil, err
	}
	return &result, nil
}

func (a *weixinAPI) post(endpoint string, body any, result any, timeoutSec int) error {
	url := a.baseURL + "/" + endpoint

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}

	for k, v := range a.buildHeaders() {
		req.Header.Set(k, v)
	}

	client := a.client
	if timeoutSec > 0 {
		client = &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}

func generateClientID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CDNUploadResult 保存 CDN 媒体上传结果
type CDNUploadResult struct {
	EncryptQueryParam string
	AESKeyB64         string
	CiphertextSize    int
	RawSize           int
}

// uploadMediaToCDN 上传文件到微信 CDN
func uploadMediaToCDN(api *weixinAPI, filePath, toUserID string, mediaType int) (*CDNUploadResult, error) {
	// 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	rawSize := len(data)
	rawMD5 := md5Hash(data)

	// 生成 AES 密钥
	aesKey := make([]byte, 16)
	rand.Read(aesKey)
	aesKeyHex := hex.EncodeToString(aesKey)

	// 生成文件密钥
	fileKey := generateClientID() + generateClientID()

	// 计算填充后大小
	cipherSize := aesECBPaddedSize(rawSize)

	// 获取上传 URL
	uploadResp, err := api.getUploadURL(fileKey, mediaType, toUserID, rawSize, rawMD5, cipherSize, aesKeyHex)
	if err != nil {
		return nil, err
	}

	if uploadResp.UploadParam == "" {
		return nil, fmt.Errorf("no upload_param returned")
	}

	// 加密数据
	encrypted := aesECBEncrypt(data, aesKey)

	// 上传到 CDN
	cdnURL := fmt.Sprintf("%s/upload?encrypted_query_param=%s&filekey=%s",
		api.cdnBaseURL, url.QueryEscape(uploadResp.UploadParam), url.QueryEscape(fileKey))

	cdnReq, err := http.NewRequest(http.MethodPost, cdnURL, strings.NewReader(string(encrypted)))
	if err != nil {
		return nil, err
	}
	cdnReq.Header.Set(common.HeaderContentType, "application/octet-stream")

	client := &http.Client{Timeout: 2 * time.Minute}
	cdnResp, err := client.Do(cdnReq)
	if err != nil {
		return nil, err
	}
	defer cdnResp.Body.Close()

	if cdnResp.StatusCode >= 400 {
		errMsg := cdnResp.Header.Get("x-error-message")
		if errMsg == "" {
			body, _ := io.ReadAll(cdnResp.Body)
			errMsg = string(body[:min(200, len(body))])
		}
		return nil, fmt.Errorf("CDN error %d: %s", cdnResp.StatusCode, errMsg)
	}

	downloadParam := cdnResp.Header.Get("x-encrypted-param")
	if downloadParam == "" {
		return nil, fmt.Errorf("CDN response missing x-encrypted-param")
	}

	aesKeyB64 := base64.StdEncoding.EncodeToString([]byte(aesKeyHex))

	return &CDNUploadResult{
		EncryptQueryParam: downloadParam,
		AESKeyB64:         aesKeyB64,
		CiphertextSize:    cipherSize,
		RawSize:           rawSize,
	}, nil
}
