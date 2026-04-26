---
id: REQ-071
title: "微信会话暂停与恢复"
status: active
level: story
priority: P2
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
    context: "参考 Tencent/openclaw-weixin 的 session-guard 机制，SimpleClaw 会话过期后立即 re-login 是错误策略"
    reason: "功能差距补齐"
    snapshot: "会话过期暂停：errcode -14 → 暂停所有请求 → 定时自动恢复"
source_code:
  - pkg/channel/weixin/weixin_channel.go
---

# 微信会话暂停与恢复

## 描述
当微信 API 返回 errcode -14（会话过期）时，当前 SimpleClaw 立即尝试 re-login（删除凭证+QR 扫码），这会导致不必要的交互中断。正确做法是暂停一段时间后自动恢复，因为会话过期可能是暂时性的。

## 验收标准
- [ ] 会话暂停机制：errcode -14 时暂停所有 API 请求，而非立即 re-login — 参考 openclaw-weixin `src/api/session-guard.ts`
- [ ] 暂停时长：可配置，默认若干分钟 — 参考 openclaw-weixin `src/monitor/monitor.ts:114-126`
- [ ] 暂停期间跳过所有 getUpdates/sendMessage 调用
- [ ] 暂停到期后自动恢复轮询，无需重新 QR 登录
- [ ] 连续失败退避：3 次连续失败后 backoff 30 秒 — 参考 openclaw-weixin `src/monitor/monitor.ts:135-146`
- [ ] 区分暂时性过期和永久失效：暂停恢复后仍失败才触发 re-login

## 代码参考
| 验收标准 | 代码位置 |
|----------|----------|
| handleSessionExpired | `pkg/channel/weixin/weixin_channel.go:568-578` |
| relogin | `pkg/channel/weixin/weixin_channel.go:612-636` |
| handlePollError | `pkg/channel/weixin/weixin_channel.go:552-560` |
| applyRetryDelay | `pkg/channel/weixin/weixin_channel.go:597-603` |
