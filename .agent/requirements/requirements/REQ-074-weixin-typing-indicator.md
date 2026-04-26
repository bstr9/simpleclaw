---
id: REQ-074
title: "微信 typing 指示器"
status: completed
level: story
priority: P3
cluster: channel
created_at: "2026-04-26T22:00:00"
updated_at: "2026-04-27T12:00:00"
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
    context: "参考 openclaw-weixin 的 typing 指示器，SimpleClaw 完全无此功能"
    reason: "功能差距补齐"
    snapshot: "typing 指示器：getConfig 获取 typing_ticket + sendTyping 发送/取消输入状态"
source_code:
  - pkg/channel/weixin/weixin_channel.go
---

# 微信 typing 指示器

## 描述
微信 API 支持 typing 指示器，在 AI 生成回复期间向用户显示"正在输入"状态。当前 SimpleClaw 完全未实现此功能。

## 验收标准
- [x] weixinAPI 增加 `getConfig(ilinkUserId, contextToken)` 方法 — `pkg/channel/weixin/api.go`
- [x] weixinAPI 增加 `sendTyping(ilinkUserId, typingTicket, status)` 方法 — `pkg/channel/weixin/api.go`
- [x] typing_ticket 缓存：避免每次消息都调用 getConfig — `pkg/channel/weixin/typing.go` TypingTicketCache (24h TTL)
- [x] 消息处理前调用 sendTyping(status=1) 开始输入指示 — `processMessage()` weixin_channel.go
- [x] 消息回复后调用 sendTyping(status=2) 取消输入指示 — `TypingController.StartTyping()` 返回 stop 函数
- [x] typing 失败不影响主流程（仅打印警告）— 所有 typing 错误仅 Debug 日志

## 代码参考
| 验收标准 | 代码位置 |
|----------|----------|
| weixinAPI 结构体 | `pkg/channel/weixin/weixin_channel.go:947-952` |
| processMessage | `pkg/channel/weixin/weixin_channel.go:638-696` |
| sendText | `pkg/channel/weixin/weixin_channel.go:714-730` |
