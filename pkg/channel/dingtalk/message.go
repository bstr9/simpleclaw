// Package dingtalk 提供钉钉渠道实现。
// message.go 定义钉钉消息类型和解析逻辑。
package dingtalk

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bstr9/simpleclaw/pkg/types"
)

// MessageType 定义钉钉消息类型常量
type MessageType string

const (
	// MsgTypeText 文本消息
	MsgTypeText MessageType = "text"
	// MsgTypePicture 图片消息
	MsgTypePicture MessageType = "picture"
	// MsgTypeAudio 音频消息
	MsgTypeAudio MessageType = "audio"
	// MsgTypeRichText 富文本消息
	MsgTypeRichText MessageType = "richText"
	// MsgTypeMarkdown Markdown 消息
	MsgTypeMarkdown MessageType = "markdown"
	// MsgTypeActionCard 动作卡片消息
	MsgTypeActionCard MessageType = "actionCard"
)

// ConversationType 定义会话类型常量
type ConversationType string

const (
	// ConversationTypeSingle 单聊 (1对1)
	ConversationTypeSingle ConversationType = "1"
	// ConversationTypeGroup 群聊
	ConversationTypeGroup ConversationType = "2"
)

// IncomingMessage 表示从钉钉 Stream 接收的消息
type IncomingMessage struct {
	// 基本消息字段
	MessageID         string           `json:"messageId"`
	ConversationID    string           `json:"conversationId"`
	ConversationType  ConversationType `json:"conversationType"`
	ConversationTitle string           `json:"conversationTitle"`
	SenderID          string           `json:"senderId"`
	SenderStaffID     string           `json:"senderStaffId"`
	SenderNick        string           `json:"senderNick"`
	ChatbotUserID     string           `json:"chatbotUserId"`
	MessageType       MessageType      `json:"msgtype"`
	CreateAt          int64            `json:"createAt"`
	RobotCode         string           `json:"robotCode"`

	// 基于消息类型的内容字段
	Text       *TextContent     `json:"text,omitempty"`
	Picture    *PictureContent  `json:"picture,omitempty"`
	Audio      *AudioContent    `json:"audio,omitempty"`
	RichText   *RichTextContent `json:"richText,omitempty"`
	Extensions map[string]any   `json:"extensions,omitempty"`
}

// TextContent 表示文本消息内容
type TextContent struct {
	Content string `json:"content"`
}

// PictureContent 表示图片消息内容
type PictureContent struct {
	DownloadCode string `json:"downloadCode"`
}

// AudioContent 表示音频消息内容
type AudioContent struct {
	DownloadCode string `json:"downloadCode"`
	Duration     int64  `json:"duration"`
}

// RichTextContent 表示富文本消息内容
type RichTextContent struct {
	Content string `json:"content"`
}

// DingTalkMessage 封装 IncomingMessage 并实现 ChatMessage 接口
type DingTalkMessage struct {
	*types.BaseMessage
	incoming       *IncomingMessage
	isGroup        bool
	senderStaffID  string
	conversationID string
	robotCode      string
	imagePath      string
}

// NewDingTalkMessage 从传入消息创建新的 DingTalkMessage
func NewDingTalkMessage(incoming *IncomingMessage) *DingTalkMessage {
	msg := &DingTalkMessage{
		incoming:       incoming,
		senderStaffID:  incoming.SenderStaffID,
		conversationID: incoming.ConversationID,
		robotCode:      incoming.RobotCode,
	}

	// 判断是否群聊
	msg.isGroup = incoming.ConversationType != ConversationTypeSingle

	// 根据消息类型解析内容
	var content string
	var ctxType types.ContextType

	switch incoming.MessageType {
	case MsgTypeText:
		content = incoming.Text.Content
		ctxType = types.ContextText
	case MsgTypeAudio:
		// 钉钉在 extensions 中提供语音识别结果
		if ext, ok := incoming.Extensions["content"].(map[string]any); ok {
			if recognition, ok := ext["recognition"].(string); ok {
				content = recognition
			}
		}
		ctxType = types.ContextText
	case MsgTypePicture:
		content = "[image]"
		ctxType = types.ContextImage
	case MsgTypeRichText:
		content = msg.parseRichText()
		ctxType = types.ContextText
	default:
		content = fmt.Sprintf("[%s]", incoming.MessageType)
		ctxType = types.ContextText
	}

	// 设置基本消息字段
	msg.BaseMessage = &types.BaseMessage{
		MsgID:          incoming.MessageID,
		Content:        content,
		CreateTime:     time.UnixMilli(incoming.CreateAt),
		IsGroupMessage: msg.isGroup,
		MsgType:        int(ctxType),
		Context:        types.NewContext(ctxType, content),
	}

	if msg.isGroup {
		msg.GroupID = incoming.ConversationID
		msg.FromUserID = incoming.ConversationID
	} else {
		msg.FromUserID = incoming.SenderID
	}
	msg.ToUserID = incoming.ChatbotUserID

	return msg
}

// parseRichText 从富文本消息中提取文本内容
func (m *DingTalkMessage) parseRichText() string {
	if m.incoming.RichText != nil {
		return m.incoming.RichText.Content
	}
	return ""
}

// GetRobotCode 返回机器人代码
func (m *DingTalkMessage) GetRobotCode() string {
	return m.robotCode
}

// GetSenderStaffID 返回发送者员工 ID
func (m *DingTalkMessage) GetSenderStaffID() string {
	return m.senderStaffID
}

// GetConversationID 返回会话 ID
func (m *DingTalkMessage) GetConversationID() string {
	return m.conversationID
}

// IsGroupMessage 返回是否为群消息
func (m *DingTalkMessage) IsGroupMessage() bool {
	return m.isGroup
}

// GetDownloadCode 返回媒体消息的下载码
func (m *DingTalkMessage) GetDownloadCode() string {
	switch m.incoming.MessageType {
	case MsgTypePicture:
		if m.incoming.Picture != nil {
			return m.incoming.Picture.DownloadCode
		}
	case MsgTypeAudio:
		if m.incoming.Audio != nil {
			return m.incoming.Audio.DownloadCode
		}
	}
	return ""
}

// SetImagePath 设置下载后的图片路径
func (m *DingTalkMessage) SetImagePath(path string) {
	m.imagePath = path
	m.Content = path
}

// GetImagePath 返回下载后的图片路径
func (m *DingTalkMessage) GetImagePath() string {
	return m.imagePath
}

// OutgoingMessage 表示通过钉钉 API 发送的消息
type OutgoingMessage struct {
	MsgKey   string          `json:"msgKey"`
	MsgParam json.RawMessage `json:"msgParam"`
}

// TextMessageParam 表示文本消息参数
type TextMessageParam struct {
	Content string `json:"content"`
}

// ImageMessageParam 表示图片消息参数
type ImageMessageParam struct {
	PhotoURL string `json:"photoURL,omitempty"`
	MediaID  string `json:"mediaId,omitempty"`
}

// MarkdownMessageParam 表示 Markdown 消息参数
type MarkdownMessageParam struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// ActionCardMessageParam 表示动作卡片消息参数
type ActionCardMessageParam struct {
	Title          string             `json:"title"`
	Text           string             `json:"text"`
	SingleTitle    string             `json:"singleTitle,omitempty"`
	SingleURL      string             `json:"singleURL,omitempty"`
	BtnOrientation string             `json:"btnOrientation,omitempty"`
	Btns           []ActionCardButton `json:"btns,omitempty"`
}

// ActionCardButton 表示动作卡片中的按钮
type ActionCardButton struct {
	Title     string `json:"title"`
	ActionURL string `json:"actionURL"`
}

// StreamCallback 表示来自钉钉 Stream 的回调消息
type StreamCallback struct {
	Headers map[string]string `json:"headers"`
	Data    json.RawMessage   `json:"data"`
}

// StreamAck 表示确认响应
type StreamAck struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}
