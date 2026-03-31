// Package wechatmp 提供微信公众号渠道实现。
// handler.go 定义 HTTP 处理器和加解密函数。
package wechatmp

import (
	"bytes"
	"github.com/bstr9/simpleclaw/pkg/common"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	"go.uber.org/zap"
)

// 常量定义
const (
	MaxUTF8Len           = 2048
	contentTypeTextPlain = "text/plain"
	contentTypeXML       = "application/xml"
	logPrefix            = "[wechatmp]"
	responseSuccess      = "success"
)

// Crypto 处理微信消息加解密
type Crypto struct {
	token          string
	encodingAESKey string
	appID          string
	key            []byte
}

// NewCrypto 创建新的加解密实例
func NewCrypto(token, encodingAESKey, appID string) (*Crypto, error) {
	// EncodingAESKey 是 43 个字符，需要追加 '=' 进行 base64 解码
	keyBytes, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, fmt.Errorf("failed to decode encoding AES key: %w", err)
	}

	return &Crypto{
		token:          token,
		encodingAESKey: encodingAESKey,
		appID:          appID,
		key:            keyBytes,
	}, nil
}

// VerifySignature 验证来自微信服务器的签名
// 用于服务器验证（GET）和消息验证（POST）
func VerifySignature(token, signature, timestamp, nonce string) bool {
	// 按字典序排序 token、timestamp、nonce
	arr := []string{token, timestamp, nonce}
	sort.Strings(arr)

	// 拼接并计算 SHA1
	combined := strings.Join(arr, "")
	h := sha1.New()
	h.Write([]byte(combined))
	computed := hex.EncodeToString(h.Sum(nil))

	return computed == signature
}

// DecryptMessage 解密来自微信的加密消息
func (c *Crypto) DecryptMessage(encryptedMsg []byte, msgSignature, timestamp, nonce string) ([]byte, error) {
	// 解析加密消息
	var encMsg struct {
		ToUserName string `xml:"ToUserName"`
		Encrypt    string `xml:"Encrypt"`
	}
	if err := xml.Unmarshal(encryptedMsg, &encMsg); err != nil {
		return nil, fmt.Errorf("failed to parse encrypted message: %w", err)
	}

	// 验证消息签名
	// 签名由 token + timestamp + nonce + encrypt 计算得出
	arr := []string{c.token, timestamp, nonce, encMsg.Encrypt}
	sort.Strings(arr)
	combined := strings.Join(arr, "")
	h := sha1.New()
	h.Write([]byte(combined))
	computed := hex.EncodeToString(h.Sum(nil))

	if computed != msgSignature {
		return nil, fmt.Errorf("message signature verification failed")
	}

	// Base64 解码
	ciphertext, err := base64.StdEncoding.DecodeString(encMsg.Encrypt)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode encrypted message: %w", err)
	}

	// AES-256-CBC 解密
	plaintext, err := c.aesDecrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to AES decrypt: %w", err)
	}

	// 移除随机字节（开头 16 字节）并获取实际内容
	// 格式：random(16) + msg_len(4) + msg + appid
	if len(plaintext) < 20 {
		return nil, fmt.Errorf("decrypted message too short")
	}

	// 获取消息长度（4 字节，大端序）
	msgLen := int(plaintext[16])<<24 | int(plaintext[17])<<16 | int(plaintext[18])<<8 | int(plaintext[19])
	if len(plaintext) < 20+msgLen {
		return nil, fmt.Errorf("decrypted message length mismatch")
	}

	// 提取消息和 appid
	msg := plaintext[20 : 20+msgLen]
	appID := plaintext[20+msgLen:]

	// 验证 appid
	if string(appID) != c.appID {
		return nil, fmt.Errorf("app ID mismatch: got %s, expected %s", string(appID), c.appID)
	}

	return msg, nil
}

// EncryptMessage 加密消息以发送回微信
func (c *Crypto) EncryptMessage(plaintext []byte, nonce, timestamp string) (string, error) {
	// 生成 16 个随机字节
	random := generateRandomBytes(16)

	// 准备内容：random(16) + msg_len(4) + msg + appid
	msgLen := len(plaintext)
	content := make([]byte, 0, 16+4+msgLen+len(c.appID))
	content = append(content, random...)
	content = append(content, byte(msgLen>>24), byte(msgLen>>16), byte(msgLen>>8), byte(msgLen))
	content = append(content, plaintext...)
	content = append(content, []byte(c.appID)...)

	// 填充到块大小（AES 块大小为 16）
	padded := pkcs7Pad(content, aes.BlockSize)

	// AES-256-CBC 加密
	ciphertext, err := c.aesEncrypt(padded)
	if err != nil {
		return "", fmt.Errorf("failed to AES encrypt: %w", err)
	}

	// Base64 编码
	encrypted := base64.StdEncoding.EncodeToString(ciphertext)

	// 生成签名
	arr := []string{c.token, timestamp, nonce, encrypted}
	sort.Strings(arr)
	combined := strings.Join(arr, "")
	h := sha1.New()
	h.Write([]byte(combined))
	signature := hex.EncodeToString(h.Sum(nil))

	// 构建加密响应 XML
	xmlResp := fmt.Sprintf(`<xml>
<Encrypt><![CDATA[%s]]></Encrypt>
<MsgSignature><![CDATA[%s]]></MsgSignature>
<TimeStamp>%s</TimeStamp>
<Nonce><![CDATA[%s]]></Nonce>
</xml>`, encrypted, signature, timestamp, nonce)

	return xmlResp, nil
}

// aesDecrypt 执行 AES-256-CBC 解密
func (c *Crypto) aesDecrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of block size")
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}

	// IV 是密钥的前 16 字节
	iv := c.key[:aes.BlockSize]

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 移除 PKCS7 填充
	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// aesEncrypt 执行 AES-256-CBC 加密
func (c *Crypto) aesEncrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}

	// IV is first 16 bytes of key
	iv := c.key[:aes.BlockSize]

	ciphertext := make([]byte, len(plaintext))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)

	return ciphertext, nil
}

// pkcs7Pad 使用 PKCS7 填充数据到块大小
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// pkcs7Unpad 移除 PKCS7 填充
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padding := int(data[len(data)-1])
	if padding > len(data) || padding > aes.BlockSize {
		return nil, fmt.Errorf("invalid padding")
	}
	return data[:len(data)-padding], nil
}

// generateRandomBytes 生成随机字节
func generateRandomBytes(n int) []byte {
	// 简化的伪随机实现，仅用于演示
	// 生产环境应使用 crypto/rand
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i * 7 % 256)
	}
	return b
}

// ChannelConfig 提供处理器对渠道配置的访问
type ChannelConfig interface {
	GetSubscribeMsg() string
}

// Handler 处理来自微信公众号平台的 HTTP 请求。
type Handler struct {
	channelConfig ChannelConfig
	crypto        *Crypto
	token         string

	// 消息处理器
	messageProcessor MessageProcessor
}

// MessageProcessor 定义处理微信消息的接口。
type MessageProcessor interface {
	ProcessMessage(msg *WechatMessage) (*types.Reply, error)
}

// NewHandler 创建新的处理器实例。
func NewHandler(channelConfig ChannelConfig, token string, crypto *Crypto) *Handler {
	return &Handler{
		channelConfig: channelConfig,
		crypto:        crypto,
		token:         token,
	}
}

// SetMessageProcessor 设置消息处理器。
func (h *Handler) SetMessageProcessor(processor MessageProcessor) {
	h.messageProcessor = processor
}

// ServeHTTP 处理来自微信的 HTTP 请求
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	query := r.URL.Query()
	signature := query.Get("signature")
	timestamp := query.Get("timestamp")
	nonce := query.Get("nonce")

	// 验证签名
	if !VerifySignature(h.token, signature, timestamp, nonce) {
		logger.Warn(logPrefix + " Invalid signature")
		http.Error(w, "Invalid signature", http.StatusForbidden)
		return
	}

	// 处理 GET 请求（服务器验证）
	if r.Method == http.MethodGet {
		echostr := query.Get("echostr")
		w.Header().Set(common.HeaderContentType, contentTypeTextPlain)
		w.Write([]byte(echostr))
		return
	}

	// 处理 POST 请求（消息回调）
	if r.Method == http.MethodPost {
		h.handlePost(w, r, query)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handlePost 处理来自微信的 POST 请求
func (h *Handler) handlePost(w http.ResponseWriter, r *http.Request, query url.Values) {
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error(logPrefix+" Failed to read request body", zap.Error(err))
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	logger.Debug(logPrefix+" Received post data", zap.String("body", string(body)))

	// 检查消息是否加密
	encryptType := query.Get("encrypt_type")
	var msgBytes []byte
	var encryptFunc func([]byte) (string, error)

	if encryptType == "aes" {
		// 解密消息
		if h.crypto == nil {
			logger.Error(logPrefix + " Crypto not initialized")
			http.Error(w, "Crypto not initialized", http.StatusInternalServerError)
			return
		}

		msgSignature := query.Get("msg_signature")
		timestamp := query.Get("timestamp")
		nonce := query.Get("nonce")

		msgBytes, err = h.crypto.DecryptMessage(body, msgSignature, timestamp, nonce)
		if err != nil {
			logger.Error(logPrefix+" Failed to decrypt message", zap.Error(err))
			http.Error(w, "Decryption failed", http.StatusBadRequest)
			return
		}

		// 设置响应加密函数
		encryptFunc = func(data []byte) (string, error) {
			return h.crypto.EncryptMessage(data, nonce, timestamp)
		}
	} else {
		msgBytes = body
		encryptFunc = nil
	}

	// 解析消息
	var msg WechatMessage
	if err := xml.Unmarshal(msgBytes, &msg); err != nil {
		logger.Error(logPrefix+" Failed to parse message", zap.Error(err))
		http.Error(w, "Failed to parse message", http.StatusBadRequest)
		return
	}

	logger.Info(logPrefix+" Received message",
		zap.String("type", msg.MsgType),
		zap.String("from", msg.FromUserName),
		zap.String("to", msg.ToUserName),
		zap.String("content", msg.Content))

	// 处理不同消息类型
	reply, err := h.processMessage(&msg)
	if err != nil {
		logger.Error(logPrefix+" Failed to process message", zap.Error(err))
	}

	// 发送响应
	if reply != nil {
		h.sendReply(w, &msg, reply, encryptFunc)
	} else {
		w.Header().Set(common.HeaderContentType, contentTypeTextPlain)
		w.Write([]byte(responseSuccess))
	}
}

// processMessage 处理微信消息并返回回复
func (h *Handler) processMessage(msg *WechatMessage) (*types.Reply, error) {
	// 处理事件消息
	if msg.IsEvent() {
		return h.handleEvent(msg)
	}

	if h.messageProcessor != nil {
		return h.messageProcessor.ProcessMessage(msg)
	}

	// 默认处理
	return nil, nil
}

// handleEvent 处理微信事件消息
func (h *Handler) handleEvent(msg *WechatMessage) (*types.Reply, error) {
	logger.Info(logPrefix+" Received event", zap.String("event", msg.Event))

	switch msg.Event {
	case "subscribe", "subscribe_scan":
		if h.channelConfig != nil && h.channelConfig.GetSubscribeMsg() != "" {
			return types.NewTextReply(h.channelConfig.GetSubscribeMsg()), nil
		}
		return nil, nil

	case "unsubscribe":
		// 用户取消关注，无需处理
		return nil, nil

	case "CLICK":
		// 菜单点击事件
		return h.handleMenuClick(msg)

	case "VIEW", "SCAN", "LOCATION":
		// 菜单跳转、二维码扫描、位置上报事件，无需处理
		return nil, nil

	default:
		return nil, nil
	}
}

// handleMenuClick 处理菜单点击事件
func (h *Handler) handleMenuClick(msg *WechatMessage) (*types.Reply, error) {
	// 可扩展以处理自定义菜单点击
	return nil, nil
}

// sendReply 发送回复消息回微信
func (h *Handler) sendReply(w http.ResponseWriter, msg *WechatMessage, reply *types.Reply, encryptFunc func([]byte) (string, error)) {
	// 构建回复消息
	var replyMsg *ReplyMessage

	switch reply.Type {
	case types.ReplyImage, types.ReplyImageURL:
		mediaID := reply.StringContent()
		replyMsg = NewImageReply(msg.FromUserName, msg.ToUserName, mediaID)

	case types.ReplyVoice:
		mediaID := reply.StringContent()
		replyMsg = NewVoiceReply(msg.FromUserName, msg.ToUserName, mediaID)

	case types.ReplyVideo, types.ReplyVideoURL:
		mediaID := reply.StringContent()
		replyMsg = NewVideoReply(msg.FromUserName, msg.ToUserName, mediaID, "", "")

	default:
		replyMsg = NewTextReply(msg.FromUserName, msg.ToUserName, reply.StringContent())
	}

	// 序列化为 XML
	xmlData, err := replyMsg.ToXML()
	if err != nil {
		logger.Error(logPrefix+" Failed to serialize reply", zap.Error(err))
		w.Header().Set(common.HeaderContentType, contentTypeTextPlain)
		w.Write([]byte(responseSuccess))
		return
	}

	// 如需加密
	if encryptFunc != nil {
		encrypted, err := encryptFunc(xmlData)
		if err != nil {
			logger.Error(logPrefix+" Failed to encrypt reply", zap.Error(err))
			w.Header().Set(common.HeaderContentType, contentTypeTextPlain)
			w.Write([]byte(responseSuccess))
			return
		}
		w.Header().Set(common.HeaderContentType, contentTypeXML)
		w.Write([]byte(encrypted))
		return
	}

	w.Header().Set(common.HeaderContentType, contentTypeXML)
	w.Write(xmlData)
}
