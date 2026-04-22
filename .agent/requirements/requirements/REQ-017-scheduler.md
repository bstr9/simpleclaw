---
id: REQ-017
title: "定时任务调度器"
status: active
level: story
priority: P1
cluster: bridge
created_at: "2026-04-23T10:10:00"
updated_at: "2026-04-23T18:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-008]
  merged_from: []
  depends_on: [REQ-001, REQ-004]
  related_to: [REQ-002]
versions:
  - version: 1
    date: "2026-04-23T10:10:00"
    author: ai
    context: "从 REQ-008 拆分，来源: pkg/scheduler/, cmd/simpleclaw/main.go"
    reason: "Epic 拆分为 Story"
    snapshot: "定时调度器：Cron/一次性/重复任务，支持消息发送和 AI 任务执行"
  - version: 2
    date: "2026-04-23T18:00:00"
    author: ai
    context: "基于 pkg/scheduler/ 源码逆向补充详细验收标准"
    reason: "代码级验收标准细化"
    snapshot: "定时调度器：robfig/cron 引擎(秒级)，3种调度类型(cron/interval/once)，2种动作(send_message/agent_task)，JSON文件持久化，全局单例，TaskExecutor 可插拔"
source_code:
  - pkg/scheduler/scheduler.go
  - pkg/scheduler/store.go
  - pkg/scheduler/task.go
  - pkg/scheduler/runner.go
  - pkg/scheduler/errors.go
---

# 定时任务调度器

## 描述
定时任务调度器，支持 Cron 表达式、一次性任务和固定间隔重复任务。两种任务动作类型：定时发送消息（ActionTypeSendMessage）和定时执行 AI 任务（ActionTypeAgentTask）。底层使用 `robfig/cron/v3` 引擎（秒级精度），任务持久化到 JSON 文件。任务执行器通过 `TaskExecutor` 接口可插拔，未配置时使用默认执行逻辑。

## 验收标准
- [x] 调度器核心：Cron 表达式解析、任务创建和管理
- [x] 任务类型：ActionTypeSendMessage（发送消息）、ActionTypeAgentTask（AI 任务）
- [x] 消息发送：通过指定渠道或主渠道发送定时消息
- [x] AI 任务：调用 Agent Bridge 获取 AI 回复并发送
- [x] 上下文传递：session_id、user_id、is_group、channel_type、receiver
- [x] 主渠道回退：指定渠道不可用时回退到主渠道
- [x] Agent scheduler 工具：通过 Tool 接口管理定时任务
- [x] 调度引擎使用 `robfig/cron/v3`，启用秒级精度（`cron.WithSeconds()`）和 panic 恢复（`cron.Recover`）
- [x] 三种调度类型：ScheduleTypeCron（标准 cron 表达式）、ScheduleTypeInterval（固定间隔秒数，转换为 `@every Ns`）、ScheduleTypeOnce（一次性，RFC3339 时间解析为 6 字段 cron 表达式）
- [x] 一次性任务(ScheduleTypeOnce)执行后自动从 cron 调度中移除并从 Store 中删除
- [x] 任务 ID 生成规则：`time.Now().Format("20060102150405")` + 4 字符随机后缀（a-z0-9），默认 Enabled=true
- [x] 任务运行时状态跟踪：NextRunAt（下次执行时间，从 cron.Entry.Next 获取）、LastRunAt（上次执行时间）、RunCount（执行次数），每次执行后持久化更新
- [x] Store 使用 JSON 文件持久化，默认路径 `~/.simpleclaw/scheduler/tasks.json`，创建时自动建目录（0755 权限）
- [x] Store 读写锁保护（sync.RWMutex），Save/Delete 操作后自动调用 persist 写入文件，JSON 缩进格式（MarshalIndent）
- [x] Store 加载时文件不存在不报错（`os.IsNotExist` 返回 nil），反序列化为 `[]*Task` 后按 ID 建立 map 索引
- [x] TaskExecutor 接口：`Execute(ctx context.Context, task *Task) (string, error)`，通过 `WithExecutor` RunnerOption 注入
- [x] 默认执行逻辑 `executeDefault`：send_message 动作返回 `[定时提醒] {content}` 格式字符串；agent_task 动作返回错误 "AI 任务执行器未配置"；消息内容为空返回错误
- [x] Scheduler Start()：从 Store 加载所有已启用任务并调度，调用 `cron.Start()` 启动引擎，重复调用返回 nil（幂等）
- [x] Scheduler Stop()：调用 `cron.Stop()` 停止引擎，`cancel()` 取消上下文，设置 running=false
- [x] AddTask：保存到 Store，若 Enabled=true 则立即调度（先移除已有同 ID 调度再重新添加）
- [x] RemoveTask：从 cron 移除调度条目，从 Store 删除任务
- [x] EnableTask/DisableTask：切换 Enabled 状态，启用时重新调度，禁用时移除 cron 条目，状态未变时跳过（幂等）
- [x] runTask 包含 panic 恢复（defer recover），执行后更新 LastRunAt 和 RunCount，非一次性任务更新 NextRunAt
- [x] 全局单例模式：`GetScheduler()` 通过 `sync.Once` 懒初始化，`SetScheduler()` 通过 `sync.Once` 一次性设置
- [x] 错误定义：ErrTaskNotFound（任务不存在）、ErrInvalidSchedule（无效的调度配置）、ErrInvalidAction（无效的任务动作）、ErrSchedulerNotStart（调度器未启动）
- [x] TaskContext 包含完整上下文：ChannelType、Receiver、ReceiveIDType、UserID、GroupID、SessionID、IsGroup、Extra(map[string]any 自定义扩展)

## 代码参考
| 验收标准 | 代码位置 |
|---------|---------|
| cron 引擎 + 秒级 + Recover | `pkg/scheduler/scheduler.go:New` (L33-L37) |
| 三种调度类型 | `pkg/scheduler/task.go:ScheduleType` 常量 (L11-L15), `scheduler.go:buildCronSpec` (L244-L268) |
| 一次性任务自动删除 | `pkg/scheduler/scheduler.go:runTask` (L302-L311) |
| 任务 ID 生成 | `pkg/scheduler/task.go:generateTaskID` (L84-L86), `randomSuffix` (L89-L97) |
| 运行时状态跟踪 | `pkg/scheduler/scheduler.go:runTask` (L284-L318), `scheduleTask` (L233-L236) |
| Store JSON 持久化 | `pkg/scheduler/store.go:NewStore` (L19-L39), `persist` (L114-L126) |
| Store 读写锁 + 自动 persist | `pkg/scheduler/store.go:Save` (L45-L52), `Delete` (L68-L75) |
| Store 加载容错 | `pkg/scheduler/store.go:load` (L91-L111) |
| TaskExecutor 接口 | `pkg/scheduler/runner.go:TaskExecutor` (L11-L13), `WithExecutor` (L35-L39) |
| 默认执行逻辑 | `pkg/scheduler/runner.go:executeDefault` (L50-L63) |
| Start 加载 + 幂等 | `pkg/scheduler/scheduler.go:Start` (L67-L93) |
| Stop 清理 | `pkg/scheduler/scheduler.go:Stop` (L96-L109) |
| AddTask 保存+调度 | `pkg/scheduler/scheduler.go:AddTask` (L112-L131) |
| RemoveTask 移除+删除 | `pkg/scheduler/scheduler.go:RemoveTask` (L134-L151) |
| Enable/Disable 切换 | `pkg/scheduler/scheduler.go:setTaskEnabled` (L164-L200) |
| runTask panic 恢复 | `pkg/scheduler/scheduler.go:runTask` (L272-L278) |
| 全局单例 | `pkg/scheduler/scheduler.go:GetScheduler` (L326-L333), `SetScheduler` (L336-L341) |
| 错误定义 | `pkg/scheduler/errors.go` (L7-L11) |
| TaskContext 上下文 | `pkg/scheduler/task.go:TaskContext` (L42-L51) |
