---
id: REQ-009
title: "Agent 执行循环与多步控制"
status: active
level: story
priority: P0
cluster: agent-core
created_at: "2026-04-23T10:10:00"
updated_at: "2026-04-23T16:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-001]
  merged_from: []
  depends_on: [REQ-005]
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T10:10:00"
    author: ai
    context: "从 REQ-001 拆分，来源: pkg/agent/executor.go"
    reason: "Epic 拆分为 Story"
    snapshot: "Agent 执行循环：消息→LLM→工具调用→结果反馈→LLM→最终回复，支持 max_steps 控制"
  - version: 2
    date: "2026-04-23T16:00:00"
    author: ai
    context: "深度分析 executor.go 代码，细化验收标准"
    reason: "代码深度分析细化"
    snapshot: "执行循环详细实现：并行工具执行、事件推送、流式/同步模式选择、ToolContext 注入、PreProcess 跳过"
---

# Agent 执行循环与多步控制

## 描述
Agent 核心执行循环，接收用户消息后构建提示词调用 LLM，解析 Function Calling 响应并行执行工具，将工具结果反馈给 LLM 继续推理，直到 LLM 不再调用工具或达到最大步数限制。executor 结构体封装执行逻辑，通过 onEvent 回调向外部推送执行过程事件。

## 验收标准
- [x] 执行循环（executor.run）：for step := 0; step < maxSteps; step++ 循环，每步调用 callLLMWithMode
- [x] 模式选择（callLLMWithMode）：stream=true 且无工具时走流式路径 callLLMStreamMode，否则走同步路径 callLLM
- [x] LLM 调用（callLLM）：构建 CallOptions（WithSystemPrompt、WithMaxTokens、WithTemperature），工具定义通过 WithTools 传入 ToolDefinition{Type:"function", Function}
- [x] 流式调用（callLLMStreamMode）：通过 model.CallStream 获取 StreamChunk 通道，逐块读取 Delta 和 Done 信号，拼接完整内容
- [x] Function Calling 解析：hasToolCalls 检查 response.ToolCalls 是否为空，空则走 handleNoToolCalls 返回最终文本
- [x] 工具并行执行（processToolCalls）：sync.WaitGroup 并行执行多个 ToolCall，按原始顺序添加结果消息
- [x] 单工具执行（executeTool）：从 toolRegistry.Get(name) 获取工具，parseToolCallArgs 解析参数，支持 ToolWithContext 接口注入会话上下文
- [x] 工具结果处理：成功结果 json.Marshal 后作为 role=tool 消息添加，错误结果通过 NewErrorToolResult 包装
- [x] 结果反馈：agent.AddToolResultMessage(id, result) 将工具结果按 ToolCallID 对应添加到消息历史
- [x] 多步控制：max_steps 限制（默认 15，通过 config.AgentMaxSteps 配置），超限返回 error "max steps reached"
- [x] 错误处理：LLM 调用失败 → handleLLMError（emit EventTypeError + 返回错误），工具执行失败 → 计数 errorCount 并返回
- [x] 事件推送：onEvent 回调支持 7 种事件类型（step_start/step_end/text/tool_call/tool_result/error/complete）
- [x] 无工具调用终止（handleNoToolCalls）：emit EventTypeText + EventTypeComplete，agent.AddAssistantMessage 记录最终回复
- [x] PreProcess 工具跳过：ToolStage == ToolStagePreProcess 的工具在 executor 中跳过执行
- [x] 消息历史构建：agent.GetMessagesWithSystem() 获取含系统提示词的消息列表，追加当前用户消息
