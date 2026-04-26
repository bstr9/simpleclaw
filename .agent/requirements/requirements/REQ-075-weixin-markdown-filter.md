---
id: REQ-075
title: "微信 Markdown 过滤"
status: active
level: story
priority: P3
cluster: channel
created_at: "2026-04-26T22:00:00"
updated_at: "2026-04-26T22:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-004]
  merged_from: []
  depends_on: []
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T22:00:00"
    author: ai
    context: "参考 openclaw-weixin 的 StreamingMarkdownFilter，微信不支持 markdown 渲染，需过滤"
    reason: "功能差距补齐"
    snapshot: "Markdown 过滤：发送前过滤 **bold** *italic* 等 markdown 语法，可选功能"
source_code:
  - pkg/channel/weixin/weixin_channel.go
---

# 微信 Markdown 过滤

## 描述
微信个人号不支持 markdown 渲染，但 AI 模型输出通常包含 markdown 语法（`**bold**`、`*italic*`、`` `code` ``、`### heading` 等）。发送前需要过滤这些语法，否则用户看到的是原始 markdown 标记。

## 验收标准
- [ ] Markdown 过滤器：移除 `**bold**`、`*italic*`、`` `code` ``、`### heading`、`- list`、`> quote` 等语法 — 参考 openclaw-weixin `src/messaging/markdown-filter.ts`
- [ ] 过滤在 sendText 前执行
- [ ] 通过配置开关控制是否启用（默认启用）
- [ ] 保留原始文本内容，仅移除格式标记
- [ ] 代码块（```...```）内容不过滤

## 代码参考
| 验收标准 | 代码位置 |
|----------|----------|
| sendText | `pkg/channel/weixin/weixin_channel.go:714-730` |
| Config 结构体 | `pkg/channel/weixin/weixin_channel.go:82-87` |
