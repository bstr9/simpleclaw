// Package types 提供 simpleclaw 应用的核心类型定义。
// message.go 定义消息接口，用于抽象不同渠道的消息结构。
package types

import "time"

// ChatMessage 定义聊天消息接口
// 不同渠道（微信、飞书等）需要实现此接口以统一消息处理
type ChatMessage interface {
	// GetMsgID 获取消息唯一标识
	GetMsgID() string
	// GetFromUserID 获取发送者ID
	GetFromUserID() string
	// GetToUserID 获取接收者ID
	GetToUserID() string
	// GetContent 获取消息内容
	GetContent() string
	// GetCreateTime 获取消息创建时间
	GetCreateTime() time.Time
	// IsGroup 判断是否为群聊消息
	IsGroup() bool
	// GetGroupID 获取群组ID（如果是群聊消息）
	GetGroupID() string
	// GetMsgType 获取原始消息类型
	GetMsgType() int
	// GetContext 获取消息上下文
	GetContext() *Context
}

// BaseMessage 提供基础消息结构，可嵌入到具体消息实现中
type BaseMessage struct {
	// MsgID 消息唯一标识
	MsgID string `json:"msg_id"`
	// FromUserID 发送者ID
	FromUserID string `json:"from_user_id"`
	// ToUserID 接收者ID
	ToUserID string `json:"to_user_id"`
	// Content 消息内容
	Content string `json:"content"`
	// CreateTime 消息创建时间
	CreateTime time.Time `json:"create_time"`
	// IsGroupMessage 是否为群聊消息
	IsGroupMessage bool `json:"is_group"`
	// GroupID 群组ID
	GroupID string `json:"group_id,omitempty"`
	// MsgType 原始消息类型
	MsgType int `json:"msg_type"`
	// Context 消息上下文
	Context *Context `json:"context,omitempty"`
}

// GetMsgID 实现 ChatMessage 接口
func (m *BaseMessage) GetMsgID() string {
	return m.MsgID
}

// GetFromUserID 实现 ChatMessage 接口
func (m *BaseMessage) GetFromUserID() string {
	return m.FromUserID
}

// GetToUserID 实现 ChatMessage 接口
func (m *BaseMessage) GetToUserID() string {
	return m.ToUserID
}

// GetContent 实现 ChatMessage 接口
func (m *BaseMessage) GetContent() string {
	return m.Content
}

// GetCreateTime 实现 ChatMessage 接口
func (m *BaseMessage) GetCreateTime() time.Time {
	return m.CreateTime
}

// IsGroup 实现 ChatMessage 接口
func (m *BaseMessage) IsGroup() bool {
	return m.IsGroupMessage
}

// GetGroupID 实现 ChatMessage 接口
func (m *BaseMessage) GetGroupID() string {
	return m.GroupID
}

// GetMsgType 实现 ChatMessage 接口
func (m *BaseMessage) GetMsgType() int {
	return m.MsgType
}

// GetContext 实现 ChatMessage 接口
func (m *BaseMessage) GetContext() *Context {
	return m.Context
}

// SetContext 设置消息上下文
func (m *BaseMessage) SetContext(ctx *Context) {
	m.Context = ctx
}

// NewBaseMessage 创建基础消息实例
func NewBaseMessage(msgID, fromUserID, toUserID, content string) *BaseMessage {
	return &BaseMessage{
		MsgID:      msgID,
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		Content:    content,
		CreateTime: time.Now(),
	}
}

// NewGroupMessage 创建群聊消息实例
func NewGroupMessage(msgID, fromUserID, toUserID, groupID, content string) *BaseMessage {
	return &BaseMessage{
		MsgID:          msgID,
		FromUserID:     fromUserID,
		ToUserID:       toUserID,
		Content:        content,
		CreateTime:     time.Now(),
		IsGroupMessage: true,
		GroupID:        groupID,
	}
}

// MessageOption 消息选项函数类型
type MessageOption func(*BaseMessage)

// WithMsgType 设置消息类型选项
func WithMsgType(msgType int) MessageOption {
	return func(m *BaseMessage) {
		m.MsgType = msgType
	}
}

// WithCreateTime 设置创建时间选项
func WithCreateTime(t time.Time) MessageOption {
	return func(m *BaseMessage) {
		m.CreateTime = t
	}
}

// WithContext 设置上下文选项
func WithContext(ctx *Context) MessageOption {
	return func(m *BaseMessage) {
		m.Context = ctx
	}
}

// NewTextMessage 创建文本消息
func NewTextMessage(msgID, fromUserID, toUserID, content string, opts ...MessageOption) *BaseMessage {
	msg := NewBaseMessage(msgID, fromUserID, toUserID, content)
	msg.MsgType = int(ContextText)
	msg.Context = NewContext(ContextText, content)
	for _, opt := range opts {
		opt(msg)
	}
	return msg
}

// NewGroupTextMessage 创建群聊文本消息
func NewGroupTextMessage(msgID, fromUserID, toUserID, groupID, content string, opts ...MessageOption) *BaseMessage {
	msg := NewGroupMessage(msgID, fromUserID, toUserID, groupID, content)
	msg.MsgType = int(ContextText)
	msg.Context = NewContext(ContextText, content)
	for _, opt := range opts {
		opt(msg)
	}
	return msg
}

// MessageHandler 消息处理函数类型
type MessageHandler func(msg ChatMessage) *Reply

// MessageFilter 消息过滤函数类型
type MessageFilter func(msg ChatMessage) bool

// MessageMiddleware 消息中间件函数类型
type MessageMiddleware func(next MessageHandler) MessageHandler
