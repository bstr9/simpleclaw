// Package channel 提供消息渠道的抽象接口和基础实现。
// channel.go 定义了 Channel 接口和 BaseChannel 基础结构。
package channel

import (
	"context"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/types"
)

// Channel 定义消息渠道接口
// 所有渠道（微信、飞书、Web 等）都需要实现此接口
type Channel interface {
	// Startup 启动渠道
	// ctx 用于控制启动超时和取消
	Startup(ctx context.Context) error

	// Stop 停止渠道，优雅关闭
	Stop() error

	// Send 发送消息给用户
	// reply: 回复内容
	// ctx: 消息上下文
	Send(reply *types.Reply, ctx *types.Context) error

	// ChannelType 返回渠道类型标识
	ChannelType() string

	// Name 返回当前登录的用户名
	Name() string

	// UserID 返回当前登录的用户ID
	UserID() string
}

// EventHandler 定义事件回调函数类型
// 用于 Agent 模式下的事件流（如 SSE）
type EventHandler func(event map[string]any)

// BaseChannel 提供渠道的基础实现
// 可嵌入到具体渠道实现中
type BaseChannel struct {
	mu              sync.RWMutex
	channelType     string
	name            string
	userID          string
	started         bool
	startupErr      error
	cloudMode       bool
	notSupportTypes []types.ReplyType
}

// NewBaseChannel 创建基础渠道实例
func NewBaseChannel(channelType string) *BaseChannel {
	return &BaseChannel{
		channelType: channelType,
		notSupportTypes: []types.ReplyType{
			types.ReplyVoice,
			types.ReplyImage,
		},
	}
}

// ChannelType 返回渠道类型
func (b *BaseChannel) ChannelType() string {
	return b.channelType
}

// Name 返回用户名
func (b *BaseChannel) Name() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.name
}

// SetName 设置用户名
func (b *BaseChannel) SetName(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.name = name
}

// UserID 返回用户ID
func (b *BaseChannel) UserID() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.userID
}

// SetUserID 设置用户ID
func (b *BaseChannel) SetUserID(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.userID = id
}

// IsStarted 检查渠道是否已启动
func (b *BaseChannel) IsStarted() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.started
}

// SetStarted 设置启动状态
func (b *BaseChannel) SetStarted(started bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started = started
}

// ReportStartupSuccess 报告启动成功
func (b *BaseChannel) ReportStartupSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started = true
	b.startupErr = nil
}

// ReportStartupError 报告启动失败
func (b *BaseChannel) ReportStartupError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started = false
	b.startupErr = err
}

// StartupError 获取启动错误
func (b *BaseChannel) StartupError() error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.startupErr
}

// SetCloudMode 设置云模式
func (b *BaseChannel) SetCloudMode(cloud bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cloudMode = cloud
}

// IsCloudMode 检查是否为云模式
func (b *BaseChannel) IsCloudMode() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.cloudMode
}

// NotSupportTypes 返回不支持的回复类型
func (b *BaseChannel) NotSupportTypes() []types.ReplyType {
	return b.notSupportTypes
}

// SetNotSupportTypes 设置不支持的回复类型
func (b *BaseChannel) SetNotSupportTypes(types []types.ReplyType) {
	b.notSupportTypes = types
}

// IsReplyTypeSupported 检查回复类型是否支持
func (b *BaseChannel) IsReplyTypeSupported(replyType types.ReplyType) bool {
	for _, t := range b.notSupportTypes {
		if t == replyType {
			return false
		}
	}
	return true
}

// Stop 基础停止实现（可被覆盖）
func (b *BaseChannel) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.started = false
	return nil
}

// Send 基础发送实现（需要子类实现）
func (b *BaseChannel) Send(reply *types.Reply, ctx *types.Context) error {
	return nil
}

// Startup 基础启动实现（需要子类实现）
func (b *BaseChannel) Startup(ctx context.Context) error {
	return nil
}
