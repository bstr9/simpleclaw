// Package wecombot 提供企业微信机器人渠道实现。
// message.go 定义企业微信机器人消息类型和解析逻辑。
package wecombot

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/types"
)

// 消息类型常量
const (
	MsgTypeText  = "text"
	MsgTypeVoice = "voice"
	MsgTypeImage = "image"
	MsgTypeMixed = "mixed"
	MsgTypeFile  = "file"
	MsgTypeVideo = "video"
)

// 聊天类型常量
const (
	ChatTypeSingle = "single"
	ChatTypeGroup  = "group"
)

// MediaType 媒体类型
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeFile  MediaType = "file"
	MediaTypeVideo MediaType = "video"
)

// IncomingMessage 企业微信机器人接收到的消息结构
type IncomingMessage struct {
	// 基础字段
	MsgID      string `json:"msgid"`
	MsgType    string `json:"msgtype"`
	CreateTime int64  `json:"create_time"`
	ChatType   string `json:"chattype"`
	ChatID     string `json:"chatid"`
	AiBotID    string `json:"aibotid"`

	// 发送者信息
	From struct {
		UserID string `json:"userid"`
	} `json:"from"`

	// 消息内容（根据类型不同使用不同字段）
	Text  *TextContent  `json:"text,omitempty"`
	Voice *VoiceContent `json:"voice,omitempty"`
	Image *ImageContent `json:"image,omitempty"`
	Mixed *MixedContent `json:"mixed,omitempty"`
	File  *FileContent  `json:"file,omitempty"`
	Video *VideoContent `json:"video,omitempty"`
}

// TextContent 文本消息内容
type TextContent struct {
	Content string `json:"content"`
}

// VoiceContent 语音消息内容
type VoiceContent struct {
	Content string `json:"content"` // 语音识别后的文本
}

// ImageContent 图片消息内容
type ImageContent struct {
	URL    string `json:"url"`
	AESKey string `json:"aeskey"`
}

// MixedContent 混合消息内容
type MixedContent struct {
	MsgItems []MixedMsgItem `json:"msg_item"`
}

// MixedMsgItem 混合消息项
type MixedMsgItem struct {
	MsgType string     `json:"msgtype"`
	Text    *TextItem  `json:"text,omitempty"`
	Image   *ImageItem `json:"image,omitempty"`
}

// TextItem 文本项
type TextItem struct {
	Content string `json:"content"`
}

// ImageItem 图片项
type ImageItem struct {
	URL    string `json:"url"`
	AESKey string `json:"aeskey"`
}

// FileContent 文件消息内容
type FileContent struct {
	URL    string `json:"url"`
	AESKey string `json:"aeskey"`
}

// VideoContent 视频消息内容
type VideoContent struct {
	URL    string `json:"url"`
	AESKey string `json:"aeskey"`
}

// WecomBotMessage 企业微信机器人消息，实现 ChatMessage 接口
type WecomBotMessage struct {
	*types.BaseMessage
	incoming  *IncomingMessage
	isGroup   bool
	reqID     string
	tmpDir    string
	prepareFn func() error
	imagePath string
}

// NewWecomBotMessage 创建企业微信机器人消息实例
func NewWecomBotMessage(incoming *IncomingMessage, tmpDir string) (*WecomBotMessage, error) {
	if tmpDir == "" {
		tmpDir = "/tmp"
	}

	msg := &WecomBotMessage{
		incoming: incoming,
		isGroup:  incoming.ChatType == ChatTypeGroup,
		tmpDir:   tmpDir,
	}

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	content, ctxType, prepareFn := buildMessageContent(incoming, msg)
	msg.BaseMessage = &types.BaseMessage{
		MsgID:          incoming.MsgID,
		Content:        content,
		CreateTime:     time.Unix(incoming.CreateTime, 0),
		IsGroupMessage: msg.isGroup,
		MsgType:        int(ctxType),
		Context:        types.NewContext(ctxType, content),
	}
	msg.prepareFn = prepareFn
	msg.setParticipants(incoming)

	return msg, nil
}

// buildMessageContent 构建消息内容和准备函数
func buildMessageContent(incoming *IncomingMessage, msg *WecomBotMessage) (string, types.ContextType, func() error) {
	switch incoming.MsgType {
	case MsgTypeText:
		return buildTextContent(incoming, msg)
	case MsgTypeVoice:
		return buildVoiceContent(incoming)
	case MsgTypeImage:
		return buildImageContent(incoming, msg)
	case MsgTypeMixed:
		return msg.parseMixedMessage(), types.ContextText, nil
	case MsgTypeFile:
		return buildFileContent(incoming, msg)
	case MsgTypeVideo:
		return buildVideoContent(incoming, msg)
	default:
		return "", types.ContextText, nil
	}
}

// buildTextContent 构建文本消息内容
func buildTextContent(incoming *IncomingMessage, msg *WecomBotMessage) (string, types.ContextType, func() error) {
	content := incoming.Text.Content
	if msg.isGroup {
		content = removeMentions(content)
	}
	return content, types.ContextText, nil
}

// buildVoiceContent 构建语音消息内容
func buildVoiceContent(incoming *IncomingMessage) (string, types.ContextType, func() error) {
	return incoming.Voice.Content, types.ContextText, nil
}

// buildImageContent 构建图片消息内容
func buildImageContent(incoming *IncomingMessage, msg *WecomBotMessage) (string, types.ContextType, func() error) {
	return "[图片]", types.ContextImage, func() error {
		if incoming.Image == nil {
			return fmt.Errorf("图片内容为空")
		}
		imagePath := filepath.Join(msg.tmpDir, fmt.Sprintf("wecom_%s.png", msg.MsgID))
		data, err := decryptMedia(incoming.Image.URL, incoming.Image.AESKey)
		if err != nil {
			return fmt.Errorf("下载图片失败: %w", err)
		}
		if err := os.WriteFile(imagePath, data, 0644); err != nil {
			return fmt.Errorf("保存图片失败: %w", err)
		}
		msg.imagePath = imagePath
		msg.Content = imagePath
		return nil
	}
}

// buildFileContent 构建文件消息内容
func buildFileContent(incoming *IncomingMessage, msg *WecomBotMessage) (string, types.ContextType, func() error) {
	return "[文件]", types.ContextFile, func() error {
		if incoming.File == nil {
			return fmt.Errorf("文件内容为空")
		}
		data, err := decryptMedia(incoming.File.URL, incoming.File.AESKey)
		if err != nil {
			return fmt.Errorf("下载文件失败: %w", err)
		}
		ext := guessExtFromBytes(data)
		basePath := filepath.Join(msg.tmpDir, fmt.Sprintf("wecom_%s", msg.MsgID))
		filePath := basePath + ext
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return fmt.Errorf("保存文件失败: %w", err)
		}
		msg.Content = filePath
		return nil
	}
}

// buildVideoContent 构建视频消息内容
func buildVideoContent(incoming *IncomingMessage, msg *WecomBotMessage) (string, types.ContextType, func() error) {
	return "[视频]", types.ContextVideo, func() error {
		if incoming.Video == nil {
			return fmt.Errorf("视频内容为空")
		}
		videoPath := filepath.Join(msg.tmpDir, fmt.Sprintf("wecom_%s.mp4", msg.MsgID))
		data, err := decryptMedia(incoming.Video.URL, incoming.Video.AESKey)
		if err != nil {
			return fmt.Errorf("下载视频失败: %w", err)
		}
		if err := os.WriteFile(videoPath, data, 0644); err != nil {
			return fmt.Errorf("保存视频失败: %w", err)
		}
		msg.Content = videoPath
		return nil
	}
}

// setParticipants 设置消息的发送者和接收者信息
func (m *WecomBotMessage) setParticipants(incoming *IncomingMessage) {
	if m.isGroup {
		m.GroupID = incoming.ChatID
		m.FromUserID = incoming.From.UserID
		m.ToUserID = incoming.AiBotID
	} else {
		m.FromUserID = incoming.From.UserID
		m.ToUserID = incoming.AiBotID
	}
}

// Prepare 准备消息（下载媒体文件等）
func (m *WecomBotMessage) Prepare() error {
	if m.prepareFn != nil {
		return m.prepareFn()
	}
	return nil
}

// parseMixedMessage 解析混合消息
func (m *WecomBotMessage) parseMixedMessage() string {
	if m.incoming.Mixed == nil {
		return ""
	}

	var parts []string
	for _, item := range m.incoming.Mixed.MsgItems {
		part := m.parseMixedItem(item)
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, "\n")
}

// parseMixedItem 解析混合消息项
func (m *WecomBotMessage) parseMixedItem(item MixedMsgItem) string {
	switch item.MsgType {
	case MsgTypeText:
		return m.parseTextItem(item.Text)
	case MsgTypeImage:
		return m.parseImageItem(item.Image)
	default:
		return ""
	}
}

// parseTextItem 解析文本项
func (m *WecomBotMessage) parseTextItem(text *TextItem) string {
	if text == nil {
		return ""
	}
	content := text.Content
	if m.isGroup {
		content = removeMentions(content)
	}
	return content
}

// parseImageItem 解析图片项
func (m *WecomBotMessage) parseImageItem(image *ImageItem) string {
	if image == nil {
		return ""
	}
	imgPath := filepath.Join(m.tmpDir, fmt.Sprintf("wecom_%s_%d.png", m.MsgID, time.Now().UnixNano()))
	data, err := decryptMedia(image.URL, image.AESKey)
	if err != nil {
		return ""
	}
	os.WriteFile(imgPath, data, 0644)
	return fmt.Sprintf("[图片: %s]", imgPath)
}

// GetReqID 获取请求ID
func (m *WecomBotMessage) GetReqID() string {
	return m.reqID
}

// SetReqID 设置请求ID
func (m *WecomBotMessage) SetReqID(reqID string) {
	m.reqID = reqID
}

// GetImagePath 获取图片路径
func (m *WecomBotMessage) GetImagePath() string {
	return m.imagePath
}

// GetChatID 获取聊天ID
func (m *WecomBotMessage) GetChatID() string {
	return m.incoming.ChatID
}

// GetFromUserID 获取发送者ID
func (m *WecomBotMessage) GetFromUserID() string {
	return m.incoming.From.UserID
}

// OutgoingMessage 发送消息结构
type OutgoingMessage struct {
	Cmd     string          `json:"cmd"`
	Headers *MessageHeaders `json:"headers,omitempty"`
	Body    json.RawMessage `json:"body,omitempty"`
}

// MessageHeaders 消息头
type MessageHeaders struct {
	ReqID string `json:"req_id,omitempty"`
}

// TextMessageBody 文本消息体
type TextMessageBody struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

// StreamMessageBody 流式消息体
type StreamMessageBody struct {
	MsgType string `json:"msgtype"`
	Stream  struct {
		ID      string `json:"id"`
		Finish  bool   `json:"finish"`
		Content string `json:"content"`
	} `json:"stream"`
}

// ImageMessageBody 图片消息体
type ImageMessageBody struct {
	MsgType string `json:"msgtype"`
	Image   struct {
		MediaID string `json:"media_id"`
	} `json:"image"`
}

// FileMessageBody 文件消息体
type FileMessageBody struct {
	MsgType string `json:"msgtype"`
	File    struct {
		MediaID string `json:"media_id"`
	} `json:"file"`
}

// VideoMessageBody 视频消息体
type VideoMessageBody struct {
	MsgType string `json:"msgtype"`
	Video   struct {
		MediaID string `json:"media_id"`
	} `json:"video"`
}

// UploadInitBody 上传初始化请求体
type UploadInitBody struct {
	Type        string `json:"type"`
	Filename    string `json:"filename"`
	TotalSize   int64  `json:"total_size"`
	TotalChunks int    `json:"total_chunks"`
	MD5         string `json:"md5"`
}

// UploadChunkBody 上传分块请求体
type UploadChunkBody struct {
	UploadID   string `json:"upload_id"`
	ChunkIndex int    `json:"chunk_index"`
	Base64Data string `json:"base64_data"`
}

// UploadFinishBody 完成上传请求体
type UploadFinishBody struct {
	UploadID string `json:"upload_id"`
}

// UploadInitResponse 上传初始化响应
type UploadInitResponse struct {
	ErrCode int `json:"errcode"`
	Body    struct {
		UploadID string `json:"upload_id"`
	} `json:"body"`
}

// UploadFinishResponse 完成上传响应
type UploadFinishResponse struct {
	ErrCode int `json:"errcode"`
	Body    struct {
		MediaID string `json:"media_id"`
	} `json:"body"`
}

// WSMessage WebSocket 消息
type WSMessage struct {
	Cmd     string          `json:"cmd"`
	ErrCode int             `json:"errcode,omitempty"`
	ErrMsg  string          `json:"errmsg,omitempty"`
	Headers *MessageHeaders `json:"headers,omitempty"`
	Body    json.RawMessage `json:"body,omitempty"`
}

// EventCallbackBody 事件回调体
type EventCallbackBody struct {
	Event struct {
		EventType string `json:"eventtype"`
	} `json:"event"`
	From struct {
		UserID string `json:"userid"`
	} `json:"from"`
}

// decryptMedia 下载并解密企业微信媒体文件
func decryptMedia(url, aesKey string) ([]byte, error) {
	// 下载加密文件
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	encrypted, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解密
	decodedKey, err := base64.StdEncoding.DecodeString(aesKey)
	if err != nil {
		return nil, fmt.Errorf("解码密钥失败: %w", err)
	}

	if len(decodedKey) != 32 {
		return nil, fmt.Errorf("无效的 AES 密钥长度: %d", len(decodedKey))
	}

	block, err := aes.NewCipher(decodedKey)
	if err != nil {
		return nil, fmt.Errorf("创建密码块失败: %w", err)
	}

	iv := decodedKey[:16]
	mode := cipher.NewCBCDecrypter(block, iv)

	decrypted := make([]byte, len(encrypted))
	mode.CryptBlocks(decrypted, encrypted)

	// 移除 PKCS7 填充
	decrypted = pkcs7Unpad(decrypted)

	return decrypted, nil
}

// pkcs7Unpad 移除 PKCS7 填充
func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padding := int(data[len(data)-1])
	if padding > len(data) || padding > 32 {
		return data
	}
	return data[:len(data)-padding]
}

// removeMentions 移除消息中的 @ 提及
func removeMentions(content string) string {
	re := regexp.MustCompile(`@\S+\s*`)
	return strings.TrimSpace(re.ReplaceAllString(content, ""))
}

// 魔数签名
var magicSignatures = []struct {
	sig []byte
	ext string
}{
	{[]byte("%PDF"), ".pdf"},
	{[]byte("\x89PNG\r\n\x1a\n"), ".png"},
	{[]byte("\xff\xd8\xff"), ".jpg"},
	{[]byte("GIF87a"), ".gif"},
	{[]byte("GIF89a"), ".gif"},
	{[]byte("RIFF"), ".webp"},
	{[]byte("PK\x03\x04"), ".zip"},
	{[]byte("\x1f\x8b"), ".gz"},
	{[]byte("Rar!\x1a\x07"), ".rar"},
	{[]byte("7z\xbc\xaf\x27\x1c"), ".7z"},
}

// Office 文件标记
var officeZipMarkers = map[string]string{
	"word/": ".docx",
	"xl/":   ".xlsx",
	"ppt/":  ".pptx",
}

// guessExtFromBytes 根据文件内容魔数猜测扩展名
func guessExtFromBytes(data []byte) string {
	if len(data) < 8 {
		return ""
	}

	ext := matchMagicSignature(data)
	if ext != "" {
		return ext
	}

	return detectMP4Format(data)
}

// matchMagicSignature 匹配文件魔数签名
func matchMagicSignature(data []byte) string {
	for _, ms := range magicSignatures {
		if !bytes.HasPrefix(data, ms.sig) {
			continue
		}

		// 特殊处理 webp 格式
		if ms.ext == ".webp" && !isValidWebP(data) {
			continue
		}

		// 特殊处理 zip 格式（可能是 Office 文件）
		if ms.ext == ".zip" {
			if officeExt := detectOfficeFormat(data); officeExt != "" {
				return officeExt
			}
		}
		return ms.ext
	}
	return ""
}

// isValidWebP 验证是否为有效的 WebP 格式
func isValidWebP(data []byte) bool {
	return len(data) >= 12 && string(data[8:12]) == "WEBP"
}

// detectOfficeFormat 检测 Office 文件格式
func detectOfficeFormat(data []byte) string {
	searchLen := 2000
	if len(data) < searchLen {
		searchLen = len(data)
	}
	for marker, ext := range officeZipMarkers {
		if bytes.Contains(data[:searchLen], []byte(marker)) {
			return ext
		}
	}
	return ""
}

// detectMP4Format 检测 MP4 格式
func detectMP4Format(data []byte) string {
	if len(data) >= 12 && bytes.Contains(data[4:12], []byte("ftyp")) {
		return ".mp4"
	}
	return ""
}
