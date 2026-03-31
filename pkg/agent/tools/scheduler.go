// Package tools 提供内置工具实现
package tools

import (
	"fmt"
	"time"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/scheduler"
)

// SchedulerTool 计划任务工具
type SchedulerTool struct {
	toolCtx *agent.ToolContext
}

// NewSchedulerTool 创建计划任务工具实例
func NewSchedulerTool() *SchedulerTool {
	return &SchedulerTool{}
}

// Name 返回工具名称
func (t *SchedulerTool) Name() string {
	return "cron"
}

// Description 返回工具描述
func (t *SchedulerTool) Description() string {
	return `管理定时任务和提醒 (cron)。

操作类型 (action):
- create: 创建定时任务
- list: 列出所有任务
- get: 查询任务详情
- delete: 删除任务
- enable: 启用任务
- disable: 禁用任务

创建任务示例:
- 30秒后提醒: action="create", schedule_type="once", schedule_value="+30s"
- 每分钟提醒: action="create", schedule_type="interval", schedule_value="60"
- 每天8点提醒: action="create", schedule_type="cron", schedule_value="0 8 * * *"

重要：必须实际调用此工具来创建任务，不能只是口头回复用户。`
}

// Parameters 返回参数 JSON Schema
func (t *SchedulerTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "操作类型: create(创建), list(列表), get(查询), delete(删除), enable(启用), disable(禁用)",
				"enum":        []string{"create", "list", "get", "delete", "enable", "disable"},
			},
			"task_id": map[string]any{
				"type":        "string",
				"description": "任务ID（用于 get/delete/enable/disable 操作）",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "任务名称（用于 create 操作）",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "固定消息内容（与 ai_task 二选一）",
			},
			"ai_task": map[string]any{
				"type":        "string",
				"description": "AI任务描述（与 message 二选一）",
			},
			"schedule_type": map[string]any{
				"type":        "string",
				"description": "调度类型: cron(cron表达式), interval(固定间隔秒数), once(一次性)",
				"enum":        []string{"cron", "interval", "once"},
			},
			"schedule_value": map[string]any{
				"type":        "string",
				"description": "调度值: cron表达式/间隔秒数/时间(+5s,+10m,+1h或ISO格式)",
			},
		},
		"required": []string{"action"},
	}
}

// Stage 返回工具执行阶段
func (t *SchedulerTool) Stage() agent.ToolStage {
	return agent.ToolStagePostProcess
}

// Execute 执行工具（无上下文）
func (t *SchedulerTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	return t.ExecuteWithContext(nil, params)
}

// ExecuteWithContext 执行工具（带上下文）
func (t *SchedulerTool) ExecuteWithContext(ctx *agent.ToolContext, params map[string]any) (*agent.ToolResult, error) {
	t.toolCtx = ctx

	action, _ := params["action"].(string)
	if action == "" {
		return agent.NewErrorToolResult(fmt.Errorf("action 参数是必需的")), nil
	}

	switch action {
	case "create":
		return t.handleCreate(params)
	case "list":
		return t.handleList()
	case "get":
		return t.handleGet(params)
	case "delete":
		return t.handleDelete(params)
	case "enable":
		return t.handleEnable(params)
	case "disable":
		return t.handleDisable(params)
	default:
		return agent.NewErrorToolResult(fmt.Errorf("不支持的操作: %s", action)), nil
	}
}

func (t *SchedulerTool) handleCreate(params map[string]any) (*agent.ToolResult, error) {
	name, _ := params["name"].(string)
	message, _ := params["message"].(string)
	aiTask, _ := params["ai_task"].(string)
	scheduleType, _ := params["schedule_type"].(string)
	scheduleValue, _ := params["schedule_value"].(string)

	if name == "" {
		return agent.NewErrorToolResult(fmt.Errorf("缺少任务名称 (name)")), nil
	}

	if message == "" && aiTask == "" {
		return agent.NewErrorToolResult(fmt.Errorf("必须提供 message（固定消息）或 ai_task（AI任务）之一")), nil
	}

	if message != "" && aiTask != "" {
		return agent.NewErrorToolResult(fmt.Errorf("message 和 ai_task 只能提供其中一个")), nil
	}

	if scheduleType == "" {
		return agent.NewErrorToolResult(fmt.Errorf("缺少调度类型 (schedule_type)")), nil
	}

	if scheduleValue == "" {
		return agent.NewErrorToolResult(fmt.Errorf("缺少调度值 (schedule_value)")), nil
	}

	schedule, err := t.parseSchedule(scheduleType, scheduleValue)
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("无效的调度配置: %w", err)), nil
	}

	var actionConfig scheduler.ActionConfig
	if message != "" {
		actionConfig = scheduler.ActionConfig{
			Type:    scheduler.ActionTypeSendMessage,
			Content: message,
		}
	} else {
		actionConfig = scheduler.ActionConfig{
			Type:            scheduler.ActionTypeAgentTask,
			TaskDescription: aiTask,
		}
	}

	// 保存上下文到任务
	if t.toolCtx != nil {
		actionConfig.Context = &scheduler.TaskContext{
			SessionID:     t.toolCtx.SessionID,
			UserID:        t.toolCtx.UserID,
			GroupID:       t.toolCtx.GroupID,
			IsGroup:       t.toolCtx.IsGroup,
			ChannelType:   t.toolCtx.ChannelType,
			Receiver:      t.toolCtx.Receiver,
			ReceiveIDType: t.toolCtx.ReceiveIDType,
			Extra:         t.toolCtx.Extra,
		}
	}

	task := scheduler.NewTask(name, *schedule, actionConfig)

	s := scheduler.GetScheduler()
	if err := s.AddTask(task); err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("创建任务失败: %w", err)), nil
	}

	scheduleDesc := t.formatScheduleDescription(*schedule)
	contentDesc := "固定消息"
	if aiTask != "" {
		contentDesc = "AI任务"
	}

	nextRun := ""
	if task.NextRunAt != nil {
		nextRun = task.NextRunAt.Format(time.RFC3339)
	}

	return agent.NewToolResult(map[string]any{
		"message":      "定时任务创建成功",
		"task_id":      task.ID,
		"name":         name,
		"schedule":     scheduleDesc,
		"content":      message + aiTask,
		"content_type": contentDesc,
		"next_run":     nextRun,
	}), nil
}

func (t *SchedulerTool) handleList() (*agent.ToolResult, error) {
	s := scheduler.GetScheduler()
	tasks := s.ListTasks()

	if len(tasks) == 0 {
		return agent.NewToolResult(map[string]any{
			"message": "暂无定时任务",
			"tasks":   []any{},
		}), nil
	}

	var taskList []map[string]any
	for _, task := range tasks {
		nextRun := ""
		if task.NextRunAt != nil {
			nextRun = task.NextRunAt.Format(time.RFC3339)
		}
		taskList = append(taskList, map[string]any{
			"id":       task.ID,
			"name":     task.Name,
			"enabled":  task.Enabled,
			"schedule": t.formatScheduleDescription(task.Schedule),
			"next_run": nextRun,
		})
	}

	return agent.NewToolResult(map[string]any{
		"message": fmt.Sprintf("共 %d 个定时任务", len(tasks)),
		"tasks":   taskList,
	}), nil
}

func (t *SchedulerTool) handleGet(params map[string]any) (*agent.ToolResult, error) {
	taskID, _ := params["task_id"].(string)
	if taskID == "" {
		return agent.NewErrorToolResult(fmt.Errorf(common.ErrMissingTaskID)), nil
	}

	s := scheduler.GetScheduler()
	task, err := s.GetTask(taskID)
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf(common.ErrTaskNotFoundFmt, taskID)), nil
	}

	nextRun := ""
	if task.NextRunAt != nil {
		nextRun = task.NextRunAt.Format(time.RFC3339)
	}
	lastRun := ""
	if task.LastRunAt != nil {
		lastRun = task.LastRunAt.Format(time.RFC3339)
	}

	return agent.NewToolResult(map[string]any{
		"id":         task.ID,
		"name":       task.Name,
		"enabled":    task.Enabled,
		"schedule":   t.formatScheduleDescription(task.Schedule),
		"action":     task.Action,
		"next_run":   nextRun,
		"last_run":   lastRun,
		"created_at": task.CreatedAt.Format(time.RFC3339),
	}), nil
}

func (t *SchedulerTool) handleDelete(params map[string]any) (*agent.ToolResult, error) {
	taskID, _ := params["task_id"].(string)
	if taskID == "" {
		return agent.NewErrorToolResult(fmt.Errorf(common.ErrMissingTaskID)), nil
	}

	s := scheduler.GetScheduler()
	task, err := s.GetTask(taskID)
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf(common.ErrTaskNotFoundFmt, taskID)), nil
	}

	if err := s.RemoveTask(taskID); err != nil {
		return agent.NewErrorToolResult(fmt.Errorf("删除任务失败: %w", err)), nil
	}

	return agent.NewToolResult(map[string]any{
		"message": fmt.Sprintf("任务 '%s' (%s) 已删除", task.Name, taskID),
	}), nil
}

func (t *SchedulerTool) handleEnable(params map[string]any) (*agent.ToolResult, error) {
	return t.setTaskEnabled(params, true)
}

func (t *SchedulerTool) handleDisable(params map[string]any) (*agent.ToolResult, error) {
	return t.setTaskEnabled(params, false)
}

func (t *SchedulerTool) setTaskEnabled(params map[string]any, enabled bool) (*agent.ToolResult, error) {
	taskID, _ := params["task_id"].(string)
	if taskID == "" {
		return agent.NewErrorToolResult(fmt.Errorf(common.ErrMissingTaskID)), nil
	}

	s := scheduler.GetScheduler()
	task, err := s.GetTask(taskID)
	if err != nil {
		return agent.NewErrorToolResult(fmt.Errorf(common.ErrTaskNotFoundFmt, taskID)), nil
	}

	if enabled {
		if err := s.EnableTask(taskID); err != nil {
			return agent.NewErrorToolResult(fmt.Errorf("启用任务失败: %w", err)), nil
		}
	} else {
		if err := s.DisableTask(taskID); err != nil {
			return agent.NewErrorToolResult(fmt.Errorf("禁用任务失败: %w", err)), nil
		}
	}

	status := "已启用"
	if !enabled {
		status = "已禁用"
	}
	return agent.NewToolResult(map[string]any{
		"message": fmt.Sprintf("任务 '%s' (%s) %s", task.Name, taskID, status),
	}), nil
}

func (t *SchedulerTool) parseSchedule(scheduleType, scheduleValue string) (*scheduler.ScheduleConfig, error) {
	switch scheduleType {
	case "cron":
		return &scheduler.ScheduleConfig{Type: scheduler.ScheduleTypeCron, Expression: scheduleValue}, nil
	case "interval":
		var seconds int
		_, err := fmt.Sscanf(scheduleValue, "%d", &seconds)
		if err != nil || seconds <= 0 {
			return nil, fmt.Errorf("无效的间隔秒数: %s", scheduleValue)
		}
		return &scheduler.ScheduleConfig{Type: scheduler.ScheduleTypeInterval, Seconds: seconds}, nil
	case "once":
		var runAt time.Time
		if len(scheduleValue) > 0 && scheduleValue[0] == '+' {
			duration, err := t.parseRelativeTime(scheduleValue)
			if err != nil {
				return nil, err
			}
			runAt = time.Now().Add(duration)
		} else {
			var err error
			runAt, err = time.Parse(time.RFC3339, scheduleValue)
			if err != nil {
				return nil, fmt.Errorf("无效的时间格式: %w", err)
			}
		}
		return &scheduler.ScheduleConfig{Type: scheduler.ScheduleTypeOnce, RunAt: runAt.Format(time.RFC3339)}, nil
	default:
		return nil, fmt.Errorf("未知的调度类型: %s", scheduleType)
	}
}

func (t *SchedulerTool) parseRelativeTime(value string) (time.Duration, error) {
	var amount int
	var unit string
	_, err := fmt.Sscanf(value, "+%d%s", &amount, &unit)
	if err != nil {
		return 0, fmt.Errorf("无效的相对时间格式: %s", value)
	}

	switch unit {
	case "s":
		return time.Duration(amount) * time.Second, nil
	case "m":
		return time.Duration(amount) * time.Minute, nil
	case "h":
		return time.Duration(amount) * time.Hour, nil
	case "d":
		return time.Duration(amount) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("未知的时间单位: %s", unit)
	}
}

func (t *SchedulerTool) formatScheduleDescription(schedule scheduler.ScheduleConfig) string {
	switch schedule.Type {
	case scheduler.ScheduleTypeCron:
		return fmt.Sprintf("Cron: %s", schedule.Expression)
	case scheduler.ScheduleTypeInterval:
		seconds := schedule.Seconds
		if seconds >= 86400 {
			return fmt.Sprintf("每 %d 天", seconds/86400)
		} else if seconds >= 3600 {
			return fmt.Sprintf("每 %d 小时", seconds/3600)
		} else if seconds >= 60 {
			return fmt.Sprintf("每 %d 分钟", seconds/60)
		}
		return fmt.Sprintf("每 %d 秒", seconds)
	case scheduler.ScheduleTypeOnce:
		runAt, _ := time.Parse(time.RFC3339, schedule.RunAt)
		return fmt.Sprintf("一次性 (%s)", runAt.Format("2006-01-02 15:04"))
	}
	return "未知"
}
