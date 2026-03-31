// Package scheduler 提供定时任务调度服务
package scheduler

import "errors"

// 错误定义
var (
	ErrTaskNotFound      = errors.New("任务不存在")
	ErrInvalidSchedule   = errors.New("无效的调度配置")
	ErrInvalidAction     = errors.New("无效的任务动作")
	ErrSchedulerNotStart = errors.New("调度器未启动")
)
