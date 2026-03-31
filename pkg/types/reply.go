// Package types 提供 simpleclaw 应用的核心类型定义。
// reply.go 定义回复类型，用于表示机器人响应消息的类型和内容。
package types

import "fmt"

// ReplyType 定义回复类型枚举。
type ReplyType int

const (
	// ReplyText 文本回复
	ReplyText ReplyType = iota + 1 // 1
	// ReplyVoice 语音回复
	ReplyVoice // 2
	// ReplyImage 图片回复（本地文件）
	ReplyImage // 3
	// ReplyImageURL 图片URL回复
	ReplyImageURL // 4
	// ReplyVideoURL 视频URL回复
	ReplyVideoURL // 5
	// ReplyFile 文件回复
	ReplyFile // 6
	// ReplyCard 名片回复
	ReplyCard // 7
	// ReplyInviteRoom 邀请入群回复
	ReplyInviteRoom // 8
	// ReplyInfo 信息回复（系统提示）
	ReplyInfo // 9
	// ReplyError 错误回复
	ReplyError // 10
	// ReplyText_ 文本回复（备用）
	ReplyText_ // 11
	// ReplyVideo 视频回复（本地文件）
	ReplyVideo // 12
	// ReplyMiniApp 小程序回复
	ReplyMiniApp // 13
)

// String 返回 ReplyType 的字符串表示，用于日志输出
func (rt ReplyType) String() string {
	names := map[ReplyType]string{
		ReplyText:       "TEXT",
		ReplyVoice:      "VOICE",
		ReplyImage:      "IMAGE",
		ReplyImageURL:   "IMAGE_URL",
		ReplyVideoURL:   "VIDEO_URL",
		ReplyFile:       "FILE",
		ReplyCard:       "CARD",
		ReplyInviteRoom: "INVITE_ROOM",
		ReplyInfo:       "INFO",
		ReplyError:      "ERROR",
		ReplyText_:      "TEXT_",
		ReplyVideo:      "VIDEO",
		ReplyMiniApp:    "MINIAPP",
	}
	if name, ok := names[rt]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", rt)
}

// Reply 表示响应消息，包含类型和内容。
type Reply struct {
	// Type 回复类型
	Type ReplyType `json:"type"`
	// Content 回复内容
	Content any `json:"content"`
}

// NewReply 创建新的 Reply 实例
func NewReply(typ ReplyType, content any) *Reply {
	return &Reply{
		Type:    typ,
		Content: content,
	}
}

// NewTextReply 创建文本回复
func NewTextReply(text string) *Reply {
	return &Reply{
		Type:    ReplyText,
		Content: text,
	}
}

// NewErrorReply 创建错误回复
func NewErrorReply(errMsg string) *Reply {
	return &Reply{
		Type:    ReplyError,
		Content: errMsg,
	}
}

// NewInfoReply 创建信息回复（系统提示）
func NewInfoReply(info string) *Reply {
	return &Reply{
		Type:    ReplyInfo,
		Content: info,
	}
}

// NewImageReply 创建图片回复（本地文件路径）
func NewImageReply(imagePath string) *Reply {
	return &Reply{
		Type:    ReplyImage,
		Content: imagePath,
	}
}

// NewImageURLReply 创建图片URL回复
func NewImageURLReply(imageURL string) *Reply {
	return &Reply{
		Type:    ReplyImageURL,
		Content: imageURL,
	}
}

// NewVoiceReply 创建语音回复
func NewVoiceReply(voicePath string) *Reply {
	return &Reply{
		Type:    ReplyVoice,
		Content: voicePath,
	}
}

// NewVideoReply 创建视频回复（本地文件路径）
func NewVideoReply(videoPath string) *Reply {
	return &Reply{
		Type:    ReplyVideo,
		Content: videoPath,
	}
}

// NewVideoURLReply 创建视频URL回复
func NewVideoURLReply(videoURL string) *Reply {
	return &Reply{
		Type:    ReplyVideoURL,
		Content: videoURL,
	}
}

// NewFileReply 创建文件回复
func NewFileReply(filePath string) *Reply {
	return &Reply{
		Type:    ReplyFile,
		Content: filePath,
	}
}

// NewCardReply 创建名片回复
func NewCardReply(cardInfo any) *Reply {
	return &Reply{
		Type:    ReplyCard,
		Content: cardInfo,
	}
}

// NewInviteRoomReply 创建邀请入群回复
func NewInviteRoomReply(roomInfo any) *Reply {
	return &Reply{
		Type:    ReplyInviteRoom,
		Content: roomInfo,
	}
}

// NewMiniAppReply 创建小程序回复
func NewMiniAppReply(miniAppInfo any) *Reply {
	return &Reply{
		Type:    ReplyMiniApp,
		Content: miniAppInfo,
	}
}

// IsText 检查是否为文本类型回复
func (r *Reply) IsText() bool {
	return r.Type == ReplyText || r.Type == ReplyText_
}

// IsError 检查是否为错误回复
func (r *Reply) IsError() bool {
	return r.Type == ReplyError
}

// IsInfo 检查是否为信息回复
func (r *Reply) IsInfo() bool {
	return r.Type == ReplyInfo
}

// IsMedia 检查是否为媒体类型回复（图片、视频、语音、文件）
func (r *Reply) IsMedia() bool {
	switch r.Type {
	case ReplyImage, ReplyImageURL, ReplyVoice, ReplyVideo, ReplyVideoURL, ReplyFile:
		return true
	default:
		return false
	}
}

// StringContent 安全地获取字符串内容
// 如果内容不是字符串类型，返回格式化的字符串
func (r *Reply) StringContent() string {
	if str, ok := r.Content.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", r.Content)
}

// String 返回 Reply 的字符串表示，用于日志输出
func (r *Reply) String() string {
	return fmt.Sprintf("Reply{Type: %s, Content: %v}", r.Type, r.Content)
}
