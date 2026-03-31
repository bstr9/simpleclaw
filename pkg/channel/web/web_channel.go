// Package web 提供 Web 渠道实现，支持 HTTP API 和 SSE 流式响应。
package web

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	"go.uber.org/zap"
)

const (
	defaultPort         = 9899
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 300 * time.Second // SSE 需要较长超时
	defaultIdleTimeout  = 120 * time.Second
	keepaliveInterval   = 15 * time.Second
	sseTimeout          = 5 * time.Minute
)

// SSEEvent 定义 SSE 事件结构
type SSEEvent struct {
	Type          string         `json:"type"`
	Content       string         `json:"content,omitempty"`
	Tool          string         `json:"tool,omitempty"`
	Arguments     map[string]any `json:"arguments,omitempty"`
	Status        string         `json:"status,omitempty"`
	Result        string         `json:"result,omitempty"`
	ExecutionTime float64        `json:"execution_time,omitempty"`
	RequestID     string         `json:"request_id,omitempty"`
	Timestamp     int64          `json:"timestamp,omitempty"`
}

// MessageRequest 定义消息请求结构
type MessageRequest struct {
	SessionID   string       `json:"session_id"`
	Message     string       `json:"message"`
	Stream      bool         `json:"stream"`
	Attachments []Attachment `json:"attachments"`
}

// Attachment 定义附件结构
type Attachment struct {
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
}

// UploadResponse 定义上传响应结构
type UploadResponse struct {
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	FilePath   string `json:"file_path,omitempty"`
	FileName   string `json:"file_name,omitempty"`
	FileType   string `json:"file_type,omitempty"`
	PreviewURL string `json:"preview_url,omitempty"`
}

// Config 定义 Web 渠道配置
type Config struct {
	Port         int    `json:"port"`
	Host         string `json:"host"`
	UploadDir    string `json:"upload_dir"`
	StaticDir    string `json:"static_dir"`
	AllowOrigins string `json:"allow_origins"`
}

// WebChannel Web 渠道实现
type WebChannel struct {
	*channel.BaseChannel

	config *Config
	server *http.Server
	mux    *http.ServeMux

	// SSE 管理
	sseQueues map[string]chan SSEEvent
	sseMu     sync.RWMutex

	// 会话管理
	sessionQueues map[string]chan *ResponseData
	sessionMu     sync.RWMutex

	// 请求映射
	requestToSession map[string]string
	requestMu        sync.RWMutex

	// 消息ID计数器
	msgIDCounter int64
	msgIDMu      sync.Mutex

	// 启动停止控制
	startupOnce sync.Once
	stopOnce    sync.Once

	// 配置访问器（用于动态获取配置）
	configProvider ConfigProvider

	// 消息处理器
	messageHandler MessageHandler

	// agentBridge Agent桥接器引用
	agentBridge AgentBridgeInterface
}

// AgentBridgeInterface Agent桥接器接口
type AgentBridgeInterface interface {
	HasVoiceEngine() bool
	TextToSpeech(ctx context.Context, text string) ([]byte, error)
	SpeechToText(ctx context.Context, audio []byte) (string, error)
	ListVoiceEngines() []string
	HasTranslator() bool
	Translate(text, from, to string) (string, error)
	ListTranslators() []string
	GetMemoryManager() any
	AddMemory(ctx context.Context, content, userID string, scope any) error
	SearchMemory(ctx context.Context, query string, limit int) (any, error)
	GetMemoryStats(ctx context.Context) map[string]any
	ListPlugins() map[string]any
}

// ConfigProvider 配置提供者接口
type ConfigProvider interface {
	GetString(key string, defaultValue string) string
	GetInt(key string, defaultValue int) int
	GetBool(key string, defaultValue bool) bool
	GetStringSlice(key string, defaultValue []string) []string
}

// MessageHandler 消息处理器接口
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *WebMessage) (*types.Reply, error)
}

// ResponseData 响应数据结构
type ResponseData struct {
	Type      string  `json:"type"`
	Content   string  `json:"content"`
	RequestID string  `json:"request_id"`
	Timestamp float64 `json:"timestamp"`
}

// WebMessage Web 消息结构
type WebMessage struct {
	types.BaseMessage
	SessionID   string       `json:"session_id"`
	RequestID   string       `json:"request_id"`
	Attachments []Attachment `json:"attachments"`
	Stream      bool         `json:"stream"`
	OnEvent     func(event map[string]any)
}

// GetOnEvent 返回事件回调函数
func (m *WebMessage) GetOnEvent() func(event map[string]any) {
	return m.OnEvent
}

// NewWebChannel 创建 Web 渠道实例
func NewWebChannel(cfg *Config) *WebChannel {
	if cfg == nil {
		cfg = &Config{
			Port:         defaultPort,
			Host:         "0.0.0.0",
			AllowOrigins: "*",
		}
	}
	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}

	return &WebChannel{
		BaseChannel:      channel.NewBaseChannel("web"),
		config:           cfg,
		sseQueues:        make(map[string]chan SSEEvent),
		sessionQueues:    make(map[string]chan *ResponseData),
		requestToSession: make(map[string]string),
	}
}

// SetConfigProvider 设置配置提供者
func (w *WebChannel) SetConfigProvider(provider ConfigProvider) {
	w.configProvider = provider
}

// SetMessageHandler 设置消息处理器
func (w *WebChannel) SetMessageHandler(handler any) {
	if h, ok := handler.(MessageHandler); ok {
		w.messageHandler = h
	}
}

// SetAgentBridge 设置 Agent 桥接器
func (w *WebChannel) SetAgentBridge(bridge AgentBridgeInterface) {
	w.agentBridge = bridge
}

// Startup 启动 Web 服务
func (w *WebChannel) Startup(ctx context.Context) error {
	var startErr error
	w.startupOnce.Do(func() {
		// 设置不支持的回复类型
		w.SetNotSupportTypes([]types.ReplyType{types.ReplyVoice})

		// 创建路由
		w.mux = http.NewServeMux()
		w.registerRoutes()

		// 创建 HTTP 服务器
		addr := fmt.Sprintf("%s:%d", w.config.Host, w.config.Port)
		w.server = &http.Server{
			Addr:         addr,
			Handler:      w.corsMiddleware(w.loggingMiddleware(w.mux)),
			ReadTimeout:  defaultReadTimeout,
			WriteTimeout: defaultWriteTimeout,
			IdleTimeout:  defaultIdleTimeout,
		}

		// 打印启动信息
		w.logStartupInfo()

		// 在后台启动服务器
		go func() {
			if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("HTTP server error", zap.Error(err))
				w.ReportStartupError(err)
			}
		}()

		w.ReportStartupSuccess()
		logger.Info("Web channel started", zap.String("addr", addr))
	})

	return startErr
}

// Stop 停止 Web 服务
func (w *WebChannel) Stop() error {
	var err error
	w.stopOnce.Do(func() {
		if w.server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err = w.server.Shutdown(ctx); err != nil {
				logger.Error("Error shutting down HTTP server", zap.Error(err))
			} else {
				logger.Info("HTTP server stopped gracefully")
			}
		}

		// 清理 SSE 队列
		w.sseMu.Lock()
		for _, ch := range w.sseQueues {
			close(ch)
		}
		w.sseQueues = make(map[string]chan SSEEvent)
		w.sseMu.Unlock()

		// 清理会话队列
		w.sessionMu.Lock()
		for _, ch := range w.sessionQueues {
			close(ch)
		}
		w.sessionQueues = make(map[string]chan *ResponseData)
		w.sessionMu.Unlock()

		w.SetStarted(false)
	})

	return err
}

// Send 发送消息（实现 Channel 接口）
func (w *WebChannel) Send(reply *types.Reply, ctx *types.Context) error {
	// 检查是否支持该回复类型
	if !w.IsReplyTypeSupported(reply.Type) {
		logger.Warn("Reply type not supported by web channel",
			zap.String("type", reply.Type.String()))
		return nil
	}

	// 获取请求ID
	requestIDI, ok := ctx.Get("request_id")
	if !ok {
		logger.Error("No request_id found in context")
		return fmt.Errorf("no request_id in context")
	}
	requestID, ok := requestIDI.(string)
	if !ok {
		logger.Error("request_id is not a string")
		return fmt.Errorf("request_id is not a string")
	}

	// 获取会话ID
	sessionID := w.getRequestSession(requestID)
	if sessionID == "" {
		logger.Error("No session_id found for request", zap.String("request_id", requestID))
		return fmt.Errorf("no session_id for request")
	}

	content := ""
	if reply.Content != nil {
		content = fmt.Sprintf("%v", reply.Content)
	}

	// 检查是否为 SSE 模式
	if w.hasSSEQueue(requestID) {
		event := SSEEvent{
			Type:      "done",
			Content:   content,
			RequestID: requestID,
			Timestamp: time.Now().Unix(),
		}
		w.pushSSEEvent(requestID, event)
		return nil
	}

	// 轮询模式
	if w.hasSessionQueue(sessionID) {
		response := &ResponseData{
			Type:      reply.Type.String(),
			Content:   content,
			RequestID: requestID,
			Timestamp: float64(time.Now().Unix()),
		}
		w.pushSessionResponse(sessionID, response)
	}

	return nil
}

// generateMsgID 生成消息ID
func (w *WebChannel) generateMsgID() string {
	w.msgIDMu.Lock()
	defer w.msgIDMu.Unlock()
	w.msgIDCounter++
	return fmt.Sprintf("%d%d", time.Now().Unix(), w.msgIDCounter)
}

// generateRequestID 生成请求ID
func (w *WebChannel) generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

// SSE 队列管理方法

func (w *WebChannel) createSSEQueue(requestID string) chan SSEEvent {
	w.sseMu.Lock()
	defer w.sseMu.Unlock()
	ch := make(chan SSEEvent, 100)
	w.sseQueues[requestID] = ch
	return ch
}

func (w *WebChannel) getSSEQueue(requestID string) (chan SSEEvent, bool) {
	w.sseMu.RLock()
	defer w.sseMu.RUnlock()
	ch, ok := w.sseQueues[requestID]
	return ch, ok
}

func (w *WebChannel) hasSSEQueue(requestID string) bool {
	w.sseMu.RLock()
	defer w.sseMu.RUnlock()
	_, ok := w.sseQueues[requestID]
	return ok
}

func (w *WebChannel) pushSSEEvent(requestID string, event SSEEvent) {
	w.sseMu.RLock()
	ch, ok := w.sseQueues[requestID]
	w.sseMu.RUnlock()

	if ok {
		select {
		case ch <- event:
		default:
			logger.Warn("SSE queue full, dropping event", zap.String("request_id", requestID))
		}
	}
}

func (w *WebChannel) removeSSEQueue(requestID string) {
	w.sseMu.Lock()
	defer w.sseMu.Unlock()
	if ch, ok := w.sseQueues[requestID]; ok {
		close(ch)
		delete(w.sseQueues, requestID)
	}
}

// 会话队列管理方法

func (w *WebChannel) createSessionQueue(sessionID string) chan *ResponseData {
	w.sessionMu.Lock()
	defer w.sessionMu.Unlock()
	ch := make(chan *ResponseData, 100)
	w.sessionQueues[sessionID] = ch
	return ch
}

func (w *WebChannel) hasSessionQueue(sessionID string) bool {
	w.sessionMu.RLock()
	defer w.sessionMu.RUnlock()
	_, ok := w.sessionQueues[sessionID]
	return ok
}

func (w *WebChannel) pushSessionResponse(sessionID string, response *ResponseData) {
	w.sessionMu.RLock()
	ch, ok := w.sessionQueues[sessionID]
	w.sessionMu.RUnlock()

	if ok {
		select {
		case ch <- response:
		default:
			logger.Warn("Session queue full, dropping response", zap.String("session_id", sessionID))
		}
	}
}

// 请求-会话映射方法

func (w *WebChannel) mapRequestToSession(requestID, sessionID string) {
	w.requestMu.Lock()
	defer w.requestMu.Unlock()
	w.requestToSession[requestID] = sessionID
}

func (w *WebChannel) getRequestSession(requestID string) string {
	w.requestMu.RLock()
	defer w.requestMu.RUnlock()
	return w.requestToSession[requestID]
}

func (w *WebChannel) removeRequestMapping(requestID string) {
	w.requestMu.Lock()
	defer w.requestMu.Unlock()
	delete(w.requestToSession, requestID)
}

// corsMiddleware CORS 中间件
func (w *WebChannel) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		w.setCORSHeader(rw, origin)
		w.setCORSMethods(rw)

		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(rw, r)
	})
}

// setCORSHeader 设置 CORS 头
func (w *WebChannel) setCORSHeader(rw http.ResponseWriter, origin string) {
	allowOrigins := w.config.AllowOrigins
	if allowOrigins == "" || allowOrigins == "*" {
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		return
	}

	if isOriginAllowed(origin, allowOrigins) {
		rw.Header().Set("Access-Control-Allow-Origin", origin)
	}
}

// isOriginAllowed 检查 origin 是否允许
func isOriginAllowed(origin, allowOrigins string) bool {
	for _, o := range splitOrigins(allowOrigins) {
		if o == origin {
			return true
		}
	}
	return false
}

// setCORSMethods 设置 CORS 方法和头
func (w *WebChannel) setCORSMethods(rw http.ResponseWriter) {
	rw.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	rw.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	rw.Header().Set("Access-Control-Max-Age", "86400")
}

func (w *WebChannel) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 创建响应记录器
		lrw := &responseWriter{ResponseWriter: rw, statusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		logger.Debug("HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", lrw.statusCode),
			zap.Duration("duration", duration),
			zap.String("remote", r.RemoteAddr),
		)
	})
}

// responseWriter 用于记录响应状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush 实现 http.Flusher 接口，支持 SSE 流式响应
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// 工具函数

func splitOrigins(origins string) []string {
	if origins == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(origins); i++ {
		if origins[i] == ',' {
			if i > start {
				result = append(result, origins[start:i])
			}
			start = i + 1
		}
	}
	if start < len(origins) {
		result = append(result, origins[start:])
	}
	return result
}

func (w *WebChannel) logStartupInfo() {
	port := w.config.Port
	logger.Info("[WebChannel] 全部可用通道如下，可修改配置文件中的 channel_type 字段进行切换：")
	logger.Info("[WebChannel]   1. weixin           - 微信")
	logger.Info("[WebChannel]   2. web              - 网页")
	logger.Info("[WebChannel]   3. terminal         - 终端")
	logger.Info("[WebChannel]   4. feishu           - 飞书")
	logger.Info("[WebChannel]   5. dingtalk         - 钉钉")
	logger.Info("[WebChannel]   6. wecom_bot        - 企微智能机器人")
	logger.Info("[WebChannel]   7. wechatcom_app    - 企微自建应用")
	logger.Info("[WebChannel]   8. wechatmp         - 个人公众号")
	logger.Info("[WebChannel] ✅ Web控制台已运行")
	logger.Info(fmt.Sprintf("[WebChannel] 🌐 本地访问: http://localhost:%d", port))
	logger.Info(fmt.Sprintf("[WebChannel] 🌍 服务器访问: http://YOUR_IP:%d (请将YOUR_IP替换为服务器IP)", port))
}

// getUploadDir 获取上传目录
func (w *WebChannel) getUploadDir() string {
	if w.config.UploadDir != "" {
		return w.config.UploadDir
	}

	// 默认使用工作目录下的 tmp 目录
	var workspace string
	if w.configProvider != nil {
		workspace = w.configProvider.GetString("agent_workspace", "~/cow")
	} else {
		workspace = "~/cow"
	}

	// 展开路径
	if len(workspace) > 0 && workspace[0] == '~' {
		home, _ := os.UserHomeDir()
		workspace = filepath.Join(home, workspace[1:])
	}

	tmpDir := filepath.Join(workspace, "tmp")
	os.MkdirAll(tmpDir, 0755)
	return tmpDir
}

// writeJSON 写入 JSON 响应
func writeJSON(rw http.ResponseWriter, status int, data any) {
	rw.Header().Set(common.HeaderContentType, "application/json; charset=utf-8")
	rw.WriteHeader(status)
	json.NewEncoder(rw).Encode(data)
}

// writeError 写入错误响应
func writeError(rw http.ResponseWriter, status int, message string) {
	writeJSON(rw, status, map[string]any{
		"status":  "error",
		"message": message,
	})
}

// writeSuccess 写入成功响应
func writeSuccess(rw http.ResponseWriter, data any) {
	dataMap, ok := data.(map[string]any)
	if !ok {
		dataMap = make(map[string]any)
	}
	dataMap["status"] = "success"
	writeJSON(rw, http.StatusOK, dataMap)
}
