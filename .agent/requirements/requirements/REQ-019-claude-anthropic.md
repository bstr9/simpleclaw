---
id: REQ-019
title: "Claude Anthropic 适配"
status: active
level: story
priority: P0
cluster: llm
created_at: "2026-04-23T16:00:00"
updated_at: "2026-04-23T16:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-005]
  merged_from: []
  depends_on: [REQ-018]
  related_to: [REQ-024]
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/llm/claude.go (785行)"
    reason: "逆向代码生成需求"
    snapshot: "Anthropic Claude API 独立适配，SSE 流式事件解析，content_block 工具调用格式，多模型 max_tokens 分级"
---

# Claude Anthropic 适配

## 描述
Anthropic Claude API 的独立适配实现。由于 Claude API 不兼容 OpenAI 格式，需要完全独立实现 `ClaudeModel` 结构体。核心差异包括：系统提示词从消息列表中提取为独立 `system` 字段、内容使用 `content_block` 数组格式（text/tool_use/tool_result）、流式使用 SSE 事件类型（message_start/content_block_delta/message_stop）、工具调用使用 `input_schema` 而非 `parameters`、`MaxTokens` 必填且按模型分级。

## 验收标准
- [x] `ClaudeModel` 结构体实现 `Model` 接口：`Name()`、`Call()`、`CallStream()`、`SupportsTools()` 返回 true
- [x] `NewClaudeModel(cfg)` 构造：校验 `APIKey`，默认模型 `Claude35Sonnet`，默认 BaseURL `https://api.anthropic.com`，默认超时 120s
- [x] 认证头设置 `setHeaders()`：`x-api-key` + `anthropic-version: 2023-06-01`
- [x] API 端点：`POST {apiBase}/v1/messages`
- [x] 请求构建 `buildClaudeRequest()`：`claudeRequest` 结构体含 `Model`/`MaxTokens`/`Messages`/`System`/`Temperature`/`TopP`/`TopK`/`StopSequences`/`Tools`/`ToolChoice`/`Stream`
- [x] 系统提示词提取 `extractSystemPrompt()`：从消息列表中提取 role=system 消息，设置到 `req.System` 字段而非 messages 数组
- [x] 消息格式转换 `convertMessagesToClaude()`：`convertTextMessage()` → `claudeContent{text}`，`convertAssistantToolCallMessage()` → `tool_use` 块，`convertToolResultMessage()` → `tool_result` 块（role 改为 user）
- [x] 工具定义转换 `convertToolsToClaudeFormat()`：`ToolDefinition.Parameters` → `claudeTool.InputSchema`，仅处理 type=function
- [x] 响应解析 `convertToResponse()`：遍历 `claudeContentBlock`，text 块拼接 Content，tool_use 块转 `ToolCall`（input 序列化为 Arguments JSON）
- [x] MaxTokens 分级 `getMaxTokens()`：Claude 3 Opus 4096，Claude 3.5/3.7 8192，Claude 4 64000
- [x] 流式事件类型常量：`ClaudeEventMessageStart`/`ContentBlockStart`/`ContentBlockDelta`/`ContentBlockStop`/`MessageDelta`/`MessageStop`/`Error`
- [x] 流式处理 `readStream()`：bufio 逐行解析 SSE，`processStreamEvent()` 按事件类型分发
- [x] 流式工具调用累积：`toolUsesMap map[int]*ToolCall` 在 `handleContentBlockStart` 初始化，在 `handleContentBlockDelta`（input_json_delta）追加参数
- [x] 流式完成判断 `determineFinishReason()`：有工具调用 → "tool_calls"，否则使用 stopReason 或默认 "stop"
- [x] 模型常量：`Claude3Opus`/`Claude3Sonnet`/`Claude3Haiku`/`Claude35Sonnet`/`Claude35Haiku`/`Claude4Sonnet`/`Claude4Opus`
- [x] 代理配置：`cfg.Proxy` 通过 `http.Transport.Proxy` 设置
- [x] 错误处理：HTTP 非 200 时解析 `claudeErrorResponse`，流式 `ClaudeEventError` 发送 `StreamChunk.Error`
