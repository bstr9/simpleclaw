// Package terminal 提供终端渠道实现，用于命令行交互。
package terminal

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	"go.uber.org/zap"
)

const (
	channelTypeTerminal = "terminal"
	userPrompt          = "User: "
	botPrompt           = "Bot: "
)

// TerminalMessage 表示来自终端的消息。
type TerminalMessage struct {
	types.BaseMessage
}

// NewTerminalMessage 创建新的终端消息。
func NewTerminalMessage(msgID, content string) *TerminalMessage {
	msg := &TerminalMessage{}
	msg.MsgID = msgID
	msg.Content = content
	msg.CreateTime = time.Now()
	msg.FromUserID = "User"
	msg.ToUserID = "Chatgpt"
	msg.MsgType = int(types.ContextText)
	msg.Context = types.NewContext(types.ContextText, content)
	return msg
}

// TerminalChannel 实现终端交互的 Channel 接口。
// 从 stdin 读取用户输入，将机器人响应打印到 stdout。
type TerminalChannel struct {
	*channel.BaseChannel

	// reader 用于读取用户输入
	reader *bufio.Reader

	// ctx 用于取消的上下文
	ctx context.Context

	// cancel 是取消函数
	cancel context.CancelFunc

	// mu 保护运行状态
	mu sync.RWMutex

	// running 指示渠道是否活跃
	running bool

	// wg 等待输入循环结束
	wg sync.WaitGroup

	// msgIDCounter 是消息 ID 计数器
	msgIDCounter int64
	msgIDMu      sync.Mutex

	// messageHandler 处理传入消息
	messageHandler MessageHandler
}

// MessageHandler 处理传入的终端消息。
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg types.ChatMessage) (*types.Reply, error)
}

// NewTerminalChannel 创建新的终端渠道实例。
func NewTerminalChannel() *TerminalChannel {
	return &TerminalChannel{
		BaseChannel: channel.NewBaseChannel(channelTypeTerminal),
		reader:      bufio.NewReader(os.Stdin),
	}
}

// SetMessageHandler 设置用于处理用户输入的消息处理器。
func (t *TerminalChannel) SetMessageHandler(handler any) {
	if h, ok := handler.(MessageHandler); ok {
		t.messageHandler = h
	}
}

// Startup 启动终端输入循环。
// 从 stdin 读取用户输入并处理每条消息。
func (t *TerminalChannel) Startup(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)

	t.mu.Lock()
	t.running = true
	t.mu.Unlock()

	// 设置支持的回复类型（不支持语音）
	t.SetNotSupportTypes([]types.ReplyType{types.ReplyVoice})

	// 打印欢迎消息
	t.printWelcome()

	// 在协程中启动输入循环
	t.wg.Add(1)
	go t.inputLoop()

	t.ReportStartupSuccess()
	logger.Info("[TerminalChannel] Terminal channel started")

	return nil
}

// Stop 停止终端渠道。
func (t *TerminalChannel) Stop() error {
	t.mu.Lock()
	t.running = false
	t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
	}

	t.wg.Wait()
	t.SetStarted(false)

	logger.Info("[TerminalChannel] Terminal channel stopped")
	return nil
}

// Send 发送回复到终端输出。
// 将机器人响应打印到 stdout。
func (t *TerminalChannel) Send(reply *types.Reply, ctx *types.Context) error {
	if reply == nil {
		return nil
	}

	// 检查回复类型是否支持
	if !t.IsReplyTypeSupported(reply.Type) {
		logger.Warn("[TerminalChannel] Reply type not supported",
			zap.String("type", reply.Type.String()))
		return nil
	}

	// 打印机器人响应
	fmt.Println()
	fmt.Print(botPrompt)

	switch reply.Type {
	case types.ReplyImage:
		// 图片类型，打印占位符
		fmt.Println("<IMAGE>")
		if str, ok := reply.Content.(string); ok {
			fmt.Println(str)
		}
	case types.ReplyImageURL:
		// 图片 URL 类型，打印 URL
		fmt.Println("<IMAGE_URL>")
		if str, ok := reply.Content.(string); ok {
			fmt.Println(str)
		}
	case types.ReplyError:
		fmt.Printf("[ERROR] %v\n", reply.Content)
	default:
		// 文本和其他类型
		fmt.Println(reply.StringContent())
	}

	// 打印用户提示符等待下一次输入
	fmt.Println()
	fmt.Print(userPrompt)

	return nil
}

// inputLoop 运行主要的输入读取循环。
func (t *TerminalChannel) inputLoop() {
	defer t.wg.Done()

	for {
		if !t.isRunning() {
			return
		}

		if shouldExit := t.processInputCycle(); shouldExit {
			return
		}
	}
}

// isRunning 检查渠道是否仍在运行
func (t *TerminalChannel) isRunning() bool {
	t.mu.RLock()
	running := t.running
	t.mu.RUnlock()
	return running
}

// processInputCycle 处理一次输入循环
func (t *TerminalChannel) processInputCycle() bool {
	input, err := t.readInput()
	if err != nil {
		return t.handleInputError(err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}

	if t.isExitCommand(input) {
		t.handleExit()
		return true
	}

	t.processMessage(input)
	return false
}

// handleInputError 处理输入错误
func (t *TerminalChannel) handleInputError(err error) bool {
	select {
	case <-t.ctx.Done():
		return true
	default:
		if err.Error() == "EOF" {
			time.Sleep(100 * time.Millisecond)
			fmt.Println("\nExiting...")
			return true
		}
		logger.Error("[TerminalChannel] Error reading input", zap.Error(err))
		return false
	}
}

// handleExit 处理退出命令
func (t *TerminalChannel) handleExit() {
	fmt.Println("Goodbye!")
	t.mu.Lock()
	t.running = false
	t.mu.Unlock()
}

// processMessage 处理用户消息
func (t *TerminalChannel) processMessage(input string) {
	msg := NewTerminalMessage(t.generateMsgID(), input)

	if t.messageHandler == nil {
		logger.Warn("[TerminalChannel] No message handler set")
		return
	}

	logger.Info("[TerminalChannel] Processing message", zap.String("content", input))
	reply, err := t.messageHandler.HandleMessage(t.ctx, msg)
	if err != nil {
		logger.Error("[TerminalChannel] Error handling message", zap.Error(err))
		t.Send(types.NewErrorReply(err.Error()), nil)
		return
	}
	if reply != nil {
		t.Send(reply, msg.GetContext())
	}
}

// readInput 从 stdin 读取一行。
// 单行输入模式（匹配原始 Python 实现）。
func (t *TerminalChannel) readInput() (string, error) {
	line, err := t.reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	// 移除尾随换行符
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")

	return line, nil
}

// isExitCommand 检查输入是否为退出命令。
func (t *TerminalChannel) isExitCommand(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	return lower == "quit" || lower == "exit" || lower == "q"
}

// generateMsgID 生成唯一的消息 ID。
func (t *TerminalChannel) generateMsgID() string {
	t.msgIDMu.Lock()
	defer t.msgIDMu.Unlock()
	t.msgIDCounter++
	return fmt.Sprintf("%d%d", time.Now().Unix(), t.msgIDCounter)
}

// printWelcome 打印欢迎消息。
func (t *TerminalChannel) printWelcome() {
	fmt.Println()
	fmt.Println("============================================")
	fmt.Println("  Terminal Channel - Interactive Mode")
	fmt.Println("============================================")
	fmt.Println()
	fmt.Println("Type your message and press Enter to send.")
	fmt.Println("Type 'quit', 'exit', or 'q' to exit.")
	fmt.Println()
	fmt.Print(userPrompt)
}
