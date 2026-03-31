// Package prompt 提供模块化的提示词模板系统，用于构建 AI Agent 系统提示词。
// 支持模板变量、条件区块、继承和内置模板。
package prompt

import (
	"errors"
)

// Template 定义提示词模板接口
type Template interface {
	// Execute 使用给定数据渲染模板
	Execute(data any) (string, error)
}

// TemplateOption 是模板配置函数
type TemplateOption func(*templateConfig)

// templateConfig 模板配置
type templateConfig struct {
	delimiters    [2]string        // 左右分隔符
	escapeHTML    bool             // 是否转义 HTML
	missingKey    missingKeyAction // 缺失键处理方式
	funcMap       map[string]any   // 自定义函数映射
	baseTemplates []Template       // 基础模板（用于继承）
}

// missingKeyAction 定义模板中缺失键的处理方式
type missingKeyAction int

const (
	// MissingKeyError 对缺失键返回错误
	MissingKeyError missingKeyAction = iota
	// MissingKeyDefault 对缺失键使用默认值
	MissingKeyDefault
	// MissingKeyZero 对缺失键使用零值
	MissingKeyZero
)

// WithDelimiters 设置模板变量的自定义分隔符
func WithDelimiters(left, right string) TemplateOption {
	return func(c *templateConfig) {
		c.delimiters = [2]string{left, right}
	}
}

// WithEscapeHTML 启用或禁用 HTML 转义
func WithEscapeHTML(escape bool) TemplateOption {
	return func(c *templateConfig) {
		c.escapeHTML = escape
	}
}

// WithFuncMap 向模板添加自定义函数
func WithFuncMap(funcMap map[string]any) TemplateOption {
	return func(c *templateConfig) {
		if c.funcMap == nil {
			c.funcMap = make(map[string]any)
		}
		for k, v := range funcMap {
			c.funcMap[k] = v
		}
	}
}

// WithBaseTemplates 设置用于继承的基础模板
func WithBaseTemplates(templates ...Template) TemplateOption {
	return func(c *templateConfig) {
		c.baseTemplates = append(c.baseTemplates, templates...)
	}
}

// ContextFile 表示加载到提示词中的上下文文件
type ContextFile struct {
	// Path 文件的相对路径
	Path string `json:"path"`
	// Content 文件内容
	Content string `json:"content"`
}

// ToolInfo 包含提示词生成所需的工具信息
type ToolInfo struct {
	// Name 工具的唯一标识符
	Name string `json:"name"`
	// Description 工具功能说明
	Description string `json:"description"`
	// Summary 简短的单行描述
	Summary string `json:"summary,omitempty"`
}

// UserInfo 包含用户身份信息
type UserInfo struct {
	// Name 用户真实姓名
	Name string `json:"name,omitempty"`
	// Nickname 用户希望被称呼的名字
	Nickname string `json:"nickname,omitempty"`
	// Timezone 用户时区
	Timezone string `json:"timezone,omitempty"`
	// Notes 关于用户的附加备注
	Notes string `json:"notes,omitempty"`
}

// RuntimeInfo 包含运行时信息
type RuntimeInfo struct {
	// CurrentTime 当前时间字符串
	CurrentTime string `json:"current_time,omitempty"`
	// Weekday 当前星期
	Weekday string `json:"weekday,omitempty"`
	// Timezone 当前时区
	Timezone string `json:"timezone,omitempty"`
	// Model 正在使用的 LLM 模型
	Model string `json:"model,omitempty"`
	// Workspace 工作空间目录
	Workspace string `json:"workspace,omitempty"`
	// Channel 通信渠道
	Channel string `json:"channel,omitempty"`
}

// BuildOptions 包含构建系统提示词的所有选项
type BuildOptions struct {
	// Language 提示词语言（"zh" 或 "en"）
	Language string `json:"language"`
	// WorkspaceDir 工作空间目录路径
	WorkspaceDir string `json:"workspace_dir"`
	// BasePersona 基础人格描述（已废弃，使用 AGENT.md）
	BasePersona string `json:"base_persona,omitempty"`
	// UserIdentity 用户信息
	UserIdentity *UserInfo `json:"user_identity,omitempty"`
	// Tools 可用工具列表
	Tools []*ToolInfo `json:"tools,omitempty"`
	// ContextFiles 已加载的上下文文件列表
	ContextFiles []*ContextFile `json:"context_files,omitempty"`
	// SkillsPrompt 来自技能管理器的格式化技能提示词
	SkillsPrompt string `json:"skills_prompt,omitempty"`
	// HasMemoryTools 指示是否有记忆工具可用
	HasMemoryTools bool `json:"has_memory_tools"`
	// Runtime 运行时信息
	Runtime *RuntimeInfo `json:"runtime,omitempty"`
}

// 常见错误定义
var (
	// ErrTemplateNotFound 模板未找到
	ErrTemplateNotFound = errors.New("模板未找到")
	// ErrInvalidTemplate 无效的模板语法
	ErrInvalidTemplate = errors.New("无效的模板语法")
	// ErrMissingKey 模板数据中缺失必需键
	ErrMissingKey = errors.New("模板数据中缺失必需键")
)

// Builder 提示词构建器
// 实现模块化的系统提示词构建，支持工具、技能、记忆等多个子系统
type Builder struct {
	workspaceDir string
	language     string
	options      *BuildOptions
}

// NewBuilder 创建新的提示词构建器
func NewBuilder(workspaceDir string, language string) *Builder {
	if language == "" {
		language = "zh"
	}
	return &Builder{
		workspaceDir: workspaceDir,
		language:     language,
		options: &BuildOptions{
			Language:     language,
			WorkspaceDir: workspaceDir,
		},
	}
}

// WithTools 设置工具列表
func (b *Builder) WithTools(tools []*ToolInfo) *Builder {
	b.options.Tools = tools
	return b
}

// WithUserIdentity 设置用户身份信息
func (b *Builder) WithUserIdentity(user *UserInfo) *Builder {
	b.options.UserIdentity = user
	return b
}

// WithContextFiles 设置上下文文件列表
func (b *Builder) WithContextFiles(files []*ContextFile) *Builder {
	b.options.ContextFiles = files
	return b
}

// WithSkillsPrompt 设置技能提示词
func (b *Builder) WithSkillsPrompt(prompt string) *Builder {
	b.options.SkillsPrompt = prompt
	return b
}

// WithMemoryTools 设置是否有记忆工具
func (b *Builder) WithMemoryTools(hasMemory bool) *Builder {
	b.options.HasMemoryTools = hasMemory
	return b
}

// WithRuntime 设置运行时信息
func (b *Builder) WithRuntime(runtime *RuntimeInfo) *Builder {
	b.options.Runtime = runtime
	return b
}

// Build 构建完整的系统提示词
func (b *Builder) Build() string {
	return BuildSystemPrompt(b.options)
}

// GetOptions 获取构建选项
func (b *Builder) GetOptions() *BuildOptions {
	return b.options
}
