---
id: REQ-015
title: "Web 渠道与前端 UI"
status: active
level: story
priority: P0
cluster: channels
created_at: "2026-04-23T10:10:00"
updated_at: "2026-04-23T18:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-004]
  merged_from: []
  depends_on: []
  related_to: [REQ-008]
versions:
  - version: 1
    date: "2026-04-23T10:10:00"
    author: ai
    context: "从 REQ-004 拆分，来源: extensions/web/"
    reason: "Epic 拆分为 Story"
    snapshot: "Web 渠道：WebSocket + SSE 双向通信，嵌入前端 UI，支持流式输出"
  - version: 2
    date: "2026-04-23T18:00:00"
    author: ai
    context: "逆向代码分析，扩展验收标准至代码级细节"
    reason: "代码逆向补充详细验收标准"
    snapshot: "Web 渠道：HTTP 服务+SSE 流式推送+会话管理+文件上传+管理后台，20+ API 端点，CORS 中间件，TTS/STT/翻译/记忆 API 代理"
source_code:
  - pkg/channel/web/web_channel.go
  - pkg/channel/web/handlers.go
  - pkg/channel/web/admin_handlers.go
---

# Web 渠道与前端 UI

## 描述
Web 渠道提供浏览器端聊天界面和 HTTP API 服务。核心特性：HTTP 服务器（默认端口 9899）+ SSE 流式推送 + 轮询模式双通道响应、会话管理、文件上传（100MB 限制）、CORS 中间件、20+ API 端点（聊天/配置/渠道/模型/语音/记忆/翻译）、管理后台（登录/初始化/状态/配置）。前端可通过 StaticDir 配置或内嵌默认 HTML。

## 验收标准
- [x] WebChannel 嵌入 BaseChannel，渠道名称 "web"，实现 Channel 接口（Startup/Stop/Send）
- [x] 配置结构 Config：Port（默认 9899）、Host（默认 0.0.0.0）、UploadDir、StaticDir、AllowOrigins（默认 "*"）
- [x] HTTP 服务器参数：ReadTimeout 30s、WriteTimeout 300s（SSE 需长超时）、IdleTimeout 120s
- [x] SSE 流式推送：SSEEvent 结构（type/content/tool/arguments/status/result/execution_time/request_id/timestamp）
- [x] SSE 队列管理：createSSEQueue/getSSEQueue/pushSSEEvent/removeSSEQueue，缓冲区 100，满时丢弃事件并警告
- [x] SSE 连接处理 handleStream：text/event-stream 响应头、keepalive 每 15 秒、超时 5 分钟、done/error 事件关闭连接
- [x] 会话队列管理：sessionQueues map（缓冲区 100），支持轮询模式获取响应
- [x] 请求-会话映射 requestToSession：requestID → sessionID，支持跨请求路由响应
- [x] 消息请求 MessageRequest：session_id、message、stream(bool)、attachments([]Attachment)
- [x] 附件结构 Attachment：file_path、file_name、file_type
- [x] 消息处理 handleMessage：POST /message，生成 requestID，映射到 session，异步调用 messageHandler
- [x] WebMessage 结构：BaseMessage + SessionID + RequestID + Attachments + Stream + OnEvent 回调
- [x] SSE 事件回调 makeSSECallback：处理 text→delta、complete→done、error→error、tool_call→tool_start、tool_result→tool_end、step_start/step_end→step 事件类型
- [x] 工具结果截断：tool_end 结果超过 2000 字符时截断并追加 "…"
- [x] 轮询模式 handlePoll：POST /poll，按 session_id 从 sessionQueue 非阻塞读取响应
- [x] Send 方法：优先检查 SSE 队列（pushSSEEvent type=done），否则走 session 队列（pushSessionResponse）
- [x] 回复类型过滤：Startup 时设置 NotSupportTypes 包含 ReplyVoice
- [x] 文件上传 handleUpload：POST /upload，100MB 限制（MaxBytesReader），文件名 web_{timestamp}{ext}，保存到 uploadDir
- [x] 上传文件类型识别：按扩展名分类为 image（jpg/jpeg/png/gif/webp/bmp/svg）或 video（mp4/webm/avi/mov/mkv）
- [x] 上传文件访问 handleUploads：GET /uploads/{filename}，路径遍历防护（absPath 必须以 absUploadDir 为前缀），缓存 86400 秒
- [x] 上传目录 getUploadDir：优先 config.UploadDir，默认 ~/cow/tmp，通过 ConfigProvider 获取 agent_workspace
- [x] CORS 中间件 corsMiddleware：AllowOrigins "*" 时设置 Access-Control-Allow-Origin: *，否则按逗号分隔匹配；预检请求返回 200
- [x] CORS 头设置：Allow-Methods GET/POST/PUT/DELETE/OPTIONS，Allow-Headers Content-Type/Authorization/X-Requested-With，Max-Age 86400
- [x] 日志中间件 loggingMiddleware：记录 method/path/status/duration/remote，使用 responseWriter 包装器捕获状态码
- [x] 聊天页面 handleChat：优先从 StaticDir 加载 chat.html，否则返回内嵌 defaultChatHTML（完整单页聊天应用）
- [x] SPA 路由 serveSPA：StaticDir 配置时尝试匹配静态文件，无匹配返回 404
- [x] 配置 API handleConfig：GET 获取运行配置，POST 动态更新（model/debug/agent_max_steps），PUT 重新加载配置文件
- [x] 渠道 API handleChannels/handleChannelOps：列出渠道（weixin/web/terminal/feishu/dingtalk），支持 GET/POST/DELETE 操作
- [x] 会话 API handleSessions/handleSessionOps：列出活跃会话、查询会话信息、删除会话（同时清理 requestToSession 映射）
- [x] 历史记录 API handleHistory：按 session_id 获取/清空历史
- [x] 插件 API handlePlugins：通过 agentBridge.ListPlugins() 获取插件列表（name/version/enabled/description/priority）
- [x] 语音 API handleVoiceInfo/handleTTS/handleSTT：查询语音引擎状态、文本转语音（audio/mpeg）、语音转文本
- [x] 翻译 API handleTranslate：POST /api/translate，text/from/to 参数，默认翻译到中文
- [x] 记忆 API handleMemoryInfo/handleMemorySearch：查询记忆系统状态、语义搜索记忆
- [x] 模型/提供商 API handleModels/handleProviders：列出可用模型和提供商（openai/anthropic/zhipu/deepseek/qwen）
- [x] 管理后台登录 handleAdminLogin：用户名 admin，密码通过 subtle.ConstantTimeCompare 比对哈希，生成 64 字符 hex token
- [x] 管理后台初始化 handleAdminSetup：首次设置 provider/apiKey/modelName/adminPassword（≥6 位），写入 config
- [x] 管理后台状态 handleAdminStatus：返回版本/Go 版本/OS/运行时间/内存/CPU 核数/会话数/LLM 连接状态
- [x] 管理后台配置 handleAdminConfig：GET 获取配置，PUT 更新配置（model/agent 参数/debug），持久化保存
- [x] AgentBridgeInterface 接口：HasVoiceEngine/TextToSpeech/SpeechToText/ListVoiceEngines/HasTranslator/Translate/ListTranslators/GetMemoryManager/AddMemory/SearchMemory/GetMemoryStats/ListPlugins
- [x] ConfigProvider 接口：GetString/GetInt/GetBool/GetStringSlice 动态配置访问
- [x] 优雅停止 Stop：server.Shutdown(10s 超时) → 清理 SSE 队列 → 清理 session 队列，sync.Once 防止重复
- [x] 启动幂等 startupOnce：确保 Startup 只执行一次
- [x] 消息 ID 生成 generateMsgID：时间戳 + 原子计数器

## 代码参考
| 验收标准 | 代码位置 |
|---------|---------|
| WebChannel 结构 | web_channel.go:78-113 (WebChannel struct) |
| 配置与默认值 | web_channel.go:69-75 (Config), web_channel.go:168-190 (NewWebChannel) |
| HTTP 服务器参数 | web_channel.go:22-28 (常量), web_channel.go:222-228 (server 配置) |
| SSEEvent 结构 | web_channel.go:31-41 (SSEEvent) |
| SSE 队列管理 | web_channel.go:359-402 (create/get/has/push/removeSSEQueue) |
| SSE 连接处理 | handlers.go:168-241 (handleStream) |
| 会话队列管理 | web_channel.go:406-433 (create/has/push session) |
| 请求-会话映射 | web_channel.go:437-453 (map/get/remove RequestMapping) |
| 消息请求结构 | web_channel.go:44-49 (MessageRequest), web_channel.go:52-56 (Attachment) |
| 消息处理 | handlers.go:95-166 (handleMessage) |
| WebMessage 结构 | web_channel.go:153-165 (WebMessage) |
| SSE 事件回调 | handlers.go:668-764 (makeSSECallback) |
| 工具结果截断 | handlers.go:654-656, handlers.go:733-735 (maxResultLen=2000) |
| 轮询模式 | handlers.go:343-386 (handlePoll) |
| Send 方法 | web_channel.go:286-342 (Send) |
| 回复类型过滤 | web_channel.go:214 (SetNotSupportTypes) |
| 文件上传 | handlers.go:243-305 (handleUpload) |
| 文件类型识别 | handlers.go:35-43 (imageExtensions/videoExtensions) |
| 上传文件访问 | handlers.go:307-341 (handleUploads, 路径遍历防护) |
| 上传目录 | web_channel.go:582-604 (getUploadDir) |
| CORS 中间件 | web_channel.go:456-473 (corsMiddleware) |
| CORS 头设置 | web_channel.go:476-503 (setCORSHeader/setCORSMethods/isOriginAllowed) |
| 日志中间件 | web_channel.go:505-523 (loggingMiddleware) |
| 聊天页面 | handlers.go:388-404 (handleChat), handlers.go:766-872 (defaultChatHTML) |
| SPA 路由 | handlers.go:81-93 (serveSPA) |
| 配置 API | handlers.go:406-487 (handleConfig/getConfig/updateConfig/reloadConfig) |
| 渠道 API | handlers.go:511-624 (handleChannels/getChannels/handleChannelOps) |
| 会话 API | handlers.go:874-945 (handleSessions/handleSessionOps) |
| 历史记录 API | handlers.go:947-986 (handleHistory/clearHistory) |
| 插件 API | handlers.go:988-1014 (handlePlugins) |
| 语音 API | handlers.go:1031-1119 (handleVoiceInfo/handleTTS/handleSTT) |
| 翻译 API | handlers.go:1230-1277 (handleTranslate) |
| 记忆 API | handlers.go:1164-1228 (handleMemoryInfo/handleMemorySearch) |
| 模型/提供商 API | handlers.go:1121-1162 (handleModels/handleProviders) |
| 管理后台登录 | admin_handlers.go:60-103 (handleAdminLogin) |
| 管理后台初始化 | admin_handlers.go:116-181 (handleAdminSetup) |
| 管理后台状态 | admin_handlers.go:183-209 (handleAdminStatus) |
| 管理后台配置 | admin_handlers.go:211-271 (handleAdminConfig) |
| AgentBridge 接口 | web_channel.go:116-129 (AgentBridgeInterface) |
| ConfigProvider 接口 | web_channel.go:132-137 (ConfigProvider) |
| 优雅停止 | web_channel.go:249-283 (Stop) |
| 启动幂等 | web_channel.go:212 (startupOnce.Do) |
| 消息 ID 生成 | web_channel.go:345-350 (generateMsgID) |
