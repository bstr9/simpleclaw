// Package voice 提供语音处理核心功能
// factory.go 提供语音引擎工厂方法
package voice

import (
	"fmt"
	"sync"
)

// 全局引擎注册表
var (
	enginesMu sync.RWMutex
	engines   = make(map[EngineType]func(Config) (VoiceEngine, error))
)

// RegisterEngine 注册语音引擎构造函数
// 允许第三方实现扩展
func RegisterEngine(engineType EngineType, constructor func(Config) (VoiceEngine, error)) {
	enginesMu.Lock()
	defer enginesMu.Unlock()
	engines[engineType] = constructor
}

// NewEngine 根据配置创建语音引擎实例
// 支持所有已注册的引擎类型
func NewEngine(cfg Config) (VoiceEngine, error) {
	if cfg.EngineType == "" {
		return nil, fmt.Errorf("语音引擎类型不能为空")
	}

	enginesMu.RLock()
	constructor, ok := engines[cfg.EngineType]
	enginesMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("未知的语音引擎类型: %s", cfg.EngineType)
	}

	// 应用默认值
	applyDefaults(&cfg)

	return constructor(cfg)
}

// applyDefaults 应用默认配置值
func applyDefaults(cfg *Config) {
	if cfg.Language == "" {
		cfg.Language = "zh-CN"
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 16000
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = "mp3"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30
	}
}

// ListEngines 列出所有已注册的引擎类型
func ListEngines() []EngineType {
	enginesMu.RLock()
	defer enginesMu.RUnlock()

	types := make([]EngineType, 0, len(engines))
	for t := range engines {
		types = append(types, t)
	}
	return types
}
