---
id: REQ-007
title: "语音处理系统"
status: active
level: epic
priority: P1
cluster: voice
created_at: "2026-04-23T10:00:00"
updated_at: "2026-04-23T10:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: []
  merged_from: []
  depends_on: [REQ-004]
  related_to: []
  refined_by: [REQ-030, REQ-031, REQ-032]
versions:
  - version: 1
    date: "2026-04-23T10:00:00"
    author: ai
    context: "从代码逆向分析提取需求，来源: pkg/voice/"
    reason: "逆向代码生成需求"
    snapshot: "9 个语音平台适配，统一 TTS/ASR 接口，支持音频格式转换（ffmpeg/SILK）"
---

# 语音处理系统

## 描述
统一 VoiceEngine 接口的语音处理系统，支持 9 个 TTS/ASR 平台。提供文本转语音（TTS）和语音转文本（ASR）能力，支持多种音频格式转换（需 ffmpeg），SILK 编解码（微信语音）。

## 验收标准
- [x] VoiceEngine 接口：TTS(ctx, text) → []byte、ASR(ctx, audio) → string、Name()
- [x] TTSEngine 纯 TTS 接口（仅 TTS 的引擎）
- [x] ASREngine 纯 ASR 接口（仅 ASR 的引擎）
- [x] 9 个引擎适配：OpenAI（TTS+ASR）、Azure（TTS+ASR）、Baidu（TTS+ASR）、Ali（TTS+ASR）、Tencent（TTS+ASR）、Xunfei（TTS+ASR）、Google（TTS+ASR）、Edge（仅 TTS）、ElevenLabs（仅 TTS）
- [x] Config 配置：EngineType、APIKey、SecretKey、Region、VoiceID、Language、OutputFormat、SampleRate、Proxy、Timeout
- [x] 音频格式支持：MP3、WAV、PCM、OGG、AMR、SILK、M4A
- [x] 音频格式转换（convert.go）：ffmpeg 转码、SILK 编解码
- [x] 工厂模式注册：init() 中 RegisterEngine()
- [x] VoiceError 统一错误类型：Engine + Operation + 原始错误
