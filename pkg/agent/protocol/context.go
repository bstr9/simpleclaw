// Package protocol 提供 Agent 交互协议的核心定义。
// context.go 定义了协议上下文相关的类型和接口。
package protocol

import (
	"sync"
	"time"
)

// TeamContext 表示团队协作上下文
type TeamContext struct {
	// Name 团队名称
	Name string `json:"name"`
	// Description 团队描述
	Description string `json:"description"`
	// Rule 团队规则
	Rule string `json:"rule"`
	// Agents 团队中的 Agent 列表
	Agents []string `json:"agents"`
	// UserTask 用户任务（向后兼容）
	UserTask string `json:"user_task"`
	// Task 当前任务实例
	Task *Task `json:"task"`
	// TaskShortName 任务目录名称
	TaskShortName string `json:"task_short_name"`
	// AgentOutputs 已执行的 Agent 输出列表
	AgentOutputs []*AgentOutput `json:"agent_outputs"`
	// CurrentSteps 当前执行步数
	CurrentSteps int `json:"current_steps"`
	// MaxSteps 最大执行步数
	MaxSteps int `json:"max_steps"`
	// mu 读写锁
	mu sync.RWMutex
}

// NewTeamContext 创建新的团队上下文
func NewTeamContext(name, description, rule string, agents []string, maxSteps int) *TeamContext {
	if maxSteps <= 0 {
		maxSteps = 100
	}
	return &TeamContext{
		Name:         name,
		Description:  description,
		Rule:         rule,
		Agents:       agents,
		AgentOutputs: make([]*AgentOutput, 0),
		CurrentSteps: 0,
		MaxSteps:     maxSteps,
	}
}

// SetTask 设置当前任务
func (c *TeamContext) SetTask(task *Task) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Task = task
	c.UserTask = task.Content
}

// GetTask 获取当前任务
func (c *TeamContext) GetTask() *Task {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Task
}

// AddAgentOutput 添加 Agent 输出
func (c *TeamContext) AddAgentOutput(output *AgentOutput) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AgentOutputs = append(c.AgentOutputs, output)
	c.CurrentSteps++
}

// GetAgentOutputs 获取所有 Agent 输出
func (c *TeamContext) GetAgentOutputs() []*AgentOutput {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*AgentOutput, len(c.AgentOutputs))
	copy(result, c.AgentOutputs)
	return result
}

// GetCurrentSteps 获取当前执行步数
func (c *TeamContext) GetCurrentSteps() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.CurrentSteps
}

// IncrementSteps 增加执行步数
func (c *TeamContext) IncrementSteps() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CurrentSteps++
	return c.CurrentSteps
}

// CanContinue 检查是否可以继续执行
func (c *TeamContext) CanContinue() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.CurrentSteps < c.MaxSteps
}

// Reset 重置上下文
func (c *TeamContext) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Task = nil
	c.UserTask = ""
	c.TaskShortName = ""
	c.AgentOutputs = make([]*AgentOutput, 0)
	c.CurrentSteps = 0
}

// AgentOutput 表示 Agent 的执行输出
type AgentOutput struct {
	// AgentName Agent 名称
	AgentName string `json:"agent_name"`
	// Output 输出内容
	Output string `json:"output"`
	// Timestamp 输出时间戳
	Timestamp int64 `json:"timestamp"`
}

// NewAgentOutput 创建新的 Agent 输出
func NewAgentOutput(agentName, output string) *AgentOutput {
	return &AgentOutput{
		AgentName: agentName,
		Output:    output,
		Timestamp: currentTimeMillis(),
	}
}

// currentTimeMillis 获取当前时间戳（毫秒）
func currentTimeMillis() int64 {
	return time.Now().UnixMilli()
}

// UnixMilli 返回当前时间的毫秒时间戳
func UnixMilli() int64 {
	return time.Now().UnixMilli()
}
