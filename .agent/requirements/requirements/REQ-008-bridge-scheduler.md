---
id: REQ-008
title: "桥接与调度系统"
status: active
level: epic
priority: P0
cluster: bridge
created_at: "2026-04-23T10:00:00"
updated_at: "2026-04-23T16:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: []
  merged_from: []
  depends_on: [REQ-001, REQ-004, REQ-005]
  refined_by: [REQ-016, REQ-017]
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T10:00:00"
    author: ai
    context: "从代码逆向分析提取需求，来源: pkg/bridge/ 和 pkg/scheduler/"
    reason: "逆向代码生成需求"
    snapshot: "渠道-LLM-Agent 消息桥接、定时任务调度器、API Server、Admin 管理界面"
  - version: 2
    date: "2026-04-22T16:13:03"
    author: ai
    context: "元数据自动同步"
    reason: "自动补充反向关系: refined_by"
    snapshot: "自动同步元数据"
  - version: 3
    date: "2026-04-23T16:00:00"
    author: ai
    context: "深度分析 bridge.go 和 agent_bridge.go 代码，细化验收标准"
    reason: "代码深度分析细化"
    snapshot: "详细 Bridge/AgentBridge 实现：会话隔离、并行组件初始化、Protocol 集成、消息验证、调度器持久化"
---

# 桥接与调度系统

## 描述
连接渠道、LLM 和 Agent 的消息桥接层。Bridge 根据 config.Agent 决定走普通 LLM 模式或 Agent 模式，通过 BotType 路由到对应 LLM 提供商。AgentBridge 管理每个会话的 Agent 实例（会话隔离），并行初始化可选组件（语音/翻译/嵌入器），集成 Protocol 层、Memory、Skills、Plugin 等子系统。定时调度器基于 robfig/cron 支持秒级 Cron 表达式、间隔任务和一次性任务，任务持久化到 JSON 文件。API Server 提供 RESTful 接口和 SSE 流式端点。

## 验收标准
- [x] Bridge 桥接层：FetchReplyContent 根据 cfg.Agent 决定走 Agent 模式（FetchAgentReply）或普通模式（直接 Call LLM）
- [x] BotType 路由：initBotTypes 根据模型名前缀自动映射（claude→BotTypeClaude, gemini→BotTypeGemini 等），默认 BotTypeOpenAI
- [x] Model 懒加载：GetBot(botType) 通过 createBot 创建并缓存 llm.Model 实例，每个 BotType 只创建一次
- [x] createBot 配置映射：按 BotType 提取对应 APIKey/APIBase/Provider，Xunfei 使用 Extra（app_id/api_key/api_secret）
- [x] Bridge.Reset：清空 bots 缓存并重新初始化 BotType 映射（支持模型热切换）
- [x] AgentBridge 会话隔离：agents map[sessionID]*Agent，每个会话独立 Agent 实例，sessionLocks 保证同会话串行处理
- [x] 默认 Agent：后台 goroutine 预初始化 defaultAgent，空 sessionID 返回默认实例
- [x] AgentReply 流程：extractSessionID → getSessionLock → GetAgent → setAgentToolContext → AddUserMessage → a.Run → persistSessionMessages
- [x] ToolContext 注入：从 types.Context 提取 user_id/group_id/is_group/channel_type/receiver/receive_id_type 到 ToolContext
- [x] 会话持久化：persistSessionMessages 将 query（RoleUser）和 response（RoleAssistant）写入 memoryMgr
- [x] 会话恢复：AgentInitializer.restoreSessionHistory 从 memoryMgr.GetSessionMessages 获取最近 50 条消息，恢复到新 Agent
- [x] 会话清理：ClearSession 清除 Agent 实例 + SessionManager 会话 + memoryMgr 数据；ClearAllSessions 重置所有
- [x] 可选组件并行初始化：initVoiceEngine/initTranslator/initEmbedder 三个 goroutine 并行启动
- [x] 技能加载：skillsRegistry.LoadFromDir 加载 workspace/skills + ~/.agents/skills/（全局技能目录）
- [x] 速率限制：TokenBucket(100, 10) 默认配置，TryAcquireToken/AcquireToken 提供阻塞/非阻塞获取
- [x] 响应缓存：ExpireMap[string, string] 5分钟过期，CacheResponse/GetCachedResponse/ClearCache
- [x] 时间检查：TimeChecker 服务时间范围控制，IsInServiceTime/SetServiceTimeRange/GetServiceTimeRange
- [x] 翻译集成：initTranslator 初始化百度翻译器，Translate/TranslateToChinese/TranslateToEnglish
- [x] 嵌入器集成：initEmbedder 使用 OpenAI text-embedding-3-small，CachedEmbedder 包装（EmbeddingCache 10000 条）
- [x] Protocol 集成：CreateTask/CreateTextTask/CreateImageTask/CreateAudioTask/CreateMixedTask/RunTask
- [x] Protocol 事件工厂：CreateTextEvent/CreateToolCallEvent/CreateToolResultEvent/CreateCompleteEvent
- [x] 消息验证：SanitizeMessages 修复 tool_use/tool_result 邻接关系，移除孤立和不匹配的块
- [x] Reply 工厂：NewTextReply/NewErrorReply/NewInfoReply/NewImageReply/NewVoiceReply/NewVideoReply/NewFileReply/NewCardReply
- [x] API Server：RESTful 接口（/v1/chat, /v1/chat/stream, /v1/session/, /v1/models, /v1/health, /v1/info），CORS、API Key 认证、TokenBucket 限流
- [x] Admin 管理界面：Web UI（go:embed 前端构建产物），JWT 认证
- [x] 定时调度器（scheduler）：Cron 表达式（秒级）、interval 间隔、once 一次性任务
- [x] 调度任务类型：ActionTypeSendMessage、ActionTypeAgentTask
- [x] 任务持久化：JSON 文件存储（~/.simpleclaw/scheduler/tasks.json），Store.LoadAll/Save/Delete
- [x] BridgeTaskExecutor：调度器任务执行器，通过渠道发送消息或调用 Agent
- [x] 优雅关闭：信号监听 → 依次关闭 API/Admin/扩展/调度器/渠道 → 日志 Sync
