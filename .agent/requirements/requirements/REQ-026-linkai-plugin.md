---
id: REQ-026
title: "LinkAI 插件：知识库、Midjourney 绘画与文档摘要"
status: active
level: story
priority: P1
cluster: plugins
created_at: "2026-04-23T16:00:00"
updated_at: "2026-04-23T16:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-006]
  merged_from: []
  depends_on: [REQ-004]
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/plugin/linkai/linkai.go (1945行)"
    reason: "逆向代码生成需求"
    snapshot: "LinkAI 集成插件：群聊应用映射、Midjourney 绘画（速率限制）、文档摘要（文件/URL/对话）、过期会话管理"
---

# LinkAI 插件：知识库、Midjourney 绘画与文档摘要

## 描述
LinkAI 插件是系统中最大的插件（1945 行），集成 LinkAI 平台能力。三大核心功能：(1) 知识库/群聊应用映射，通过 GroupAppMap 将群聊消息路由到不同应用；(2) Midjourney 绘画，支持图片生成/放大/变体，含速率限制和任务追踪；(3) 文档摘要，支持文件、URL、分享链接的自动总结及后续对话。使用 ExpiredMap 管理用户会话状态。

## 验收标准
- [x] LinkAIPlugin 结构体：嵌入 `BasePlugin`，持有 `config *Config`、`client *LinkAIClient`、`mjBot *MJBot`、`summary *SummaryService`、`userMap *ExpiredMap`（30 分钟 TTL） (linkai.go:96-105)
- [x] MidjourneyConfig 配置：`Enabled`、`AutoTranslate`（默认 true）、`ImgProxy`（默认 true）、`MaxTasks`（默认 3）、`MaxTasksPerUser`（默认 1）、`UseImageCreatePrefix`（默认 true）、`Mode`（默认 "fast"） (linkai.go:56-71)
- [x] SummaryConfig 配置：`Enabled`（默认 false）、`GroupEnabled`（默认 true）、`MaxFileSize`（默认 5000KB）、`Type`（默认 ["FILE","SHARING"]） (linkai.go:74-83)
- [x] Config 配置：`GroupAppMap map[string]string`（群名→应用编码）、`Midjourney`、`Summary` (linkai.go:86-93)
- [x] LinkAIClient：`apiKey`、`baseURL`、`client *http.Client`，通过 `NewLinkAIClient(apiKey, baseURL)` 创建 (linkai.go:1062-1069)
- [x] API 路径常量：`apiSummaryFile="/v1/summary/file"`、`apiSummaryURL="/v1/summary/url"`、`apiSummaryChat="/v1/summary/chat"`、`apiMJGenerate="/v1/img/midjourney/generate"`、`apiMJOperate="/v1/img/midjourney/operate"`、`apiMJTasks="/v1/img/midjourney/tasks/"` (linkai.go:44-52)
- [x] OnInit 初始化：加载配置 → `NewLinkAIClient(cfg.LinkAIAPIKey, cfg.LinkAIAPIBase)` → `NewMJBot(&p.config.Midjourney, p.fetchGroupAppCode)` → `NewSummaryService(p.client, &p.config.Summary)` (linkai.go:152-184)
- [x] 消息路由 `onHandleContext`：按 `types.ContextType` 分发到 `handleTextMessage`/`handleImageMessage`/`handleImageCreateMessage`/`handleFileMessage`/`handleSharingMessage` (linkai.go:209-236)
- [x] 文本消息处理 `handleTextMessage`：依次检查 linkai 管理命令 → Midjourney 命令 → 摘要对话功能 → 群聊应用消息 (linkai.go:239-266)
- [x] Midjourney 命令触发：`$mj`（生成）、`$mju`（放大）、`$mjv`（变体）、`$mjr`（重置），通过 `tryHandleMidjourneyCommand` 分发 (linkai.go:269-289)
- [x] MJBot 结构体：`client`、`config *MidjourneyConfig`、`fetchGroupApp`、`tasks map[string]*MJTask`、`tempDict`、`mu sync.Mutex` (linkai.go:1647-1653)
- [x] MJ 任务类型：`MJTaskGenerate`、`MJTaskUpscale`、`MJTaskVariation`、`MJTaskReset` (linkai.go:1618-1621)
- [x] MJ 任务状态：`MJStatusPending`、`MJStatusFinished`、`MJStatusExpired`、`MJStatusAborted` (linkai.go:1628-1631)
- [x] MJTask 结构体：`ID`、`UserID`、`TaskType`、`RawPrompt`、`Status`、`ImgURL`、`ImgID`、`ExpiryTime` (linkai.go:1635-1643)
- [x] MJ 速率限制 `CheckRateLimit(sessionID, ec, maxTasks, maxTasksPerUser)`：检查全局任务数和每用户任务数 (linkai.go:1673)
- [x] MJ 生成 `Generate(prompt, sessionID, ec)`：自动翻译 → 调用 API → 创建任务 → 返回回复 (linkai.go:1709)
- [x] MJ 操作 `DoOperate(taskType, taskID, sessionID)`：放大/变体/重置 (linkai.go:1760)
- [x] SummaryService 结构体：`client *LinkAIClient`、`config *SummaryConfig` (linkai.go:1468-1470)
- [x] 摘要结果 `SummaryResult`：`Summary`、`SummaryID`、`FileID` (linkai.go:1482-1485)
- [x] 摘要文件 `SummaryFile(filePath, appCode, sessionID)`：调用 `/v1/summary/file` API (linkai.go:1495)
- [x] 摘要 URL `SummaryURL(url, appCode, sessionID)`：调用 `/v1/summary/url` API (linkai.go:1513)
- [x] 摘要对话 `SummaryChat(question, summaryID, appCode, sessionID)`：调用 `/v1/summary/chat` API (linkai.go:1530)
- [x] 文件校验 `CheckFile(path)`：检查 `MaxFileSize` 和 `Type` 白名单 (linkai.go:1547)
- [x] URL 校验 `CheckURL(content)`：正则匹配 HTTP/HTTPS 链接 (linkai.go:1577)
- [x] 摘要对话交互：`cmdStartChat="开启对话"` → 保存 sumID → 后续消息走 `handleSummaryChat`；`cmdExitChat="退出对话"` → 清除状态 (linkai.go:38-39, 296-313)
- [x] 群聊应用映射 `handleGroupAppMessage`：通过 `GroupAppMap[group_name]` 获取 app_code，调用 `LinkAIClient.Chat` (linkai.go:873)
- [x] ExpiredMap 会话管理：`NewExpiredMap(30*time.Minute)`，`Set`/`Get`/`Delete` 操作，自动清理过期条目 (linkai.go:1878-1933)
- [x] 管理命令 `handleAdminCommand`：`$linkai open/close`（开关插件）、`$linkai app`（查看应用）、`$linkai sum`（摘要管理）、`$linkai mj`（MJ 管理） (linkai.go:608-722)
