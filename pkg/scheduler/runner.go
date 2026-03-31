// Package scheduler 提供定时任务调度服务
package scheduler

import (
	"context"
	"errors"
	"fmt"
)

// TaskExecutor 任务执行器接口
type TaskExecutor interface {
	Execute(ctx context.Context, task *Task) (string, error)
}

// Runner 任务执行器
type Runner struct {
	executor TaskExecutor
}

// NewRunner 创建任务执行器
func NewRunner(opts ...RunnerOption) *Runner {
	r := &Runner{}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// RunnerOption 执行器选项
type RunnerOption func(*Runner)

// WithExecutor 设置任务执行器
func WithExecutor(executor TaskExecutor) RunnerOption {
	return func(r *Runner) {
		r.executor = executor
	}
}

// Run 执行任务
func (r *Runner) Run(ctx context.Context, task *Task) (string, error) {
	if r.executor == nil {
		return r.executeDefault(ctx, task)
	}
	return r.executor.Execute(ctx, task)
}

// executeDefault 默认执行逻辑
func (r *Runner) executeDefault(ctx context.Context, task *Task) (string, error) {
	switch task.Action.Type {
	case ActionTypeSendMessage:
		content := task.Action.Content
		if content == "" {
			return "", errors.New("消息内容为空")
		}
		return fmt.Sprintf("[定时提醒] %s", content), nil
	case ActionTypeAgentTask:
		return "", errors.New("AI 任务执行器未配置")
	default:
		return "", fmt.Errorf("未知的任务类型: %s", task.Action.Type)
	}
}
