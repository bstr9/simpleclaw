---
id: REQ-069
title: "微信多账号支持"
status: active
level: story
priority: P1
cluster: channel
created_at: "2026-04-26T22:00:00"
updated_at: "2026-04-26T22:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-004]
  merged_from: []
  depends_on: []
  refined_by: [REQ-070, REQ-071, REQ-072, REQ-073]
  related_to: [REQ-055]
versions:
  - version: 1
    date: "2026-04-26T22:00:00"
    author: ai
    context: "参考 Tencent/openclaw-weixin 实现，发现 SimpleClaw 微信渠道仅支持单账号，需补齐多账号能力"
    reason: "功能差距补齐"
    snapshot: "微信多账号：accounts 索引 + 每账号凭证 + 多账号轮询 + outbound 路由"
source_code:
  - pkg/channel/weixin/weixin_channel.go
  - extensions/weixin/extension.go
---

# 微信多账号支持

## 描述
参考腾讯官方 openclaw-weixin 的多账号架构，为 SimpleClaw 微信渠道添加多账号支持。当前仅支持单账号（单个 credentials.json），需要改为 accounts 索引 + 每账号独立凭证，支持多个微信账号同时在线。

## 验收标准
- [ ] 账号索引文件：`accounts.json` 存储已注册账号 ID 列表 — 参考 openclaw-weixin `src/auth/accounts.ts:49-60`
- [ ] 每账号凭证文件：`accounts/{accountId}.json` 存储 token/baseUrl/userId — 参考 openclaw-weixin `src/auth/accounts.ts:113-120`
- [ ] 账号注册：QR 登录成功后自动注册到索引 — 参考 openclaw-weixin `src/auth/accounts.ts:63-72`
- [ ] 账号解析：`resolveWeixinAccount()` 合并配置+凭证 — 参考 openclaw-weixin `src/auth/accounts.ts:355-380`
- [ ] 多账号轮询：每个账号独立 pollLoop goroutine
- [ ] contextToken 按账号隔离：每账号独立的 contextToken map
- [ ] outbound 路由：根据 contextToken 或 accountId 选择发送账号 — 参考 openclaw-weixin `src/channel.ts:63-104`
- [ ] 凭证文件权限：0600，防止其他用户读取 token
- [ ] 过期账号清理：同一 userId 的旧账号自动清理 — 参考 openclaw-weixin `src/auth/accounts.ts:90-107`

## 代码参考
| 验收标准 | 代码位置 |
|----------|----------|
| 账号索引 | `extensions/weixin/accounts.go` (新建) |
| 每账号凭证 | `extensions/weixin/accounts.go` (新建) |
| 多账号轮询 | `extensions/weixin/extension.go` Startup |
| outbound 路由 | `pkg/channel/weixin/weixin_channel.go` Send |
