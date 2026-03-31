package extension

import (
	"context"
	"testing"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/channel"
)

type mockExtension struct {
	id          string
	name        string
	description string
	version     string
	registered  bool
	started     bool
}

func (e *mockExtension) ID() string          { return e.id }
func (e *mockExtension) Name() string        { return e.name }
func (e *mockExtension) Description() string { return e.description }
func (e *mockExtension) Version() string     { return e.version }
func (e *mockExtension) Register(api ExtensionAPI) error {
	e.registered = true
	api.RegisterChannel(e.id, func() (channel.Channel, error) { return nil, nil })
	return nil
}
func (e *mockExtension) Startup(ctx context.Context) error {
	e.started = true
	return nil
}
func (e *mockExtension) Shutdown(ctx context.Context) error {
	e.started = false
	return nil
}

type mockTool struct {
	name string
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return "mock tool" }
func (t *mockTool) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}
func (t *mockTool) Stage() agent.ToolStage { return agent.ToolStagePreProcess }
func (t *mockTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	return agent.NewToolResult("ok"), nil
}

func TestManager_Register(t *testing.T) {
	mgr := NewManager()

	ext := &mockExtension{
		id:          "test",
		name:        "Test Extension",
		description: "A test extension",
		version:     "1.0.0",
	}

	err := mgr.Register(ext)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	_, ok := mgr.Get("test")
	if !ok {
		t.Error("Extension not registered")
	}
}

func TestManager_Register_Nil(t *testing.T) {
	mgr := NewManager()

	err := mgr.Register(nil)
	if err == nil {
		t.Error("expected error for nil extension")
	}
}

func TestManager_Unregister(t *testing.T) {
	mgr := NewManager()

	ext := &mockExtension{id: "test"}
	_ = mgr.Register(ext)

	err := mgr.Unregister("test")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	_, ok := mgr.Get("test")
	if ok {
		t.Error("Extension still registered after unregister")
	}
}

func TestManager_Unregister_NotFound(t *testing.T) {
	mgr := NewManager()

	err := mgr.Unregister("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent extension")
	}
}

func TestManager_List(t *testing.T) {
	mgr := NewManager()

	_ = mgr.Register(&mockExtension{id: "ext1"})
	_ = mgr.Register(&mockExtension{id: "ext2"})

	list := mgr.List()
	if len(list) != 2 {
		t.Errorf("Expected 2 extensions, got %d", len(list))
	}
}

func TestManager_ListInfo(t *testing.T) {
	mgr := NewManager()

	_ = mgr.Register(&mockExtension{id: "ext1", name: "Extension 1"})
	_ = mgr.Register(&mockExtension{id: "ext2", name: "Extension 2"})

	infos := mgr.ListInfo()
	if len(infos) != 2 {
		t.Errorf("Expected 2 infos, got %d", len(infos))
	}
}

func TestManager_StartupAll(t *testing.T) {
	mgr := NewManager()

	ext := &mockExtension{id: "test"}
	_ = mgr.Register(ext)

	ctx := context.Background()
	err := mgr.StartupAll(ctx)
	if err != nil {
		t.Fatalf("StartupAll failed: %v", err)
	}

	if !ext.started {
		t.Error("Extension not started")
	}
}

func TestManager_ShutdownAll(t *testing.T) {
	mgr := NewManager()

	ext := &mockExtension{id: "test"}
	_ = mgr.Register(ext)

	ctx := context.Background()
	_ = mgr.StartupAll(ctx)

	err := mgr.ShutdownAll(ctx)
	if err != nil {
		t.Fatalf("ShutdownAll failed: %v", err)
	}

	if ext.started {
		t.Error("Extension still started after shutdown")
	}
}

func TestManager_Options(t *testing.T) {
	api := NewAPI()
	mgr := NewManager(
		WithExtensionDir("/ext/dir"),
		WithWorkingDir("/work/dir"),
		WithAPI(api),
	)

	if mgr.ExtensionDir() != "/ext/dir" {
		t.Errorf("expected /ext/dir, got %s", mgr.ExtensionDir())
	}
}

func TestManager_RegisterAll(t *testing.T) {
	api := NewAPI()
	mgr := NewManager(WithAPI(api))

	ext := &mockExtension{id: "test"}
	_ = mgr.Register(ext)

	err := mgr.RegisterAll()
	if err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}
}

func TestManager_RegisterAll_NoAPI(t *testing.T) {
	mgr := NewManager()

	ext := &mockExtension{id: "test"}
	_ = mgr.Register(ext)

	err := mgr.RegisterAll()
	if err == nil {
		t.Error("expected error when API is nil")
	}
}

func TestAPI_RegisterChannel(t *testing.T) {
	api := NewAPI()

	called := false
	api.RegisterChannel("test", func() (channel.Channel, error) {
		called = true
		return nil, nil
	})

	if !channel.IsChannelRegistered("test") {
		t.Error("Channel not registered")
	}
	_ = called
}

func TestAPI_RegisterTool(t *testing.T) {
	api := NewAPI()

	tool := &mockTool{name: "test_tool"}
	api.RegisterTool(tool)
}

func TestAPI_Options(t *testing.T) {
	api := NewAPI(
		WithAPIWorkingDir("/work"),
		WithAPIExtensionDir("/ext"),
	)

	if api.WorkingDir() != "/work" {
		t.Errorf("expected /work, got %s", api.WorkingDir())
	}
	if api.ExtensionDir() != "/ext" {
		t.Errorf("expected /ext, got %s", api.ExtensionDir())
	}
}

func TestAPI_Config(t *testing.T) {
	api := NewAPI()
	api.config = map[string]any{"key1": "value1", "key2": 123}

	if api.Config("key1") != "value1" {
		t.Error("expected value1")
	}
	if api.Config("key2") != 123 {
		t.Error("expected 123")
	}
	if api.Config("nonexistent") != nil {
		t.Error("expected nil")
	}
}

func TestAPI_ConfigString(t *testing.T) {
	api := NewAPI()
	api.config = map[string]any{"key1": "value1", "key2": 123}

	if api.ConfigString("key1") != "value1" {
		t.Error("expected value1")
	}
	if api.ConfigString("key2") != "" {
		t.Error("expected empty string for non-string value")
	}
	if api.ConfigString("nonexistent") != "" {
		t.Error("expected empty string for nonexistent key")
	}
}

func TestAPI_SkillPaths(t *testing.T) {
	api := NewAPI()
	api.RegisterSkillPath("/path1")
	api.RegisterSkillPath("/path2")

	paths := api.GetSkillPaths()
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
}

func TestAPI_ChannelCreators(t *testing.T) {
	api := NewAPI()
	api.RegisterChannel("channel1", func() (channel.Channel, error) { return nil, nil })

	creators := api.GetChannelCreators()
	if len(creators) < 1 {
		t.Error("expected at least 1 creator")
	}
}

func TestAPI_EventHandlers(t *testing.T) {
	api := NewAPI()

	called := false
	api.RegisterEventHandler("test_event", func(ctx context.Context, data any) error {
		called = true
		return nil
	})

	err := api.EmitEvent(context.Background(), "test_event", nil)
	if err != nil {
		t.Fatalf("EmitEvent failed: %v", err)
	}

	if !called {
		t.Error("event handler not called")
	}
}

func TestAPI_ResolvePath(t *testing.T) {
	api := NewAPI(WithAPIExtensionDir("/ext"))

	result := api.ResolvePath("/absolute/path")
	if result != "/absolute/path" {
		t.Errorf("expected /absolute/path, got %s", result)
	}

	result = api.ResolvePath("relative/path")
	if result != "/ext/relative/path" {
		t.Errorf("expected /ext/relative/path, got %s", result)
	}
}

func TestGlobalRegistry(t *testing.T) {
	ext := &mockExtension{id: "global_test"}

	RegisterExtension(ext)

	extensions := GetGlobalExtensions()
	found := false
	for _, e := range extensions {
		if e.ID() == "global_test" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Extension not found in global registry")
	}
}
