// Package edge 提供 Microsoft Edge TTS 语音引擎实现
// 基于 Microsoft Edge 浏览器的在线 TTS 服务
package edge

import (
	"bytes"
	"context"
	"fmt"

	edgetts "github.com/difyz9/edge-tts-go/pkg/communicate"

	"github.com/bstr9/simpleclaw/pkg/voice"
)

// Engine Edge TTS 语音引擎
// 实现 voice.VoiceEngine 接口，仅支持 TTS 功能
type Engine struct {
	config voice.Config
}

// 默认配置
const (
	// DefaultVoice 默认中文语音
	DefaultVoice = "zh-CN-YunxiNeural"
	// DefaultRate 默认语速
	DefaultRate = "+0%"
	// DefaultVolume 默认音量
	DefaultVolume = "+0%"
	// DefaultPitch 默认音调
	DefaultPitch = "+0Hz"
)

// 中文可用语音列表
var ChineseVoices = []string{
	"zh-CN-XiaoxiaoNeural",         // 晓晓 - 普通话女声
	"zh-CN-XiaoyiNeural",           // 晓伊 - 普通话女声
	"zh-CN-YunjianNeural",          // 云健 - 普通话男声
	"zh-CN-YunxiNeural",            // 云希 - 普通话男声
	"zh-CN-YunxiaNeural",           // 云夏 - 普通话男声
	"zh-CN-YunyangNeural",          // 云扬 - 普通话男声
	"zh-CN-liaoning-XiaobeiNeural", // 晓北 - 辽宁口音
	"zh-CN-shaanxi-XiaoniNeural",   // 晓妮 - 陕西口音
	"zh-HK-HiuGaaiNeural",          // 粤语女声
	"zh-HK-HiuMaanNeural",          // 粤语女声
	"zh-HK-WanLungNeural",          // 粤语男声
	"zh-TW-HsiaoChenNeural",        // 台湾女声
	"zh-TW-HsiaoYuNeural",          // 台湾女声
	"zh-TW-YunJheNeural",           // 台湾男声
}

// New 创建 Edge TTS 语音引擎实例
func New(cfg voice.Config) (*Engine, error) {
	return &Engine{
		config: cfg,
	}, nil
}

// Name 返回引擎名称
func (e *Engine) Name() string {
	return "edge"
}

// TTS 将文本转换为语音
// ctx: 上下文
// text: 要转换的文本
// 返回 MP3 格式的语音数据
func (e *Engine) TTS(ctx context.Context, text string) ([]byte, error) {
	voiceID := e.config.VoiceID
	if voiceID == "" {
		voiceID = DefaultVoice
	}

	// 从配置获取语速、音量、音调参数
	rate := getExtraString(e.config.Extra, "rate", DefaultRate)
	volume := getExtraString(e.config.Extra, "volume", DefaultVolume)
	pitch := getExtraString(e.config.Extra, "pitch", DefaultPitch)

	// 创建 TTS 客户端
	comm, err := edgetts.NewCommunicate(
		text,
		voiceID,
		rate,
		volume,
		pitch,
		e.config.Proxy,
		10, // connectTimeout
		60, // receiveTimeout
	)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("创建TTS客户端失败: %w", err))
	}

	// 使用流式 API 收集音频数据
	var audioBuffer bytes.Buffer
	chunkChan, errChan := comm.Stream(ctx)

	for chunk := range chunkChan {
		if chunk.Type == "audio" {
			audioBuffer.Write(chunk.Data)
		}
	}

	if err := <-errChan; err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", err)
	}

	return audioBuffer.Bytes(), nil
}

// ASR Edge TTS 不支持语音识别
func (e *Engine) ASR(ctx context.Context, audio []byte) (string, error) {
	return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("edge TTS 不支持语音识别功能"))
}

// getExtraString 从额外配置中获取字符串值
func getExtraString(extra map[string]any, key, defaultValue string) string {
	if extra == nil {
		return defaultValue
	}
	if val, ok := extra[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// init 注册引擎到工厂
func init() {
	voice.RegisterEngine(voice.EngineEdge, func(cfg voice.Config) (voice.VoiceEngine, error) {
		return New(cfg)
	})
}
