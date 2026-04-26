---
id: REQ-043
title: "Executor 执行器接口"
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
  depends_on: [REQ-041, REQ-042]
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "Protocol 执行器接口定义，负责任务的调度和执行"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析，从 protocol.go 提取完整的执行器接口、事件系统和协议配置实现"
    reason: "扩展验收标准，补充事件回调、事件数据类型、协议配置等详细功能项"
    snapshot: "Executor 接口支持 Run/RunWithMessages 双模式执行，8种事件类型覆盖完整执行生命周期，Protocol 配置提供默认单例"
source_code:
  - pkg/agent/protocol/protocol.go
---

# Executor 执行器接口

## 描述
Protocol 执行器接口定义，负责任务的调度和执行。Executor 接口定义了两种执行模式：Run 接受 Task 执行任务，RunWithMessages 使用已有消息历史执行任务。两种模式均通过 onEvent 回调实时推送执行进度，支持 8 种事件类型覆盖完整的执行生命周期。Protocol 配置结构体定义协议名称、版本和描述，DefaultProtocol 提供默认单例（uai-agent-protocol v1.0.0）。Bridge 层通过 Executor 接口与 Protocol 层交互，实现跨层调用的统一抽象。

## 验收标准
- [x] Executor 接口定义，包含 Run 和 RunWithMessages 两个方法
- [x] Run 方法签名：接受 context.Context、*Task、onEvent 回调，返回 *AgentResult 和 error
- [x] RunWithMessages 方法签名：接受 context.Context、[]Message、onEvent 回调，返回 *AgentResult 和 error
- [x] onEvent 回调函数类型：func(event map[string]any)，用于实时推送执行事件
- [x] 事件类型 EventTypeText：文本输出事件，对应 TextEventData（含 Delta 流式字段）
- [x] 事件类型 EventTypeToolCall：工具调用事件，对应 ToolCallEventData（ToolName/ToolCallID/Arguments）
- [x] 事件类型 EventTypeToolResult：工具结果事件，对应 ToolResultEventData（含 ExecutionTime）
- [x] 事件类型 EventTypeError：错误事件，对应 ErrorEventData（Message/Code）
- [x] 事件类型 EventTypeStepStart：步骤开始事件，对应 StepEventData（Step/MaxSteps/Action）
- [x] 事件类型 EventTypeStepEnd：步骤结束事件，对应 StepEventData
- [x] 事件类型 EventTypeComplete：完成事件，对应 CompleteEventData（FinalAnswer/StepCount/Status）
- [x] 事件类型 EventTypeThinking：思考事件，对应 ThinkingEventData（Thought）
- [x] Event 结构体包含 Type(string)/Timestamp(int64)/Data(map[string]any) 字段
- [x] NewEvent 构造函数，自动设置 Timestamp 为 currentTimeMillis
- [x] TextEventData 支持 Text 完整文本和 Delta 增量文本（流式输出场景）
- [x] ToolCallEventData 包含 ToolName/ToolCallID/Arguments，Arguments 为 map[string]any
- [x] ToolResultEventData 包含 ToolCallID/ToolName/Result(any)/Status/ExecutionTime(秒)
- [x] ErrorEventData 包含 Message 和可选 Code 字段
- [x] CompleteEventData 包含 FinalAnswer/StepCount/Status，描述任务最终完成状态
- [x] Protocol 配置结构体包含 Name/Version/Description 字段
- [x] DefaultProtocol 默认单例（uai-agent-protocol v1.0.0）可直接使用
- [x] GetProtocol() 函数获取默认协议配置

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| Executor 接口定义 | `pkg/agent/protocol/protocol.go:10-16` |
| Run 方法签名 | `pkg/agent/protocol/protocol.go:12` |
| RunWithMessages 方法签名 | `pkg/agent/protocol/protocol.go:15` |
| onEvent 回调类型 | `pkg/agent/protocol/protocol.go:12` |
| EventTypeText 常量 | `pkg/agent/protocol/protocol.go:21` |
| EventTypeToolCall 常量 | `pkg/agent/protocol/protocol.go:23` |
| EventTypeToolResult 常量 | `pkg/agent/protocol/protocol.go:25` |
| EventTypeError 常量 | `pkg/agent/protocol/protocol.go:27` |
| EventTypeStepStart 常量 | `pkg/agent/protocol/protocol.go:29` |
| EventTypeStepEnd 常量 | `pkg/agent/protocol/protocol.go:31` |
| EventTypeComplete 常量 | `pkg/agent/protocol/protocol.go:33` |
| EventTypeThinking 常量 | `pkg/agent/protocol/protocol.go:35` |
| Event 结构体 | `pkg/agent/protocol/protocol.go:39-46` |
| NewEvent 构造函数 | `pkg/agent/protocol/protocol.go:49-55` |
| TextEventData（含 Delta） | `pkg/agent/protocol/protocol.go:58-63` |
| ToolCallEventData | `pkg/agent/protocol/protocol.go:66-73` |
| ToolResultEventData | `pkg/agent/protocol/protocol.go:76-87` |
| ErrorEventData | `pkg/agent/protocol/protocol.go:90-95` |
| CompleteEventData | `pkg/agent/protocol/protocol.go:108-115` |
| Protocol 配置结构体 | `pkg/agent/protocol/protocol.go:124-131` |
| DefaultProtocol 单例 | `pkg/agent/protocol/protocol.go:134-138` |
| GetProtocol() 函数 | `pkg/agent/protocol/protocol.go:141-143` |
