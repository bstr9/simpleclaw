package bridge

import (
	"testing"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/config"
)

func TestAgentBridge_NewAgentBridge(t *testing.T) {
	cfg := &config.Config{
		Model:         "openai",
		ModelName:     "glm-5",
		OpenAIAPIKey:  "test-key",
		OpenAIAPIBase: "https://api.example.com/v1",
	}
	config.Set(cfg)

	b := newBridge()
	ab := NewAgentBridge(b)

	if ab == nil {
		t.Fatal("NewAgentBridge returned nil")
	}

	if ab.bridge != b {
		t.Error("bridge not set correctly")
	}

	if ab.agents == nil {
		t.Error("agents map not initialized")
	}

	if ab.initializer == nil {
		t.Error("initializer not set")
	}
}

func TestAgentBridge_GetAgent(t *testing.T) {
	cfg := &config.Config{
		Model:         "openai",
		ModelName:     "glm-5",
		OpenAIAPIKey:  "test-key",
		OpenAIAPIBase: "https://api.example.com/v1",
	}
	config.Set(cfg)

	b := newBridge()
	ab := NewAgentBridge(b)

	t.Run("default agent", func(t *testing.T) {
		a, err := ab.GetAgent("")
		if err != nil {
			t.Fatalf("GetAgent failed: %v", err)
		}
		if a == nil {
			t.Error("agent is nil")
		}
		if ab.defaultAgent == nil {
			t.Error("default agent not stored")
		}
	})

	t.Run("session agent", func(t *testing.T) {
		a, err := ab.GetAgent("session-1")
		if err != nil {
			t.Fatalf("GetAgent failed: %v", err)
		}
		if a == nil {
			t.Error("agent is nil")
		}
		if ab.agents["session-1"] == nil {
			t.Error("session agent not stored")
		}
	})

	t.Run("reuse existing agent", func(t *testing.T) {
		a1, _ := ab.GetAgent("session-2")
		a2, _ := ab.GetAgent("session-2")
		if a1 != a2 {
			t.Error("should return same agent instance")
		}
	})
}

func TestAgentBridge_ClearSession(t *testing.T) {
	cfg := &config.Config{
		Model:         "openai",
		ModelName:     "glm-5",
		OpenAIAPIKey:  "test-key",
		OpenAIAPIBase: "https://api.example.com/v1",
	}
	config.Set(cfg)

	b := newBridge()
	ab := NewAgentBridge(b)

	ab.GetAgent("")
	ab.GetAgent("session-1")
	ab.GetAgent("session-2")

	if ab.SessionCount() != 3 {
		t.Errorf("expected 3 sessions (1 default + 2 session), got %d", ab.SessionCount())
	}

	ab.ClearSession("session-1")

	if _, exists := ab.agents["session-1"]; exists {
		t.Error("session-1 should be cleared")
	}

	if _, exists := ab.agents["session-2"]; !exists {
		t.Error("session-2 should still exist")
	}

	if ab.SessionCount() != 2 {
		t.Errorf("expected 2 sessions after clear, got %d", ab.SessionCount())
	}
}

func TestAgentBridge_ClearAllSessions(t *testing.T) {
	cfg := &config.Config{
		Model:         "openai",
		ModelName:     "glm-5",
		OpenAIAPIKey:  "test-key",
		OpenAIAPIBase: "https://api.example.com/v1",
	}
	config.Set(cfg)

	b := newBridge()
	ab := NewAgentBridge(b)

	ab.GetAgent("")
	ab.GetAgent("session-1")
	ab.GetAgent("session-2")

	ab.ClearAllSessions()

	if ab.defaultAgent != nil {
		t.Error("default agent should be nil")
	}

	if len(ab.agents) != 0 {
		t.Errorf("agents should be empty, got %d", len(ab.agents))
	}
}

func TestAgentInitializer_BuildSystemPrompt(t *testing.T) {
	cfg := &config.Config{
		Model:         "openai",
		ModelName:     "glm-5",
		OpenAIAPIKey:  "test-key",
		OpenAIAPIBase: "https://api.example.com/v1",
		AgentMaxSteps: 15,
	}
	config.Set(cfg)

	b := newBridge()
	ab := NewAgentBridge(b)
	ai := ab.initializer

	prompt := ai.buildSystemPrompt(agent.NewToolRegistry(), "/tmp/workspace")

	if prompt == "" {
		t.Error("system prompt should not be empty")
	}

	if len(prompt) < 100 {
		t.Errorf("system prompt seems too short: %d chars", len(prompt))
	}
}

func TestAgentInitializer_GetToolInfos(t *testing.T) {
	cfg := &config.Config{
		Model:        "openai",
		ModelName:    "glm-5",
		OpenAIAPIKey: "test-key",
	}
	config.Set(cfg)

	b := newBridge()
	ab := NewAgentBridge(b)
	ai := ab.initializer

	registry := agent.NewToolRegistry()
	registry.Register(&mockTool{name: "read", desc: "read file"})
	registry.Register(&mockTool{name: "write", desc: "write file"})

	infos := ai.getToolInfos(registry)

	if len(infos) != 2 {
		t.Errorf("expected 2 tool infos, got %d", len(infos))
	}

	found := make(map[string]bool)
	for _, info := range infos {
		found[info.Name] = true
	}

	if !found["read"] || !found["write"] {
		t.Error("missing expected tools")
	}
}

type mockTool struct {
	name string
	desc string
}

func (m *mockTool) Name() string               { return m.name }
func (m *mockTool) Description() string        { return m.desc }
func (m *mockTool) Parameters() map[string]any { return nil }
func (m *mockTool) Stage() agent.ToolStage     { return agent.ToolStagePostProcess }
func (m *mockTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	return agent.NewToolResult("ok"), nil
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}
