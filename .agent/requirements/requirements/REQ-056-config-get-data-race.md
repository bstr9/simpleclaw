---
id: REQ-056
title: "修复 config.Get() 数据竞争"
status: active
level: task
priority: P0
cluster: config
created_at: "2026-04-26T16:00:00"
updated_at: "2026-04-26T16:00:00"
relations:
  depends_on: [REQ-054]
  related_to: [REQ-057, REQ-058]
versions:
  - version: 1
    date: "2026-04-26T16:00:00"
    author: ai
    context: "架构评审发现 config.Get() 在 RLock 下写入 cfg，构成数据竞争"
    reason: "架构评审 P0 级发现"
    snapshot: "修复 config.Get() 中 RLock 下写入 cfg 的数据竞争问题"
---

# 修复 config.Get() 数据竞争

## 描述
`pkg/config/config.go` 的 `Get()` 函数在 `RLock` 保护下检测 `cfg == nil` 并写入默认配置，这是教科书级的数据竞争：两个 goroutine 同时调用 `Get()` 且 `cfg == nil` 时，会并发写入同一变量。

**当前问题代码** (`config.go:345-353`):
```go
func Get() *Config {
    cfgMu.RLock()           // 读锁
    defer cfgMu.RUnlock()
    if cfg == nil {
        cfg = getDefaultConfig()  // ❌ 读锁下写入 — 数据竞争
    }
    return cfg
}
```

**推荐修复方案**: 使用 `sync.Once` + 写锁升级模式，或改用 `atomic.Pointer[Config]`（Go 1.19+）实现零读锁竞争的原子指针交换。

## 验收标准
- [ ] `Get()` 不在读锁下执行写入操作
- [ ] 并发调用 `Get()` 不会触发 `go test -race` 报警
- [ ] `Load()` / `Reload()` / `Set()` / `Get()` 整体并发安全
- [ ] 现有调用方无需修改
