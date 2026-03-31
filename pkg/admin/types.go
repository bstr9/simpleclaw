package admin

type AdminConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Host          string `mapstructure:"host"`
	Port          int    `mapstructure:"port"`
	Username      string `mapstructure:"username"`
	PasswordHash  string `mapstructure:"password_hash"`
	SessionSecret string `mapstructure:"session_secret"`
	StaticDir     string `mapstructure:"static_dir"`
}

func DefaultAdminConfig() *AdminConfig {
	return &AdminConfig{
		Enabled:       true,
		Host:          "0.0.0.0",
		Port:          8081,
		Username:      "admin",
		SessionSecret: "",
		StaticDir:     "",
	}
}

type SetupRequest struct {
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
	APIBase  string `json:"api_base"`
	Channel  string `json:"channel"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}

type ConfigResponse struct {
	Config map[string]interface{} `json:"config"`
}

type UpdateConfigRequest struct {
	Config map[string]interface{} `json:"config"`
}

type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

type TestLLMRequest struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	APIBase  string `json:"api_base"`
	Model    string `json:"model"`
}

type TestLLMResponse struct {
	Success bool   `json:"success"`
	Model   string `json:"model,omitempty"`
	Error   string `json:"error,omitempty"`
}

type SystemStatus struct {
	Version    string          `json:"version"`
	Uptime     string          `json:"uptime"`
	Channels   []ChannelStatus `json:"channels"`
	Configured bool            `json:"configured"`
}

type ChannelStatus struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    int         `json:"code,omitempty"`
}
