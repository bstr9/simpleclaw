// Package wechatcom 提供企业微信消息类型和解析。
// 该文件定义企业微信回调的消息结构和解析逻辑。
package wechatcom

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/bstr9/simpleclaw/pkg/types"
)

// WechatMessage 表示企业微信消息
type WechatMessage struct {
	XMLName     xml.Name `xml:"xml"`
	MsgType     string   `xml:"MsgType"`
	ToUserID    string   `xml:"ToUserName"`
	FromUserID  string   `xml:"FromUserName"`
	CreateTime  int64    `xml:"CreateTime"`
	MsgID       string   `xml:"MsgId"`
	AgentID     int      `xml:"AgentID"`
	Event       string   `xml:"Event"`
	ChangeType  string   `xml:"ChangeType"`
	Content     string   `xml:"Content"`
	MediaID     string   `xml:"MediaId"`
	Format      string   `xml:"Format"`
	PicURL      string   `xml:"PicUrl"`
	Recognition string   `xml:"Recognition"` // 语音识别结果

	// Context type determined by parser
	ctype types.ContextType

	// Temporary file path for media content
	tmpFilePath string

	// Prepare function for lazy media download
	prepareFn func() error
}

// GetMsgID 返回消息 ID
func (m *WechatMessage) GetMsgID() string {
	return m.MsgID
}

// GetFromUserID 返回发送者用户 ID
func (m *WechatMessage) GetFromUserID() string {
	return m.FromUserID
}

// GetToUserID 返回接收者用户 ID
func (m *WechatMessage) GetToUserID() string {
	return m.ToUserID
}

// GetContent 返回消息内容
func (m *WechatMessage) GetContent() string {
	return m.Content
}

// GetCreateTime 返回消息创建时间
func (m *WechatMessage) GetCreateTime() time.Time {
	return time.Unix(m.CreateTime, 0)
}

// IsGroup 返回 false（企业微信应用消息非群组）
func (m *WechatMessage) IsGroup() bool {
	return false
}

// GetGroupID 返回空字符串（企业微信应用不适用）
func (m *WechatMessage) GetGroupID() string {
	return ""
}

// GetMsgType 返回消息类型（整数）
func (m *WechatMessage) GetMsgType() int {
	return int(m.ctype)
}

// GetContext 返回消息上下文
func (m *WechatMessage) GetContext() *types.Context {
	return types.NewContext(m.ctype, m.Content)
}

// GetContextType 返回上下文类型
func (m *WechatMessage) GetContextType() types.ContextType {
	return m.ctype
}

// Prepare 准备消息（必要时下载媒体）
func (m *WechatMessage) Prepare() error {
	if m.prepareFn != nil {
		return m.prepareFn()
	}
	return nil
}

// GetTmpFilePath 返回媒体内容的临时文件路径
func (m *WechatMessage) GetTmpFilePath() string {
	return m.tmpFilePath
}

// ParseWechatMessage 将 XML 消息数据解析为 WechatMessage
func ParseWechatMessage(data string) (*WechatMessage, error) {
	var msg WechatMessage
	if err := xml.Unmarshal([]byte(data), &msg); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	// 根据消息类型确定上下文类型
	switch msg.MsgType {
	case "text":
		msg.ctype = types.ContextText
	case "voice":
		msg.ctype = types.ContextVoice
		msg.Content = msg.Recognition // 如有语音识别文本则使用
	case "image":
		msg.ctype = types.ContextImage
	case "video", "shortvideo":
		msg.ctype = types.ContextVideo
	case "file":
		msg.ctype = types.ContextFile
	case "event":
		// 事件在我们的系统中没有内容类型
		msg.ctype = types.ContextText
	default:
		return nil, fmt.Errorf("unsupported message type: %s", msg.MsgType)
	}

	return &msg, nil
}
