---
id: REQ-050
title: "Terminal 渠道"
status: active
level: story
priority: P2
cluster: channels
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-004]
  merged_from: []
  depends_on: []
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "命令行交互渠道，支持标准输入/输出对话模式"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析，扩展验收标准至完整覆盖"
    reason: "从现有代码逆向补充详细验收标准"
    snapshot: "Terminal渠道：Channel接口实现、bufio标准输入循环、消息处理管线、多回复类型渲染、退出命令、线程安全、欢迎横幅"
source_code:
  - pkg/channel/terminal/
---

# Terminal 渠道

## 描述
命令行交互渠道，支持标准输入/输出对话模式，是最基础的交互渠道。Terminal Channel 实现 Channel 接口，通过 bufio.Reader 从标准输入读取用户消息，标准输出打印 Agent 回复，支持文本、图片、错误等多种回复类型的格式化输出，支持 quit/exit/q 退出命令和优雅关闭。作为开发调试和快速体验的首选渠道。

## 验收标准

### 常量与类型定义
- [x] channelTypeTerminal="terminal" 渠道类型常量
- [x] userPrompt="User: " 用户输入提示符
- [x] botPrompt="Bot: " 机器人输出提示符
- [x] TerminalMessage 结构体嵌入 types.BaseMessage
- [x] NewTerminalMessage(msgID, content) 创建消息，FromUserID="User", ToUserID="Chatgpt", MsgType=ContextText

### 核心结构体
- [x] TerminalChannel 结构体嵌入 *channel.BaseChannel
- [x] TerminalChannel 持有 reader(*bufio.Reader) 从标准输入读取
- [x] TerminalChannel 持有 ctx/cancel 控制生命周期
- [x] TerminalChannel 持有 mu(sync.Mutex)/running(bool) 线程安全控制
- [x] TerminalChannel 持有 wg(sync.WaitGroup) 等待 goroutine 退出
- [x] TerminalChannel 持有 msgIDCounter(int64) 消息ID计数器
- [x] TerminalChannel 持有 messageHandler(MessageHandler) 消息处理器

### MessageHandler 接口
- [x] MessageHandler 接口定义 HandleMessage(ctx, msg) (*types.Reply, error)
- [x] SetMessageHandler(handler) 通过类型断言设置消息处理器

### 启动流程
- [x] NewTerminalChannel() 构造函数，创建 bufio.NewReader(os.Stdin)
- [x] Startup(ctx) 启动渠道：设置 running=true，禁用语音回复类型
- [x] Startup(ctx) 打印欢迎横幅
- [x] Startup(ctx) 启动 inputLoop goroutine，报告启动成功

### 输入循环
- [x] inputLoop() goroutine 主循环，持续读取用户输入
- [x] isRunning() 线程安全检查运行状态
- [x] processInputCycle() 读取输入、去除空白、检查退出、处理消息
- [x] readInput() 从 stdin 读取一行，去除 \n 和 \r
- [x] isExitCommand(input) 检查退出命令：quit/exit/q（不区分大小写）

### 消息处理
- [x] processMessage(input) 创建 TerminalMessage，调用 handler.HandleMessage，发送回复
- [x] generateMsgID() 生成消息ID：时间戳 + 计数器，互斥锁保护

### 回复输出
- [x] Send(reply, ctx) 检查支持的回复类型
- [x] ReplyImage 类型输出 `<IMAGE>` 标记
- [x] ReplyImageURL 类型输出 `<IMAGE_URL>` 标记
- [x] ReplyError 类型输出 `[ERROR]` 前缀
- [x] 默认文本类型直接输出内容

### 退出与关闭
- [x] handleExit() 打印 "Goodbye!" 并设置 running=false
- [x] handleInputError(err) 处理 EOF 和上下文取消
- [x] Stop() 设置 running=false，cancel 上下文，WaitGroup 等待 goroutine 退出

### 欢迎信息
- [x] printWelcome() 打印横幅和使用说明

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| 渠道类型常量/提示符 | `pkg/channel/terminal/terminal_channel.go` 常量定义区 |
| TerminalMessage 结构体 | `pkg/channel/terminal/terminal_channel.go` TerminalMessage/NewTerminalMessage |
| TerminalChannel 结构体 | `pkg/channel/terminal/terminal_channel.go` TerminalChannel 结构体定义 |
| MessageHandler 接口 | `pkg/channel/terminal/terminal_channel.go` MessageHandler/SetMessageHandler |
| 构造函数 | `pkg/channel/terminal/terminal_channel.go` NewTerminalChannel |
| 启动流程 | `pkg/channel/terminal/terminal_channel.go` Startup |
| 输入循环 | `pkg/channel/terminal/terminal_channel.go` inputLoop/isRunning/processInputCycle |
| 输入读取 | `pkg/channel/terminal/terminal_channel.go` readInput |
| 退出命令检测 | `pkg/channel/terminal/terminal_channel.go` isExitCommand |
| 消息处理 | `pkg/channel/terminal/terminal_channel.go` processMessage/generateMsgID |
| 回复输出 | `pkg/channel/terminal/terminal_channel.go` Send |
| 退出与关闭 | `pkg/channel/terminal/terminal_channel.go` handleExit/handleInputError/Stop |
| 欢迎信息 | `pkg/channel/terminal/terminal_channel.go` printWelcome |
