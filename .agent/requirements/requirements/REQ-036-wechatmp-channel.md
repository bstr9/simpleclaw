---
id: REQ-036
title: "微信公众号渠道"
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
    context: "从代码逆向分析细化需求，来源: pkg/channel/wechatmp/ 和 extensions/wechatmp/"
    reason: "逆向代码生成需求"
    snapshot: "微信公众号渠道：消息验证、被动回复、WechatMP 工具、技能路径注册"
---

# 微信公众号渠道

## 描述
微信公众号（WeChat MP）渠道实现，支持消息签名验证和被动回复。扩展层注册 WechatmpChannel、技能路径和可选 WechatMPTool（需 AppID/AppSecret 凭据）。未配置凭据时跳过渠道创建。

## 验收标准
- [x] WechatmpChannel 实现 channel.Channel 接口，嵌入 BaseChannel（pkg/channel/wechatmp/wechatmp_channel.go）
- [x] 消息验证：微信公众号签名验证机制，确保消息来源合法（pkg/channel/wechatmp/）
- [x] 被动回复：微信被动消息回复，支持文本/图片/图文等类型（pkg/channel/wechatmp/）
- [x] WechatMPExtension 注册：渠道 ChannelWechatMP、技能路径、可选 WechatMPTool（extensions/wechatmp/extension.go:50-71）
- [x] WechatMPTool：需 WechatmpAppID/WechatmpAppSecret 认证，仅凭据存在时注册（extension.go:62-67）
- [x] 可选渠道创建：仅当 WechatmpAppID/WechatmpAppSecret 非空时创建渠道，否则跳过并输出警告（extension.go:102-117）
- [x] 技能路径注册：扩展目录下 wechatmp/skills（extension.go:59-60）
- [x] 扩展生命周期：init() 注册 → Register → Startup → Shutdown（extensions/wechatmp/extension.go）
