---
id: REQ-001
title: "Agent 执行引擎"
status: active
level: epic
priority: P0
cluster: agent-core
created_at: "2026-04-23T10:00:00"
updated_at: "2026-04-22T16:13:03"
relations:
  supersedes: []
  conflicts_with: []
  refines: []
  merged_from: []
  depends_on: []
  refined_by: [REQ-009, REQ-010, REQ-011]
  related_to: [REQ-002, REQ-003]
versions:
  - version: 1
    date: "2026-04-23T10:00:00"
    author: ai
    context: "从代码逆向分析提取需求，来源: pkg/agent/"
    reason: "逆向代码生成需求"
    snapshot: "Agent 执行引擎，支持 Function Calling 循环、工具注册与调用、会话管理"
  - version: 2
    date: "2026-04-22T16:13:03"
    author: ai
    context: "元数据自动同步"
    reason: "自动补充反向关系: refined_by"
    snapshot: "自动同步元数据"
---

# Agent 执行引擎

## 描述
Agent 核心执行引擎，负责接收用户消息、构建提示词、调用 LLM、解析 Function Calling 响应、执行工具调用、将结果反馈给 LLM 形成闭环。支持多步执行（max_steps）、会话隔离、工具注册表等核心能力。

## 验收标准
- [x] Agent 执行循环：消息 → LLM → 工具调用 → 结果反馈 → LLM → 最终回复
- [x] 工具注册表（ToolRegistry）：统一注册和查找工具
- [x] 多步执行控制（max_steps 限制）
- [x] 会话管理（session 隔离、上下文窗口管理）
- [x] 提示词构建器（系统提示词、工具描述、记忆注入）
- [x] 技能系统（从 Markdown 文件加载技能到 workspace）
- [x] 流式输出支持（SSE 事件流）
- [x] 优雅关闭和资源清理
