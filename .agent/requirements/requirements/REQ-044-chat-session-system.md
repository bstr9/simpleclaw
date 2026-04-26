---
id: REQ-044
title: "Chat/Session 会话系统"
status: active
level: epic
priority: P1
cluster: agent-core
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: []
  merged_from: []
  depends_on: [REQ-001]
  refined_by: [REQ-045, REQ-046]
  related_to: [REQ-008]
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "Agent 会话服务层，管理 Chat 会话生命周期和上下文"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析，从已实现代码中提取完整验收标准"
    reason: "扩展验收标准从5条到25条，补充代码参考映射"
    snapshot: "Chat/Session 会话系统：包含 ChatService 对话服务、SessionManager 会话管理、上下文管理、对话历史管理、流式输出、会话持久化、并发安全等完整功能"
source_code:
  - pkg/agent/chat/chat.go
  - pkg/agent/chat/session.go
---

# Chat/Session 会话系统

## 描述
Agent 会话服务层，管理 Chat 会话生命周期和上下文。SessionManager 管理会话创建/恢复/清理，ChatService 提供对话接口。两个核心文件共约 1080 行代码，实现了完整的会话管理功能，包括会话隔离、上下文窗口管理、历史消息存储和并发安全保护。

## 验收标准

### ChatService 对话服务
- [x] ChatService 结构体定义，包含 sessionManager、agentFactory、options 三个核心字段
- [x] NewChatService 构造函数支持函数选项模式（ChatOption）配置
- [x] ChatService.Run 方法：按 sessionID 自动获取或创建会话，执行对话并返回结果
- [x] ChatService.RunWithSession 方法：使用现有 Session 对象执行对话
- [x] ChatService.handleEvent 事件分发：根据 EventType 将 Agent 事件转换为 Chunk 输出
- [x] ChatService.formatResult 结果格式化：支持 string、[]byte 和通用类型的结果转换

### 流式输出与事件系统
- [x] ChunkType 枚举定义：Content、ToolStart、ToolCalls、Error、Complete 五种块类型
- [x] EventType 枚举定义：Text、ToolCall、ToolResult、Error、StepStart、StepEnd、Complete、TurnEnd 八种事件类型
- [x] Chunk 结构体：包含 Type、Delta、SegmentID、Tool、Arguments、ToolCalls、Error 字段
- [x] ToolCallInfo 结构体：记录工具调用的 Name、Arguments、Result、Status、Elapsed 信息
- [x] streamState 流式状态管理：维护 segmentID、pendingToolResults、pendingToolArguments
- [x] 工具调用事件处理：EventToolCall 缓存参数到 pendingToolArguments，发送 ToolStart 块
- [x] 工具结果事件处理：EventToolResult 从缓存取参数，组装 ToolCallInfo，追加到 pendingToolResults
- [x] 轮次结束处理：EventTurnEnd 批量发送待处理的工具调用结果，递增 segmentID

### ChatOptions 对话选项
- [x] ChatOptions 结构体：MaxContextTurns(默认20)、Temperature(默认0.7)、MaxTokens(默认2048)、Stream(默认true)
- [x] DefaultChatOptions 函数提供默认值
- [x] WithMaxContextTurns、WithTemperature、WithMaxTokens、WithStream 四个函数选项

### AgentExecutor 接口
- [x] AgentExecutor 接口定义：Run(ctx, query, onEvent) 和 RunWithHistory(ctx, messages, onEvent) 两个方法
- [x] ChatService 通过 agentFactory 工厂函数按 sessionID 创建 AgentExecutor 实例

### 会话生命周期管理
- [x] SessionStatus 三态枚举：StatusActive、StatusIdle、StatusClosed
- [x] Session 结构体：ID、UserID、ChannelType、Status、Messages、Metadata、CreatedAt/UpdatedAt/LastActiveAt、mu 读写锁
- [x] NewSession 构造函数支持 SessionOption 函数选项模式
- [x] WithUserID、WithChannelType、WithMetadata 三个会话选项函数

### 消息管理
- [x] Session.AddMessage：线程安全地追加消息并更新时间戳
- [x] Session.AddUserMessage / AddAssistantMessage：快捷添加用户/助手消息
- [x] Session.AddToolCallMessage / AddToolResultMessage：添加工具调用和工具结果消息
- [x] Session.GetMessages：返回线程安全的消息副本
- [x] Session.GetMessagesWithSystem：获取包含系统提示词的消息列表
- [x] Session.ClearMessages / TrimMessages：清空或裁剪消息历史
- [x] Session.GetMessageCount：获取消息数量

### 会话状态转换
- [x] Session.IsActive：检查会话是否活跃
- [x] Session.Close：关闭会话（StatusClosed）
- [x] Session.SetIdle / Activate：空闲与激活状态切换，更新 LastActiveAt

### 会话管理器
- [x] SessionManager：管理 sessions map、store、maxSessions(默认1000)、sessionTimeout(默认30min)、cleanupInterval(默认5min)
- [x] SessionManagerOption 函数选项：WithSessionStore、WithMaxSessions、WithSessionTimeout、WithCleanupInterval
- [x] CreateSession：创建新会话，检查最大数量限制，尝试清理过期会话后重试
- [x] GetSession：优先从内存查找，未命中则从 store 加载并缓存
- [x] GetOrCreateSession：获取已有会话或创建新会话，支持从 store 恢复
- [x] DeleteSession：关闭并删除会话，同步删除 store 中的数据
- [x] ListSessions / ListActiveSessions：列出全部/活跃会话
- [x] SessionCount / SaveSession：会话计数与手动持久化

### 会话过期清理
- [x] cleanupLoop 后台定时清理：按 cleanupInterval 周期触发
- [x] cleanupExpiredSessions：基于 LastActiveAt 和 sessionTimeout 判断过期
- [x] cleanupExpiredSessionsLocked：持锁版本，供 CreateSession 达到上限时复用
- [x] SessionManager.Close：停止清理协程，保存所有活跃会话到 store，清空内存

### 会话持久化
- [x] SessionStore 接口：Save、Load、Delete、List 四个方法
- [x] Session.ToJSON：序列化会话为 JSON
- [x] GenerateSessionID：基于 SHA256 + 时间戳 + 前缀生成会话 ID

### 上下文管理
- [x] ContextManager：maxMessages、maxTokens、summarizer 三个核心字段
- [x] ContextSummarizer 接口：Summarize(ctx, messages) 方法
- [x] TrimMessages：按消息数量裁剪，支持保留系统提示词
- [x] EstimateTokens：基于字符数/4 估算 token 数
- [x] ShouldSummarize：判断消息数或 token 数是否超限需摘要

### 对话历史管理
- [x] ConversationHistory：messages 列表 + mu 读写锁 + maxMessages 限制
- [x] Add / AddUser / AddAssistant：添加消息，超限时自动裁剪
- [x] GetAll / GetLast：获取全部或最近 N 条消息，返回线程安全副本
- [x] Clear / Count：清空历史与消息计数

### 对话结果
- [x] ChatResult 结构体：Response、MessageCount、TokenUsage、Duration、ToolCalls 五个字段

## 代码参考

| 验收标准 | 代码位置 | 说明 |
|---------|---------|------|
| ChatService 结构体 | chat.go:147-154 | 包含 sessionManager、agentFactory、options |
| NewChatService 构造函数 | chat.go:157-168 | 函数选项模式创建实例 |
| ChatService.Run | chat.go:171-217 | 主对话入口，自动获取/创建会话 |
| ChatService.RunWithSession | chat.go:220-257 | 使用现有 Session 执行对话 |
| ChatService.handleEvent | chat.go:260-340 | 事件分发与 Chunk 转换 |
| ChunkType 枚举 | chat.go:16-29 | 五种流式输出块类型 |
| EventType 枚举 | chat.go:34-51 | 八种事件类型 |
| Chunk 结构体 | chat.go:54-69 | 流式数据块定义 |
| ToolCallInfo 结构体 | chat.go:72-83 | 工具调用信息记录 |
| streamState | chat.go:359-372 | 流式输出状态管理 |
| ChatOptions 默认值 | chat.go:98-105 | MaxContextTurns=20, Temperature=0.7 等 |
| AgentExecutor 接口 | chat.go:139-144 | Run + RunWithHistory 两个方法 |
| SessionStatus 枚举 | session.go:19-30 | Active/Idle/Closed 三态 |
| Session 结构体 | session.go:33-54 | 含 ID、Messages、Metadata、mu 等 |
| NewSession 构造函数 | session.go:57-74 | 函数选项模式 |
| Session 消息方法 | session.go:101-199 | AddMessage/AddUser/GetMessages 等 |
| Session 状态方法 | session.go:201-223 | IsActive/Close/SetIdle/Activate |
| SessionManager 结构体 | session.go:235-250 | sessions map + store + 配置 |
| SessionManager 创建/获取 | session.go:304-392 | CreateSession/GetSession/GetOrCreateSession |
| SessionManager 清理 | session.go:469-501 | cleanupLoop/cleanupExpiredSessions |
| SessionStore 接口 | session.go:525-534 | Save/Load/Delete/List |
| GenerateSessionID | session.go:537-540 | SHA256 生成会话 ID |
| ContextManager | chat.go:375-442 | 上下文管理与摘要判断 |
| ConversationHistory | chat.go:445-525 | 对话历史管理 |
| ChatResult | chat.go:528-539 | 对话结果结构体 |
