package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetNotNil(t *testing.T) {
	cfg := Get()
	if cfg == nil {
		t.Error("Get should not return nil")
	}
}

func TestMaskSensitive(t *testing.T) {
	cfg := &Config{
		ChannelType:  "web",
		Model:        "openai",
		OpenAIAPIKey: "sk-test123456789",
	}

	masked := cfg.MaskSensitive()
	if masked["open_ai_api_key"] == "sk-test123456789" {
		t.Error("API key should be masked")
	}
	if masked["channel_type"] != "web" {
		t.Errorf("ChannelType = %v, want web", masked["channel_type"])
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "***"},
		{"abc", "***"},
		{"abcdef", "***"},
		{"abcdefg", "abc*****efg"},
		{"sk-test123456789secret", "sk-*****ret"},
	}

	for _, tt := range tests {
		result := maskKey(tt.input)
		if result != tt.expected {
			t.Errorf("maskKey(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestConfig_Methods(t *testing.T) {
	t.Run("IsAgentEnabled", func(t *testing.T) {
		cfg := &Config{Agent: true}
		if !cfg.IsAgentEnabled() {
			t.Error("expected true")
		}
		cfg.Agent = false
		if cfg.IsAgentEnabled() {
			t.Error("expected false")
		}
	})

	t.Run("GetModel", func(t *testing.T) {
		cfg := &Config{Model: "gpt-4"}
		if cfg.GetModel() != "gpt-4" {
			t.Error("expected gpt-4")
		}
	})

	t.Run("GetChannelType", func(t *testing.T) {
		cfg := &Config{ChannelType: "feishu"}
		if cfg.GetChannelType() != "feishu" {
			t.Error("expected feishu")
		}
	})

	t.Run("GetOpenAIAPIKey", func(t *testing.T) {
		cfg := &Config{OpenAIAPIKey: "sk-test"}
		if cfg.GetOpenAIAPIKey() != "sk-test" {
			t.Error("expected sk-test")
		}
	})

	t.Run("GetWorkspace default", func(t *testing.T) {
		cfg := &Config{}
		if cfg.GetWorkspace() != "~/cow" {
			t.Errorf("expected ~/cow, got %s", cfg.GetWorkspace())
		}
	})

	t.Run("GetWorkspace custom", func(t *testing.T) {
		cfg := &Config{AgentWorkspace: "/custom/workspace"}
		if cfg.GetWorkspace() != "/custom/workspace" {
			t.Errorf("expected /custom/workspace, got %s", cfg.GetWorkspace())
		}
	})

	t.Run("IsDebugEnabled", func(t *testing.T) {
		cfg := &Config{Debug: true}
		if !cfg.IsDebugEnabled() {
			t.Error("expected true")
		}
		cfg.Debug = false
		if cfg.IsDebugEnabled() {
			t.Error("expected false")
		}
	})
}

func TestConfig_ToolsMethods(t *testing.T) {
	t.Run("GetTools nil", func(t *testing.T) {
		cfg := &Config{}
		tools := cfg.GetTools()
		if tools == nil {
			t.Error("expected non-nil ToolsConfig")
		}
	})

	t.Run("GetTools set", func(t *testing.T) {
		cfg := &Config{Tools: &ToolsConfig{}}
		tools := cfg.GetTools()
		if tools == nil {
			t.Error("expected non-nil ToolsConfig")
		}
	})

	t.Run("GetWebSearch nil", func(t *testing.T) {
		cfg := &Config{}
		search := cfg.GetWebSearch()
		if search != nil {
			t.Error("expected nil")
		}
	})

	t.Run("GetWebSearch set", func(t *testing.T) {
		cfg := &Config{
			Tools: &ToolsConfig{
				Web: &WebToolsConfig{
					Search: &WebSearchConfig{Provider: "duckduckgo"},
				},
			},
		}
		search := cfg.GetWebSearch()
		if search == nil || search.Provider != "duckduckgo" {
			t.Error("expected duckduckgo provider")
		}
	})

	t.Run("GetWebFetch nil", func(t *testing.T) {
		cfg := &Config{}
		fetch := cfg.GetWebFetch()
		if fetch != nil {
			t.Error("expected nil")
		}
	})

	t.Run("GetWebFetch set", func(t *testing.T) {
		cfg := &Config{
			Tools: &ToolsConfig{
				Web: &WebToolsConfig{
					Fetch: &WebFetchConfig{MaxChars: 10000},
				},
			},
		}
		fetch := cfg.GetWebFetch()
		if fetch == nil || fetch.MaxChars != 10000 {
			t.Error("expected MaxChars 10000")
		}
	})
}

func TestConfig_IsWebSearchEnabled(t *testing.T) {
	t.Run("nil tools", func(t *testing.T) {
		cfg := &Config{}
		if !cfg.IsWebSearchEnabled() {
			t.Error("expected true (duckduckgo default)")
		}
	})

	t.Run("explicitly disabled", func(t *testing.T) {
		enabled := false
		cfg := &Config{
			Tools: &ToolsConfig{
				Web: &WebToolsConfig{
					Search: &WebSearchConfig{Enabled: &enabled},
				},
			},
		}
		if cfg.IsWebSearchEnabled() {
			t.Error("expected false")
		}
	})

	t.Run("enabled", func(t *testing.T) {
		enabled := true
		cfg := &Config{
			Tools: &ToolsConfig{
				Web: &WebToolsConfig{
					Search: &WebSearchConfig{Enabled: &enabled},
				},
			},
		}
		if !cfg.IsWebSearchEnabled() {
			t.Error("expected true")
		}
	})
}

func TestConfig_IsWebFetchEnabled(t *testing.T) {
	t.Run("nil tools", func(t *testing.T) {
		cfg := &Config{}
		if !cfg.IsWebFetchEnabled() {
			t.Error("expected true (default)")
		}
	})

	t.Run("explicitly disabled", func(t *testing.T) {
		enabled := false
		cfg := &Config{
			Tools: &ToolsConfig{
				Web: &WebToolsConfig{
					Fetch: &WebFetchConfig{Enabled: &enabled},
				},
			},
		}
		if cfg.IsWebFetchEnabled() {
			t.Error("expected false")
		}
	})
}

func TestSet(t *testing.T) {
	newCfg := &Config{Model: "test-model"}
	Set(newCfg)

	cfg := Get()
	if cfg.Model != "test-model" {
		t.Errorf("expected test-model, got %s", cfg.Model)
	}

	// Reset to nil for other tests
	Set(nil)
}

func TestGetDefaultConfig(t *testing.T) {
	cfg := getDefaultConfig()

	if cfg.Model != "gpt-3.5-turbo" {
		t.Errorf("expected gpt-3.5-turbo, got %s", cfg.Model)
	}
	if cfg.WebPort != 9899 {
		t.Errorf("expected 9899, got %d", cfg.WebPort)
	}
	if cfg.AgentWorkspace != "~/cow" {
		t.Errorf("expected ~/cow, got %s", cfg.AgentWorkspace)
	}
}

func TestLoad(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{
		"model": "gpt-4",
		"bot_type": "test",
		"channel_type": "web",
		"web_port": 9000,
		"debug": true
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Reset global state
	Set(nil)

	// Load config
	if err := Load(configPath); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	cfg := Get()
	if cfg.Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", cfg.Model)
	}
	if cfg.BotType != "test" {
		t.Errorf("expected test, got %s", cfg.BotType)
	}
	if cfg.ChannelType != "web" {
		t.Errorf("expected web, got %s", cfg.ChannelType)
	}
	if cfg.WebPort != 9000 {
		t.Errorf("expected 9000, got %d", cfg.WebPort)
	}
	if !cfg.Debug {
		t.Error("expected debug true")
	}

	// Reset for other tests
	Set(nil)
}

func TestLoadNonExistent(t *testing.T) {
	_ = Load("/nonexistent/path/config.json")
}

func TestSyncToEnv(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("TEST_OPENAI_API_KEY")
	os.Unsetenv("TEST_CLAUDE_API_KEY")

	cfg := &Config{
		SyncToEnv:     true,
		OpenAIAPIKey:  "test-openai-key",
		ClaudeAPIKey:  "test-claude-key",
		GeminiAPIKey:  "test-gemini-key",
		MinimaxAPIKey: "test-minimax-key",
	}

	syncToEnv(cfg)

	if os.Getenv("OPENAI_API_KEY") != "test-openai-key" {
		t.Error("OPENAI_API_KEY not set")
	}
	if os.Getenv("CLAUDE_API_KEY") != "test-claude-key" {
		t.Error("CLAUDE_API_KEY not set")
	}
	if os.Getenv("GEMINI_API_KEY") != "test-gemini-key" {
		t.Error("GEMINI_API_KEY not set")
	}

	// Cleanup
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("CLAUDE_API_KEY")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("MINIMAX_API_KEY")
}
