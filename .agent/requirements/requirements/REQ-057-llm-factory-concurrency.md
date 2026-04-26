---
id: REQ-057
title: "修复 LLM 工厂并发安全"
status: active
level: task
priority: P0
cluster: llm
created_at: "2026-04-26T16:00:00"
updated_at: "2026-04-26T16:00:00"
relations:
  depends_on: [REQ-005, REQ-023]
  related_to: [REQ-056, REQ-058]
versions:
  - version: 1
    date: "2026-04-26T16:00:00"
    author: ai
    context: "架构评审发现 llm.RegisterProvider() 无锁写入 providerBaseURLs map，与 NewModel() 读取可并发发生"
    reason: "架构评审 P0 级发现"
    snapshot: "修复 LLM 工厂 providerBaseURLs 和 providerFactories 的并发安全问题"
---

# 修复 LLM 工厂并发安全

## 描述
`pkg/llm/factory.go` 中 `providerBaseURLs` 和 `providerFactories` 是普通 `map`，没有 `sync.RWMutex` 保护。`RegisterProvider()` 无锁写入 `providerBaseURLs`，与 `NewModel()` / `detectProvider()` / `GetProviderBaseURL()` / `ListProviders()` 的读取可并发发生，构成数据竞争。

**对比**: Channel factory (`pkg/channel/factory.go`) 和 Voice factory (`pkg/voice/factory.go`) 都已正确使用 `sync.RWMutex` 保护。

**问题代码** (`factory.go:32, 157-159`):
```go
var providerBaseURLs = map[string]string{}  // 无 mutex

func RegisterProvider(name, baseURL string) {
    providerBaseURLs[strings.ToLower(name)] = baseURL  // ❌ 无锁写入
}
```

**推荐修复方案**: 添加 `sync.RWMutex` 保护，参照 Channel/Voice factory 的实现模式。

## 验收标准
- [ ] `providerBaseURLs` 和 `providerFactories` 所有读写操作受 `sync.RWMutex` 保护
- [ ] `RegisterProvider()` 加写锁
- [ ] `NewModel()` / `detectProvider()` / `GetProviderBaseURL()` / `ListProviders()` 加读锁
- [ ] 并发调用不触发 `go test -race` 报警
- [ ] 与 Channel/Voice factory 的并发保护模式一致
