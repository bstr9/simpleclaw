// feishu 包提供飞书渠道实现。
// message.go 定义飞书消息结构和解析逻辑。
package feishu

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/types"
)

// MessageType 定义飞书消息类型
type MessageType string

const (
	MsgTypeText  MessageType = "text"
	MsgTypeImage MessageType = "image"
	MsgTypePost  MessageType = "post"
	MsgTypeFile  MessageType = "file"
	MsgTypeAudio MessageType = "audio"
	MsgTypeVideo MessageType = "video"
)

// ChatType 定义飞书会话类型
type ChatType string

const (
	ChatTypeP2P   ChatType = "p2p"   // 私聊
	ChatTypeGroup ChatType = "group" // 群聊
)

// FeishuEvent 表示飞书 Webhook 事件结构
type FeishuEvent struct {
	AppID     string       `json:"app_id"`
	Type      string       `json:"type"`
	Header    *EventHeader `json:"header,omitempty"`
	Event     *EventBody   `json:"event,omitempty"`
	Schema    string       `json:"schema"`
	Challenge string       `json:"challenge,omitempty"` // URL 验证
}

// EventHeader 表示事件头
type EventHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	CreateTime string `json:"create_time"`
	Token      string `json:"token"`
	AppID      string `json:"app_id"`
	TenantKey  string `json:"tenant_key"`
}

// EventBody 表示事件体
type EventBody struct {
	Sender  *Sender  `json:"sender"`
	Message *Message `json:"message"`
}

// Sender 表示消息发送者信息
type Sender struct {
	SenderID  *SenderID `json:"sender_id"`
	Type      string    `json:"type"`
	TenantKey string    `json:"tenant_key"`
}

// SenderID 表示发送者 ID 信息
type SenderID struct {
	OpenID  string `json:"open_id"`
	UserID  string `json:"user_id"`
	UnionID string `json:"union_id"`
}

// Message 表示飞书消息结构
type Message struct {
	MessageID   string     `json:"message_id"`
	RootID      string     `json:"root_id"`
	ParentID    string     `json:"parent_id"`
	CreateTime  string     `json:"create_time"`
	ChatID      string     `json:"chat_id"`
	ChatType    string     `json:"chat_type"`
	MessageType string     `json:"message_type"`
	Content     string     `json:"content"`
	Mentions    []*Mention `json:"mentions,omitempty"`
}

// Mention 表示消息中的 @ 提及
type Mention struct {
	Key       string     `json:"key"`
	ID        *MentionID `json:"id"`
	Name      string     `json:"name"`
	TenantKey string     `json:"tenant_key"`
}

// MentionID 表示提及 ID
type MentionID struct {
	OpenID  string `json:"open_id"`
	UserID  string `json:"user_id"`
	UnionID string `json:"union_id"`
}

// TextContent 表示文本消息内容
type TextContent struct {
	Text string `json:"text"`
}

// ImageContent 表示图片消息内容
type ImageContent struct {
	ImageKey string `json:"image_key"`
}

// PostContent 表示富文本消息内容
type PostContent struct {
	Title   string           `json:"title"`
	Content [][]*PostElement `json:"content"`
}

// PostElement 表示富文本中的元素
type PostElement struct {
	Tag      string `json:"tag"`
	Text     string `json:"text,omitempty"`
	ImageKey string `json:"image_key,omitempty"`
	Href     string `json:"href,omitempty"`
}

// FileContent 表示文件消息内容
type FileContent struct {
	FileKey  string `json:"file_key"`
	FileName string `json:"file_name"`
}

// FeishuMessage 表示解析后的飞书消息
// 实现 types.ChatMessage 接口
type FeishuMessage struct {
	types.BaseMessage

	// 飞书特有字段
	MsgID       string
	CreateTime  time.Time
	IsGroupChat bool
	ChatID      string
	OpenID      string
	AppID       string
	ChatType    ChatType
	MsgType     MessageType
	RawContent  string

	// WebSocket 模式使用的字段
	ContentText  string
	SenderID     string
	SenderOpenID string

	// 解析后的内容字段
	TextContent string
	ImagePaths  []string
	FilePath    string
	FileName    string

	// 用于下载资源的访问令牌
	AccessToken string

	// @ 提及列表
	Mentions []*Mention

	// 消息处理的上下文类型
	CtxType types.ContextType

	// 回复用的接收者 ID 类型
	ReceiveIDType string

	// 机器人 OpenID 用于检查提及
	BotOpenID string

	// extraContext 存储额外的上下文数据（如 channel_type, receiver 等）
	extraContext map[string]any
}

// NewFeishuMessage 从事件创建飞书消息
func NewFeishuMessage(event *FeishuEvent, accessToken string) (*FeishuMessage, error) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil, fmt.Errorf("invalid event: missing message data")
	}

	msg := event.Event.Message
	sender := event.Event.Sender

	// 解析创建时间
	var createTime time.Time
	if msg.CreateTime != "" {
		if ts, err := parseFeishuTimestamp(msg.CreateTime); err == nil {
			createTime = ts
		}
	}

	// 获取发送者 OpenID
	var openID string
	if sender != nil && sender.SenderID != nil {
		openID = sender.SenderID.OpenID
	}

	// 确定会话类型
	chatType := ChatTypeP2P
	isGroup := false
	if msg.ChatType == "group" {
		chatType = ChatTypeGroup
		isGroup = true
	}

	fm := &FeishuMessage{
		MsgID:         msg.MessageID,
		CreateTime:    createTime,
		IsGroupChat:   isGroup,
		ChatID:        msg.ChatID,
		OpenID:        openID,
		AppID:         event.AppID,
		ChatType:      chatType,
		MsgType:       MessageType(msg.MessageType),
		RawContent:    msg.Content,
		AccessToken:   accessToken,
		Mentions:      msg.Mentions,
		ReceiveIDType: "open_id",
	}

	// 设置 BaseMessage 字段
	fm.MsgID = msg.MessageID
	fm.FromUserID = openID
	fm.ToUserID = event.AppID
	fm.IsGroupMessage = isGroup
	fm.GroupID = msg.ChatID

	// 解析消息内容
	if err := fm.parseContent(); err != nil {
		return nil, fmt.Errorf("failed to parse message content: %w", err)
	}

	return fm, nil
}

// parseContent 根据消息类型解析消息内容
func (fm *FeishuMessage) parseContent() error {
	switch fm.MsgType {
	case MsgTypeText:
		return fm.parseTextContent()
	case MsgTypeImage:
		return fm.parseImageContent()
	case MsgTypePost:
		return fm.parsePostContent()
	case MsgTypeFile:
		return fm.parseFileContent()
	default:
		return fmt.Errorf("unsupported message type: %s", fm.MsgType)
	}
}

// parseTextContent 解析文本消息内容
func (fm *FeishuMessage) parseTextContent() error {
	var content TextContent
	if err := json.Unmarshal([]byte(fm.RawContent), &content); err != nil {
		return fmt.Errorf("failed to parse text content: %w", err)
	}

	fm.TextContent = content.Text
	fm.CtxType = types.ContextText
	fm.Content = content.Text

	// 移除群消息中的 @ 提及占位符
	if fm.IsGroupChat {
		fm.TextContent = removeMentionPlaceholder(fm.TextContent)
		fm.Content = fm.TextContent
	}

	return nil
}

// parseImageContent 解析图片消息内容
func (fm *FeishuMessage) parseImageContent() error {
	var content ImageContent
	if err := json.Unmarshal([]byte(fm.RawContent), &content); err != nil {
		return fmt.Errorf("failed to parse image content: %w", err)
	}

	fm.CtxType = types.ContextImage
	// 图片 key 将在后续用于下载图片
	fm.Content = fmt.Sprintf("[image:%s]", content.ImageKey)
	fm.ImagePaths = []string{content.ImageKey}

	return nil
}

// parsePostContent 解析富文本消息内容
func (fm *FeishuMessage) parsePostContent() error {
	var content PostContent
	if err := json.Unmarshal([]byte(fm.RawContent), &content); err != nil {
		return fmt.Errorf("failed to parse post content: %w", err)
	}

	fm.CtxType = types.ContextText

	textParts, imageKeys := extractPostContent(content)
	fm.TextContent = joinTextParts(textParts)
	fm.Content = fm.TextContent
	fm.ImagePaths = imageKeys

	// 将图片引用追加到内容中
	for _, key := range imageKeys {
		fm.Content += fmt.Sprintf("\n[image:%s]", key)
	}

	return nil
}

// extractPostContent 从富文本内容中提取文本和图片
func extractPostContent(content PostContent) (textParts []string, imageKeys []string) {
	if content.Title != "" {
		textParts = append(textParts, content.Title)
	}

	for _, block := range content.Content {
		textParts, imageKeys = extractBlockElements(block, textParts, imageKeys)
	}

	return textParts, imageKeys
}

// extractBlockElements 从内容块中提取元素
func extractBlockElements(block []*PostElement, textParts, imageKeys []string) ([]string, []string) {
	for _, element := range block {
		switch element.Tag {
		case "text":
			if element.Text != "" {
				textParts = append(textParts, element.Text)
			}
		case "img":
			if element.ImageKey != "" {
				imageKeys = append(imageKeys, element.ImageKey)
			}
		}
	}
	return textParts, imageKeys
}

// parseFileContent 解析文件消息内容
func (fm *FeishuMessage) parseFileContent() error {
	var content FileContent
	if err := json.Unmarshal([]byte(fm.RawContent), &content); err != nil {
		return fmt.Errorf("failed to parse file content: %w", err)
	}

	fm.CtxType = types.ContextFile
	fm.FilePath = content.FileKey
	fm.FileName = content.FileName
	fm.Content = fmt.Sprintf("[file:%s]", content.FileName)

	return nil
}

// GetMsgID 返回消息 ID
func (fm *FeishuMessage) GetMsgID() string {
	return fm.MsgID
}

// GetFromUserID 返回发送者 OpenID
func (fm *FeishuMessage) GetFromUserID() string {
	return fm.OpenID
}

// GetToUserID 返回应用 ID（机器人）
func (fm *FeishuMessage) GetToUserID() string {
	return fm.AppID
}

// GetContent 返回消息内容
func (fm *FeishuMessage) GetContent() string {
	return fm.Content
}

// GetCreateTime 返回消息创建时间
func (fm *FeishuMessage) GetCreateTime() time.Time {
	return fm.CreateTime
}

// IsGroup 返回是否为群消息
func (fm *FeishuMessage) IsGroup() bool {
	return fm.IsGroupChat
}

// GetGroupID 返回群聊 ID
func (fm *FeishuMessage) GetGroupID() string {
	return fm.ChatID
}

// GetMsgType 返回消息类型
func (fm *FeishuMessage) GetMsgType() int {
	return int(fm.CtxType)
}

// GetContext 返回消息上下文
func (fm *FeishuMessage) GetContext() *types.Context {
	ctx := types.NewContext(fm.CtxType, fm.Content)
	for k, v := range fm.extraContext {
		ctx.Set(k, v)
	}
	return ctx
}

// SetExtraContext 设置额外的上下文数据
func (fm *FeishuMessage) SetExtraContext(key string, value any) {
	if fm.extraContext == nil {
		fm.extraContext = make(map[string]any)
	}
	fm.extraContext[key] = value
}

// IsMentionBot 检查消息中是否提及了机器人
func (fm *FeishuMessage) IsMentionBot(botOpenID string) bool {
	if botOpenID == "" {
		// 如果没有机器人 OpenID，假设第一个提及是机器人（飞书只投递被提及的消息）
		return len(fm.Mentions) > 0
	}

	for _, m := range fm.Mentions {
		if m.ID != nil && m.ID.OpenID == botOpenID {
			return true
		}
	}
	return false
}

// GetSessionID 返回会话跟踪用的会话 ID
func (fm *FeishuMessage) GetSessionID(groupSharedSession bool) string {
	if fm.IsGroupChat {
		if groupSharedSession {
			return fm.ChatID // 整个群共享会话
		}
		return fmt.Sprintf("%s:%s", fm.OpenID, fm.ChatID)
	}
	return fm.OpenID
}

// 辅助函数

// parseFeishuTimestamp 解析飞书时间戳字符串为 time.Time
func parseFeishuTimestamp(ts string) (time.Time, error) {
	// 飞书使用毫秒级 Unix 时间戳字符串
	var ms int64
	if _, err := fmt.Sscanf(ts, "%d", &ms); err != nil {
		return time.Time{}, err
	}
	return time.Unix(ms/1000, (ms%1000)*1e6), nil
}

// removeMentionPlaceholder 移除文本中的 @_user_1 占位符
func removeMentionPlaceholder(text string) string {
	placeholders := []string{"@_user_1", "@_user_2", "@_user_3"}
	for _, p := range placeholders {
		text = strings.ReplaceAll(text, p, "")
	}
	return text
}

// joinTextParts 用换行符连接文本片段
func joinTextParts(parts []string) string {
	return strings.Join(parts, "\n")
}
