// Package scheduler 提供定时任务调度服务
package scheduler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Store 任务存储
type Store struct {
	mu       sync.RWMutex
	filePath string
	tasks    map[string]*Task
}

// NewStore 创建任务存储
func NewStore(opts ...StoreOption) *Store {
	s := &Store{
		tasks: make(map[string]*Task),
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.filePath == "" {
		homeDir, _ := os.UserHomeDir()
		s.filePath = filepath.Join(homeDir, ".simpleclaw", "scheduler", "tasks.json")
	}

	dir := filepath.Dir(s.filePath)
	os.MkdirAll(dir, 0755)

	s.load()

	return s
}

// StoreOption 存储选项
type StoreOption func(*Store)

// Save 保存任务
func (s *Store) Save(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[task.ID] = task

	return s.persist()
}

// Get 获取任务
func (s *Store) Get(taskID string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}

	return task, nil
}

// Delete 删除任务
func (s *Store) Delete(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tasks, taskID)

	return s.persist()
}

// LoadAll 加载所有任务
func (s *Store) LoadAll() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// load 从文件加载任务
func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var tasks []*Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return err
	}

	s.tasks = make(map[string]*Task)
	for _, task := range tasks {
		s.tasks[task.ID] = task
	}

	return nil
}

// persist 持久化任务到文件
func (s *Store) persist() error {
	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}
