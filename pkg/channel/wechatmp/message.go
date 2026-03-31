// Package wechatmp 提供微信公众号渠道实现。
// message.go 定义微信公众号消息结构，用于解析和构建消息。
package wechatmp

import (
	"encoding/xml"
	"time"

	"github.com/bstr9/simpleclaw/pkg/types"
)

// WechatMessage 表示从微信公众号平台收到的消息。
// 包含微信回调消息中可能出现的所有字段。
type WechatMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`   // 开发者微信号
	FromUserName string   `xml:"FromUserName"` // 发送者 OpenID
	CreateTime   int64    `xml:"CreateTime"`   // 消息创建时间戳
	MsgType      string   `xml:"MsgType"`      // 消息类型
	Content      string   `xml:"Content"`      // 文本内容（文本消息）
	MsgID        int64    `xml:"MsgId"`        // 消息 ID（每条消息唯一）

	// 图片消息字段
	PicURL  string `xml:"PicUrl"`  // 图片链接（图片消息）
	MediaID string `xml:"MediaId"` // 媒体文件 ID，用于下载

	// 语音消息字段
	Format      string `xml:"Format"`      // 语音格式（如 amr）
	Recognition string `xml:"Recognition"` // 语音识别结果（如已开启）

	// 视频/小视频消息字段
	ThumbMediaID string `xml:"ThumbMediaId"` // 缩略图媒体 ID

	// 位置消息字段
	Location_X float64 `xml:"Location_X"` // 纬度
	Location_Y float64 `xml:"Location_Y"` // 经度
	Scale      int     `xml:"Scale"`      // 地图缩放级别
	Label      string  `xml:"Label"`      // 位置信息

	// 链接消息字段
	Title       string `xml:"Title"`       // 链接标题
	Description string `xml:"Description"` // 链接描述
	URL         string `xml:"Url"`         // 链接 URL

	// 事件消息字段
	Event     string  `xml:"Event"`     // 事件类型（关注、取消关注等）
	EventKey  string  `xml:"EventKey"`  // 事件 KEY 值
	Ticket    string  `xml:"Ticket"`    // 二维码 ticket（扫码事件）
	Latitude  float64 `xml:"Latitude"`  // 纬度（位置事件）
	Longitude float64 `xml:"Longitude"` // 经度（位置事件）
	Precision float64 `xml:"Precision"` // 位置精度
}

// IsText 检查消息是否为文本消息
func (m *WechatMessage) IsText() bool {
	return m.MsgType == "text"
}

// IsImage 检查消息是否为图片消息
func (m *WechatMessage) IsImage() bool {
	return m.MsgType == "image"
}

// IsVoice 检查消息是否为语音消息
func (m *WechatMessage) IsVoice() bool {
	return m.MsgType == "voice"
}

// IsVideo 检查消息是否为视频消息
func (m *WechatMessage) IsVideo() bool {
	return m.MsgType == "video" || m.MsgType == "shortvideo"
}

// IsLocation 检查消息是否为位置消息
func (m *WechatMessage) IsLocation() bool {
	return m.MsgType == "location"
}

// IsLink 检查消息是否为链接消息
func (m *WechatMessage) IsLink() bool {
	return m.MsgType == "link"
}

// IsEvent 检查消息是否为事件消息
func (m *WechatMessage) IsEvent() bool {
	return m.MsgType == "event"
}

// IsSubscribe 检查消息是否为关注事件
func (m *WechatMessage) IsSubscribe() bool {
	return m.IsEvent() && (m.Event == "subscribe" || m.Event == "subscribe_scan")
}

// IsUnsubscribe 检查消息是否为取消关注事件
func (m *WechatMessage) IsUnsubscribe() bool {
	return m.IsEvent() && m.Event == "unsubscribe"
}

// GetSender 返回发送者 OpenID
func (m *WechatMessage) GetSender() string {
	return m.FromUserName
}

// GetReceiver 返回接收者（开发者微信号）
func (m *WechatMessage) GetReceiver() string {
	return m.ToUserName
}

// GetMessageID 返回消息 ID 字符串
func (m *WechatMessage) GetMessageID() string {
	return int64ToString(m.MsgID)
}

// GetCreateTime 返回消息创建时间作为 time.Time
func (m *WechatMessage) GetCreateTime() time.Time {
	return time.Unix(m.CreateTime, 0)
}

// ToContextType 将微信消息类型转换为内部 ContextType
func (m *WechatMessage) ToContextType() types.ContextType {
	switch m.MsgType {
	case "text":
		return types.ContextText
	case "voice":
		if m.Recognition != "" {
			return types.ContextText // 语音识别结果作为文本处理
		}
		return types.ContextVoice
	case "image":
		return types.ContextImage
	case "video", "shortvideo":
		return types.ContextVideo
	case "location":
		return types.ContextSharing
	default:
		return types.ContextText
	}
}

// GetContent 根据消息类型返回消息内容
func (m *WechatMessage) GetContent() string {
	switch m.MsgType {
	case "text":
		return m.Content
	case "voice":
		if m.Recognition != "" {
			return m.Recognition
		}
		return m.MediaID
	case "image":
		return m.PicURL
	case "location":
		return m.Label
	case "link":
		return m.Title + "\n" + m.Description + "\n" + m.URL
	default:
		return m.Content
	}
}

// ReplyMessage 表示要发送回微信用户的回复消息。
type ReplyMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`   // 接收者 OpenID（CDATA 包裹）
	FromUserName string   `xml:"FromUserName"` // 开发者微信号（CDATA 包裹）
	CreateTime   int64    `xml:"CreateTime"`   // 回复时间戳
	MsgType      string   `xml:"MsgType"`      // 回复消息类型
	Content      string   `xml:"Content"`      // 文本内容（文本回复）

	// 图片回复字段
	Image struct {
		MediaID string `xml:"MediaId"`
	} `xml:"Image,omitempty"`

	// 语音回复字段
	Voice struct {
		MediaID string `xml:"MediaId"`
	} `xml:"Voice,omitempty"`

	// 视频回复字段
	Video struct {
		MediaID     string `xml:"MediaId"`
		Title       string `xml:"Title,omitempty"`
		Description string `xml:"Description,omitempty"`
	} `xml:"Video,omitempty"`

	// 音乐回复字段
	Music struct {
		Title        string `xml:"Title,omitempty"`
		Description  string `xml:"Description,omitempty"`
		MusicURL     string `xml:"MusicUrl,omitempty"`
		HQMusicURL   string `xml:"HQMusicUrl,omitempty"`
		ThumbMediaID string `xml:"ThumbMediaId"`
	} `xml:"Music,omitempty"`

	// 图文回复字段
	ArticleCount int `xml:"ArticleCount,omitempty"`
	Articles     struct {
		Item []NewsItem `xml:"item"`
	} `xml:"Articles,omitempty"`
}

// NewsItem 表示图文回复中的文章项
type NewsItem struct {
	Title       string `xml:"Title,omitempty"`
	Description string `xml:"Description,omitempty"`
	PicURL      string `xml:"PicUrl,omitempty"`
	URL         string `xml:"Url,omitempty"`
}

// NewTextReply 创建文本回复消息
func NewTextReply(toUser, fromUser, content string) *ReplyMessage {
	return &ReplyMessage{
		ToUserName:   toUser,
		FromUserName: fromUser,
		CreateTime:   time.Now().Unix(),
		MsgType:      "text",
		Content:      content,
	}
}

// NewImageReply 创建图片回复消息
func NewImageReply(toUser, fromUser, mediaID string) *ReplyMessage {
	reply := &ReplyMessage{
		ToUserName:   toUser,
		FromUserName: fromUser,
		CreateTime:   time.Now().Unix(),
		MsgType:      "image",
	}
	reply.Image.MediaID = mediaID
	return reply
}

// NewVoiceReply 创建语音回复消息
func NewVoiceReply(toUser, fromUser, mediaID string) *ReplyMessage {
	reply := &ReplyMessage{
		ToUserName:   toUser,
		FromUserName: fromUser,
		CreateTime:   time.Now().Unix(),
		MsgType:      "voice",
	}
	reply.Voice.MediaID = mediaID
	return reply
}

// NewVideoReply 创建视频回复消息
func NewVideoReply(toUser, fromUser, mediaID, title, description string) *ReplyMessage {
	reply := &ReplyMessage{
		ToUserName:   toUser,
		FromUserName: fromUser,
		CreateTime:   time.Now().Unix(),
		MsgType:      "video",
	}
	reply.Video.MediaID = mediaID
	reply.Video.Title = title
	reply.Video.Description = description
	return reply
}

// ToXML 将回复消息序列化为 XML
func (r *ReplyMessage) ToXML() ([]byte, error) {
	return xml.Marshal(r)
}

func int64ToString(n int64) string {
	if n == 0 {
		return ""
	}
	return string(rune(n))
}
