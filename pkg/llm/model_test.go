package llm

import (
	"testing"
)

func withTopP(topP float64) Option {
	return func(o *CallOptions) {
		o.TopP = &topP
	}
}

func withStop(stop ...string) Option {
	return func(o *CallOptions) {
		o.Stop = stop
	}
}

func withToolChoice(choice any) Option {
	return func(o *CallOptions) {
		o.ToolChoice = choice
	}
}

func withResponseFormat(format any) Option {
	return func(o *CallOptions) {
		o.ResponseFormat = format
	}
}

func withFrequencyPenalty(penalty float64) Option {
	return func(o *CallOptions) {
		o.FrequencyPenalty = &penalty
	}
}

func withPresencePenalty(penalty float64) Option {
	return func(o *CallOptions) {
		o.PresencePenalty = &penalty
	}
}

func withUser(user string) Option {
	return func(o *CallOptions) {
		o.User = user
	}
}

func withExtra(key string, value any) Option {
	return func(o *CallOptions) {
		if o.Extra == nil {
			o.Extra = make(map[string]any)
		}
		o.Extra[key] = value
	}
}

func mergeOptions(opts ...Option) CallOptions {
	result := CallOptions{}
	for _, opt := range opts {
		opt(&result)
	}
	return result
}

func TestMessageRole(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		expected string
	}{
		{
			name:     "system role",
			role:     RoleSystem,
			expected: "system",
		},
		{
			name:     "user role",
			role:     RoleUser,
			expected: "user",
		},
		{
			name:     "assistant role",
			role:     RoleAssistant,
			expected: "assistant",
		},
		{
			name:     "tool role",
			role:     RoleTool,
			expected: "tool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.role) != tt.expected {
				t.Errorf("Role = %q, want %q", tt.role, tt.expected)
			}
		})
	}
}

func TestMessage(t *testing.T) {
	tests := []struct {
		name    string
		message Message
	}{
		{
			name: "simple user message",
			message: Message{
				Role:    RoleUser,
				Content: "Hello, world!",
			},
		},
		{
			name: "system message with name",
			message: Message{
				Role:    RoleSystem,
				Content: "You are a helpful assistant.",
				Name:    "system",
			},
		},
		{
			name: "assistant message with tool calls",
			message: Message{
				Role:    RoleAssistant,
				Content: "",
				ToolCalls: []ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location": "Beijing"}`,
						},
					},
				},
			},
		},
		{
			name: "tool response message",
			message: Message{
				Role:       RoleTool,
				Content:    `{"temperature": 25}`,
				ToolCallID: "call_123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.message.Role == "" {
				t.Error("Message role should not be empty")
			}
		})
	}
}

func TestToolDefinition(t *testing.T) {
	tests := []struct {
		name     string
		tool     ToolDefinition
		toolType string
		funcName string
	}{
		{
			name: "simple function tool",
			tool: ToolDefinition{
				Type: "function",
				Function: FunctionDefinition{
					Name:        "get_weather",
					Description: "Get the current weather",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "City name",
							},
						},
					},
				},
			},
			toolType: "function",
			funcName: "get_weather",
		},
		{
			name: "tool without parameters",
			tool: ToolDefinition{
				Type: "function",
				Function: FunctionDefinition{
					Name:        "simple_action",
					Description: "Perform a simple action",
				},
			},
			toolType: "function",
			funcName: "simple_action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tool.Type != tt.toolType {
				t.Errorf("Tool type = %q, want %q", tt.tool.Type, tt.toolType)
			}
			if tt.tool.Function.Name != tt.funcName {
				t.Errorf("Function name = %q, want %q", tt.tool.Function.Name, tt.funcName)
			}
		})
	}
}

func TestToolCall(t *testing.T) {
	tests := []struct {
		name         string
		toolCall     ToolCall
		expectedID   string
		expectedType string
		expectedFunc string
		expectedArgs string
	}{
		{
			name: "complete tool call",
			toolCall: ToolCall{
				ID:   "call_abc123",
				Type: "function",
				Function: FunctionCall{
					Name:      "search",
					Arguments: `{"query": "golang testing"}`,
				},
			},
			expectedID:   "call_abc123",
			expectedType: "function",
			expectedFunc: "search",
			expectedArgs: `{"query": "golang testing"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.toolCall.ID != tt.expectedID {
				t.Errorf("ToolCall ID = %q, want %q", tt.toolCall.ID, tt.expectedID)
			}
			if tt.toolCall.Type != tt.expectedType {
				t.Errorf("ToolCall Type = %q, want %q", tt.toolCall.Type, tt.expectedType)
			}
			if tt.toolCall.Function.Name != tt.expectedFunc {
				t.Errorf("Function Name = %q, want %q", tt.toolCall.Function.Name, tt.expectedFunc)
			}
			if tt.toolCall.Function.Arguments != tt.expectedArgs {
				t.Errorf("Function Arguments = %q, want %q", tt.toolCall.Function.Arguments, tt.expectedArgs)
			}
		})
	}
}

func TestUsage(t *testing.T) {
	tests := []struct {
		name               string
		usage              Usage
		expectedPrompt     int
		expectedCompletion int
		expectedTotal      int
	}{
		{
			name: "typical usage",
			usage: Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
			expectedPrompt:     100,
			expectedCompletion: 50,
			expectedTotal:      150,
		},
		{
			name:               "zero usage",
			usage:              Usage{},
			expectedPrompt:     0,
			expectedCompletion: 0,
			expectedTotal:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.usage.PromptTokens != tt.expectedPrompt {
				t.Errorf("PromptTokens = %d, want %d", tt.usage.PromptTokens, tt.expectedPrompt)
			}
			if tt.usage.CompletionTokens != tt.expectedCompletion {
				t.Errorf("CompletionTokens = %d, want %d", tt.usage.CompletionTokens, tt.expectedCompletion)
			}
			if tt.usage.TotalTokens != tt.expectedTotal {
				t.Errorf("TotalTokens = %d, want %d", tt.usage.TotalTokens, tt.expectedTotal)
			}
		})
	}
}

func TestResponse(t *testing.T) {
	tests := []struct {
		name            string
		response        Response
		expectedContent string
		expectedReason  string
	}{
		{
			name: "text response",
			response: Response{
				Content:      "Hello! How can I help you?",
				Usage:        Usage{PromptTokens: 10, CompletionTokens: 8, TotalTokens: 18},
				FinishReason: "stop",
				Model:        "gpt-4",
			},
			expectedContent: "Hello! How can I help you?",
			expectedReason:  "stop",
		},
		{
			name: "tool call response",
			response: Response{
				Content: "",
				ToolCalls: []ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: FunctionCall{
							Name:      "calculate",
							Arguments: `{"expression": "2+2"}`,
						},
					},
				},
				FinishReason: "tool_calls",
			},
			expectedContent: "",
			expectedReason:  "tool_calls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.response.Content != tt.expectedContent {
				t.Errorf("Content = %q, want %q", tt.response.Content, tt.expectedContent)
			}
			if tt.response.FinishReason != tt.expectedReason {
				t.Errorf("FinishReason = %q, want %q", tt.response.FinishReason, tt.expectedReason)
			}
		})
	}
}

func TestStreamChunk(t *testing.T) {
	tests := []struct {
		name        string
		chunk       StreamChunk
		expectDelta string
		expectDone  bool
	}{
		{
			name: "text delta chunk",
			chunk: StreamChunk{
				Delta: "Hello",
			},
			expectDelta: "Hello",
			expectDone:  false,
		},
		{
			name: "done chunk",
			chunk: StreamChunk{
				Done:         true,
				FinishReason: "stop",
			},
			expectDelta: "",
			expectDone:  true,
		},
		{
			name: "tool call delta chunk",
			chunk: StreamChunk{
				ToolCallDelta: &ToolCallDelta{
					Index: 0,
					ID:    "call_xyz",
					Type:  "function",
					Function: &FunctionCallDelta{
						Name:      "search",
						Arguments: `{"q"`,
					},
				},
			},
			expectDelta: "",
			expectDone:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.chunk.Delta != tt.expectDelta {
				t.Errorf("Delta = %q, want %q", tt.chunk.Delta, tt.expectDelta)
			}
			if tt.chunk.Done != tt.expectDone {
				t.Errorf("Done = %v, want %v", tt.chunk.Done, tt.expectDone)
			}
		})
	}
}

func TestCallOptions(t *testing.T) {
	tests := []struct {
		name  string
		opts  []Option
		check func(CallOptions) bool
	}{
		{
			name: "with temperature",
			opts: []Option{WithTemperature(0.7)},
			check: func(co CallOptions) bool {
				return co.Temperature != nil && *co.Temperature == 0.7
			},
		},
		{
			name: "with max tokens",
			opts: []Option{WithMaxTokens(1000)},
			check: func(co CallOptions) bool {
				return co.MaxTokens == 1000
			},
		},
		{
			name: "with top p",
			opts: []Option{withTopP(0.9)},
			check: func(co CallOptions) bool {
				return co.TopP != nil && *co.TopP == 0.9
			},
		},
		{
			name: "with stop sequences",
			opts: []Option{withStop("END", "STOP")},
			check: func(co CallOptions) bool {
				return len(co.Stop) == 2 && co.Stop[0] == "END" && co.Stop[1] == "STOP"
			},
		},
		{
			name: "with tools",
			opts: []Option{WithTools([]ToolDefinition{
				{
					Type: "function",
					Function: FunctionDefinition{
						Name: "test_func",
					},
				},
			})},
			check: func(co CallOptions) bool {
				return len(co.Tools) == 1 && co.Tools[0].Function.Name == "test_func"
			},
		},
		{
			name: "with tool choice",
			opts: []Option{withToolChoice("auto")},
			check: func(co CallOptions) bool {
				return co.ToolChoice == "auto"
			},
		},
		{
			name: "with system prompt",
			opts: []Option{WithSystemPrompt("You are helpful.")},
			check: func(co CallOptions) bool {
				return co.SystemPrompt == "You are helpful."
			},
		},
		{
			name: "with response format",
			opts: []Option{withResponseFormat("json_object")},
			check: func(co CallOptions) bool {
				return co.ResponseFormat == "json_object"
			},
		},
		{
			name: "with frequency penalty",
			opts: []Option{withFrequencyPenalty(0.5)},
			check: func(co CallOptions) bool {
				return co.FrequencyPenalty != nil && *co.FrequencyPenalty == 0.5
			},
		},
		{
			name: "with presence penalty",
			opts: []Option{withPresencePenalty(0.3)},
			check: func(co CallOptions) bool {
				return co.PresencePenalty != nil && *co.PresencePenalty == 0.3
			},
		},
		{
			name: "with user",
			opts: []Option{withUser("user123")},
			check: func(co CallOptions) bool {
				return co.User == "user123"
			},
		},
		{
			name: "with extra",
			opts: []Option{withExtra("custom_key", "custom_value")},
			check: func(co CallOptions) bool {
				return co.Extra != nil && co.Extra["custom_key"] == "custom_value"
			},
		},
		{
			name: "multiple options",
			opts: []Option{
				WithTemperature(0.5),
				WithMaxTokens(500),
				withUser("test_user"),
			},
			check: func(co CallOptions) bool {
				return co.Temperature != nil && *co.Temperature == 0.5 &&
					co.MaxTokens == 500 && co.User == "test_user"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeOptions(tt.opts...)
			if !tt.check(result) {
				t.Errorf("CallOptions check failed for test %q", tt.name)
			}
		})
	}
}

func TestMergeOptions(t *testing.T) {
	tests := []struct {
		name     string
		opts     []Option
		expected CallOptions
	}{
		{
			name:     "empty options",
			opts:     nil,
			expected: CallOptions{},
		},
		{
			name:     "single option",
			opts:     []Option{WithMaxTokens(100)},
			expected: CallOptions{MaxTokens: 100},
		},
		{
			name: "multiple options combined",
			opts: []Option{
				WithTemperature(0.8),
				WithMaxTokens(2000),
				withTopP(0.95),
				withStop("\n", "END"),
			},
			expected: CallOptions{
				Temperature: ptr(0.8),
				MaxTokens:   2000,
				TopP:        ptr(0.95),
				Stop:        []string{"\n", "END"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeOptions(tt.opts...)

			if tt.expected.MaxTokens != result.MaxTokens {
				t.Errorf("MaxTokens = %d, want %d", result.MaxTokens, tt.expected.MaxTokens)
			}

			if tt.expected.Temperature != nil && result.Temperature == nil {
				t.Error("Temperature is nil, expected non-nil")
			} else if tt.expected.Temperature != nil && *tt.expected.Temperature != *result.Temperature {
				t.Errorf("Temperature = %v, want %v", *result.Temperature, *tt.expected.Temperature)
			}

			if tt.expected.TopP != nil && result.TopP == nil {
				t.Error("TopP is nil, expected non-nil")
			} else if tt.expected.TopP != nil && *tt.expected.TopP != *result.TopP {
				t.Errorf("TopP = %v, want %v", *result.TopP, *tt.expected.TopP)
			}

			if len(tt.expected.Stop) != len(result.Stop) {
				t.Errorf("Stop length = %d, want %d", len(result.Stop), len(tt.expected.Stop))
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		opts     CallOptions
		defaults CallOptions
		check    func(CallOptions) bool
	}{
		{
			name:     "apply temperature default",
			opts:     CallOptions{},
			defaults: CallOptions{Temperature: ptr(0.7)},
			check: func(co CallOptions) bool {
				return co.Temperature != nil && *co.Temperature == 0.7
			},
		},
		{
			name:     "temperature override default",
			opts:     CallOptions{Temperature: ptr(0.5)},
			defaults: CallOptions{Temperature: ptr(0.7)},
			check: func(co CallOptions) bool {
				return co.Temperature != nil && *co.Temperature == 0.5
			},
		},
		{
			name:     "apply max tokens default",
			opts:     CallOptions{},
			defaults: CallOptions{MaxTokens: 1000},
			check: func(co CallOptions) bool {
				return co.MaxTokens == 1000
			},
		},
		{
			name:     "apply system prompt default",
			opts:     CallOptions{},
			defaults: CallOptions{SystemPrompt: "Default system prompt"},
			check: func(co CallOptions) bool {
				return co.SystemPrompt == "Default system prompt"
			},
		},
		{
			name:     "apply stop sequences default",
			opts:     CallOptions{},
			defaults: CallOptions{Stop: []string{"END"}},
			check: func(co CallOptions) bool {
				return len(co.Stop) == 1 && co.Stop[0] == "END"
			},
		},
		{
			name:     "apply extra default",
			opts:     CallOptions{},
			defaults: CallOptions{Extra: map[string]any{"key": "value"}},
			check: func(co CallOptions) bool {
				return co.Extra != nil && co.Extra["key"] == "value"
			},
		},
		{
			name: "multiple defaults applied",
			opts: CallOptions{MaxTokens: 500},
			defaults: CallOptions{
				Temperature:  ptr(0.7),
				MaxTokens:    1000,
				SystemPrompt: "Default",
			},
			check: func(co CallOptions) bool {
				return co.Temperature != nil && *co.Temperature == 0.7 &&
					co.MaxTokens == 500 &&
					co.SystemPrompt == "Default"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opts
			result.ApplyDefaults(tt.defaults)
			if !tt.check(result) {
				t.Errorf("ApplyDefaults check failed for test %q", tt.name)
			}
		})
	}
}

func TestModelConfig(t *testing.T) {
	tests := []struct {
		name   string
		config ModelConfig
	}{
		{
			name: "minimal config",
			config: ModelConfig{
				Model:  "gpt-4",
				APIKey: "test-key",
			},
		},
		{
			name: "full config",
			config: ModelConfig{
				ModelName:      "GPT-4",
				Model:          "gpt-4-turbo",
				APIBase:        "https://api.openai.com/v1",
				APIKey:         "sk-test",
				Proxy:          "http://localhost:8080",
				Provider:       "openai",
				RequestTimeout: 30,
				DefaultOptions: CallOptions{MaxTokens: 4096},
				Extra:          map[string]any{"custom": "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.Model == "" {
				t.Error("Model should not be empty")
			}
			if tt.config.APIKey == "" {
				t.Error("APIKey should not be empty")
			}
		})
	}
}

func TestModelInfo(t *testing.T) {
	tests := []struct {
		name    string
		info    ModelInfo
		checkID string
	}{
		{
			name: "gpt-4 info",
			info: ModelInfo{
				ID:                "gpt-4",
				Name:              "GPT-4",
				Provider:          "openai",
				ContextWindow:     8192,
				SupportsVision:    false,
				SupportsTools:     true,
				SupportsStreaming: true,
			},
			checkID: "gpt-4",
		},
		{
			name: "gpt-4-vision info",
			info: ModelInfo{
				ID:                "gpt-4-vision-preview",
				Name:              "GPT-4 Vision",
				Provider:          "openai",
				ContextWindow:     128000,
				SupportsVision:    true,
				SupportsTools:     true,
				SupportsStreaming: true,
			},
			checkID: "gpt-4-vision-preview",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.info.ID != tt.checkID {
				t.Errorf("ID = %q, want %q", tt.info.ID, tt.checkID)
			}
		})
	}
}

func ptr(f float64) *float64 {
	return &f
}
