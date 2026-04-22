---
id: REQ-030
title: "TTS 语音合成引擎"
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
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/voice/ 各引擎目录"
    reason: "逆向代码生成需求"
    snapshot: "9 个 TTS 引擎适配：OpenAI、Azure、Baidu、Ali、Tencent、Xunfei、Google、Edge、ElevenLabs"
---

# TTS 语音合成引擎

## 描述
文本转语音（TTS）引擎实现，覆盖 9 个 TTS 平台。其中 7 个同时支持 ASR（OpenAI、Azure、Baidu、Ali、Tencent、Xunfei、Google）实现 VoiceEngine 完整接口，2 个仅支持 TTS（Edge、ElevenLabs）实现 TTSEngine 接口。所有引擎通过工厂模式统一注册，按 Config 配置创建实例。

## 验收标准
- [x] OpenAI TTS：基于 go-openai SDK，调用 CreateSpeech API，默认模型 tts-1，默认语音 alloy，支持自定义 APIBase/Proxy/Timeout（pkg/voice/openai/openai.go）
- [x] Azure TTS：Azure 语音服务，支持 Region/VoiceID/Language 配置（pkg/voice/azure/azure.go）
- [x] Baidu TTS：百度语音合成，支持 APIKey/SecretKey 认证（pkg/voice/baidu/baidu.go）
- [x] Ali TTS：阿里云语音合成，支持 APIKey/SecretKey 认证（pkg/voice/ali/ali.go）
- [x] Tencent TTS：腾讯云语音合成，支持 SecretKey 配置（pkg/voice/tencent/tencent.go）
- [x] Xunfei TTS：讯飞语音合成，支持 WebSocket 流式合成（pkg/voice/xunfei/xunfei.go）
- [x] Google TTS：Google Cloud 语音合成，支持 ServiceAccount 认证（pkg/voice/google/google.go）
- [x] Edge TTS：基于 edge-tts-go 库，免费微软 Edge TTS 服务，支持 14 个中文语音（zh-CN-YunxiNeural 等），支持语速/音量/音调参数配置（pkg/voice/edge/edge.go）
- [x] ElevenLabs TTS：基于 HTTP API 调用，默认模型 eleven_multilingual_v2，默认语音 Rachel，支持 voice_settings 自定义（pkg/voice/elevent/elevent.go）
- [x] 纯 TTS 引擎（Edge、ElevenLabs）实现 VoiceEngine 接口但 ASR 方法返回 VoiceError
- [x] 所有引擎在 init() 中调用 voice.RegisterEngine() 注册到工厂
- [x] Config 统一配置：EngineType、APIKey、SecretKey、Region、VoiceID、Language、Model、OutputFormat、SampleRate、Proxy、Timeout、Extra
