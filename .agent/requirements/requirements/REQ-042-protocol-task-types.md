---
id: REQ-042
title: "Task 任务类型系统"
status: active
level: story
priority: P2
cluster: agent-core
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-039]
  merged_from: []
  depends_on: [REQ-040]
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "Protocol 层的任务类型定义和执行结果模型"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析，从 task.go 和 result.go 提取完整任务和结果模型实现"
    reason: "扩展验收标准，补充任务类型枚举、Functional Options、媒体附件、动作模型、Token 统计等详细功能项"
    snapshot: "Task 任务系统完整实现，6种类型、4种状态、8个Option、媒体附件、深拷贝；AgentResult 结果模型含3种动作类型和Token统计"
source_code:
  - pkg/agent/protocol/task.go
  - pkg/agent/protocol/result.go
---

# Task 任务类型系统

## 描述
Protocol 层的任务类型定义和执行结果模型。Task 结构体描述了单次任务的完整信息（ID、内容、类型、状态、元数据、媒体附件），通过 Functional Options 模式创建，支持 6 种任务类型和 4 种状态的生命周期管理。AgentResult 描述任务执行的最终结果，包含 3 种动作类型（工具调用、思考、最终回答）、Token 用量统计和状态判断。ToolResult 封装工具执行的返回结果，区分成功和失败状态。

## 验收标准
- [x] TaskType 枚举定义 6 种类型：TaskTypeText/TaskTypeImage/TaskTypeVideo/TaskTypeAudio/TaskTypeFile/TaskTypeMixed
- [x] TaskType.String() 方法返回字符串表示
- [x] TaskStatus 枚举定义 4 种状态：TaskStatusInit/TaskStatusProcessing/TaskStatusCompleted/TaskStatusFailed
- [x] TaskStatus.String() 方法返回字符串表示
- [x] TaskStatus.IsTerminal() 方法判断是否为终态（Completed 或 Failed）
- [x] Task 结构体包含 ID/Content/Type/Status/CreatedAt/UpdatedAt/Metadata/Images/Videos/Audios/Files 字段
- [x] NewTask 构造函数，使用 uuid 自动生成 ID，默认类型 Text、默认状态 Init
- [x] NewTask 初始化 Metadata 和所有媒体切片为空（非 nil）
- [x] 8 个 TaskOption 函数：WithTaskType/WithTaskStatus/WithTaskMetadata/WithTaskImages/WithTaskVideos/WithTaskAudios/WithTaskFiles/WithTaskID
- [x] WithTaskMetadata 当传入 nil 时不覆盖（保留默认空 map）
- [x] Task.GetText() 返回任务文本内容（Content 字段）
- [x] Task.UpdateStatus() 更新状态并自动刷新 UpdatedAt 时间
- [x] Task.SetMetadata()/GetMetadata() 元数据读写，SetMetadata 自动初始化 nil map
- [x] Task.AddImage()/AddVideo()/AddAudio()/AddFile() 媒体文件追加，自动刷新 UpdatedAt
- [x] Task.HasMedia() 检查是否包含任何媒体内容
- [x] Task.Clone() 深拷贝任务，Metadata 使用 maps.Copy，媒体切片使用 append 拷贝
- [x] AgentActionType 枚举定义 3 种动作：ActionTypeToolUse/ActionTypeThinking/ActionTypeFinalAnswer
- [x] AgentAction 结构体包含 ID/AgentID/AgentName/ActionType/Content/ToolResult/Thought/Timestamp 字段
- [x] NewAgentAction 使用 uuid 生成 ID，支持 3 个 ActionOption：WithContent/WithToolResult/WithThought
- [x] ToolResult 结构体包含 ToolName/InputParams/Output/Status/ErrorMessage/ExecutionTime 字段
- [x] NewToolResult 创建成功结果（Status=success），NewErrorToolResult 创建失败结果（Status=error）
- [x] ToolResult.IsSuccess()/IsError() 状态判断方法
- [x] AgentResult 结构体包含 FinalAnswer/StepCount/Status/ErrorMessage/Actions/Usage 字段
- [x] NewAgentResult/NewSuccessResult 创建成功结果，NewErrorResult 创建失败结果
- [x] AgentResult.IsSuccess()/IsError() 状态判断方法
- [x] AgentResult.AddAction() 追加动作，自动初始化 nil Actions 切片
- [x] AgentResult.SetUsage() 设置 Token 用量，自动计算 TotalTokens
- [x] AgentResult.GetToolActions() 过滤获取所有工具调用动作
- [x] AgentResult.GetFinalAnswerAction() 获取最终回答动作
- [x] TokenUsage 结构体包含 PromptTokens/CompletionTokens/TotalTokens 字段

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| TaskType 6种枚举 | `pkg/agent/protocol/task.go:15-28` |
| TaskType.String() | `pkg/agent/protocol/task.go:31-33` |
| TaskStatus 4种枚举 | `pkg/agent/protocol/task.go:38-47` |
| TaskStatus.String() | `pkg/agent/protocol/task.go:50-52` |
| TaskStatus.IsTerminal() | `pkg/agent/protocol/task.go:55-57` |
| Task 结构体 | `pkg/agent/protocol/task.go:60-85` |
| NewTask 构造函数 | `pkg/agent/protocol/task.go:88-109` |
| 默认值初始化 | `pkg/agent/protocol/task.go:93-101` |
| 8个 TaskOption | `pkg/agent/protocol/task.go:114-170` |
| WithTaskMetadata nil 保护 | `pkg/agent/protocol/task.go:131-133` |
| Task.GetText() | `pkg/agent/protocol/task.go:173-175` |
| Task.UpdateStatus() | `pkg/agent/protocol/task.go:178-181` |
| SetMetadata/GetMetadata | `pkg/agent/protocol/task.go:184-199` |
| AddImage/AddVideo/AddAudio/AddFile | `pkg/agent/protocol/task.go:202-223` |
| Task.HasMedia() | `pkg/agent/protocol/task.go:226-228` |
| Task.Clone() | `pkg/agent/protocol/task.go:231-248` |
| AgentActionType 3种枚举 | `pkg/agent/protocol/result.go:14-21` |
| AgentAction 结构体 | `pkg/agent/protocol/result.go:78-95` |
| NewAgentAction 及 ActionOption | `pkg/agent/protocol/result.go:98-136` |
| ToolResult 结构体 | `pkg/agent/protocol/result.go:29-42` |
| NewToolResult/NewErrorToolResult | `pkg/agent/protocol/result.go:45-65` |
| ToolResult.IsSuccess/IsError | `pkg/agent/protocol/result.go:68-75` |
| AgentResult 结构体 | `pkg/agent/protocol/result.go:139-152` |
| NewAgentResult/NewSuccessResult | `pkg/agent/protocol/result.go:165-182` |
| NewErrorResult | `pkg/agent/protocol/result.go:185-193` |
| AgentResult.IsSuccess/IsError | `pkg/agent/protocol/result.go:196-203` |
| AgentResult.AddAction | `pkg/agent/protocol/result.go:206-211` |
| AgentResult.SetUsage | `pkg/agent/protocol/result.go:214-220` |
| AgentResult.GetToolActions | `pkg/agent/protocol/result.go:223-231` |
| AgentResult.GetFinalAnswerAction | `pkg/agent/protocol/result.go:234-241` |
| TokenUsage 结构体 | `pkg/agent/protocol/result.go:155-162` |
