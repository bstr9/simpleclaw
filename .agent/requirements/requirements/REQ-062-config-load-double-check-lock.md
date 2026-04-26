---
id: REQ-062
title: "config.Load() 移除 sync.Once 改为双重检查锁"
status: active
level: task
priority: P1
cluster: architecture
created_at: "2026-04-26T19:00:00"
updated_at: "2026-04-26T19:00:00"
relations:
  depends_on: [REQ-056]
  related_to: [REQ-056, REQ-060]
versions:
  - version: 1
    date: "2026-04-26T19:00:00"
    author: ai
    context: "第二轮架构修复：config.Load() 使用 sync.Once 不支持不同路径重新加载"
    reason: "第二轮架构修复"
    snapshot: "config.Load() 移除 sync.Once，改为双重检查锁，允许不同路径加载配置"
---

# config.Load() 移除 sync.Once 改为双重检查锁

## 描述
`config.Load()` 使用 `sync.Once` 限制配置只加载一次，但实际场景中可能需要用不同路径重新加载配置。应改为双重检查锁模式，与 `config.Get()` 的修复方式保持一致。

**修复方案**:
- 移除 `cfgOnce sync.Once`
- `Load()` 改为双重检查锁：RLock 快速路径检查是否已加载，Lock 慢路径执行加载
- 允许不同路径重新加载（相同路径跳过）

## 验收标准
- [ ] `cfgOnce sync.Once` 已移除
- [ ] `Load()` 使用双重检查锁
- [ ] 不同路径可重新加载配置
- [ ] 相同路径跳过重复加载
- [ ] 并发安全
