// Package common 提供通用错误消息定义
package common

// HTTP 错误消息
const (
	ErrMethodNotAllowed        = "Method not allowed"
	ErrAgentBridgeNotAvailable = "Agent bridge not available"
)

// DingTalk 错误消息
const (
	ErrGetAccessToken = "get access token: %w"
)

// Skill 错误消息
const (
	ErrSkillNotFound = "skill not found: %s"
)

// Scheduler 错误消息
const (
	ErrMissingTaskID   = "缺少任务ID (task_id)"
	ErrTaskNotFoundFmt = "任务 '%s' 不存在"
)

// Plugin 错误消息
const (
	ErrPluginNameRequired = "请提供插件名"
	ErrPluginNotFoundFmt  = "插件 %s 不存在"
	ErrPluginNotExist     = "插件不存在"
)

// 记忆文件名
const MemoryFileName = "MEMORY.md"
