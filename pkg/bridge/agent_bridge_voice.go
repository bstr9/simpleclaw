// Package bridge 提供消息处理的核心路由层
// agent_bridge_voice.go 语音引擎相关方法
package bridge

import (
	"context"

	"github.com/bstr9/simpleclaw/pkg/voice"
)

// GetVoiceEngine 获取语音引擎实例
func (ab *AgentBridge) GetVoiceEngine() voice.VoiceEngine {
	return ab.voiceEngine
}

// HasVoiceEngine 检查是否配置了语音引擎
func (ab *AgentBridge) HasVoiceEngine() bool {
	return ab.voiceEngine != nil
}

// TextToSpeech 文本转语音
func (ab *AgentBridge) TextToSpeech(ctx context.Context, text string) ([]byte, error) {
	if ab.voiceEngine == nil {
		return nil, nil
	}
	return ab.voiceEngine.TTS(ctx, text)
}

// SpeechToText 语音转文本
func (ab *AgentBridge) SpeechToText(ctx context.Context, audio []byte) (string, error) {
	if ab.voiceEngine == nil {
		return "", nil
	}
	return ab.voiceEngine.ASR(ctx, audio)
}

// ListVoiceEngines 列出所有已注册的语音引擎
func (ab *AgentBridge) ListVoiceEngines() []voice.EngineType {
	return voice.ListEngines()
}
