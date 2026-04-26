---
id: REQ-046
title: "上下文管理与对话历史"
status: active
level: story
priority: P2
cluster: agent-core
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-044]
  merged_from: []
  depends_on: [REQ-045]
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "会话上下文管理，包括对话历史的存储、检索和过期清理"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析 session.go，从已实现代码中提取完整验收标准"
    reason: "扩展验收标准从4条到25条，补充代码参考映射"
    snapshot: "上下文管理与对话历史：包含 Session 生命周期管理、SessionManager 会话管理器、消息增删查改、会话过期清理、持久化存储、会话 ID 生成"
source_code:
  - pkg/agent/chat/session.go
---

# 上下文管理与对话历史

## 描述
会话上下文管理，包括对话历史的存储、检索和过期清理。session.go 约 540 行代码，实现了 Session 结构体及生命周期管理、上下文窗口控制、会话过期与清理机制。支持按时间或消息数量管理上下文窗口，自动清理过期会话释放资源。

## 验收标准

### Session 核心结构
- [x] Session 结构体定义：ID、UserID、ChannelType、Status(SessionStatus)、Messages([]llm.Message)、Metadata(map[string]any)、CreatedAt、UpdatedAt、LastActiveAt、mu(sync.RWMutex) 共十个字段
- [x] NewSession 构造函数：初始化 Status=StatusActive、空 Messages 切片、空 Metadata map、三个时间字段设为当前时间
- [x] SessionOption 函数选项模式：支持 WithUserID、WithChannelType、WithMetadata 三个选项函数

### 会话状态管理
- [x] SessionStatus 三态枚举：StatusActive("active")、StatusIdle("idle")、StatusClosed("closed")
- [x] Session.IsActive 方法：检查 Status 是否为 StatusActive
- [x] Session.Close 方法：将 Status 设为 StatusClosed，更新 UpdatedAt
- [x] Session.SetIdle 方法：将 Status 设为 StatusIdle，更新 UpdatedAt
- [x] Session.Activate 方法：将 Status 设为 StatusActive，更新 LastActiveAt 和 UpdatedAt

### 消息添加
- [x] Session.AddMessage：加写锁追加消息到 Messages 切片，更新 UpdatedAt 和 LastActiveAt
- [x] Session.AddUserMessage：调用 AddMessage 添加 Role=RoleUser 的消息
- [x] Session.AddAssistantMessage：调用 AddMessage 添加 Role=RoleAssistant 的消息
- [x] Session.AddToolCallMessage：添加包含 ToolCalls 的助手消息
- [x] Session.AddToolResultMessage：添加 Role=RoleTool 的消息，携带 ToolCallID 和 Content

### 消息查询与操作
- [x] Session.GetMessages：加读锁返回消息的完整副本（copy 切片），确保线程安全
- [x] Session.GetMessagesWithSystem：在消息列表前插入系统提示词（Role=RoleSystem），返回新列表
- [x] Session.ClearMessages：加写锁清空 Messages 切片，更新 UpdatedAt
- [x] Session.TrimMessages(n)：加写锁保留最近 n 条消息，裁剪旧的，更新 UpdatedAt
- [x] Session.GetMessageCount：加读锁返回消息数量

### 会话序列化
- [x] Session.ToJSON：使用 json.Marshal 序列化整个 Session 结构体为 JSON 字符串
- [x] 所有字段均带 json tag，支持 JSON 序列化/反序列化

### SessionManager 核心结构
- [x] SessionManager 结构体：sessions(map[string]*Session)、mu(sync.RWMutex)、store(SessionStore)、maxSessions(int)、sessionTimeout(time.Duration)、cleanupInterval(time.Duration)、stopCleanup(chan struct{})
- [x] NewSessionManager 构造函数：默认 maxSessions=1000、sessionTimeout=30min、cleanupInterval=5min，启动后台清理协程
- [x] SessionManagerOption 函数选项：WithSessionStore、WithMaxSessions、WithSessionTimeout、WithCleanupInterval

### 会话创建与获取
- [x] SessionManager.CreateSession：加写锁检查 ID 是否已存在，检查最大会话数限制，超出时尝试清理过期会话后重试
- [x] CreateSession 创建成功后若 store 存在则调用 store.Save 持久化，持久化失败仅警告不回滚
- [x] SessionManager.GetSession：先从内存 map 查找，未命中时若 store 存在则从 store.Load 加载并缓存到内存
- [x] SessionManager.GetOrCreateSession：加写锁，内存有则 Activate 返回，store 有则 Load+Activate+缓存，否则新建+持久化

### 会话删除与列表
- [x] SessionManager.DeleteSession：加写锁，先 Close 会话，再从 map 删除，再从 store 删除
- [x] DeleteSession 中 store 删除失败仅记录警告，不回滚内存删除
- [x] SessionManager.ListSessions：加读锁，返回所有会话的切片
- [x] SessionManager.ListActiveSessions：加读锁，过滤仅返回 IsActive() 为 true 的会话
- [x] SessionManager.SessionCount：加读锁，返回会话 map 长度

### 会话持久化
- [x] SessionManager.SaveSession：若 store 为 nil 直接返回 nil，否则加读锁获取 Session 后调用 store.Save
- [x] SessionStore 接口定义四个方法：Save(session *Session) error、Load(id string) (*Session, error)、Delete(id string) error、List() ([]*Session, error)
- [x] SessionManager.Close 方法：关闭 stopCleanup channel 停止清理协程，加写锁保存所有活跃会话到 store，清空 sessions map

### 会话过期清理
- [x] cleanupLoop 后台协程：按 cleanupInterval 周期触发 cleanupExpiredSessions，收到 stopCleanup 信号时退出
- [x] cleanupExpiredSessions：加写锁调用 cleanupExpiredSessionsLocked
- [x] cleanupExpiredSessionsLocked：遍历所有会话，now.Sub(session.LastActiveAt) > sessionTimeout 时 Close 并删除
- [x] CreateSession 中达到 maxSessions 上限时调用 cleanupExpiredSessionsLocked 尝试腾出空间

### 会话 ID 生成
- [x] GenerateSessionID(prefix)：基于 SHA256(time.Now().String() + prefix) 生成哈希，取前 16 个十六进制字符，格式为 prefix_hash

## 代码参考

| 验收标准 | 代码位置 | 说明 |
|---------|---------|------|
| Session 结构体 | session.go:33-54 | 含 ID/UserID/ChannelType/Status/Messages/Metadata/时间/mu |
| NewSession 构造函数 | session.go:57-74 | 默认 Active 状态，空切片/map |
| SessionOption 选项 | session.go:77-98 | WithUserID/WithChannelType/WithMetadata |
| SessionStatus 枚举 | session.go:19-30 | Active/Idle/Closed 三态 |
| Session 状态方法 | session.go:201-223 | IsActive/Close/SetIdle/Activate |
| AddMessage | session.go:101-108 | 加写锁追加消息+更新时间 |
| AddUserMessage | session.go:111-116 | Role=RoleUser 快捷方法 |
| AddAssistantMessage | session.go:119-124 | Role=RoleAssistant 快捷方法 |
| AddToolCallMessage | session.go:127-132 | 带 ToolCalls 的助手消息 |
| AddToolResultMessage | session.go:135-141 | Role=RoleTool + ToolCallID |
| GetMessages | session.go:144-151 | 返回线程安全副本 |
| GetMessagesWithSystem | session.go:154-169 | 插入系统提示词 |
| ClearMessages | session.go:172-178 | 清空消息+更新时间 |
| TrimMessages | session.go:181-191 | 保留最近 n 条 |
| GetMessageCount | session.go:194-199 | 返回消息数量 |
| ToJSON | session.go:226-232 | JSON 序列化 |
| SessionManager 结构体 | session.go:235-250 | sessions map + store + 配置 |
| NewSessionManager | session.go:284-301 | 默认值+启动清理协程 |
| SessionManagerOption | session.go:253-281 | Store/MaxSessions/Timeout/CleanupInterval |
| CreateSession | session.go:304-334 | 检查存在/上限/清理/持久化 |
| GetSession | session.go:337-358 | 内存→store 逐级查找 |
| GetOrCreateSession | session.go:361-392 | 获取/加载/创建三路合并 |
| DeleteSession | session.go:395-416 | Close+内存删除+store删除 |
| ListSessions | session.go:419-428 | 返回全部会话 |
| ListActiveSessions | session.go:431-442 | 过滤活跃会话 |
| SessionCount | session.go:445-450 | 会话数量 |
| SaveSession | session.go:453-467 | 手动持久化到 store |
| cleanupLoop | session.go:470-482 | 后台定时清理 |
| cleanupExpiredSessions | session.go:485-489 | 加锁委托 Locked 版本 |
| cleanupExpiredSessionsLocked | session.go:492-501 | 基于 LastActiveAt+Timeout 判断 |
| SessionManager.Close | session.go:504-522 | 停清理+保存全部+清空 |
| SessionStore 接口 | session.go:525-534 | Save/Load/Delete/List |
| GenerateSessionID | session.go:537-540 | SHA256+前缀+16字符哈希 |
