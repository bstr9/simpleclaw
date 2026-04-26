---
id: REQ-055
title: "Pair 用户配对系统"
status: active
level: story
priority: P1
cluster: bridge
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T18:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-008]
  merged_from: []
  depends_on: [REQ-004]
  refined_by: []
  related_to: [REQ-014, REQ-033]
versions:
  - version: 2
    date: "2026-04-26T18:00:00"
    author: ai
    context: "代码审查发现14/15标准已实现，仅CleanExpired无调用方"
    reason: "验收标准代码验证"
    snapshot: "用户配对系统：14/15已实现，CleanExpired已编码但无调度调用"
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求，Oracle 验证发现 pkg/pair/ 无 REQ 覆盖"
    reason: "审查发现缺失的需求文档"
    snapshot: "用户配对系统：OAuth 认证配对、Session 绑定、SQLite 持久化、飞书/微信 Provider"
source_code:
  - pkg/pair/types.go
  - pkg/pair/manager.go
  - pkg/pair/store.go
  - pkg/pair/providers/feishu.go
  - pkg/pair/providers/weixin.go
---

# Pair 用户配对系统

## 描述
用户配对（Pair）系统，管理渠道用户与系统会话之间的绑定关系。通过 OAuth 认证流程将渠道用户（如飞书、微信）与系统会话配对，实现跨会话的用户身份识别。支持多渠道 Provider 插件化注册、SQLite 持久化存储、过期清理。

## 验收标准
- [x] Provider 接口：ChannelType()/StartPair(userID)/CheckStatus(userID)/RequiredScopes()/IsUserAuthorized(userID) — `pkg/pair/manager.go:12-18`
- [x] Manager 配对管理器：RegisterProvider/CheckSessionPair/StartPair/CompletePair/GetUserAuth/GetSessionPair/Close — `pkg/pair/manager.go:33-171`
- [x] 配对状态三态：StatusPendingPair(pending)/StatusActive(active)/StatusExpired(expired) — `pkg/pair/types.go:5-9`
- [x] PairStatus 结构体：Paired(bool)/Status/AuthURL/ExpiresAt/Name/OpenID — `pkg/pair/types.go:11-18`
- [x] UserAuth 结构体：UserID/ChannelType/Token/RefreshToken/Scopes/GrantedAt/ExpiresAt/Name — `pkg/pair/types.go:20-29`
- [x] SessionPair 结构体：SessionID/UserID/ChannelType/Status/PairedAt/ExpiresAt — `pkg/pair/types.go:31-38`
- [x] PairRequest 结构体：SessionID/UserID/ChannelType — `pkg/pair/types.go:40-44`
- [x] PairResult 结构体：Success/AuthURL/Message — `pkg/pair/types.go:46-50`
- [x] SQLite 持久化存储（modernc.org/sqlite，无 CGO）：user_auths 表 + session_pairs 表 + user 索引 — `pkg/pair/store.go:13,52-72`
- [x] Store CRUD：SaveUserAuth/GetUserAuth/SaveSessionPair/GetSessionPair/DeleteUserAuth/DeleteSessionPair — `pkg/pair/store.go:86-192`
- [ ] 过期清理：CleanExpired(ctx) 删除过期的 session_pairs 和 user_auths — 已编码(`store.go:194-207`)但无调度调用方，过期记录会无限累积
- [x] 飞书 Provider：lark-cli 设备码认证流程，config init → auth login --no-wait → auth status — `pkg/pair/providers/feishu.go:56-115`
- [x] 微信 Provider：函数注入模式（SetLoginStatusFunc/SetQRURLFunc），默认已配对 — `pkg/pair/providers/weixin.go:18-24`（已编码但未集成到微信渠道）
- [x] 会话配对检查流程：先查 SessionPair → 有效则返回 Active → 再查 UserAuth → 有效则自动创建 SessionPair → 否则返回 PendingPair — `pkg/pair/manager.go:40-76`
- [x] 并发安全：Manager 使用 sync.RWMutex，Store 使用 sync.RWMutex — `manager.go:21`, `store.go:17`

## 已知缺陷
1. **CleanExpired 无调度调用**：`Store.CleanExpired()` 已实现但全代码库无调用方，建议在 `Manager` 或 `Extension.Startup()` 中添加定时清理协程
2. **微信 Provider 未集成**：`providers/weixin.go` 已编码但从未注册到微信渠道，`StartPair` 返回空字符串(no-op)
3. **飞书 Provider 重复常量**：`providers/feishu.go:20-24` 重新声明了 `StatusPendingPair/StatusActive/StatusExpired`，应引用 `pair.StatusPendingPair`
4. **无 Pair 配置项**：`pkg/config/` 中缺少 pair_enabled/cleanup_interval 等配置字段
