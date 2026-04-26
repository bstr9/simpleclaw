---
id: REQ-058
title: "修复 Scheduler 全局状态并发安全"
status: active
level: task
priority: P0
cluster: scheduler
created_at: "2026-04-26T16:00:00"
updated_at: "2026-04-26T16:00:00"
relations:
  depends_on: [REQ-017]
  related_to: [REQ-056, REQ-057]
versions:
  - version: 1
    date: "2026-04-26T16:00:00"
    author: ai
    context: "架构评审发现 scheduler 全局状态使用 sync.Once + bool 的不当组合，globalSchedulerSet 无同步保护"
    reason: "架构评审 P0 级发现"
    snapshot: "修复 Scheduler 全局单例的并发安全问题，替换 sync.Once + bool 为 sync.Mutex 模式"
---

# 修复 Scheduler 全局状态并发安全

## 描述
`pkg/scheduler/scheduler.go` 的全局单例使用 `sync.Once` + `bool` 组合存在两个问题：

1. **`sync.Once` 语义错误**：`SetScheduler()` 使用 `sync.Once`，意味着只能设置一次。如果 `GetScheduler()` 先运行（创建默认实例），`SetScheduler()` 会被静默忽略，无法注入自定义调度器。

2. **`globalSchedulerSet` 无同步保护**：`SetScheduler()` 中写入 `globalSchedulerSet = true`，`GetScheduler()` 中无锁读取 `if !globalSchedulerSet`，构成数据竞争。

**问题代码** (`scheduler.go:321-341`):
```go
var globalScheduler *Scheduler
var globalSchedulerMu sync.Once     // ❌ 误用作互斥锁
var globalSchedulerSet bool         // ❌ 无同步保护

func SetScheduler(s *Scheduler) {
    globalSchedulerMu.Do(func() {
        globalScheduler = s
        globalSchedulerSet = true  // ❌ 无锁写
    })
}
```

**推荐修复方案**: 使用 `sync.Mutex` 替代 `sync.Once`，移除 `globalSchedulerSet` bool，参照 `config.go` 的 `sync.RWMutex` + `Load()`/`Set()` 模式。

## 验收标准
- [ ] 替换 `sync.Once` 为 `sync.Mutex` 或 `sync.RWMutex`
- [ ] 移除 `globalSchedulerSet` bool，改用 nil 检查
- [ ] `SetScheduler()` 可多次调用（后调用覆盖前值）
- [ ] `GetScheduler()` 与 `SetScheduler()` 并发安全
- [ ] 并发调用不触发 `go test -race` 报警
