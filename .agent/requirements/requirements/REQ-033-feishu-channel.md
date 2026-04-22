---
id: REQ-033
title: "飞书渠道"
status: active
level: story
priority: P0
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
    context: "从代码逆向分析细化需求，来源: pkg/channel/feishu/ 和 extensions/feishu/"
    reason: "逆向代码生成需求"
    snapshot: "飞书渠道：双模式事件接收（WebSocket/Webhook）、消息收发、流式卡片、lark-cli 工具、Pair 配对"
---

# 飞书渠道

## 描述
飞书（Lark/Feishu）渠道实现，支持 WebSocket 长连接和 Webhook 两种事件接收模式。提供文本/图片/富文本/文件/视频/卡片消息收发、流式卡片输出、Typing 表情、消息去重、访问令牌管理、事件加密解密。扩展层集成 lark-cli 工具和 19 个官方技能，支持 Pair 配对认证。

## 验收标准
- [x] FeishuChannel 实现 channel.Channel 接口：Startup/Stop/Send，嵌入 BaseChannel（feishu_channel.go:79-110）
- [x] Config 配置：AppID、AppSecret、VerificationToken、EncryptKey、Port（默认 9891）、EventMode（webhook/websocket）、BotName、GroupSharedSession、StreamOutput（feishu_channel.go:66-76）
- [x] 双事件模式：WebSocket 模式（WSClient 长连接）和 Webhook 模式（HTTP 服务器），按 EventMode 切换（feishu_channel.go:220-225）
- [x] FeishuEvent 事件信封：AppID、Type、Header（EventID/EventType/CreateTime/Token）、Event（Sender+Message）、Schema、Challenge（message.go:35-42）
- [x] 消息类型解析：Text（文本）、Image（图片）、Post（富文本含 Markdown）、File（文件）、Audio、Video（message.go:17-24, 240-253）
- [x] 群聊/私聊区分：ChatType（p2p/group），群聊消息去 @占位符，SessionID 按群共享或用户+群区分（message.go:29-32, 430-438）
- [x] 消息发送：支持文本（Post+Markdown）、图片（上传获取 image_key）、文件（上传获取 file_key）、视频（上传获取 file_key）、卡片（interactive）（feishu_channel.go:439-478）
- [x] 流式卡片：StreamingCardController，节流更新（500ms），状态机（Idle→Creating→Streaming→Completed），buildStreamingCard 构建卡片 JSON（streaming_card.go:31-238）
- [x] 流式消息：SendStreamMessage/UpdateStreamMessage 直接发送和更新文本消息（feishu_channel.go:546-654）
- [x] Typing 表情：SendTypingReaction/RemoveTypingReaction 防止飞书超时重发（feishu_channel.go:342-436）
- [x] 访问令牌管理：getAccessToken 双重检查锁，自动刷新，2 小时过期（feishu_channel.go:769-834）
- [x] 消息去重：processedMsgs 缓存 + 定期清理（7 小时过期），isMessageProcessed/markMessageProcessed（feishu_channel.go:1121-1155）
- [x] 事件加密解密：DecryptEvent，AES-256-CBC + PKCS7 Unpad（feishu_channel.go:1179-1209）
- [x] 扩展注册：FeishuExtension 注册渠道 "feishu"、技能目录、lark_cli 工具（extensions/feishu/extension.go:70-97）
- [x] lark-cli 工具：LarkCLITool 封装 200+ 命令，与渠道共享 AppID/AppSecret，支持 install/update/status 操作（extensions/feishu/tools/lark_cli.go）
- [x] Pair 配对管理：PairManager 接口（CheckSessionPair/StartPair/CompletePair），PairManagerAdapter 适配（feishu_channel.go:112-131, extension.go:337-404）
- [x] 后台更新检查：backgroundUpdateCheck 每 24 小时检查 lark-cli 版本更新（extension.go:254-292）
