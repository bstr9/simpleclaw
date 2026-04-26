// Package scheduler 提供定时任务调度服务
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/bstr9/simpleclaw/pkg/logger"
)

// Scheduler 调度器
type Scheduler struct {
	mu sync.RWMutex

	cron   *cron.Cron
	store  *Store
	runner *Runner

	entries map[string]cron.EntryID

	ctx    context.Context
	cancel context.CancelFunc

	running bool
}

// New 创建新的调度器
func New(opts ...Option) *Scheduler {
	s := &Scheduler{
		cron:    cron.New(cron.WithSeconds(), cron.WithChain(cron.Recover(cron.DefaultLogger))),
		entries: make(map[string]cron.EntryID),
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.store == nil {
		s.store = NewStore()
	}

	if s.runner == nil {
		s.runner = NewRunner()
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())

	return s
}

// Option 调度器选项
type Option func(*Scheduler)

// WithRunner 设置任务执行器
func WithRunner(runner *Runner) Option {
	return func(s *Scheduler) {
		s.runner = runner
	}
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	tasks := s.store.LoadAll()
	for _, task := range tasks {
		if task.Enabled {
			if err := s.scheduleTask(task); err != nil {
				logger.Warn("[Scheduler] 加载任务失败",
					zap.String("task_id", task.ID),
					zap.Error(err))
			}
		}
	}

	s.cron.Start()
	s.running = true

	logger.Info("[Scheduler] 调度器已启动",
		zap.Int("tasks", len(s.entries)))

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cron.Stop()
	s.cancel()
	s.running = false

	logger.Info("[Scheduler] 调度器已停止")
}

// AddTask 添加任务
func (s *Scheduler) AddTask(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.Save(task); err != nil {
		return fmt.Errorf("保存任务失败: %w", err)
	}

	if task.Enabled {
		if err := s.scheduleTask(task); err != nil {
			return fmt.Errorf("调度任务失败: %w", err)
		}
	}

	logger.Info("[Scheduler] 任务已添加",
		zap.String("task_id", task.ID),
		zap.String("name", task.Name))

	return nil
}

// RemoveTask 移除任务
func (s *Scheduler) RemoveTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.entries[taskID]; exists {
		s.cron.Remove(entryID)
		delete(s.entries, taskID)
	}

	if err := s.store.Delete(taskID); err != nil {
		return fmt.Errorf("删除任务失败: %w", err)
	}

	logger.Info("[Scheduler] 任务已移除",
		zap.String("task_id", taskID))

	return nil
}

// EnableTask 启用任务
func (s *Scheduler) EnableTask(taskID string) error {
	return s.setTaskEnabled(taskID, true)
}

// DisableTask 禁用任务
func (s *Scheduler) DisableTask(taskID string) error {
	return s.setTaskEnabled(taskID, false)
}

// setTaskEnabled 设置任务启用状态
func (s *Scheduler) setTaskEnabled(taskID string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, err := s.store.Get(taskID)
	if err != nil {
		return err
	}

	if task.Enabled == enabled {
		return nil
	}

	task.Enabled = enabled
	task.UpdatedAt = time.Now()

	if enabled {
		if err := s.scheduleTask(task); err != nil {
			return err
		}
	} else {
		if entryID, exists := s.entries[taskID]; exists {
			s.cron.Remove(entryID)
			delete(s.entries, taskID)
		}
	}

	if err := s.store.Save(task); err != nil {
		return fmt.Errorf("保存任务失败: %w", err)
	}

	logger.Info("[Scheduler] 任务状态已更新",
		zap.String("task_id", taskID),
		zap.Bool("enabled", enabled))

	return nil
}

// GetTask 获取任务
func (s *Scheduler) GetTask(taskID string) (*Task, error) {
	return s.store.Get(taskID)
}

// ListTasks 列出所有任务
func (s *Scheduler) ListTasks() []*Task {
	return s.store.LoadAll()
}

// scheduleTask 调度任务
func (s *Scheduler) scheduleTask(task *Task) error {
	if entryID, exists := s.entries[task.ID]; exists {
		s.cron.Remove(entryID)
		delete(s.entries, task.ID)
	}

	spec, err := s.buildCronSpec(task)
	if err != nil {
		return err
	}

	entryID, err := s.cron.AddFunc(spec, func() {
		s.runTask(task)
	})
	if err != nil {
		return fmt.Errorf("添加 cron 任务失败: %w", err)
	}

	s.entries[task.ID] = entryID

	now := time.Now()
	next := s.cron.Entry(entryID).Next
	task.NextRunAt = &next
	task.UpdatedAt = now

	s.store.Save(task)

	return nil
}

// buildCronSpec 构建 cron 表达式
func (s *Scheduler) buildCronSpec(task *Task) (string, error) {
	switch task.Schedule.Type {
	case ScheduleTypeCron:
		return task.Schedule.Expression, nil

	case ScheduleTypeInterval:
		return fmt.Sprintf("@every %ds", task.Schedule.Seconds), nil

	case ScheduleTypeOnce:
		runAt, err := time.Parse(time.RFC3339, task.Schedule.RunAt)
		if err != nil {
			return "", fmt.Errorf("解析时间失败: %w", err)
		}
		return fmt.Sprintf("%d %d %d %d %d %d",
			runAt.Second(),
			runAt.Minute(),
			runAt.Hour(),
			runAt.Day(),
			int(runAt.Month()),
			runAt.Weekday()), nil

	default:
		return "", fmt.Errorf("未知的调度类型: %s", task.Schedule.Type)
	}
}

// runTask 执行任务
func (s *Scheduler) runTask(task *Task) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("[Scheduler] 任务执行 panic",
				zap.String("task_id", task.ID),
				zap.Any("panic", r))
		}
	}()

	logger.Info("[Scheduler] 执行任务",
		zap.String("task_id", task.ID),
		zap.String("name", task.Name))

	now := time.Now()
	task.LastRunAt = &now
	task.RunCount++

	ctx := s.ctx

	result, err := s.runner.Run(ctx, task)
	if err != nil {
		logger.Error("[Scheduler] 任务执行失败",
			zap.String("task_id", task.ID),
			zap.Error(err))
	} else {
		logger.Info("[Scheduler] 任务执行完成",
			zap.String("task_id", task.ID),
			zap.String("result", result))
	}

	s.mu.Lock()
	if task.Schedule.Type == ScheduleTypeOnce {
		if entryID, exists := s.entries[task.ID]; exists {
			s.cron.Remove(entryID)
			delete(s.entries, task.ID)
		}
		s.mu.Unlock()
		s.store.Delete(task.ID)
		logger.Info("[Scheduler] 一次性任务已删除",
			zap.String("task_id", task.ID))
		return
	} else if entryID, exists := s.entries[task.ID]; exists {
		next := s.cron.Entry(entryID).Next
		task.NextRunAt = &next
	}
	s.mu.Unlock()

	s.store.Save(task)
}

var globalScheduler *Scheduler
var globalSchedulerMu sync.Mutex
var globalSchedulerOnce sync.Once

// GetScheduler 获取全局调度器
// 使用 sync.Once 保证只初始化一次，避免并发竞态
func GetScheduler() *Scheduler {
	globalSchedulerOnce.Do(func() {
		if globalScheduler == nil {
			globalScheduler = New()
		}
	})
	globalSchedulerMu.Lock()
	defer globalSchedulerMu.Unlock()
	return globalScheduler
}

// SetScheduler 设置全局调度器
func SetScheduler(s *Scheduler) {
	globalSchedulerMu.Lock()
	defer globalSchedulerMu.Unlock()
	globalScheduler = s
	// 重置 sync.Once，允许后续 GetScheduler 重新检测
	globalSchedulerOnce = sync.Once{}
}
