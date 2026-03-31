package llm

import (
	"testing"
)

func TestNewOpenAIModel(t *testing.T) {
	tests := []struct {
		name        string
		config      ModelConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config with api key and model",
			config: ModelConfig{
				Model:  "gpt-4",
				APIKey: "test-api-key",
			},
			expectError: false,
		},
		{
			name: "missing api key",
			config: ModelConfig{
				Model: "gpt-4",
			},
			expectError: true,
			errorMsg:    "api_key is required",
		},
		{
			name: "missing model",
			config: ModelConfig{
				APIKey: "test-api-key",
			},
			expectError: true,
			errorMsg:    "model is required",
		},
		{
			name: "config with custom base url",
			config: ModelConfig{
				Model:   "deepseek-chat",
				APIKey:  "test-key",
				APIBase: "https://api.deepseek.com/v1",
			},
			expectError: false,
		},
		{
			name: "config with proxy",
			config: ModelConfig{
				Model:  "gpt-4",
				APIKey: "test-key",
				Proxy:  "http://localhost:8080",
			},
			expectError: false,
		},
		{
			name: "config with invalid proxy url",
			config: ModelConfig{
				Model:  "gpt-4",
				APIKey: "test-key",
				Proxy:  "://invalid-url",
			},
			expectError: true,
			errorMsg:    "invalid proxy URL",
		},
		{
			name: "config with timeout",
			config: ModelConfig{
				Model:          "gpt-4",
				APIKey:         "test-key",
				RequestTimeout: 30,
			},
			expectError: false,
		},
		{
			name: "config with provider",
			config: ModelConfig{
				Model:    "gpt-4",
				APIKey:   "test-key",
				Provider: "openai",
			},
			expectError: false,
		},
		{
			name: "full config",
			config: ModelConfig{
				ModelName:      "GPT-4 Turbo",
				Model:          "gpt-4-turbo",
				APIBase:        "https://api.openai.com/v1",
				APIKey:         "sk-test",
				Proxy:          "http://proxy:8080",
				Provider:       "openai",
				RequestTimeout: 60,
				DefaultOptions: CallOptions{MaxTokens: 4096},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := NewOpenAIModel(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if tt.errorMsg != "" && err.Error()[:len(tt.errorMsg)] != tt.errorMsg {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if model == nil {
					t.Error("model should not be nil")
				}
			}
		})
	}
}

func TestOpenAIModelName(t *testing.T) {
	tests := []struct {
		name         string
		config       ModelConfig
		expectedName string
	}{
		{
			name: "with model name",
			config: ModelConfig{
				ModelName: "My GPT-4",
				Model:     "gpt-4",
				APIKey:    "test-key",
			},
			expectedName: "My GPT-4",
		},
		{
			name: "without model name",
			config: ModelConfig{
				Model:  "gpt-4-turbo",
				APIKey: "test-key",
			},
			expectedName: "",
		},
		{
			name: "model name different from model",
			config: ModelConfig{
				ModelName: "Production Model",
				Model:     "gpt-4o-mini",
				APIKey:    "test-key",
			},
			expectedName: "Production Model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := NewOpenAIModel(tt.config)
			if err != nil {
				t.Fatalf("failed to create model: %v", err)
			}

			if model.Name() != tt.expectedName {
				t.Errorf("Name() = %q, want %q", model.Name(), tt.expectedName)
			}
		})
	}
}

func TestOpenAISupportsTools(t *testing.T) {
	tests := []struct {
		name        string
		config      ModelConfig
		expectTools bool
	}{
		{
			name: "openai provider supports tools",
			config: ModelConfig{
				Model:    "gpt-4",
				APIKey:   "test-key",
				Provider: "openai",
			},
			expectTools: true,
		},
		{
			name: "default provider supports tools",
			config: ModelConfig{
				Model:  "gpt-4",
				APIKey: "test-key",
			},
			expectTools: true,
		},
		{
			name: "empty provider supports tools",
			config: ModelConfig{
				Model:    "gpt-4",
				APIKey:   "test-key",
				Provider: "",
			},
			expectTools: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := NewOpenAIModel(tt.config)
			if err != nil {
				t.Fatalf("failed to create model: %v", err)
			}

			if model.SupportsTools() != tt.expectTools {
				t.Errorf("SupportsTools() = %v, want %v", model.SupportsTools(), tt.expectTools)
			}
		})
	}
}

func TestOpenAIModelInterface(t *testing.T) {
	config := ModelConfig{
		ModelName: "Test Model",
		Model:     "gpt-4",
		APIKey:    "test-key",
	}

	model, err := NewOpenAIModel(config)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	var _ Model = model

	if model.Name() != "Test Model" {
		t.Errorf("Name() = %q, want %q", model.Name(), "Test Model")
	}

	if !model.SupportsTools() {
		t.Error("SupportsTools() should return true")
	}
}

func TestOpenAIModelConfigPreservation(t *testing.T) {
	tests := []struct {
		name   string
		config ModelConfig
	}{
		{
			name: "preserve api base",
			config: ModelConfig{
				Model:   "gpt-4",
				APIKey:  "test-key",
				APIBase: "https://custom.api.com/v1",
			},
		},
		{
			name: "preserve proxy",
			config: ModelConfig{
				Model:  "gpt-4",
				APIKey: "test-key",
				Proxy:  "http://localhost:8080",
			},
		},
		{
			name: "preserve timeout",
			config: ModelConfig{
				Model:          "gpt-4",
				APIKey:         "test-key",
				RequestTimeout: 45,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := NewOpenAIModel(tt.config)
			if err != nil {
				t.Fatalf("failed to create model: %v", err)
			}

			if model == nil {
				t.Error("model should not be nil")
			}
		})
	}
}

func TestOpenAIDefaultOptionsMerge(t *testing.T) {
	config := ModelConfig{
		Model:  "gpt-4",
		APIKey: "test-key",
		DefaultOptions: CallOptions{
			MaxTokens: 1000,
		},
	}

	model, err := NewOpenAIModel(config)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	if model == nil {
		t.Error("model should not be nil")
	}
}

func TestConvertMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
	}{
		{
			name: "simple user message",
			messages: []Message{
				{Role: RoleUser, Content: "Hello"},
			},
		},
		{
			name: "system and user messages",
			messages: []Message{
				{Role: RoleSystem, Content: "You are helpful."},
				{Role: RoleUser, Content: "Hi"},
			},
		},
		{
			name: "message with tool calls",
			messages: []Message{
				{
					Role:    RoleAssistant,
					Content: "",
					ToolCalls: []ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: FunctionCall{
								Name:      "test_func",
								Arguments: "{}",
							},
						},
					},
				},
			},
		},
		{
			name: "tool response message",
			messages: []Message{
				{
					Role:       RoleTool,
					Content:    "result",
					ToolCallID: "call_1",
				},
			},
		},
		{
			name: "message with name",
			messages: []Message{
				{
					Role:    RoleUser,
					Content: "Hello",
					Name:    "Alice",
				},
			},
		},
		{
			name:     "empty messages",
			messages: []Message{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMessages(tt.messages)

			if len(result) != len(tt.messages) {
				t.Errorf("convertMessages length = %d, want %d", len(result), len(tt.messages))
			}

			for i, msg := range result {
				if i < len(tt.messages) && string(tt.messages[i].Role) != msg.Role {
					t.Errorf("message[%d] role = %q, want %q", i, msg.Role, tt.messages[i].Role)
				}
			}
		})
	}
}

func TestConvertUsage(t *testing.T) {
	tests := []struct {
		name             string
		promptTokens     int
		completionTokens int
		totalTokens      int
	}{
		{
			name:             "typical usage",
			promptTokens:     100,
			completionTokens: 50,
			totalTokens:      150,
		},
		{
			name:             "zero usage",
			promptTokens:     0,
			completionTokens: 0,
			totalTokens:      0,
		},
		{
			name:             "large usage",
			promptTokens:     10000,
			completionTokens: 5000,
			totalTokens:      15000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := Usage{
				PromptTokens:     tt.promptTokens,
				CompletionTokens: tt.completionTokens,
				TotalTokens:      tt.totalTokens,
			}

			if usage.PromptTokens != tt.promptTokens {
				t.Errorf("PromptTokens = %d, want %d", usage.PromptTokens, tt.promptTokens)
			}
			if usage.CompletionTokens != tt.completionTokens {
				t.Errorf("CompletionTokens = %d, want %d", usage.CompletionTokens, tt.completionTokens)
			}
			if usage.TotalTokens != tt.totalTokens {
				t.Errorf("TotalTokens = %d, want %d", usage.TotalTokens, tt.totalTokens)
			}
		})
	}
}

func TestOpenAIWithDifferentProviders(t *testing.T) {
	tests := []struct {
		name   string
		config ModelConfig
	}{
		{
			name: "deepseek provider",
			config: ModelConfig{
				Model:    "deepseek-chat",
				APIKey:   "test-key",
				APIBase:  "https://api.deepseek.com/v1",
				Provider: "deepseek",
			},
		},
		{
			name: "zhipu provider",
			config: ModelConfig{
				Model:    "glm-4",
				APIKey:   "test-key",
				APIBase:  "https://open.bigmodel.cn/api/paas/v4",
				Provider: "zhipu",
			},
		},
		{
			name: "moonshot provider",
			config: ModelConfig{
				Model:    "moonshot-v1-8k",
				APIKey:   "test-key",
				APIBase:  "https://api.moonshot.cn/v1",
				Provider: "moonshot",
			},
		},
		{
			name: "qwen provider",
			config: ModelConfig{
				Model:    "qwen-turbo",
				APIKey:   "test-key",
				APIBase:  "https://dashscope.aliyuncs.com/compatible-mode/v1",
				Provider: "qwen",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := NewOpenAIModel(tt.config)
			if err != nil {
				t.Fatalf("failed to create model: %v", err)
			}

			if model == nil {
				t.Error("model should not be nil")
			}

			if !model.SupportsTools() {
				t.Error("model should support tools")
			}
		})
	}
}

func TestOpenAIErrorConditions(t *testing.T) {
	tests := []struct {
		name        string
		config      ModelConfig
		expectError bool
	}{
		{
			name:        "empty config",
			config:      ModelConfig{},
			expectError: true,
		},
		{
			name: "only api key",
			config: ModelConfig{
				APIKey: "test-key",
			},
			expectError: true,
		},
		{
			name: "only model",
			config: ModelConfig{
				Model: "gpt-4",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOpenAIModel(tt.config)
			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
