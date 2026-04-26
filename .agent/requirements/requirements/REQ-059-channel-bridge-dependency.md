---
id: REQ-059
title: "反转 Channel→Bridge 依赖方向"
status: active
level: story
priority: P1
cluster: architecture
created_at: "2026-04-26T16:00:00"
updated_at: "2026-04-26T16:00:00"
relations:
  depends_on: [REQ-004, REQ-008]
  related_to: [REQ-056, REQ-057]
  refines: []
versions:
  - version: 1
    date: "2026-04-26T16:00:00"
    author: ai
    context: "架构评审发现 channel/manager.go 导入 pkg/bridge，依赖方向倒置（底层依赖上层）"
    reason: "架构评审 P1 级发现"
    snapshot: "反转 Channel→Bridge 依赖方向，消除 Service Locator 反模式"
---

# 反转 Channel→Bridge 依赖方向

## 描述
当前 `pkg/channel/manager.go` 导入 `pkg/bridge`，底层 Channel 模块依赖上层 Bridge 模块，依赖方向倒置。同时 `bridge.GetBridge()` 是 Service Locator 反模式，任何包都可以获取 Bridge 的全部能力，隐藏了真实依赖关系。

**具体问题**:
1. `channel/manager.go:11` 导入 `pkg/bridge`
2. `manager.go:297` 调用 `bridge.GetBridge().GetAgentBridge()` — Service Locator
3. `manager.go:350-398` 的 `bridgeMessageHandler` 直接调用 `bridge.GetBridge().FetchReplyContent()` / `bridge.GetBridge().FetchAgentReply()`

**推荐修复方案**:
1. 在 `channel` 包中定义 `MessageProcessor` 接口（只暴露 Channel 需要的方法）
2. 由 Bridge 层实现该接口并通过依赖注入传入 ChannelManager
3. 移除 channel 包对 bridge 包的导入
4. 将 `bridgeMessageHandler` 移到 bridge 包中

## 验收标准
- [ ] `pkg/channel/` 不再导入 `pkg/bridge/`
- [ ] Channel 通过接口接收消息处理器，而非主动获取 Bridge
- [ ] 依赖方向为 Bridge → Channel（上层依赖底层）
- [ ] 现有消息路由功能不受影响
- [ ] `go build ./...` 和 `go vet ./...` 通过
