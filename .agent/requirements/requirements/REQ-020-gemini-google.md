---
id: REQ-020
title: "Gemini Google 适配"
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
    context: "从代码逆向分析细化需求，来源: pkg/llm/gemini.go (670行)"
    reason: "逆向代码生成需求"
    snapshot: "Google Gemini API 独立适配，generateContent/streamGenerateContent 端点，FunctionDeclarations 工具格式，API Key 查询参数认证"
---

# Gemini Google 适配

## 描述
Google Generative AI (Gemini) API 的独立适配实现。Gemini API 不兼容 OpenAI 格式，需要完全独立实现 `GeminiModel` 结构体。核心差异包括：认证通过 URL 查询参数 `key=` 而非 Header、角色映射 user→user/assistant→model/tool→user、系统提示词使用 `systemInstruction` 字段、工具定义使用 `FunctionDeclarations`、同步端点 `generateContent` / 流式端点 `streamGenerateContent?alt=sse`、函数调用 ID 需自行生成。

## 验收标准
- [x] `GeminiModel` 结构体实现 `Model` 接口：`Name()`、`Call()`、`CallStream()`、`SupportsTools()` 返回 true
- [x] `NewGeminiModel(cfg)` 构造：校验 `APIKey` 和 `Model`，默认 BaseURL `https://generativelanguage.googleapis.com`，默认超时 120s
- [x] 认证方式：API Key 通过 URL 查询参数 `?key=` 传递，非 Bearer Token Header
- [x] 同步端点：`POST {apiBase}/v1beta/models/{model}:generateContent?key={apiKey}`
- [x] 流式端点：`POST {apiBase}/v1beta/models/{model}:streamGenerateContent?alt=sse&key={apiKey}`
- [x] API 版本：`GeminiAPIVersion = "v1beta"`
- [x] 请求构建 `buildGeminiRequest()`：`geminiRequest` 含 `Contents`/`SystemInstruction`/`SafetySettings`/`GenerationConfig`/`Tools`
- [x] 安全设置默认 `BLOCK_NONE`：`HARM_CATEGORY_HATE_SPEECH`/`HARASSMENT`/`SEXUALLY_EXPLICIT`/`DANGEROUS_CONTENT`
- [x] 系统指令 `setSystemInstruction()`：`systemInstruction` 字段为 `geminiContent{Parts: [{Text: prompt}]}`
- [x] 角色映射 `convertMessageToGeminiContent()`：`RoleUser`/`RoleTool` → "user"，`RoleAssistant` → "model"
- [x] 工具调用结果转换：`RoleTool` + `ToolCallID` → `geminiFunctionResponse{Name: msg.Name, Response: ...}`
- [x] 助手工具调用转换：`msg.ToolCalls` → `geminiFunctionCall{Name, Args}` 块，`Arguments` JSON 反序列化为 map
- [x] 生成配置 `setGenerationConfig()`：Temperature/TopP/MaxOutputTokens/StopSequences
- [x] 工具定义转换 `convertToolsToGeminiFormat()`：`ToolDefinition` → `geminiFunctionDeclaration{Name, Description, Parameters}`，包裹在 `geminiTool{FunctionDeclarations}` 中
- [x] 响应解析 `convertToResponse()`：遍历 `geminiCandidate.Content.Parts`，text → Content，functionCall → ToolCall（ID 自生成 `call_{timestamp}`）
- [x] Usage 转换：`geminiUsageMetadata.PromptTokenCount`/`CandidatesTokenCount`/`TotalTokenCount`
- [x] 流式处理 `readStream()`：SSE 格式解析，`processGeminiStreamData()` 处理每个响应帧
- [x] 流式函数调用 `handleFunctionCall()`：ID 格式 `call_{nano}_{index}`，立即发送 `ToolCallDelta`
- [x] FinishReason 映射：有工具调用 → "tool_calls"，否则小写 `candidate.FinishReason`
- [x] 代理配置：`cfg.Proxy` 通过 `http.Transport.Proxy` 设置
- [x] 错误处理：`geminiError.Code`/`Message`，HTTP 非 200 错误
