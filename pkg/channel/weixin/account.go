// Package weixin 提供微信个人号渠道实现
// account.go 实现多账号状态结构和账号索引/凭证管理
package weixin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

const (
	// weixinStateDir 微信状态目录名（位于用户主目录下）
	weixinStateDir = ".weixin"

	// accountsSubDir 账号凭证子目录名
	accountsSubDir = "accounts"

	// accountIndexFile 账号索引文件名
	accountIndexFile = "accounts.json"
)

// accountState 账号状态，包含单个微信账号的所有运行时数据
type accountState struct {
	id            string       // 账号唯一标识（使用 UserID）
	api           *weixinAPI   // API 客户端
	credentials   *Credentials // 登录凭证
	loginStatus   LoginStatus  // 登录状态
	currentQRURL  string       // 当前二维码 URL

	// 轮询状态
	stopChan      chan struct{}  // 停止信号
	pollWg        sync.WaitGroup // 轮询 goroutine 等待组
	getUpdatesBuf string         // 同步游标

	// 消息上下文令牌
	contextTokens map[string]string // 用户→contextToken 映射
	contextMu     sync.RWMutex      // contextToken 读写锁

	// 消息去重
	receivedMsgs *expiredMap // 已接收消息 ID 缓存

	// 临时目录（每账号独立）
	tmpDir string // 媒体文件临时目录
}

// resolveStateDir 返回微信状态目录路径（~/.weixin）
func resolveStateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, weixinStateDir)
}

// resolveAccountsDir 返回账号凭证目录路径（~/.weixin/accounts）
func resolveAccountsDir() string {
	return filepath.Join(resolveStateDir(), accountsSubDir)
}

// accountIndexPath 返回账号索引文件路径
func accountIndexPath(dir string) string {
	return filepath.Join(dir, accountIndexFile)
}

// accountCredentialPath 返回单个账号凭证文件路径
func accountCredentialPath(dir, accountID string) string {
	return filepath.Join(dir, accountsSubDir, normalizeAccountID(accountID)+".json")
}

// normalizeAccountID 将账号 ID 中的 @ 替换为 - 以确保文件系统安全
func normalizeAccountID(id string) string {
	return strings.ReplaceAll(id, "@", "-")
}

// denormalizeAccountID 将账号 ID 中的 - 还原为 @ 以用于 API 调用
func denormalizeAccountID(id string) string {
	// 只还原最后一个 - 为 @，因为 UserID 格式为 xxx@im.bot
	// 从右向左查找，还原最后一个分隔符
	lastDash := strings.LastIndex(id, "-")
	if lastDash == -1 {
		return id
	}
	return id[:lastDash] + "@" + id[lastDash+1:]
}

// --- 账号索引管理 ---

// loadAccountIndex 从索引文件加载账号 ID 列表
func loadAccountIndex(dir string) []string {
	indexPath := accountIndexPath(dir)
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil
	}

	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		logger.Warn(logPrefix+" 解析账号索引文件失败", zap.Error(err))
		return nil
	}
	return ids
}

// saveAccountIndex 将账号 ID 列表保存到索引文件
func saveAccountIndex(dir string, ids []string) error {
	data, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	indexPath := accountIndexPath(dir)
	if err := os.WriteFile(indexPath, data, 0600); err != nil {
		return err
	}

	logger.Debug(logPrefix+" 账号索引已保存", zap.String("path", indexPath), zap.Int("count", len(ids)))
	return nil
}

// registerAccountID 将账号 ID 注册到索引中（如已存在则跳过）
func registerAccountID(dir, accountID string) error {
	ids := loadAccountIndex(dir)

	// 检查是否已存在
	for _, id := range ids {
		if id == accountID {
			return nil
		}
	}

	// 追加并保存
	ids = append(ids, accountID)
	return saveAccountIndex(dir, ids)
}

// unregisterAccountID 从索引中移除账号 ID
func unregisterAccountID(dir, accountID string) error {
	ids := loadAccountIndex(dir)
	if ids == nil {
		return nil
	}

	// 过滤掉目标 ID
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != accountID {
			filtered = append(filtered, id)
		}
	}

	if len(filtered) == len(ids) {
		// 没有变化，无需写入
		return nil
	}

	return saveAccountIndex(dir, filtered)
}

// --- 账号凭证管理 ---

// loadAccountCredential 加载指定账号的凭证
func loadAccountCredential(dir, accountID string) (*Credentials, error) {
	credPath := accountCredentialPath(dir, accountID)
	data, err := os.ReadFile(credPath)
	if err != nil {
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

// saveAccountCredential 保存指定账号的凭证（权限 0600）
func saveAccountCredential(dir, accountID string, creds *Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	// 确保账号目录存在
	accountsDir := filepath.Join(dir, accountsSubDir)
	if err := os.MkdirAll(accountsDir, 0700); err != nil {
		return err
	}

	credPath := accountCredentialPath(dir, accountID)
	if err := os.WriteFile(credPath, data, 0600); err != nil {
		return err
	}

	logger.Info(logPrefix+" 账号凭证已保存",
		zap.String("account_id", accountID),
		zap.String("path", credPath))
	return nil
}

// removeAccountCredential 删除指定账号的凭证文件
func removeAccountCredential(dir, accountID string) error {
	credPath := accountCredentialPath(dir, accountID)
	err := os.Remove(credPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		logger.Info(logPrefix+" 账号凭证已删除",
			zap.String("account_id", accountID),
			zap.String("path", credPath))
	}
	return nil
}

// listAccountIDs 返回所有已注册的账号 ID 列表
func listAccountIDs(dir string) []string {
	return loadAccountIndex(dir)
}

// --- 过期账号清理 ---

// clearStaleAccountsForUserId 清理与指定 userId 相同的过期账号
// currentAccountID 为当前活跃账号，不会被清理
// onClear 回调在每个账号清理时调用
func clearStaleAccountsForUserId(dir, currentAccountID, userId string, onClear func(accountID string)) {
	ids := loadAccountIndex(dir)
	if ids == nil {
		return
	}

	for _, id := range ids {
		if id == currentAccountID {
			continue
		}

		// 尝试加载凭证以检查 userId
		creds, err := loadAccountCredential(dir, id)
		if err != nil {
			// 无法加载凭证，跳过
			continue
		}

		if creds.UserID == userId {
			logger.Info(logPrefix+" 清理过期账号",
				zap.String("account_id", id),
				zap.String("user_id", userId))

			// 删除凭证文件
			if err := removeAccountCredential(dir, id); err != nil {
				logger.Warn(logPrefix+" 删除过期账号凭证失败",
					zap.String("account_id", id),
					zap.Error(err))
			}

			// 从索引中移除
			if err := unregisterAccountID(dir, id); err != nil {
				logger.Warn(logPrefix+" 从索引中移除过期账号失败",
					zap.String("account_id", id),
					zap.Error(err))
			}

			// 调用回调
			if onClear != nil {
				onClear(id)
			}
		}
	}
}

// --- 旧版凭证迁移 ---

// migrateLegacyCredentials 将旧版单文件凭证迁移到多账号结构
// legacyPath: 旧凭证文件路径（如 ~/.weixin_cow_credentials.json）
// accountsDir: 新状态目录（如 ~/.weixin）
func migrateLegacyCredentials(legacyPath, accountsDir string) error {
	// 检查旧文件是否存在
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 旧文件不存在，无需迁移
			return nil
		}
		return err
	}

	// 解析旧凭证
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		logger.Warn(logPrefix+" 解析旧凭证文件失败，跳过迁移", zap.Error(err))
		return nil
	}

	// 确定账号 ID（使用 UserID，若为空则回退到 "default"）
	accountID := creds.UserID
	if accountID == "" {
		accountID = "default"
	}

	// 保存到新的多账号路径
	if err := saveAccountCredential(accountsDir, accountID, &creds); err != nil {
		return err
	}

	// 注册到索引
	if err := registerAccountID(accountsDir, accountID); err != nil {
		return err
	}

	// 将旧文件重命名为 .bak
	bakPath := legacyPath + ".bak"
	if err := os.Rename(legacyPath, bakPath); err != nil {
		logger.Warn(logPrefix+" 重命名旧凭证文件失败",
			zap.String("legacy_path", legacyPath),
			zap.Error(err))
	} else {
		logger.Info(logPrefix+" 旧凭证文件已备份",
			zap.String("original", legacyPath),
			zap.String("backup", bakPath))
	}

	logger.Info(logPrefix+" 旧凭证迁移完成",
		zap.String("account_id", accountID),
		zap.String("user_id", creds.UserID))

	return nil
}
