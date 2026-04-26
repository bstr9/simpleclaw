---
id: REQ-005
title: "LLM 提供商适配层"
status: active
level: epic
priority: P0
cluster: llm
created_at: "2026-04-23T10:00:00"
updated_at: "2026-04-26T10:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: []
  merged_from: []
  depends_on: []
  related_to: [REQ-001, REQ-004]
  refined_by: [REQ-018, REQ-019, REQ-020, REQ-021, REQ-022, REQ-023, REQ-024]
versions:
  - version: 1
    date: "2026-04-23T10:00:00"
    author: ai
    context: "从代码逆向分析提取需求，来源: pkg/llm/"
    reason: "逆向代码生成需求"
    snapshot: "15+ LLM 提供商统一适配，支持同步/流式调用、Function Calling、多模态输入"
  - version: 2
    date: "2026-04-26T10:00:00"
    author: ai
    context: "需求审查确认 Epic 级别不需要精确提供商计数"
    reason: "与 REQ-023 计数对齐审查，保持 15+ 表述不变（Epic 级别允许模糊计数）"
    snapshot: "15+ LLM 提供商统一适配，Epic 级别计数保持不变"
---

# LLM 提供商适配层

## 描述
统一 Model 接口的 LLM 提供商适配层，支持 15+ 个 LLM 提供商。OpenAI 兼容 API 为基础实现，特殊提供商（Claude、Gemini、讯飞）独立实现。支持同步调用、流式输出、Function Calling、多模态输入等能力。

## 验收标准
- [x] Model 统一接口：Name()、Call()、CallStream()、SupportsTools()
- [x] 统一消息格式：Role（system/user/assistant/tool）、ToolCall、ToolCallDelta
- [x] CallOptions 配置：Temperature、MaxTokens、TopP、Tools、ToolChoice、SystemPrompt、ResponseFormat 等
- [x] 流式输出：StreamChunk 通道，支持增量文本和增量工具调用
- [x] OpenAI 兼容提供商复用：DeepSeek、Qwen 等复用 OpenAI 实现
- [x] Claude 适配：Anthropic API，流式 + 多模态
- [x] Gemini 适配：Google Generative AI API
- [x] DashScope 适配：通义千问，多模态 + 流式
- [x] Zhipu 适配：GLM 系列
- [x] Minimax 适配：独立 API
- [x] Baidu 适配：文心一言
- [x] Xunfei 适配：WebSocket API
- [x] Moonshot 适配：Kimi
- [x] Doubao 适配：字节豆包
- [x] ModelScope 适配：阿里 ModelScope
- [x] LinkAI 适配：LinkAI 集成
- [x] ModelConfig：API Key、Proxy、Provider、RequestTimeout、DefaultOptions
- [x] ModelInfo 元数据：ContextWindow、SupportsVision、SupportsTools、SupportsStreaming
- [x] 工厂模式注册：init() 中 RegisterProvider()，按配置创建实例
