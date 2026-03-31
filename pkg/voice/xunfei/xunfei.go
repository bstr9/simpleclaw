// Package xunfei 提供讯飞语音引擎实现
// 支持 TTS (文本转语音) 和 ASR (语音转文本) 功能
// 使用讯飞开放平台 WebSocket API
package xunfei

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/bstr9/simpleclaw/pkg/voice"
)

// Engine 讯飞语音引擎
// 实现 voice.VoiceEngine 接口，支持 TTS 和 ASR 功能
type Engine struct {
	config voice.Config
}

// 默认配置常量
const (
	// DefaultTTSURL 默认 TTS WebSocket 地址
	DefaultTTSURL = "wss://tts-api.xfyun.cn/v2/tts"
	// DefaultASRURL 默认 ASR WebSocket 地址
	DefaultASRURL = "wss://ws-api.xfyun.cn/v2/iat"
	// DefaultHost 默认主机地址
	DefaultHost = "ws-api.xfyun.cn"
	// DefaultVoice 默认发音人
	DefaultVoice = "xiaoyan"
	// DefaultLanguage 默认语言
	DefaultLanguage = "zh_cn"
	// DefaultTTSAue 默认 TTS 音频编码 (lame = mp3)
	DefaultTTSAue = "lame"
	// DefaultASRDomain 默认 ASR 领域
	DefaultASRDomain = "iat"
)

// 帧状态常量
const (
	StatusFirstFrame    = 0 // 第一帧
	StatusContinueFrame = 1 // 中间帧
	StatusLastFrame     = 2 // 最后一帧
)

// TTS 请求/响应结构

// ttsCommonRequest TTS 公共参数
type ttsCommonRequest struct {
	AppID string `json:"app_id"`
}

// ttsBusinessRequest TTS 业务参数
type ttsBusinessRequest struct {
	Aue    string `json:"aue"`              // 音频编码: lame(mp3), raw(pcm), speex
	Sfl    int    `json:"sfl,omitempty"`    // 开启流式返回
	Auf    string `json:"auf,omitempty"`    // 音频采样率: audio/L16;rate=16000
	Vcn    string `json:"vcn"`              // 发音人
	Tte    string `json:"tte"`              // 文本编码: utf8
	Speed  int    `json:"speed,omitempty"`  // 语速
	Volume int    `json:"volume,omitempty"` // 音量
	Pitch  int    `json:"pitch,omitempty"`  // 音高
}

// ttsDataRequest TTS 数据参数
type ttsDataRequest struct {
	Status int    `json:"status"` // 帧状态
	Text   string `json:"text"`   // Base64 编码的文本
}

// ttsRequest TTS 完整请求
type ttsRequest struct {
	Common   ttsCommonRequest   `json:"common"`
	Business ttsBusinessRequest `json:"business"`
	Data     ttsDataRequest     `json:"data"`
}

// ttsResponse TTS 响应结构
type ttsResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Sid     string `json:"sid"`
	Data    struct {
		Audio  string `json:"audio"`  // Base64 编码的音频
		Status int    `json:"status"` // 状态: 2 表示结束
		Ced    int    `json:"ced"`    // 已合成进度
	} `json:"data"`
}

// ASR 请求/响应结构

// asrCommonRequest ASR 公共参数
type asrCommonRequest struct {
	AppID string `json:"app_id"`
}

// asrBusinessRequest ASR 业务参数
type asrBusinessRequest struct {
	Domain   string `json:"domain"`            // 领域: iat
	Language string `json:"language"`          // 语言: zh_cn, en_us
	Accent   string `json:"accent,omitempty"`  // 方言: mandarin
	VadEos   int    `json:"vad_eos,omitempty"` // 静音检测时长
	Dwa      string `json:"dwa,omitempty"`     // 动态修正: wpgs
	Pte      string `json:"pte,omitempty"`     // 垂直领域
}

// asrDataRequest ASR 数据参数
type asrDataRequest struct {
	Status   int    `json:"status"`   // 帧状态
	Format   string `json:"format"`   // 音频格式: audio/L16;rate=16000
	Encoding string `json:"encoding"` // 编码: raw
	Audio    string `json:"audio"`    // Base64 编码的音频
}

// asrRequest ASR 完整请求
type asrRequest struct {
	Common   asrCommonRequest   `json:"common"`
	Business asrBusinessRequest `json:"business"`
	Data     asrDataRequest     `json:"data"`
}

// asrResponse ASR 响应结构
type asrResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Sid     string `json:"sid"`
	Data    struct {
		Result struct {
			Sn int   `json:"sn"`           // 句子序号
			Ls bool  `json:"ls"`           // 是否最后一句
			Rg []int `json:"rg,omitempty"` // 替换范围
			Ws []struct {
				Bg int `json:"bg"` // 起始位置
				Cw []struct {
					W  string `json:"w"`            // 词
					Wp string `json:"wp,omitempty"` // 词性
				} `json:"cw"`
			} `json:"ws"`
		} `json:"result"`
		Status int `json:"status"`
	} `json:"data"`
}

// New 创建讯飞语音引擎实例
// cfg: 语音引擎配置
// 需要配置: APIKey (AppID), SecretKey, APIKey (API Key)
func New(cfg voice.Config) (*Engine, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("讯飞 AppID 不能为空")
	}
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("讯飞 API Secret 不能为空")
	}
	// SecretKey 在 Extra 中存储 APISecret
	if _, ok := cfg.Extra["api_secret"]; !ok && cfg.APIKey != "" {
		// 如果 Extra 中没有 api_secret，尝试从其他配置获取
	}

	return &Engine{
		config: cfg,
	}, nil
}

// Name 返回引擎名称
func (e *Engine) Name() string {
	return "xunfei"
}

// TTS 将文本转换为语音
// ctx: 上下文
// text: 要转换的文本
// 返回音频数据（默认 MP3 格式）
func (e *Engine) TTS(ctx context.Context, text string) ([]byte, error) {
	appID, apiKey, apiSecret := e.getTTSConfig()
	req := e.buildTTSRequest(appID, text)

	wsURL, err := e.buildAuthURL(DefaultTTSURL, apiKey, apiSecret)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("生成鉴权URL失败: %w", err))
	}

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("连接WebSocket失败: %w", err))
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	reqJSON, _ := json.Marshal(req)
	if err := conn.Write(ctx, websocket.MessageText, reqJSON); err != nil {
		return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("发送请求失败: %w", err))
	}

	return e.receiveTTSAudio(ctx, conn)
}

// getTTSConfig 获取 TTS 配置参数
func (e *Engine) getTTSConfig() (appID, apiKey, apiSecret string) {
	return e.getConfig()
}

// getConfig 获取通用配置参数
func (e *Engine) getConfig() (appID, apiKey, apiSecret string) {
	appID = e.config.APIKey
	apiKey = e.config.SecretKey
	apiSecret = getExtraString(e.config.Extra, "api_secret")
	if apiSecret == "" {
		apiSecret = apiKey
	}
	return
}

// buildTTSRequest 构建 TTS 请求
func (e *Engine) buildTTSRequest(appID, text string) ttsRequest {
	vcn := e.config.VoiceID
	if vcn == "" {
		vcn = DefaultVoice
	}

	aue := parseAudioEncoding(e.config.OutputFormat)
	business := ttsBusinessRequest{
		Aue: aue,
		Sfl: 1,
		Vcn: vcn,
		Tte: "utf8",
	}

	business.Auf = buildSampleRate(e.config.SampleRate)
	e.applyBusinessParams(&business)

	return ttsRequest{
		Common:   ttsCommonRequest{AppID: appID},
		Business: business,
		Data: ttsDataRequest{
			Status: StatusLastFrame,
			Text:   base64.StdEncoding.EncodeToString([]byte(text)),
		},
	}
}

// parseAudioEncoding 解析音频编码格式
func parseAudioEncoding(format string) string {
	switch strings.ToLower(format) {
	case "mp3":
		return "lame"
	case "pcm", "wav":
		return "raw"
	case "speex":
		return "speex"
	default:
		return DefaultTTSAue
	}
}

// buildSampleRate 构建采样率字符串
func buildSampleRate(sampleRate int) string {
	if sampleRate > 0 {
		return fmt.Sprintf("audio/L16;rate=%d", sampleRate)
	}
	return "audio/L16;rate=16000"
}

// applyBusinessParams 应用业务参数
func (e *Engine) applyBusinessParams(business *ttsBusinessRequest) {
	if speed, ok := e.config.Extra["speed"].(int); ok {
		business.Speed = speed
	}
	if volume, ok := e.config.Extra["volume"].(int); ok {
		business.Volume = volume
	}
	if pitch, ok := e.config.Extra["pitch"].(int); ok {
		business.Pitch = pitch
	}
}

// receiveTTSAudio 接收 TTS 音频数据
func (e *Engine) receiveTTSAudio(ctx context.Context, conn *websocket.Conn) ([]byte, error) {
	var audioData []byte
	var done bool
	var err error
	for {
		var msgType websocket.MessageType
		var msg []byte
		msgType, msg, err = conn.Read(ctx)
		if err != nil {
			return nil, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("读取响应失败: %w", err))
		}

		if msgType != websocket.MessageText {
			continue
		}

		audioData, done, err = e.processTTSMessage(msg, audioData)
		if err != nil {
			return nil, err
		}
		if done {
			break
		}
	}
	return audioData, nil
}

// processTTSMessage 处理单条 TTS 消息
func (e *Engine) processTTSMessage(msg []byte, audioData []byte) ([]byte, bool, error) {
	var resp ttsResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		return audioData, false, nil
	}

	if resp.Code != 0 {
		return nil, false, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("API错误 [%d]: %s", resp.Code, resp.Message))
	}

	if resp.Data.Audio != "" {
		audio, err := base64.StdEncoding.DecodeString(resp.Data.Audio)
		if err != nil {
			return nil, false, voice.NewVoiceError(e.Name(), "tts", fmt.Errorf("解码音频失败: %w", err))
		}
		audioData = append(audioData, audio...)
	}

	return audioData, resp.Data.Status == 2, nil
}

// ASR 将语音转换为文本
// ctx: 上下文
// audio: 音频数据
// 返回识别出的文本
func (e *Engine) ASR(ctx context.Context, audio []byte) (string, error) {
	appID, apiKey, apiSecret := e.getASRConfig()
	business := e.buildASRBusiness()
	sampleRate := e.getASRSampleRate()

	wsURL, err := e.buildAuthURL(DefaultASRURL, apiKey, apiSecret)
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("生成鉴权URL失败: %w", err))
	}

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("连接WebSocket失败: %w", err))
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	resultMap, recvErr := e.startASRReceiver(ctx, conn, appID, business, sampleRate, audio)

	if recvErr != nil {
		return "", voice.NewVoiceError(e.Name(), "asr", recvErr)
	}

	return e.mergeASRResults(resultMap)
}

// getASRConfig 获取 ASR 配置参数
func (e *Engine) getASRConfig() (appID, apiKey, apiSecret string) {
	return e.getConfig()
}

// buildASRBusiness 构建 ASR 业务参数
func (e *Engine) buildASRBusiness() asrBusinessRequest {
	language := DefaultLanguage
	if e.config.Language != "" {
		language = e.config.Language
	}
	return asrBusinessRequest{
		Domain:   DefaultASRDomain,
		Language: language,
		Accent:   "mandarin",
		VadEos:   6000,
		Dwa:      "wpgs",
	}
}

// getASRSampleRate 获取 ASR 采样率
func (e *Engine) getASRSampleRate() int {
	if e.config.SampleRate == 0 {
		return 16000
	}
	return e.config.SampleRate
}

// startASRReceiver 启动 ASR 接收器并发送音频帧
func (e *Engine) startASRReceiver(ctx context.Context, conn *websocket.Conn, appID string, business asrBusinessRequest, sampleRate int, audio []byte) (*sync.Map, error) {
	var resultMap sync.Map
	var resultMu sync.Mutex
	var recvErr error

	done := make(chan struct{})

	go e.runASRReceiver(ctx, conn, &resultMap, &resultMu, &recvErr, done)
	e.sendASRFrames(ctx, conn, appID, business, sampleRate, audio)
	<-done

	return &resultMap, recvErr
}

// runASRReceiver 运行 ASR 接收循环
func (e *Engine) runASRReceiver(ctx context.Context, conn *websocket.Conn, resultMap *sync.Map, resultMu *sync.Mutex, recvErr *error, done chan struct{}) {
	defer close(done)
	for {
		msgType, msg, err := conn.Read(ctx)
		if err != nil {
			*recvErr = err
			return
		}

		if msgType != websocket.MessageText {
			continue
		}

		var resp asrResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}

		if resp.Code != 0 {
			*recvErr = fmt.Errorf("API错误 [%d]: %s", resp.Code, resp.Message)
			return
		}

		e.processASRResult(&resp, resultMap, resultMu)

		if resp.Data.Status == 2 {
			return
		}
	}
}

// processASRResult 处理 ASR 识别结果
func (e *Engine) processASRResult(resp *asrResponse, resultMap *sync.Map, resultMu *sync.Mutex) {
	if resp.Data.Result.Ws == nil {
		return
	}
	sn := resp.Data.Result.Sn
	text := extractText(resp.Data.Result)

	if len(resp.Data.Result.Rg) >= 2 {
		resultMu.Lock()
		for i := resp.Data.Result.Rg[0]; i <= resp.Data.Result.Rg[1]; i++ {
			resultMap.Delete(i)
		}
		resultMu.Unlock()
	}

	resultMap.Store(sn, text)
}

// sendASRFrames 分帧发送音频数据
func (e *Engine) sendASRFrames(ctx context.Context, conn *websocket.Conn, appID string, business asrBusinessRequest, sampleRate int, audio []byte) {
	const frameSize = 1280
	offset := 0
	status := StatusFirstFrame

	for offset < len(audio) {
		end := offset + frameSize
		if end > len(audio) {
			end = len(audio)
			status = StatusLastFrame
		}

		frame := audio[offset:end]
		req := e.buildASRFrameRequest(appID, business, status, sampleRate, frame)

		if status == StatusFirstFrame {
			status = StatusContinueFrame
		}

		reqJSON, _ := json.Marshal(req)
		conn.Write(ctx, websocket.MessageText, reqJSON)

		offset = end
		time.Sleep(40 * time.Millisecond)
	}
}

// buildASRFrameRequest 构建 ASR 帧请求
func (e *Engine) buildASRFrameRequest(appID string, business asrBusinessRequest, status, sampleRate int, frame []byte) asrRequest {
	req := asrRequest{
		Common: asrCommonRequest{AppID: appID},
		Data: asrDataRequest{
			Status:   status,
			Format:   fmt.Sprintf("audio/L16;rate=%d", sampleRate),
			Encoding: "raw",
			Audio:    base64.StdEncoding.EncodeToString(frame),
		},
	}
	if status == StatusFirstFrame {
		req.Business = business
	}
	return req
}

// mergeASRResults 合并 ASR 识别结果
func (e *Engine) mergeASRResults(resultMap *sync.Map) (string, error) {
	var keys []int
	resultMap.Range(func(key, value interface{}) bool {
		k := key.(int)
		keys = append(keys, k)
		return true
	})
	sort.Ints(keys)

	var result strings.Builder
	for _, k := range keys {
		if v, ok := resultMap.Load(k); ok {
			result.WriteString(v.(string))
		}
	}

	if result.Len() == 0 {
		return "", voice.NewVoiceError(e.Name(), "asr", fmt.Errorf("未能识别出文本"))
	}

	return result.String(), nil
}

// buildAuthURL 生成带鉴权参数的 WebSocket URL
func (e *Engine) buildAuthURL(baseURL, apiKey, apiSecret string) (string, error) {
	// 生成 RFC1123 格式的时间戳
	now := time.Now().UTC()
	date := now.Format("Mon, 02 Jan 2006 15:04:05 GMT")

	// 拼接签名原文
	signatureOrigin := fmt.Sprintf("host: %s\ndate: %s\nGET /v2/%s HTTP/1.1",
		DefaultHost, date, getEndpoint(baseURL))

	// HMAC-SHA256 签名
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(signatureOrigin))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 构建 authorization
	authorizationOrigin := fmt.Sprintf(`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`,
		apiKey, signature)
	authorization := base64.StdEncoding.EncodeToString([]byte(authorizationOrigin))

	// 构建最终 URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	query := u.Query()
	query.Set("authorization", authorization)
	query.Set("date", date)
	query.Set("host", DefaultHost)
	u.RawQuery = query.Encode()

	return u.String(), nil
}

// getEndpoint 从 URL 中提取端点
func getEndpoint(baseURL string) string {
	if strings.Contains(baseURL, "/v2/tts") {
		return "tts"
	}
	if strings.Contains(baseURL, "/v2/iat") {
		return "iat"
	}
	return ""
}

// extractText 从 ASR 结果中提取文本
func extractText(result struct {
	Sn int   `json:"sn"`
	Ls bool  `json:"ls"`
	Rg []int `json:"rg,omitempty"`
	Ws []struct {
		Bg int `json:"bg"`
		Cw []struct {
			W  string `json:"w"`
			Wp string `json:"wp,omitempty"`
		} `json:"cw"`
	} `json:"ws"`
}) string {
	var text strings.Builder
	for _, ws := range result.Ws {
		for _, cw := range ws.Cw {
			text.WriteString(cw.W)
		}
	}
	return text.String()
}

// getExtraString 从 Extra 配置中获取字符串值
func getExtraString(extra map[string]any, key string) string {
	if extra == nil {
		return ""
	}
	if v, ok := extra[key].(string); ok {
		return v
	}
	return ""
}

// init 注册引擎到工厂
func init() {
	voice.RegisterEngine(voice.EngineXunfei, func(cfg voice.Config) (voice.VoiceEngine, error) {
		return New(cfg)
	})
}
