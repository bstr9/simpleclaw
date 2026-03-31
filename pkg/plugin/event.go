// Package plugin 提供插件系统，用于扩展 simpleclaw 应用程序。
// event.go 定义插件事件处理的类型和动作。
package plugin

import (
	"fmt"
	"sync"
)

// Event 表示插件可以监听的事件类型。
type Event int

const (
	// EventOnReceiveMessage 在收到消息时触发。
	// EventContext 包含: channel, context
	EventOnReceiveMessage Event = iota + 1 // 1

	// EventOnHandleContext 在处理消息之前触发。
	// EventContext 包含: channel, context, reply
	EventOnHandleContext // 2

	// EventOnDecorateReply 在获取回复后、装饰之前触发。
	// EventContext 包含: channel, context, reply
	EventOnDecorateReply // 3

	// EventOnSendReply 在发送回复之前触发。
	// EventContext 包含: channel, context, reply
	EventOnSendReply // 4
)

// String 返回 Event 的字符串表示。
func (e Event) String() string {
	names := map[Event]string{
		EventOnReceiveMessage: "ON_RECEIVE_MESSAGE",
		EventOnHandleContext:  "ON_HANDLE_CONTEXT",
		EventOnDecorateReply:  "ON_DECORATE_REPLY",
		EventOnSendReply:      "ON_SEND_REPLY",
	}
	if name, ok := names[e]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", e)
}

// EventAction 表示事件处理器处理事件后要执行的动作。
type EventAction int

const (
	// ActionContinue 表示事件继续传递给下一个插件处理器。
	// 如果没有更多处理器，则应用默认事件处理逻辑。
	ActionContinue EventAction = iota + 1 // 1

	// ActionBreak 表示事件在此停止，并执行默认处理逻辑。
	// 此事件不再调用更多插件处理器。
	ActionBreak // 2

	// ActionBreakPass 表示事件在此停止，并跳过默认处理。
	// 不再调用更多插件处理器，且跳过默认逻辑。
	ActionBreakPass // 3
)

// String 返回 EventAction 的字符串表示。
func (a EventAction) String() string {
	names := map[EventAction]string{
		ActionContinue:  "CONTINUE",
		ActionBreak:     "BREAK",
		ActionBreakPass: "BREAK_PASS",
	}
	if name, ok := names[a]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", a)
}

// EventContext 包含事件的上下文数据。
// 它提供线程安全的事件数据访问，并允许插件控制事件流程。
type EventContext struct {
	// Event 是正在处理的事件类型。
	Event Event `json:"event"`

	// Data 包含事件特定的数据。
	Data map[string]any `json:"data,omitempty"`

	// Action 控制处理器处理后的事件流程。
	action EventAction

	// breakedBy 记录哪个插件中断了事件链。
	breakedBy string

	// mu 保护 Data 和 action 的并发访问。
	mu sync.RWMutex
}

// NewEventContext 创建一个具有指定事件类型和数据的新 EventContext。
func NewEventContext(event Event, data map[string]any) *EventContext {
	if data == nil {
		data = make(map[string]any)
	}
	return &EventContext{
		Event:  event,
		Data:   data,
		action: ActionContinue,
	}
}

// Get 根据键从事件数据中获取值。
func (ec *EventContext) Get(key string) (any, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	if ec.Data == nil {
		return nil, false
	}
	val, ok := ec.Data[key]
	return val, ok
}

// GetString 从事件数据中获取字符串值。
func (ec *EventContext) GetString(key string) (string, bool) {
	val, ok := ec.Get(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetInt 从事件数据中获取整数值。
func (ec *EventContext) GetInt(key string) (int, bool) {
	val, ok := ec.Get(key)
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

// GetBool 从事件数据中获取布尔值。
func (ec *EventContext) GetBool(key string) (bool, bool) {
	val, ok := ec.Get(key)
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// Set 在事件数据中存储键值对。
func (ec *EventContext) Set(key string, value any) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if ec.Data == nil {
		ec.Data = make(map[string]any)
	}
	ec.Data[key] = value
}

// Delete 从事件数据中删除键。
func (ec *EventContext) Delete(key string) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if ec.Data != nil {
		delete(ec.Data, key)
	}
}

// Action 返回当前事件动作。
func (ec *EventContext) Action() EventAction {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.action
}

// SetAction 设置事件动作以控制事件流程。
func (ec *EventContext) SetAction(action EventAction) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.action = action
}

// Break 将动作设置为 ActionBreak，并记录中断事件的插件名称。
func (ec *EventContext) Break(pluginName string) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.action = ActionBreak
	ec.breakedBy = pluginName
}

// BreakPass 将动作设置为 ActionBreakPass，并记录中断事件的插件名称。
func (ec *EventContext) BreakPass(pluginName string) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.action = ActionBreakPass
	ec.breakedBy = pluginName
}

// IsBreak 返回事件链是否应该停止。
func (ec *EventContext) IsBreak() bool {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.action == ActionBreak || ec.action == ActionBreakPass
}

// IsPass 返回是否应跳过默认处理。
func (ec *EventContext) IsPass() bool {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.action == ActionBreakPass
}

// BreakedBy 返回中断事件链的插件名称。
func (ec *EventContext) BreakedBy() string {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.breakedBy
}

// Clone 创建 EventContext 的浅拷贝。
func (ec *EventContext) Clone() *EventContext {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	data := make(map[string]any)
	for k, v := range ec.Data {
		data[k] = v
	}
	return &EventContext{
		Event:     ec.Event,
		Data:      data,
		action:    ec.action,
		breakedBy: ec.breakedBy,
	}
}

// String 返回 EventContext 的字符串表示。
func (ec *EventContext) String() string {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return fmt.Sprintf("EventContext{Event: %s, Action: %s, BreakedBy: %s}",
		ec.Event, ec.action, ec.breakedBy)
}
