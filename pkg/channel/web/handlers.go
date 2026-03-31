package web

import (
	"github.com/bstr9/simpleclaw/pkg/common"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	errMethodNotAllowed = "Method not allowed"
	errInvalidJSON      = "Invalid JSON: "
	errSessionIDReq     = "session_id required"
	errRequestIDReq     = "request_id required"
	errInvalidRequestID = "invalid request_id"
	errStreamingNotSupp = "Streaming not supported"
	headerContentType   = common.HeaderContentType
	statusNotConfigured = "not_configured"
	statusSuccess       = "success"
)

var (
	imageExtensions = map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".bmp": true, ".svg": true,
	}
	videoExtensions = map[string]bool{
		".mp4": true, ".webm": true, ".avi": true,
		".mov": true, ".mkv": true,
	}
)

func (w *WebChannel) registerRoutes() {
	w.mux.HandleFunc("/", w.handleRoot)
	w.mux.HandleFunc("/message", w.handleMessage)
	w.mux.HandleFunc("/stream", w.handleStream)
	w.mux.HandleFunc("/upload", w.handleUpload)
	w.mux.HandleFunc("/uploads/", w.handleUploads)
	w.mux.HandleFunc("/poll", w.handlePoll)
	w.mux.HandleFunc("/chat", w.handleChat)
	w.mux.HandleFunc("/config", w.handleConfig)
	w.mux.HandleFunc("/api/tools", w.handleTools)
	w.mux.HandleFunc("/api/skills", w.handleSkills)
	w.mux.HandleFunc("/api/channels", w.handleChannels)
	w.mux.HandleFunc("/api/channel/", w.handleChannelOps)
	w.mux.HandleFunc("/api/sessions", w.handleSessions)
	w.mux.HandleFunc("/api/session/", w.handleSessionOps)
	w.mux.HandleFunc("/api/history/", w.handleHistory)
	w.mux.HandleFunc("/api/plugins", w.handlePlugins)
	w.mux.HandleFunc("/api/agent", w.handleAgentInfo)
	w.mux.HandleFunc("/api/voice", w.handleVoiceInfo)
	w.mux.HandleFunc("/api/tts", w.handleTTS)
	w.mux.HandleFunc("/api/stt", w.handleSTT)
	w.mux.HandleFunc("/api/models", w.handleModels)
	w.mux.HandleFunc("/api/providers", w.handleProviders)
	w.mux.HandleFunc("/api/memory", w.handleMemoryInfo)
	w.mux.HandleFunc("/api/memory/search", w.handleMemorySearch)
	w.mux.HandleFunc("/api/translate", w.handleTranslate)
}

func (w *WebChannel) handleRoot(rw http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(rw, r)
		return
	}
	http.Redirect(rw, r, "/chat", http.StatusFound)
}

func (w *WebChannel) handleMessage(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(rw, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", time.Now().Unix())
	}

	message := req.Message
	if len(req.Attachments) > 0 {
		message = w.appendAttachments(message, req.Attachments)
	}

	requestID := w.generateRequestID()
	w.mapRequestToSession(requestID, sessionID)

	if !w.hasSessionQueue(sessionID) {
		w.createSessionQueue(sessionID)
	}

	if req.Stream {
		w.createSSEQueue(requestID)
	}

	msg := &WebMessage{
		BaseMessage: types.BaseMessage{
			MsgID:      w.generateMsgID(),
			FromUserID: sessionID,
			ToUserID:   "Chatgpt",
			Content:    message,
			CreateTime: time.Now(),
		},
		SessionID:   sessionID,
		RequestID:   requestID,
		Attachments: req.Attachments,
		Stream:      req.Stream,
		OnEvent:     w.makeSSECallback(requestID),
	}

	if w.messageHandler != nil {
		go func() {
			ctx := context.Background()
			_, err := w.messageHandler.HandleMessage(ctx, msg)
			if err != nil {
				logger.Error("Message handler error", zap.Error(err))
				if req.Stream {
					w.pushSSEEvent(requestID, SSEEvent{
						Type:    "error",
						Content: err.Error(),
					})
				}
			}
		}()
	}

	writeSuccess(rw, map[string]any{
		"request_id": requestID,
		"stream":     req.Stream,
	})
}

func (w *WebChannel) handleStream(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	requestID := r.URL.Query().Get("request_id")
	if requestID == "" {
		writeError(rw, http.StatusBadRequest, errRequestIDReq)
		return
	}

	queue, ok := w.getSSEQueue(requestID)
	if !ok {
		writeError(rw, http.StatusNotFound, errInvalidRequestID)
		return
	}

	rw.Header().Set(headerContentType, "text/event-stream; charset=utf-8")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")
	rw.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := rw.(http.Flusher)
	if !ok {
		writeError(rw, http.StatusInternalServerError, errStreamingNotSupp)
		return
	}

	fmt.Fprintf(rw, ": connected\n\n")
	flusher.Flush()

	deadline := time.Now().Add(sseTimeout)
	keepaliveTicker := time.NewTicker(keepaliveInterval)
	defer keepaliveTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			w.removeSSEQueue(requestID)
			w.removeRequestMapping(requestID)
			return

		case <-keepaliveTicker.C:
			if time.Now().After(deadline) {
				w.removeSSEQueue(requestID)
				w.removeRequestMapping(requestID)
				return
			}
			fmt.Fprintf(rw, ": keepalive\n\n")
			flusher.Flush()

		case event, ok := <-queue:
			if !ok {
				return
			}

			data, err := json.Marshal(event)
			if err != nil {
				logger.Error("Failed to marshal SSE event", zap.Error(err))
				continue
			}

			fmt.Fprintf(rw, "data: %s\n\n", data)
			flusher.Flush()

			if event.Type == "done" || event.Type == "error" {
				w.removeSSEQueue(requestID)
				w.removeRequestMapping(requestID)
				return
			}
		}
	}
}

func (w *WebChannel) handleUpload(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(rw, r.Body, 100<<20)

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		writeError(rw, http.StatusBadRequest, "Failed to parse multipart form: "+err.Error())
		return
	}

	sessionID := r.FormValue("session_id")

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(rw, http.StatusBadRequest, "No file uploaded: "+err.Error())
		return
	}
	defer file.Close()

	uploadDir := w.getUploadDir()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	safeName := fmt.Sprintf("web_%d%s", time.Now().UnixNano(), ext)
	savePath := filepath.Join(uploadDir, safeName)

	dst, err := os.Create(savePath)
	if err != nil {
		writeError(rw, http.StatusInternalServerError, "Failed to create file: "+err.Error())
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		writeError(rw, http.StatusInternalServerError, "Failed to save file: "+err.Error())
		return
	}

	fileType := "file"
	if imageExtensions[ext] {
		fileType = "image"
	} else if videoExtensions[ext] {
		fileType = "video"
	}

	previewURL := fmt.Sprintf("/uploads/%s", safeName)

	logger.Info("[WebChannel] File uploaded",
		zap.String("original", header.Filename),
		zap.String("saved", savePath),
		zap.String("type", fileType),
		zap.String("session", sessionID),
	)

	writeSuccess(rw, map[string]any{
		"file_path":   savePath,
		"file_name":   header.Filename,
		"file_type":   fileType,
		"preview_url": previewURL,
	})
}

func (w *WebChannel) handleUploads(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	filename := strings.TrimPrefix(r.URL.Path, "/uploads/")
	if filename == "" {
		writeError(rw, http.StatusBadRequest, "Filename required")
		return
	}

	uploadDir := w.getUploadDir()
	filePath := filepath.Join(uploadDir, filename)

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		writeError(rw, http.StatusBadRequest, "Invalid file path")
		return
	}

	absUploadDir, err := filepath.Abs(uploadDir)
	if err != nil {
		writeError(rw, http.StatusInternalServerError, "Server error")
		return
	}

	if !strings.HasPrefix(absPath, absUploadDir) {
		writeError(rw, http.StatusForbidden, "Access denied")
		return
	}

	rw.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(rw, r, filePath)
}

func (w *WebChannel) handlePoll(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(rw, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	if req.SessionID == "" {
		writeError(rw, http.StatusBadRequest, errSessionIDReq)
		return
	}

	w.sessionMu.RLock()
	queue, ok := w.sessionQueues[req.SessionID]
	w.sessionMu.RUnlock()

	if !ok {
		writeError(rw, http.StatusNotFound, "Invalid session ID")
		return
	}

	select {
	case response := <-queue:
		writeSuccess(rw, map[string]any{
			"has_content": true,
			"content":     response.Content,
			"type":        response.Type,
			"request_id":  response.RequestID,
			"timestamp":   response.Timestamp,
		})
	default:
		writeSuccess(rw, map[string]any{
			"has_content": false,
		})
	}
}

func (w *WebChannel) handleChat(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	if w.config.StaticDir != "" {
		chatPath := filepath.Join(w.config.StaticDir, "chat.html")
		if _, err := os.Stat(chatPath); err == nil {
			http.ServeFile(rw, r, chatPath)
			return
		}
	}

	rw.Header().Set(headerContentType, "text/html; charset=utf-8")
	rw.Write([]byte(defaultChatHTML))
}

func (w *WebChannel) handleConfig(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.getConfig(rw, r)
	case http.MethodPost:
		w.updateConfig(rw, r)
	case http.MethodPut:
		w.reloadConfig(rw, r)
	default:
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (w *WebChannel) getConfig(rw http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	useAgent := cfg.Agent

	title := "AI Assistant"
	if useAgent {
		title = "CowAgent"
	}

	writeSuccess(rw, map[string]any{
		"use_agent":                useAgent,
		"title":                    title,
		"model":                    cfg.Model,
		"bot_type":                 cfg.BotType,
		"port":                     w.config.Port,
		"agent_max_context_tokens": cfg.AgentMaxContextTokens,
		"agent_max_context_turns":  cfg.AgentMaxContextTurns,
		"agent_max_steps":          cfg.AgentMaxSteps,
		"debug":                    cfg.Debug,
	})
}

func (w *WebChannel) updateConfig(rw http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(rw, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	cfg := config.Get()
	if model, ok := req["model"].(string); ok && model != "" {
		cfg.Model = model
	}
	if debug, ok := req["debug"].(bool); ok {
		cfg.Debug = debug
		if debug {
			logger.SetLevel(zapcore.DebugLevel)
		} else {
			logger.SetLevel(zapcore.InfoLevel)
		}
	}
	if maxSteps, ok := req["agent_max_steps"].(float64); ok {
		cfg.AgentMaxSteps = int(maxSteps)
	}

	writeSuccess(rw, map[string]any{
		"status":  "updated",
		"message": "Config updated successfully",
	})
}

func (w *WebChannel) reloadConfig(rw http.ResponseWriter, r *http.Request) {
	if err := config.Reload(); err != nil {
		writeError(rw, http.StatusInternalServerError, "Failed to reload config: "+err.Error())
		return
	}

	cfg := config.Get()
	if cfg.Debug {
		logger.SetLevel(zapcore.DebugLevel)
	} else {
		logger.SetLevel(zapcore.InfoLevel)
	}

	writeSuccess(rw, map[string]any{
		"status":  "reloaded",
		"message": "Config reloaded successfully",
	})
}

func (w *WebChannel) handleTools(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	writeSuccess(rw, map[string]any{
		"tools": []any{},
	})
}

func (w *WebChannel) handleSkills(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	writeSuccess(rw, map[string]any{
		"skills": []any{},
	})
}

func (w *WebChannel) handleChannels(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.getChannels(rw, r)
	case http.MethodPost:
		w.manageChannel(rw, r)
	default:
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (w *WebChannel) getChannels(rw http.ResponseWriter, r *http.Request) {
	channels := []map[string]any{
		{
			"name":   "weixin",
			"label":  map[string]string{"zh": "微信", "en": "WeChat"},
			"active": false,
		},
		{
			"name":   "web",
			"label":  map[string]string{"zh": "网页", "en": "Web"},
			"active": true,
		},
		{
			"name":   "terminal",
			"label":  map[string]string{"zh": "终端", "en": "Terminal"},
			"active": false,
		},
		{
			"name":   "feishu",
			"label":  map[string]string{"zh": "飞书", "en": "Feishu"},
			"active": false,
		},
		{
			"name":   "dingtalk",
			"label":  map[string]string{"zh": "钉钉", "en": "DingTalk"},
			"active": false,
		},
	}

	writeSuccess(rw, map[string]any{
		"channels": channels,
	})
}

func (w *WebChannel) handleChannelOps(rw http.ResponseWriter, r *http.Request) {
	channelName := r.PathValue("channel_name")
	if channelName == "" {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/channel/"), "/")
		if len(parts) > 0 {
			channelName = parts[0]
		}
	}

	if channelName == "" {
		writeError(rw, http.StatusBadRequest, "channel_name required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.getChannelInfo(rw, channelName)
	case http.MethodPost:
		w.startChannel(rw, r, channelName)
	case http.MethodDelete:
		w.stopChannel(rw, channelName)
	default:
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (w *WebChannel) getChannelInfo(rw http.ResponseWriter, channelName string) {
	writeSuccess(rw, map[string]any{
		"name":   channelName,
		"status": "unknown",
		"active": channelName == "web",
	})
}

func (w *WebChannel) startChannel(rw http.ResponseWriter, r *http.Request, channelName string) {
	writeSuccess(rw, map[string]any{
		"name":    channelName,
		"status":  "started",
		"message": "Channel start requested",
	})
}

func (w *WebChannel) stopChannel(rw http.ResponseWriter, channelName string) {
	writeSuccess(rw, map[string]any{
		"name":    channelName,
		"status":  "stopped",
		"message": "Channel stop requested",
	})
}

func (w *WebChannel) manageChannel(rw http.ResponseWriter, r *http.Request) {
	var req struct {
		Action  string         `json:"action"`
		Channel string         `json:"channel"`
		Config  map[string]any `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(rw, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	if req.Action == "" || req.Channel == "" {
		writeError(rw, http.StatusBadRequest, "action and channel required")
		return
	}

	writeError(rw, http.StatusNotImplemented, "Channel management not implemented")
}

func sseEventFromMessageUpdate(data map[string]any) (SSEEvent, bool) {
	delta, _ := data["delta"].(string)
	if delta == "" {
		return SSEEvent{}, false
	}
	return SSEEvent{
		Type:    "delta",
		Content: delta,
	}, true
}

func sseEventFromToolStart(data map[string]any) SSEEvent {
	toolName, _ := data["tool_name"].(string)
	arguments, _ := data["arguments"].(map[string]any)
	return SSEEvent{
		Type:      "tool_start",
		Tool:      toolName,
		Arguments: arguments,
	}
}

func sseEventFromToolEnd(data map[string]any) SSEEvent {
	toolName, _ := data["tool_name"].(string)
	status, _ := data["status"].(string)
	result := data["result"]
	execTime, _ := data["execution_time"].(float64)

	resultStr := fmt.Sprintf("%v", result)
	const maxResultLen = 2000
	if len(resultStr) > maxResultLen {
		resultStr = resultStr[:maxResultLen] + "…"
	}

	return SSEEvent{
		Type:          "tool_end",
		Tool:          toolName,
		Status:        status,
		Result:        resultStr,
		ExecutionTime: execTime,
	}
}

func (w *WebChannel) makeSSECallback(requestID string) func(event map[string]any) {
	return func(event map[string]any) {
		if !w.hasSSEQueue(requestID) {
			return
		}

		eventType, _ := event["type"].(string)
		data, _ := event["data"].(map[string]any)

		var sseEvent SSEEvent
		var valid bool

		switch eventType {
		case "message_update":
			sseEvent, valid = sseEventFromMessageUpdate(data)
			if !valid {
				return
			}
		case "tool_execution_start":
			sseEvent = sseEventFromToolStart(data)
		case "tool_execution_end":
			sseEvent = sseEventFromToolEnd(data)
		default:
			return
		}

		w.pushSSEEvent(requestID, sseEvent)
	}
}

const defaultChatHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>AI Chat</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f5f5; }
        .container { max-width: 800px; margin: 0 auto; padding: 20px; height: 100vh; display: flex; flex-direction: column; }
        h1 { text-align: center; margin-bottom: 20px; color: #333; }
        .chat-container { flex: 1; overflow-y: auto; border: 1px solid #e0e0e0; border-radius: 8px; padding: 16px; background: #fff; }
        .message { margin: 12px 0; padding: 12px 16px; border-radius: 8px; max-width: 80%; word-wrap: break-word; }
        .message.user { background: #007bff; color: white; margin-left: auto; }
        .message.assistant { background: #f0f0f0; border: 1px solid #e0e0e0; }
        .input-container { display: flex; gap: 8px; margin-top: 16px; }
        #message-input { flex: 1; padding: 12px; border: 1px solid #e0e0e0; border-radius: 8px; font-size: 16px; outline: none; }
        #message-input:focus { border-color: #007bff; }
        #send-btn { padding: 12px 24px; background: #007bff; color: white; border: none; border-radius: 8px; cursor: pointer; font-size: 16px; }
        #send-btn:hover { background: #0056b3; }
        #send-btn:disabled { background: #ccc; cursor: not-allowed; }
    </style>
</head>
<body>
    <div class="container">
        <h1>AI Chat</h1>
        <div class="chat-container" id="chat-container"></div>
        <div class="input-container">
            <input type="text" id="message-input" placeholder="Type a message..." />
            <button id="send-btn">Send</button>
        </div>
    </div>
    <script>
        const chatContainer = document.getElementById('chat-container');
        const messageInput = document.getElementById('message-input');
        const sendBtn = document.getElementById('send-btn');
        let sessionId = 'session_' + Date.now();
        
        function addMessage(content, isUser) {
            const div = document.createElement('div');
            div.className = 'message ' + (isUser ? 'user' : 'assistant');
            div.textContent = content;
            chatContainer.appendChild(div);
            chatContainer.scrollTop = chatContainer.scrollHeight;
        }
        
        async function sendMessage() {
            const message = messageInput.value.trim();
            if (!message) return;
            
            addMessage(message, true);
            messageInput.value = '';
            sendBtn.disabled = true;
            
            try {
                const response = await fetch('/message', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ session_id: sessionId, message: message, stream: true })
                });
                
                const data = await response.json();
                if (data.request_id) {
                    const eventSource = new EventSource('/stream?request_id=' + data.request_id);
                    let assistantDiv = null;
                    
                    eventSource.onmessage = function(e) {
                        const event = JSON.parse(e.data);
                        if (event.type === 'delta') {
                            if (!assistantDiv) {
                                assistantDiv = document.createElement('div');
                                assistantDiv.className = 'message assistant';
                                chatContainer.appendChild(assistantDiv);
                            }
                            assistantDiv.textContent += event.content;
                            chatContainer.scrollTop = chatContainer.scrollHeight;
                        } else if (event.type === 'done') {
                            if (!assistantDiv) {
                                addMessage(event.content || '', false);
                            }
                            eventSource.close();
                            sendBtn.disabled = false;
                        } else if (event.type === 'error') {
                            addMessage('Error: ' + event.content, false);
                            eventSource.close();
                            sendBtn.disabled = false;
                        }
                    };
                    
                    eventSource.onerror = function() {
                        eventSource.close();
                        sendBtn.disabled = false;
                    };
                }
            } catch (err) {
                addMessage('Error: ' + err.message, false);
                sendBtn.disabled = false;
            }
        }
        
        sendBtn.addEventListener('click', sendMessage);
        messageInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') sendMessage();
        });
    </script>
</body>
</html>`

func (w *WebChannel) handleSessions(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	w.sessionMu.RLock()
	sessions := make([]string, 0, len(w.sessionQueues))
	for sid := range w.sessionQueues {
		sessions = append(sessions, sid)
	}
	w.sessionMu.RUnlock()

	writeSuccess(rw, map[string]any{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

func (w *WebChannel) handleSessionOps(rw http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	if sessionID == "" {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/session/"), "/")
		if len(parts) > 0 {
			sessionID = parts[0]
		}
	}

	if sessionID == "" {
		writeError(rw, http.StatusBadRequest, errSessionIDReq)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.getSessionInfo(rw, r, sessionID)
	case http.MethodDelete:
		w.deleteSession(rw, r, sessionID)
	default:
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (w *WebChannel) getSessionInfo(rw http.ResponseWriter, r *http.Request, sessionID string) {
	w.sessionMu.RLock()
	_, exists := w.sessionQueues[sessionID]
	w.sessionMu.RUnlock()

	writeSuccess(rw, map[string]any{
		"session_id": sessionID,
		"exists":     exists,
	})
}

func (w *WebChannel) deleteSession(rw http.ResponseWriter, r *http.Request, sessionID string) {
	w.sessionMu.Lock()
	delete(w.sessionQueues, sessionID)
	w.sessionMu.Unlock()

	w.requestMu.Lock()
	for reqID, sid := range w.requestToSession {
		if sid == sessionID {
			delete(w.requestToSession, reqID)
		}
	}
	w.requestMu.Unlock()

	writeSuccess(rw, map[string]any{
		"session_id": sessionID,
		"deleted":    true,
	})
}

func (w *WebChannel) handleHistory(rw http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	if sessionID == "" {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/history/"), "/")
		if len(parts) > 0 {
			sessionID = parts[0]
		}
	}

	if sessionID == "" {
		writeError(rw, http.StatusBadRequest, errSessionIDReq)
		return
	}

	if r.Method == http.MethodDelete {
		w.clearHistory(rw, sessionID)
		return
	}

	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	writeSuccess(rw, map[string]any{
		"session_id": sessionID,
		"messages":   []any{},
	})
}

func (w *WebChannel) clearHistory(rw http.ResponseWriter, sessionID string) {
	w.sessionMu.Lock()
	delete(w.sessionQueues, sessionID)
	w.sessionMu.Unlock()

	writeSuccess(rw, map[string]any{
		"session_id": sessionID,
		"cleared":    true,
	})
}

func (w *WebChannel) handlePlugins(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	plugins := []map[string]any{}
	if w.agentBridge != nil {
		list := w.agentBridge.ListPlugins()
		for name, metaAny := range list {
			if meta, ok := metaAny.(map[string]any); ok {
				plugins = append(plugins, map[string]any{
					"name":        name,
					"version":     meta["version"],
					"enabled":     meta["enabled"],
					"description": meta["description"],
					"priority":    meta["priority"],
				})
			}
		}
	}

	writeSuccess(rw, map[string]any{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

func (w *WebChannel) handleAgentInfo(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	writeSuccess(rw, map[string]any{
		"status":    "running",
		"sessions":  0,
		"max_steps": 15,
		"tools":     0,
		"model":     "",
	})
}

func (w *WebChannel) handleVoiceInfo(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	enabled := false
	engines := []string{}
	if w.agentBridge != nil {
		enabled = w.agentBridge.HasVoiceEngine()
		engines = w.agentBridge.ListVoiceEngines()
	}

	writeSuccess(rw, map[string]any{
		"enabled": enabled,
		"engine":  "",
		"engines": engines,
	})
}

func (w *WebChannel) handleTTS(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	var req struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(rw, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	if req.Text == "" {
		writeError(rw, http.StatusBadRequest, "text required")
		return
	}

	if w.agentBridge == nil || !w.agentBridge.HasVoiceEngine() {
		writeSuccess(rw, map[string]any{
			"status":  statusNotConfigured,
			"message": "TTS engine not configured",
		})
		return
	}

	audio, err := w.agentBridge.TextToSpeech(r.Context(), req.Text)
	if err != nil {
		writeError(rw, http.StatusInternalServerError, "TTS failed: "+err.Error())
		return
	}

	rw.Header().Set(headerContentType, "audio/mpeg")
	rw.Write(audio)
}

func (w *WebChannel) handleSTT(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	if w.agentBridge == nil || !w.agentBridge.HasVoiceEngine() {
		writeSuccess(rw, map[string]any{
			"status":  statusNotConfigured,
			"message": "STT engine not configured",
		})
		return
	}

	audio, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(rw, http.StatusBadRequest, "Failed to read audio data: "+err.Error())
		return
	}

	text, err := w.agentBridge.SpeechToText(r.Context(), audio)
	if err != nil {
		writeError(rw, http.StatusInternalServerError, "STT failed: "+err.Error())
		return
	}

	writeSuccess(rw, map[string]any{
		"status": statusSuccess,
		"text":   text,
	})
}

func (w *WebChannel) handleModels(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	models := []map[string]any{
		{"id": "gpt-4", "provider": "openai", "type": "chat"},
		{"id": "gpt-3.5-turbo", "provider": "openai", "type": "chat"},
		{"id": "claude-3-opus", "provider": "anthropic", "type": "chat"},
		{"id": "claude-3-sonnet", "provider": "anthropic", "type": "chat"},
		{"id": "glm-4", "provider": "zhipu", "type": "chat"},
		{"id": "glm-5", "provider": "zhipu", "type": "chat"},
		{"id": "deepseek-chat", "provider": "deepseek", "type": "chat"},
		{"id": "qwen-max", "provider": "qwen", "type": "chat"},
	}

	writeSuccess(rw, map[string]any{
		"models": models,
		"count":  len(models),
	})
}

func (w *WebChannel) handleProviders(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	providers := []map[string]any{
		{"name": "openai", "label": "OpenAI", "models": []string{"gpt-4", "gpt-3.5-turbo"}},
		{"name": "anthropic", "label": "Anthropic", "models": []string{"claude-3-opus", "claude-3-sonnet"}},
		{"name": "zhipu", "label": "智谱AI", "models": []string{"glm-4", "glm-5"}},
		{"name": "deepseek", "label": "DeepSeek", "models": []string{"deepseek-chat"}},
		{"name": "qwen", "label": "通义千问", "models": []string{"qwen-max"}},
	}

	writeSuccess(rw, map[string]any{
		"providers": providers,
		"count":     len(providers),
	})
}

func (w *WebChannel) handleMemoryInfo(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	enabled := false
	stats := map[string]any{}
	if w.agentBridge != nil {
		stats = w.agentBridge.GetMemoryStats(r.Context())
		enabled = stats != nil
	}

	writeSuccess(rw, map[string]any{
		"enabled": enabled,
		"status":  "active",
		"stats":   stats,
	})
}

func (w *WebChannel) handleMemorySearch(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(rw, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	if req.Query == "" {
		writeError(rw, http.StatusBadRequest, "query required")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	if w.agentBridge == nil {
		writeSuccess(rw, map[string]any{
			"status":  statusNotConfigured,
			"message": "Memory system not initialized",
			"results": []any{},
		})
		return
	}

	results, err := w.agentBridge.SearchMemory(r.Context(), req.Query, req.Limit)
	if err != nil {
		writeError(rw, http.StatusInternalServerError, "Search failed: "+err.Error())
		return
	}

	writeSuccess(rw, map[string]any{
		"status":  statusSuccess,
		"results": results,
	})
}

func (w *WebChannel) handleTranslate(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	var req struct {
		Text string `json:"text"`
		From string `json:"from"`
		To   string `json:"to"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(rw, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	if req.Text == "" {
		writeError(rw, http.StatusBadRequest, "text required")
		return
	}

	if req.To == "" {
		req.To = "zh"
	}

	if w.agentBridge == nil || !w.agentBridge.HasTranslator() {
		writeSuccess(rw, map[string]any{
			"status":  statusNotConfigured,
			"message": "Translator not initialized",
		})
		return
	}

	result, err := w.agentBridge.Translate(req.Text, req.From, req.To)
	if err != nil {
		writeError(rw, http.StatusInternalServerError, "Translation failed: "+err.Error())
		return
	}

	writeSuccess(rw, map[string]any{
		"status":        statusSuccess,
		"original_text": req.Text,
		"translated":    result,
		"from":          req.From,
		"to":            req.To,
	})
}

func (w *WebChannel) appendAttachments(message string, attachments []Attachment) string {
	var fileRefs []string
	for _, att := range attachments {
		if att.FilePath == "" {
			continue
		}
		switch att.FileType {
		case "image":
			fileRefs = append(fileRefs, fmt.Sprintf("[图片: %s]", att.FilePath))
		case "video":
			fileRefs = append(fileRefs, fmt.Sprintf("[视频: %s]", att.FilePath))
		default:
			fileRefs = append(fileRefs, fmt.Sprintf("[文件: %s]", att.FilePath))
		}
	}
	if len(fileRefs) > 0 {
		logger.Debug("Attached files to message", zap.Int("count", len(fileRefs)))
		return message + "\n" + strings.Join(fileRefs, "\n")
	}
	return message
}
