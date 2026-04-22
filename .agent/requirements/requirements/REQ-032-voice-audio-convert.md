---
id: REQ-032
title: "音频格式转换"
status: active
level: story
priority: P1
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
    context: "从代码逆向分析细化需求，来源: pkg/voice/convert.go"
    reason: "逆向代码生成需求"
    snapshot: "音频格式转换：WAV/PCM 互转、SILK 编解码、ffmpeg 外部转码、重采样、音频分割"
---

# 音频格式转换

## 描述
音频格式转换模块（convert.go），支持 WAV/PCM 互转、SILK 编解码（微信语音）、重采样、音频分割。复杂格式（MP3、OGG、AMR、M4A）需要 ffmpeg 外部工具支持，SILK 编解码需要 go-silk 外部库。

## 验收标准
- [x] AudioFormat 类型定义 7 种格式：MP3、WAV、PCM、OGG、AMR、SILK、M4A（voice.go:131-148）
- [x] Converter 接口：Convert(input, srcFormat, dstFormat, opts) 和 ConvertFile(srcPath, dstPath, opts)（convert.go:37-43）
- [x] AudioConverter 实现：解码源格式到 PCM，再从 PCM 编码到目标格式（convert.go:54-76）
- [x] WAV↔PCM 互转：pcmToWAV 构建 44 字节 WAV 头（含 RIFF/WAVE/fmt/data 块），wavToPCM 解析 data 块提取 PCM（convert.go:128-200）
- [x] SILK 编解码接口：silkToPCM 和 pcmToSILK，需外部库 github.com/wdvxdr1123/go-silk（convert.go:202-215）
- [x] SILK 采样率适配：FindClosestSilkRate 从 7 个支持采样率（8k/12k/16k/24k/32k/44.1k/48k）中找最接近值（convert.go:218-237）
- [x] 重采样：ResampleSimple 线性插值重采样，支持 16 位 PCM 采样率转换（convert.go:263-296）
- [x] 音频分割：SplitAudio 按 maxSegmentBytes 分割 PCM 数据（convert.go:299-314）
- [x] WAV 信息解析：GetWAVInfo 提取采样率、声道数、位深度（convert.go:240-252）
- [x] PCM 读写：ReadPCMFromReader/WritePCMToWriter（convert.go:317-325）
- [x] PCM 数据验证：ValidatePCMData 检查数据长度对齐（convert.go:328-343）
- [x] 默认转换选项：SampleRate=16000、Channels=1、BitDepth=16（convert.go:28-34）
- [x] MP3/OGG/AMR/M4A 格式需要 ffmpeg 外部库支持，返回错误提示（convert.go:86-87, 103）
