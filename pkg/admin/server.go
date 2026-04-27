package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/llm"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	config        *AdminConfig
	httpServer    *http.Server
	mux           *http.ServeMux
	auth          *AuthManager
	configPath    string
	startTime     time.Time
	sessions      map[string]*Session
	sessionsMu    sync.RWMutex
	staticFS      fs.FS
	useEmbedded   bool
	webChannelURL string
}

type Session struct {
	Token     string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func NewServer(cfg *AdminConfig, configPath string) *Server {
	if cfg == nil {
		cfg = DefaultAdminConfig()
	}

	s := &Server{
		config:     cfg,
		mux:        http.NewServeMux(),
		configPath: configPath,
		startTime:  time.Now(),
		sessions:   make(map[string]*Session),
		auth:       NewAuthManager(cfg),
	}

	if HasEmbeddedUI() {
		s.staticFS = GetDistFS()
		s.useEmbedded = true
		logger.Info("[Admin] Using embedded static files")
	} else if cfg.StaticDir != "" {
		if _, err := os.Stat(cfg.StaticDir); err == nil {
			logger.Info("[Admin] Using static directory", zap.String("dir", cfg.StaticDir))
		}
	}

	s.registerRoutes()
	return s
}

func (s *Server) SetStaticFS(fsys fs.FS) {
	s.staticFS = fsys
	s.useEmbedded = true
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/admin/api/setup", s.handleSetup)
	s.mux.HandleFunc("/admin/api/auth/login", s.handleLogin)
	s.mux.HandleFunc("/admin/api/auth/logout", s.withAuth(s.handleLogout))
	s.mux.HandleFunc("/admin/api/config", s.withAuth(s.handleConfig))
	s.mux.HandleFunc("/admin/api/config/validate", s.withAuth(s.handleValidate))
	s.mux.HandleFunc("/admin/api/test/llm", s.withAuth(s.handleTestLLM))
	s.mux.HandleFunc("/admin/api/status", s.handleStatus)
	s.mux.HandleFunc("/admin/api/channels", s.withAuth(s.handleChannels))
	s.mux.HandleFunc("/admin/api/channels/", s.withAuth(s.handleChannelAction))
	s.mux.HandleFunc("/admin/api/providers", s.handleProviders)
	s.mux.HandleFunc("/message", s.proxyToWebChannel)
	s.mux.HandleFunc("/stream", s.proxyToWebChannel)
	s.mux.HandleFunc("/upload", s.proxyToWebChannel)
	s.mux.HandleFunc("/uploads/", s.proxyToWebChannel)
	s.mux.HandleFunc("/config", s.proxyToWebChannel)
	s.mux.HandleFunc("/api/", s.proxyToWebChannel)
	s.mux.HandleFunc("/", s.handleSPA)
}

func (s *Server) withAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.config.Enabled && s.config.PasswordHash != "" {
			token := s.extractToken(r)
			if token == "" {
				writeAPIError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			session := s.validateSession(token)
			if session == nil {
				writeAPIError(w, http.StatusUnauthorized, "Invalid or expired session")
				return
			}
		}
		handler(w, r)
	}
}

func (s *Server) extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	cookie, err := r.Cookie("admin_token")
	if err == nil {
		return cookie.Value
	}

	return r.URL.Query().Get("token")
}

func (s *Server) validateSession(token string) *Session {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	session, ok := s.sessions[token]
	if !ok {
		return nil
	}

	if time.Now().After(session.ExpiresAt) {
		return nil
	}

	return session
}

func (s *Server) createSession(username string) *Session {
	token := generateToken()
	session := &Session{
		Token:     token,
		Username:  username,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	s.sessionsMu.Lock()
	s.sessions[token] = session
	s.sessionsMu.Unlock()

	return session
}

func (s *Server) removeSession(token string) {
	s.sessionsMu.Lock()
	delete(s.sessions, token)
	s.sessionsMu.Unlock()
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if s.hasPassword() {
		writeAPIError(w, http.StatusBadRequest, "Already configured with password")
		return
	}

	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if req.Password == "" {
		writeAPIError(w, http.StatusBadRequest, "password is required")
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	username := req.Username
	if username == "" {
		username = "admin"
	}

	cfg := config.Get()

	if !s.isConfigured() {
		if req.Model == "" || req.APIKey == "" || req.Channel == "" {
			writeAPIError(w, http.StatusBadRequest, "model, api_key and channel are required for initial setup")
			return
		}
		cfg.Model = req.Model
		cfg.OpenAIAPIKey = req.APIKey
		if req.APIBase != "" {
			cfg.OpenAIAPIBase = req.APIBase
		}
		cfg.ChannelType = req.Channel
		cfg.Agent = true
	}

	adminCfg := &AdminConfig{
		Enabled:      true,
		Host:         s.config.Host,
		Port:         s.config.Port,
		Username:     username,
		PasswordHash: string(passwordHash),
	}

	if err := s.saveConfigWithAdmin(cfg, adminCfg); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "Failed to save config: "+err.Error())
		return
	}

	s.config.PasswordHash = string(passwordHash)
	s.config.Username = req.Username

	writeAPISuccess(w, map[string]any{
		"success":     true,
		"config_path": s.configPath,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if !s.auth.ValidatePassword(req.Username, req.Password) {
		writeAPIError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	session := s.createSession(req.Username)

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		Expires:  session.ExpiresAt,
	})

	writeAPISuccess(w, LoginResponse{
		Success: true,
		Token:   session.Token,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := s.extractToken(r)
	if token != "" {
		s.removeSession(token)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	writeAPISuccess(w, map[string]any{"success": true})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getConfig(w, r)
	case http.MethodPut:
		s.updateConfig(w, r)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *Server) getConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := config.Get()
	masked := cfg.MaskSensitive()

	masked["admin"] = map[string]any{
		"enabled":  s.config.Enabled,
		"username": s.config.Username,
		"host":     s.config.Host,
		"port":     s.config.Port,
	}

	writeAPISuccess(w, ConfigResponse{Config: masked})
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if err := s.saveConfig(req.Config); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "Failed to save config: "+err.Error())
		return
	}

	if err := config.Reload(s.configPath); err != nil {
		logger.Warn("[Admin] Failed to reload config", zap.Error(err))
	}

	writeAPISuccess(w, map[string]any{"success": true})
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	cfg := config.Get()
	errors := validateConfig(cfg)

	writeAPISuccess(w, ValidationResult{
		Valid:  len(errors) == 0,
		Errors: errors,
	})
}

func (s *Server) handleTestLLM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req TestLLMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	result := s.testLLMConnection(&req)
	writeAPISuccess(w, result)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	uptime := time.Since(s.startTime).Round(time.Second)
	cfg := config.Get()

	status := map[string]any{
		"version":        "1.0.0",
		"uptime":         uptime.String(),
		"channels":       s.getChannelStatuses(),
		"configured":     s.isConfigured(),
		"has_password":   s.hasPassword(),
		"has_llm_config": cfg.OpenAIAPIKey != "" && cfg.OpenAIAPIKey != "YOUR_OPENAI_API_KEY_HERE",
		"has_channels":   cfg.ChannelType != "",
		"llm_provider":   cfg.Model,
		"api_key":        maskAPIKey(cfg.OpenAIAPIKey),
		"base_url":       cfg.OpenAIAPIBase,
		"model":          cfg.ModelName,
		"channel_type":   cfg.ChannelType,
		"admin_username": s.config.Username,
	}

	writeAPISuccess(w, status)
}

func maskAPIKey(key string) string {
	if key == "" || len(key) < 8 {
		return ""
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	channels := s.getChannelStatuses()
	writeAPISuccess(w, channels)
}

// handleChannelAction 处理渠道操作请求（如启动/停止）
func (s *Server) handleChannelAction(w http.ResponseWriter, r *http.Request) {
	// 从 URL 路径解析渠道名称: /admin/api/channels/{name}/toggle
	path := r.URL.Path
	prefix := "/admin/api/channels/"
	if !strings.HasPrefix(path, prefix) {
		writeAPIError(w, http.StatusNotFound, "Not found")
		return
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	channelName := parts[0]
	action := ""
	if len(parts) > 1 {
		action = strings.TrimSuffix(parts[1], "/")
	}

	if action != "toggle" {
		writeAPIError(w, http.StatusNotFound, "未知操作")
		return
	}

	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	mgr := channel.GetChannelManager()
	if mgr == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "渠道管理器不可用")
		return
	}

	ch := mgr.GetChannel(channelName)
	if ch != nil {
		// 渠道正在运行，停止它
		mgr.Stop(channelName)

		// 持久化：从配置中移除该渠道
		cfg := config.Get()
		channels := parseChannelTypes(cfg.ChannelType)
		var newChannels []string
		for _, c := range channels {
			if c != channelName {
				newChannels = append(newChannels, c)
			}
		}
		newChannelType := strings.Join(newChannels, ",")
		cfg.ChannelType = newChannelType
		if err := s.saveChannelTypeToConfig(newChannelType); err != nil {
			logger.Warn("[Admin] 持久化渠道状态失败", zap.Error(err))
		}

		writeAPISuccess(w, map[string]any{
			"channel": channelName,
			"action":  "stopped",
			"running": false,
		})
	} else {
		// 渠道未运行，启动它
		err := mgr.AddChannel(channelName)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "启动渠道失败: "+err.Error())
			return
		}

		// 持久化：向配置中添加该渠道
		cfg := config.Get()
		channels := parseChannelTypes(cfg.ChannelType)
		found := false
		for _, c := range channels {
			if c == channelName {
				found = true
				break
			}
		}
		if !found {
			channels = append(channels, channelName)
		}
		newChannelType := strings.Join(channels, ",")
		cfg.ChannelType = newChannelType
		if err := s.saveChannelTypeToConfig(newChannelType); err != nil {
			logger.Warn("[Admin] 持久化渠道状态失败", zap.Error(err))
		}

		writeAPISuccess(w, map[string]any{
			"channel": channelName,
			"action":  "started",
			"running": true,
		})
	}
}

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if strings.HasPrefix(path, "/admin/api/") {
		writeAPIError(w, http.StatusNotFound, "API endpoint not found")
		return
	}

	if s.config.StaticDir != "" {
		filePath := filepath.Join(s.config.StaticDir, path)
		if _, err := os.Stat(filePath); err == nil {
			http.ServeFile(w, r, filePath)
			return
		}
	}

	if s.useEmbedded && s.staticFS != nil {
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		logger.Info("[Admin SPA] Request", zap.String("path", path), zap.String("cleanPath", cleanPath), zap.Bool("hasStaticFS", s.staticFS != nil))

		content, err := fs.ReadFile(s.staticFS, cleanPath)
		if err != nil {
			logger.Warn("[Admin SPA] File not found in embed", zap.String("cleanPath", cleanPath), zap.Error(err))
		}
		if err == nil {
			contentType := getContentType(cleanPath)
			logger.Info("[Admin SPA] Serving file", zap.String("cleanPath", cleanPath), zap.String("contentType", contentType), zap.Int("size", len(content)))
			w.Header().Set("Content-Type", contentType)
			if strings.HasPrefix(cleanPath, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			w.Write(content)
			return
		}

		ext := filepath.Ext(cleanPath)
		if ext != "" && ext != ".html" {
			logger.Warn("[Admin SPA] Static file not found", zap.String("path", cleanPath))
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		indexContent, err := fs.ReadFile(s.staticFS, "index.html")
		if err == nil {
			logger.Info("[Admin SPA] Serving index.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(indexContent)
			return
		}
	}

	http.Error(w, "Admin UI not available. Build the frontend first.", http.StatusNotFound)
}

func getContentType(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".woff", ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	default:
		return "application/octet-stream"
	}
}

func (s *Server) isConfigured() bool {
	cfg := config.Get()
	return cfg.OpenAIAPIKey != "" && cfg.OpenAIAPIKey != "YOUR_OPENAI_API_KEY_HERE"
}

func (s *Server) hasPassword() bool {
	return s.config.PasswordHash != ""
}

func (s *Server) saveConfig(newConfig map[string]any) error {
	data, err := json.MarshalIndent(newConfig, "", "    ")
	if err != nil {
		return err
	}

	tempPath := s.configPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return err
	}

	if err := os.Rename(tempPath, s.configPath); err != nil {
		os.Remove(tempPath)
		return err
	}

	return nil
}

// saveChannelTypeToConfig 持久化渠道类型配置到磁盘
func (s *Server) saveChannelTypeToConfig(channelType string) error {
	// 读取当前配置文件
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var configMap map[string]any
	if err := json.Unmarshal(data, &configMap); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 更新 channel_type 字段
	configMap["channel_type"] = channelType

	// 写回磁盘
	if err := s.saveConfig(configMap); err != nil {
		return fmt.Errorf("保存配置文件失败: %w", err)
	}

	// 重新加载内存配置
	if err := config.Reload(s.configPath); err != nil {
		logger.Warn("[Admin] 重载配置失败", zap.Error(err))
	}

	return nil
}

func (s *Server) saveConfigWithAdmin(cfg *config.Config, adminCfg *AdminConfig) error {
	configMap := map[string]any{
		"model":              cfg.Model,
		"model_name":         cfg.ModelName,
		"channel_type":       cfg.ChannelType,
		"open_ai_api_key":    cfg.OpenAIAPIKey,
		"open_ai_api_base":   cfg.OpenAIAPIBase,
		"agent":              cfg.Agent,
		"agent_workspace":    cfg.AgentWorkspace,
		"agent_max_steps":    cfg.AgentMaxSteps,
		"debug":              cfg.Debug,
		"single_chat_prefix": cfg.SingleChatPrefix,
		"group_chat_prefix":  cfg.GroupChatPrefix,
		"character_desc":     cfg.CharacterDesc,
		"admin": map[string]any{
			"enabled":       adminCfg.Enabled,
			"host":          adminCfg.Host,
			"port":          adminCfg.Port,
			"username":      adminCfg.Username,
			"password_hash": adminCfg.PasswordHash,
		},
	}

	return s.saveConfig(configMap)
}

// testLLMConnection 测试 LLM 连接是否可用
func (s *Server) testLLMConnection(req *TestLLMRequest) *TestLLMResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	model, err := llm.NewModel(llm.ModelConfig{
		Provider: req.Provider,
		Model:    req.Model,
		APIKey:   req.APIKey,
		APIBase:  req.APIBase,
	})
	if err != nil {
		return &TestLLMResponse{Success: false, Model: req.Model, Error: err.Error()}
	}

	resp, err := model.Call(ctx, []llm.Message{
		{Role: llm.RoleUser, Content: "Hi"},
	}, llm.WithMaxTokens(5))
	if err != nil {
		return &TestLLMResponse{Success: false, Model: req.Model, Error: err.Error()}
	}

	return &TestLLMResponse{Success: true, Model: resp.Model}
}

// getChannelStatuses 获取所有渠道运行状态
func (s *Server) getChannelStatuses() []ChannelStatus {
	cfg := config.Get()
	configuredChannels := parseChannelTypes(cfg.ChannelType)

	mgr := channel.GetChannelManager()

	// 所有已注册的渠道类型
	allChannelTypes := channel.GetRegisteredChannelTypes()

	statuses := make([]ChannelStatus, 0, len(allChannelTypes))
	for _, chName := range allChannelTypes {
		running := false
		if mgr != nil {
			channelInst := mgr.GetChannel(chName)
			running = channelInst != nil
		}
		enabled := false
		for _, c := range configuredChannels {
			if c == chName {
				enabled = true
				break
			}
		}
		statuses = append(statuses, ChannelStatus{
			Name:    chName,
			Type:    chName,
			Enabled: enabled,
			Running: running,
		})
	}

	return statuses
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("[Admin] Starting admin server",
		zap.String("addr", addr),
		zap.Bool("configured", s.isConfigured()))

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func parseChannelTypes(raw string) []string {
	if raw == "" {
		return []string{}
	}
	types := strings.Split(raw, ",")
	result := make([]string, 0, len(types))
	for _, t := range types {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

func writeAPISuccess(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   message,
		Code:    status,
	})
}

func validateConfig(cfg *config.Config) []string {
	var errors []string

	if cfg.Model == "" {
		errors = append(errors, "model is required")
	}

	if cfg.OpenAIAPIKey == "" || cfg.OpenAIAPIKey == "YOUR_OPENAI_API_KEY_HERE" {
		errors = append(errors, "open_ai_api_key is required")
	}

	if cfg.ChannelType == "" {
		errors = append(errors, "channel_type is required")
	}

	return errors
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	providers := []map[string]any{
		{"name": "openai", "label": "OpenAI", "models": []string{"gpt-4", "gpt-4o", "gpt-3.5-turbo"}},
		{"name": "anthropic", "label": "Anthropic", "models": []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"}},
		{"name": "zhipu", "label": "智谱AI", "models": []string{"glm-4", "glm-4-plus", "glm-5"}},
		{"name": "deepseek", "label": "DeepSeek", "models": []string{"deepseek-chat", "deepseek-coder"}},
		{"name": "qwen", "label": "通义千问", "models": []string{"qwen-max", "qwen-plus", "qwen-turbo"}},
	}

	writeAPISuccess(w, map[string]any{
		"providers": providers,
		"count":     len(providers),
	})
}

func (s *Server) proxyToWebChannel(w http.ResponseWriter, r *http.Request) {
	targetURL := s.webChannelURL
	if targetURL == "" {
		targetURL = "http://localhost:9899"
	}

	proxyReq, err := http.NewRequest(r.Method, targetURL+r.URL.Path+"?"+r.URL.RawQuery, r.Body)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "Failed to create proxy request")
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, "Web channel unavailable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err != nil {
			break
		}
	}
}

func (s *Server) SetWebChannelURL(url string) {
	s.webChannelURL = url
}
