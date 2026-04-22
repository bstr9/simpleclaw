---
id: REQ-021
title: "特殊 API 提供商适配（DashScope/讯飞/百度）"
status: active
level: story
priority: P1
cluster: llm
created_at: "2026-04-23T16:00:00"
updated_at: "2026-04-23T16:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-005]
  merged_from: []
  depends_on: [REQ-018]
  related_to: [REQ-022]
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/llm/dashscope.go (886行), xunfei.go (484行), baidu.go (518行)"
    reason: "逆向代码生成需求"
    snapshot: "三类特殊 API 提供商：DashScope 灵积平台（多模态+流式+工具调用）、讯飞星火（WebSocket+HMAC签名）、百度文心（OAuth令牌+流式）"
---

# 特殊 API 提供商适配（DashScope/讯飞/百度）

## 描述
三类使用特殊 API 协议的 LLM 提供商适配，无法复用 OpenAI 兼容客户端，需独立实现。DashScope（阿里灵积）使用自定义请求格式，区分文本生成和多模态端点；讯飞星火使用 WebSocket 协议和 HMAC-SHA256 签名认证；百度文心使用 OAuth2 access_token 认证和 SSE 流式。

## 验收标准

### DashScope 灵积平台
- [x] `DashScopeModel` 结构体实现 `Model` 接口：`Name()`、`Call()`、`CallStream()`、`SupportsTools()`
- [x] `NewDashScopeModel(cfg)` 构造：校验 `APIKey`，默认模型 `qwen-plus`，默认 BaseURL `https://dashscope.aliyuncs.com/api/v1`
- [x] 双端点路由：文本生成 `/services/aigc/text-generation/generation`，多模态 `/services/aigc/multimodal-generation/generation`
- [x] 多模态模型识别 `isMultiModalModel()`：前缀匹配 `qwen3.`/`qwen3-`/`qwq-`，加上已知 VL/Qwen3.5 模型
- [x] 请求结构 `dashScopeRequest`：`Model`/`Input{Messages}`/`Parameters{ResultFormat, Temperature, TopP, MaxTokens, Stop, Tools, ToolChoice, EnableThinking, ThinkingBudget, IncrementalOutput}`
- [x] `ResultFormat: "message"` 固定设置，流式模式 `IncrementalOutput: true`
- [x] 认证：`Authorization: Bearer {apiKey}` Header
- [x] 流式启用：`X-DashScope-SSE: enable` Header
- [x] 模型信息表 `dashScopeModelInfo`：含 ContextWindow/SupportsVision/SupportsTools/SupportsStreaming（如 qwen-long 1M context，VL 系列不支持工具）
- [x] 模型别名映射 `mapDashScopeModel()`：如 "qwen" → "qwen-turbo"，"qwq" → "qwq-plus"
- [x] 多模态消息格式：`isMultiModalModel` 时 Content 转为 `[]dashScopeContentBlock{{Text: ...}}`
- [x] 流式增量输出 `sendContentDelta()`：基于 `lastContent` 前缀比对计算增量 delta
- [x] 工具调用流式输出 `sendToolCalls()`：直接发送完整 `ToolCallDelta`
- [x] 推理模型支持：QwQ Plus/32B，`EnableThinking`/`ThinkingBudget` 参数

### 讯飞星火（WebSocket API）
- [x] `XunfeiModel` 结构体实现 `Model` 接口，`SupportsTools()` 返回 false（WebSocket 不支持工具调用）
- [x] 配置提取 `extractXunfeiConfig()`：从 `cfg.Extra` 读取 `app_id`/`api_key`/`api_secret`/`domain`
- [x] 组合 APIKey 格式解析 `parseAPIKeyFormat()`：`appID:apiKey:apiSecret` 三段式
- [x] WebSocket URL 映射 `sparkURLs`：Lite→v1.1，V2→v2.1，V3→v3.1，Pro128K，V35→v3.5，V4Ultra→v4.0
- [x] HMAC-SHA256 认证 `generateAuthURL()`：签名原始串 `host: {host}\ndate: {date}\nGET {path} HTTP/1.1`，authorization Base64 编码
- [x] 请求结构 `sparkRequest`：`Header{AppID, UID}`/`Parameter{Chat{Domain, Temperature, MaxTokens, Auditing}}`/`Payload{Message{Text}}`
- [x] `Call()` 实现：内部调用 `CallStream()` 收集所有 chunk 拼接
- [x] `CallStream()` 实现：WebSocket 连接 + goroutine `handleWebSocket()` 读写
- [x] 流式状态码：`status=2` 表示完成，发送 Usage 和 Done
- [x] 域常量：`SparkDomainLite`/`V2`/`V3`/`Pro128K`/`V35`/`V4Ultra`

### 百度文心（OAuth Token API）
- [x] `BaiduModel` 结构体实现 `Model` 接口：`Name()`、`Call()`、`CallStream()`、`SupportsTools()`
- [x] 双密钥认证：`cfg.APIKey`（client_id）+ `cfg.SecretKey`（client_secret）
- [x] OAuth 令牌获取 `getAccessToken()`：POST `https://aip.baidubce.com/oauth/2.0/token?grant_type=client_credentials`，令牌缓存 + 读写锁 `tokenMutex`
- [x] 令牌过期处理：提前 5 分钟刷新，未提供 expires_in 时默认 24 小时
- [x] API 端点：`POST https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/{model}?access_token={token}`
- [x] 模型标识符常量：`BaiduModelERNIEBot`/`ERNIEBotTurbo`/`ERNIEBot4`/`ERNIEBot8K`/`ERNIESpeed`/`ERNIESpeed128K`/`ERNIELite`/`ERNIELite8K`/`ERNIETiny`
- [x] 工具支持判断 `isBaiduModelSupportsTools()`：ERNIEBot/ERNIEBot4/ERNIEBot8K/ERNIESpeed/ERNIELite 支持，其余不支持
- [x] 请求构建 `buildBaiduRequest()`：跳过 `RoleTool` 消息（百度工具支持有限），`SystemPrompt` 单独设置 `req.System`
- [x] 流式处理 `processBaiduStream()`：SSE 格式，`is_end=true` 时发送 Done，`SentenceResults` 提取增量内容
- [x] 流式 Usage 累积：最后一个含 Usage 的 chunk 携带 token 统计
