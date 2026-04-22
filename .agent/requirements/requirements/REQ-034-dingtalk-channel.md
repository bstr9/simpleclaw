---
id: REQ-034
title: "钉钉渠道"
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
    context: "从代码逆向分析细化需求，来源: pkg/channel/dingtalk/ 和 extensions/dingtalk/"
    reason: "逆向代码生成需求"
    snapshot: "钉钉渠道：Stream 模式消息接收、DingtalkTool 工具、技能路径注册"
---

# 钉钉渠道

## 描述
钉钉（Dingtalk）渠道实现，支持 Stream 模式接收消息和回调。扩展层注册 DingtalkTool 工具（含 send_message、get_user、get_department 操作）和技能路径。凭据可选，未配置时跳过渠道创建。

## 验收标准
- [x] DingtalkChannel 实现 channel.Channel 接口，嵌入 BaseChannel（pkg/channel/dingtalk/dingtalk_channel.go）
- [x] 消息结构定义：DingtalkMessage 及消息类型（pkg/channel/dingtalk/message.go）
- [x] Stream 模式：钉钉 Stream 协议长连接接收事件（pkg/channel/dingtalk/dingtalk_channel.go）
- [x] DingtalkExtension 注册：渠道 ChannelDingtalk、技能路径、DingtalkTool（extensions/dingtalk/extension.go:50-71）
- [x] DingtalkTool：操作 send_message、get_user、get_department，需 ClientID/ClientSecret 认证（extensions/dingtalk/tools/）
- [x] 可选工具注册：仅当 DingtalkClientID 和 DingtalkClientSecret 非空时注册 DingtalkTool（extension.go:62-67）
- [x] 可选渠道创建：仅当凭据存在时创建渠道实例，否则跳过并输出警告（extension.go:102-117）
- [x] 扩展生命周期：init() 注册 → Register（注册组件） → Startup → Shutdown（extensions/dingtalk/extension.go）
