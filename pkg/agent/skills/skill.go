// Package skills 提供 AI Agent 的技能系统。
// 技能是从带有 frontmatter 元数据的 markdown 文件加载的专用模块，
// 为特定任务提供指令。
package skills

import (
	"context"
)

// Source 标识技能的来源。
type Source string

const (
	// SourceBuiltin 表示项目内置的技能。
	SourceBuiltin Source = "builtin"
	// SourceCustom 表示用户安装的自定义技能。
	SourceCustom Source = "custom"
)

// Skill 表示从 markdown 文件加载的技能。
// 技能为特定任务提供专用指令。
type Skill interface {
	// Name 返回技能的唯一名称。
	Name() string

	// Description 返回技能功能的简要描述。
	Description() string

	// Execute 执行技能，接收输入并返回结果。
	Execute(ctx context.Context, input any) (any, error)
}

// SkillInfo 包含技能的核心信息。
type SkillInfo struct {
	// name 是技能的唯一标识符。
	name string

	// description 描述技能的功能。
	description string

	// FilePath 是技能 markdown 文件的路径。
	FilePath string `json:"file_path"`

	// BaseDir 是技能所在的目录。
	BaseDir string `json:"base_dir"`

	// Source 标识技能是内置的还是自定义的。
	Source Source `json:"source"`

	// Content 是技能的完整 markdown 内容。
	Content string `json:"content"`

	// DisableModelInvocation 防止技能被包含在提示词中。
	DisableModelInvocation bool `json:"disable_model_invocation,omitempty"`

	// Frontmatter 包含从技能文件解析的元数据。
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
}

// Name 返回技能名称（实现 Skill 接口）。
func (s *SkillInfo) Name() string {
	return s.name
}

// Description 返回技能描述（实现 Skill 接口）。
func (s *SkillInfo) Description() string {
	return s.description
}

// SetName 设置技能名称。
func (s *SkillInfo) SetName(name string) {
	s.name = name
}

// SetDescription 设置技能描述。
func (s *SkillInfo) SetDescription(desc string) {
	s.description = desc
}

// Execute 是默认实现，返回未修改的输入。
// 具体技能应实现自己的 Execute 方法。
func (s *SkillInfo) Execute(ctx context.Context, input any) (any, error) {
	return input, nil
}

// Metadata 包含技能的额外配置。
type Metadata struct {
	// Always 无视要求，始终包含此技能。
	Always bool `json:"always,omitempty"`

	// SkillKey 是技能键的可选覆盖值。
	SkillKey string `json:"skill_key,omitempty"`

	// PrimaryEnv 是此技能使用的主要环境变量。
	PrimaryEnv string `json:"primary_env,omitempty"`

	// Emoji 是 UI 显示的可选表情符号。
	Emoji string `json:"emoji,omitempty"`

	// Homepage 是技能文档的可选 URL。
	Homepage string `json:"homepage,omitempty"`

	// OS 列出支持的操作系统（例如 "darwin", "linux", "win32"）。
	OS []string `json:"os,omitempty"`

	// Requires 指定运行时要求。
	Requires *Requirements `json:"requires,omitempty"`

	// Install 指定安装规范。
	Install []*InstallSpec `json:"install,omitempty"`
}

// Requirements 定义技能运行所需的要求。
type Requirements struct {
	// Bins 是必需的二进制文件（必须全部存在）。
	Bins []string `json:"bins,omitempty"`

	// AnyBins 要求至少存在列出的一个二进制文件。
	AnyBins []string `json:"anyBins,omitempty"`

	// Env 是必需的环境变量（必须全部设置）。
	Env []string `json:"env,omitempty"`

	// AnyEnv 要求至少存在列出的一个环境变量。
	AnyEnv []string `json:"anyEnv,omitempty"`
}

// InstallSpec 指定如何安装技能的依赖。
type InstallSpec struct {
	// Kind 是安装类型（brew、pip、npm、download 等）。
	Kind string `json:"kind"`

	// ID 是可选的标识符。
	ID string `json:"id,omitempty"`

	// Label 是可选的显示标签。
	Label string `json:"label,omitempty"`

	// Bins 列出此安装提供的二进制文件。
	Bins []string `json:"bins,omitempty"`

	// OS 列出支持的操作系统。
	OS []string `json:"os,omitempty"`

	// Formula 是 homebrew 公式名称。
	Formula string `json:"formula,omitempty"`

	// Package 是 pip/npm 包名。
	Package string `json:"package,omitempty"`

	// Module 是 Python 模块名。
	Module string `json:"module,omitempty"`

	// URL 是下载类型的下载地址。
	URL string `json:"url,omitempty"`

	// Archive 是下载类型的归档文件名。
	Archive string `json:"archive,omitempty"`

	// Extract 指示是否解压归档文件。
	Extract bool `json:"extract,omitempty"`

	// StripComponents 是解压时要去除的路径组件数量。
	StripComponents int `json:"strip_components,omitempty"`

	// TargetDir 是安装的目标目录。
	TargetDir string `json:"target_dir,omitempty"`
}

// Entry 将技能与其解析的元数据组合在一起。
type Entry struct {
	// Skill 是底层的技能。
	Skill Skill `json:"-"`

	// SkillInfo 包含技能的信息。
	*SkillInfo `json:"skill_info,omitempty"`

	// Metadata 包含解析的 frontmatter 元数据。
	Metadata *Metadata `json:"metadata,omitempty"`

	// UserInvocable 指示用户是否可以直接调用此技能。
	UserInvocable bool `json:"user_invocable"`
}

// Snapshot 捕获特定运行的技能配置快照。
type Snapshot struct {
	// Prompt 是包含技能描述的格式化提示文本。
	Prompt string `json:"prompt"`

	// Skills 是技能摘要列表（名称和 primary_env）。
	Skills []map[string]string `json:"skills"`

	// ResolvedSkills 是完全解析的技能信息列表。
	ResolvedSkills []*SkillInfo `json:"resolved_skills,omitempty"`

	// Version 是快照的可选版本号。
	Version int `json:"version,omitempty"`
}

// LoadResult 包含从目录加载技能的结果。
type LoadResult struct {
	// Skills 是成功加载的技能列表。
	Skills []*SkillInfo `json:"skills"`

	// Diagnostics 包含加载过程中遇到的警告或错误。
	Diagnostics []string `json:"diagnostics,omitempty"`
}
