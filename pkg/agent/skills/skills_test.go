package skills

import (
	"context"
	"testing"
)

func newSkillInfo(name, description string) *SkillInfo {
	return &SkillInfo{
		name:        name,
		description: description,
	}
}

func TestSource(t *testing.T) {
	tests := []struct {
		source   Source
		expected string
	}{
		{SourceBuiltin, "builtin"},
		{SourceCustom, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.source) != tt.expected {
				t.Errorf("期望 %s，实际为 %s", tt.expected, tt.source)
			}
		})
	}
}

func TestSkillInfo(t *testing.T) {
	tests := []struct {
		name        string
		description string
		filePath    string
		source      Source
	}{
		{"test_skill", "测试技能描述", "/path/to/skill.md", SourceBuiltin},
		{"custom_skill", "自定义技能", "/custom/skills/skill.md", SourceCustom},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := newSkillInfo(tt.name, tt.description)
			info.FilePath = tt.filePath
			info.Source = tt.source

			if info.Name() != tt.name {
				t.Errorf("Name() = %s, want %s", info.Name(), tt.name)
			}
			if info.Description() != tt.description {
				t.Errorf("Description() = %s, want %s", info.Description(), tt.description)
			}
			if info.FilePath != tt.filePath {
				t.Errorf("FilePath = %s, want %s", info.FilePath, tt.filePath)
			}
			if info.Source != tt.source {
				t.Errorf("Source = %s, want %s", info.Source, tt.source)
			}
		})
	}
}

func TestSkillInfoSetters(t *testing.T) {
	info := newSkillInfo("original", "原始描述")

	info.SetName("new_name")
	if info.Name() != "new_name" {
		t.Errorf("SetName 后 Name() = %s, want new_name", info.Name())
	}

	info.SetDescription("新描述")
	if info.Description() != "新描述" {
		t.Errorf("SetDescription 后 Description() = %s, want 新描述", info.Description())
	}
}

func TestSkillInfoExecute(t *testing.T) {
	ctx := context.Background()
	info := newSkillInfo("test", "测试")

	result, err := info.Execute(ctx, "输入数据")
	if err != nil {
		t.Errorf("Execute 失败: %v", err)
	}
	if result != "输入数据" {
		t.Errorf("Execute 返回值应该等于输入，实际为 %v", result)
	}
}

func TestMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata *Metadata
		check    func(t *testing.T, m *Metadata)
	}{
		{
			name: "Always 标志",
			metadata: &Metadata{
				Always:     true,
				SkillKey:   "test_key",
				PrimaryEnv: "TEST_API_KEY",
			},
			check: func(t *testing.T, m *Metadata) {
				if !m.Always {
					t.Error("期望 Always 为 true")
				}
				if m.SkillKey != "test_key" {
					t.Error("期望 SkillKey 为 test_key")
				}
			},
		},
		{
			name: "OS 列表",
			metadata: &Metadata{
				OS: []string{"darwin", "linux"},
			},
			check: func(t *testing.T, m *Metadata) {
				if len(m.OS) != 2 {
					t.Errorf("期望 2 个 OS，实际为 %d", len(m.OS))
				}
			},
		},
		{
			name: "Requirements",
			metadata: &Metadata{
				Requires: &Requirements{
					Bins:    []string{"git", "docker"},
					AnyBins: []string{"python", "python3"},
					Env:     []string{"API_KEY"},
				},
			},
			check: func(t *testing.T, m *Metadata) {
				if len(m.Requires.Bins) != 2 {
					t.Errorf("期望 2 个 Bins")
				}
				if len(m.Requires.AnyBins) != 2 {
					t.Errorf("期望 2 个 AnyBins")
				}
			},
		},
		{
			name: "Install Specs",
			metadata: &Metadata{
				Install: []*InstallSpec{
					{Kind: "brew", Formula: "git"},
					{Kind: "pip", Package: "requests"},
				},
			},
			check: func(t *testing.T, m *Metadata) {
				if len(m.Install) != 2 {
					t.Errorf("期望 2 个 Install specs")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, tt.metadata)
		})
	}
}

func TestEntry(t *testing.T) {
	skillInfo := newSkillInfo("test_skill", "测试技能")

	entry := &Entry{
		Skill:         skillInfo,
		SkillInfo:     skillInfo,
		UserInvocable: true,
	}

	if entry.Skill.Name() != "test_skill" {
		t.Error("期望 Skill.Name() 为 test_skill")
	}
	if !entry.UserInvocable {
		t.Error("期望 UserInvocable 为 true")
	}
}

func TestSnapshot(t *testing.T) {
	snapshot := &Snapshot{
		Prompt: "可用技能列表...",
		Skills: []map[string]string{
			{"name": "skill1", "primary_env": "KEY1"},
			{"name": "skill2", "primary_env": "KEY2"},
		},
		Version: 1,
	}

	if snapshot.Prompt == "" {
		t.Error("期望 Prompt 不为空")
	}
	if len(snapshot.Skills) != 2 {
		t.Errorf("期望 2 个技能，实际为 %d", len(snapshot.Skills))
	}
	if snapshot.Version != 1 {
		t.Errorf("期望 Version 为 1")
	}
}

func TestLoadResult(t *testing.T) {
	result := &LoadResult{
		Skills: []*SkillInfo{
			newSkillInfo("skill1", "技能1"),
			newSkillInfo("skill2", "技能2"),
		},
		Diagnostics: []string{"警告: 某些配置缺失"},
	}

	if len(result.Skills) != 2 {
		t.Errorf("期望 2 个技能，实际为 %d", len(result.Skills))
	}
	if len(result.Diagnostics) != 1 {
		t.Errorf("期望 1 条诊断信息")
	}
}

func TestSkillRegistry(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Registry)
		validate func(t *testing.T, r *Registry)
	}{
		{
			name: "注册和获取技能",
			setup: func(r *Registry) {
				r.Register(newSkillInfo("test_skill", "测试技能"))
			},
			validate: func(t *testing.T, r *Registry) {
				entry, ok := r.Get("test_skill")
				if !ok {
					t.Error("期望找到技能 test_skill")
					return
				}
				if entry.Skill.Name() != "test_skill" {
					t.Errorf("期望技能名称为 test_skill")
				}
			},
		},
		{
			name:  "获取不存在的技能",
			setup: func(r *Registry) {},
			validate: func(t *testing.T, r *Registry) {
				_, ok := r.Get("nonexistent")
				if ok {
					t.Error("期望找不到技能 nonexistent")
				}
			},
		},
		{
			name: "注册多个技能",
			setup: func(r *Registry) {
				r.Register(newSkillInfo("skill1", "技能1"))
				r.Register(newSkillInfo("skill2", "技能2"))
				r.Register(newSkillInfo("skill3", "技能3"))
			},
			validate: func(t *testing.T, r *Registry) {
				entries := r.List()
				if len(entries) != 3 {
					t.Errorf("期望 3 个技能，实际为 %d", len(entries))
				}
			},
		},
		{
			name: "移除技能",
			setup: func(r *Registry) {
				r.Register(newSkillInfo("to_remove", "待移除"))
				r.Remove("to_remove")
			},
			validate: func(t *testing.T, r *Registry) {
				_, ok := r.Get("to_remove")
				if ok {
					t.Error("期望技能已被移除")
				}
			},
		},
		{
			name: "启用禁用技能",
			setup: func(r *Registry) {
				r.Register(newSkillInfo("toggle_skill", "开关技能"))
				r.Disable("toggle_skill")
			},
			validate: func(t *testing.T, r *Registry) {
				if r.IsEnabled("toggle_skill") {
					t.Error("期望技能被禁用")
				}
				enabled := r.ListEnabled()
				if len(enabled) != 0 {
					t.Errorf("期望 0 个启用的技能，实际为 %d", len(enabled))
				}
			},
		},
		{
			name: "重新启用技能",
			setup: func(r *Registry) {
				r.Register(newSkillInfo("re_enable", "重新启用"))
				r.Disable("re_enable")
				r.Enable("re_enable")
			},
			validate: func(t *testing.T, r *Registry) {
				if !r.IsEnabled("re_enable") {
					t.Error("期望技能被启用")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			tt.setup(r)
			tt.validate(t, r)
		})
	}
}

func TestRegistryExecute(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry()

	r.Register(newSkillInfo("echo", "回显技能"))

	result, err := r.Execute(ctx, "echo", "测试输入")
	if err != nil {
		t.Errorf("Execute 失败: %v", err)
	}
	if result != "测试输入" {
		t.Errorf("期望结果为 '测试输入'，实际为 %v", result)
	}
}

func TestRegistryExecuteErrors(t *testing.T) {
	ctx := context.Background()
	r := NewRegistry()

	t.Run("技能不存在", func(t *testing.T) {
		_, err := r.Execute(ctx, "nonexistent", nil)
		if err == nil {
			t.Error("期望返回错误")
		}
	})

	r.Register(newSkillInfo("disabled_skill", "禁用技能"))
	r.Disable("disabled_skill")

	t.Run("技能已禁用", func(t *testing.T) {
		_, err := r.Execute(ctx, "disabled_skill", nil)
		if err == nil {
			t.Error("期望返回错误")
		}
	})
}

func TestRegistryEnableDisableErrors(t *testing.T) {
	r := NewRegistry()

	t.Run("启用不存在的技能", func(t *testing.T) {
		err := r.Enable("nonexistent")
		if err == nil {
			t.Error("期望返回错误")
		}
	})

	t.Run("禁用不存在的技能", func(t *testing.T) {
		err := r.Disable("nonexistent")
		if err == nil {
			t.Error("期望返回错误")
		}
	})
}

func TestRegistrySetConfig(t *testing.T) {
	r := NewRegistry()

	config := map[string]any{
		"key1": "value1",
		"key2": 42,
	}

	r.SetConfig(config)
}

func TestRegistryGetSkill(t *testing.T) {
	r := NewRegistry()
	skillInfo := newSkillInfo("test", "测试技能")
	r.Register(skillInfo)

	skill, ok := r.GetSkill("test")
	if !ok {
		t.Error("期望找到技能")
		return
	}
	if skill.Name() != "test" {
		t.Error("期望技能名称为 test")
	}

	_, ok = r.GetSkill("nonexistent")
	if ok {
		t.Error("期望找不到技能")
	}
}

func TestRegisterEntry(t *testing.T) {
	r := NewRegistry()

	skillInfo := newSkillInfo("entry_skill", "通过Entry注册")
	entry := &Entry{
		Skill:         skillInfo,
		SkillInfo:     skillInfo,
		UserInvocable: true,
	}

	r.RegisterEntry(entry)

	_, ok := r.Get("entry_skill")
	if !ok {
		t.Error("期望通过 RegisterEntry 注册的技能存在")
	}
}

func TestFilter(t *testing.T) {
	r := NewRegistry()

	r.Register(newSkillInfo("skill1", "技能1"))
	r.Register(newSkillInfo("skill2", "技能2"))
	r.Register(newSkillInfo("skill3", "技能3"))
	r.Disable("skill3")

	tests := []struct {
		name   string
		filter *Filter
		want   int
	}{
		{
			name:   "无过滤",
			filter: &Filter{},
			want:   2,
		},
		{
			name:   "包含禁用",
			filter: &Filter{IncludeDisabled: true},
			want:   3,
		},
		{
			name:   "名称过滤",
			filter: &Filter{Names: []string{"skill1", "skill2"}},
			want:   2,
		},
		{
			name:   "名称过滤不存在的",
			filter: &Filter{Names: []string{"nonexistent"}},
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := r.Filter(tt.filter)
			if len(entries) != tt.want {
				t.Errorf("期望 %d 个结果，实际为 %d", tt.want, len(entries))
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	r := NewRegistry()

	prompt := r.BuildPrompt(&Filter{})
	if prompt == "" {
		t.Error("期望非空提示（即使是空的技能列表）")
	}

	r.Register(newSkillInfo("skill1", "技能1描述"))
	r.Register(newSkillInfo("skill2", "技能2描述"))

	prompt = r.BuildPrompt(&Filter{})
	if prompt == "" {
		t.Error("期望非空提示")
	}
}

func TestBuildSnapshot(t *testing.T) {
	r := NewRegistry()
	r.Register(newSkillInfo("skill1", "技能1"))

	snapshot := r.BuildSnapshot(&Filter{}, 1)

	if snapshot.Prompt == "" {
		t.Error("期望 Prompt 不为空")
	}
	if len(snapshot.Skills) != 1 {
		t.Errorf("期望 1 个技能，实际为 %d", len(snapshot.Skills))
	}
	if snapshot.Version != 1 {
		t.Errorf("期望 Version 为 1")
	}
}

func TestFormatForPrompt(t *testing.T) {
	tests := []struct {
		name      string
		entries   []*Entry
		wantEmpty bool
	}{
		{
			name:      "空列表",
			entries:   nil,
			wantEmpty: false,
		},
		{
			name: "有技能",
			entries: []*Entry{
				{Skill: newSkillInfo("skill1", "描述1")},
				{Skill: newSkillInfo("skill2", "描述2")},
			},
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := FormatForPrompt(tt.entries)
			if (prompt == "") != tt.wantEmpty {
				t.Errorf("期望空=%v，实际为 '%s'", tt.wantEmpty, prompt)
			}
		})
	}
}

func TestParseMetadata(t *testing.T) {
	tests := []struct {
		name        string
		frontmatter map[string]any
		check       func(t *testing.T, m *Metadata)
	}{
		{
			name:        "nil frontmatter",
			frontmatter: nil,
			check: func(t *testing.T, m *Metadata) {
				if m != nil {
					t.Error("期望返回 nil")
				}
			},
		},
		{
			name: "基本字段",
			frontmatter: map[string]any{
				"always":      "true",
				"skill_key":   "test_key",
				"primary_env": "API_KEY",
				"emoji":       "🔧",
				"homepage":    "https://example.com",
			},
			check: func(t *testing.T, m *Metadata) {
				if !m.Always {
					t.Error("期望 Always 为 true")
				}
				if m.SkillKey != "test_key" {
					t.Error("期望 SkillKey")
				}
				if m.PrimaryEnv != "API_KEY" {
					t.Error("期望 PrimaryEnv")
				}
			},
		},
		{
			name: "布尔类型 always",
			frontmatter: map[string]any{
				"always": true,
			},
			check: func(t *testing.T, m *Metadata) {
				if !m.Always {
					t.Error("期望 Always 为 true")
				}
			},
		},
		{
			name: "OS 字符串",
			frontmatter: map[string]any{
				"os": "darwin",
			},
			check: func(t *testing.T, m *Metadata) {
				if len(m.OS) != 1 || m.OS[0] != "darwin" {
					t.Error("期望 OS 为 ['darwin']")
				}
			},
		},
		{
			name: "OS 数组",
			frontmatter: map[string]any{
				"os": []any{"darwin", "linux"},
			},
			check: func(t *testing.T, m *Metadata) {
				if len(m.OS) != 2 {
					t.Errorf("期望 2 个 OS")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ParseMetadata(tt.frontmatter)
			tt.check(t, m)
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    any
		expected bool
	}{
		{true, true},
		{false, false},
		{"true", true},
		{"false", false},
		{"TRUE", true},
		{"FALSE", false},
		{123, false},
		{nil, false},
	}

	for _, tt := range tests {
		result := parseBool(tt.input, false)
		if result != tt.expected {
			t.Errorf("parseBool(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
