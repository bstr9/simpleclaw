---
id: REQ-047
title: "REST API Server (pkg/api/)"
status: active
level: story
priority: P1
cluster: bridge
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-008]
  merged_from: []
  depends_on: [REQ-001]
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "独立的 RESTful API Server，提供聊天、流式输出、会话管理等 API 端点"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析，从 pkg/api/server.go 提取完整验收标准"
    reason: "细化验收标准，从8条扩展至22条，补充代码参考映射"
    snapshot: "独立 RESTful API Server，8个HTTP端点、SSE流式输出、CORS/认证/限流中间件、会话管理、生命周期控制"
source_code:
  - pkg/api/server.go
---

# REST API Server (pkg/api/)

## 描述
独立的 RESTful API Server，与 Admin Server 分离。提供聊天、流式输出、会话管理、模型列表等 API 端点。支持 CORS、API Key 认证、TokenBucket 限流。server.go 约 468 行代码，通过 AgentBridge 与 Agent 核心交互，提供标准化的 HTTP API 接口供外部系统调用。

## 验收标准

### 服务器核心
- [x] Server 结构体包含 config、bridge、router、server、sessions 五个核心字段
- [x] Config 结构体支持 Host(默认0.0.0.0)、Port(默认8080)、APIKey、EnableCORS(默认true)、RateLimit(默认60) 五项配置
- [x] NewServer 构造函数接收 Config 和 AgentBridge，初始化路由和 HTTP Server
- [x] Server.Start() 启动 HTTP 监听服务，Server.Stop() 优雅关闭服务

### HTTP 端点
- [x] POST /v1/chat — 同步聊天接口，返回完整 ChatResponse
- [x] POST /v1/chat/stream — SSE 流式聊天接口，逐事件推送响应
- [x] GET /v1/session/{id} — 获取指定会话详情
- [x] DELETE /v1/session/{id} — 删除指定会话
- [x] GET /v1/sessions — 列出所有活跃会话
- [x] GET /v1/models — 列出可用模型列表
- [x] GET /v1/health — 健康检查端点
- [x] GET /v1/info — 服务信息端点

### SSE 流式输出
- [x] 响应 Content-Type 为 text/event-stream
- [x] 流式上下文设置5分钟超时（context.WithTimeout）
- [x] eventChan 缓冲区大小为100，防止背压阻塞
- [x] 支持三种 SSE 事件类型：message（消息片段）、done（完成标记）、error（错误通知）
- [x] SSEEvent 结构体包含 Type、Data、Retry 三个字段

### 请求/响应结构
- [x] ChatRequest 包含 message、session_id、model、stream 四个字段
- [x] ChatResponse 包含 session_id、message、type、created_at 四个字段
- [x] created_at 字段使用 time.Time 类型记录响应时间戳

### 中间件
- [x] CORS 中间件：设置 Access-Control-Allow-Origin: *，允许所有方法和头部，可通过 EnableCORS 配置开关
- [x] 认证中间件（authMiddleware）：支持 Bearer Token 校验，APIKey 为空时跳过认证
- [x] 限流中间件（rateLimitMiddleware）：基于 TokenBucket 实现，速率可通过 RateLimit 配置

### 辅助功能
- [x] writeJSON 辅助方法：统一 JSON 响应写入
- [x] writeError 辅助方法：统一错误响应写入，包含 HTTP 状态码和错误信息
- [x] parseBearerToken 辅助方法：从 Authorization 头部解析 Bearer Token
- [x] 会话管理通过 AgentBridge 的 GetAgent/ClearSession/SessionCount 实现

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| Server 结构体 | `pkg/api/server.go` Server struct |
| Config 结构体 | `pkg/api/server.go` Config struct |
| NewServer 构造函数 | `pkg/api/server.go` NewServer() |
| Start/Stop 生命周期 | `pkg/api/server.go` Start(), Stop() |
| 8个HTTP端点注册 | `pkg/api/server.go` setupRoutes() |
| handleChat 同步聊天 | `pkg/api/server.go` handleChat() |
| handleChatStream 流式聊天 | `pkg/api/server.go` handleChatStream() |
| 会话查询/删除 | `pkg/api/server.go` handleGetSession(), handleDeleteSession() |
| 会话列表 | `pkg/api/server.go` handleListSessions() |
| 模型列表 | `pkg/api/server.go` handleListModels() |
| 健康检查/服务信息 | `pkg/api/server.go` handleHealth(), handleInfo() |
| SSE 流式机制 | `pkg/api/server.go` handleChatStream() 内 eventChan |
| CORS 中间件 | `pkg/api/server.go` corsMiddleware() |
| 认证中间件 | `pkg/api/server.go` authMiddleware() |
| 限流中间件 | `pkg/api/server.go` rateLimitMiddleware() |
| ChatRequest/ChatResponse | `pkg/api/server.go` 对应 struct 定义 |
| SSEEvent 结构体 | `pkg/api/server.go` SSEEvent struct |
| writeJSON/writeError | `pkg/api/server.go` writeJSON(), writeError() |
| parseBearerToken | `pkg/api/server.go` parseBearerToken() |
