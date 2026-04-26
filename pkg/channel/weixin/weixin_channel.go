// Package weixin 提供微信个人号渠道实现
// 本包实现了 ilink bot 协议用于微信通信（支持多账号）
package weixin

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
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
	// QR 登录与重试设置（仅在本文件使用）
	qrLoginTimeout         = 480 * time.Second
	maxQRRefreshes         = 10
	sessionExpiredErrCode  = -14
	maxConsecutiveFailures = 3
	backoffDelay           = 30 * time.Second
	retryDelay             = 2 * time.Second
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
	BaseURL               string `json:"base_url"`
	CDNBaseURL            string `json:"cdn_base_url"`
	Token                 string `json:"token"`
	CredentialsPath       string `json:"credentials_path"`
	MarkdownFilterEnabled *bool  `json:"markdown_filter_enabled"` // nil=默认启用
	SessionPauseMinutes   int    `json:"session_pause_minutes"`   // 会话暂停时长（分钟），0=默认
}

// Credentials 存储的登录凭证
type Credentials struct {
	Token   string `json:"token"`
	BaseURL string `json:"base_url"`
	BotID   string `json:"bot_id"`
	UserID  string `json:"user_id"`
}

// WeixinChannel 实现微信个人号渠道（支持多账号）
type WeixinChannel struct {
	*channel.BaseChannel

	config *Config

	// 多账号管理
	accounts   map[string]*accountState // 账号ID→账号状态
	accountsMu sync.RWMutex             // 账号映射读写锁

	// 共享资源
	client   *http.Client // HTTP 客户端（所有账号共享）
	stateDir string       // 状态目录路径（~/.weixin）

	// 持久化存储
	syncBufStore      *SyncBufStore     // 同步游标存储
	contextTokenStore *ContextTokenStore // 上下文令牌存储

	// 会话暂停守卫
	sessionGuard *SessionGuard

	// Markdown 过滤器
	markdownFilter *MarkdownFilter

	// Typing 缓存
	typingCache *TypingTicketCache

	// 消息处理器
	messageHandler MessageHandler

	// 全局停止信号
	stopChan chan struct{}
	stopWg   sync.WaitGroup

	// 登录状态（兼容旧接口）
	loginStatus  LoginStatus
	currentQRURL string
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
		BaseChannel:    channel.NewBaseChannel("weixin"),
		accounts:       make(map[string]*accountState),
		loginStatus:    LoginStatusIdle,
		stopChan:       make(chan struct{}),
		markdownFilter: NewMarkdownFilter(true), // 默认启用
		typingCache:    NewTypingTicketCache(),
	}
}

// Startup 启动微信渠道
func (w *WeixinChannel) Startup(ctx context.Context) error {
	// 初始化配置
	w.initConfig()

	// 设置 HTTP 客户端（共享）
	w.client = &http.Client{
		Timeout: defaultAPITimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}

	// 设置状态目录
	w.stateDir = resolveStateDir()

	// 初始化持久化存储
	accountsDir := resolveAccountsDir()
	w.syncBufStore = NewSyncBufStore(accountsDir)
	w.contextTokenStore = NewContextTokenStore(accountsDir)

	// 迁移旧版凭证（如果存在）
	if w.config.CredentialsPath != "" {
		migrateLegacyCredentials(w.config.CredentialsPath, w.stateDir)
	}

	// 加载已有账号
	accountIDs := listAccountIDs(w.stateDir)

	if len(accountIDs) == 0 {
		// 无已有账号，执行 QR 登录创建第一个账号
		if err := w.addAccountWithQRLogin(ctx, ""); err != nil {
			return fmt.Errorf("首次 QR 登录失败: %w", err)
		}
	} else {
		// 启动所有已有账号
		for _, accountID := range accountIDs {
			if err := w.startExistingAccount(accountID); err != nil {
				logger.Warn(logPrefix+" 启动账号失败，跳过",
					zap.String("account_id", accountID),
					zap.Error(err))
				continue
			}
		}
	}

	// 检查是否有至少一个活跃账号
	w.accountsMu.RLock()
	activeCount := len(w.accounts)
	w.accountsMu.RUnlock()

	if activeCount == 0 {
		return fmt.Errorf("没有可用的微信账号")
	}

	logger.Info(logPrefix+" 渠道启动成功",
		zap.Int("account_count", activeCount))

	w.ReportStartupSuccess()
	return nil
}

// Stop 停止微信渠道
func (w *WeixinChannel) Stop() error {
	logger.Info(logPrefix + " Stop() called")
	close(w.stopChan)

	// 停止所有账号的轮询
	w.accountsMu.RLock()
	for _, as := range w.accounts {
		close(as.stopChan)
	}
	w.accountsMu.RUnlock()

	// 等待所有轮询 goroutine 退出
	w.accountsMu.RLock()
	for _, as := range w.accounts {
		as.pollWg.Wait()
	}
	w.accountsMu.RUnlock()

	// 保存所有账号的持久化数据
	w.accountsMu.RLock()
	for id, as := range w.accounts {
		w.syncBufStore.Save(id, as.getUpdatesBuf)
		w.contextTokenStore.Save(id, as.contextTokens)
	}
	w.accountsMu.RUnlock()

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
		logger.Error(logPrefix + " 上下文中未找到接收者")
		return fmt.Errorf("no receiver in context")
	}

	// 检查会话是否暂停
	accountID, _ := ctx.GetString("account_id")
	if accountID != "" {
		if err := w.sessionGuard.AssertActive(accountID); err != nil {
			logger.Warn(logPrefix+" 发送被阻止：会话已暂停",
				zap.String("account_id", accountID),
				zap.Error(err))
			return err
		}
	}

	// 查找接收者对应的账号
	as := w.findAccountForReceiver(receiver, accountID)
	if as == nil {
		logger.Error(logPrefix+" 未找到接收者对应的账号",
			zap.String("receiver", receiver))
		return fmt.Errorf("no account for receiver: %s", receiver)
	}

	// 获取 contextToken
	contextToken := w.getContextTokenForAccount(as, receiver, ctx)
	if contextToken == "" {
		logger.Error(logPrefix+" 未找到接收者的 contextToken",
			zap.String("receiver", receiver),
			zap.String("account_id", as.id))
		return fmt.Errorf("no context_token for receiver: %s", receiver)
	}

	switch reply.Type {
	case types.ReplyText, types.ReplyText_:
		text := w.markdownFilter.Filter(reply.StringContent())
		return w.sendText(as, text, receiver, contextToken)
	case types.ReplyImage, types.ReplyImageURL:
		return w.sendImage(as, reply.StringContent(), receiver, contextToken)
	case types.ReplyFile:
		return w.sendFile(as, reply.StringContent(), receiver, contextToken)
	case types.ReplyVideo, types.ReplyVideoURL:
		return w.sendVideo(as, reply.StringContent(), receiver, contextToken)
	default:
		text := w.markdownFilter.Filter(reply.StringContent())
		return w.sendText(as, text, receiver, contextToken)
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
	// 根据 MarkdownFilterEnabled 配置更新过滤器
	if w.config.MarkdownFilterEnabled != nil {
		w.markdownFilter = NewMarkdownFilter(*w.config.MarkdownFilterEnabled)
	}
	// 根据 SessionPauseMinutes 配置更新会话暂停时长
	if w.config.SessionPauseMinutes > 0 {
		w.sessionGuard = NewSessionGuard(w.config.SessionPauseMinutes)
	}
}

// --- 多账号管理 ---

// findAccountForReceiver 根据接收者查找对应的账号
// 如果指定了 accountID 则直接使用，否则遍历所有账号查找
func (w *WeixinChannel) findAccountForReceiver(receiver, accountID string) *accountState {
	w.accountsMu.RLock()
	defer w.accountsMu.RUnlock()

	// 指定了账号 ID，直接查找
	if accountID != "" {
		return w.accounts[accountID]
	}

	// 遍历所有账号，查找拥有该接收者 contextToken 的账号
	for _, as := range w.accounts {
		as.contextMu.RLock()
		_, exists := as.contextTokens[receiver]
		as.contextMu.RUnlock()
		if exists {
			return as
		}
	}

	// 兜底：返回第一个可用账号
	for _, as := range w.accounts {
		return as
	}
	return nil
}

// getContextTokenForAccount 从指定账号获取接收者的 contextToken
func (w *WeixinChannel) getContextTokenForAccount(as *accountState, receiver string, ctx *types.Context) string {
	// 首先检查消息是否有上下文令牌
	if msg, ok := ctx.Get("msg"); ok {
		if wxMsg, ok := msg.(*WeixinMessage); ok && wxMsg.contextToken != "" {
			return wxMsg.contextToken
		}
	}
	as.contextMu.RLock()
	defer as.contextMu.RUnlock()
	return as.contextTokens[receiver]
}

// startExistingAccount 启动已有凭证的账号
func (w *WeixinChannel) startExistingAccount(accountID string) error {
	creds, err := loadAccountCredential(w.stateDir, accountID)
	if err != nil {
		return fmt.Errorf("加载凭证失败: %w", err)
	}

	baseURL := creds.BaseURL
	if baseURL == "" {
		baseURL = w.config.BaseURL
	}

	as := w.newAccountState(normalizeAccountID(accountID))
	as.credentials = creds
	as.api = newWeixinAPI(baseURL, creds.Token, w.config.CDNBaseURL, w.client)
	as.loginStatus = LoginStatusLoggedIn

	// 从持久化存储加载同步游标和 contextToken
	as.getUpdatesBuf = w.syncBufStore.Load(as.id)
	as.contextTokens = w.contextTokenStore.Load(as.id)

	// 设置临时目录
	home, _ := os.UserHomeDir()
	as.tmpDir = filepath.Join(home, "cow", "tmp", as.id)
	os.MkdirAll(as.tmpDir, 0755)

	// 注册账号
	w.accountsMu.Lock()
	w.accounts[as.id] = as
	w.accountsMu.Unlock()

	// 启动轮询
	go w.pollLoop(as)

	logger.Info(logPrefix+" 账号已启动",
		zap.String("account_id", as.id))
	return nil
}

// addAccountWithQRLogin 通过 QR 登录添加新账号
// label 用于标识账号（为空则自动生成）
func (w *WeixinChannel) addAccountWithQRLogin(ctx context.Context, label string) error {
	w.loginStatus = LoginStatusWaiting

	api := newWeixinAPI(w.config.BaseURL, "", w.config.CDNBaseURL, w.client)
	qrcode, _, err := w.fetchInitialQRCode(ctx, api)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(qrLoginTimeout)
	refreshCount := 0
	scannedPrinted := false

	for {
		if w.isLoginCancelled(ctx) {
			return fmt.Errorf("登录取消")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("QR 登录超时")
		}

		statusResp, err := api.pollQRStatus(ctx, qrcode)
		if err != nil {
			return err
		}

		action, data1, _ := w.handleQRStatus(statusResp, &refreshCount, &scannedPrinted, api, ctx)
		switch action {
		case qrActionContinue:
			time.Sleep(1 * time.Second)
		case qrActionRefresh:
			qrcode = data1
			time.Sleep(1 * time.Second)
		case qrActionRedirect:
			// IDC 重定向已由 handleQRStatus 更新 api.baseURL
			// 继续用新的 base URL 轮询
			time.Sleep(1 * time.Second)
		case qrActionSuccess:
			// handleQRConfirmed 已保存凭证到多账号目录
			// 现在创建 accountState 并启动轮询
			accountID := statusResp.UserID
			if accountID == "" {
				accountID = "default"
			}
			normalizedID := normalizeAccountID(accountID)

			// 清理同 userId 的旧账号
			clearStaleAccountsForUserId(w.stateDir, normalizedID, accountID, func(staleID string) {
				// 停止旧账号的轮询
				w.accountsMu.Lock()
				if old, ok := w.accounts[staleID]; ok {
					close(old.stopChan)
					old.pollWg.Wait()
					delete(w.accounts, staleID)
				}
				w.accountsMu.Unlock()
			})

			// 注册新账号
			registerAccountID(w.stateDir, normalizedID)

			// 创建并启动
			as := w.newAccountState(normalizedID)
			baseURL := data1 // data1 是 baseURL（来自 handleQRConfirmed）
			if baseURL == "" {
				baseURL = w.config.BaseURL
			}
			as.api = newWeixinAPI(baseURL, statusResp.BotToken, w.config.CDNBaseURL, w.client)
			as.credentials = &Credentials{
				Token:   statusResp.BotToken,
				BaseURL: baseURL,
				BotID:   statusResp.BotID,
				UserID:  accountID,
			}
			as.loginStatus = LoginStatusLoggedIn
			home, _ := os.UserHomeDir()
			as.tmpDir = filepath.Join(home, "cow", "tmp", as.id)
			os.MkdirAll(as.tmpDir, 0755)

			w.accountsMu.Lock()
			w.accounts[as.id] = as
			w.accountsMu.Unlock()

			go w.pollLoop(as)

			w.loginStatus = LoginStatusLoggedIn
			w.currentQRURL = ""

			logger.Info(logPrefix+" 新账号登录成功",
				zap.String("account_id", as.id))
			return nil
		case qrActionError:
			return fmt.Errorf("%s", data1)
		}
	}
}

// newAccountState 创建新的账号状态实例
func (w *WeixinChannel) newAccountState(id string) *accountState {
	return &accountState{
		id:            id,
		loginStatus:   LoginStatusIdle,
		stopChan:      make(chan struct{}),
		contextTokens: make(map[string]string),
		receivedMsgs:  newExpiredMap(7*time.Hour + 6*time.Minute),
	}
}

// --- QR 登录 ---

// qrAction 二维码状态处理动作
type qrAction int

const (
	qrActionContinue qrAction = iota
	qrActionRefresh
	qrActionSuccess
	qrActionRedirect // IDC 重定向
	qrActionError
)

// fetchInitialQRCode 获取初始二维码
func (w *WeixinChannel) fetchInitialQRCode(ctx context.Context, api *weixinAPI) (qrcode, qrURL string, err error) {
	qrResp, err := api.fetchQRCode(ctx)
	if err != nil {
		return "", "", fmt.Errorf("获取二维码失败: %w", err)
	}

	if qrResp.QRCode == "" {
		return "", "", fmt.Errorf("服务器未返回二维码")
	}

	w.currentQRURL = qrResp.QRImageContent
	logger.Info(logPrefix+" 二维码已生成", zap.String("url", qrResp.QRImageContent))
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
			logger.Info(logPrefix + " 二维码已扫描，等待确认...")
			*scannedPrinted = true
		}
		return qrActionContinue, "", ""
	case "scaned_but_redirect":
		redirectHost := statusResp.RedirectHost
		if redirectHost != "" {
			newBaseURL := "https://" + redirectHost
			api.updateBaseURL(newBaseURL)
			logger.Info(logPrefix+" IDC 重定向，切换到新节点",
				zap.String("redirect_host", redirectHost),
				zap.String("new_base_url", newBaseURL))
		} else {
			logger.Warn(logPrefix + " 收到 scaned_but_redirect 但缺少 redirect_host，继续使用当前节点")
		}
		return qrActionRedirect, "", ""
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
		return qrActionError, fmt.Sprintf("二维码在 %d 次刷新后仍过期", maxQRRefreshes), ""
	}
	logger.Info(logPrefix+" 二维码已过期，刷新中...",
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
		return qrActionError, "登录已确认但缺少 token/bot_id", ""
	}

	w.currentQRURL = ""
	w.loginStatus = LoginStatusLoggedIn
	logger.Info(logPrefix+" 登录成功", zap.String("bot_id", statusResp.BotID))

	resultBaseURL := statusResp.BaseURL
	if resultBaseURL == "" {
		resultBaseURL = w.config.BaseURL
	}

	// 保存凭证到多账号目录
	accountID := statusResp.UserID
	if accountID == "" {
		accountID = "default"
	}
	normalizedID := normalizeAccountID(accountID)

	creds := &Credentials{
		Token:   statusResp.BotToken,
		BaseURL: resultBaseURL,
		BotID:   statusResp.BotID,
		UserID:  accountID,
	}

	// 保存到多账号凭证路径
	if err := saveAccountCredential(w.stateDir, normalizedID, creds); err != nil {
		logger.Warn(logPrefix+" 保存凭证失败", zap.Error(err))
	}

	return qrActionSuccess, resultBaseURL, ""
}

// printQRCode 打印二维码到终端
func (w *WeixinChannel) printQRCode(qrURL string) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("  请使用微信扫描二维码（约 2 分钟内过期）")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  二维码地址: %s\n\n", qrURL)
}

// --- 轮询循环 ---

// pollLoop 运行指定账号的长轮询循环
func (w *WeixinChannel) pollLoop(as *accountState) {
	w.stopWg.Add(1)
	as.pollWg.Add(1)
	defer w.stopWg.Done()
	defer as.pollWg.Done()

	logger.Info(logPrefix+" 启动长轮询",
		zap.String("account_id", as.id))
	consecutiveFailures := 0

	for {
		select {
		case <-w.stopChan:
			return
		case <-as.stopChan:
			return
		default:
		}

		resp, err := as.api.getUpdates(as.getUpdatesBuf)
		if err != nil {
			consecutiveFailures = w.handlePollError(err, consecutiveFailures, as)
			continue
		}

		if w.isSessionExpired(resp) {
			consecutiveFailures = w.handleSessionExpired(consecutiveFailures, as)
			continue
		}

		if w.hasAPIError(resp) {
			consecutiveFailures = w.handleAPIError(resp, consecutiveFailures, as)
			continue
		}

		consecutiveFailures = 0
		w.updateSyncCursor(resp, as)

		// 检查 IDC 重定向
		if resp.BaseURL != "" && resp.BaseURL != as.api.baseURL {
			logger.Info(logPrefix+" getUpdates 返回新 base URL，切换 IDC",
				zap.String("account_id", as.id),
				zap.String("old_base_url", as.api.baseURL),
				zap.String("new_base_url", resp.BaseURL))
			as.api.updateBaseURL(resp.BaseURL)
			// 持久化新 URL
			as.credentials.BaseURL = resp.BaseURL
			if err := saveAccountCredential(w.stateDir, as.id, as.credentials); err != nil {
				logger.Warn(logPrefix+" 保存重定向 URL 失败", zap.Error(err))
			}
		}

		for _, rawMsg := range resp.Messages {
			w.processMessage(rawMsg, as)
		}
	}
}

// handlePollError 处理轮询错误
func (w *WeixinChannel) handlePollError(err error, consecutiveFailures int, as *accountState) int {
	consecutiveFailures++
	logger.Error(logPrefix+" getUpdates 错误",
		zap.String("account_id", as.id),
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
// 优先暂停等待恢复，连续暂停超过阈值后触发重新登录
func (w *WeixinChannel) handleSessionExpired(consecutiveFailures int, as *accountState) int {
	// 检查是否应该触发重新登录
	if w.sessionGuard.ShouldRelogin(as.id) {
		logger.Error(logPrefix+" 连续暂停次数过多，尝试重新登录...",
			zap.String("account_id", as.id))
		if w.relogin(as) {
			logger.Info(logPrefix+" 重新登录成功，恢复轮询",
				zap.String("account_id", as.id))
			w.sessionGuard.ClearPause(as.id)
			as.getUpdatesBuf = ""
			return 0
		}
		logger.Error(logPrefix+" 重新登录失败，5 分钟后重试",
			zap.String("account_id", as.id))
		time.Sleep(5 * time.Minute)
		return consecutiveFailures
	}

	// 暂停会话，等待自动恢复
	w.sessionGuard.Pause(as.id)
	remaining := w.sessionGuard.GetRemainingPause(as.id)

	logger.Warn(logPrefix+" 会话已过期，暂停等待恢复",
		zap.String("account_id", as.id),
		zap.Duration("pause_duration", remaining))

	// 可中断的等待
	select {
	case <-w.stopChan:
		return consecutiveFailures
	case <-as.stopChan:
		return consecutiveFailures
	case <-time.After(remaining):
		logger.Info(logPrefix+" 暂停到期，恢复轮询",
			zap.String("account_id", as.id))
	}

	return 0
}

// hasAPIError 检查是否有 API 错误
func (w *WeixinChannel) hasAPIError(resp *updatesResponse) bool {
	return resp.Ret != 0 || resp.ErrCode != 0
}

// handleAPIError 处理 API 错误
func (w *WeixinChannel) handleAPIError(resp *updatesResponse, consecutiveFailures int, as *accountState) int {
	consecutiveFailures++
	logger.Error(logPrefix+" getUpdates API 错误",
		zap.String("account_id", as.id),
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

// updateSyncCursor 更新同步游标并持久化
func (w *WeixinChannel) updateSyncCursor(resp *updatesResponse, as *accountState) {
	if resp.GetUpdatesBuf != "" {
		as.getUpdatesBuf = resp.GetUpdatesBuf
		// 异步持久化同步游标
		go w.syncBufStore.Save(as.id, as.getUpdatesBuf)
	}
}

// relogin 尝试在会话过期后重新登录指定账号
func (w *WeixinChannel) relogin(as *accountState) bool {
	// 删除账号凭证文件
	removeAccountCredential(w.stateDir, as.id)

	as.loginStatus = LoginStatusWaiting
	ctx, cancel := context.WithTimeout(context.Background(), qrLoginTimeout)
	defer cancel()

	api := newWeixinAPI(w.config.BaseURL, "", w.config.CDNBaseURL, w.client)
	qrcode, _, err := w.fetchInitialQRCode(ctx, api)
	if err != nil {
		as.loginStatus = LoginStatusIdle
		return false
	}

	deadline := time.Now().Add(qrLoginTimeout)
	refreshCount := 0
	scannedPrinted := false

	for {
		if w.isLoginCancelled(ctx) {
			as.loginStatus = LoginStatusIdle
			return false
		}
		if time.Now().After(deadline) {
			as.loginStatus = LoginStatusIdle
			return false
		}

		statusResp, err := api.pollQRStatus(ctx, qrcode)
		if err != nil {
			as.loginStatus = LoginStatusIdle
			return false
		}

		action, data1, _ := w.handleQRStatus(statusResp, &refreshCount, &scannedPrinted, api, ctx)
		switch action {
		case qrActionContinue:
			time.Sleep(1 * time.Second)
		case qrActionRefresh:
			qrcode = data1
			time.Sleep(1 * time.Second)
		case qrActionRedirect:
			// IDC 重定向已更新 api.baseURL
			time.Sleep(1 * time.Second)
		case qrActionSuccess:
			baseURL := data1
			if baseURL == "" {
				baseURL = w.config.BaseURL
			}
			as.api = newWeixinAPI(baseURL, statusResp.BotToken, w.config.CDNBaseURL, w.client)
			as.loginStatus = LoginStatusLoggedIn

			// 保存新凭证
			accountID := statusResp.UserID
			if accountID == "" {
				accountID = "default"
			}
			normalizedID := normalizeAccountID(accountID)
			creds := &Credentials{
				Token:   statusResp.BotToken,
				BaseURL: baseURL,
				BotID:   statusResp.BotID,
				UserID:  accountID,
			}
			saveAccountCredential(w.stateDir, normalizedID, creds)
			as.credentials = creds

			// 清除上下文令牌
			as.contextMu.Lock()
			as.contextTokens = make(map[string]string)
			as.contextMu.Unlock()

			// 清除暂停状态
			w.sessionGuard.ClearPause(as.id)

			return true
		case qrActionError:
			as.loginStatus = LoginStatusIdle
			return false
		}
	}
}

// --- 消息处理 ---

// processMessage 处理单条传入消息
func (w *WeixinChannel) processMessage(rawMsg map[string]any, as *accountState) {
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
	if as.receivedMsgs.Exists(msgID) {
		return
	}
	as.receivedMsgs.Set(msgID, true)

	fromUser, _ := rawMsg["from_user_id"].(string)
	contextToken, _ := rawMsg["context_token"].(string)

	// 存储上下文令牌
	if contextToken != "" && fromUser != "" {
		as.contextMu.Lock()
		as.contextTokens[fromUser] = contextToken
		as.contextMu.Unlock()
		// 异步持久化 contextToken
		go w.contextTokenStore.Save(as.id, as.contextTokens)
	}

	// 解析消息
	wxMsg := parseWeixinMessage(rawMsg, w.config.CDNBaseURL, as.tmpDir)
	if wxMsg == nil {
		return
	}

	logger.Info(logPrefix+" 收到消息",
		zap.String("account_id", as.id),
		zap.String("from", fromUser),
		zap.String("type", wxMsg.ctype.String()),
		zap.String("content", truncateString(wxMsg.content, 50)))

	// 调用消息处理器（带 typing 指示器）
	if w.messageHandler != nil {
		ctx := context.Background()

		// 启动 typing 指示器
		typingCtrl := NewTypingController(as.api, w.typingCache)
		stopTyping := typingCtrl.StartTyping(fromUser, contextToken)
		defer stopTyping()

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
			replyCtx.Set("account_id", as.id)

			if err := w.Send(reply, replyCtx); err != nil {
				logger.Error(logPrefix+" 发送回复失败", zap.Error(err))
			}
		}
	}
}

// --- 消息发送方法 ---

// sendText 发送文本消息
func (w *WeixinChannel) sendText(as *accountState, text, receiver, contextToken string) error {
	if len(text) <= textChunkLimit {
		return as.api.sendText(receiver, text, contextToken)
	}

	// 分割长文本
	chunks := splitText(text, textChunkLimit)
	for i, chunk := range chunks {
		if err := as.api.sendText(receiver, chunk, contextToken); err != nil {
			return fmt.Errorf("发送分片 %d/%d 失败: %w", i+1, len(chunks), err)
		}
		if i < len(chunks)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

// sendImage 发送图片消息
func (w *WeixinChannel) sendImage(as *accountState, pathOrURL, receiver, contextToken string) error {
	localPath, err := w.resolveMediaPath(pathOrURL, as)
	if err != nil {
		w.sendText(as, "[图片发送失败：文件未找到]", receiver, contextToken)
		return err
	}

	result, err := uploadMediaToCDN(as.api, localPath, receiver, mediaTypeImage)
	if err != nil {
		w.sendText(as, "[图片发送失败]", receiver, contextToken)
		return err
	}

	return as.api.sendImageItem(receiver, contextToken, result.EncryptQueryParam, result.AESKeyB64, result.CiphertextSize)
}

// sendFile 发送文件消息
func (w *WeixinChannel) sendFile(as *accountState, pathOrURL, receiver, contextToken string) error {
	localPath, err := w.resolveMediaPath(pathOrURL, as)
	if err != nil {
		w.sendText(as, "[文件发送失败：文件未找到]", receiver, contextToken)
		return err
	}

	result, err := uploadMediaToCDN(as.api, localPath, receiver, mediaTypeFile)
	if err != nil {
		w.sendText(as, "[文件发送失败]", receiver, contextToken)
		return err
	}

	fileName := filepath.Base(localPath)
	return as.api.sendFileItem(receiver, contextToken, result.EncryptQueryParam, result.AESKeyB64, fileName, result.RawSize)
}

// sendVideo 发送视频消息
func (w *WeixinChannel) sendVideo(as *accountState, pathOrURL, receiver, contextToken string) error {
	localPath, err := w.resolveMediaPath(pathOrURL, as)
	if err != nil {
		w.sendText(as, "[视频发送失败：文件未找到]", receiver, contextToken)
		return err
	}

	result, err := uploadMediaToCDN(as.api, localPath, receiver, mediaTypeVideo)
	if err != nil {
		w.sendText(as, "[视频发送失败]", receiver, contextToken)
		return err
	}

	return as.api.sendVideoItem(receiver, contextToken, result.EncryptQueryParam, result.AESKeyB64, result.CiphertextSize)
}

// resolveMediaPath 将文件路径或 URL 解析为本地路径，如需要则下载
func (w *WeixinChannel) resolveMediaPath(pathOrURL string, as *accountState) (string, error) {
	if pathOrURL == "" {
		return "", fmt.Errorf("路径为空")
	}

	localPath := strings.TrimPrefix(pathOrURL, "file://")

	if strings.HasPrefix(localPath, "http://") || strings.HasPrefix(localPath, "https://") {
		return w.downloadMedia(localPath, as)
	}

	if _, err := os.Stat(localPath); err != nil {
		return "", fmt.Errorf("文件未找到: %s", localPath)
	}

	return localPath, nil
}

// downloadMedia 从 URL 下载文件到账号的临时目录
func (w *WeixinChannel) downloadMedia(urlStr string, as *accountState) (string, error) {
	resp, err := w.client.Get(urlStr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败: %s", resp.Status)
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
	savePath := filepath.Join(as.tmpDir, filename)

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return "", err
	}

	return savePath, nil
}

// --- 辅助函数 ---

// truncateString 截断字符串到指定长度
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// splitText 将文本按指定长度分割
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

// --- 消息去重 ---

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

// --- CDN 上传 ---

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
		return nil, fmt.Errorf("未返回 upload_param")
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
		return nil, fmt.Errorf("CDN 错误 %d: %s", cdnResp.StatusCode, errMsg)
	}

	downloadParam := cdnResp.Header.Get("x-encrypted-param")
	if downloadParam == "" {
		return nil, fmt.Errorf("CDN 响应缺少 x-encrypted-param")
	}

	aesKeyB64 := base64.StdEncoding.EncodeToString([]byte(aesKeyHex))

	return &CDNUploadResult{
		EncryptQueryParam: downloadParam,
		AESKeyB64:         aesKeyB64,
		CiphertextSize:    cipherSize,
		RawSize:           rawSize,
	}, nil
}
