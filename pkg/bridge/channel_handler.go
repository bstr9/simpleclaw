// Package bridge 提供消息处理的核心路由层
// channel_handler.go 实现渠道消息处理器，连接渠道与 Bridge
package bridge

import (
	"context"
	"fmt"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/plugin"
	"github.com/bstr9/simpleclaw/pkg/types"
)

// channelMessageHandler 连接渠道到 Bridge 的消息处理器
// 实现 channel.MessageProcessor 接口
type channelMessageHandler struct {
	useSession bool
	pluginMgr  *plugin.Manager
}

// NewChannelMessageHandler 创建渠道消息处理器
// 返回 channel.MessageProcessor 接口，供 ChannelManager 依赖注入使用
func NewChannelMessageHandler() channel.MessageProcessor {
	return &channelMessageHandler{
		useSession: true,
		pluginMgr:  plugin.GetManager(),
	}
}

// FetchReply 获取消息回复（非流式）
func (h *channelMessageHandler) FetchReply(content string, ctx *types.Context) (*types.Reply, error) {
	return GetBridge().FetchReplyContent(content, ctx)
}

// FetchAgentReply 获取消息回复（流式，带事件回调）
func (h *channelMessageHandler) FetchAgentReply(content string, ctx *types.Context, onEvent func(event map[string]any)) (*types.Reply, error) {
	return GetBridge().FetchAgentReply(content, ctx, onEvent)
}

// HandleMessage 处理渠道消息（兼容旧接口）
func (h *channelMessageHandler) HandleMessage(ctx context.Context, msg types.ChatMessage) (*types.Reply, error) {
	kwargs := map[string]any{
		"session_id":   h.getSessionID(msg),
		"user_id":      msg.GetFromUserID(),
		"is_group":     msg.IsGroup(),
		"message_type": msg.GetMsgType(),
	}
	if msg.IsGroup() {
		kwargs["group_id"] = msg.GetGroupID()
	}

	msgCtx := msg.GetContext()
	if msgCtx == nil {
		msgCtx = types.NewContextWithKwargs(types.ContextText, msg.GetContent(), kwargs)
	} else {
		for k, v := range kwargs {
			msgCtx.Set(k, v)
		}
	}

	sessionID, _ := kwargs["session_id"].(string)

	var onEvent func(event map[string]any)
	if webMsg, ok := msg.(interface {
		GetOnEvent() func(event map[string]any)
	}); ok {
		onEvent = webMsg.GetOnEvent()
	}

	h.emitEvent(plugin.EventOnReceiveMessage, kwargs)

	var reply *types.Reply
	var err error
	if onEvent != nil {
		reply, err = GetBridge().FetchAgentReply(msg.GetContent(), msgCtx, onEvent)
	} else {
		reply, err = GetBridge().FetchReplyContent(msg.GetContent(), msgCtx)
	}

	if reply != nil {
		h.emitEvent(plugin.EventOnSendReply, map[string]any{
			"session_id": sessionID,
			"reply_type": reply.Type,
			"content":    reply.StringContent(),
		})
	}

	return reply, err
}

// ProcessMessageWithStream 流式处理消息（兼容旧接口）
func (h *channelMessageHandler) ProcessMessageWithStream(ctx context.Context, msg any, onEvent func(event map[string]any)) (*types.Reply, error) {
	chatMsg, ok := msg.(types.ChatMessage)
	if !ok {
		return nil, fmt.Errorf("invalid message type")
	}

	kwargs := map[string]any{
		"session_id":   h.getSessionID(chatMsg),
		"user_id":      chatMsg.GetFromUserID(),
		"is_group":     chatMsg.IsGroup(),
		"message_type": chatMsg.GetMsgType(),
	}
	if chatMsg.IsGroup() {
		kwargs["group_id"] = chatMsg.GetGroupID()
	}

	msgCtx := chatMsg.GetContext()
	if msgCtx == nil {
		msgCtx = types.NewContextWithKwargs(types.ContextText, chatMsg.GetContent(), kwargs)
	} else {
		for k, v := range kwargs {
			msgCtx.Set(k, v)
		}
	}

	h.emitEvent(plugin.EventOnReceiveMessage, kwargs)

	reply, err := GetBridge().FetchAgentReply(chatMsg.GetContent(), msgCtx, onEvent)

	if reply != nil {
		sessionID, _ := kwargs["session_id"].(string)
		h.emitEvent(plugin.EventOnSendReply, map[string]any{
			"session_id": sessionID,
			"reply_type": reply.Type,
			"content":    reply.StringContent(),
		})
	}

	return reply, err
}

func (h *channelMessageHandler) getSessionID(msg types.ChatMessage) string {
	if msg.IsGroup() && msg.GetGroupID() != "" {
		return "group_" + msg.GetGroupID()
	}
	if msg.GetFromUserID() != "" {
		return "user_" + msg.GetFromUserID()
	}
	return ""
}

func (h *channelMessageHandler) emitEvent(event plugin.Event, data map[string]any) {
	if h.pluginMgr == nil {
		return
	}
	ec := plugin.NewEventContext(event, data)
	_ = h.pluginMgr.PublishEvent(event, ec)
}
