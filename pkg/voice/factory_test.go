package voice

import (
	"context"
	"sync"
	"testing"
)

type testEngine struct {
	name string
	cfg  Config
}

func (e *testEngine) TTS(ctx context.Context, text string) ([]byte, error) {
	return []byte(text), nil
}

func (e *testEngine) ASR(ctx context.Context, audio []byte) (string, error) {
	return string(audio), nil
}

func (e *testEngine) Name() string {
	return e.name
}

func TestVoiceFactory(t *testing.T) {
	enginesMu.Lock()
	engines = make(map[EngineType]func(Config) (VoiceEngine, error))
	enginesMu.Unlock()

	tests := []struct {
		name        string
		register    bool
		engineType  EngineType
		cfg         Config
		expectError bool
		errorMsg    string
	}{
		{"空引擎类型", false, "", Config{}, true, "语音引擎类型不能为空"},
		{"未注册的引擎类型", false, EngineType("unknown"), Config{EngineType: "unknown"}, true, "未知的语音引擎类型: unknown"},
		{
			name:        "已注册的引擎类型",
			register:    true,
			engineType:  EngineType("test"),
			cfg:         Config{EngineType: "test", APIKey: "test-key", Language: "zh-CN"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.register {
				RegisterEngine(tt.engineType, func(cfg Config) (VoiceEngine, error) {
					return &testEngine{name: "test-engine", cfg: cfg}, nil
				})
				defer func() {
					enginesMu.Lock()
					delete(engines, tt.engineType)
					enginesMu.Unlock()
				}()
			}

			engine, err := NewEngine(tt.cfg)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("error = %s, want %s", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if engine == nil {
					t.Error("engine should not be nil")
				}
			}
		})
	}
}

func TestRegisterEngine(t *testing.T) {
	enginesMu.Lock()
	engines = make(map[EngineType]func(Config) (VoiceEngine, error))
	enginesMu.Unlock()

	tests := []struct {
		name       string
		engineType EngineType
	}{
		{"注册OpenAI模拟引擎", EngineType("mock-openai")},
		{"注册Azure模拟引擎", EngineType("mock-azure")},
		{"注册自定义引擎", EngineType("custom")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constructor := func(cfg Config) (VoiceEngine, error) {
				return &testEngine{name: string(tt.engineType)}, nil
			}

			RegisterEngine(tt.engineType, constructor)

			enginesMu.RLock()
			_, ok := engines[tt.engineType]
			enginesMu.RUnlock()

			if !ok {
				t.Errorf("engine %s should be registered", tt.engineType)
			}

			enginesMu.Lock()
			delete(engines, tt.engineType)
			enginesMu.Unlock()
		})
	}
}

func TestListEngines(t *testing.T) {
	enginesMu.Lock()
	engines = make(map[EngineType]func(Config) (VoiceEngine, error))
	enginesMu.Unlock()

	list := ListEngines()
	if len(list) != 0 {
		t.Errorf("initial list should be empty, got %d", len(list))
	}

	testEngines := []EngineType{"list-test-1", "list-test-2", "list-test-3"}
	for _, et := range testEngines {
		RegisterEngine(et, func(cfg Config) (VoiceEngine, error) {
			return &testEngine{name: string(et)}, nil
		})
	}

	list = ListEngines()
	if len(list) != len(testEngines) {
		t.Errorf("list length = %d, want %d", len(list), len(testEngines))
	}

	engineMap := make(map[EngineType]bool)
	for _, et := range list {
		engineMap[et] = true
	}

	for _, et := range testEngines {
		if !engineMap[et] {
			t.Errorf("engine %s not found in list", et)
		}
	}

	enginesMu.Lock()
	for _, et := range testEngines {
		delete(engines, et)
	}
	enginesMu.Unlock()
}

func TestApplyDefaults(t *testing.T) {
	tests := []struct {
		name               string
		input              Config
		expectedLanguage   string
		expectedSampleRate int
		expectedFormat     string
		expectedTimeout    int
	}{
		{
			name:               "空配置应用默认值",
			input:              Config{},
			expectedLanguage:   "zh-CN",
			expectedSampleRate: 16000,
			expectedFormat:     "mp3",
			expectedTimeout:    30,
		},
		{
			name: "部分配置保留原值",
			input: Config{
				Language:   "en-US",
				SampleRate: 0,
				Timeout:    60,
			},
			expectedLanguage:   "en-US",
			expectedSampleRate: 16000,
			expectedFormat:     "mp3",
			expectedTimeout:    60,
		},
		{
			name: "完整配置保留原值",
			input: Config{
				Language:     "ja-JP",
				SampleRate:   44100,
				OutputFormat: "wav",
				Timeout:      120,
			},
			expectedLanguage:   "ja-JP",
			expectedSampleRate: 44100,
			expectedFormat:     "wav",
			expectedTimeout:    120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.input
			applyDefaults(&cfg)

			if cfg.Language != tt.expectedLanguage {
				t.Errorf("Language = %s, want %s", cfg.Language, tt.expectedLanguage)
			}
			if cfg.SampleRate != tt.expectedSampleRate {
				t.Errorf("SampleRate = %d, want %d", cfg.SampleRate, tt.expectedSampleRate)
			}
			if cfg.OutputFormat != tt.expectedFormat {
				t.Errorf("OutputFormat = %s, want %s", cfg.OutputFormat, tt.expectedFormat)
			}
			if cfg.Timeout != tt.expectedTimeout {
				t.Errorf("Timeout = %d, want %d", cfg.Timeout, tt.expectedTimeout)
			}
		})
	}
}

func TestConcurrentRegistration(t *testing.T) {
	enginesMu.Lock()
	engines = make(map[EngineType]func(Config) (VoiceEngine, error))
	enginesMu.Unlock()

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			engineType := EngineType("concurrent-test")
			RegisterEngine(engineType, func(cfg Config) (VoiceEngine, error) {
				return &testEngine{name: string(engineType)}, nil
			})
		}(i)
	}

	wg.Wait()

	enginesMu.RLock()
	_, ok := engines["concurrent-test"]
	enginesMu.RUnlock()

	if !ok {
		t.Error("engine should be registered after concurrent registration")
	}

	enginesMu.Lock()
	delete(engines, "concurrent-test")
	enginesMu.Unlock()
}

func TestConcurrentListEngines(t *testing.T) {
	enginesMu.Lock()
	engines = make(map[EngineType]func(Config) (VoiceEngine, error))
	enginesMu.Unlock()

	for i := 0; i < 5; i++ {
		engineType := EngineType("list-concurrent-test")
		RegisterEngine(engineType, func(cfg Config) (VoiceEngine, error) {
			return &testEngine{name: "list-concurrent-test"}, nil
		})
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ListEngines()
		}()
	}

	wg.Wait()

	enginesMu.Lock()
	delete(engines, "list-concurrent-test")
	enginesMu.Unlock()
}
