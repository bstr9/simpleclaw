---
id: REQ-004
title: "多渠道接入系统"
status: active
level: epic
priority: P0
cluster: channels
created_at: "2026-04-23T10:00:00"
updated_at: "2026-04-22T16:13:03"
relations:
  supersedes: []
  conflicts_with: []
  refines: []
  merged_from: []
  depends_on: []
  refined_by: [REQ-014, REQ-015, REQ-033, REQ-034, REQ-035, REQ-036, REQ-037, REQ-038]
  related_to: [REQ-005, REQ-007]
versions:
  - version: 1
    date: "2026-04-23T10:00:00"
    author: ai
    context: "从代码逆向分析提取需求，来源: pkg/channel/ 和 extensions/"
    reason: "逆向代码生成需求"
    snapshot: "9 个消息渠道：Terminal、Web、飞书、钉钉、微信、公众号、微信客服、企微机器人、QQ"
  - version: 2
    date: "2026-04-22T16:13:03"
    author: ai
    context: "元数据自动同步"
    reason: "自动补充反向关系: refined_by"
    snapshot: "自动同步元数据"
---

# 多渠道接入系统

## 描述
统一 Channel 接口的消息渠道系统，支持 9 个渠道同时运行。每个渠道实现 Startup/Stop/Send 生命周期，通过工厂模式自动注册。渠道管理器支持多渠道并行启动、主渠道选举、优雅关闭。

## 验收标准
- [x] Channel 统一接口：Startup(ctx)、Stop()、Send(reply, ctx)、ChannelType()、Name()、UserID()
- [x] BaseChannel 基础实现：线程安全的状态管理、云模式、回复类型过滤
- [x] 工厂模式注册：init() 中调用 RegisterChannel()，按名称查找和创建
- [x] ChannelManager：多渠道启动/停止、主渠道选举、渠道查找
- [x] Terminal 渠道：命令行交互模式
- [x] Web 渠道：WebSocket + SSE，支持前端 UI 嵌入
- [x] 飞书渠道：事件订阅、消息收发、卡片交互
- [x] 钉钉渠道：Stream 模式、消息回调、机器人交互
- [x] 微信渠道：扫码登录、联系人管理、消息收发、群聊
- [x] 公众号渠道（wechatmp）：消息验证、被动回复
- [x] 微信客服渠道（wechatcom）：客服消息系统
- [x] 企微机器人渠道（wecombot）：Webhook 消息推送
- [x] QQ 渠道：WebSocket 连接、消息收发
- [x] 扩展系统（extensions/）：渠道从 pkg/channel 迁移到 extensions/，通过 extension.Manager 加载
