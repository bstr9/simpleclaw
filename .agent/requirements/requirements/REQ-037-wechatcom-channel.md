---
id: REQ-037
title: "微信客服渠道"
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
    context: "从代码逆向分析细化需求，来源: pkg/channel/wechatcom/ 和 extensions/wechatcom/"
    reason: "逆向代码生成需求"
    snapshot: "微信客服渠道：客服消息系统、WechatCom 工具、技能路径注册"
---

# 微信客服渠道

## 描述
企业微信客服（WeChat Customer Service）渠道实现，支持客服消息收发。扩展层注册 WechatcomChannel、技能路径和可选 WechatComTool（需 CorpID 凭据）。未配置凭据时跳过渠道创建。

## 验收标准
- [x] WechatcomChannel 实现 channel.Channel 接口，嵌入 BaseChannel（pkg/channel/wechatcom/wechatcom_channel.go）
- [x] 客服消息系统：微信客服消息接收和回复（pkg/channel/wechatcom/）
- [x] WechatComExtension 注册：渠道 ChannelWechatComApp、技能路径、可选 WechatComTool（extensions/wechatcom/extension.go:50-71）
- [x] WechatComTool：需 WecomCorpID 认证，仅 CorpID 非空时注册（extension.go:62-67）
- [x] 可选渠道创建：仅当 WecomCorpID/WecomSecret 非空时创建渠道，否则跳过并输出警告（extension.go:102-117）
- [x] 技能路径注册：扩展目录下 wechatcom/skills（extension.go:59-60）
- [x] 扩展生命周期：init() 注册 → Register → Startup → Shutdown（extensions/wechatcom/extension.go）
