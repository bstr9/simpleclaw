package skills

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// frontmatter 相关常量
const (
	frontmatterDelimiter = "---\n"
	frontmatterEndMarker = "\n---\n"
	mdFileExtension      = ".md"
	commentPrefix        = "#"
)

// Loader 从目录加载技能文件。
type Loader struct {
	parser *SkillParser
}

// NewLoader 创建新的 Loader 实例。
func NewLoader() *Loader {
	return &Loader{
		parser: NewSkillParser(),
	}
}

// SkillParser 解析技能 markdown 文件。
type SkillParser struct{}

// NewSkillParser 创建新的 SkillParser 实例。
func NewSkillParser() *SkillParser {
	return &SkillParser{}
}

// LoadFromDir 从目录加载技能文件（递归查找子目录中的 SKILL.md）。
func (l *Loader) LoadFromDir(dir string, source Source) *LoadResult {
	result := &LoadResult{
		Skills:      make([]*SkillInfo, 0),
		Diagnostics: make([]string, 0),
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, fmt.Sprintf("无法读取目录 %s: %v", dir, err))
		return result
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dir, name)

		if entry.IsDir() {
			skillFile := filepath.Join(fullPath, "SKILL.md")
			if _, err := os.Stat(skillFile); err == nil {
				skill, err := l.parser.ParseFile(skillFile)
				if err != nil {
					result.Diagnostics = append(result.Diagnostics, fmt.Sprintf("解析文件 %s 失败: %v", skillFile, err))
					continue
				}

				skill.Source = source
				skill.BaseDir = fullPath
				result.Skills = append(result.Skills, skill)
			}
			continue
		}

		if !strings.HasSuffix(name, mdFileExtension) {
			continue
		}

		skill, err := l.parser.ParseFile(fullPath)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, fmt.Sprintf("解析文件 %s 失败: %v", fullPath, err))
			continue
		}

		skill.Source = source
		skill.BaseDir = dir
		result.Skills = append(result.Skills, skill)
	}

	return result
}

// ParseFile 解析单个技能文件。
func (p *SkillParser) ParseFile(filePath string) (*SkillInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	frontmatter, body, err := parseFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	name := extractName(filePath, frontmatter)
	description := extractDescription(frontmatter, body)

	return &SkillInfo{
		name:        name,
		description: description,
		FilePath:    filePath,
		Content:     body,
		Frontmatter: frontmatter,
	}, nil
}

// parseFrontmatter 解析 markdown 文件的 frontmatter。
func parseFrontmatter(content string) (map[string]any, string, error) {
	frontmatter := make(map[string]any)
	body := content

	if !strings.HasPrefix(content, frontmatterDelimiter) {
		return frontmatter, body, nil
	}

	endIdx := strings.Index(content[4:], frontmatterEndMarker)
	if endIdx == -1 {
		return frontmatter, body, nil
	}

	frontmatterStr := content[4 : 4+endIdx]
	body = content[4+endIdx+5:]

	lines := strings.Split(frontmatterStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, commentPrefix) {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, "\"'")
			frontmatter[key] = value
		}
	}

	return frontmatter, body, nil
}

// extractName 从文件名或 frontmatter 提取技能名称。
func extractName(filePath string, frontmatter map[string]any) string {
	if name, ok := frontmatter["name"].(string); ok && name != "" {
		return name
	}
	base := filepath.Base(filePath)
	return strings.TrimSuffix(base, mdFileExtension)
}

// extractDescription 从 frontmatter 或内容提取描述。
func extractDescription(frontmatter map[string]any, body string) string {
	if desc, ok := frontmatter["description"].(string); ok && desc != "" {
		return desc
	}
	if len(body) > 100 {
		return body[:100] + "..."
	}
	return body
}

// FormatForPrompt 将技能列表格式化为提示文本。
func FormatForPrompt(entries []*Entry) string {
	if len(entries) == 0 {
		return "没有可用的技能。"
	}

	var sb strings.Builder
	sb.WriteString("可用技能：\n\n")

	for _, entry := range entries {
		fmt.Fprintf(&sb, "## %s\n", entry.Skill.Name())
		fmt.Fprintf(&sb, "%s\n\n", entry.Skill.Description())
	}

	return sb.String()
}

// ParseMetadata 从 frontmatter 解析元数据。
func ParseMetadata(frontmatter map[string]any) *Metadata {
	if frontmatter == nil {
		return nil
	}

	metadata := &Metadata{}
	metadata.Always = parseMetadataBool(frontmatter, "always", false)
	metadata.SkillKey = parseMetadataString(frontmatter, "skill_key")
	metadata.PrimaryEnv = parseMetadataString(frontmatter, "primary_env")
	metadata.Emoji = parseMetadataString(frontmatter, "emoji")
	metadata.Homepage = parseMetadataString(frontmatter, "homepage")
	metadata.OS = parseMetadataStringSlice(frontmatter, "os")

	if _, ok := frontmatter["requires"]; ok {
		metadata.Requires = parseRequirements(frontmatter["requires"])
	}

	return metadata
}

// parseMetadataBool 从 frontmatter 解析布尔值字段。
func parseMetadataBool(frontmatter map[string]any, key string, defaultValue bool) bool {
	if v, ok := frontmatter[key].(string); ok {
		return parseBool(v, defaultValue)
	}
	if v, ok := frontmatter[key].(bool); ok {
		return v
	}
	return defaultValue
}

// parseMetadataString 从 frontmatter 解析字符串字段。
func parseMetadataString(frontmatter map[string]any, key string) string {
	if v, ok := frontmatter[key].(string); ok {
		return v
	}
	return ""
}

// parseMetadataStringSlice 从 frontmatter 解析字符串数组字段。
func parseMetadataStringSlice(frontmatter map[string]any, key string) []string {
	if v, ok := frontmatter[key].(string); ok {
		return []string{v}
	}
	if v, ok := frontmatter[key].([]any); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// parseRequirements 解析依赖要求。
func parseRequirements(v any) *Requirements {
	req := &Requirements{}

	m, ok := v.(map[string]any)
	if !ok {
		return req
	}

	req.Bins = parseStringSlice(m, "bins")
	req.AnyBins = parseStringSlice(m, "anyBins")
	req.Env = parseStringSlice(m, "env")
	req.AnyEnv = parseStringSlice(m, "anyEnv")

	return req
}

// parseStringSlice 从 map 中解析字符串数组字段。
func parseStringSlice(m map[string]any, key string) []string {
	items, ok := m[key].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// parseBool 解析布尔值。
func parseBool(v any, defaultValue bool) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.ToLower(val) == "true"
	default:
		return defaultValue
	}
}

// Registry 管理技能注册和执行。
type Registry struct {
	mu      sync.RWMutex
	skills  map[string]*Entry
	loader  *Loader
	config  map[string]any
	enabled map[string]bool
}

// NewRegistry 创建新的技能注册表。
func NewRegistry() *Registry {
	return &Registry{
		skills:  make(map[string]*Entry),
		loader:  NewLoader(),
		config:  make(map[string]any),
		enabled: make(map[string]bool),
	}
}

// Register 向注册表添加技能。
func (r *Registry) Register(skill Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := &Entry{
		Skill:         skill,
		SkillInfo:     extractSkillInfo(skill),
		UserInvocable: true,
	}
	r.skills[skill.Name()] = entry
	r.enabled[skill.Name()] = true
}

// RegisterEntry 向注册表添加技能条目。
func (r *Registry) RegisterEntry(entry *Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.skills[entry.Skill.Name()] = entry
	r.enabled[entry.Skill.Name()] = true
}

// Get 根据名称获取技能。
func (r *Registry) Get(name string) (*Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.skills[name]
	return entry, ok
}

// GetSkill 根据名称获取技能接口。
func (r *Registry) GetSkill(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.skills[name]
	if !ok {
		return nil, false
	}
	return entry.Skill, true
}

// Remove 从注册表移除技能。
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.skills, name)
	delete(r.enabled, name)
}

// List 返回所有已注册的技能。
func (r *Registry) List() []*Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]*Entry, 0, len(r.skills))
	for _, entry := range r.skills {
		entries = append(entries, entry)
	}
	return entries
}

// ListEnabled 返回所有已启用的技能。
func (r *Registry) ListEnabled() []*Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]*Entry, 0)
	for name, entry := range r.skills {
		if r.enabled[name] {
			entries = append(entries, entry)
		}
	}
	return entries
}

// Execute 根据名称执行技能，接收输入参数。
func (r *Registry) Execute(ctx context.Context, name string, input any) (any, error) {
	r.mu.RLock()
	entry, ok := r.skills[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf(common.ErrSkillNotFound, name)
	}

	if !r.enabled[name] {
		return nil, fmt.Errorf("skill is disabled: %s", name)
	}

	return entry.Skill.Execute(ctx, input)
}

// Enable 启用技能。
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.skills[name]; !ok {
		return fmt.Errorf(common.ErrSkillNotFound, name)
	}

	r.enabled[name] = true
	return nil
}

// Disable 禁用技能。
func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.skills[name]; !ok {
		return fmt.Errorf(common.ErrSkillNotFound, name)
	}

	r.enabled[name] = false
	return nil
}

// IsEnabled 检查技能是否已启用。
func (r *Registry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.enabled[name]
}

// SetConfig 设置注册表的配置。
func (r *Registry) SetConfig(config map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.config = config
}

// LoadFromDir 从多个目录加载技能文件。
// 目录按顺序加载，后面的目录会覆盖前面同名的技能。
func (r *Registry) LoadFromDir(dirs ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		result := r.loader.LoadFromDir(dir, SourceCustom)
		for _, skill := range result.Skills {
			entry := r.createEntry(skill)
			r.skills[skill.name] = entry
			r.enabled[skill.name] = true
		}
		for _, diag := range result.Diagnostics {
			logger.Debug("Skill loading diagnostic", zap.String("msg", diag))
		}
	}

	logger.Debug("Skills loaded", zap.Int("count", len(r.skills)))
	return nil
}

// Refresh 从配置的目录重新加载所有技能。
func (r *Registry) Refresh(dirs ...string) error {
	r.mu.Lock()

	r.skills = make(map[string]*Entry)
	r.enabled = make(map[string]bool)

	r.mu.Unlock()

	return r.LoadFromDir(dirs...)
}

// Filter 返回匹配过滤条件的技能。
func (r *Registry) Filter(filter *Filter) []*Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []*Entry
	for name, entry := range r.skills {
		if !r.matchesFilter(name, filter, entry) {
			continue
		}
		entries = append(entries, entry)
	}

	return entries
}

// matchesFilter 检查技能是否匹配过滤条件
func (r *Registry) matchesFilter(name string, filter *Filter, entry *Entry) bool {
	// 除非明确请求，否则跳过已禁用的技能
	if !filter.IncludeDisabled && !r.enabled[name] {
		return false
	}

	// 应用名称过滤
	if filter.Names != nil && !containsName(filter.Names, name) {
		return false
	}

	// 检查要求
	return r.shouldInclude(entry)
}

// containsName 检查名称是否在列表中
func containsName(names []string, target string) bool {
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}

// BuildPrompt 生成包含可用技能的格式化提示文本。
func (r *Registry) BuildPrompt(filter *Filter) string {
	entries := r.Filter(filter)
	return FormatForPrompt(entries)
}

// BuildSnapshot 创建当前技能配置的快照。
func (r *Registry) BuildSnapshot(filter *Filter, version int) *Snapshot {
	entries := r.Filter(filter)
	prompt := FormatForPrompt(entries)

	skillsInfo := make([]map[string]string, 0, len(entries))
	resolvedSkills := make([]*SkillInfo, 0, len(entries))

	for _, entry := range entries {
		info := map[string]string{
			"name": entry.Skill.Name(),
		}
		if entry.Metadata != nil && entry.Metadata.PrimaryEnv != "" {
			info["primary_env"] = entry.Metadata.PrimaryEnv
		}
		skillsInfo = append(skillsInfo, info)

		if entry.SkillInfo != nil {
			resolvedSkills = append(resolvedSkills, entry.SkillInfo)
		}
	}

	return &Snapshot{
		Prompt:         prompt,
		Skills:         skillsInfo,
		ResolvedSkills: resolvedSkills,
		Version:        version,
	}
}

// createEntry 从 SkillInfo 创建 Entry。
func (r *Registry) createEntry(info *SkillInfo) *Entry {
	metadata := ParseMetadata(info.Frontmatter)

	userInvocable := true
	if v, ok := info.Frontmatter["user-invocable"]; ok {
		userInvocable = parseBool(v, true)
	}

	return &Entry{
		Skill:         info,
		SkillInfo:     info,
		Metadata:      metadata,
		UserInvocable: userInvocable,
	}
}

// shouldInclude 根据要求判断是否应包含该技能。
func (r *Registry) shouldInclude(entry *Entry) bool {
	if entry.Metadata == nil {
		return true
	}

	if !r.checkOSCompatibility(entry.Metadata.OS) {
		return false
	}

	if entry.Metadata.Always {
		return true
	}

	return r.checkRequirements(entry.Metadata.Requires)
}

// checkOSCompatibility 检查操作系统兼容性。
func (r *Registry) checkOSCompatibility(requiredOS []string) bool {
	if len(requiredOS) == 0 {
		return true
	}

	currentOS := runtime.GOOS
	osMap := map[string]string{
		"darwin":  "darwin",
		"linux":   "linux",
		"windows": "win32",
	}
	normalized := osMap[currentOS]

	for _, os := range requiredOS {
		if os == normalized || os == currentOS {
			return true
		}
	}
	return false
}

// checkRequirements 检查依赖要求是否满足。
func (r *Registry) checkRequirements(req *Requirements) bool {
	if req == nil {
		return true
	}

	for _, bin := range req.Bins {
		if !hasBinary(bin) {
			return false
		}
	}

	if len(req.AnyBins) > 0 && !hasAnyBinary(req.AnyBins) {
		return false
	}

	for _, env := range req.Env {
		if !hasEnvVar(env) {
			return false
		}
	}

	if len(req.AnyEnv) > 0 && !hasAnyEnvVar(req.AnyEnv) {
		return false
	}

	return true
}

// Filter 定义技能过滤条件。
type Filter struct {
	// Names 是要包含的技能名称列表。为空表示全部。
	Names []string

	// IncludeDisabled 在结果中包含已禁用的技能。
	IncludeDisabled bool
}

// extractSkillInfo 从 Skill 接口提取 SkillInfo。
func extractSkillInfo(skill Skill) *SkillInfo {
	if info, ok := skill.(*SkillInfo); ok {
		return info
	}
	return &SkillInfo{
		name:        skill.Name(),
		description: skill.Description(),
	}
}

// hasBinary 检查 PATH 中是否存在指定的二进制文件。
func hasBinary(name string) bool {
	_, err := execLookPath(name)
	return err == nil
}

// hasAnyBinary 检查是否存在给定二进制文件中的任意一个。
func hasAnyBinary(names []string) bool {
	for _, name := range names {
		if hasBinary(name) {
			return true
		}
	}
	return false
}

// hasEnvVar 检查环境变量是否已设置。
func hasEnvVar(name string) bool {
	val, exists := os.LookupEnv(name)
	return exists && strings.TrimSpace(val) != ""
}

// hasAnyEnvVar 检查是否存在给定环境变量中的任意一个。
func hasAnyEnvVar(names []string) bool {
	for _, name := range names {
		if hasEnvVar(name) {
			return true
		}
	}
	return false
}

// execLookPath 查找可执行文件路径。
var execLookPath = exec.LookPath
