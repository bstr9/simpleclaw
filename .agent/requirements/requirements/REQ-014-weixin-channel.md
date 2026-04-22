---
id: REQ-014
title: "微信渠道接入"
status: active
level: story
priority: P0
cluster: channels
created_at: "2026-04-23T10:10:00"
updated_at: "2026-04-23T18:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-004]
  merged_from: []
  depends_on: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T10:10:00"
    author: ai
    context: "从 REQ-004 拆分，来源: extensions/weixin/"
    reason: "Epic 拆分为 Story"
    snapshot: "微信个人号渠道：扫码登录、联系人管理、消息收发、群聊支持"
  - version: 2
    date: "2026-04-23T18:00:00"
    author: ai
    context: "逆向代码分析，扩展验收标准至代码级细节"
    reason: "代码逆向补充详细验收标准"
    snapshot: "微信个人号 ilink bot 协议渠道：QR 登录（凭证持久化）、长轮询消息接收、去重、多类型消息收发、CDN 加密上传下载、会话过期重登录"
source_code:
  - pkg/channel/weixin/weixin_channel.go
  - pkg/channel/weixin/message.go
---

# 微信渠道接入

## 描述
微信个人号渠道，基于 ilink bot 协议实现。支持二维码扫码登录（凭证持久化）、长轮询消息接收与去重、文本/图片/文件/视频/语音消息收发、CDN 加密媒体上传下载、会话过期自动重登录。是复杂度最高的渠道之一（1281 行核心 + 501 行消息处理）。

## 验收标准
- [x] WeixinChannel 嵌入 BaseChannel，渠道名称 "weixin"，实现 Channel 接口（Startup/Stop/Send）
- [x] 配置结构 Config：BaseURL（默认 https://ilinkai.weixin.qq.com）、CDNBaseURL（默认 https://novac2c.cdn.weixin.qq.com/c2c）、Token、CredentialsPath（默认 ~/.weixin_cow_credentials.json）
- [x] 凭证持久化 Credentials：Token、BaseURL、BotID、UserID，保存为 JSON 文件（权限 0600）
- [x] 启动流程 Startup：initConfig → HTTP 客户端（TLS, 15s 超时）→ setupTmpDir → loadCredentials 或 performQRLogin → newWeixinAPI → pollLoop
- [x] QR 登录流程：fetchInitialQRCode → 循环 pollQRStatus → 处理 wait/scanned/expired/confirmed 状态
- [x] QR 状态处理：wait→继续等待，scanned→更新状态为 LoginStatusScanned，expired→刷新（最多 10 次），confirmed→保存凭证并返回
- [x] QR 超时 480 秒，刷新次数上限 maxQRRefreshes=10，终端打印二维码 URL
- [x] 登录状态机 LoginStatus：idle → waiting_scan → scanned → logged_in
- [x] 长轮询 pollLoop：通过 getUpdatesBuf 游标增量拉取，POST ilink/bot/getupdates（超时 35s+5s）
- [x] 消息去重：expiredMap（TTL 7h6m），按 message_id 或 seq 去重
- [x] 消息处理 processMessage：仅处理 messageTypeUser（type=1），忽略机器人消息（type=2）
- [x] 上下文令牌管理：per-receiver contextTokens map，每条消息的 context_token 存储到对应用户，发送时按 receiver 查找
- [x] 消息解析 parseWeixinMessage：解析 item_list 中 text/voice/image/video/file 各类型
- [x] 文本消息解析：text_item.text + ref_msg 引用消息（buildRefText 构建引用文本）
- [x] 语音消息解析：优先取 voice_item.text（语音转文字），无文字则作为媒体项处理
- [x] 媒体消息处理：image→ContextImage（立即下载），video/file/voice→延迟下载（prepareFunc），下载失败降级为 ContextText
- [x] CDN 媒体下载 downloadMedia：从 CDN /download 端点获取加密数据 → AES ECB 解密 → 保存到 tmpDir
- [x] CDN 媒体上传 uploadMediaToCDN：AES ECB 加密（PKCS7 填充，16 字节密钥）→ getUploadURL → POST CDN /upload → 获取 x-encrypted-param 和 AES key
- [x] AES 密钥解析 parseAESKey：支持十六进制（32 字符）和 Base64 格式，Base64 解码后 32 字节视为十六进制编码
- [x] 发送消息 Send：按 ReplyType 分发到 sendText/sendImage/sendFile/sendVideo，不支持的类型降级为 sendText
- [x] 文本发送 sendText：超过 textChunkLimit（4000 字符）时按段落/换行分割，分片间隔 500ms
- [x] 图片发送 sendImage：resolveMediaPath → uploadMediaToCDN → sendImageItem（encrypt_query_param + aes_key + ciphertext_size）
- [x] 文件发送 sendFile：resolveMediaPath → uploadMediaToCDN → sendFileItem（额外传 file_name + raw_size）
- [x] 视频发送 sendVideo：resolveMediaPath → uploadMediaToCDN → sendVideoItem（encrypt_query_param + aes_key + ciphertext_size）
- [x] 媒体路径解析 resolveMediaPath：支持 file:// 前缀剥离、http/https URL 自动下载到 tmpDir、本地文件路径检查
- [x] 媒体下载 downloadMedia：根据 Content-Type 推断扩展名（jpg/png/gif/webp/mp4/pdf），随机文件名 wx_media_{hex}{ext}
- [x] 临时目录 setupTmpDir：默认 ~/cow/tmp，自动创建（权限 0755）
- [x] API 请求头 buildHeaders：Content-Type JSON、AuthorizationType ilink_bot_token、随机 X-WECHAT-UIN、Bearer Token
- [x] 消息发送 API：ilink/bot/sendmessage，包含 from_user_id、to_user_id、client_id（随机生成）、message_type=2、message_state=2（FINISH）、context_token
- [x] 消息项类型：itemText=1, itemImage=2, itemVoice=3, itemFile=4, itemVideo=5
- [x] 错误恢复：连续失败 ≥3 次时退避 30 秒，否则重试 2 秒；会话过期（errcode -14）触发 relogin
- [x] 会话过期重登录 relogin：删除凭证文件 → performQRLogin → 重建 API 客户端 → 清空 contextTokens，失败则 5 分钟后重试
- [x] 优雅停止 Stop：close(stopChan) → pollWg.Wait() → BaseChannel.Stop()

## 代码参考
| 验收标准 | 代码位置 |
|---------|---------|
| WeixinChannel 结构 | weixin_channel.go:98-127 (WeixinChannel struct) |
| Config 默认值 | weixin_channel.go:82-87 (Config), weixin_channel.go:266-280 (initConfig) |
| 凭证持久化 | weixin_channel.go:89-95 (Credentials), weixin_channel.go:290-329 (load/saveCredentials) |
| Startup 流程 | weixin_channel.go:166-204 (Startup) |
| QR 登录流程 | weixin_channel.go:332-379 (performQRLogin) |
| QR 状态处理 | weixin_channel.go:421-438 (handleQRStatus) |
| QR 过期刷新 | weixin_channel.go:441-459 (handleQRExpired) |
| QR 确认处理 | weixin_channel.go:462-485 (handleQRConfirmed) |
| 登录状态机 | weixin_channel.go:72-79 (LoginStatus 常量) |
| 长轮询循环 | weixin_channel.go:496-532 (pollLoop) |
| 消息去重 | weixin_channel.go:878-937 (expiredMap), weixin_channel.go:640-648 (去重逻辑) |
| 消息处理 | weixin_channel.go:632-689 (processMessage) |
| 上下文令牌管理 | weixin_channel.go:116-117 (contextTokens), weixin_channel.go:652-657 (存储), weixin_channel.go:692-703 (查询) |
| 消息解析 | message.go:54-76 (parseWeixinMessage) |
| 文本消息解析 | message.go:104-113 (parseTextItem), message.go:116-144 (buildRefText) |
| 语音消息解析 | message.go:147-157 (parseVoiceItem) |
| 媒体消息处理 | message.go:194-253 (setupMedia) |
| CDN 媒体下载 | message.go:256-312 (downloadMedia), message.go:314-349 (downloadFromCDN) |
| CDN 媒体上传 | weixin_channel.go:1208-1281 (uploadMediaToCDN) |
| AES 密钥解析 | message.go:420-448 (parseAESKey) |
| AES ECB 加解密 | message.go:364-417 (aesECBEncrypt, aesECBDecrypt, aesECBPaddedSize) |
| Send 分发 | weixin_channel.go:216-253 (Send) |
| 文本分片发送 | weixin_channel.go:707-723 (sendText, textChunkLimit=4000) |
| 图片/文件/视频发送 | weixin_channel.go:725-772 (sendImage/sendFile/sendVideo) |
| 媒体路径解析 | weixin_channel.go:775-791 (resolveMediaPath) |
| 媒体 URL 下载 | weixin_channel.go:794-839 (downloadMedia) |
| 临时目录 | weixin_channel.go:283-287 (setupTmpDir) |
| API 请求头 | weixin_channel.go:959-975 (buildHeaders) |
| 消息发送 API | weixin_channel.go:1056-1138 (sendText/sendImageItem/sendFileItem/sendVideoItem/sendItems) |
| 错误恢复 | weixin_channel.go:590-596 (applyRetryDelay), weixin_channel.go:561-571 (handleSessionExpired) |
| 会话过期重登录 | weixin_channel.go:606-629 (relogin) |
| 优雅停止 | weixin_channel.go:207-213 (Stop) |
