---
id: REQ-029
title: "Agent 多智能体插件 + Finish 未知命令兜底插件"
status: active
level: story
priority: P1
cluster: plugins
created_at: "2026-04-23T16:00:00"
updated_at: "2026-04-23T16:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-006]
  merged_from: []
  depends_on: [REQ-004]
  related_to: [REQ-001]
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/plugin/agent/agent.go (409行) + pkg/plugin/finish/finish.go (138行)"
    reason: "逆向代码生成需求"
    snapshot: "Agent 多智能体协作插件（团队/智能体配置、use命令路由、AgentMesh 占位）+ Finish 未知命令兜底检测（-999优先级、$前缀拦截）"
---

# Agent 多智能体插件 + Finish 未知命令兜底插件

## 描述
两个插件构成命令处理的最后一道防线：

**Agent 插件**（409 行）：基于 AgentMesh 框架的多智能体协作任务处理插件。通过团队（Team）和智能体（Agent）两级配置组织协作，支持 `use <team> <task>` 命令路由。当前为框架实现，AgentMesh 实际执行为占位符，待集成真正的多智能体调度引擎。

**Finish 插件**（138 行）：轻量级未知命令检测器，优先级设为 -999（最低），确保在所有其他插件之后执行。当消息以 TriggerPrefix（"$"）开头但未被任何插件处理时，返回 ErrorMessage 提示用户。

## 验收标准

### Agent 插件
- [x] AgentPlugin 结构体：嵌入 `BasePlugin`，持有 `config *Config` (agent.go:72-77)
- [x] 版本 "0.1.0"，优先级 1，描述 "使用 AgentMesh 框架实现多智能体协作任务处理" (agent.go:84-87)
- [x] 配置结构体 `Config`：`DefaultTeam string`、`Teams map[string]TeamConfig`、`Tools map[string]any`、`TriggerPrefix string`（默认 "$"） (agent.go:18-30)
- [x] 团队配置 `TeamConfig`：`Description string`、`Rule string`、`Model string`、`MaxSteps int`、`Agents []AgentConfig` (agent.go:33-48)
- [x] 智能体配置 `AgentConfig`：`Name string`、`Description string`、`SystemPrompt string`、`Model string`（可选，默认用团队模型）、`MaxSteps int`、`Tools []string` (agent.go:51-69)
- [x] 默认配置 `createDefaultConfig`：含 "default" 团队，内置 "assistant" 智能体，MaxSteps=20/10 (agent.go:369-401)
- [x] OnInit 初始化：加载配置文件，不存在时创建默认配置 (agent.go:107-124)
- [x] OnLoad 注册 `EventOnHandleContext` 处理器 (agent.go:127-135)
- [x] 消息触发：`TriggerPrefix+"agent "` 前缀，提取 task 内容 (agent.go:166-168)
- [x] 空任务处理：返回 `getHelpText(true)` 帮助信息 (agent.go:174-178)
- [x] 团队查询 `isTeamsQueryCommand`：识别 "teams"/"list teams"/"show teams" 命令 (agent.go:214-217)
- [x] 团队列表 `handleTeamsQuery`：调用 `getAvailableTeams()` 展示所有配置的团队名 (agent.go:220-228)
- [x] 命令路由 `parseTeamAndTask`：解析 `use <team> <task>` 格式，返回 (teamName, task, handled) 三元组 (agent.go:231-250)
- [x] 默认团队解析 `resolveDefaultTeam`：优先 `config.DefaultTeam`，否则取第一个可用团队 (agent.go:253-266)
- [x] 任务执行 `executeTask`：查找 `config.Teams[teamName]`，当前返回团队信息+任务摘要（AgentMesh 集成占位） (agent.go:269-289)
- [x] 帮助文本 `getHelpText`：展示命令用法、可用团队列表、示例 (agent.go:315-347)

### Finish 插件
- [x] FinishPlugin 结构体：嵌入 `BasePlugin`，持有 `config *Config` (finish.go:12-16)
- [x] 版本 "1.0.0"，优先级 -999（最低），`SetHidden(true)` (finish.go:38-39)
- [x] 配置 `Config`：`Enabled bool`（默认 true）、`TriggerPrefix string`（默认 "$"）、`ErrorMessage string`（默认 "未知插件命令\n查看插件命令列表请输入#help 插件名\n"） (finish.go:19-28, 44-47)
- [x] OnInit 加载配置：从 `ctx.Config` 读取 enabled、trigger_prefix、error_message (finish.go:63-81)
- [x] OnLoad 注册 `EventOnHandleContext` 处理器 (finish.go:84-88)
- [x] 未知命令检测 `onHandleContext`：检查消息是否以 TriggerPrefix 开头但未被其他插件处理（优先级 -999 保证最后执行），设置 ErrorMessage 为 reply 并 `BreakPass` (finish.go:97-115，隐含在优先级机制中)
- [x] OnUnload 委托 `BasePlugin.OnUnload` 清理 (finish.go:92-95)
- [x] 插件接口实现验证：`var _ plugin.Plugin = (*FinishPlugin)(nil)` 编译时检查 (finish.go:31)
