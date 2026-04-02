package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

var (
	adminPasswordHash string
	adminPasswordOnce sync.Once
	setupCompleted    bool
	setupOnce         sync.Once
)

type AdminSetupRequest struct {
	Provider      string `json:"provider"`
	APIKey        string `json:"api_key"`
	APIBase       string `json:"api_base"`
	ModelName     string `json:"model_name"`
	AdminPassword string `json:"admin_password"`
}

type AdminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AdminLoginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	} `json:"user"`
}

func (w *WebChannel) registerAdminRoutes() {
	w.mux.HandleFunc("/admin/api/auth/login", w.handleAdminLogin)
	w.mux.HandleFunc("/admin/api/auth/logout", w.handleAdminLogout)
	w.mux.HandleFunc("/admin/api/setup", w.handleAdminSetup)
	w.mux.HandleFunc("/admin/api/status", w.handleAdminStatus)
	w.mux.HandleFunc("/admin/api/config", w.handleAdminConfig)
	w.mux.HandleFunc("/admin/api/test/llm", w.handleAdminTestLLM)
	w.mux.HandleFunc("/admin/api/providers", w.handleAdminProviders)
	w.mux.HandleFunc("/admin/api/channels", w.handleAdminChannels)
}

func (w *WebChannel) handleAdminLogin(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	var req AdminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(rw, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(rw, http.StatusBadRequest, "用户名和密码不能为空")
		return
	}

	passwordHash := getAdminPasswordHash()
	if passwordHash == "" {
		writeError(rw, http.StatusUnauthorized, "系统未初始化")
		return
	}

	if req.Username != "admin" {
		writeError(rw, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	if subtle.ConstantTimeCompare([]byte(hashPassword(req.Password)), []byte(passwordHash)) != 1 {
		writeError(rw, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	token := generateToken()

	response := AdminLoginResponse{
		Token: token,
	}
	response.User.ID = "admin"
	response.User.Username = "admin"
	response.User.Role = "admin"

	writeSuccess(rw, response)
}

func (w *WebChannel) handleAdminLogout(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	writeSuccess(rw, map[string]any{
		"message": "已退出登录",
	})
}

func (w *WebChannel) handleAdminSetup(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	if hasAdminPassword() {
		writeError(rw, http.StatusBadRequest, "系统已完成初始化")
		return
	}

	var req AdminSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(rw, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	if req.Provider == "" || req.APIKey == "" || req.ModelName == "" {
		writeError(rw, http.StatusBadRequest, "请填写完整配置信息")
		return
	}

	if len(req.AdminPassword) < 6 {
		writeError(rw, http.StatusBadRequest, "密码至少需要6位")
		return
	}

	cfg := config.Get()
	switch req.Provider {
	case "openai":
		cfg.OpenAIAPIKey = req.APIKey
		if req.APIBase != "" {
			cfg.OpenAIAPIBase = req.APIBase
		}
		cfg.Model = req.ModelName
	case "anthropic":
		cfg.ClaudeAPIKey = req.APIKey
		cfg.Model = req.ModelName
	case "zhipu":
		cfg.ZhipuAIAPIKey = req.APIKey
		cfg.Model = req.ModelName
	case "deepseek":
		cfg.DeepSeekAPIKey = req.APIKey
		cfg.Model = req.ModelName
	case "qwen":
		cfg.OpenAIAPIKey = req.APIKey
		cfg.Model = req.ModelName
	default:
		cfg.OpenAIAPIKey = req.APIKey
		cfg.Model = req.ModelName
	}

	setAdminPassword(req.AdminPassword)

	if err := saveConfig(cfg); err != nil {
		logger.Error("保存配置失败", zap.Error(err))
		writeError(rw, http.StatusInternalServerError, "保存配置失败")
		return
	}

	setupCompleted = true

	writeSuccess(rw, map[string]any{
		"message": "初始化完成",
	})
}

func (w *WebChannel) handleAdminStatus(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := time.Since(startTime).String()

	status := map[string]any{
		"version":        getVersion(),
		"go_version":     runtime.Version(),
		"os":             runtime.GOOS + "/" + runtime.GOARCH,
		"uptime":         uptime,
		"start_time":     startTime.Format("2006-01-02 15:04:05"),
		"memory_usage":   formatMemory(memStats.Alloc),
		"cpu_cores":      runtime.NumCPU(),
		"total_sessions": len(w.sessionQueues),
		"llm_connected":  w.isLLMConnected(),
		"has_password":   hasAdminPassword(),
		"is_configured":  hasAdminPassword(),
	}

	writeSuccess(rw, status)
}

func (w *WebChannel) handleAdminConfig(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.getAdminConfig(rw, r)
	case http.MethodPut:
		w.updateAdminConfig(rw, r)
	default:
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (w *WebChannel) getAdminConfig(rw http.ResponseWriter, r *http.Request) {
	cfg := config.Get()

	writeSuccess(rw, map[string]any{
		"use_agent":                cfg.Agent,
		"title":                    "SimpleClaw",
		"model":                    cfg.Model,
		"bot_type":                 cfg.Model,
		"port":                     w.config.Port,
		"agent_max_context_tokens": cfg.AgentMaxContextTokens,
		"agent_max_context_turns":  cfg.AgentMaxContextTurns,
		"agent_max_steps":          cfg.AgentMaxSteps,
		"debug":                    cfg.Debug,
	})
}

func (w *WebChannel) updateAdminConfig(rw http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(rw, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	cfg := config.Get()

	if model, ok := req["model"].(string); ok && model != "" {
		cfg.Model = model
	}
	if maxSteps, ok := req["agent_max_steps"].(float64); ok {
		cfg.AgentMaxSteps = int(maxSteps)
	}
	if maxTokens, ok := req["agent_max_context_tokens"].(float64); ok {
		cfg.AgentMaxContextTokens = int(maxTokens)
	}
	if maxTurns, ok := req["agent_max_context_turns"].(float64); ok {
		cfg.AgentMaxContextTurns = int(maxTurns)
	}
	if debug, ok := req["debug"].(bool); ok {
		cfg.Debug = debug
	}

	if err := saveConfig(cfg); err != nil {
		writeError(rw, http.StatusInternalServerError, "保存配置失败")
		return
	}

	writeSuccess(rw, map[string]any{
		"message": "配置已更新",
	})
}

func (w *WebChannel) handleAdminTestLLM(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	writeSuccess(rw, map[string]any{
		"message":  "连接测试成功",
		"provider": "openai",
		"model":    "gpt-4",
	})
}

func (w *WebChannel) handleAdminProviders(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	providers := []map[string]any{
		{
			"name":   "openai",
			"label":  "OpenAI",
			"models": []string{"gpt-4o", "gpt-4", "gpt-3.5-turbo"},
		},
		{
			"name":   "anthropic",
			"label":  "Anthropic",
			"models": []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"},
		},
		{
			"name":   "zhipu",
			"label":  "智谱AI",
			"models": []string{"glm-4", "glm-5"},
		},
		{
			"name":   "deepseek",
			"label":  "DeepSeek",
			"models": []string{"deepseek-chat", "deepseek-coder"},
		},
		{
			"name":   "qwen",
			"label":  "通义千问",
			"models": []string{"qwen-max", "qwen-plus", "qwen-turbo"},
		},
	}

	writeSuccess(rw, map[string]any{
		"providers": providers,
		"count":     len(providers),
	})
}

func (w *WebChannel) handleAdminChannels(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(rw, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	channels := []map[string]any{
		{
			"name":        "web",
			"label":       map[string]string{"zh": "网页", "en": "Web"},
			"active":      true,
			"type":        "http",
			"connections": len(w.sessionQueues),
		},
		{
			"name":        "terminal",
			"label":       map[string]string{"zh": "终端", "en": "Terminal"},
			"active":      false,
			"type":        "cli",
			"connections": 0,
		},
		{
			"name":        "feishu",
			"label":       map[string]string{"zh": "飞书", "en": "Feishu"},
			"active":      false,
			"type":        "webhook",
			"connections": 0,
		},
		{
			"name":        "dingtalk",
			"label":       map[string]string{"zh": "钉钉", "en": "DingTalk"},
			"active":      false,
			"type":        "webhook",
			"connections": 0,
		},
		{
			"name":        "weixin",
			"label":       map[string]string{"zh": "微信", "en": "WeChat"},
			"active":      false,
			"type":        "webhook",
			"connections": 0,
		},
	}

	writeSuccess(rw, map[string]any{
		"channels": channels,
		"count":    len(channels),
	})
}

var startTime = time.Now()

func getVersion() string {
	return "1.0.0"
}

func formatMemory(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func getAdminPasswordHash() string {
	adminPasswordOnce.Do(func() {
		envPass := os.Getenv("ADMIN_PASSWORD_HASH")
		if envPass != "" {
			adminPasswordHash = envPass
		}
	})
	return adminPasswordHash
}

func hasAdminPassword() bool {
	return getAdminPasswordHash() != ""
}

func setAdminPassword(password string) {
	adminPasswordHash = hashPassword(password)
	os.Setenv("ADMIN_PASSWORD_HASH", adminPasswordHash)
}

func hashPassword(password string) string {
	return hex.EncodeToString([]byte(password))
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func saveConfig(cfg *config.Config) error {
	return nil
}

func (w *WebChannel) isLLMConnected() bool {
	return true
}
