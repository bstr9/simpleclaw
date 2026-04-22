---
id: REQ-025
title: "Tool 插件：工具注册、LLM 智能选择与内置工具"
status: active
level: story
priority: P0
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
    context: "从代码逆向分析细化需求，来源: pkg/plugin/tool/tool.go (1654行)"
    reason: "逆向代码生成需求"
    snapshot: "工具调用插件：全局注册表、LLM 智能选择工具、4 个内置工具（URL获取/天气/计算器/搜索）"
---

# Tool 插件：工具注册、LLM 智能选择与内置工具

## 描述
Tool 插件是插件系统中最复杂的组件（1654 行），提供工具调用框架，桥接 Agent Tool 系统。核心功能包括：全局工具注册表（支持 init 自动注册）、LLM 智能工具选择、4 个内置工具实现。插件通过 `TriggerPrefix+"tool"` 前缀触发，支持指定工具名执行或 LLM 自动路由。

## 验收标准
- [x] Tool 接口定义：`Name() string`、`Description() string`、`Run(query string, config map[string]any) (string, error)` (tool.go:35-44)
- [x] 全局工具注册表 `globalRegistry`：`toolRegistry` 结构体，含 `tools map[string]Tool` 和 `sync.RWMutex` (tool.go:130-139)
- [x] `RegisterTool(t Tool)` 全局注册函数，通过 `globalRegistry.Register(t)` 注册 (tool.go:142-144)
- [x] `toolRegistry.GetTool(name string) (Tool, bool)` 按名称查询注册表 (tool.go:154-159)
- [x] `init()` 函数自动注册 4 个内置工具：URLGetTool、MeteoTool、CalculatorTool、SearchTool (tool.go:1649)
- [x] ToolPlugin 结构体：嵌入 `BasePlugin`，持有 `config *Config`、`tools map[string]Tool`、`toolRegistry`、`llmModel llm.Model` (tool.go:116-124)
- [x] 配置结构体 `Config`：`Tools []string`、`ToolConfigs map[string]ToolConfig`、`Kwargs map[string]any`、`TriggerPrefix`（默认"$"）、`ThinkDepth`（默认2）、`RequestTimeout`（默认120）、`ModelName`（默认"gpt-3.5-turbo"）、`Temperature`、`LLMAPIKey`、`LLMAPIBase`、`BingSubscriptionKey`、`BingSearchURL`、`GoogleAPIKey`、`GoogleCSEID` (tool.go:56-96)
- [x] 单工具配置 `ToolConfig`：`Enabled bool`、`Config map[string]any` (tool.go:47-53)
- [x] `OnInit` 初始化流程：加载配置 → `initLLMModel()` → `loadTools()` → 校验 `len(p.config.Tools) > 0` (tool.go:188-221)
- [x] `initLLMModel()` 使用 `llm.NewModel(llm.ModelConfig{ModelName, APIKey, APIBase})` 初始化，APIBase 默认 `https://api.openai.com/v1` (tool.go:224-251)
- [x] `OnLoad` 注册 `EventOnHandleContext` 处理器 (tool.go:254-262)
- [x] 消息触发条件：`shouldHandleMessage` 检查 `TriggerPrefix+"tool"` 前缀 (tool.go:319-327)
- [x] 命令解析：`parseToolQuery` 从 subCmd 中匹配已加载工具名，提取 `toolName` 和 `query` (tool.go:355-375)
- [x] LLM 智能工具选择 `selectToolWithLLM`：`buildToolDescriptions` → `buildSystemPrompt` → `callLLMForToolSelection` → `tryToolByName` → 回退 `tryAllTools` (tool.go:426-442)
- [x] 系统提示词构建 `buildSystemPrompt`：列出可用工具名和描述，指示 LLM 回复工具名或 "unknown" (tool.go:456-462)
- [x] LLM 调用 `callLLMForToolSelection`：30 秒超时，Temperature=0，MaxTokens=50，返回清洗后的工具名 (tool.go:465-486)
- [x] 无 LLM 时降级策略：`executeTool` 遍历所有已加载工具，按顺序尝试 `tool.Run()` (tool.go:398-423)
- [x] 工具配置合并 `getToolConfig`：全局 Kwargs + 全局 API 配置 + 工具特定 `ToolConfigs[name].Config` (tool.go:539-569)
- [x] reset 命令：`handleResetCommand` 调用 `resetTools()`（重新加载工具） (tool.go:337-352)
- [x] 内置工具 URLGetTool：HTTP GET 获取 URL 内容，移除 HTML 标签，限制返回 4000 字符 (tool.go:706-791)
- [x] 内置工具 MeteoTool：地理编码 + 天气查询，支持当前天气和每日预报，`extractCity` 提取城市名 (tool.go:822-1081)
- [x] 内置工具 CalculatorTool：数学表达式求值，支持加减乘除、幂运算、三角函数、对数，`evaluateParentheses` 递归处理括号 (tool.go:1085-1475)
- [x] 内置工具 SearchTool：支持 Bing Search 和 Google Custom Search，`bingSearch` 和 `googleSearch` 双引擎 (tool.go:1479-1647)
