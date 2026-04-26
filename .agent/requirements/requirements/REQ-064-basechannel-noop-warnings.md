---
id: REQ-064
title: "BaseChannel 空操作添加日志警告"
status: active
level: task
priority: P2
cluster: architecture
created_at: "2026-04-26T19:00:00"
updated_at: "2026-04-26T19:00:00"
relations:
  depends_on: [REQ-008]
  related_to: [REQ-061]
versions:
  - version: 1
    date: "2026-04-26T19:00:00"
    author: ai
    context: "第二轮架构修复：BaseChannel.Send/Startup 为空操作，子类忘记实现时无任何提示"
    reason: "第二轮架构修复"
    snapshot: "BaseChannel.Send/Startup 添加 logger.Warn 警告，子类未实现时可见"
---

# BaseChannel 空操作添加日志警告

## 描述
`BaseChannel.Send()` 和 `BaseChannel.Startup()` 是空操作（no-op），当子类忘记实现这些方法时，不会有任何错误或警告提示，导致运行时行为难以排查。

**修复方案**:
- `Send()` 添加 `logger.Warn("Send 方法未实现")`
- `Startup()` 添加 `logger.Warn("Startup 方法未实现")`
- 不改变返回值或函数签名

## 验收标准
- [ ] `BaseChannel.Send()` 添加 `logger.Warn`
- [ ] `BaseChannel.Startup()` 添加 `logger.Warn`
- [ ] 不改变函数签名或返回值
