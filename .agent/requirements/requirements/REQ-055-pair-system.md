---
id: REQ-055
title: "Pair 用户配对系统"
status: active
level: story
priority: P1
cluster: bridge
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T21:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-008]
  merged_from: []
  depends_on: [REQ-004]
  refined_by: []
  related_to: [REQ-014, REQ-033]
versions:
  - version: 5
    date: "2026-04-26T21:00:00"
    author: ai
    context: "残留技术债务清理：移除微信扩展无效 PairManager 集成，WeixinProvider/WeixinChannel 预留接口添加文档"
    reason: "Oracle 残留债务处理"
    snapshot: "用户配对系统：微信扩展不再启动 PairManager，WeixinProvider 方法标注为预留接口"
  - version: 4
    date: "2026-04-26T20:00:00"
    author: ai
    context: "Oracle 验证后深度修复：飞书/微信扩展集成 PairManager，添加 Pair 配置项，消除所有 deadcode"
    reason: "完整集成验证"
    snapshot: "用户配对系统：15/15已实现，4个缺陷全部修复，飞书/微信扩展均已集成"
  - version: 3
    date: "2026-04-26T19:00:00"
    author: ai
    context: "修复3个已知缺陷：CleanExpired调度、FeishuProvider重复常量、WeixinProvider集成"
    reason: "代码缺陷修复后更新验收标准"
    snapshot: "用户配对系统：15/15已实现，3个缺陷已修复"
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
  - extensions/feishu/extension.go
  - extensions/weixin/extension.go
  - pkg/config/config.go
---

# Pair 用户配对系统

## 描述
用户配对（Pair）系统，管理渠道用户与系统会话之间的绑定关系。通过 OAuth 认证流程将渠道用户（如飞书、微信）与系统会话配对，实现跨会话的用户身份识别。支持多渠道 Provider 插件化注册、SQLite 持久化存储、过期清理。

## 验收标准
- [x] Provider 接口：ChannelType()/StartPair(userID)/CheckStatus(userID)/RequiredScopes()/IsUserAuthorized(userID) — `pkg/pair/manager.go:12-18`
- [x] Manager 配对管理器：RegisterProvider/GetProvider/CheckSessionPair/StartPair/CompletePair/GetUserAuth/GetSessionPair/StartCleanupLoop/Close — `pkg/pair/manager.go:33-171`
- [x] 配对状态三态：StatusPendingPair(pending)/StatusActive(active)/StatusExpired(expired) — `pkg/pair/types.go:5-9`
- [x] PairStatus 结构体：Paired(bool)/Status/AuthURL/ExpiresAt/Name/OpenID — `pkg/pair/types.go:11-18`
- [x] UserAuth 结构体：UserID/ChannelType/Token/RefreshToken/Scopes/GrantedAt/ExpiresAt/Name — `pkg/pair/types.go:20-29`
- [x] SessionPair 结构体：SessionID/UserID/ChannelType/Status/PairedAt/ExpiresAt — `pkg/pair/types.go:31-38`
- [x] PairRequest 结构体：SessionID/UserID/ChannelType — `pkg/pair/types.go:40-44`
- [x] PairResult 结构体：Success/AuthURL/Message — `pkg/pair/types.go:46-50`
- [x] SQLite 持久化存储（modernc.org/sqlite，无 CGO）：user_auths 表 + session_pairs 表 + user 索引 — `pkg/pair/store.go:13,52-72`
- [x] Store CRUD：SaveUserAuth/GetUserAuth/SaveSessionPair/GetSessionPair/DeleteUserAuth/DeleteSessionPair — `pkg/pair/store.go:86-192`
- [x] 过期清理：CleanExpired(ctx) 删除过期的 session_pairs 和 user_auths — `pkg/pair/store.go:194-207`，调度调用方为 `Manager.StartCleanupLoop()` `pkg/pair/manager.go:30-52`，飞书扩展在 `initPairManager()` 中启动 `extensions/feishu/extension.go:163`
- [x] 飞书 Provider：lark-cli 设备码认证流程，config init → auth login --no-wait → auth status — `pkg/pair/providers/feishu.go:56-115`
- [x] 微信 Provider：函数注入模式（SetLoginStatusFunc/SetQRURLFunc），默认已配对 — `pkg/pair/providers/weixin.go:18-24`，已集成到微信扩展 `extensions/weixin/extension.go:106-157`
- [x] 会话配对检查流程：先查 SessionPair → 有效则返回 Active → 再查 UserAuth → 有效则自动创建 SessionPair → 否则返回 PendingPair — `pkg/pair/manager.go:40-76`
- [x] 并发安全：Manager 使用 sync.RWMutex，Store 使用 sync.RWMutex — `manager.go:21`, `store.go:17`

## 已知缺陷
1. ~~**CleanExpired 无调度调用**~~：已修复，`Manager.StartCleanupLoop()` 在飞书/微信扩展 Startup 时启动
2. ~~**微信 Provider 未集成**~~：已修复，`extensions/weixin/extension.go` 完整集成 PairManager + WeixinProvider
3. ~~**飞书 Provider 重复常量**~~：已修复，改用 `pair.StatusXxx` 包级常量
4. ~~**无 Pair 配置项**~~：已修复，`pkg/config/config.go` 添加 `PairEnabled`/`PairCleanupInterval` 字段

## 已知限制

### WeixinProvider 为功能桩（Functional Stub）

微信个人号的配对机制与飞书 OAuth 模式本质不同：

- **飞书**：使用 OAuth 设备码认证流程（`lark-cli auth login`），用户需在浏览器中授权，`CheckStatus()` 会真实查询授权状态
- **微信个人号**：基于扫码登录（QR Code），无 OAuth 授权流程，用户扫码即登录成功，无需额外的配对步骤

因此 `WeixinProvider` 的当前实现为功能桩：

| 方法 | 行为 | 原因 |
|------|------|------|
| `CheckStatus(userID)` | 固定返回 `Paired:true, Status:Active` | 微信个人号登录即使用，无需配对验证 |
| `StartPair(userID)` | 返回空字符串 `""`，无 error | 微信无 OAuth 授权链接可提供 |
| `IsUserAuthorized(userID)` | 固定返回 `true` | 微信个人号登录后即已授权 |
| `GetLoginStatus()` | 通过函数注入获取渠道登录状态 | 预留接口，供未来 QR 登录状态查询使用 |
| `GetCurrentQRURL()` | 通过函数注入获取渠道二维码 URL | 预留接口，供未来 QR 登录流程使用 |

**影响**：
- 微信渠道的 `PairManager.StartCleanupLoop()` 会持续运行清理循环，但由于所有 `CheckStatus()` 调用均返回 `Paired:true`，`session_pairs` 表中不会有过期数据需要清理，属于资源浪费
- `SetLoginStatusFunc`/`SetQRURLFunc` 注入已连接但当前未被 Pair 流程调用
- 微信消息处理流程中未调用 `CheckSessionPair()`（飞书在 `websocket.go:186` 调用），因为微信不需要配对验证

**未来改进方向**：
1. 微信渠道可跳过 PairManager 初始化，因为 Provider 始终返回已配对状态
2. 若未来微信接入 OAuth 授权（如微信公众平台），需将 `CheckStatus()` 改为真实查询逻辑
3. `GetLoginStatus()`/`GetCurrentQRURL()` 可用于构建"微信登录状态监控"功能
