// Package extension 提供统一的扩展系统。
// manager.go 实现扩展管理器。
package extension

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sort"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

var globalRegistry = struct {
	mu         sync.RWMutex
	extensions map[string]Extension
}{
	extensions: make(map[string]Extension),
}

// RegisterExtension 注册扩展到全局注册表。
func RegisterExtension(ext Extension) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	id := ext.ID()
	if _, exists := globalRegistry.extensions[id]; exists {
		logger.Warn("[Extension] Overwriting existing global extension",
			zap.String("id", id))
	}

	globalRegistry.extensions[id] = ext
	logger.Debug("[Extension] Extension registered globally",
		zap.String("id", id))
}

// GetGlobalExtensions 获取全局注册的扩展列表。
func GetGlobalExtensions() []Extension {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	exts := make([]Extension, 0, len(globalRegistry.extensions))
	for _, ext := range globalRegistry.extensions {
		exts = append(exts, ext)
	}
	return exts
}

// Manager 扩展管理器。
type Manager struct {
	mu sync.RWMutex

	extensions map[string]Extension
	infos      map[string]*ExtensionInfo
	api        ExtensionAPI

	extensionDir string
	workingDir   string
	config       map[string]any
}

// NewManager 创建扩展管理器。
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{
		extensions: make(map[string]Extension),
		infos:      make(map[string]*ExtensionInfo),
		config:     make(map[string]any),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// ManagerOption 管理器选项。
type ManagerOption func(*Manager)

// WithExtensionDir 设置扩展目录。
func WithExtensionDir(dir string) ManagerOption {
	return func(m *Manager) {
		m.extensionDir = dir
	}
}

// WithWorkingDir 设置工作目录。
func WithWorkingDir(dir string) ManagerOption {
	return func(m *Manager) {
		m.workingDir = dir
	}
}

// WithAPI 设置扩展 API。
func WithAPI(api ExtensionAPI) ManagerOption {
	return func(m *Manager) {
		m.api = api
	}
}

// Register 注册扩展。
func (m *Manager) Register(ext Extension) error {
	if ext == nil {
		return fmt.Errorf("extension is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	id := ext.ID()
	if _, exists := m.extensions[id]; exists {
		logger.Warn("[ExtensionManager] Overwriting existing extension",
			zap.String("id", id))
	}

	m.extensions[id] = ext
	m.infos[id] = &ExtensionInfo{
		ID:          ext.ID(),
		Name:        ext.Name(),
		Description: ext.Description(),
		Version:     ext.Version(),
	}

	logger.Info("[ExtensionManager] Extension registered",
		zap.String("id", id),
		zap.String("name", ext.Name()),
		zap.String("version", ext.Version()))

	return nil
}

// Unregister 注销扩展。
func (m *Manager) Unregister(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ext, exists := m.extensions[id]
	if !exists {
		return fmt.Errorf("extension not found: %s", id)
	}

	ctx := context.Background()
	if err := ext.Shutdown(ctx); err != nil {
		logger.Warn("[ExtensionManager] Extension shutdown failed",
			zap.String("id", id),
			zap.Error(err))
	}

	delete(m.extensions, id)
	delete(m.infos, id)

	logger.Info("[ExtensionManager] Extension unregistered",
		zap.String("id", id))

	return nil
}

// Get 获取扩展。
func (m *Manager) Get(id string) (Extension, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ext, ok := m.extensions[id]
	return ext, ok
}

// List 列出所有扩展。
func (m *Manager) List() []Extension {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exts := make([]Extension, 0, len(m.extensions))
	for _, ext := range m.extensions {
		exts = append(exts, ext)
	}

	sort.Slice(exts, func(i, j int) bool {
		return exts[i].ID() < exts[j].ID()
	})

	return exts
}

// ListInfo 列出所有扩展信息。
func (m *Manager) ListInfo() []*ExtensionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]*ExtensionInfo, 0, len(m.infos))
	for _, info := range m.infos {
		infos = append(infos, info)
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ID < infos[j].ID
	})

	return infos
}

// StartupAll 启动所有扩展。
func (m *Manager) StartupAll(ctx context.Context) error {
	m.mu.RLock()
	exts := make([]Extension, 0, len(m.extensions))
	for _, ext := range m.extensions {
		exts = append(exts, ext)
	}
	m.mu.RUnlock()

	for _, ext := range exts {
		if err := ext.Startup(ctx); err != nil {
			return fmt.Errorf("failed to startup extension %s: %w", ext.ID(), err)
		}
	}

	return nil
}

// ShutdownAll 关闭所有扩展。
func (m *Manager) ShutdownAll(ctx context.Context) error {
	m.mu.RLock()
	exts := make([]Extension, 0, len(m.extensions))
	for _, ext := range m.extensions {
		exts = append(exts, ext)
	}
	m.mu.RUnlock()

	var errs []error
	for _, ext := range exts {
		if err := ext.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown extension %s: %w", ext.ID(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// RegisterAll 注册所有扩展到 API。
func (m *Manager) RegisterAll() error {
	m.mu.RLock()
	exts := make([]Extension, 0, len(m.extensions))
	for _, ext := range m.extensions {
		exts = append(exts, ext)
	}
	api := m.api
	m.mu.RUnlock()

	if api == nil {
		return fmt.Errorf("extension API not set")
	}

	for _, ext := range exts {
		if err := ext.Register(api); err != nil {
			return fmt.Errorf("failed to register extension %s: %w", ext.ID(), err)
		}
		logger.Info("[ExtensionManager] Extension components registered",
			zap.String("id", ext.ID()))
	}

	return nil
}

// LoadFromDir 从目录加载扩展。
// 支持 Go 插件（.so）和内建扩展。
func (m *Manager) LoadFromDir(dir string) error {
	if dir == "" {
		dir = m.extensionDir
	}
	if dir == "" {
		return nil
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read extension directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		extDir := filepath.Join(dir, entry.Name())

		if err := m.loadExtension(extDir); err != nil {
			logger.Warn("[ExtensionManager] Failed to load extension",
				zap.String("dir", extDir),
				zap.Error(err))
			continue
		}
	}

	return nil
}

// LoadGlobalExtensions 加载全局注册的扩展。
func (m *Manager) LoadGlobalExtensions() error {
	globalExts := GetGlobalExtensions()
	for _, ext := range globalExts {
		if err := m.Register(ext); err != nil {
			logger.Warn("[ExtensionManager] Failed to register global extension",
				zap.String("id", ext.ID()),
				zap.Error(err))
			continue
		}
	}
	return nil
}

// loadExtension 加载单个扩展目录。
func (m *Manager) loadExtension(extDir string) error {
	soPath := filepath.Join(extDir, "extension.so")
	if _, err := os.Stat(soPath); err == nil {
		return m.loadPluginExtension(soPath)
	}

	return fmt.Errorf("no valid extension found in %s", extDir)
}

// loadPluginExtension 加载 Go 插件扩展。
func (m *Manager) loadPluginExtension(soPath string) error {
	p, err := plugin.Open(soPath)
	if err != nil {
		return fmt.Errorf("failed to open plugin: %w", err)
	}

	sym, err := p.Lookup("Extension")
	if err != nil {
		return fmt.Errorf("failed to lookup Extension symbol: %w", err)
	}

	ext, ok := sym.(Extension)
	if !ok {
		return fmt.Errorf("Extension symbol does not implement Extension interface")
	}

	return m.Register(ext)
}

// ExtensionDir 返回扩展目录。
func (m *Manager) ExtensionDir() string {
	return m.extensionDir
}
