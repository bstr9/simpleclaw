package plugin

import (
	"testing"
)

type mockPlugin struct {
	*BasePlugin
	initCalled   bool
	loadCalled   bool
	unloadCalled bool
	initError    error
	loadError    error
	unloadError  error
}

func newMockPlugin(name, version string) *mockPlugin {
	return &mockPlugin{
		BasePlugin: NewBasePlugin(name, version),
	}
}

func (p *mockPlugin) OnInit(ctx *PluginContext) error {
	p.initCalled = true
	return p.initError
}

func (p *mockPlugin) OnLoad(ctx *PluginContext) error {
	p.loadCalled = true
	return p.loadError
}

func (p *mockPlugin) OnUnload(ctx *PluginContext) error {
	p.unloadCalled = true
	return p.unloadError
}

func TestPluginContext(t *testing.T) {
	t.Run("NewPluginContext", func(t *testing.T) {
		bus := NewEventBus()
		ctx := &PluginContext{
			Config:     map[string]any{"key": "value"},
			PluginDir:  "/plugins",
			PluginPath: "/plugins/test",
			DataDir:    "/data",
			LogDir:     "/logs",
			EventBus:   bus,
			PluginName: "test-plugin",
		}

		if ctx.PluginName != "test-plugin" {
			t.Errorf("PluginName = %s, want test-plugin", ctx.PluginName)
		}
		if ctx.PluginDir != "/plugins" {
			t.Errorf("PluginDir = %s, want /plugins", ctx.PluginDir)
		}
		if ctx.EventBus == nil {
			t.Error("EventBus should not be nil")
		}
		if ctx.Config["key"] != "value" {
			t.Errorf("Config[key] = %v, want value", ctx.Config["key"])
		}
	})

	t.Run("ConfigOperations", func(t *testing.T) {
		tests := []struct {
			name   string
			config map[string]any
		}{
			{"nil config", nil},
			{"empty config", map[string]any{}},
			{"with values", map[string]any{"key1": "value1", "key2": 123, "key3": true}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ctx := &PluginContext{
					Config:     tt.config,
					PluginName: "test",
				}

				if tt.config == nil && ctx.Config != nil {
					t.Error("nil config should remain nil")
				}
			})
		}
	})
}

func TestBasePlugin(t *testing.T) {
	t.Run("NewBasePlugin", func(t *testing.T) {
		p := NewBasePlugin("test-plugin", "1.0.0")

		if p.Name() != "test-plugin" {
			t.Errorf("Name() = %s, want test-plugin", p.Name())
		}
		if p.Version() != "1.0.0" {
			t.Errorf("Version() = %s, want 1.0.0", p.Version())
		}
	})

	t.Run("MetadataOperations", func(t *testing.T) {
		p := NewBasePlugin("test", "1.0.0")

		tests := []struct {
			name  string
			setup func()
			check func(*testing.T, *BasePlugin)
		}{
			{
				name:  "SetPriority",
				setup: func() { p.SetPriority(100) },
				check: func(t *testing.T, p *BasePlugin) {
					if meta := p.GetMetadata(); meta.Priority != 100 {
						t.Errorf("Priority = %d, want 100", meta.Priority)
					}
				},
			},
			{
				name:  "SetDescription",
				setup: func() { p.SetDescription("test description") },
				check: func(t *testing.T, p *BasePlugin) {
					if meta := p.GetMetadata(); meta.Description != "test description" {
						t.Errorf("Description = %s, want 'test description'", meta.Description)
					}
				},
			},
			{
				name:  "SetAuthor",
				setup: func() { p.SetAuthor("test author") },
				check: func(t *testing.T, p *BasePlugin) {
					if meta := p.GetMetadata(); meta.Author != "test author" {
						t.Errorf("Author = %s, want 'test author'", meta.Author)
					}
				},
			},
			{
				name:  "SetHidden",
				setup: func() { p.SetHidden(true) },
				check: func(t *testing.T, p *BasePlugin) {
					if meta := p.GetMetadata(); !meta.Hidden {
						t.Error("Hidden should be true")
					}
				},
			},
			{
				name:  "SetEnabled",
				setup: func() { p.SetEnabled(false) },
				check: func(t *testing.T, p *BasePlugin) {
					if meta := p.GetMetadata(); meta.Enabled {
						t.Error("Enabled should be false")
					}
				},
			},
			{
				name:  "SetDependencies",
				setup: func() { p.SetDependencies([]string{"dep1", "dep2"}) },
				check: func(t *testing.T, p *BasePlugin) {
					if meta := p.GetMetadata(); len(meta.Dependencies) != 2 {
						t.Errorf("Dependencies length = %d, want 2", len(meta.Dependencies))
					}
				},
			},
			{
				name:  "SetPath",
				setup: func() { p.SetPath("/test/path") },
				check: func(t *testing.T, p *BasePlugin) {
					if p.Path() != "/test/path" {
						t.Errorf("Path() = %s, want /test/path", p.Path())
					}
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.setup()
				tt.check(t, p)
			})
		}
	})

	t.Run("ConfigOperations", func(t *testing.T) {
		p := NewBasePlugin("test", "1.0.0")

		tests := []struct {
			name  string
			key   string
			value any
		}{
			{"string value", "strKey", "stringValue"},
			{"int value", "intKey", 42},
			{"bool value", "boolKey", true},
			{"float value", "floatKey", 3.14},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				p.SetConfig(tt.key, tt.value)

				val, ok := p.GetConfig(tt.key)
				if !ok {
					t.Errorf("GetConfig(%s) should exist", tt.key)
				}
				if val != tt.value {
					t.Errorf("GetConfig(%s) = %v, want %v", tt.key, val, tt.value)
				}
			})
		}
	})

	t.Run("GetConfigString", func(t *testing.T) {
		p := NewBasePlugin("test", "1.0.0")
		p.SetConfig("strKey", "stringValue")
		p.SetConfig("intKey", 123)

		tests := []struct {
			name     string
			key      string
			expected string
		}{
			{"string key", "strKey", "stringValue"},
			{"non-string key", "intKey", ""},
			{"missing key", "missing", ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := p.GetConfigString(tt.key); got != tt.expected {
					t.Errorf("GetConfigString(%s) = %s, want %s", tt.key, got, tt.expected)
				}
			})
		}
	})

	t.Run("GetConfigInt", func(t *testing.T) {
		p := NewBasePlugin("test", "1.0.0")
		p.SetConfig("intKey", 42)
		p.SetConfig("int64Key", int64(100))
		p.SetConfig("floatKey", float64(200))
		p.SetConfig("strKey", "not a number")

		tests := []struct {
			name     string
			key      string
			expected int
		}{
			{"int key", "intKey", 42},
			{"int64 key", "int64Key", 100},
			{"float key", "floatKey", 200},
			{"string key", "strKey", 0},
			{"missing key", "missing", 0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := p.GetConfigInt(tt.key); got != tt.expected {
					t.Errorf("GetConfigInt(%s) = %d, want %d", tt.key, got, tt.expected)
				}
			})
		}
	})

	t.Run("GetConfigBool", func(t *testing.T) {
		p := NewBasePlugin("test", "1.0.0")
		p.SetConfig("boolKey", true)
		p.SetConfig("strKey", "not a bool")

		tests := []struct {
			name     string
			key      string
			expected bool
		}{
			{"bool key true", "boolKey", true},
			{"string key", "strKey", false},
			{"missing key", "missing", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := p.GetConfigBool(tt.key); got != tt.expected {
					t.Errorf("GetConfigBool(%s) = %v, want %v", tt.key, got, tt.expected)
				}
			})
		}
	})

	t.Run("EventHandler", func(t *testing.T) {
		p := NewBasePlugin("test", "1.0.0")
		handlerCalled := false

		handler := func(ec *EventContext) error {
			handlerCalled = true
			return nil
		}

		p.RegisterHandler(EventOnReceiveMessage, handler)

		ec := NewEventContext(EventOnReceiveMessage, nil)
		err := p.OnEvent(EventOnReceiveMessage, ec)
		if err != nil {
			t.Errorf("OnEvent returned error: %v", err)
		}
		if !handlerCalled {
			t.Error("Handler should have been called")
		}

		p.UnregisterHandler(EventOnReceiveMessage)
		handlerCalled = false
		err = p.OnEvent(EventOnReceiveMessage, ec)
		if err != nil {
			t.Errorf("OnEvent returned error: %v", err)
		}
		if handlerCalled {
			t.Error("Handler should not have been called after unregister")
		}
	})

	t.Run("DefaultLifecycleMethods", func(t *testing.T) {
		p := NewBasePlugin("test", "1.0.0")
		ctx := &PluginContext{PluginName: "test"}

		if err := p.OnInit(ctx); err != nil {
			t.Errorf("OnInit returned error: %v", err)
		}
		if err := p.OnLoad(ctx); err != nil {
			t.Errorf("OnLoad returned error: %v", err)
		}
		if err := p.OnUnload(ctx); err != nil {
			t.Errorf("OnUnload returned error: %v", err)
		}
	})

	t.Run("HelpText", func(t *testing.T) {
		p := NewBasePlugin("test", "1.0.0")
		if help := p.HelpText(); help != "No help information available" {
			t.Errorf("HelpText() = %s, want 'No help information available'", help)
		}
	})
}

func TestMetadata(t *testing.T) {
	t.Run("MetadataFields", func(t *testing.T) {
		meta := &Metadata{
			Name:         "test-plugin",
			NameCN:       "测试插件",
			Version:      "1.0.0",
			Description:  "A test plugin",
			Author:       "test author",
			Priority:     10,
			Hidden:       false,
			Enabled:      true,
			Dependencies: []string{"dep1", "dep2"},
			Path:         "/plugins/test",
		}

		if meta.Name != "test-plugin" {
			t.Errorf("Name = %s, want test-plugin", meta.Name)
		}
		if meta.NameCN != "测试插件" {
			t.Errorf("NameCN = %s, want 测试插件", meta.NameCN)
		}
		if meta.Version != "1.0.0" {
			t.Errorf("Version = %s, want 1.0.0", meta.Version)
		}
		if meta.Priority != 10 {
			t.Errorf("Priority = %d, want 10", meta.Priority)
		}
		if len(meta.Dependencies) != 2 {
			t.Errorf("Dependencies length = %d, want 2", len(meta.Dependencies))
		}
	})
}

func TestPluginInterface(t *testing.T) {
	t.Run("MockPluginImplementsInterface", func(t *testing.T) {
		var _ Plugin = newMockPlugin("test", "1.0.0")
	})

	t.Run("PluginLifecycle", func(t *testing.T) {
		p := newMockPlugin("test", "1.0.0")
		ctx := &PluginContext{PluginName: "test"}

		if p.initCalled {
			t.Error("initCalled should be false initially")
		}

		_ = p.OnInit(ctx)
		if !p.initCalled {
			t.Error("OnInit should have set initCalled to true")
		}

		_ = p.OnLoad(ctx)
		if !p.loadCalled {
			t.Error("OnLoad should have set loadCalled to true")
		}

		_ = p.OnUnload(ctx)
		if !p.unloadCalled {
			t.Error("OnUnload should have set unloadCalled to true")
		}
	})
}
