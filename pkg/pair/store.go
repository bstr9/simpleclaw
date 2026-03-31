package pair

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	mu   sync.RWMutex
	db   *sql.DB
	path string
}

func NewStore(workspaceDir string) (*Store, error) {
	if workspaceDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home dir: %w", err)
		}
		workspaceDir = filepath.Join(homeDir, ".simpleclaw")
	}

	pairDir := filepath.Join(workspaceDir, "pair")
	if err := os.MkdirAll(pairDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create pair dir: %w", err)
	}

	dbPath := filepath.Join(pairDir, "pair.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	s := &Store{db: db, path: dbPath}
	if err := s.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init tables: %w", err)
	}

	return s, nil
}

func (s *Store) initTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS user_auths (
			user_id TEXT NOT NULL,
			channel_type TEXT NOT NULL,
			token TEXT NOT NULL,
			refresh_token TEXT,
			scopes TEXT,
			granted_at DATETIME NOT NULL,
			expires_at DATETIME,
			PRIMARY KEY (user_id, channel_type)
		)`,
		`CREATE TABLE IF NOT EXISTS session_pairs (
			session_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			channel_type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			paired_at DATETIME,
			expires_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_session_pairs_user ON session_pairs(user_id)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SaveUserAuth(auth *UserAuth) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	scopesJSON, _ := json.Marshal(auth.Scopes)
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO user_auths 
		(user_id, channel_type, token, refresh_token, scopes, granted_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		auth.UserID, auth.ChannelType, auth.Token, auth.RefreshToken,
		string(scopesJSON), auth.GrantedAt, auth.ExpiresAt,
	)
	return err
}

func (s *Store) GetUserAuth(userID, channelType string) (*UserAuth, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var auth UserAuth
	var scopesJSON string
	var refreshToken, expiresAt sql.NullString

	err := s.db.QueryRow(`
		SELECT user_id, channel_type, token, refresh_token, scopes, granted_at, expires_at
		FROM user_auths WHERE user_id = ? AND channel_type = ?`,
		userID, channelType,
	).Scan(&auth.UserID, &auth.ChannelType, &auth.Token, &refreshToken,
		&scopesJSON, &auth.GrantedAt, &expiresAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	auth.RefreshToken = refreshToken.String
	json.Unmarshal([]byte(scopesJSON), &auth.Scopes)
	if expiresAt.Valid {
		auth.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt.String)
	}

	return &auth, nil
}

func (s *Store) SaveSessionPair(pair *SessionPair) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO session_pairs
		(session_id, user_id, channel_type, status, paired_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		pair.SessionID, pair.UserID, pair.ChannelType,
		pair.Status, pair.PairedAt, pair.ExpiresAt,
	)
	return err
}

func (s *Store) GetSessionPair(sessionID string) (*SessionPair, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pair SessionPair
	var pairedAt, expiresAt sql.NullString

	err := s.db.QueryRow(`
		SELECT session_id, user_id, channel_type, status, paired_at, expires_at
		FROM session_pairs WHERE session_id = ?`,
		sessionID,
	).Scan(&pair.SessionID, &pair.UserID, &pair.ChannelType,
		&pair.Status, &pairedAt, &expiresAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if pairedAt.Valid {
		pair.PairedAt, _ = time.Parse(time.RFC3339, pairedAt.String)
	}
	if expiresAt.Valid {
		pair.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt.String)
	}

	return &pair, nil
}

func (s *Store) DeleteUserAuth(userID, channelType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM user_auths WHERE user_id = ? AND channel_type = ?`,
		userID, channelType)
	return err
}

func (s *Store) DeleteSessionPair(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM session_pairs WHERE session_id = ?`, sessionID)
	return err
}

func (s *Store) CleanExpired(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Format(time.RFC3339)

	_, err := s.db.ExecContext(ctx, `DELETE FROM session_pairs WHERE expires_at < ?`, now)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM user_auths WHERE expires_at < ?`, now)
	return err
}
