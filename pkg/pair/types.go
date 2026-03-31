package pair

import "time"

const (
	StatusPendingPair = "pending"
	StatusActive      = "active"
	StatusExpired     = "expired"
)

type PairStatus struct {
	Paired    bool      `json:"paired"`
	Status    string    `json:"status"`
	AuthURL   string    `json:"auth_url,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Name      string    `json:"name,omitempty"`
	OpenID    string    `json:"open_id,omitempty"`
}

type UserAuth struct {
	UserID       string    `json:"user_id"`
	ChannelType  string    `json:"channel_type"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scopes       []string  `json:"scopes"`
	GrantedAt    time.Time `json:"granted_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Name         string    `json:"name,omitempty"`
}

type SessionPair struct {
	SessionID   string    `json:"session_id"`
	UserID      string    `json:"user_id"`
	ChannelType string    `json:"channel_type"`
	Status      string    `json:"status"`
	PairedAt    time.Time `json:"paired_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
}

type PairRequest struct {
	SessionID   string `json:"session_id"`
	UserID      string `json:"user_id"`
	ChannelType string `json:"channel_type"`
}

type PairResult struct {
	Success bool   `json:"success"`
	AuthURL string `json:"auth_url,omitempty"`
	Message string `json:"message,omitempty"`
}
