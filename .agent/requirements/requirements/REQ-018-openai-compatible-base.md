---
id: REQ-018
title: "OpenAI 兼容基础实现"
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
  depends_on: []
  related_to: [REQ-019, REQ-020, REQ-021, REQ-022]
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/llm/openai.go, chatgpt.go, deepseek.go, qwen.go"
    reason: "逆向代码生成需求"
    snapshot: "OpenAI 兼容 API 基础客户端，被 6+ 提供商复用，支持同步/流式调用、工具调用、代理和超时配置"
---

# OpenAI 兼容基础实现

## 描述
OpenAI 兼容 API 的基础客户端实现，作为 LLM 提供商适配层的基石。`OpenAIModel` 结构体实现 `Model` 接口，提供同步调用（`Call`）和流式调用（`CallStream`），支持 Function Calling、自定义 API Base URL、HTTP 代理和请求超时。6+ 个 OpenAI 兼容提供商（DeepSeek、Qwen、Moonshot、Doubao、Zhipu、ModelScope、LinkAI、ChatGPT）通过嵌入 `OpenAIModel` 复用此实现。

## 验收标准
- [x] `OpenAIModel` 结构体实现 `Model` 接口：`Name()`、`Call()`、`CallStream()`、`SupportsTools()`
- [x] `NewOpenAIModel(cfg ModelConfig)` 构造函数：校验 `APIKey` 和 `Model` 必填，基于 `openai.DefaultConfig` 创建客户端
- [x] 自定义 API Base URL：`cfg.APIBase` 设置 `config.BaseURL`，支持 OpenAI 兼容提供商接入
- [x] HTTP 代理配置：`cfg.Proxy` 通过 `http.Transport.Proxy` 设置代理
- [x] 请求超时：`cfg.RequestTimeout` 设置 `httpClient.Timeout`
- [x] 同步调用 `Call()`：合并 `DefaultOptions` 与传入 `Option`，`buildRequest()` 构建请求，`CreateChatCompletion()` 执行
- [x] 流式调用 `CallStream()`：`CreateChatCompletionStream()` 发起，`processStream()` 在 goroutine 中读取并写入 `chan StreamChunk`
- [x] 流式工具调用增量：`handleStreamToolCall()` 通过 `toolCallsIndex` 映射索引，发送 `ToolCallDelta`
- [x] 请求构建 `buildRequest()`：`applyBasicOptions()` 应用 Temperature/MaxTokens/TopP/Stop/FrequencyPenalty/PresencePenalty/User
- [x] 工具定义注入：`applyToolsToRequest()` 将 `ToolDefinition` 转换为 `openai.Tool`，支持 `ToolChoice` 配置
- [x] 响应格式控制：`applyResponseFormat()` 支持 string 和 map 两种 `ResponseFormat`
- [x] 系统提示词注入：`applySystemPrompt()` 在消息首条非 system 时前置插入 system 消息
- [x] 消息格式转换：`convertMessages()` 将 `llm.Message` 转为 `openai.ChatCompletionMessage`，含 ToolCalls 和 ToolCallID
- [x] Usage 转换：`convertUsage()` 将 `openai.Usage` 转为 `llm.Usage`
- [x] ChatGPT 变体：`ChatGPTModel` 嵌入 `OpenAIModel`，默认模型 `gpt-3.5-turbo`，`isChatGPTModelSupportsTools()` 排除 `gpt-3.5-turbo-instruct`
- [x] DeepSeek 变体：`DeepSeekModel` 嵌入 `OpenAIModel`，默认 BaseURL `https://api.deepseek.com/v1`，默认模型 `deepseek-chat`
- [x] Qwen 变体：`QwenModel` 嵌入 `OpenAIModel`，使用 DashScope 兼容模式 BaseURL `https://dashscope.aliyuncs.com/compatible-mode/v1`，含模型名称别名映射 `mapQwenModel()`
