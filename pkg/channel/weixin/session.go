// Package weixin 提供微信个人号渠道实现
// 本文件实现同步游标和上下文令牌的持久化存储，支持多账号
package weixin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

const (
	// sessionLogPrefix 会话存储日志前缀
	sessionLogPrefix = "[Weixin]"

	// syncBufSuffix 同步游标文件后缀
	syncBufSuffix = ".sync.json"

	// contextTokenSuffix 上下文令牌文件后缀
	contextTokenSuffix = ".context-tokens.json"
)

// resolveAccountFilePath 构建账号级文件路径
func resolveAccountFilePath(dir, accountID, suffix string) string {
	return filepath.Join(dir, accountID+suffix)
}

// --- 同步游标存储 ---

// syncBufData 同步游标文件数据格式
type syncBufData struct {
	AccountID      string `json:"account_id"`
	GetUpdatesBuf  string `json:"get_updates_buf"`
	SavedAt        string `json:"saved_at"`
}

// SyncBufStore 管理 getUpdatesBuf 的持久化存储
// getUpdatesBuf 是微信长轮询的同步游标，重启后丢失会导致重复消息
type SyncBufStore struct {
	dir string // 存储目录（~/.weixin/accounts/）
	mu  sync.Mutex
}

// NewSyncBufStore 创建同步游标存储
func NewSyncBufStore(dir string) *SyncBufStore {
	return &SyncBufStore{dir: dir}
}

// Save 保存账号的同步游标到文件
// 文件路径: {dir}/{accountID}.sync.json
func (s *SyncBufStore) Save(accountID, buf string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 确保目录存在
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		logger.Warn(sessionLogPrefix+" 保存同步游标失败", zap.Error(err))
		return nil
	}

	data := syncBufData{
		AccountID:     accountID,
		GetUpdatesBuf: buf,
		SavedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		logger.Warn(sessionLogPrefix+" 保存同步游标失败", zap.Error(err))
		return nil
	}

	path := resolveAccountFilePath(s.dir, accountID, syncBufSuffix)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		logger.Warn(sessionLogPrefix+" 保存同步游标失败", zap.Error(err))
		return nil
	}

	logger.Info(sessionLogPrefix+" 同步游标已保存", zap.String("account_id", accountID))
	return nil
}

// Load 加载账号的同步游标
func (s *SyncBufStore) Load(accountID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := resolveAccountFilePath(s.dir, accountID, syncBufSuffix)
	raw, err := os.ReadFile(path)
	if err != nil {
		logger.Warn(sessionLogPrefix+" 加载同步游标失败", zap.Error(err))
		return ""
	}

	var data syncBufData
	if err := json.Unmarshal(raw, &data); err != nil {
		logger.Warn(sessionLogPrefix+" 加载同步游标失败", zap.Error(err))
		return ""
	}

	logger.Info(sessionLogPrefix+" 同步游标已加载", zap.String("account_id", accountID))
	return data.GetUpdatesBuf
}

// Clear 清除账号的同步游标（re-login 时调用）
func (s *SyncBufStore) Clear(accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := resolveAccountFilePath(s.dir, accountID, syncBufSuffix)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Warn(sessionLogPrefix+" 清除同步游标失败", zap.Error(err))
		return nil
	}

	logger.Info(sessionLogPrefix+" 同步游标已清除", zap.String("account_id", accountID))
	return nil
}

// --- 上下文令牌存储 ---

// contextTokenData 上下文令牌文件数据格式
type contextTokenData struct {
	AccountID string            `json:"account_id"`
	Tokens    map[string]string `json:"tokens"`
	SavedAt   string            `json:"saved_at"`
}

// ContextTokenStore 管理 contextToken 的持久化存储
// contextToken 是消息回复的必要参数，重启后丢失会导致无法回复
type ContextTokenStore struct {
	dir string // 存储目录（~/.weixin/accounts/）
	mu  sync.Mutex
}

// NewContextTokenStore 创建上下文令牌存储
func NewContextTokenStore(dir string) *ContextTokenStore {
	return &ContextTokenStore{dir: dir}
}

// Save 保存账号的所有 contextToken
// 文件路径: {dir}/{accountID}.context-tokens.json
func (s *ContextTokenStore) Save(accountID string, tokens map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 确保目录存在
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		logger.Warn(sessionLogPrefix+" 保存上下文令牌失败", zap.Error(err))
		return nil
	}

	data := contextTokenData{
		AccountID: accountID,
		Tokens:    tokens,
		SavedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		logger.Warn(sessionLogPrefix+" 保存上下文令牌失败", zap.Error(err))
		return nil
	}

	path := resolveAccountFilePath(s.dir, accountID, contextTokenSuffix)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		logger.Warn(sessionLogPrefix+" 保存上下文令牌失败", zap.Error(err))
		return nil
	}

	logger.Info(sessionLogPrefix+" 上下文令牌已保存", zap.String("account_id", accountID))
	return nil
}

// Load 加载账号的所有 contextToken
func (s *ContextTokenStore) Load(accountID string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := resolveAccountFilePath(s.dir, accountID, contextTokenSuffix)
	raw, err := os.ReadFile(path)
	if err != nil {
		logger.Warn(sessionLogPrefix+" 加载上下文令牌失败", zap.Error(err))
		return map[string]string{}
	}

	var data contextTokenData
	if err := json.Unmarshal(raw, &data); err != nil {
		logger.Warn(sessionLogPrefix+" 加载上下文令牌失败", zap.Error(err))
		return map[string]string{}
	}

	// 确保返回非 nil map
	if data.Tokens == nil {
		data.Tokens = map[string]string{}
	}

	logger.Info(sessionLogPrefix+" 上下文令牌已加载", zap.String("account_id", accountID))
	return data.Tokens
}

// Clear 清除账号的所有 contextToken
func (s *ContextTokenStore) Clear(accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := resolveAccountFilePath(s.dir, accountID, contextTokenSuffix)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Warn(sessionLogPrefix+" 清除上下文令牌失败", zap.Error(err))
		return nil
	}

	logger.Info(sessionLogPrefix+" 上下文令牌已清除", zap.String("account_id", accountID))
	return nil
}
