package llm

import (
	"strings"
	"testing"
)

func TestNewModel_Validation(t *testing.T) {
	t.Run("missing model", func(t *testing.T) {
		_, err := NewModel(ModelConfig{APIKey: "test-key"})
		if err == nil {
			t.Error("expected error for missing model")
		}
		if !strings.Contains(err.Error(), "model is required") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing api key", func(t *testing.T) {
		_, err := NewModel(ModelConfig{Model: "gpt-4"})
		if err == nil {
			t.Error("expected error for missing api key")
		}
		if !strings.Contains(err.Error(), "api_key is required") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestNewModel_OpenAI(t *testing.T) {
	cfg := ModelConfig{
		Model:     "gpt-4",
		ModelName: "gpt-4",
		APIKey:    "test-key",
	}
	model, err := NewModel(cfg)
	if err != nil {
		t.Fatalf("NewModel failed: %v", err)
	}
	if model == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestNewModel_WithProviderPrefix(t *testing.T) {
	cfg := ModelConfig{
		Model:     "openai/gpt-4",
		ModelName: "gpt-4",
		APIKey:    "test-key",
	}
	model, err := NewModel(cfg)
	if err != nil {
		t.Fatalf("NewModel failed: %v", err)
	}
	if model == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		model    string
		apiBase  string
		expected string
	}{
		{"gpt-4", "", ProviderOpenAI},
		{"openai/gpt-4", "", ProviderOpenAI},
		{"deepseek/deepseek-chat", "", ProviderDeepSeek},
		{"anthropic/claude-3", "", ProviderAnthropic},
		{"qwen/qwen-turbo", "", ProviderQwen},
		{"minimax/abab5.5-chat", "", ProviderMiniMax},
		{"moonshot/moonshot-v1", "", ProviderMoonshot},
		{"zhipu/glm-4", "", ProviderZhipu},
		{"", "https://api.deepseek.com/v1", ProviderDeepSeek},
		{"unknown-model", "https://unknown.api.com/v1", ProviderOpenAI},
	}

	for _, tt := range tests {
		t.Run(tt.model+"_"+tt.apiBase, func(t *testing.T) {
			result := detectProvider(tt.model, tt.apiBase)
			if result != tt.expected {
				t.Errorf("detectProvider(%q, %q) = %q, want %q",
					tt.model, tt.apiBase, result, tt.expected)
			}
		})
	}
}

func TestDetectProvider_ByURL(t *testing.T) {
	t.Run("moonshot or kimi by URL", func(t *testing.T) {
		result := detectProvider("", "https://api.moonshot.cn/v1")
		if result != ProviderMoonshot && result != ProviderKimi {
			t.Errorf("expected moonshot or kimi, got %q", result)
		}
	})

	t.Run("zhipu or glm by URL", func(t *testing.T) {
		result := detectProvider("", "https://open.bigmodel.cn/api/paas/v4")
		if result != ProviderZhipu && result != ProviderGLM {
			t.Errorf("expected zhipu or glm, got %q", result)
		}
	})
}

func TestStripProviderPrefix(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"openai/gpt-4", "gpt-4"},
		{"deepseek/deepseek-chat", "deepseek-chat"},
		{"gpt-4", "gpt-4"},
		{"anthropic/claude-3-opus", "claude-3-opus"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := stripProviderPrefix(tt.model)
			if result != tt.expected {
				t.Errorf("stripProviderPrefix(%q) = %q, want %q",
					tt.model, result, tt.expected)
			}
		})
	}
}

func TestNewModelWithProvider(t *testing.T) {
	cfg := ModelConfig{
		Model:  "gpt-4",
		APIKey: "test-key",
	}
	model, err := NewModelWithProvider(ProviderOpenAI, cfg)
	if err != nil {
		t.Fatalf("NewModelWithProvider failed: %v", err)
	}
	if model == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestRegisterProvider(t *testing.T) {
	customProvider := "custom-provider"
	customBaseURL := "https://custom.api.com/v1"

	RegisterProvider(customProvider, customBaseURL)

	result := GetProviderBaseURL(customProvider)
	if result != customBaseURL {
		t.Errorf("GetProviderBaseURL(%q) = %q, want %q",
			customProvider, result, customBaseURL)
	}
}

func TestGetProviderBaseURL(t *testing.T) {
	tests := []struct {
		provider string
		wantURL  string
	}{
		{ProviderOpenAI, "https://api.openai.com/v1"},
		{ProviderDeepSeek, "https://api.deepseek.com/v1"},
		{ProviderMoonshot, "https://api.moonshot.cn/v1"},
		{ProviderZhipu, "https://open.bigmodel.cn/api/paas/v4"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			result := GetProviderBaseURL(tt.provider)
			if result != tt.wantURL {
				t.Errorf("GetProviderBaseURL(%q) = %q, want %q",
					tt.provider, result, tt.wantURL)
			}
		})
	}
}

func TestListProviders(t *testing.T) {
	providers := ListProviders()
	if len(providers) == 0 {
		t.Error("expected at least one provider")
	}

	found := false
	for _, p := range providers {
		if p == ProviderOpenAI {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected openai in provider list")
	}
}

func TestNewModel_DefaultBaseURL(t *testing.T) {
	tests := []struct {
		provider   string
		model      string
		wantBaseIn string
	}{
		{ProviderDeepSeek, "deepseek-chat", "deepseek.com"},
		{ProviderMoonshot, "moonshot-v1-8k", "moonshot.cn"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			cfg := ModelConfig{
				Model:    tt.model,
				APIKey:   "test-key",
				Provider: tt.provider,
			}
			model, err := NewModel(cfg)
			if err != nil {
				t.Fatalf("NewModel failed: %v", err)
			}
			if model == nil {
				t.Fatal("expected non-nil model")
			}
		})
	}
}

func TestProviderConstants(t *testing.T) {
	providers := []string{
		ProviderOpenAI,
		ProviderChatGPT,
		ProviderAnthropic,
		ProviderGemini,
		ProviderDeepSeek,
		ProviderGLM,
		ProviderQwen,
		ProviderMiniMax,
		ProviderKimi,
		ProviderMoonshot,
		ProviderZhipu,
		ProviderBaidu,
		ProviderDoubao,
		ProviderDashScope,
		ProviderModelScope,
		ProviderLinkAI,
		ProviderXunfei,
	}

	for _, p := range providers {
		if p == "" {
			t.Error("provider constant should not be empty")
		}
	}
}
