---
id: REQ-016
title: "Admin 管理界面"
status: active
level: story
priority: P1
cluster: bridge
created_at: "2026-04-23T10:10:00"
updated_at: "2026-04-26T10:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-008]
  merged_from: []
  depends_on: []
  refined_by: [REQ-053]
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T10:10:00"
    author: ai
    context: "从 REQ-008 拆分，来源: pkg/admin/"
    reason: "Epic 拆分为 Story"
    snapshot: "Admin Web UI：SPA 管理界面，JWT 认证，配置管理，状态监控，提供商测试"
  - version: 2
    date: "2026-04-23T18:00:00"
    author: ai
    context: "基于 pkg/admin/ 源码逆向补充详细验收标准"
    reason: "代码级验收标准细化"
    snapshot: "Admin 管理界面：SPA + REST API，session cookie 认证(bcrypt+crypto/rand)，配置原子写入，敏感字段脱敏，Web 渠道反向代理，嵌入静态资源"
  - version: 3
    date: "2026-04-26T10:00:00"
    author: ai
    context: "需求审查发现提供商列表 API 硬编码 5 个提供商"
    reason: "在提供商列表 API 验收标准中补充说明，未来应从 REQ-023 工厂注册表动态获取"
    snapshot: "Admin 管理界面，提供商列表 API 标注硬编码问题"
source_code:
  - pkg/admin/server.go
  - pkg/admin/auth.go
  - pkg/admin/types.go
  - pkg/admin/embed.go
---

# Admin 管理界面

## 描述
Admin 管理界面提供 Web UI 对系统进行配置和监控。支持初始设置、登录认证、配置查看/修改、状态监控、LLM 提供商测试等功能。前端构建产物可通过 `//go:embed` 嵌入到 Go 二进制中，也可从外部静态目录加载。所有 API 以 JSON 格式交互，受 session cookie 认证保护。

## 验收标准
- [x] 初始设置 API `/admin/api/setup`（POST）：首次配置时设置管理员密码和 LLM/渠道参数，已配置后再次调用返回 400 错误 "Already configured with password"
- [x] 密码哈希使用 `bcrypt.GenerateFromPassword` (DefaultCost) 存储，认证使用 `bcrypt.CompareHashAndPassword` 验证
- [x] 登录 API `/admin/api/auth/login`（POST）：验证用户名密码后创建 session，返回 JSON `{success, token}` 并设置 `admin_token` cookie（HttpOnly, Path=/, Expires=24h）
- [x] Session Token 通过 `crypto/rand` 生成 32 字节随机数后 hex 编码（64字符），存储在 `Server.sessions` map 中（sync.RWMutex 保护）
- [x] Session 有效期 24 小时（`ExpiresAt = CreatedAt + 24h`），过期 session 在 `validateSession` 中判定为无效
- [x] 登出 API `/admin/api/auth/logout`（需认证）：从 sessions map 中删除 token，同时设置 cookie MaxAge=-1 清除浏览器端
- [x] Token 提取顺序：优先 `Authorization: Bearer <token>` 头，其次 `admin_token` cookie，最后 `?token=` URL 查询参数
- [x] 认证中间件 `withAuth`：仅当 `config.Enabled && config.PasswordHash != ""` 时启用认证，未配置密码时所有接口免认证
- [x] 配置读取 API `/admin/api/config`（GET，需认证）：调用 `config.Get().MaskSensitive()` 脱敏后返回，附加 admin 段（enabled/username/host/port，不含 password_hash）
- [x] API Key 脱敏 `maskAPIKey`：key 长度 <8 返回空字符串，否则保留前4+后4字符，中间用 `****` 替代
- [x] 配置更新 API `/admin/api/config`（PUT，需认证）：接收 JSON config 对象，原子写入（先写 `.tmp` 再 `os.Rename`），文件权限 0600，保存后调用 `config.Reload` 热重载
- [x] 配置验证 API `/admin/api/config/validate`（POST，需认证）：验证 model、open_ai_api_key、channel_type 三个必填字段，返回 `{valid, errors[]}`
- [x] LLM 连接测试 API `/admin/api/test/llm`（POST，需认证）：接收 provider/api_key/api_base/model 参数，当前返回成功响应（占位实现）
- [x] 系统状态 API `/admin/api/status`（GET，无需认证）：返回版本号、uptime、渠道状态、是否已配置、是否有密码、LLM 配置状态、脱敏 API Key、base_url、model、channel_type、admin_username
- [x] 渠道状态 API `/admin/api/channels`（GET，需认证）：解析 `config.ChannelType`（逗号分隔），返回每个渠道的 Name/Type/Enabled/Running 状态
- [x] 提供商列表 API `/admin/api/providers`（GET，无需认证）：返回 OpenAI/Anthropic/智谱AI/DeepSeek/通义千问 5 个提供商及其可用模型列表（当前硬编码 5 个提供商，未来应从 REQ-023 工厂注册表动态获取）
- [x] Web 渠道反向代理：`/message`、`/stream`、`/upload`、`/uploads/`、`/config`、`/api/*` 路径转发到 `webChannelURL`（默认 `http://localhost:9899`），支持 SSE 流式转发（http.Flusher）
- [x] SPA 静态文件服务 `handleSPA`：优先从 StaticDir 本地目录读取，其次从嵌入式 fs.FS 读取，未匹配路径回退 index.html，`/admin/api/*` 路径返回 404
- [x] 嵌入式 UI：通过 `//go:embed all:static` 嵌入前端构建产物，`HasEmbeddedUI()` 检测 static/index.html 和 static/assets/ 是否存在
- [x] 静态资源缓存策略：`assets/` 目录下文件设置 `Cache-Control: public, max-age=31536000`（1年），其他文件设置 `no-cache`
- [x] MIME 类型检测 `getContentType`：覆盖 js/css/html/json/png/jpg/svg/ico/woff/woff2/ttf 等常见类型，未知类型返回 `application/octet-stream`
- [x] 服务器生命周期：`Start()` 启动 HTTP 服务（ReadTimeout=30s, WriteTimeout=30s, IdleTimeout=60s），`Shutdown(ctx)` 优雅关闭
- [x] 默认配置：Host=`0.0.0.0`，Port=`31415`，Username=`admin`，SessionSecret 和 StaticDir 为空

## 代码参考
| 验收标准 | 代码位置 |
|---------|---------|
| 初始设置 API | `pkg/admin/server.go:handleSetup` (L167) |
| 密码 bcrypt 哈希 | `pkg/admin/auth.go:ValidatePassword` (L15), `pkg/admin/server.go:handleSetup` (L189) |
| 登录 API + cookie | `pkg/admin/server.go:handleLogin` (L238-L269) |
| Token crypto/rand 生成 | `pkg/admin/server.go:generateToken` (L600-L604) |
| Session 存储 + 24h 过期 | `pkg/admin/server.go:createSession` (L145-L159), `validateSession` (L129-L143) |
| 登出 + cookie 清除 | `pkg/admin/server.go:handleLogout` (L272-L287) |
| Token 提取三重来源 | `pkg/admin/server.go:extractToken` (L115-L127) |
| withAuth 认证中间件 | `pkg/admin/server.go:withAuth` (L96-L113) |
| 配置读取 + MaskSensitive | `pkg/admin/server.go:getConfig` (L300-L312) |
| API Key 脱敏 | `pkg/admin/server.go:maskAPIKey` (L392-L397) |
| 配置原子写入 | `pkg/admin/server.go:saveConfig` (L507-L524), `updateConfig` (L314-L331) |
| 配置验证 | `pkg/admin/server.go:validateConfig` (L641-L657), `handleValidate` (L333-L346) |
| LLM 连接测试 | `pkg/admin/server.go:testLLMConnection` (L552-L557), `handleTestLLM` (L348-L362) |
| 系统状态 API | `pkg/admin/server.go:handleStatus` (L364-L390) |
| 渠道状态 API | `pkg/admin/server.go:handleChannels` (L399-L407), `getChannelStatuses` (L559-L574) |
| 提供商列表 | `pkg/admin/server.go:handleProviders` (L659-L677) |
| Web 渠道反向代理 | `pkg/admin/server.go:proxyToWebChannel` (L679-L726) |
| SPA 静态文件 | `pkg/admin/server.go:handleSPA` (L409-L468) |
| 嵌入式 UI | `pkg/admin/embed.go:GetDistFS` (L57), `HasEmbeddedUI` (L65), `checkStaticFiles` (L22) |
| 缓存策略 | `pkg/admin/server.go:handleSPA` (L441-L445) |
| MIME 类型 | `pkg/admin/server.go:getContentType` (L470-L496) |
| 服务器生命周期 | `pkg/admin/server.go:Start` (L576-L591), `Shutdown` (L593-L598) |
| 默认配置 | `pkg/admin/types.go:DefaultAdminConfig` (L13-L22) |
