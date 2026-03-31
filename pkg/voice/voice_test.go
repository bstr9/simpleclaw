// Package voice 提供语音处理核心功能
// voice_test.go 测试语音接口
package voice

import (
	"context"
	"testing"
)

// mockEngine 用于测试的模拟引擎
type mockEngine struct {
	name string
}

func (m *mockEngine) TTS(ctx context.Context, text string) ([]byte, error) {
	return []byte(text), nil
}

func (m *mockEngine) ASR(ctx context.Context, audio []byte) (string, error) {
	return string(audio), nil
}

func (m *mockEngine) Name() string {
	return m.name
}

// TestVoiceEngineType 测试语音引擎类型
func TestVoiceEngineType(t *testing.T) {
	tests := []struct {
		name       string
		engineType EngineType
		expected   string
	}{
		{"OpenAI引擎类型", EngineOpenAI, "openai"},
		{"Azure引擎类型", EngineAzure, "azure"},
		{"百度引擎类型", EngineBaidu, "baidu"},
		{"阿里引擎类型", EngineAli, "ali"},
		{"腾讯引擎类型", EngineTencent, "tencent"},
		{"讯飞引擎类型", EngineXunfei, "xunfei"},
		{"Google引擎类型", EngineGoogle, "google"},
		{"Edge引擎类型", EngineEdge, "edge"},
		{"ElevenLabs引擎类型", EngineElevenLabs, "elevenlabs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.engineType) != tt.expected {
				t.Errorf("EngineType = %s, want %s", tt.engineType, tt.expected)
			}
		})
	}
}

// TestTTSEngineInterface 测试TTS引擎接口
func TestTTSEngineInterface(t *testing.T) {
	tests := []struct {
		name     string
		engine   TTSEngine
		text     string
		expected []byte
	}{
		{"模拟TTS引擎", &mockEngine{name: "test-tts"}, "你好世界", []byte("你好世界")},
		{"空文本TTS", &mockEngine{name: "test-tts"}, "", []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.engine.TTS(context.Background(), tt.text)
			if err != nil {
				t.Errorf("TTS() error = %v", err)
			}
			if string(result) != string(tt.expected) {
				t.Errorf("TTS() = %s, want %s", result, tt.expected)
			}
			if tt.engine.Name() == "" {
				t.Error("Name() should not be empty")
			}
		})
	}
}

// TestASREngineInterface 测试ASR引擎接口
func TestASREngineInterface(t *testing.T) {
	tests := []struct {
		name     string
		engine   ASREngine
		audio    []byte
		expected string
	}{
		{"模拟ASR引擎", &mockEngine{name: "test-asr"}, []byte("音频数据"), "音频数据"},
		{"空音频ASR", &mockEngine{name: "test-asr"}, []byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.engine.ASR(context.Background(), tt.audio)
			if err != nil {
				t.Errorf("ASR() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("ASR() = %s, want %s", result, tt.expected)
			}
			if tt.engine.Name() == "" {
				t.Error("Name() should not be empty")
			}
		})
	}
}

// TestVoiceEngineInterface 测试完整语音引擎接口
func TestVoiceEngineInterface(t *testing.T) {
	tests := []struct {
		name       string
		engine     VoiceEngine
		ttsText    string
		asrAudio   []byte
		engineName string
	}{
		{
			name:       "完整语音引擎",
			engine:     &mockEngine{name: "full-engine"},
			ttsText:    "测试文本",
			asrAudio:   []byte("测试音频"),
			engineName: "full-engine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audio, err := tt.engine.TTS(context.Background(), tt.ttsText)
			if err != nil {
				t.Errorf("TTS() error = %v", err)
			}
			if string(audio) != tt.ttsText {
				t.Errorf("TTS() = %s, want %s", audio, tt.ttsText)
			}

			text, err := tt.engine.ASR(context.Background(), tt.asrAudio)
			if err != nil {
				t.Errorf("ASR() error = %v", err)
			}
			if text != string(tt.asrAudio) {
				t.Errorf("ASR() = %s, want %s", text, tt.asrAudio)
			}

			if tt.engine.Name() != tt.engineName {
				t.Errorf("Name() = %s, want %s", tt.engine.Name(), tt.engineName)
			}
		})
	}
}

// TestConfigStruct 测试配置结构体
func TestConfigStruct(t *testing.T) {
	cfg := Config{
		EngineType:   EngineOpenAI,
		APIKey:       "test-key",
		SecretKey:    "secret",
		Region:       "eastus",
		APIBase:      "https://api.test.com",
		VoiceID:      "voice-1",
		Language:     "zh-CN",
		Model:        "tts-1",
		SampleRate:   44100,
		OutputFormat: "wav",
		Proxy:        "http://proxy:8080",
		Timeout:      60,
	}

	if cfg.EngineType != EngineOpenAI {
		t.Errorf("EngineType = %s, want openai", cfg.EngineType)
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %s, want test-key", cfg.APIKey)
	}
	if cfg.SampleRate != 44100 {
		t.Errorf("SampleRate = %d, want 44100", cfg.SampleRate)
	}
}

// TestAudioFormat 测试音频格式
func TestAudioFormat(t *testing.T) {
	tests := []struct {
		name   string
		format AudioFormat
		value  string
	}{
		{"MP3格式", FormatMP3, "mp3"},
		{"WAV格式", FormatWAV, "wav"},
		{"PCM格式", FormatPCM, "pcm"},
		{"OGG格式", FormatOGG, "ogg"},
		{"AMR格式", FormatAMR, "amr"},
		{"SILK格式", FormatSILK, "silk"},
		{"M4A格式", FormatM4A, "m4a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.format) != tt.value {
				t.Errorf("AudioFormat = %s, want %s", tt.format, tt.value)
			}
		})
	}
}

// TestVoiceError 测试语音错误
func TestVoiceError(t *testing.T) {
	err := NewVoiceError("test-engine", "tts", context.Canceled)

	if err.Engine != "test-engine" {
		t.Errorf("Engine = %s, want test-engine", err.Engine)
	}
	if err.Operation != "tts" {
		t.Errorf("Operation = %s, want tts", err.Operation)
	}
	if err.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

// TestResultStruct 测试结果结构体
func TestResultStruct(t *testing.T) {
	result := Result{
		Text:       "测试文本",
		Audio:      []byte("音频数据"),
		Format:     FormatMP3,
		Duration:   1000,
		SampleRate: 44100,
	}

	if result.Text != "测试文本" {
		t.Errorf("Text = %s, want 测试文本", result.Text)
	}
	if string(result.Audio) != "音频数据" {
		t.Errorf("Audio = %s, want 音频数据", result.Audio)
	}
	if result.Format != FormatMP3 {
		t.Errorf("Format = %s, want mp3", result.Format)
	}
}
