---
id: REQ-039
title: "Protocol 多智能体协议系统"
status: active
level: epic
priority: P1
cluster: agent-core
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: []
  merged_from: []
  depends_on: [REQ-001]
  refined_by: [REQ-040, REQ-041, REQ-042, REQ-043]
  related_to: [REQ-008]
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "Protocol 协议层，定义多智能体协作的消息格式、任务类型、执行器接口"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析，从已实现的 Protocol 包中提取完整验收标准"
    reason: "扩展验收标准，补充代码参考和详细功能项"
    snapshot: "Protocol 协议系统完整实现，包含执行器接口、8种事件类型、消息块体系、任务生命周期、团队上下文、执行结果模型"
source_code:
  - pkg/agent/protocol/context.go
  - pkg/agent/protocol/doc.go
  - pkg/agent/protocol/message.go
  - pkg/agent/protocol/protocol.go
  - pkg/agent/protocol/result.go
  - pkg/agent/protocol/task.go
---

# Protocol 多智能体协议系统

## 描述
Agent Protocol 协议层，定义多智能体协作的消息格式、任务类型、执行器接口。支持 Text/Image/Audio/Mixed 多种消息块，Team 多智能体团队协作，Task 任务编排。作为 Agent 核心与 Bridge 层之间的协议抽象，为多智能体场景提供统一的消息传递和任务调度能力。

## 验收标准
- [x] Executor 执行器接口定义，支持 Run 和 RunWithMessages 两种执行模式
- [x] 8 种事件类型常量：EventTypeText/EventTypeToolCall/EventTypeToolResult/EventTypeError/EventTypeStepStart/EventTypeStepEnd/EventTypeComplete/EventTypeThinking
- [x] Event 事件结构体，包含 Type/Timestamp/Data 字段及 NewEvent 构造函数
- [x] TextEventData 文本事件数据，支持完整文本和增量文本（Delta 流式输出）
- [x] ToolCallEventData 工具调用事件数据，包含 ToolName/ToolCallID/Arguments
- [x] ToolResultEventData 工具结果事件数据，包含 ToolCallID/ToolName/Result/Status/ExecutionTime
- [x] ErrorEventData 错误事件数据，包含 Message/Code 字段
- [x] StepEventData 步骤事件数据，包含 Step/MaxSteps/Action 字段
- [x] CompleteEventData 完成事件数据，包含 FinalAnswer/StepCount/Status 字段
- [x] ThinkingEventData 思考事件数据，包含 Thought 字段
- [x] Protocol 协议配置结构体，包含 Name/Version/Description
- [x] DefaultProtocol 默认协议单例（uai-agent-protocol v1.0.0）及 GetProtocol 获取函数
- [x] MessageBlock 消息块结构体，支持 text/tool_use/tool_result 三种类型
- [x] SanitizeMessages 消息修复机制，修复 tool_use/tool_result 邻接关系和孤立消息
- [x] ExtractTextFromContent 文本提取，支持 MessageBlock 切片和 map 切片两种输入
- [x] CompressTurnToTextOnly 对话轮次压缩，保留用户文本和最终助手文本
- [x] Task 任务结构体，支持 6 种任务类型和 4 种状态
- [x] TeamContext 团队协作上下文，支持任务管理、Agent 输出收集、步数控制
- [x] AgentResult 执行结果模型，包含动作列表、Token 用量统计和状态判断
- [x] 事件回调机制，Executor 通过 onEvent 回调实时推送执行进度

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| Executor 接口 | `pkg/agent/protocol/protocol.go:10-16` |
| 8 种事件类型常量 | `pkg/agent/protocol/protocol.go:19-36` |
| Event 结构体及构造 | `pkg/agent/protocol/protocol.go:39-55` |
| TextEventData | `pkg/agent/protocol/protocol.go:58-63` |
| ToolCallEventData | `pkg/agent/protocol/protocol.go:66-73` |
| ToolResultEventData | `pkg/agent/protocol/protocol.go:76-87` |
| ErrorEventData | `pkg/agent/protocol/protocol.go:90-95` |
| StepEventData | `pkg/agent/protocol/protocol.go:98-105` |
| CompleteEventData | `pkg/agent/protocol/protocol.go:108-115` |
| ThinkingEventData | `pkg/agent/protocol/protocol.go:118-121` |
| Protocol 配置及默认单例 | `pkg/agent/protocol/protocol.go:124-143` |
| MessageBlock 结构体 | `pkg/agent/protocol/message.go:16-25` |
| SanitizeMessages 消息修复 | `pkg/agent/protocol/message.go:37-66` |
| ExtractTextFromContent 文本提取 | `pkg/agent/protocol/message.go:363-374` |
| CompressTurnToTextOnly 轮次压缩 | `pkg/agent/protocol/message.go:407-447` |
| Task 任务结构体及类型 | `pkg/agent/protocol/task.go:12-85` |
| TeamContext 团队上下文 | `pkg/agent/protocol/context.go:11-34` |
| AgentResult 结果模型 | `pkg/agent/protocol/result.go:139-152` |
| 事件回调机制 | `pkg/agent/protocol/protocol.go:12` |
