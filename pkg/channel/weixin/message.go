// Package weixin 提供微信个人号渠道实现
// message.go 实现消息解析和媒体处理
package weixin

import (
	"crypto/aes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	"go.uber.org/zap"
)

// WeixinMessage 表示解析后的微信消息
type WeixinMessage struct {
	types.BaseMessage

	// 微信特有字段
	contextToken string
	ctype        types.ContextType
	content      string
	imagePath    string
	prepareFunc  func()
}

// GetFromUserID 返回消息发送者 ID
func (m *WeixinMessage) GetFromUserID() string {
	return m.FromUserID
}

// GetContextToken 返回用于回复的上下文令牌
func (m *WeixinMessage) GetContextToken() string {
	return m.contextToken
}

// GetContextType 返回上下文类型
func (m *WeixinMessage) GetContextType() types.ContextType {
	return m.ctype
}

// Prepare 准备消息（如需要则下载媒体文件）
func (m *WeixinMessage) Prepare() {
	if m.prepareFunc != nil {
		m.prepareFunc()
	}
}

// parseWeixinMessage 解析来自 getUpdates API 的原始消息
func parseWeixinMessage(rawMsg map[string]any, cdnBaseURL, tmpDir string) *WeixinMessage {
	msg := &WeixinMessage{}

	msg.MsgID = getStringOr(rawMsg, "message_id", getStringOr(rawMsg, "seq", ""))
	msg.CreateTime = time.Unix(0, getInt64Or(rawMsg, "create_time_ms", 0)*1e6)
	msg.contextToken = getStringOr(rawMsg, "context_token", "")
	msg.IsGroupMessage = false

	fromUserID := getStringOr(rawMsg, "from_user_id", "")
	toUserID := getStringOr(rawMsg, "to_user_id", "")
	msg.FromUserID = fromUserID
	msg.ToUserID = toUserID
	msg.GroupID = ""

	itemList, _ := rawMsg["item_list"].([]any)
	textBody, mediaItem, mediaType, refText := parseItemList(itemList)

	msg.processContent(textBody, mediaItem, mediaType, refText, cdnBaseURL, tmpDir)
	msg.Context = types.NewContext(msg.ctype, msg.content)
	msg.MsgType = int(msg.ctype)

	return msg
}

// parseItemList 解析消息项列表
func parseItemList(itemList []any) (textBody string, mediaItem map[string]any, mediaType int, refText string) {
	for _, item := range itemList {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		itype := getIntOr(itemMap, "type", 0)

		switch itype {
		case itemText:
			textBody, refText = parseTextItem(itemMap, refText, &mediaItem, &mediaType)
		case itemVoice:
			textBody, mediaItem, mediaType = parseVoiceItem(itemMap, textBody, mediaItem, mediaType)
		case itemImage, itemVideo, itemFile:
			if mediaItem == nil {
				mediaItem = itemMap
				mediaType = itype
			}
		}
	}
	return textBody, mediaItem, mediaType, refText
}

// parseTextItem 解析文本消息项
func parseTextItem(itemMap map[string]any, refText string, mediaItem *map[string]any, mediaType *int) (string, string) {
	textItem, _ := itemMap["text_item"].(map[string]any)
	textBody := getStringOr(textItem, "text", "")

	refMsg, _ := itemMap["ref_msg"].(map[string]any)
	if refMsg != nil {
		refText = buildRefText(refMsg, mediaItem, mediaType)
	}
	return textBody, refText
}

// buildRefText 构建引用文本
func buildRefText(refMsg map[string]any, mediaItem *map[string]any, mediaType *int) string {
	refTitle := getStringOr(refMsg, "title", "")
	refMI, _ := refMsg["message_item"].(map[string]any)
	var refBody string
	if getIntOr(refMI, "type", 0) == itemText {
		refTextItem, _ := refMI["text_item"].(map[string]any)
		refBody = getStringOr(refTextItem, "text", "")
	}

	refType := getIntOr(refMI, "type", 0)
	if refType == itemImage || refType == itemVideo || refType == itemFile {
		if *mediaItem == nil {
			*mediaItem = refMI
			*mediaType = refType
		}
	}

	if refTitle == "" && refBody == "" {
		return ""
	}
	parts := []string{}
	if refTitle != "" {
		parts = append(parts, refTitle)
	}
	if refBody != "" {
		parts = append(parts, refBody)
	}
	return fmt.Sprintf("[引用: %s]\n", joinParts(parts, " | "))
}

// parseVoiceItem 解析语音消息项
func parseVoiceItem(itemMap map[string]any, textBody string, mediaItem map[string]any, mediaType int) (string, map[string]any, int) {
	voiceItem, _ := itemMap["voice_item"].(map[string]any)
	voiceText := getStringOr(voiceItem, "text", "")
	if voiceText != "" {
		return voiceText, mediaItem, mediaType
	}
	if mediaItem == nil {
		return textBody, itemMap, itemVoice
	}
	return textBody, mediaItem, mediaType
}

// processContent 处理消息内容
func (m *WeixinMessage) processContent(textBody string, mediaItem map[string]any, mediaType int, refText, cdnBaseURL, tmpDir string) {
	if mediaItem != nil && textBody == "" {
		m.setupMedia(mediaItem, mediaType, cdnBaseURL, tmpDir)
		return
	}

	m.ctype = types.ContextText
	if mediaItem != nil {
		m.appendMediaContent(mediaItem, mediaType, cdnBaseURL, tmpDir, textBody)
		textBody = m.content
	}
	m.content = refText + textBody
	m.Content = m.content
}

// appendMediaContent 将媒体内容附加到文本
func (m *WeixinMessage) appendMediaContent(mediaItem map[string]any, mediaType int, cdnBaseURL, tmpDir, textBody string) {
	mediaPath := downloadMedia(mediaItem, mediaType, cdnBaseURL, tmpDir, m.MsgID)
	if mediaPath == "" {
		return
	}
	var mediaLabel string
	switch mediaType {
	case itemImage:
		mediaLabel = "图片"
	case itemVideo:
		mediaLabel = "视频"
	default:
		mediaLabel = "文件"
	}
	m.content = textBody + fmt.Sprintf("\n[%s: %s]", mediaLabel, mediaPath)
}

// setupMedia 将消息设置为媒体类型并延迟下载
func (m *WeixinMessage) setupMedia(item map[string]any, mediaType int, cdnBaseURL, tmpDir string) {
	switch mediaType {
	case itemImage:
		m.ctype = types.ContextImage
		imagePath := downloadMedia(item, itemImage, cdnBaseURL, tmpDir, m.MsgID)
		if imagePath != "" {
			m.content = imagePath
			m.Content = imagePath
			m.imagePath = imagePath
		} else {
			m.ctype = types.ContextText
			m.content = "[Image download failed]"
			m.Content = m.content
		}

	case itemVideo:
		m.ctype = types.ContextFile
		savePath := filepath.Join(tmpDir, fmt.Sprintf("wx_%s.mp4", m.MsgID))
		m.content = savePath
		m.Content = savePath

		m.prepareFunc = func() {
			path := downloadMedia(item, itemVideo, cdnBaseURL, tmpDir, m.MsgID)
			if path != "" {
				m.content = path
				m.Content = path
			}
		}

	case itemFile:
		m.ctype = types.ContextFile
		fileItem, _ := item["file_item"].(map[string]any)
		fileName := getStringOr(fileItem, "file_name", fmt.Sprintf("wx_%s", m.MsgID))
		savePath := filepath.Join(tmpDir, fileName)
		m.content = savePath
		m.Content = savePath

		m.prepareFunc = func() {
			path := downloadMedia(item, itemFile, cdnBaseURL, tmpDir, m.MsgID)
			if path != "" {
				m.content = path
				m.Content = path
			}
		}

	case itemVoice:
		m.ctype = types.ContextVoice
		savePath := filepath.Join(tmpDir, fmt.Sprintf("wx_%s.silk", m.MsgID))
		m.content = savePath
		m.Content = savePath

		m.prepareFunc = func() {
			path := downloadMedia(item, itemVoice, cdnBaseURL, tmpDir, m.MsgID)
			if path != "" {
				m.content = path
				m.Content = path
			}
		}
	}
}

// downloadMedia 从 CDN 下载媒体文件
func downloadMedia(item map[string]any, mediaType int, cdnBaseURL, tmpDir, msgID string) string {
	typeKeyMap := map[int]string{
		itemImage: "image_item",
		itemVideo: "video_item",
		itemFile:  "file_item",
		itemVoice: "voice_item",
	}

	key := typeKeyMap[mediaType]
	info, _ := item[key].(map[string]any)
	media, _ := info["media"].(map[string]any)

	encryptParam := getStringOr(media, "encrypt_query_param", "")
	aesKey := getStringOr(info, "aeskey", "")
	if aesKey == "" {
		aesKey = getStringOr(media, "aes_key", "")
	}

	if encryptParam == "" || aesKey == "" {
		return ""
	}

	// 确定保存路径
	var savePath string
	if mediaType == itemFile {
		fileName := getStringOr(info, "file_name", "")
		if fileName != "" {
			savePath = filepath.Join(tmpDir, fileName)
		} else {
			savePath = filepath.Join(tmpDir, fmt.Sprintf("wx_%s.bin", msgID))
		}
	} else {
		extMap := map[int]string{
			itemImage: ".jpg",
			itemVideo: ".mp4",
			itemVoice: ".silk",
		}
		ext := extMap[mediaType]
		savePath = filepath.Join(tmpDir, fmt.Sprintf("wx_%s%s", msgID, ext))
	}

	// 从 CDN 下载
	data, err := downloadFromCDN(cdnBaseURL, encryptParam, aesKey)
	if err != nil {
		logger.Debug("CDN download failed", zap.Error(err))
		return ""
	}

	// 确保目录存在
	os.MkdirAll(filepath.Dir(savePath), 0755)

	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return ""
	}

	return savePath
}

func downloadFromCDN(cdnBaseURL, encryptParam, aesKeyHex string) ([]byte, error) {
	// 构建下载 URL
	downloadURL := fmt.Sprintf("%s/download?encrypted_query_param=%s",
		cdnBaseURL, url.QueryEscape(encryptParam))

	// 发起 HTTP 请求
	resp, err := http.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("CDN download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CDN download failed: status %d", resp.StatusCode)
	}

	// 读取加密数据
	encryptedData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read CDN response: %w", err)
	}

	// 解析 AES 密钥
	aesKey, err := parseAESKey(aesKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AES key: %w", err)
	}

	// 解密数据
	decryptedData, err := aesECBDecrypt(encryptedData, aesKey)
	if err != nil {
		return nil, fmt.Errorf("AES decryption failed: %w", err)
	}

	return decryptedData, nil
}

// AES 和 MD5 辅助函数

func md5Hash(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func aesECBPaddedSize(plaintextSize int) int {
	// PKCS7 填充：下一个 16 的倍数
	return ((plaintextSize + 1 + 15) / 16) * 16
}

func aesECBEncrypt(plaintext, key []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}

	// PKCS7 填充
	padLen := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	// ECB 加密
	ciphertext := make([]byte, len(padded))
	for i := 0; i < len(padded); i += aes.BlockSize {
		block.Encrypt(ciphertext[i:i+aes.BlockSize], padded[i:i+aes.BlockSize])
	}

	return ciphertext
}

func aesECBDecrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of block size")
	}

	// ECB 解密
	plaintext := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += aes.BlockSize {
		block.Decrypt(plaintext[i:i+aes.BlockSize], ciphertext[i:i+aes.BlockSize])
	}

	// 移除 PKCS7 填充
	padLen := int(plaintext[len(plaintext)-1])
	if padLen > aes.BlockSize || padLen > len(plaintext) {
		return plaintext, nil // 无有效填充
	}

	// 验证填充
	for i := len(plaintext) - padLen; i < len(plaintext); i++ {
		if plaintext[i] != byte(padLen) {
			return plaintext, nil // 无效填充
		}
	}

	return plaintext[:len(plaintext)-padLen], nil
}

// parseAESKey 从十六进制或 base64 格式解析 AES 密钥
func parseAESKey(aesKey string) ([]byte, error) {
	// 首先尝试十六进制
	if len(aesKey) == 32 {
		key, err := hex.DecodeString(aesKey)
		if err == nil && len(key) == 16 {
			return key, nil
		}
	}

	// 尝试 base64
	decoded, err := base64.StdEncoding.DecodeString(aesKey)
	if err != nil {
		return nil, fmt.Errorf("invalid AES key format")
	}

	// 如果是 32 字节，可能是十六进制编码的
	if len(decoded) == 32 {
		key, err := hex.DecodeString(string(decoded))
		if err == nil && len(key) == 16 {
			return key, nil
		}
	}

	if len(decoded) == 16 {
		return decoded, nil
	}

	return nil, fmt.Errorf("invalid AES key length: %d", len(decoded))
}

// map 提取辅助函数

func getStringOr(m map[string]any, key, def string) string {
	if m == nil {
		return def
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return def
}

func getIntOr(m map[string]any, key string, def int) int {
	if m == nil {
		return def
	}
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return def
}

func getInt64Or(m map[string]any, key string, def int64) int64 {
	if m == nil {
		return def
	}
	switch v := m[key].(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	}
	return def
}

func joinParts(parts []string, sep string) string {
	var sb strings.Builder
	for i, p := range parts {
		if i > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(p)
	}
	return sb.String()
}
