---
id: REQ-068
title: "linkai.go God Object 拆分"
status: active
level: task
priority: P0
cluster: architecture
created_at: "2026-04-26T19:00:00"
updated_at: "2026-04-26T19:00:00"
relations:
  depends_on: [REQ-008]
  related_to: [REQ-067]
versions:
  - version: 1
    date: "2026-04-26T19:00:00"
    author: ai
    context: "第二轮架构修复：linkai.go 1945行 God Object，职责过多"
    reason: "第二轮架构修复"
    snapshot: "linkai.go 拆分为 4 个文件：linkai.go(454行) + knowledge.go(120行) + midjourney.go(712行) + summary.go(710行)"
---

# linkai.go God Object 拆分

## 描述
`pkg/plugin/linkai/linkai.go` 为 1945 行的 God Object，混合了插件核心、知识库管理、Midjourney 图片生成和对话摘要功能，职责过多。

**拆分方案**:
- `linkai.go` (454行) — LinkAIPlugin struct, 生命周期方法, API 客户端, 群组管理
- `knowledge.go` (120行) — 知识库 CRUD, 搜索, 索引
- `midjourney.go` (712行) — Midjourney 图片生成, 任务管理, 回调处理
- `summary.go` (710行) — 对话摘要, 文件摘要, URL 摘要

## 验收标准
- [ ] linkai.go 不超过 500 行
- [ ] 知识库逻辑独立到 knowledge.go
- [ ] Midjourney 逻辑独立到 midjourney.go
- [ ] 摘要逻辑独立到 summary.go
- [ ] `go build` 和 `go vet` 通过
- [ ] 无行为变更，无函数签名变更
