package pair

import (
	"fmt"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

type Provider interface {
	ChannelType() string
	StartPair(userID string) (authURL string, err error)
	CheckStatus(userID string) (PairStatus, error)
	RequiredScopes() []string
	IsUserAuthorized(userID string) (bool, error)
}

type Manager struct {
	mu        sync.RWMutex
	store     *Store
	providers map[string]Provider
}

func NewManager(store *Store) *Manager {
	return &Manager{
		store:     store,
		providers: make(map[string]Provider),
	}
}

func (m *Manager) RegisterProvider(p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.ChannelType()] = p
	logger.Info("[PairManager] Provider registered", zap.String("channel", p.ChannelType()))
}

func (m *Manager) CheckSessionPair(sessionID, userID, channelType string) (*PairStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionPair, err := m.store.GetSessionPair(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session pair: %w", err)
	}

	if sessionPair != nil && sessionPair.Status == StatusActive {
		expiresAt := sessionPair.ExpiresAt
		if expiresAt.IsZero() || expiresAt.After(time.Now()) {
			return &PairStatus{Paired: true, Status: StatusActive}, nil
		}
	}

	auth, err := m.store.GetUserAuth(userID, channelType)
	if err != nil {
		return nil, fmt.Errorf("failed to get user auth: %w", err)
	}

	if auth != nil && (auth.ExpiresAt.IsZero() || auth.ExpiresAt.After(time.Now())) {
		err := m.store.SaveSessionPair(&SessionPair{
			SessionID:   sessionID,
			UserID:      userID,
			ChannelType: channelType,
			Status:      StatusActive,
			PairedAt:    time.Now(),
		})
		if err != nil {
			logger.Warn("[PairManager] Failed to save session pair", zap.Error(err))
		}
		return &PairStatus{Paired: true, Status: StatusActive}, nil
	}

	return &PairStatus{Paired: false, Status: StatusPendingPair}, nil
}

func (m *Manager) StartPair(sessionID, userID, channelType string) (*PairResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.providers[channelType]
	if !ok {
		return &PairResult{
			Success: false,
			Message: fmt.Sprintf("no provider for channel type: %s", channelType),
		}, nil
	}

	authURL, err := p.StartPair(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to start pair: %w", err)
	}

	err = m.store.SaveSessionPair(&SessionPair{
		SessionID:   sessionID,
		UserID:      userID,
		ChannelType: channelType,
		Status:      StatusPendingPair,
	})
	if err != nil {
		logger.Warn("[PairManager] Failed to save pending session pair", zap.Error(err))
	}

	return &PairResult{
		Success: true,
		AuthURL: authURL,
		Message: "Please authorize using the URL above",
	}, nil
}

func (m *Manager) CompletePair(sessionID, userID, channelType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.providers[channelType]
	if !ok {
		return fmt.Errorf("no provider for channel type: %s", channelType)
	}

	status, err := p.CheckStatus(userID)
	if err != nil {
		return fmt.Errorf("failed to check status: %w", err)
	}

	if !status.Paired {
		return fmt.Errorf("user not yet authorized")
	}

	auth := &UserAuth{
		UserID:      userID,
		ChannelType: channelType,
		GrantedAt:   time.Now(),
		ExpiresAt:   status.ExpiresAt,
	}
	if err := m.store.SaveUserAuth(auth); err != nil {
		return fmt.Errorf("failed to save user auth: %w", err)
	}

	sessionPair := &SessionPair{
		SessionID:   sessionID,
		UserID:      userID,
		ChannelType: channelType,
		Status:      StatusActive,
		PairedAt:    time.Now(),
	}
	if err := m.store.SaveSessionPair(sessionPair); err != nil {
		return fmt.Errorf("failed to save session pair: %w", err)
	}

	logger.Info("[PairManager] Pair completed",
		zap.String("session_id", sessionID),
		zap.String("user_id", userID),
		zap.String("channel", channelType))

	return nil
}

func (m *Manager) GetUserAuth(userID, channelType string) (*UserAuth, error) {
	return m.store.GetUserAuth(userID, channelType)
}

func (m *Manager) GetSessionPair(sessionID string) (*SessionPair, error) {
	return m.store.GetSessionPair(sessionID)
}

func (m *Manager) Close() error {
	return m.store.Close()
}
