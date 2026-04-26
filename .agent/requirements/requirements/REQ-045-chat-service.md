---
id: REQ-045
title: "会话服务与生命周期"
status: active
level: story
priority: P1
cluster: agent-core
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-044]
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
    snapshot: "ChatService 提供对话接口，处理消息收发、会话隔离"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析 chat.go，从已实现代码中提取完整验收标准"
    reason: "扩展验收标准从4条到25条，补充代码参考映射"
    snapshot: "ChatService 对话服务：包含 Run/RunWithSession 对话入口、流式输出与事件系统、工具调用生命周期管理、对话选项配置、AgentExecutor 接口抽象"
source_code:
  - pkg/agent/chat/chat.go
---

# 会话服务与生命周期

## 描述
ChatService 提供对话接口，处理消息收发、会话隔离、历史管理。chat.go 约 539 行代码，实现了 Chat 接口定义和消息处理流程，支持多会话并发隔离，每个用户/渠道拥有独立的会话上下文，确保对话不串扰。

## 验收标准

### ChatService 核心结构
- [x] ChatService 结构体定义：包含 sessionManager(*SessionManager)、agentFactory(工厂函数)、options(*ChatOptions) 三个核心字段
- [x] NewChatService 构造函数：接收 SessionManager、agentFactory 和可变 ChatOption 参数，应用默认选项后覆盖
- [x] agentFactory 工厂函数签名：func(sessionID string) (AgentExecutor, error)，按会话 ID 创建独立的 Agent 执行器

### 对话执行入口
- [x] ChatService.Run 方法：接收 ctx、sessionID、query、sendChunk 回调和可选 ChatOption，自动获取或创建会话后执行对话
- [x] Run 方法中通过 s.sessionManager.GetOrCreateSession(sessionID) 自动恢复或创建会话
- [x] Run 方法中通过 s.agentFactory(sessionID) 创建 AgentExecutor，失败时返回包装错误
- [x] Run 方法中调用 session.AddUserMessage(query) 记录用户输入，agent.RunWithHistory 执行对话
- [x] Run 方法中对话成功后调用 session.AddAssistantMessage(response) 记录助手响应
- [x] Run 方法结束时发送 ChunkTypeComplete 完成块，并记录会话 ID 和消息数日志
- [x] Run 方法中对话失败时发送 ChunkTypeError 错误块并返回错误

- [x] ChatService.RunWithSession 方法：接收现有 Session 对象执行对话，跳过会话获取/创建步骤
- [x] RunWithSession 方法使用 session.ID 创建 AgentExecutor，复用已有会话上下文
- [x] RunWithSession 方法中对话流程与 Run 一致：添加用户消息→执行→添加助手消息→发送完成块

### 流式输出 Chunk 类型系统
- [x] ChunkType 枚举定义五种类型：ChunkTypeContent("content")、ChunkTypeToolStart("tool_start")、ChunkTypeToolCalls("tool_calls")、ChunkTypeError("error")、ChunkTypeComplete("complete")
- [x] Chunk 结构体包含：Type(ChunkType)、Delta(string)、SegmentID(int)、Tool(string)、Arguments(map[string]any)、ToolCalls([]ToolCallInfo)、Error(string)
- [x] ToolCallInfo 结构体记录工具调用详情：Name、Arguments(map[string]any)、Result(string)、Status(string)、Elapsed(string)

### 事件类型与分发
- [x] EventType 枚举定义八种事件：EventText、EventToolCall、EventToolResult、EventError、EventStepStart、EventStepEnd、EventComplete、EventTurnEnd
- [x] ChatService.handleEvent 方法：从 event map 中提取 type 字符串和 data map，按 EventType 分发处理
- [x] EventText 处理：提取 delta 字符串，发送 ChunkTypeContent 块，携带 Delta 和 SegmentID
- [x] EventToolCall 处理：提取 tool_name、arguments、tool_call_id，缓存参数到 state.pendingToolArguments，发送 ChunkTypeToolStart 块
- [x] EventToolCall 中 tool_call_id 为空时使用 tool_name 作为 fallback ID
- [x] EventToolResult 处理：从 pendingToolArguments 取缓存参数，调用 formatResult 格式化结果，组装 ToolCallInfo 追加到 pendingToolResults
- [x] EventToolResult 中提取 execution_time 并格式化为 "X.XXs" 格式的 Elapsed 字段
- [x] EventTurnEnd 处理：当 has_tool_calls 为 true 且 pendingToolResults 非空时，批量发送 ChunkTypeToolCalls 块，重置 pendingToolResults 并递增 segmentID
- [x] EventComplete 处理：发送 ChunkTypeComplete 完成块

### 流式状态管理
- [x] streamState 结构体：维护 segmentID(int)、pendingToolResults([]ToolCallInfo)、pendingToolArguments(map[string]map[string]any)
- [x] newStreamState 构造函数：初始化 segmentID=0、空 pendingToolResults 列表、空 pendingToolArguments map
- [x] segmentID 机制：每个工具调用轮次后递增，用于区分不同段落的文本流

### 结果格式化
- [x] ChatService.formatResult 方法：支持 string 直接返回、[]byte 转字符串返回、其他类型 fmt.Sprintf("%v") 返回
- [x] formatResult 处理 nil 输入时返回空字符串

### ChatOptions 对话选项配置
- [x] ChatOptions 结构体：MaxContextTurns(int)、Temperature(float64)、MaxTokens(int)、Stream(bool)
- [x] DefaultChatOptions 默认值：MaxContextTurns=20、Temperature=0.7、MaxTokens=2048、Stream=true
- [x] ChatOption 函数选项模式：WithMaxContextTurns、WithTemperature、WithMaxTokens、WithStream 四个选项函数
- [x] Run/RunWithSession 方法中先拷贝默认选项再应用调用时传入的选项，支持每次对话独立配置

### AgentExecutor 接口抽象
- [x] AgentExecutor 接口定义两个方法：Run(ctx, query, onEvent) 和 RunWithHistory(ctx, messages, onEvent)
- [x] ChatService 依赖 AgentExecutor 接口而非具体实现，实现对话逻辑与 Agent 执行逻辑解耦
- [x] onEvent 回调签名：func(event map[string]any)，Agent 通过此回调上报事件给 ChatService

### ChatResult 对话结果
- [x] ChatResult 结构体：Response(string)、MessageCount(int)、TokenUsage(*llm.Usage)、Duration(time.Duration)、ToolCalls([]ToolCallInfo)
- [x] ChatResult 记录完整的对话执行信息，包含响应内容、消息数量、Token 使用、耗时和工具调用记录

## 代码参考

| 验收标准 | 代码位置 | 说明 |
|---------|---------|------|
| ChatService 结构体 | chat.go:147-154 | sessionManager + agentFactory + options |
| NewChatService 构造函数 | chat.go:157-168 | 函数选项模式创建实例 |
| ChatService.Run | chat.go:171-217 | 主对话入口，含会话获取、Agent创建、消息记录 |
| ChatService.RunWithSession | chat.go:220-257 | 使用现有 Session 执行对话 |
| ChatService.handleEvent | chat.go:260-340 | 事件分发：Text/ToolCall/ToolResult/TurnEnd/Complete |
| ChunkType 枚举 | chat.go:16-29 | Content/ToolStart/ToolCalls/Error/Complete |
| EventType 枚举 | chat.go:34-51 | 八种事件类型定义 |
| Chunk 结构体 | chat.go:54-69 | 流式数据块，含 Delta/SegmentID/Tool/Arguments 等 |
| ToolCallInfo 结构体 | chat.go:72-83 | 工具调用详情：Name/Arguments/Result/Status/Elapsed |
| streamState 状态管理 | chat.go:359-372 | segmentID + pendingToolResults + pendingToolArguments |
| formatResult 格式化 | chat.go:343-356 | string/[]byte/通用类型结果转换 |
| ChatOptions 默认值 | chat.go:98-105 | MaxContextTurns=20, Temp=0.7, MaxTokens=2048, Stream=true |
| ChatOption 选项函数 | chat.go:108-136 | WithMaxContextTurns/WithTemperature/WithMaxTokens/WithStream |
| AgentExecutor 接口 | chat.go:139-144 | Run + RunWithHistory 两个方法 |
| ChatResult 结构体 | chat.go:528-539 | Response/MessageCount/TokenUsage/Duration/ToolCalls |
