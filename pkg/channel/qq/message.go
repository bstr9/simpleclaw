// Package qq 提供 QQ Bot 渠道的 WebSocket 实现。
// message.go 实现 QQ Bot 的消息解析和处理。
package qq

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/types"
)

// EventType QQ Bot WebSocket 事件类型常量
const (
	EventTypeGroupAtMessage = "GROUP_AT_MESSAGE_CREATE" // 群聊 @ 消息
	EventTypeC2CMessage     = "C2C_MESSAGE_CREATE"      // 私聊消息
	EventTypeATMessage      = "AT_MESSAGE_CREATE"       // 频道 @ 消息
	EventTypeDirectMessage  = "DIRECT_MESSAGE_CREATE"   // 频道私信
	EventTypeGroupAddRobot  = "GROUP_ADD_ROBOT"         // 机器人加入群聊
	EventTypeFriendAdd      = "FRIEND_ADD"              // 添加好友
)

// QQMessage 解析后的 QQ Bot 消息
type QQMessage struct {
	types.BaseMessage

	// QQ 特有字段
	EventType    string         `json:"event_type"`               // 事件类型
	GroupOpenID  string         `json:"group_openid,omitempty"`   // 群 OpenID
	UserOpenID   string         `json:"user_openid,omitempty"`    // 用户 OpenID
	MemberOpenID string         `json:"member_openid,omitempty"`  // 群成员 OpenID
	AuthorID     string         `json:"author_id,omitempty"`      // 作者 ID
	ActualUserID string         `json:"actual_user_id,omitempty"` // 实际用户 ID
	OtherUserID  string         `json:"other_user_id"`            // 对方用户 ID
	ImagePath    string         `json:"image_path,omitempty"`     // 图片路径
	RawMsg       map[string]any `json:"raw_msg"`                  // 原始消息
}

// NewQQMessage 从事件数据创建 QQ 消息
func NewQQMessage(eventData map[string]any, eventType string, tmpDir string) (*QQMessage, error) {
	msg := &QQMessage{
		EventType: eventType,
		RawMsg:    eventData,
	}

	msg.MsgID = getStringOr(eventData, "id", "")
	msg.CreateTime = parseTimestamp(getStringOr(eventData, "timestamp", ""))

	author, _ := eventData["author"].(map[string]any)
	fromUserID := getStringOr(author, "member_openid", getStringOr(author, "id", ""))
	groupOpenID := getStringOr(eventData, "group_openid", "")

	content := strings.TrimSpace(getStringOr(eventData, "content", ""))
	imageAttachments := extractImageAttachments(eventData)

	msg.parseContent(content, imageAttachments, tmpDir)
	msg.setEventFields(eventType, fromUserID, groupOpenID, author)
	msg.Context = types.NewContext(types.ContextType(msg.MsgType), msg.Content)

	return msg, nil
}

// parseTimestamp 解析时间戳
func parseTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Now()
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t
	}
	return time.Now()
}

// extractImageAttachments 提取图片附件
func extractImageAttachments(eventData map[string]any) []map[string]any {
	attachments, _ := eventData["attachments"].([]any)
	var imageAttachments []map[string]any
	for _, att := range attachments {
		if attMap, ok := att.(map[string]any); ok {
			contentType := getStringOr(attMap, "content_type", "")
			if strings.HasPrefix(contentType, "image/") {
				imageAttachments = append(imageAttachments, attMap)
			}
		}
	}
	return imageAttachments
}

// parseContent 解析消息内容
func (msg *QQMessage) parseContent(content string, imageAttachments []map[string]any, tmpDir string) {
	hasImage := len(imageAttachments) > 0

	if hasImage && content == "" {
		msg.MsgType = int(types.ContextImage)
		if len(imageAttachments) > 0 {
			imgPath, err := downloadImage(imageAttachments[0], tmpDir, msg.MsgID)
			if err == nil {
				msg.Content = imgPath
				msg.ImagePath = imgPath
			} else {
				msg.Content = "[Image download failed]"
			}
		}
		return
	}

	msg.MsgType = int(types.ContextText)
	msg.Content = content
	for idx, att := range imageAttachments {
		imgPath, err := downloadImage(att, tmpDir, fmt.Sprintf("%s_%d", msg.MsgID, idx))
		if err == nil {
			msg.Content += fmt.Sprintf("\n[图片: %s]", imgPath)
		}
	}
}

// setEventFields 根据事件类型设置字段
func (msg *QQMessage) setEventFields(eventType, fromUserID, groupOpenID string, author map[string]any) {
	msg.IsGroupMessage = eventType == EventTypeGroupAtMessage

	switch eventType {
	case EventTypeGroupAtMessage:
		msg.FromUserID = fromUserID
		msg.OtherUserID = groupOpenID
		msg.GroupID = groupOpenID
		msg.GroupOpenID = groupOpenID
		msg.ActualUserID = fromUserID
		msg.MemberOpenID = fromUserID
	case EventTypeC2CMessage:
		userOpenID := getStringOr(author, "user_openid", fromUserID)
		msg.FromUserID = userOpenID
		msg.OtherUserID = userOpenID
		msg.ActualUserID = userOpenID
		msg.UserOpenID = userOpenID
	case EventTypeATMessage:
		msg.setChannelFields(author, fromUserID)
	case EventTypeDirectMessage:
		msg.setDirectMessageFields(author, fromUserID, getStringOr(msg.RawMsg, "guild_id", ""))
	default:
	}
}

// setChannelFields 设置频道消息字段
func (msg *QQMessage) setChannelFields(_ map[string]any, fromUserID string) {
	channelID := getStringOr(msg.RawMsg, "channel_id", "")
	msg.FromUserID = fromUserID
	msg.OtherUserID = channelID
	msg.ActualUserID = fromUserID
}

// setDirectMessageFields 设置频道私信字段
func (msg *QQMessage) setDirectMessageFields(_ map[string]any, fromUserID, guildID string) {
	msg.FromUserID = fromUserID
	msg.OtherUserID = fmt.Sprintf("dm_%s_%s", guildID, fromUserID)
	msg.ActualUserID = fromUserID
}

// downloadImage 从 QQ 下载图片附件
func downloadImage(attachment map[string]any, tmpDir, msgID string) (string, error) {
	imgURL := getStringOr(attachment, "url", "")
	if imgURL == "" {
		return "", fmt.Errorf("no image URL in attachment")
	}

	// 确保 URL 有协议
	if !strings.HasPrefix(imgURL, "http://") && !strings.HasPrefix(imgURL, "https://") {
		imgURL = "https://" + imgURL
	}

	// 如需要则创建临时目录
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create tmp dir: %w", err)
	}

	// 下载图片
	resp, err := http.Get(imgURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("image download failed: %s", resp.Status)
	}

	// 确定文件扩展名
	ext := ".png"
	contentType := resp.Header.Get(common.HeaderContentType)
	switch {
	case strings.Contains(contentType, "jpeg"), strings.Contains(contentType, "jpg"):
		ext = ".jpg"
	case strings.Contains(contentType, "gif"):
		ext = ".gif"
	case strings.Contains(contentType, "webp"):
		ext = ".webp"
	}

	// 保存到文件
	savePath := filepath.Join(tmpDir, fmt.Sprintf("qq_%s%s", msgID, ext))
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %w", err)
	}

	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	return savePath, nil
}

// 辅助函数：从 map 提取值

func getStringOr(m map[string]any, key, def string) string {
	if m == nil {
		return def
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return def
}
