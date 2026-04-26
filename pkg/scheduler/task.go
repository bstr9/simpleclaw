// Package scheduler 提供定时任务调度服务
package scheduler

import (
	cryptoRand "crypto/rand"
	"fmt"
	"time"
)

// ScheduleType 调度类型
type ScheduleType string

const (
	ScheduleTypeCron     ScheduleType = "cron"     // cron 表达式
	ScheduleTypeInterval ScheduleType = "interval" // 固定间隔
	ScheduleTypeOnce     ScheduleType = "once"     // 一次性
)

// ActionType 任务动作类型
type ActionType string

const (
	ActionTypeSendMessage ActionType = "send_message" // 发送固定消息
	ActionTypeAgentTask   ActionType = "agent_task"   // 执行 AI 任务
)

// ScheduleConfig 调度配置
type ScheduleConfig struct {
	Type       ScheduleType `json:"type"`
	Expression string       `json:"expression,omitempty"` // cron 表达式
	Seconds    int          `json:"seconds,omitempty"`    // 间隔秒数
	RunAt      string       `json:"run_at,omitempty"`     // 一次性执行时间 (RFC3339)
}

// ActionConfig 任务动作配置
type ActionConfig struct {
	Type            ActionType   `json:"type"`
	Content         string       `json:"content,omitempty"`
	TaskDescription string       `json:"task_description,omitempty"`
	Context         *TaskContext `json:"context,omitempty"`
}

// TaskContext 任务执行上下文
type TaskContext struct {
	ChannelType   string         `json:"channel_type,omitempty"`
	Receiver      string         `json:"receiver,omitempty"`
	ReceiveIDType string         `json:"receive_id_type,omitempty"`
	UserID        string         `json:"user_id,omitempty"`
	GroupID       string         `json:"group_id,omitempty"`
	SessionID     string         `json:"session_id,omitempty"`
	IsGroup       bool           `json:"is_group,omitempty"`
	Extra         map[string]any `json:"extra,omitempty"`
}

// Task 定时任务
type Task struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Enabled   bool           `json:"enabled"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Schedule  ScheduleConfig `json:"schedule"`
	Action    ActionConfig   `json:"action"`

	// 运行时状态
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	RunCount  int        `json:"run_count"`
}

// NewTask 创建新任务
func NewTask(name string, schedule ScheduleConfig, action ActionConfig) *Task {
	now := time.Now()
	return &Task{
		ID:        generateTaskID(),
		Name:      name,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
		Schedule:  schedule,
		Action:    action,
	}
}

// generateTaskID 生成任务 ID
func generateTaskID() string {
	return time.Now().Format("20060102150405") + randomSuffix(4)
}

// randomSuffix 生成随机后缀
func randomSuffix(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	randBuf := make([]byte, n)
	// crypto/rand.Read 在所有主流操作系统上几乎不可能失败，
	// 如果失败说明系统熵源故障，应立即终止而非生成可预测的 ID
	if _, err := cryptoRand.Read(randBuf); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read 失败: %v", err))
	}
	for i := range b {
		b[i] = letters[int(randBuf[i])%len(letters)]
	}
	return string(b)
}
