---
id: REQ-024
title: "LLM 工具调用（Function Calling）跨提供商适配"
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
  depends_on: [REQ-018, REQ-019, REQ-020, REQ-021]
  related_to: [REQ-009]
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/llm/model.go, openai.go, claude.go, gemini.go, dashscope.go"
    reason: "逆向代码生成需求"
    snapshot: "跨提供商统一的 Function Calling 适配，ToolDefinition/ToolCall/ToolCallDelta 标准类型，各提供商独立格式转换和流式增量解析"
---

# LLM 工具调用（Function Calling）跨提供商适配

## 描述
LLM Function Calling 的跨提供商统一适配层。定义标准类型（`ToolDefinition`/`ToolCall`/`ToolCallDelta`），各提供商独立实现格式转换：OpenAI 兼容提供商直接映射、Claude 使用 `tool_use`/`tool_result` 内容块、Gemini 使用 `FunctionCall`/`FunctionResponse`、DashScope 使用自定义工具格式。流式场景下各提供商分别实现增量工具调用解析。

## 验收标准

### 标准类型定义（model.go）
- [x] `ToolDefinition`：`Type`（固定 "function"）+ `Function{Name, Description, Parameters(any)}`
- [x] `ToolCall`：`ID` + `Type` + `Function{Name, Arguments(JSON string)}`
- [x] `ToolCallDelta`：`Index` + `ID` + `Type` + `Function{Name, Arguments}`（流式增量）
- [x] `FunctionCall`：`Name` + `Arguments`（完整参数 JSON）
- [x] `FunctionCallDelta`：`Name` + `Arguments`（增量参数片段）
- [x] `CallOptions.Tools`：`[]ToolDefinition` 工具定义列表
- [x] `CallOptions.ToolChoice`：`any` 类型，支持 "auto"/"none"/"required"/指定工具
- [x] `Message.ToolCalls`：助手消息中的工具调用列表
- [x] `Message.ToolCallID`：工具结果消息的对应调用 ID
- [x] `RoleTool`：工具结果消息角色
- [x] `Response.ToolCalls`：响应中的工具调用列表
- [x] `StreamChunk.ToolCallDelta`：流式增量工具调用
- [x] `Model.SupportsTools()`：声明模型是否支持工具调用

### OpenAI 兼容格式转换
- [x] `applyToolsToRequest()`：`ToolDefinition` → `openai.Tool{Type, Function{Name, Description, Parameters}}`
- [x] `ToolChoice` 传递：string 直接设置，map 直接传递
- [x] 响应解析：`choice.Message.ToolCalls` → `[]ToolCall{ID, Type, Function{Name, Arguments}}`
- [x] 流式增量 `handleStreamToolCall()`：`toolCallsIndex` 索引映射，`ToolCallDelta` 累积

### Claude 格式转换
- [x] `convertToolsToClaudeFormat()`：`ToolDefinition.Parameters` → `claudeTool.InputSchema`，仅处理 type=function
- [x] 请求格式：`claudeRequest.Tools = []claudeTool{Name, Description, InputSchema}`
- [x] 响应解析：`tool_use` 内容块 → `ToolCall{ID, Type:"function", Function{Name, Arguments(input→JSON)}}`
- [x] 工具结果消息：`RoleTool` + `ToolCallID` → `claudeContent{Type:"tool_result", ToolUseID, Content}`
- [x] 助手工具调用：`msg.ToolCalls` → `claudeContent{Type:"tool_use", ID, Name, Input}`
- [x] 流式累积：`toolUsesMap map[int]*ToolCall`，`content_block_start` 初始化，`input_json_delta` 追加 Arguments

### Gemini 格式转换
- [x] `convertToolsToGeminiFormat()`：`ToolDefinition` → `geminiFunctionDeclaration{Name, Description, Parameters}`，包裹在 `geminiTool{FunctionDeclarations}`
- [x] 请求格式：`geminiPart.FunctionCall{Name, Args(map)}`
- [x] 响应解析：`geminiFunctionCall` → `ToolCall{ID(自生成), Type:"function", Function{Name, Arguments(Args→JSON)}}`
- [x] 工具结果消息：`RoleTool` + `ToolCallID` → `geminiFunctionResponse{Name: msg.Name, Response: ...}`，角色映射为 user
- [x] 流式处理：函数调用在 `processCandidateParts()` 中直接发送 `ToolCallDelta`

### DashScope 格式转换
- [x] `dashScopeTool`：`Type` + `Function{Name, Description, Parameters}`
- [x] 请求格式：`dashScopeParameters.Tools` + `ToolChoice`
- [x] 响应解析：`dashScopeMessage.ToolCalls` → `[]ToolCall{ID, Type, Function{Name, Arguments}}`
- [x] 流式工具调用：`sendToolCalls()` 发送完整 `ToolCallDelta`

### 能力声明
- [x] 各模型 `SupportsTools()` 返回值：OpenAI(true)、Claude(true)、Gemini(true)、DashScope(按 ModelInfo)、Xunfei(false)、Baidu(按模型)、其余兼容提供商(true)
