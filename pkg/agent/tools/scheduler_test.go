package tools

import (
	"testing"
	"time"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/scheduler"
)

func TestSchedulerTool_Name(t *testing.T) {
	tool := NewSchedulerTool()
	if tool.Name() != "cron" {
		t.Errorf("Expected name 'cron', got '%s'", tool.Name())
	}
}

func TestSchedulerTool_Description(t *testing.T) {
	tool := NewSchedulerTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestSchedulerTool_Parameters(t *testing.T) {
	tool := NewSchedulerTool()
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Error("Expected type to be 'object'")
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	requiredFields := []string{"action", "task_id", "name", "message", "ai_task", "schedule_type", "schedule_value"}
	for _, field := range requiredFields {
		if _, exists := props[field]; !exists {
			t.Errorf("Expected property '%s' to exist", field)
		}
	}
}

func TestSchedulerTool_Stage(t *testing.T) {
	tool := NewSchedulerTool()
	if tool.Stage() != agent.ToolStagePostProcess {
		t.Error("Expected stage to be ToolStagePostProcess")
	}
}

func TestSchedulerTool_Execute_MissingAction(t *testing.T) {
	tool := NewSchedulerTool()
	result, err := tool.Execute(map[string]any{})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing action")
	}
}

func TestSchedulerTool_Execute_InvalidAction(t *testing.T) {
	tool := NewSchedulerTool()
	result, err := tool.Execute(map[string]any{"action": "invalid"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for invalid action")
	}
}

func TestSchedulerTool_Execute_CreateTask(t *testing.T) {
	// Initialize scheduler for testing
	s := scheduler.New()
	s.Start()
	scheduler.SetScheduler(s)

	// Clear any existing tasks
	for _, task := range s.ListTasks() {
		s.RemoveTask(task.ID)
	}

	tool := NewSchedulerTool()

	params := map[string]any{
		"action":         "create",
		"name":           "测试任务",
		"message":        "这是一条测试消息",
		"schedule_type":  "once",
		"schedule_value": "+30s",
	}

	result, err := tool.Execute(params)

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	// Verify task was created
	tasks := s.ListTasks()
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
}

func TestSchedulerTool_Execute_CreateTask_MissingName(t *testing.T) {
	tool := NewSchedulerTool()

	params := map[string]any{
		"action":         "create",
		"message":        "测试消息",
		"schedule_type":  "once",
		"schedule_value": "+30s",
	}

	result, _ := tool.Execute(params)

	if result.IsSuccess() {
		t.Error("Expected error result for missing name")
	}
}

func TestSchedulerTool_Execute_CreateTask_MissingContent(t *testing.T) {
	tool := NewSchedulerTool()

	params := map[string]any{
		"action":         "create",
		"name":           "测试任务",
		"schedule_type":  "once",
		"schedule_value": "+30s",
	}

	result, _ := tool.Execute(params)

	if result.IsSuccess() {
		t.Error("Expected error result for missing content")
	}
}

func TestSchedulerTool_Execute_CreateTask_MissingSchedule(t *testing.T) {
	tool := NewSchedulerTool()

	params := map[string]any{
		"action":  "create",
		"name":    "测试任务",
		"message": "测试消息",
	}

	result, _ := tool.Execute(params)

	if result.IsSuccess() {
		t.Error("Expected error result for missing schedule")
	}
}

func TestSchedulerTool_Execute_ListTasks(t *testing.T) {
	// Initialize scheduler
	s := scheduler.New()
	s.Start()
	scheduler.SetScheduler(s)

	for _, task := range s.ListTasks() {
		s.RemoveTask(task.ID)
	}

	tool := NewSchedulerTool()

	// Create a task first
	_, _ = tool.Execute(map[string]any{
		"action":         "create",
		"name":           "列表测试任务",
		"message":        "测试消息",
		"schedule_type":  "once",
		"schedule_value": "+1h",
	})

	// List tasks
	result, err := tool.Execute(map[string]any{"action": "list"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}
}

func TestSchedulerTool_Execute_GetTask(t *testing.T) {
	// Initialize scheduler
	s := scheduler.New()
	s.Start()
	scheduler.SetScheduler(s)

	for _, task := range s.ListTasks() {
		s.RemoveTask(task.ID)
	}

	tool := NewSchedulerTool()

	// Create a task
	createResult, _ := tool.Execute(map[string]any{
		"action":         "create",
		"name":           "获取测试任务",
		"message":        "测试消息",
		"schedule_type":  "once",
		"schedule_value": "+1h",
	})

	taskID := createResult.Result.(map[string]any)["task_id"].(string)

	// Get task
	result, err := tool.Execute(map[string]any{
		"action":  "get",
		"task_id": taskID,
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}
}

func TestSchedulerTool_Execute_DeleteTask(t *testing.T) {
	// Initialize scheduler
	s := scheduler.New()
	s.Start()
	scheduler.SetScheduler(s)

	for _, task := range s.ListTasks() {
		s.RemoveTask(task.ID)
	}

	tool := NewSchedulerTool()

	// Create a task
	createResult, _ := tool.Execute(map[string]any{
		"action":         "create",
		"name":           "删除测试任务",
		"message":        "测试消息",
		"schedule_type":  "once",
		"schedule_value": "+1h",
	})

	taskID := createResult.Result.(map[string]any)["task_id"].(string)

	// Delete task
	result, err := tool.Execute(map[string]any{
		"action":  "delete",
		"task_id": taskID,
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	// Verify task was deleted
	tasks := s.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks after delete, got %d", len(tasks))
	}
}

func TestSchedulerTool_Execute_EnableDisableTask(t *testing.T) {
	// Initialize scheduler
	s := scheduler.New()
	s.Start()
	scheduler.SetScheduler(s)

	for _, task := range s.ListTasks() {
		s.RemoveTask(task.ID)
	}

	tool := NewSchedulerTool()

	// Create a task
	createResult, _ := tool.Execute(map[string]any{
		"action":         "create",
		"name":           "启用禁用测试任务",
		"message":        "测试消息",
		"schedule_type":  "interval",
		"schedule_value": "60",
	})

	taskID := createResult.Result.(map[string]any)["task_id"].(string)

	// Disable task
	disableResult, _ := tool.Execute(map[string]any{
		"action":  "disable",
		"task_id": taskID,
	})

	if !disableResult.IsSuccess() {
		t.Errorf("Expected success result for disable, got: %v", disableResult)
	}

	// Enable task
	enableResult, _ := tool.Execute(map[string]any{
		"action":  "enable",
		"task_id": taskID,
	})

	if !enableResult.IsSuccess() {
		t.Errorf("Expected success result for enable, got: %v", enableResult)
	}
}

func TestSchedulerTool_ParseSchedule(t *testing.T) {
	tool := NewSchedulerTool()

	tests := []struct {
		name          string
		scheduleType  string
		scheduleValue string
		wantErr       bool
	}{
		{"cron表达式", "cron", "0 8 * * *", false},
		{"间隔秒数", "interval", "60", false},
		{"相对时间", "once", "+30s", false},
		{"绝对时间", "once", time.Now().Add(1 * time.Hour).Format(time.RFC3339), false},
		{"无效间隔", "interval", "invalid", true},
		{"无效时间", "once", "invalid", true},
		{"未知类型", "unknown", "value", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.parseSchedule(tt.scheduleType, tt.scheduleValue)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSchedule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSchedulerTool_ParseRelativeTime(t *testing.T) {
	tool := NewSchedulerTool()

	tests := []struct {
		input   string
		wantErr bool
		minDur  time.Duration
	}{
		{"+30s", false, 29 * time.Second},
		{"+5m", false, 4 * time.Minute},
		{"+1h", false, 59 * time.Minute},
		{"+1d", false, 23 * time.Hour},
		{"invalid", true, 0},
		{"+30", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			dur, err := tool.parseRelativeTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRelativeTime() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && dur < tt.minDur {
				t.Errorf("parseRelativeTime() duration = %v, want >= %v", dur, tt.minDur)
			}
		})
	}
}

func TestSchedulerTool_FormatScheduleDescription(t *testing.T) {
	tool := NewSchedulerTool()

	tests := []struct {
		name     string
		schedule scheduler.ScheduleConfig
		contains string
	}{
		{"cron", scheduler.ScheduleConfig{Type: scheduler.ScheduleTypeCron, Expression: "0 8 * * *"}, "Cron"},
		{"间隔秒", scheduler.ScheduleConfig{Type: scheduler.ScheduleTypeInterval, Seconds: 30}, "秒"},
		{"间隔分钟", scheduler.ScheduleConfig{Type: scheduler.ScheduleTypeInterval, Seconds: 120}, "分钟"},
		{"间隔小时", scheduler.ScheduleConfig{Type: scheduler.ScheduleTypeInterval, Seconds: 7200}, "小时"},
		{"间隔天", scheduler.ScheduleConfig{Type: scheduler.ScheduleTypeInterval, Seconds: 86400}, "天"},
		{"一次性", scheduler.ScheduleConfig{Type: scheduler.ScheduleTypeOnce, RunAt: time.Now().Format(time.RFC3339)}, "一次性"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := tool.formatScheduleDescription(tt.schedule)
			if desc == "" {
				t.Error("Description should not be empty")
			}
		})
	}
}
