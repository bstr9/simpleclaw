---
id: REQ-038
title: "企微机器人渠道"
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
    context: "从代码逆向分析细化需求，来源: pkg/channel/wecombot/ 和 extensions/wecombot/"
    reason: "逆向代码生成需求"
    snapshot: "企微机器人渠道：Webhook 消息推送"
---

# 企微机器人渠道

## 描述
企业微信群机器人（WeCom Bot）渠道实现，通过 Webhook 推送消息到企微群。扩展层注册 WecomBotChannel，无额外工具和技能注册。最轻量的渠道实现，仅需 Webhook 地址即可发送消息。

## 验收标准
- [x] WecomBotChannel 实现 channel.Channel 接口，嵌入 BaseChannel（pkg/channel/wecombot/wecombot_channel.go）
- [x] Webhook 消息推送：通过企业微信群机器人 Webhook URL 推送消息（pkg/channel/wecombot/）
- [x] 消息类型：支持文本、Markdown、图片、图文、文件等消息格式（pkg/channel/wecombot/）
- [x] WecomBotExtension 注册：渠道 ChannelWecomBot，无额外工具/技能（extensions/wecombot/extension.go:47-58）
- [x] 渠道创建：NewWecomBotChannel(nil) 直接创建实例（extension.go:80-89）
- [x] 扩展生命周期：init() 注册 → Register（注册渠道） → Startup → Shutdown（extensions/wecombot/extension.go）
