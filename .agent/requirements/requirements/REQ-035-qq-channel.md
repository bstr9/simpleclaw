---
id: REQ-035
title: "QQ 渠道"
status: active
level: story
priority: P1
cluster: channels
created_at: "2026-04-23T16:00:00"
updated_at: "2026-04-23T16:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-004]
  merged_from: []
  depends_on: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/channel/qq/ 和 extensions/qq/"
    reason: "逆向代码生成需求"
    snapshot: "QQ 渠道：WebSocket 连接接收消息、消息收发"
---

# QQ 渠道

## 描述
QQ 渠道实现，通过 WebSocket 长连接接收 QQ 消息。扩展层注册 QQExtension，创建 QQChannel 实例。无额外工具和技能注册，仅提供基础消息收发能力。

## 验收标准
- [x] QQChannel 实现 channel.Channel 接口，嵌入 BaseChannel（pkg/channel/qq/qq_channel.go）
- [x] WebSocket 连接：长连接接收 QQ 消息事件（pkg/channel/qq/qq_channel.go）
- [x] 消息收发：接收 QQ 消息并转换为统一 ChatMessage，发送回复（pkg/channel/qq/qq_channel.go）
- [x] QQExtension 注册：渠道 ChannelQQ，无额外工具/技能（extensions/qq/extension.go:47-58）
- [x] 扩展生命周期：init() 注册 → Register（注册渠道） → Startup → Shutdown（extensions/qq/extension.go:15-90）
- [x] 渠道创建：NewQQChannel() 直接创建实例，无条件跳过（extension.go:80-89）
