// Package types 提供 simpleclaw 应用的核心类型定义。
// context.go 定义消息上下文类型，用于表示接收到的消息类型和内容。
package types

import "fmt"

// ContextType 定义消息上下文类型枚举。
type ContextType int

const (
	// ContextText 文本消息
	ContextText ContextType = iota + 1 // 1
	// ContextVoice 语音消息
	ContextVoice // 2
	// ContextImage 图片消息
	ContextImage // 3
	// ContextFile 文件消息
	ContextFile // 4
	// ContextVideo 视频消息
	ContextVideo // 5
	// ContextSharing 分享消息
	ContextSharing // 6
	// 预留 7-9
	_ // 7
	_ // 8
	_ // 9
	// ContextImageCreate 图片生成请求
	ContextImageCreate // 10
	// 预留 11-18
	_ // 11
	_ // 12
	_ // 13
	_ // 14
	_ // 15
	_ // 16
	_ // 17
	_ // 18
	// ContextAcceptFriend 接受好友请求
	ContextAcceptFriend // 19
	// ContextJoinGroup 加入群聊
	ContextJoinGroup // 20
	// ContextPatPat 拍一拍
	ContextPatPat // 21
	// ContextFunction 函数调用
	ContextFunction // 22
	// ContextExitGroup 退出群聊
	ContextExitGroup // 23
)

// String 返回 ContextType 的字符串表示，用于日志输出
func (ct ContextType) String() string {
	names := map[ContextType]string{
		ContextText:         "TEXT",
		ContextVoice:        "VOICE",
		ContextImage:        "IMAGE",
		ContextFile:         "FILE",
		ContextVideo:        "VIDEO",
		ContextSharing:      "SHARING",
		ContextImageCreate:  "IMAGE_CREATE",
		ContextAcceptFriend: "ACCEPT_FRIEND",
		ContextJoinGroup:    "JOIN_GROUP",
		ContextPatPat:       "PATPAT",
		ContextFunction:     "FUNCTION",
		ContextExitGroup:    "EXIT_GROUP",
	}
	if name, ok := names[ct]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", ct)
}

// Context 表示消息上下文，包含类型、内容和扩展字段。
type Context struct {
	// Type 消息类型
	Type ContextType `json:"type"`
	// Content 消息内容
	Content any `json:"content"`
	// Kwargs 扩展字段，用于存储额外的上下文信息
	Kwargs map[string]any `json:"kwargs,omitempty"`
}

// NewContext 创建新的 Context 实例
func NewContext(typ ContextType, content any) *Context {
	return &Context{
		Type:    typ,
		Content: content,
		Kwargs:  make(map[string]any),
	}
}

// NewContextWithKwargs 创建带有扩展字段的 Context 实例
func NewContextWithKwargs(typ ContextType, content any, kwargs map[string]any) *Context {
	if kwargs == nil {
		kwargs = make(map[string]any)
	}
	return &Context{
		Type:    typ,
		Content: content,
		Kwargs:  kwargs,
	}
}

// Get 从 Kwargs 中获取指定键的值
// 如果键不存在，返回 nil 和 false
func (c *Context) Get(key string) (any, bool) {
	if c.Kwargs == nil {
		return nil, false
	}
	val, ok := c.Kwargs[key]
	return val, ok
}

// GetString 从 Kwargs 中获取字符串值
// 如果键不存在或类型不匹配，返回空字符串和 false
func (c *Context) GetString(key string) (string, bool) {
	val, ok := c.Get(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetInt 从 Kwargs 中获取整数值
func (c *Context) GetInt(key string) (int, bool) {
	val, ok := c.Get(key)
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

// GetBool 从 Kwargs 中获取布尔值
func (c *Context) GetBool(key string) (bool, bool) {
	val, ok := c.Get(key)
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// Set 向 Kwargs 中设置键值对
// 如果 Kwargs 为 nil，会自动初始化
func (c *Context) Set(key string, value any) {
	if c.Kwargs == nil {
		c.Kwargs = make(map[string]any)
	}
	c.Kwargs[key] = value
}

// Contains 检查 Kwargs 中是否存在指定键
func (c *Context) Contains(key string) bool {
	if c.Kwargs == nil {
		return false
	}
	_, ok := c.Kwargs[key]
	return ok
}

// Delete 从 Kwargs 中删除指定键
func (c *Context) Delete(key string) {
	if c.Kwargs != nil {
		delete(c.Kwargs, key)
	}
}

// Clear 清空 Kwargs
func (c *Context) Clear() {
	c.Kwargs = make(map[string]any)
}

// Clone 创建 Context 的深拷贝
func (c *Context) Clone() *Context {
	kwargs := make(map[string]any)
	for k, v := range c.Kwargs {
		kwargs[k] = v
	}
	return &Context{
		Type:    c.Type,
		Content: c.Content,
		Kwargs:  kwargs,
	}
}

// String 返回 Context 的字符串表示，用于日志输出
func (c *Context) String() string {
	return fmt.Sprintf("Context{Type: %s, Content: %v, Kwargs: %v}",
		c.Type, c.Content, c.Kwargs)
}
