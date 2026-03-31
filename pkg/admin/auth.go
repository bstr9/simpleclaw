package admin

import (
	"golang.org/x/crypto/bcrypt"
)

type AuthManager struct {
	config *AdminConfig
}

func NewAuthManager(cfg *AdminConfig) *AuthManager {
	return &AuthManager{config: cfg}
}

func (a *AuthManager) ValidatePassword(username, password string) bool {
	if username != a.config.Username {
		return false
	}

	if a.config.PasswordHash == "" {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(a.config.PasswordHash), []byte(password))
	return err == nil
}

func (a *AuthManager) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
