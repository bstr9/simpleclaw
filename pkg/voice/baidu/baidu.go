// Package baidu 提供百度语音引擎实现
// 支持文本转语音(TTS)和语音转文本(ASR)功能
// 使用百度AI开放平台语音服务
package baidu

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/voice"
)

const (
	// 百度OAuth Token URL
	tokenURL = "https://aip.baidubce.com/oauth/2.0/token"
	// 百度TTS短文本API URL
	ttsURL = "https://tsn.baidu.com/text2audio"
	// 百度ASR API URL
	asrURL = "https://vop.baidu.com/server_api"
	// 百度长文本TTS API URL
	longTTSURL = "https://aip.baidubce.com/rpc/2.0/tts/v1/create"
	// 百度长文本TTS查询URL
	longTTSQueryURL = "https://aip.baidubce.com/rpc/2.0/tts/v1/query"
	// 引擎名称
	engineName = "baidu"
	// 客户端ID
	clientID = "simpleclaw"
	// 操作类型
	opTTS = "tts"
	opASR = "asr"
)

// BaiduEngine 百度语音引擎实现
type BaiduEngine struct {
	// config 配置
	config voice.Config
	// appID 百度应用ID
	appID string
	// apiKey API Key
	apiKey string
	// secretKey Secret Key
	secretKey string
	// devPID ASR识别模型ID
	devPID int
	// lang 语言(中文:zh, 英文:en)
	lang string
	// ctp 客户端类型(1:web端)
	ctp int
	// spd 语速(0-15)
	spd int
	// pit 音调(0-15)
	pit int
	// vol 音量(0-15)
	vol int
	// per 发音人选择(0:普通女声, 1:普通男声, 3:情感合成-男声, 4:情感合成-女声)
	per int
	// httpClient HTTP客户端
	httpClient *http.Client
	// access token缓存
	accessToken   string
	tokenExpireAt time.Time
	tokenMutex    sync.RWMutex
}

// 默认配置
const (
	defaultLang   = "zh"
	defaultCTP    = 1
	defaultSpd    = 5
	defaultPit    = 5
	defaultVol    = 5
	defaultPer    = 0
	defaultDevPID = 1537 // 普通话(支持简单的英文识别)
)

// New 创建百度语音引擎实例
func New(cfg voice.Config) (voice.VoiceEngine, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("百度 API Key不能为空")
	}
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("百度 Secret Key不能为空")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30
	}

	// 解析配置参数
	params := parseExtraParams(cfg.Extra)
	appID := parseAppID(cfg.Extra)

	return &BaiduEngine{
		config:     cfg,
		appID:      appID,
		apiKey:     cfg.APIKey,
		secretKey:  cfg.SecretKey,
		devPID:     params.devPID,
		lang:       params.lang,
		ctp:        params.ctp,
		spd:        params.spd,
		pit:        params.pit,
		vol:        params.vol,
		per:        params.per,
		httpClient: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

// ttsParams TTS参数配置
type ttsParams struct {
	lang   string
	ctp    int
	spd    int
	pit    int
	vol    int
	per    int
	devPID int
}

// parseExtraParams 解析Extra配置参数
func parseExtraParams(extra map[string]any) ttsParams {
	params := ttsParams{
		lang:   defaultLang,
		ctp:    defaultCTP,
		spd:    defaultSpd,
		pit:    defaultPit,
		vol:    defaultVol,
		per:    defaultPer,
		devPID: defaultDevPID,
	}

	if extra == nil {
		return params
	}

	if v, ok := extra["lang"].(string); ok {
		params.lang = v
	}
	if v, ok := extra["ctp"].(int); ok {
		params.ctp = v
	}
	if v, ok := extra["spd"].(int); ok {
		params.spd = v
	}
	if v, ok := extra["pit"].(int); ok {
		params.pit = v
	}
	if v, ok := extra["vol"].(int); ok {
		params.vol = v
	}
	if v, ok := extra["per"].(int); ok {
		params.per = v
	}
	if v, ok := extra["dev_pid"].(int); ok {
		params.devPID = v
	}

	return params
}

// parseAppID 解析AppID
func parseAppID(extra map[string]any) string {
	if extra == nil {
		return ""
	}
	if v, ok := extra["app_id"].(string); ok {
		return v
	}
	return ""
}

// Name 返回引擎名称
func (e *BaiduEngine) Name() string {
	return engineName
}

// TTS 文本转语音
func (e *BaiduEngine) TTS(ctx context.Context, text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("文本不能为空")
	}

	// 计算GBK编码长度判断是短文本还是长文本
	gbkLen := len(text) // 简化处理，实际应计算GBK编码长度
	if gbkLen <= 1024 {
		return e.shortTextToSpeech(ctx, text)
	}
	return e.longTextToSpeech(ctx, text)
}

// shortTextToSpeech 短文本TTS
func (e *BaiduEngine) shortTextToSpeech(ctx context.Context, text string) ([]byte, error) {
	// 获取access token
	token, err := e.getAccessToken(ctx)
	if err != nil {
		return nil, voice.NewVoiceError("baidu", "tts", err)
	}

	// 构建请求参数
	data := url.Values{}
	data.Set("tex", text)
	data.Set("tok", token)
	data.Set("cuid", clientID)
	data.Set("ctp", fmt.Sprintf("%d", e.ctp))
	data.Set("lan", e.lang)
	data.Set("spd", fmt.Sprintf("%d", e.spd))
	data.Set("pit", fmt.Sprintf("%d", e.pit))
	data.Set("vol", fmt.Sprintf("%d", e.vol))
	data.Set("per", fmt.Sprintf("%d", e.per))
	data.Set("aue", "3") // mp3格式

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", ttsURL,
		strings.NewReader(data.Encode()))
	if err != nil {
		return nil, voice.NewVoiceError(engineName, opTTS, err)
	}
	req.Header.Set(common.HeaderContentType, "application/x-www-form-urlencoded")

	// 发送请求
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, voice.NewVoiceError(engineName, opTTS, err)
	}
	defer resp.Body.Close()

	// 读取响应
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, voice.NewVoiceError(engineName, opTTS, err)
	}

	// 检查是否为错误响应(JSON格式)
	contentType := resp.Header.Get(common.HeaderContentType)
	if strings.Contains(contentType, common.ContentTypeJSON) {
		var errResp ErrorResponse
		if err := json.Unmarshal(audioData, &errResp); err == nil {
			return nil, voice.NewVoiceError(engineName, opTTS,
				fmt.Errorf("TTS失败: %s", errResp.ErrMsg))
		}
	}

	return audioData, nil
}

// longTextToSpeech 长文本TTS
func (e *BaiduEngine) longTextToSpeech(ctx context.Context, text string) ([]byte, error) {
	// 获取access token
	token, err := e.getAccessToken(ctx)
	if err != nil {
		return nil, voice.NewVoiceError(engineName, opTTS, err)
	}

	// 创建合成任务
	taskID, err := e.createLongTTSTask(ctx, token, text)
	if err != nil {
		return nil, err
	}

	// 轮询查询任务状态并获取音频
	return e.pollLongTTSTask(ctx, token, taskID)
}

// createLongTTSTask 创建长文本TTS任务
func (e *BaiduEngine) createLongTTSTask(ctx context.Context, token, text string) (int64, error) {
	createURL := fmt.Sprintf("%s?access_token=%s", longTTSURL, token)
	payload := map[string]any{
		"text":            text,
		"format":          "mp3-16k",
		"voice":           e.per,
		"lang":            e.lang,
		"speed":           e.spd,
		"pitch":           e.pit,
		"volume":          e.vol,
		"enable_subtitle": 0,
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", createURL,
		strings.NewReader(string(payloadBytes)))
	if err != nil {
		return 0, voice.NewVoiceError(engineName, opTTS, err)
	}
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0, voice.NewVoiceError(engineName, opTTS, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var createResp LongTTSCreateResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return 0, voice.NewVoiceError(engineName, opTTS, err)
	}

	if createResp.TaskID == 0 {
		return 0, voice.NewVoiceError(engineName, opTTS,
			fmt.Errorf("创建长文本合成任务失败: %s", string(body)))
	}

	return createResp.TaskID, nil
}

// pollLongTTSTask 轮询长文本TTS任务状态
func (e *BaiduEngine) pollLongTTSTask(ctx context.Context, token string, taskID int64) ([]byte, error) {
	queryURL := fmt.Sprintf("%s?access_token=%s", longTTSQueryURL, token)

	for i := 0; i < 100; i++ {
		select {
		case <-ctx.Done():
			return nil, voice.NewVoiceError(engineName, opTTS, ctx.Err())
		case <-time.After(3 * time.Second):
		}

		taskInfo, err := e.queryLongTTSTask(ctx, queryURL, taskID)
		if err != nil {
			continue
		}

		switch taskInfo.TaskStatus {
		case "Success":
			return e.downloadLongTTSAudio(taskInfo.TaskResult)
		case "Running":
			continue
		default:
			return nil, voice.NewVoiceError(engineName, opTTS,
				fmt.Errorf("长文本合成失败: %s", taskInfo.TaskStatus))
		}
	}

	return nil, voice.NewVoiceError(engineName, opTTS, fmt.Errorf("长文本合成超时"))
}

// queryLongTTSTask 查询长文本TTS任务状态
func (e *BaiduEngine) queryLongTTSTask(ctx context.Context, queryURL string, taskID int64) (*LongTTSTaskInfo, error) {
	queryPayload := map[string]any{
		"task_ids": []int64{taskID},
	}
	queryBytes, _ := json.Marshal(queryPayload)

	req, err := http.NewRequestWithContext(ctx, "POST", queryURL,
		strings.NewReader(string(queryBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var queryResp LongTTSQueryResponse
	if err := json.Unmarshal(body, &queryResp); err != nil {
		return nil, err
	}

	if len(queryResp.TasksInfo) == 0 {
		return nil, fmt.Errorf("无任务信息")
	}

	return &queryResp.TasksInfo[0], nil
}

// downloadLongTTSAudio 下载长文本TTS音频
func (e *BaiduEngine) downloadLongTTSAudio(result LongTTSTaskResult) ([]byte, error) {
	audioURL := result.SpeechURL
	if audioURL == "" {
		audioURL = result.AudioAddress
	}
	if audioURL == "" {
		return nil, voice.NewVoiceError(engineName, opTTS, fmt.Errorf("未获取到音频下载地址"))
	}

	audioResp, err := e.httpClient.Get(audioURL)
	if err != nil {
		return nil, voice.NewVoiceError(engineName, opTTS, err)
	}
	defer audioResp.Body.Close()

	audioData, err := io.ReadAll(audioResp.Body)
	if err != nil {
		return nil, voice.NewVoiceError(engineName, opTTS, err)
	}

	return audioData, nil
}

// ASR 语音转文本
func (e *BaiduEngine) ASR(ctx context.Context, audio []byte) (string, error) {
	if len(audio) == 0 {
		return "", fmt.Errorf("音频数据不能为空")
	}

	// 获取access token
	token, err := e.getAccessToken(ctx)
	if err != nil {
		return "", voice.NewVoiceError(engineName, opASR, err)
	}

	// 构建请求URL
	asrReqURL := fmt.Sprintf("%s?cuid=%s&token=%s&dev_pid=%d",
		asrURL, clientID, token, e.devPID)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", asrReqURL,
		strings.NewReader(base64.StdEncoding.EncodeToString(audio)))
	if err != nil {
		return "", voice.NewVoiceError(engineName, opASR, err)
	}

	// 设置请求头
	req.Header.Set(common.HeaderContentType, "audio/wav; rate=16000")

	// 发送请求
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", voice.NewVoiceError(engineName, opASR, err)
	}
	defer resp.Body.Close()

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", voice.NewVoiceError(engineName, opASR, err)
	}

	var asrResp ASRResponse
	if err := json.Unmarshal(body, &asrResp); err != nil {
		return "", voice.NewVoiceError(engineName, opASR, err)
	}

	if asrResp.ErrNo != 0 {
		return "", voice.NewVoiceError(engineName, opASR,
			fmt.Errorf("ASR失败: %s", asrResp.ErrMsg))
	}

	// 拼接识别结果
	return strings.Join(asrResp.Result, ""), nil
}

// getAccessToken 获取百度API访问令牌(带缓存)
func (e *BaiduEngine) getAccessToken(ctx context.Context) (string, error) {
	// 检查缓存
	e.tokenMutex.RLock()
	if e.accessToken != "" && time.Now().Before(e.tokenExpireAt) {
		token := e.accessToken
		e.tokenMutex.RUnlock()
		return token, nil
	}
	e.tokenMutex.RUnlock()

	// 获取新token
	e.tokenMutex.Lock()
	defer e.tokenMutex.Unlock()

	// 双重检查
	if e.accessToken != "" && time.Now().Before(e.tokenExpireAt) {
		return e.accessToken, nil
	}

	// 构建请求
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", e.apiKey)
	data.Set("client_secret", e.secretKey)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL,
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set(common.HeaderContentType, "application/x-www-form-urlencoded")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("获取token失败: %s - %s", tokenResp.Error, tokenResp.ErrorDescription)
	}

	// 缓存token(提前1分钟过期)
	e.accessToken = tokenResp.AccessToken
	e.tokenExpireAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return e.accessToken, nil
}

// TokenResponse Token响应结构
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	Scope            string `json:"scope"`
	SessionKey       string `json:"session_key"`
	SessionSecret    string `json:"session_secret"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	ErrNo  int    `json:"err_no"`
	ErrMsg string `json:"err_msg"`
}

// ASRResponse ASR响应结构
type ASRResponse struct {
	ErrNo  int      `json:"err_no"`
	ErrMsg string   `json:"err_msg"`
	Result []string `json:"result"`
	SN     int      `json:"sn"`
}

// LongTTSCreateResponse 长文本TTS创建响应
type LongTTSCreateResponse struct {
	TaskID int64  `json:"task_id"`
	ErrNo  int    `json:"err_no"`
	ErrMsg string `json:"err_msg"`
}

// LongTTSQueryResponse 长文本TTS查询响应
type LongTTSQueryResponse struct {
	TasksInfo []LongTTSTaskInfo `json:"tasks_info"`
}

// LongTTSTaskInfo 长文本TTS任务信息
type LongTTSTaskInfo struct {
	TaskID     int               `json:"task_id"`
	TaskStatus string            `json:"task_status"`
	TaskResult LongTTSTaskResult `json:"task_result"`
}

// LongTTSTaskResult 长文本TTS任务结果
type LongTTSTaskResult struct {
	SpeechURL    string `json:"speech_url"`
	AudioAddress string `json:"audio_address"`
}

// init 注册到工厂
func init() {
	voice.RegisterEngine(voice.EngineBaidu, New)
}
