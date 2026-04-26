---
id: REQ-067
title: "tool.go God Object 拆分"
status: active
level: task
priority: P0
cluster: architecture
created_at: "2026-04-26T19:00:00"
updated_at: "2026-04-26T19:00:00"
relations:
  depends_on: [REQ-008]
  related_to: [REQ-068]
versions:
  - version: 1
    date: "2026-04-26T19:00:00"
    author: ai
    context: "第二轮架构修复：tool.go 1654行 God Object，职责过多"
    reason: "第二轮架构修复"
    snapshot: "tool.go 拆分为 4 个文件：tool.go(411行) + registry.go(38行) + executor.go(199行) + builtin.go(908行)"
---

# tool.go God Object 拆分

## 描述
`pkg/plugin/tool/tool.go` 为 1654 行的 God Object，混合了插件核心逻辑、工具注册、工具执行和内置工具定义，职责过多。

**拆分方案**:
- `tool.go` (411行) — ToolPlugin struct, Config, 生命周期方法, 消息处理, 配置加载
- `registry.go` (38行) — Tool 接口, toolRegistry struct, RegisterTool, GetTool
- `executor.go` (199行) — executeTool, selectToolWithLLM, tryToolByName, tryAllTools, getToolConfig, prompt 构建
- `builtin.go` (908行) — URLGetTool, MeteoTool, CalculatorTool, SearchTool + init()

## 验收标准
- [ ] tool.go 不超过 500 行
- [ ] 工具注册逻辑独立到 registry.go
- [ ] 工具执行逻辑独立到 executor.go
- [ ] 内置工具定义独立到 builtin.go
- [ ] `go build` 和 `go vet` 通过
- [ ] 无行为变更，无函数签名变更
