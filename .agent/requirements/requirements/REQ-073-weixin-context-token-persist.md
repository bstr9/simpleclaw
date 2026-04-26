---
id: REQ-073
title: "微信 contextToken 持久化"
status: completed
level: story
priority: P2
cluster: channel
created_at: "2026-04-26T22:00:00"
updated_at: "2026-04-27T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-004]
  merged_from: []
  depends_on: [REQ-069]
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T22:00:00"
    author: ai
    context: "参考 openclaw-weixin 的 context-token-store，SimpleClaw 重启后 contextToken 丢失导致无法回复"
    reason: "功能差距补齐"
    snapshot: "contextToken 落盘：变更时写入文件，启动时恢复，每账号独立存储"
source_code:
  - pkg/channel/weixin/weixin_channel.go
---

# 微信 contextToken 持久化

## 描述
contextToken 是微信消息回复的必要参数，标识当前会话上下文。当前仅保存在内存 map 中，进程重启后丢失，导致重启后无法回复用户消息（直到用户再次发送新消息）。需要持久化到磁盘。

## 验收标准
- [x] contextToken 变更时写入文件 — `pkg/channel/weixin/session.go:147-177` ContextTokenStore.Save
- [x] 启动时从文件加载 contextToken — `pkg/channel/weixin/session.go:180-204` ContextTokenStore.Load
- [x] 每账号独立 contextToken 文件：`{accountId}.context-tokens.json` — `resolveAccountFilePath()` session.go:28-30
- [x] 关闭时保存当前 contextToken 到文件 — `Stop()` weixin_channel.go
- [x] 文件写入失败时仅打印警告，不影响主流程

## 代码参考
| 验收标准 | 代码位置 |
|----------|----------|
| contextTokens map | `pkg/channel/weixin/weixin_channel.go:116` |
| 存储 contextToken | `pkg/channel/weixin/weixin_channel.go:660-664` |
| 获取 contextToken | `pkg/channel/weixin/weixin_channel.go:698-710` |
| relogin 清空 | `pkg/channel/weixin/weixin_channel.go:631-633` |
