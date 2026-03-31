// Package prompt 模板渲染引擎
package prompt

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// DefaultTemplate 默认模板实现
type DefaultTemplate struct {
	name     string
	content  string
	config   *templateConfig
	compiled *template.Template
}

// NewTemplate 创建新模板
func NewTemplate(name, content string, opts ...TemplateOption) (*DefaultTemplate, error) {
	config := &templateConfig{
		delimiters: [2]string{"{{", "}}"},
		escapeHTML: true,
		missingKey: MissingKeyDefault,
		funcMap:    make(map[string]any),
	}

	for _, opt := range opts {
		opt(config)
	}

	t := &DefaultTemplate{
		name:    name,
		content: content,
		config:  config,
	}

	if err := t.compile(); err != nil {
		return nil, err
	}

	return t, nil
}

// compile 编译模板
func (t *DefaultTemplate) compile() error {
	tmpl := template.New(t.name)

	// 设置分隔符
	tmpl.Delims(t.config.delimiters[0], t.config.delimiters[1])

	// 设置缺失键处理
	switch t.config.missingKey {
	case MissingKeyError:
		tmpl.Option("missingkey=error")
	case MissingKeyZero:
		tmpl.Option("missingkey=zero")
	default:
		tmpl.Option("missingkey=default")
	}

	// 添加自定义函数
	if len(t.config.funcMap) > 0 {
		tmpl.Funcs(t.config.funcMap)
	}

	// 解析模板内容
	var err error
	tmpl, err = tmpl.Parse(t.content)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTemplate, err)
	}

	t.compiled = tmpl
	return nil
}

// Execute 渲染模板
func (t *DefaultTemplate) Execute(data any) (string, error) {
	if t.compiled == nil {
		return "", ErrTemplateNotFound
	}

	var buf bytes.Buffer
	if err := t.compiled.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// SimpleTemplate 简单变量替换模板
// 使用 {{variable}} 格式进行变量替换，不支持复杂逻辑
type SimpleTemplate struct {
	content string
}

// NewSimpleTemplate 创建简单模板
func NewSimpleTemplate(content string) *SimpleTemplate {
	return &SimpleTemplate{content: content}
}

// Execute 渲染模板，使用 map 进行变量替换
func (t *SimpleTemplate) Execute(data any) (string, error) {
	result := t.content

	// 尝试将 data 转换为 map
	var vars map[string]any
	switch v := data.(type) {
	case map[string]any:
		vars = v
	case map[string]string:
		vars = make(map[string]any)
		for key, val := range v {
			vars[key] = val
		}
	default:
		return result, nil
	}

	// 替换变量
	for key, val := range vars {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", val))
	}

	return result, nil
}

// TemplateManager 模板管理器
type TemplateManager struct {
	templates map[string]Template
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager() *TemplateManager {
	return &TemplateManager{
		templates: make(map[string]Template),
	}
}

// Register 注册模板
func (m *TemplateManager) Register(name string, tmpl Template) {
	m.templates[name] = tmpl
}

// RegisterDefault 注册默认模板
func (m *TemplateManager) RegisterDefault(name, content string, opts ...TemplateOption) error {
	tmpl, err := NewTemplate(name, content, opts...)
	if err != nil {
		return err
	}
	m.templates[name] = tmpl
	return nil
}

// RegisterSimple 注册简单模板
func (m *TemplateManager) RegisterSimple(name, content string) {
	m.templates[name] = NewSimpleTemplate(content)
}

// Get 获取模板
func (m *TemplateManager) Get(name string) (Template, error) {
	tmpl, ok := m.templates[name]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	return tmpl, nil
}

// Execute 执行模板渲染
func (m *TemplateManager) Execute(name string, data any) (string, error) {
	tmpl, err := m.Get(name)
	if err != nil {
		return "", err
	}
	return tmpl.Execute(data)
}

// Has 检查模板是否存在
func (m *TemplateManager) Has(name string) bool {
	_, ok := m.templates[name]
	return ok
}

// Names 获取所有模板名称
func (m *TemplateManager) Names() []string {
	names := make([]string, 0, len(m.templates))
	for name := range m.templates {
		names = append(names, name)
	}
	return names
}

// 内置模板变量正则
var varPattern = regexp.MustCompile(`\{\{(\w+)\}\}`)

// ExtractVariables 从模板内容中提取变量名
func ExtractVariables(content string) []string {
	matches := varPattern.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var result []string

	for _, match := range matches {
		if len(match) > 1 {
			name := match[1]
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}

	return result
}

// 内置提示词模板

const (
	// TemplateSystemPrompt 系统提示词模板名
	TemplateSystemPrompt = "system_prompt"
	// TemplateUserPrompt 用户提示词模板名
	TemplateUserPrompt = "user_prompt"
	// TemplateToolPrompt 工具提示词模板名
	TemplateToolPrompt = "tool_prompt"
	// TemplateSkillPrompt 技能提示词模板名
	TemplateSkillPrompt = "skill_prompt"
)

// 内置模板内容
var builtInTemplates = map[string]string{
	TemplateSystemPrompt: `你是一个智能助手，你的任务是{{.Task}}。

{{.Instructions}}

请根据以上信息回答用户的问题。`,

	TemplateUserPrompt: `用户问题：{{.Question}}

{{.Context}}`,

	TemplateToolPrompt: `## 可用工具

以下工具可供使用：

{{.ToolList}}

使用工具时，请遵循以下规则：
1. 确认工具名称正确
2. 检查参数是否完整
3. 处理返回结果`,

	TemplateSkillPrompt: `## 技能说明

技能名称：{{.Name}}
技能描述：{{.Description}}
技能路径：{{.Path}}

使用方法：
使用 read 工具读取 {{.Path}}/SKILL.md 文件获取详细指令。`,
}

// InitBuiltInTemplates 初始化内置模板
func InitBuiltInTemplates(mgr *TemplateManager) error {
	for name, content := range builtInTemplates {
		if err := mgr.RegisterDefault(name, content); err != nil {
			return fmt.Errorf("注册模板 %s 失败: %w", name, err)
		}
	}
	return nil
}
