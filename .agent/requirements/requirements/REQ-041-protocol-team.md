---
id: REQ-041
title: "多智能体团队协作"
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
  depends_on: [REQ-040]
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "多智能体团队协作模型，支持 Team 定义和任务编排"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析，从 context.go 提取完整的 TeamContext 和 AgentOutput 实现"
    reason: "扩展验收标准，补充团队上下文管理、步数控制、Agent 输出收集等详细功能项"
    snapshot: "TeamContext 团队上下文完整实现，支持任务管理、Agent 输出收集、步数控制、并发安全和重置"
source_code:
  - pkg/agent/protocol/context.go
---

# 多智能体团队协作

## 描述
多智能体团队协作模型，通过 TeamContext 管理团队协作的完整生命周期。TeamContext 封装了团队定义（名称、描述、规则、Agent 列表）、当前任务、Agent 输出收集、执行步数控制等核心能力。支持读写锁保护并发安全，MaxSteps 限制防止无限执行，Reset 机制允许上下文复用。AgentOutput 记录每个 Agent 的执行输出和时间戳，为多智能体协作提供完整的结果追踪。

## 验收标准
- [x] TeamContext 结构体定义，包含 Name/Description/Rule/Agents/UserTask/Task/TaskShortName/AgentOutputs/CurrentSteps/MaxSteps 字段
- [x] TeamContext 内置 sync.RWMutex 读写锁，保护并发访问安全
- [x] NewTeamContext 构造函数，接受 name/description/rule/agents/maxSteps 参数
- [x] NewTeamContext 中 maxSteps<=0 时默认设为 100
- [x] NewTeamContext 初始化 AgentOutputs 为空切片，CurrentSteps 为 0
- [x] SetTask 设置当前任务，同时更新 UserTask 字段（向后兼容）
- [x] SetTask 使用写锁保护并发安全
- [x] GetTask 获取当前任务，使用读锁保护
- [x] AddAgentOutput 添加 Agent 输出并自动递增 CurrentSteps
- [x] AddAgentOutput 使用写锁保护
- [x] GetAgentOutputs 获取所有 Agent 输出的副本（深拷贝，避免外部修改）
- [x] GetAgentOutputs 使用读锁保护
- [x] GetCurrentSteps 获取当前执行步数，使用读锁保护
- [x] IncrementSteps 递增执行步数并返回新值，使用写锁保护
- [x] CanContinue 检查是否可继续执行（CurrentSteps < MaxSteps），使用读锁保护
- [x] Reset 重置上下文，清空 Task/UserTask/TaskShortName/AgentOutputs/CurrentSteps
- [x] Reset 使用写锁保护
- [x] AgentOutput 结构体，包含 AgentName/Output/Timestamp 字段
- [x] NewAgentOutput 构造函数，使用 currentTimeMillis 生成时间戳
- [x] currentTimeMillis 和 UnixMilli 辅助函数，提供毫秒级时间戳

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| TeamContext 结构体 | `pkg/agent/protocol/context.go:11-34` |
| sync.RWMutex 读写锁 | `pkg/agent/protocol/context.go:33` |
| NewTeamContext 构造函数 | `pkg/agent/protocol/context.go:37-50` |
| maxSteps 默认值 100 | `pkg/agent/protocol/context.go:38-40` |
| AgentOutputs/CurrentSteps 初始化 | `pkg/agent/protocol/context.go:46-47` |
| SetTask 设置任务 | `pkg/agent/protocol/context.go:53-58` |
| SetTask 写锁保护 | `pkg/agent/protocol/context.go:54-55` |
| GetTask 获取任务 | `pkg/agent/protocol/context.go:61-65` |
| GetTask 读锁保护 | `pkg/agent/protocol/context.go:62-63` |
| AddAgentOutput 添加输出 | `pkg/agent/protocol/context.go:68-73` |
| AddAgentOutput 写锁保护 | `pkg/agent/protocol/context.go:69-70` |
| GetAgentOutputs 深拷贝 | `pkg/agent/protocol/context.go:76-82` |
| GetAgentOutputs 读锁保护 | `pkg/agent/protocol/context.go:77-78` |
| GetCurrentSteps | `pkg/agent/protocol/context.go:85-89` |
| IncrementSteps | `pkg/agent/protocol/context.go:92-97` |
| CanContinue | `pkg/agent/protocol/context.go:100-104` |
| Reset 重置 | `pkg/agent/protocol/context.go:107-115` |
| Reset 写锁保护 | `pkg/agent/protocol/context.go:108-109` |
| AgentOutput 结构体 | `pkg/agent/protocol/context.go:118-125` |
| NewAgentOutput 构造 | `pkg/agent/protocol/context.go:128-134` |
| currentTimeMillis/UnixMilli | `pkg/agent/protocol/context.go:137-144` |
