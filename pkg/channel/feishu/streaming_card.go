// feishu 包提供飞书渠道实现。
// streaming_card.go 实现流式消息卡片功能。
package feishu

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

const (
	streamingElementID = "streaming_content"
	throttleMs         = 500
	batchAfterGapMs    = 100
	longGapThresholdMs = 2000
)

// StreamingCardPhase 流式卡片状态
type StreamingCardPhase string

const (
	PhaseIdle      StreamingCardPhase = "idle"
	PhaseCreating  StreamingCardPhase = "creating"
	PhaseStreaming StreamingCardPhase = "streaming"
	PhaseCompleted StreamingCardPhase = "completed"
)

// StreamingCardController 流式卡片控制器
type StreamingCardController struct {
	mu sync.Mutex

	phase       StreamingCardPhase
	accumulated string
	cardMsgID   string

	flushInProgress bool
	needsReflush    bool
	lastUpdateTime  int64
	pendingTimer    *time.Timer

	channel *FeishuChannel
	msg     *FeishuMessage
	ctx     map[string]any
}

// NewStreamingCardController 创建流式卡片控制器
func NewStreamingCardController(channel *FeishuChannel, msg *FeishuMessage, ctx map[string]any) *StreamingCardController {
	return &StreamingCardController{
		phase:   PhaseIdle,
		channel: channel,
		msg:     msg,
		ctx:     ctx,
	}
}

// AppendText 追加文本并触发节流更新
func (c *StreamingCardController) AppendText(text string) {
	c.mu.Lock()
	c.accumulated += text
	shouldFlush := c.shouldFlushLocked()
	c.mu.Unlock()

	if shouldFlush {
		go c.flush()
	}
}

// UpdateText 更新完整文本并触发节流更新
func (c *StreamingCardController) UpdateText(text string) {
	c.mu.Lock()
	c.accumulated = text
	shouldFlush := c.shouldFlushLocked()
	c.mu.Unlock()

	if shouldFlush {
		go c.flush()
	}
}

// Complete 完成流式输出，发送最终卡片
func (c *StreamingCardController) Complete() {
	c.mu.Lock()
	c.phase = PhaseCompleted
	c.cancelPendingTimerLocked()
	hasContent := c.cardMsgID != "" && c.accumulated != ""
	c.mu.Unlock()

	if hasContent {
		c.flush()
	}
}

// HasContent 检查是否有累积的内容
func (c *StreamingCardController) HasContent() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cardMsgID != "" && c.accumulated != ""
}

// shouldFlushLocked 判断是否应该刷新（调用前已持有锁）
func (c *StreamingCardController) shouldFlushLocked() bool {
	if c.phase == PhaseCompleted {
		return false
	}

	now := time.Now().UnixMilli()
	elapsed := now - c.lastUpdateTime

	if elapsed >= throttleMs {
		c.cancelPendingTimerLocked()
		if elapsed > longGapThresholdMs {
			c.lastUpdateTime = now
			c.scheduleFlush(batchAfterGapMs)
			return false
		}
		return true
	}

	if c.pendingTimer == nil {
		delay := throttleMs - elapsed
		c.scheduleFlush(delay)
	}
	return false
}

// scheduleFlush 调度延迟刷新
func (c *StreamingCardController) scheduleFlush(delay int64) {
	c.pendingTimer = time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
		c.mu.Lock()
		c.pendingTimer = nil
		c.mu.Unlock()
		c.flush()
	})
}

// cancelPendingTimerLocked 取消待执行的定时器
func (c *StreamingCardController) cancelPendingTimerLocked() {
	if c.pendingTimer != nil {
		c.pendingTimer.Stop()
		c.pendingTimer = nil
	}
}

// flush 执行实际的卡片更新
func (c *StreamingCardController) flush() {
	c.mu.Lock()
	if c.flushInProgress {
		c.needsReflush = true
		c.mu.Unlock()
		return
	}

	if c.cardMsgID == "" && c.phase == PhaseIdle {
		c.phase = PhaseCreating
	}
	c.flushInProgress = true
	c.needsReflush = false
	text := c.accumulated
	msgID := c.cardMsgID
	phase := c.phase
	c.lastUpdateTime = time.Now().UnixMilli()
	c.mu.Unlock()

	var err error
	if msgID == "" {
		msgID, err = c.channel.SendStreamCard(c.msg, text, c.ctx)
		if err != nil {
			logger.Error("[Feishu] 发送流式卡片失败", zap.Error(err))
			c.mu.Lock()
			c.flushInProgress = false
			c.mu.Unlock()
			return
		}
		c.mu.Lock()
		c.cardMsgID = msgID
		c.phase = PhaseStreaming
		c.mu.Unlock()
	} else {
		if err := c.channel.UpdateStreamCard(msgID, text, phase == PhaseCompleted); err != nil {
			logger.Warn("[Feishu] 更新流式卡片失败", zap.Error(err))
		}
	}

	c.mu.Lock()
	c.flushInProgress = false
	needsReflush := c.needsReflush
	c.mu.Unlock()

	if needsReflush {
		go c.flush()
	}
}

// buildStreamingCard 构建流式卡片 JSON
func buildStreamingCard(text string, isComplete bool) string {
	var card map[string]any

	if isComplete {
		card = map[string]any{
			"schema": "2.0",
			"config": map[string]any{
				"wide_screen_mode": true,
			},
			"body": map[string]any{
				"elements": []map[string]any{
					{
						"tag":     "markdown",
						"content": text,
					},
				},
			},
		}
	} else {
		card = map[string]any{
			"schema": "2.0",
			"config": map[string]any{
				"wide_screen_mode": true,
				"streaming_mode":   true,
				"summary": map[string]any{
					"content": "思考中...",
				},
			},
			"body": map[string]any{
				"elements": []map[string]any{
					{
						"tag":        "markdown",
						"content":    text,
						"element_id": streamingElementID,
					},
					{
						"tag":     "markdown",
						"content": " ",
						"icon": map[string]any{
							"tag":   "standard_icon",
							"token": "loading_blue",
							"size":  "small",
						},
						"element_id": "loading_icon",
					},
				},
			},
		}
	}

	data, _ := json.Marshal(card)
	return string(data)
}
