---
id: REQ-040
title: "Protocol 类型与消息块"
status: active
level: story
priority: P1
cluster: agent-core
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-039]
  merged_from: []
  depends_on: []
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "定义 Protocol 的核心类型系统和消息块接口"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析，从 message.go 和 protocol.go 提取完整消息处理功能"
    reason: "扩展验收标准，补充消息修复、文本提取、轮次压缩等详细功能项"
    snapshot: "Protocol 消息体系完整实现，包含 MessageBlock 多类型消息块、SanitizeMessages 四步修复、文本提取和对话压缩"
source_code:
  - pkg/agent/protocol/protocol.go
  - pkg/agent/protocol/message.go
---

# Protocol 类型与消息块

## 描述
定义 Protocol 的核心类型系统和消息块接口。支持多种内容类型的消息传递，是整个 Protocol 层的基础。定义了文本、图片、音频等消息块类型，以及工具调用和工具结果的标准化消息格式，为上层团队协作和任务编排提供统一的消息抽象。实现消息修复（SanitizeMessages）确保 tool_use/tool_result 邻接正确，文本提取（ExtractTextFromContent）从混合内容中提取纯文本，对话压缩（CompressTurnToTextOnly）将完整轮次压缩为轻量级纯文本。

## 验收标准
- [x] MessageBlock 结构体，包含 Type/Text/ID/Name/Input/ToolUseID/Content/IsError 字段
- [x] Message 结构体，包含 Role 和 Content([]MessageBlock) 字段
- [x] 消息块类型 text：文本内容块，通过 Text 字段传递文本
- [x] 消息块类型 tool_use：工具调用块，包含 ID/Name/Input 字段标识调用
- [x] 消息块类型 tool_result：工具结果块，包含 ToolUseID/Content/IsError 字段
- [x] SanitizeMessages 消息修复入口，返回修复次数（移除数+邻接修复数）
- [x] 消息修复步骤1：repairToolUseAdjacency 修复 tool_use 后必须紧跟 tool_result 的邻接关系
- [x] 消息修复步骤2：removeLeadingOrphanToolResults 移除开头的孤立 tool_result user 消息
- [x] 消息修复步骤3：removeMismatchedToolBlocks 迭代移除不匹配的 tool_use/tool_result（最多5轮）
- [x] 消息修复步骤4：修复后有移除时重新修复邻接关系
- [x] 合成 tool_result 机制：为缺失的 tool_use 生成合成 tool_result 块（IsError=true）
- [x] collectToolIDs 收集消息中所有 tool_use 和 tool_result ID 的配对信息
- [x] findMismatchedIDs 找出不匹配的 tool_use ID（无对应 result）和 tool_result ID（无对应 use）
- [x] ExtractTextFromContent 支持 string/[]MessageBlock/[]map[string]any 三种输入类型
- [x] extractTextFromBlocks 从 MessageBlock 切片中提取 type=text 的文本
- [x] extractTextFromMaps 从 map 切片中提取 type=text 的文本
- [x] CompressTurnToTextOnly 将完整对话轮次压缩为纯文本，保留首个用户文本和最后助手文本
- [x] processMessageForMismatch 处理单条消息中的不匹配块，移除坏 tool_use 消息或过滤坏 tool_result 块
- [x] 消息修复过程输出日志：移除损坏消息数和邻接修复次数
- [x] tool_use 后无下一条消息时，在末尾追加合成 tool_result
- [x] tool_use 后下一条非 user 消息时，在中间插入合成 tool_result

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| MessageBlock 结构体 | `pkg/agent/protocol/message.go:16-25` |
| Message 结构体 | `pkg/agent/protocol/message.go:28-31` |
| text 类型消息块 | `pkg/agent/protocol/message.go:17` |
| tool_use 类型消息块 | `pkg/agent/protocol/message.go:19-21` |
| tool_result 类型消息块 | `pkg/agent/protocol/message.go:22-24` |
| SanitizeMessages 入口 | `pkg/agent/protocol/message.go:37-66` |
| repairToolUseAdjacency 邻接修复 | `pkg/agent/protocol/message.go:230-277` |
| removeLeadingOrphanToolResults 开头孤立修复 | `pkg/agent/protocol/message.go:117-135` |
| removeMismatchedToolBlocks 迭代移除 | `pkg/agent/protocol/message.go:138-168` |
| 重新修复邻接关系 | `pkg/agent/protocol/message.go:54-56` |
| 合成 tool_result 机制 | `pkg/agent/protocol/message.go:313-324` |
| collectToolIDs ID 收集 | `pkg/agent/protocol/message.go:75-95` |
| findMismatchedIDs 不匹配检测 | `pkg/agent/protocol/message.go:98-114` |
| ExtractTextFromContent 文本提取 | `pkg/agent/protocol/message.go:363-374` |
| extractTextFromBlocks | `pkg/agent/protocol/message.go:377-385` |
| extractTextFromMaps | `pkg/agent/protocol/message.go:388-398` |
| CompressTurnToTextOnly 轮次压缩 | `pkg/agent/protocol/message.go:407-447` |
| processMessageForMismatch 单条处理 | `pkg/agent/protocol/message.go:171-194` |
| 修复日志输出 | `pkg/agent/protocol/message.go:58-63` |
| 末尾追加合成 tool_result | `pkg/agent/protocol/message.go:327-335` |
| 中间插入合成 tool_result | `pkg/agent/protocol/message.go:338-344` |
