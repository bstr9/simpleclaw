---
id: REQ-066
title: "PluginManager.PublishEvent 移除多余 RLock"
status: active
level: task
priority: P2
cluster: architecture
created_at: "2026-04-26T19:00:00"
updated_at: "2026-04-26T19:00:00"
relations:
  depends_on: [REQ-008]
  related_to: [REQ-057]
versions:
  - version: 1
    date: "2026-04-26T19:00:00"
    author: ai
    context: "第二轮架构修复：PublishEvent 外层 RLock 与 ListEnabledPlugins 内部锁冗余"
    reason: "第二轮架构修复"
    snapshot: "PublishEvent 移除外层冗余 RLock，ListEnabledPlugins 已自带锁保护"
---

# PluginManager.PublishEvent 移除多余 RLock

## 描述
`PluginManager.PublishEvent()` 中外层 `RLock` 与 `ListEnabledPlugins()` 内部的锁冗余，可能导致不必要的锁竞争。

**修复方案**:
- 移除 `PublishEvent()` 外层的 `mu.RLock()/mu.RUnlock()`
- `ListEnabledPlugins()` 已自带锁保护，无需外层加锁

## 验收标准
- [ ] `PublishEvent()` 外层 RLock 已移除
- [ ] `ListEnabledPlugins()` 仍自带锁保护
- [ ] 并发安全不受影响
