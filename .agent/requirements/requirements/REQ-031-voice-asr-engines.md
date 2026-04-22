---
id: REQ-031
title: "ASR 语音识别引擎"
status: active
level: story
priority: P0
cluster: voice
created_at: "2026-04-23T16:00:00"
updated_at: "2026-04-23T16:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-007]
  merged_from: []
  depends_on: []
  related_to: [REQ-030]
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/voice/ 各引擎目录"
    reason: "逆向代码生成需求"
    snapshot: "7 个 ASR 引擎：OpenAI Whisper、Azure、Baidu、Ali、Tencent、Xunfei、Google"
---

# ASR 语音识别引擎

## 描述
语音转文本（ASR）引擎实现，7 个平台同时支持 TTS 和 ASR，实现 VoiceEngine 完整接口。ASR 接收音频字节数据，返回识别文本。仅 TTS 的引擎（Edge、ElevenLabs）ASR 方法返回 VoiceError 错误。

## 验收标准
- [x] OpenAI ASR：基于 Whisper 模型（whisper-1），调用 CreateTranscription API，支持 Language 配置，ByteReader 适配 []byte 到 io.Reader（pkg/voice/openai/openai.go:132-158）
- [x] Azure ASR：Azure 语音识别服务，支持 Region/Language 配置（pkg/voice/azure/azure.go）
- [x] Baidu ASR：百度语音识别，支持 APIKey/SecretKey 认证（pkg/voice/baidu/baidu.go）
- [x] Ali ASR：阿里云语音识别，支持 APIKey/SecretKey 认证（pkg/voice/ali/ali.go）
- [x] Tencent ASR：腾讯云语音识别，支持 SecretKey 配置（pkg/voice/tencent/tencent.go）
- [x] Xunfei ASR：讯飞语音识别，支持 WebSocket 流式识别（pkg/voice/xunfei/xunfei.go）
- [x] Google ASR：Google Cloud Speech-to-Text，支持 ServiceAccount 认证（pkg/voice/google/google.go）
- [x] 不支持 ASR 的引擎（Edge、ElevenLabs）调用 ASR 时返回 VoiceError（engine + "asr" + 错误信息）
- [x] VoiceEngine 接口：ASR(ctx context.Context, audio []byte) (string, error)
- [x] ASREngine 纯 ASR 接口定义（voice.go:38-44），用于只需 ASR 的场景
