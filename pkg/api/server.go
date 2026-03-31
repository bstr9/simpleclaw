// Package api 提供独立的 RESTful API 服务
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/bridge"
	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	"go.uber.org/zap"
)

// 错误消息常量
const (
	errMsgInvalidJSON     = "Invalid JSON: "
	errMsgMessageRequired = "message is required"
)

type Server struct {
	config      *Config
	httpServer  *http.Server
	mux         *http.ServeMux
	sseQueues   map[string]chan SSEEvent
	sseQueuesMu sync.RWMutex
	rateLimiter *common.TokenBucket
}

type Config struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	APIKey     string `json:"api_key"`
	EnableCORS bool   `json:"enable_cors"`
	RateLimit  int    `json:"rate_limit"`
}

func DefaultConfig() *Config {
	return &Config{
		Host:       "0.0.0.0",
		Port:       8080,
		EnableCORS: true,
		RateLimit:  60,
	}
}

func NewServer(cfg *Config) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	s := &Server{
		config:    cfg,
		mux:       http.NewServeMux(),
		sseQueues: make(map[string]chan SSEEvent),
	}

	if cfg.RateLimit > 0 {
		s.rateLimiter = common.NewTokenBucket(cfg.RateLimit, time.Minute)
	}

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/v1/chat", s.withMiddleware(s.handleChat))
	s.mux.HandleFunc("/v1/chat/stream", s.withMiddleware(s.handleChatStream))
	s.mux.HandleFunc("/v1/session/", s.withMiddleware(s.handleSession))
	s.mux.HandleFunc("/v1/sessions", s.withMiddleware(s.handleSessions))
	s.mux.HandleFunc("/v1/models", s.withMiddleware(s.handleModels))
	s.mux.HandleFunc("/v1/health", s.handleHealth)
	s.mux.HandleFunc("/v1/info", s.withMiddleware(s.handleInfo))
}

func (s *Server) withMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.config.EnableCORS {
			s.setCORSHeaders(w)
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		if s.config.APIKey != "" && !s.validateAPIKey(w, r) {
			return
		}

		if s.rateLimiter != nil && !s.checkRateLimit(w) {
			return
		}

		handler(w, r)
	}
}

func (s *Server) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func (s *Server) validateAPIKey(w http.ResponseWriter, r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		writeError(w, http.StatusUnauthorized, "Authorization header required")
		return false
	}
	if !strings.HasPrefix(auth, "Bearer ") {
		writeError(w, http.StatusUnauthorized, "Invalid authorization format")
		return false
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if token != s.config.APIKey {
		writeError(w, http.StatusUnauthorized, "Invalid API key")
		return false
	}
	return true
}

func (s *Server) checkRateLimit(w http.ResponseWriter) bool {
	if !s.rateLimiter.TryGetToken() {
		writeError(w, http.StatusTooManyRequests, "Rate limit exceeded")
		return false
	}
	return true
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("[API] Starting API server", zap.String("addr", addr))
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, common.ErrMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errMsgInvalidJSON+err.Error())
		return
	}

	if req.Message == "" {
		writeError(w, http.StatusBadRequest, errMsgMessageRequired)
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("api_%d", time.Now().UnixNano())
	}

	ab := bridge.GetBridge().GetAgentBridge()
	if ab == nil {
		writeError(w, http.StatusInternalServerError, common.ErrAgentBridgeNotAvailable)
		return
	}

	msgCtx := types.NewContext(types.ContextText, req.Message)
	msgCtx.Set("session_id", sessionID)

	reply, err := ab.AgentReply(r.Context(), req.Message, msgCtx, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get response: "+err.Error())
		return
	}

	response := ChatResponse{
		SessionID: sessionID,
		Message:   reply.StringContent(),
		Type:      reply.Type.String(),
		CreatedAt: time.Now().Unix(),
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, common.ErrMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errMsgInvalidJSON+err.Error())
		return
	}

	if req.Message == "" {
		writeError(w, http.StatusBadRequest, errMsgMessageRequired)
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("api_%d", time.Now().UnixNano())
	}

	flusher, ok := s.setupSSEResponse(w)
	if !ok {
		return
	}

	fmt.Fprintf(w, "event: connected\ndata: {\"session_id\":\"%s\"}\n\n", sessionID)
	flusher.Flush()

	ab := bridge.GetBridge().GetAgentBridge()
	if ab == nil {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"%s\"}\n\n", common.ErrAgentBridgeNotAvailable)
		flusher.Flush()
		return
	}

	eventChan := make(chan SSEEvent, 100)
	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	s.setSSEQueue(requestID, eventChan)
	defer s.removeSSEQueue(requestID)

	s.startAgentStream(ab, req.Message, sessionID, eventChan)
	s.processSSEEvents(w, r, eventChan, flusher)
}

// setupSSEResponse 设置 SSE 响应头并返回 flusher
func (s *Server) setupSSEResponse(w http.ResponseWriter) (http.Flusher, bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return nil, false
	}
	return flusher, true
}

// startAgentStream 启动 Agent 流处理协程
func (s *Server) startAgentStream(ab *bridge.AgentBridge, message, sessionID string, eventChan chan<- SSEEvent) {
	go func() {
		msgCtx := types.NewContext(types.ContextText, message)
		msgCtx.Set("session_id", sessionID)

		onEvent := func(event map[string]any) {
			eventType, _ := event["type"].(string)
			content, _ := event["content"].(string)
			eventChan <- SSEEvent{
				Type:    eventType,
				Content: content,
				Data:    event,
			}
		}

		_, err := ab.AgentReply(context.Background(), message, msgCtx, onEvent)
		if err != nil {
			eventChan <- SSEEvent{Type: "error", Content: err.Error()}
		}
		eventChan <- SSEEvent{Type: "done"}
	}()
}

// processSSEEvents 处理 SSE 事件循环
func (s *Server) processSSEEvents(w http.ResponseWriter, r *http.Request, eventChan <-chan SSEEvent, flusher http.Flusher) {
	timeout := time.NewTimer(5 * time.Minute)
	defer timeout.Stop()

	for {
		select {
		case event := <-eventChan:
			data, _ := json.Marshal(event.Data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()

			if event.Type == "done" || event.Type == "error" {
				return
			}
		case <-timeout.C:
			fmt.Fprintf(w, "event: timeout\ndata: {\"error\":\"request timeout\"}\n\n")
			flusher.Flush()
			return
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/session/")
	parts := strings.SplitN(path, "/", 2)
	sessionID := parts[0]

	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getSessionInfo(w, r, sessionID)
	case http.MethodDelete:
		s.clearSession(w, r, sessionID)
	default:
		writeError(w, http.StatusMethodNotAllowed, common.ErrMethodNotAllowed)
	}
}

func (s *Server) getSessionInfo(w http.ResponseWriter, r *http.Request, sessionID string) {
	ab := bridge.GetBridge().GetAgentBridge()
	if ab == nil {
		writeError(w, http.StatusInternalServerError, common.ErrAgentBridgeNotAvailable)
		return
	}

	ag, err := ab.GetAgent(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	messages := ag.GetMessages()
	history := make([]map[string]string, len(messages))
	for i, msg := range messages {
		history[i] = map[string]string{
			"role":    string(msg.Role),
			"content": msg.Content,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":    sessionID,
		"message_count": len(messages),
		"history":       history,
	})
}

func (s *Server) clearSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	ab := bridge.GetBridge().GetAgentBridge()
	if ab == nil {
		writeError(w, http.StatusInternalServerError, common.ErrAgentBridgeNotAvailable)
		return
	}

	ab.ClearSession(sessionID)

	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"session_id": sessionID,
	})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, common.ErrMethodNotAllowed)
		return
	}

	ab := bridge.GetBridge().GetAgentBridge()
	if ab == nil {
		writeError(w, http.StatusInternalServerError, common.ErrAgentBridgeNotAvailable)
		return
	}

	count := ab.SessionCount()
	writeJSON(w, http.StatusOK, map[string]any{
		"count": count,
	})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, common.ErrMethodNotAllowed)
		return
	}

	models := common.ModelList
	writeJSON(w, http.StatusOK, map[string]any{
		"models": models,
		"count":  len(models),
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"version": "1.0.0",
		"model":   cfg.Model,
		"agent":   cfg.Agent,
	})
}

type SSEEvent struct {
	Type    string
	Content string
	Data    map[string]any
}

func (s *Server) setSSEQueue(requestID string, ch chan SSEEvent) {
	s.sseQueuesMu.Lock()
	s.sseQueues[requestID] = ch
	s.sseQueuesMu.Unlock()
}

func (s *Server) removeSSEQueue(requestID string) {
	s.sseQueuesMu.Lock()
	delete(s.sseQueues, requestID)
	s.sseQueuesMu.Unlock()
}

type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
	Model     string `json:"model,omitempty"`
	Stream    bool   `json:"stream,omitempty"`
}

type ChatResponse struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	Type      string `json:"type"`
	CreatedAt int64  `json:"created_at"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
		"code":  status,
	})
}
