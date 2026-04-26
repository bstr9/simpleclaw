---
id: REQ-070
title: "微信 IDC 重定向处理"
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
    context: "参考 Tencent/openclaw-weixin 的 IDC 重定向机制，SimpleClaw 未处理 scaned_but_redirect 状态"
    reason: "功能差距补齐"
    snapshot: "QR 登录时 IDC 重定向：scaned_but_redirect → 动态切换 API base URL"
source_code:
  - pkg/channel/weixin/weixin_channel.go
---

# 微信 IDC 重定向处理

## 描述
微信 QR 登录轮询中，服务器可能返回 `scaned_but_redirect` 状态，表示需要切换到另一个 IDC 节点。当前 SimpleClaw 的 `handleQRStatus()` 不处理此状态，可能导致登录失败。

## 验收标准
- [x] `qrStatusResponse` 增加 `RedirectHost string` 字段 — `pkg/channel/weixin/api.go:108`
- [x] `handleQRStatus()` 增加 `case "scaned_but_redirect":` 分支 — `pkg/channel/weixin/weixin_channel.go` handleQRStatus
- [x] QR 轮询使用动态 base URL：当收到 redirect_host 时切换当前 API 请求地址 — `weixinAPI.updateBaseURL()` api.go
- [x] 重定向日志记录：记录 IDC 切换事件
- [x] 缺少 redirect_host 时继续使用当前 URL（不中断登录）
- [x] getUpdates 响应中的 `baseurl` 字段处理：运行时 IDC 重定向 — `pollLoop()` weixin_channel.go

## 代码参考
| 验收标准 | 代码位置 |
|----------|----------|
| qrStatusResponse | `pkg/channel/weixin/weixin_channel.go:989-995` |
| handleQRStatus | `pkg/channel/weixin/weixin_channel.go:428-445` |
| QR 轮询循环 | `pkg/channel/weixin/weixin_channel.go:338-386` |
