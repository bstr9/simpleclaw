---
id: REQ-022
title: "国内 OpenAI 兼容提供商适配（智谱/MiniMax/Moonshot/豆包/ModelScope）"
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
  related_to: [REQ-021]
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/llm/zhipu.go, minimax.go, moonshot.go, doubao.go, modelscope.go"
    reason: "逆向代码生成需求"
    snapshot: "5 个国内 OpenAI 兼容提供商适配，均嵌入 OpenAIModel 复用基础能力，各自添加模型信息表、特有参数和功能扩展"
---

# 国内 OpenAI 兼容提供商适配（智谱/MiniMax/Moonshot/豆包/ModelScope）

## 描述
5 个国内 LLM 提供商的适配实现，均兼容 OpenAI Chat Completions API 格式，通过嵌入 `*OpenAIModel` 复用基础调用能力。每个提供商在基础之上添加各自的模型信息表、默认配置、名称映射和特有功能扩展。

## 验收标准

### 智谱 AI（Zhipu/GLM）
- [x] `ZhipuModel` 嵌入 `*OpenAIModel`，委托 `Call()`/`CallStream()` 给 OpenAI 实现
- [x] 默认 BaseURL `https://open.bigmodel.cn/api/paas/v4`，默认模型 `glm-4-flash`
- [x] 模型信息表 `zhipuModels`：glm-4（128K，支持视觉+工具）、glm-4-flash（128K）、glm-4-plus（128K，视觉+工具）、glm-4-long（1M context）、glm-4v（视觉，不支持工具）、glm-4.7-thinking（131K，视觉+工具）、glm-5（131K，视觉+工具）、glm-z1-air/airx/flash
- [x] `SupportsTools()` 按模型信息表查询，默认返回 true
- [x] `GetModelInfo()` / `ListZhipuModels()` 导出模型元数据

### MiniMax
- [x] `MiniMaxModel` 使用独立 `openai.Client`（非嵌入），自定义 `MiniMaxExtraConfig`
- [x] 默认 BaseURL `https://api.minimax.chat/v1`，默认模型 `MiniMax-M2.1`
- [x] 特有功能 `CallWithReasoning()`：`reasoning_split` 参数分离思考过程，返回 `MiniMaxReasoningResponse{Content, Reasoning, ToolCalls, Usage}`
- [x] `MiniMaxReasoningDetail` 结构体：ID/Index/Text 描述思考过程
- [x] `MiniMaxExtraConfig`：`ReasoningSplit`（默认启用）+ `ShowThinking`
- [x] 独立请求构建 `buildMiniMaxRequestBody()`：添加 `reasoning_split` 字段
- [x] 独立 HTTP 请求发送 `sendMiniMaxRequest()`：直接构造 HTTP 请求而非使用 openai 库

### Moonshot（Kimi）
- [x] `MoonshotModel` 嵌入 `*OpenAIModel`，完全委托调用
- [x] 默认 BaseURL `https://api.moonshot.cn/v1`
- [x] `SupportsTools()` 返回 true
- [x] Provider 标识 `ProviderMoonshot`

### 豆包（Doubao/字节火山方舟）
- [x] `DoubaoModel` 嵌入 `*OpenAIModel`，委托 `Call()`/`CallStream()`
- [x] 默认 BaseURL `https://ark.cn-beijing.volces.com/api/v3`
- [x] 模型名使用 endpoint ID（如 `ep-xxxx-xxxx`），非模型名称
- [x] `SupportsTools()` 返回 true
- [x] 支持 thinking 模式：可通过 Extra 参数 `thinking.type: "disabled"` 控制

### ModelScope（魔搭社区）
- [x] `ModelScopeModel` 嵌入 `*OpenAIModel`，委托调用
- [x] 默认 BaseURL `https://api-inference.modelscope.cn/v1`，默认模型 `Qwen/Qwen2.5-7B-Instruct`
- [x] 模型标识符使用 `Owner/Model` 格式（如 `Qwen/Qwen2.5-7B-Instruct`、`deepseek-ai/DeepSeek-V3`）
- [x] 模型信息表 `modelscopeModelInfo`：Qwen2.5 系列（7B/14B/32B/72B，32K context）、QwQ-32B（不支持工具）、DeepSeek-V3/R1
- [x] `SupportsTools()` 按模型信息表查询，默认 true
- [x] `GetModelInfo()` / `ListModelScopeModels()` / `GetModelScopeModelInfo()` 导出元数据
