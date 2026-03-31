package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"go.uber.org/zap"
)

type WSClient struct {
	appID     string
	appSecret string
	client    *larkws.Client
	channel   *FeishuChannel
	stopOnce  sync.Once
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewWSClient(appID, appSecret string, ch *FeishuChannel) *WSClient {
	return &WSClient{
		appID:     appID,
		appSecret: appSecret,
		channel:   ch,
	}
}

func (w *WSClient) Start(ctx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(ctx)

	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(w.handleMessage).
		OnP2MessageReadV1(w.handleMessageRead).
		OnP2MessageReactionCreatedV1(w.handleReactionCreated).
		OnP2MessageReactionDeletedV1(w.handleReactionDeleted)

	// 根据主程序日志级别设置 lark SDK 日志级别
	larkLogLevel := larkcore.LogLevelInfo
	if logger.IsDebug() {
		larkLogLevel = larkcore.LogLevelDebug
	}

	w.client = larkws.NewClient(w.appID, w.appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkLogLevel),
	)

	logger.Info("[Feishu WS] 启动 WebSocket 连接",
		zap.String("app_id", maskAppID(w.appID)))

	go func() {
		if err := w.client.Start(w.ctx); err != nil {
			logger.Error("[Feishu WS] 连接错误", zap.Error(err))
		}
	}()

	return nil
}

func (w *WSClient) handleMessageRead(ctx context.Context, event *larkim.P2MessageReadV1) error {
	return nil
}

func (w *WSClient) handleReactionCreated(ctx context.Context, event *larkim.P2MessageReactionCreatedV1) error {
	return nil
}

func (w *WSClient) handleReactionDeleted(ctx context.Context, event *larkim.P2MessageReactionDeletedV1) error {
	return nil
}

func (w *WSClient) Stop() error {
	w.stopOnce.Do(func() {
		if w.cancel != nil {
			w.cancel()
		}
		logger.Info("[Feishu WS] 连接已停止")
	})
	return nil
}

func (w *WSClient) handleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event.Event == nil || event.Event.Message == nil {
		return nil
	}

	msg := event.Event.Message
	sender := event.Event.Sender
	msgID := safeString(msg.MessageId)

	if w.channel.isMessageProcessed(msgID) {
		logger.Debug("[Feishu WS] 消息已处理，跳过", zap.String("msg_id", msgID))
		return nil
	}
	w.channel.markMessageProcessed(msgID)

	logger.Debug("[Feishu WS] 收到消息",
		zap.String("msg_id", msgID),
		zap.String("msg_type", safeString(msg.MessageType)),
		zap.String("chat_id", safeString(msg.ChatId)),
		zap.String("content", safeString(msg.Content)))

	feishuMsg := &FeishuMessage{
		MsgID:        msgID,
		CreateTime:   parseCreateTime(safeString(msg.CreateTime)),
		IsGroupChat:  safeString(msg.ChatType) == "group",
		ChatID:       safeString(msg.ChatId),
		SenderID:     safeString(sender.SenderId.UserId),
		SenderOpenID: safeString(sender.SenderId.OpenId),
		OpenID:       safeString(sender.SenderId.OpenId),
	}
	feishuMsg.MsgType = MessageType(safeString(msg.MessageType))
	feishuMsg.RawContent = safeString(msg.Content)
	feishuMsg.CtxType = types.ContextText
	feishuMsg.FromUserID = safeString(sender.SenderId.OpenId)
	feishuMsg.GroupID = safeString(msg.ChatId)
	feishuMsg.IsGroupMessage = feishuMsg.IsGroupChat

	var contentStr string
	if msg.Content != nil {
		contentStr = *msg.Content
	}
	w.parseMessageContent(feishuMsg, contentStr)

	logger.Debug("[Feishu WS] 解析后消息",
		zap.String("content_text", feishuMsg.ContentText),
		zap.String("content", feishuMsg.Content),
		zap.String("open_id", feishuMsg.OpenID))

	// 异步处理消息，避免阻塞 WebSocket
	go w.processMessageAsync(feishuMsg)

	return nil
}

func (w *WSClient) processMessageAsync(feishuMsg *FeishuMessage) {
	msgID := feishuMsg.MsgID

	logger.Info("[Feishu WS] 开始异步处理消息", zap.String("msg_id", msgID))

	reactionID, err := w.channel.SendTypingReaction(msgID)
	if err != nil {
		logger.Warn("[Feishu WS] 发送 Typing 表情失败", zap.Error(err))
	}

	if w.channel == nil || w.channel.messageHandler == nil {
		logger.Warn("[Feishu WS] 消息处理器为空", zap.String("msg_id", msgID))
		w.channel.RemoveTypingReaction(msgID, reactionID)
		return
	}

	msgCtx := types.NewContext(feishuMsg.CtxType, feishuMsg.ContentText)
	msgCtx.Set("msg", feishuMsg)
	msgCtx.Set("receiver", feishuMsg.SenderOpenID)
	msgCtx.Set("receive_id_type", "open_id")
	msgCtx.Set("session_id", feishuMsg.GetSessionID(w.channel.config.GroupSharedSession))
	msgCtx.Set("stream_output", w.channel.config.StreamOutput)
	msgCtx.Set("channel_type", "feishu")

	// 设置额外的上下文数据到消息对象，以便 GetContext() 可以返回这些数据
	feishuMsg.SetExtraContext("msg", feishuMsg)
	feishuMsg.SetExtraContext("receiver", feishuMsg.SenderOpenID)
	feishuMsg.SetExtraContext("receive_id_type", "open_id")
	feishuMsg.SetExtraContext("session_id", feishuMsg.GetSessionID(w.channel.config.GroupSharedSession))
	feishuMsg.SetExtraContext("channel_type", "feishu")

	logger.Info("[Feishu WS] 调用消息处理器", zap.String("msg_id", msgID))

	sessionID := feishuMsg.GetSessionID(w.channel.config.GroupSharedSession)
	userID := feishuMsg.SenderOpenID

	if w.channel.pairManager != nil {
		pairResult, err := w.channel.pairManager.CheckSessionPair(sessionID, userID, "feishu")
		if err != nil {
			logger.Warn("[Feishu WS] Pair check failed", zap.Error(err))
		} else if !pairResult.Paired {
			pairStart, err := w.channel.pairManager.StartPair(sessionID, userID, "feishu")
			if err != nil {
				logger.Error("[Feishu WS] Failed to start pair", zap.Error(err))
			} else {
				replyMsg := fmt.Sprintf("请先授权以使用完整功能：\n%s", pairStart.AuthURL)
				reply := &types.Reply{Type: types.ReplyText, Content: replyMsg}
				if sendErr := w.channel.Send(reply, msgCtx); sendErr != nil {
					logger.Error("[Feishu WS] 发送配对链接失败", zap.Error(sendErr))
				}
			}
			w.channel.RemoveTypingReaction(msgID, reactionID)
			return
		}
	}

	var reply *types.Reply
	var procErr error

	if w.channel.config.StreamOutput {
		reply, procErr = w.processWithStream(feishuMsg, msgCtx)
	} else {
		reply, procErr = w.channel.messageHandler.ProcessMessage(context.Background(), feishuMsg)
	}

	if procErr != nil {
		logger.Error("[Feishu WS] 处理消息失败", zap.Error(procErr))
		w.channel.RemoveTypingReaction(msgID, reactionID)
		return
	}

	logger.Info("[Feishu WS] 消息处理完成", zap.String("msg_id", msgID), zap.Bool("has_reply", reply != nil))

	if reply != nil {
		if sendErr := w.channel.Send(reply, msgCtx); sendErr != nil {
			logger.Error("[Feishu WS] 发送回复失败", zap.Error(sendErr))
		} else {
			logger.Info("[Feishu WS] 回复已发送", zap.String("msg_id", msgID))
		}
	}

	w.channel.RemoveTypingReaction(msgID, reactionID)
}

func (w *WSClient) processWithStream(feishuMsg *FeishuMessage, msgCtx *types.Context) (*types.Reply, error) {
	receiver, _ := msgCtx.GetString("receiver")
	receiveIDType, _ := msgCtx.GetString("receive_id_type")
	sessionID, _ := msgCtx.GetString("session_id")

	cardController := NewStreamingCardController(w.channel, feishuMsg, map[string]any{
		"receiver":        receiver,
		"receive_id_type": receiveIDType,
		"session_id":      sessionID,
	})

	onEvent := func(event map[string]any) {
		eventType, _ := event["type"].(string)
		if eventType != "text" {
			return
		}

		text, _ := event["text"].(string)
		if text == "" {
			return
		}

		cardController.UpdateText(text)
	}

	streamHandler := &streamCardHandler{
		inner:     w.channel.messageHandler,
		onEvent:   onEvent,
		feishuMsg: feishuMsg,
		cardCtrl:  cardController,
	}

	return streamHandler.ProcessMessage(context.Background(), feishuMsg)
}

type streamCardHandler struct {
	inner     FeishuMessageProcessor
	onEvent   func(event map[string]any)
	feishuMsg *FeishuMessage
	cardCtrl  *StreamingCardController
}

func (h *streamCardHandler) ProcessMessage(ctx context.Context, msg *FeishuMessage) (*types.Reply, error) {
	var reply *types.Reply
	var err error

	if processor, ok := h.inner.(interface {
		ProcessMessageWithStream(ctx context.Context, msg *FeishuMessage, onEvent func(event map[string]any)) (*types.Reply, error)
	}); ok {
		reply, err = processor.ProcessMessageWithStream(ctx, msg, h.onEvent)
	} else {
		reply, err = h.inner.ProcessMessage(ctx, msg)
	}

	h.cardCtrl.Complete()

	if h.cardCtrl.HasContent() {
		return nil, err
	}
	return reply, err
}

func (w *WSClient) parseMessageContent(msg *FeishuMessage, content string) {
	if content == "" {
		return
	}

	switch string(msg.MsgType) {
	case "text":
		var textContent TextContent
		if err := json.Unmarshal([]byte(content), &textContent); err == nil {
			msg.ContentText = textContent.Text
			msg.Content = textContent.Text
		}
	case "post":
		var postContent PostContent
		if err := json.Unmarshal([]byte(content), &postContent); err == nil {
			msg.ContentText = extractTextFromPostContent(postContent)
			msg.Content = msg.ContentText
		}
	default:
		msg.ContentText = content
		msg.Content = content
	}
}

func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func parseCreateTime(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	if len(s) > 10 {
		timestamp := s[:10] + "." + s[10:]
		if t, err := time.Parse("2006-01-02T15:04:05.999", timestamp); err == nil {
			return t
		}
	}
	return time.Now()
}

func maskAppID(appID string) string {
	if len(appID) <= 8 {
		return appID
	}
	return appID[:4] + "****" + appID[len(appID)-4:]
}

func extractTextFromPostContent(post PostContent) string {
	var result string
	for _, paragraph := range post.Content {
		for _, elem := range paragraph {
			if elem.Text != "" {
				result += elem.Text
			}
		}
	}
	return result
}
