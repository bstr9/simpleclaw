---
id: REQ-072
title: "微信 getUpdatesBuf 持久化"
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
    context: "参考 openclaw-weixin 的 sync-buf 持久化，SimpleClaw 重启后 getUpdatesBuf 丢失导致重复消息"
    reason: "功能差距补齐"
    snapshot: "getUpdatesBuf 落盘：每次成功轮询后写入文件，启动时从文件恢复"
source_code:
  - pkg/channel/weixin/weixin_channel.go
---

# 微信 getUpdatesBuf 持久化

## 描述
getUpdatesBuf 是微信长轮询的同步游标，决定从哪个时间点开始接收新消息。当前仅保存在内存中，进程重启后丢失，导致重启后可能重复收到旧消息。需要持久化到磁盘。

## 验收标准
- [x] 每次 getUpdates 成功后保存 buf 到文件 — `pkg/channel/weixin/session.go:55-85` SyncBufStore.Save
- [x] 启动时从文件加载 buf — `pkg/channel/weixin/session.go:88-107` SyncBufStore.Load
- [x] re-login 后清空 buf 文件（从空字符串开始）— `pkg/channel/weixin/session.go:110-122` SyncBufStore.Clear
- [x] 每账号独立 buf 文件：`{accountId}.sync.json` — `resolveAccountFilePath()` session.go:28-30
- [x] 文件写入失败时仅打印警告，不影响主流程

## 代码参考
| 验收标准 | 代码位置 |
|----------|----------|
| updateSyncCursor | `pkg/channel/weixin/weixin_channel.go:606-610` |
| getUpdatesBuf 字段 | `pkg/channel/weixin/weixin_channel.go:113` |
| relogin 清空 | `pkg/channel/weixin/weixin_channel.go:572` |
