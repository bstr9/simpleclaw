---
id: REQ-023
title: "LLM 工厂注册与提供商发现"
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
  depends_on: [REQ-018, REQ-019, REQ-020, REQ-021, REQ-022]
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/llm/factory.go"
    reason: "逆向代码生成需求"
    snapshot: "LLM 工厂模式注册与自动发现，providerFactories 映射表、detectProvider 智能推断、RegisterProvider 运行时扩展"
---

# LLM 工厂注册与提供商发现

## 描述
LLM 模型的工厂模式注册与自动提供商发现机制。`NewModel()` 作为统一入口，根据配置自动检测提供商类型并创建对应模型实例。支持从模型名称前缀（`provider/model`）和 API Base URL 两种方式推断提供商，并提供运行时扩展接口注册新提供商。

## 验收标准
- [x] 提供商常量定义（18个）：`ProviderOpenAI`/`ChatGPT`/`Anthropic`/`Gemini`/`DeepSeek`/`GLM`/`Qwen`/`MiniMax`/`Kimi`/`Moonshot`/`Zhipu`/`Baidu`/`Doubao`/`DashScope`/`ModelScope`/`LinkAI`/`Xunfei`
- [x] Base URL 映射表 `providerBaseURLs`：每个提供商的默认 API 地址
- [x] 工厂映射表 `providerFactories`：提供商名 → `modelFactory` 函数（16个注册项，GLM 和 Zhipu 共享工厂）
- [x] `NewModel(cfg ModelConfig)` 统一入口：校验 Model 和 APIKey 必填，调用 `detectProvider()` 自动检测
- [x] 提供商检测 `detectProvider(model, apiBase)`：
  - 模型名称前缀解析：`deepseek/deepseek-chat` → ProviderDeepSeek
  - API Base URL 匹配：遍历 `providerBaseURLs` 查找包含关系
  - 默认回退：ProviderOpenAI
- [x] 提供商前缀剥离 `stripProviderPrefix()`：`deepseek/chat` → `chat`
- [x] Moonshot/Kimi 特殊处理：检查 Provider 后单独设置 BaseURL 并调用 `NewMoonshotModel()`
- [x] 未注册提供商回退：Provider 不在 `providerFactories` 中时，设置默认 BaseURL 并创建 `OpenAIModel`
- [x] `NewModelWithProvider(provider, cfg)` 显式指定提供商创建实例
- [x] `RegisterProvider(name, baseURL)` 运行时注册新提供商及其 BaseURL
- [x] `GetProviderBaseURL(provider)` 查询已注册提供商的 BaseURL
- [x] `ListProviders()` 返回所有已注册提供商名称列表
- [x] `ModelConfig` 结构体字段：`ModelName`/`Model`/`APIBase`/`APIKey`/`SecretKey`/`Proxy`/`Provider`/`RequestTimeout`/`DefaultOptions`/`Extra`
- [x] `ModelInfo` 元数据结构体：`ID`/`Name`/`Provider`/`ContextWindow`/`SupportsVision`/`SupportsTools`/`SupportsStreaming`
