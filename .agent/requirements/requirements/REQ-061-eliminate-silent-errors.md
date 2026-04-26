---
id: REQ-061
title: "消除静默吞错"
status: active
level: task
priority: P1
cluster: architecture
created_at: "2026-04-26T16:00:00"
updated_at: "2026-04-26T16:00:00"
relations:
  depends_on: [REQ-008]
  related_to: [REQ-006]
versions:
  - version: 1
    date: "2026-04-26T16:00:00"
    author: ai
    context: "架构评审发现多处关键操作错误被静默丢弃，导致问题难以排查"
    reason: "架构评审 P1 级发现"
    snapshot: "消除 session/infra/plugin 层的静默吞错，改为日志记录或错误返回"
---

# 消除静默吞错

## 描述
多处关键操作的错误被 `_ =` 静默丢弃，导致运行时问题难以排查。主要位置：

1. **`bridge/agent_bridge_session.go:28`**: `_ = ab.sessionMgr.DeleteSession(...)` — 会话删除失败被静默丢弃
2. **`bridge/agent_bridge_session.go:32`**: `_ = ab.memoryMgr.ClearSession(...)` — 记忆清理失败被静默丢弃
3. **`bridge/agent_bridge_session.go:105`**: `_ = ab.sessionMgr.Close()` — 会话管理器关闭失败被静默丢弃
4. **`plugin/plugin.go:306-307`**: `LoadConfig` 中 `json.Unmarshal` 错误被 `if err == nil` 静默忽略 — 配置损坏无感知
5. **`channel/manager.go:456`**: `_ = h.pluginMgr.PublishEvent(...)` — 事件发布失败被静默丢弃

**推荐修复方案**:
- 对关键操作（会话删除、记忆清理、管理器关闭）：改为 `logger.Warn` 记录错误
- 对 `LoadConfig` 的 JSON 解析错误：`logger.Warn` 记录并返回错误，而非静默跳过
- 对事件发布失败：`logger.Debug` 记录（非关键路径）

## 验收标准
- [ ] 所有 `_ =` 丢弃错误的地方改为日志记录或返回错误
- [ ] `LoadConfig` JSON 解析失败不再静默跳过
- [ ] 会话/记忆/关闭操作失败有日志记录
- [ ] 不改变函数签名（不破坏现有调用方）
