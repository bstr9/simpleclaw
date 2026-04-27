// feishu 包提供飞书渠道实现。
// handler.go 定义 HTTP 处理器和事件处理逻辑。
package feishu

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

const (
	// URLVerification URL 验证事件类型
	URLVerification = "url_verification"
	// MessageReceiveType 消息接收事件类型
	MessageReceiveType = "im.message.receive_v1"
)

// HTTPHandler 处理飞书 Webhook 的 HTTP 请求
type HTTPHandler struct {
	channel *FeishuChannel
}

// NewHTTPHandler 创建新的 HTTP 处理器
func NewHTTPHandler(channel *FeishuChannel) *HTTPHandler {
	return &HTTPHandler{
		channel: channel,
	}
}

// ServeHTTP 实现 http.Handler 接口
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 处理 GET 请求（健康检查）
	if r.Method == http.MethodGet {
		h.writeSuccess(w, "Feishu service is running")
		return
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("[Feishu] Failed to read request body", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	defer r.Body.Close()

	logger.Debug("[Feishu] Received webhook request", zap.String("body", string(body)))

	// 解析事件
	var event FeishuEvent
	if err := json.Unmarshal(body, &event); err != nil {
		logger.Error("[Feishu] Failed to parse event", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, "invalid event format")
		return
	}

	// 处理 URL 验证挑战
	if event.Type == URLVerification {
		h.handleURLVerification(w, body)
		return
	}

	// 验证令牌
	if !h.verifyToken(&event) {
		logger.Warn("[Feishu] Token verification failed")
		h.writeError(w, http.StatusUnauthorized, "token verification failed")
		return
	}

	// 处理消息事件
	if event.Header != nil && event.Header.EventType == MessageReceiveType {
		if err := h.handleMessageEvent(&event); err != nil {
			logger.Error("[Feishu] Failed to handle message event", zap.Error(err))
		}
	}

	h.writeSuccess(w, "ok")
}

// handleURLVerification 处理飞书 URL 验证挑战
func (h *HTTPHandler) handleURLVerification(w http.ResponseWriter, body []byte) {
	var challenge struct {
		Challenge string `json:"challenge"`
		Token     string `json:"token"`
		Type      string `json:"type"`
	}

	if err := json.Unmarshal(body, &challenge); err != nil {
		logger.Error("[Feishu] Failed to parse challenge", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, "invalid challenge format")
		return
	}

	logger.Info("[Feishu] URL verification received", zap.String("token", challenge.Token))

	// 响应挑战
	w.Header().Set(common.HeaderContentType, common.ContentTypeJSON)
	json.NewEncoder(w).Encode(map[string]string{
		"challenge": challenge.Challenge,
	})
}

// verifyToken 验证请求令牌
func (h *HTTPHandler) verifyToken(event *FeishuEvent) bool {
	if h.channel.config.VerificationToken == "" {
		// 未配置令牌，跳过验证
		return true
	}

	if event.Header == nil {
		return false
	}

	return event.Header.Token == h.channel.config.VerificationToken
}

// handleMessageEvent 处理接收到的消息事件
func (h *HTTPHandler) handleMessageEvent(event *FeishuEvent) error {
	if event.Event == nil || event.Event.Message == nil {
		return fmt.Errorf("invalid event: missing message data")
	}

	msg := event.Event.Message
	msgID := msg.MessageID

	// 检查重复消息（幂等性）
	if h.channel.isMessageProcessed(msgID) {
		logger.Debug("[Feishu] Duplicate message filtered", zap.String("msg_id", msgID))
		return nil
	}

	// 标记消息为已处理
	h.channel.markMessageProcessed(msgID)

	// 过滤过期消息（超过 60 秒）
	if err := h.filterStaleMessage(msg); err != nil {
		return err
	}

	// 获取访问令牌
	accessToken, err := h.channel.getAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// 解析飞书消息
	feishuMsg, err := NewFeishuMessage(event, accessToken)
	if err != nil {
		return fmt.Errorf("failed to parse feishu message: %w", err)
	}

	// 过滤没有提及的群消息
	if feishuMsg.IsGroupChat {
		if !h.shouldProcessGroupMessage(msg, feishuMsg) {
			return nil
		}
	}

	// 处理消息
	return h.channel.handleMessage(feishuMsg)
}

// filterStaleMessage 过滤过期的消息
func (h *HTTPHandler) filterStaleMessage(msg *Message) error {
	if msg.CreateTime == "" {
		return nil
	}

	// 解析创建时间
	createTime, err := parseFeishuTimestamp(msg.CreateTime)
	if err != nil {
		return nil // 解析失败则放行
	}

	// 检查消息年龄
	age := time.Since(createTime)
	if age > 60*time.Second {
		logger.Debug("[Feishu] Stale message filtered",
			zap.String("msg_id", msg.MessageID),
			zap.Duration("age", age))
		return fmt.Errorf("stale message")
	}

	return nil
}

// shouldProcessGroupMessage 判断是否应处理群消息
func (h *HTTPHandler) shouldProcessGroupMessage(msg *Message, feishuMsg *FeishuMessage) bool {
	// 群聊中的文本消息需要被 @ 提及
	if msg.MessageType == "text" && len(msg.Mentions) == 0 {
		return false
	}

	// 检查是否 @ 了机器人
	if len(msg.Mentions) > 0 {
		botOpenID := h.channel.getBotOpenID()
		if !feishuMsg.IsMentionBot(botOpenID) {
			logger.Debug("[Feishu] Bot not mentioned, ignoring",
				zap.String("msg_id", msg.MessageID))
			return false
		}
	}

	return true
}

// writeSuccess 写入成功响应
func (h *HTTPHandler) writeSuccess(w http.ResponseWriter, message string) {
	w.Header().Set(common.HeaderContentType, common.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": message,
	})
}

// writeError 写入错误响应
func (h *HTTPHandler) writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set(common.HeaderContentType, common.ContentTypeJSON)
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error":   message,
	})
}
