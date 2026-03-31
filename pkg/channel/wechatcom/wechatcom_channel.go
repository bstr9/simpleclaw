// Package wechatcom 提供企业微信应用渠道实现。
// 该包处理企业微信应用的消息接收、加解密和发送。
package wechatcom

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	"go.uber.org/zap"
)

const (
	// MaxTextLen 企业微信消息最大 UTF-8 文本长度
	MaxTextLen = 2048

	// DefaultPort 默认 HTTP 服务器端口
	DefaultPort = 9898

	// DefaultPath 默认回调 URL 路径
	DefaultPath = "/wxcomapp"

	// TokenRefreshInterval 访问令牌刷新检查间隔
	TokenRefreshInterval = 60 * time.Second

	// TokenRefreshAhead 在过期前 10 分钟刷新令牌
	TokenRefreshAhead = 600 * time.Second
)

// Config 企业微信渠道配置
type Config struct {
	CorpID         string `json:"corp_id" mapstructure:"wecom_corp_id"`
	AgentID        int    `json:"agent_id" mapstructure:"wecom_agent_id"`
	Secret         string `json:"secret" mapstructure:"wecom_secret"`
	Token          string `json:"token" mapstructure:"wecom_token"`
	EncodingAESKey string `json:"encoding_aes_key" mapstructure:"wecom_encoding_aes_key"`
	Port           int    `json:"port" mapstructure:"wecom_port"`
	CallbackPath   string `json:"callback_path"`
}

// WechatcomChannel 实现企业微信应用渠道
type WechatcomChannel struct {
	*channel.BaseChannel

	config *Config
	crypto *WechatCrypto
	client *WechatClient

	// HTTP server
	server     *http.Server
	serverOnce sync.Once
	stopChan   chan struct{}

	// Temporary directory for media files
	tmpDir string

	// Message handler
	msgHandler MessageHandler
}

// MessageHandler 处理接收到的消息
type MessageHandler func(msg *WechatMessage)

// NewWechatcomChannel 创建新的企业微信渠道实例
func NewWechatcomChannel() *WechatcomChannel {
	return &WechatcomChannel{
		BaseChannel: channel.NewBaseChannel("wechatcom_app"),
		stopChan:    make(chan struct{}),
	}
}

// initConfig 从全局配置初始化配置
func (w *WechatcomChannel) initConfig() {
	if w.config == nil {
		cfg := config.Get()
		w.config = &Config{
			CorpID:         cfg.WecomCorpID,
			AgentID:        cfg.WecomAgentID,
			Secret:         cfg.WecomSecret,
			Token:          cfg.WecomToken,
			EncodingAESKey: cfg.WecomEncodingAESKey,
			Port:           DefaultPort,
			CallbackPath:   DefaultPath,
		}
	}

	// Apply defaults
	if w.config.Port == 0 {
		w.config.Port = DefaultPort
	}
	if w.config.CallbackPath == "" {
		w.config.CallbackPath = DefaultPath
	}
}

// setupTmpDir 创建媒体文件临时目录
func (w *WechatcomChannel) setupTmpDir() {
	home, _ := os.UserHomeDir()
	w.tmpDir = filepath.Join(home, "cow", "tmp", "wechatcom")
	os.MkdirAll(w.tmpDir, 0755)
}

// Startup 启动企业微信渠道
func (w *WechatcomChannel) Startup(ctx context.Context) error {
	// Initialize configuration
	w.initConfig()

	// Validate configuration
	if w.config.CorpID == "" || w.config.Secret == "" {
		return fmt.Errorf("wechatcom: corp_id and secret are required")
	}

	// Setup temporary directory
	w.setupTmpDir()

	// Initialize crypto for message encryption/decryption
	crypto, err := NewWechatCrypto(w.config.Token, w.config.EncodingAESKey, w.config.CorpID)
	if err != nil {
		return fmt.Errorf("wechatcom: failed to initialize crypto: %w", err)
	}
	w.crypto = crypto

	// Initialize WeChat API client
	w.client = NewWechatClient(w.config.CorpID, w.config.Secret)

	// Start HTTP server for callback
	if err := w.startHTTPServer(ctx); err != nil {
		return fmt.Errorf("wechatcom: failed to start HTTP server: %w", err)
	}

	w.ReportStartupSuccess()
	logger.Info("[Wechatcom] Channel started successfully",
		zap.Int("port", w.config.Port),
		zap.Int("agent_id", w.config.AgentID))

	return nil
}

// Stop 停止企业微信渠道
func (w *WechatcomChannel) Stop() error {
	close(w.stopChan)

	if w.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.server.Shutdown(ctx); err != nil {
			logger.Warn("[Wechatcom] HTTP server shutdown error", zap.Error(err))
		}
	}

	w.SetStarted(false)
	return w.BaseChannel.Stop()
}

// Send 发送回复消息给用户
func (w *WechatcomChannel) Send(reply *types.Reply, ctx *types.Context) error {
	receiver, _ := ctx.GetString("receiver")
	if receiver == "" {
		return fmt.Errorf("no receiver in context")
	}

	switch reply.Type {
	case types.ReplyText, types.ReplyError, types.ReplyInfo:
		return w.sendText(reply.StringContent(), receiver)
	case types.ReplyVoice:
		return w.sendVoice(reply.StringContent(), receiver)
	case types.ReplyImage, types.ReplyImageURL:
		return w.sendImage(reply.StringContent(), receiver)
	default:
		logger.Warn("[Wechatcom] Unsupported reply type, fallback to text",
			zap.String("type", reply.Type.String()))
		return w.sendText(reply.StringContent(), receiver)
	}
}

// SetMessageHandler 设置消息处理回调
func (w *WechatcomChannel) SetMessageHandler(handler any) {
	if h, ok := handler.(MessageHandler); ok {
		w.msgHandler = h
	}
}

// sendText 发送文本消息，必要时分割
func (w *WechatcomChannel) sendText(text, receiver string) error {
	// Remove markdown symbols for WeChat Work
	cleanText := removeMarkdownSymbol(text)

	// Split long text
	texts := splitByUTF8Length(cleanText, MaxTextLen)
	if len(texts) > 1 {
		logger.Info("[Wechatcom] Text too long, split into parts",
			zap.Int("parts", len(texts)))
	}

	for i, t := range texts {
		if err := w.client.SendText(w.config.AgentID, receiver, t); err != nil {
			return fmt.Errorf("failed to send text part %d/%d: %w", i+1, len(texts), err)
		}
		if i < len(texts)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	logger.Debug("[Wechatcom] Sent text", zap.String("receiver", receiver))
	return nil
}

// sendVoice 发送语音消息
func (w *WechatcomChannel) sendVoice(filePath, receiver string) error {
	// 企业微信语音要求 AMR 格式，需要转换
	convertedPath, err := w.convertToAMR(filePath)
	if err != nil {
		return fmt.Errorf("failed to convert voice to AMR: %w", err)
	}
	if convertedPath != filePath {
		defer os.Remove(convertedPath)
	}

	mediaID, err := w.client.UploadMedia("voice", convertedPath)
	if err != nil {
		return fmt.Errorf("failed to upload voice: %w", err)
	}

	if err := w.client.SendVoice(w.config.AgentID, receiver, mediaID); err != nil {
		return fmt.Errorf("failed to send voice: %w", err)
	}

	logger.Debug("[Wechatcom] Sent voice", zap.String("receiver", receiver))
	return nil
}

// sendImage 发送图片消息
func (w *WechatcomChannel) sendImage(pathOrURL, receiver string) error {
	localPath := pathOrURL

	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		downloaded, err := w.downloadMedia(pathOrURL)
		if err != nil {
			return fmt.Errorf("failed to download image: %w", err)
		}
		localPath = downloaded
		defer os.Remove(localPath)
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("file not found: %s", localPath)
	}

	uploadPath := localPath
	if info.Size() >= 10*1024*1024 {
		compressed, err := w.compressImage(localPath)
		if err != nil {
			logger.Warn("[Wechatcom] Image compression failed, trying original",
				zap.Error(err))
		} else {
			uploadPath = compressed
			if compressed != localPath {
				defer os.Remove(compressed)
			}
		}
	}

	mediaID, err := w.client.UploadMedia("image", uploadPath)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	if err := w.client.SendImage(w.config.AgentID, receiver, mediaID); err != nil {
		return fmt.Errorf("failed to send image: %w", err)
	}

	logger.Debug("[Wechatcom] Sent image", zap.String("receiver", receiver))
	return nil
}

// downloadMedia 从 URL 下载文件到临时目录
func (w *WechatcomChannel) downloadMedia(urlStr string) (string, error) {
	resp, err := http.Get(urlStr)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	// Determine extension from content type
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
	}

	// Generate random filename
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

// convertToAMR 将音频文件转换为企业微信要求的 AMR 格式
func (w *WechatcomChannel) convertToAMR(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".amr" {
		return filePath, nil
	}

	amrPath := filePath + ".amr"

	cmd := exec.Command("ffmpeg", "-y", "-i", filePath, "-ar", "8000", "-ac", "1", "-ab", "12.2k", amrPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn("[Wechatcom] FFmpeg conversion failed, using original file",
			zap.Error(err),
			zap.String("output", string(output)))
		return filePath, nil
	}

	return amrPath, nil
}

// compressImage 压缩图片以符合企业微信大小限制
func (w *WechatcomChannel) compressImage(filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	const maxSize = 10 * 1024 * 1024
	if info.Size() <= maxSize {
		return filePath, nil
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	compressedPath := filePath + ".compressed" + ext

	quality := 85
	for {
		args := []string{"-y", "-i", filePath, "-q:v", fmt.Sprintf("%d", quality), compressedPath}
		cmd := exec.Command("ffmpeg", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Warn("[Wechatcom] Image compression attempt failed",
				zap.Int("quality", quality),
				zap.Error(err),
				zap.String("output", string(output)))
			quality -= 10
			if quality < 20 {
				return filePath, nil
			}
			continue
		}

		compressedInfo, err := os.Stat(compressedPath)
		if err != nil {
			return filePath, nil
		}

		if compressedInfo.Size() <= maxSize {
			logger.Info("[Wechatcom] Image compressed",
				zap.Int64("original", info.Size()),
				zap.Int64("compressed", compressedInfo.Size()),
				zap.Int("quality", quality))
			return compressedPath, nil
		}

		quality -= 10
		if quality < 20 {
			logger.Warn("[Wechatcom] Unable to compress image below limit",
				zap.Int64("size", compressedInfo.Size()))
			return compressedPath, nil
		}

		os.Remove(compressedPath)
	}
}

// startHTTPServer 启动用于微信回调的 HTTP 服务器
func (w *WechatcomChannel) startHTTPServer(ctx context.Context) error {
	var startErr error
	w.serverOnce.Do(func() {
		mux := http.NewServeMux()
		mux.HandleFunc(w.config.CallbackPath, w.handleCallback)

		addr := fmt.Sprintf("0.0.0.0:%d", w.config.Port)
		w.server = &http.Server{
			Addr:    addr,
			Handler: mux,
		}

		go func() {
			logger.Info("[Wechatcom] HTTP server started",
				zap.String("address", addr),
				zap.String("path", w.config.CallbackPath))

			if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("[Wechatcom] HTTP server error", zap.Error(err))
			}
		}()
	})
	return startErr
}

// handleCallback 处理微信回调请求
func (w *WechatcomChannel) handleCallback(resp http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		w.handleVerification(resp, req)
	case http.MethodPost:
		w.handleMessage(resp, req)
	default:
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVerification 处理微信服务器验证（GET 请求）
func (w *WechatcomChannel) handleVerification(resp http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	signature := query.Get("msg_signature")
	timestamp := query.Get("timestamp")
	nonce := query.Get("nonce")
	echostr := query.Get("echostr")

	logger.Debug("[Wechatcom] Verification request",
		zap.String("signature", signature),
		zap.String("timestamp", timestamp))

	echo, err := w.crypto.VerifyURL(signature, timestamp, nonce, echostr)
	if err != nil {
		logger.Warn("[Wechatcom] Verification failed", zap.Error(err))
		http.Error(resp, "Forbidden", http.StatusForbidden)
		return
	}

	resp.Write([]byte(echo))
}

// handleMessage 处理接收到的消息（POST 请求）
func (w *WechatcomChannel) handleMessage(resp http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	signature := query.Get("msg_signature")
	timestamp := query.Get("timestamp")
	nonce := query.Get("nonce")

	body, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Error("[Wechatcom] Failed to read request body", zap.Error(err))
		http.Error(resp, "Bad request", http.StatusBadRequest)
		return
	}

	// Decrypt message
	plaintext, err := w.crypto.DecryptMessage(string(body), signature, timestamp, nonce)
	if err != nil {
		logger.Warn("[Wechatcom] Failed to decrypt message", zap.Error(err))
		http.Error(resp, "Forbidden", http.StatusForbidden)
		return
	}

	// Parse message
	msg, err := ParseWechatMessage(plaintext)
	if err != nil {
		logger.Warn("[Wechatcom] Failed to parse message", zap.Error(err))
		resp.Write([]byte("success"))
		return
	}

	// Handle event messages
	if msg.MsgType == "event" {
		w.handleEvent(msg)
		resp.Write([]byte("success"))
		return
	}

	// Handle regular messages
	if w.msgHandler != nil {
		w.msgHandler(msg)
	}

	resp.Write([]byte("success"))
}

// handleEvent 处理微信事件
func (w *WechatcomChannel) handleEvent(msg *WechatMessage) {
	switch msg.Event {
	case "subscribe":
		logger.Info("[Wechatcom] User subscribed",
			zap.String("user", msg.FromUserID))
	case "unsubscribe":
		logger.Info("[Wechatcom] User unsubscribed",
			zap.String("user", msg.FromUserID))
	default:
		logger.Debug("[Wechatcom] Unhandled event",
			zap.String("event", msg.Event))
	}
}

// 辅助函数

// removeMarkdownSymbol 移除 markdown 格式符号
func removeMarkdownSymbol(text string) string {
	// Simple markdown removal - remove common markdown symbols
	replacer := strings.NewReplacer(
		"**", "",
		"__", "",
		"~~", "",
		"`", "",
	)
	return replacer.Replace(text)
}

// splitByUTF8Length 按 UTF-8 字节长度分割字符串
func splitByUTF8Length(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	var start int

	for start < len(text) {
		end := start + maxLen
		if end >= len(text) {
			chunks = append(chunks, text[start:])
			break
		}

		// 确保不会在 UTF-8 字符中间分割
		for end > start && !isUTF8Start(text[end]) {
			end--
		}

		if end == start {
			// 无法正确分割，直接使用 maxLen
			end = start + maxLen
		}

		chunks = append(chunks, text[start:end])
		start = end
	}

	return chunks
}

// isUTF8Start 检查字节是否是 UTF-8 字符的起始字节
func isUTF8Start(b byte) bool {
	return b&0xC0 != 0x80
}
