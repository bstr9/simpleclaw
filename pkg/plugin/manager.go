// Package plugin 提供插件系统，用于扩展 simpleclaw 应用程序。
// manager.go 定义 PluginManager 用于管理多个插件的生命周期。
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// Manager 插件管理器，负责插件的生命周期管理
type Manager struct {
	mu sync.RWMutex

	// plugins 插件实例映射，key 为插件名称
	plugins map[string]Plugin

	// metadata 插件元数据映射，key 为插件名称
	metadata map[string]*Metadata

	// contexts 插件上下文映射，key 为插件名称
	contexts map[string]*PluginContext

	// eventBus 全局事件总线
	eventBus *EventBus

	// pluginDir 插件目录
	pluginDir string
}

// 全局插件管理器实例
var (
	globalManager *Manager
	managerOnce   sync.Once
)

// GetManager 获取全局插件管理器实例
func GetManager() *Manager {
	managerOnce.Do(func() {
		globalManager = &Manager{
			plugins:   make(map[string]Plugin),
			metadata:  make(map[string]*Metadata),
			contexts:  make(map[string]*PluginContext),
			eventBus:  NewEventBus(),
			pluginDir: "./plugins",
		}
	})
	return globalManager
}

// SetPluginDir 设置插件目录
func (m *Manager) SetPluginDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pluginDir = dir
}

// GetPluginDir 获取插件目录
func (m *Manager) GetPluginDir() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pluginDir
}

// Register 注册插件
func (m *Manager) Register(p Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := p.Name()
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("插件 %s 已注册", name)
	}

	m.plugins[name] = p

	// 获取或创建元数据
	if bp, ok := p.(*BasePlugin); ok {
		m.metadata[name] = bp.GetMetadata()
	} else {
		m.metadata[name] = &Metadata{
			Name:     name,
			Version:  p.Version(),
			Enabled:  true,
			Priority: 0,
		}
	}

	logger.Info("[PluginManager] 插件已注册",
		zap.String("name", name),
		zap.String("version", p.Version()))

	return nil
}

// Unregister 注销插件
func (m *Manager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf(common.ErrPluginNotFoundFmt, name)
	}

	// 调用卸载
	if ctx, ok := m.contexts[name]; ok {
		if err := p.OnUnload(ctx); err != nil {
			logger.Warn("[PluginManager] 插件卸载失败",
				zap.String("name", name),
				zap.Error(err))
		}
	}

	delete(m.plugins, name)
	delete(m.metadata, name)
	delete(m.contexts, name)

	logger.Info("[PluginManager] 插件已注销", zap.String("name", name))
	return nil
}

// InitPlugin 初始化插件
func (m *Manager) InitPlugin(name string, pluginPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf(common.ErrPluginNotFoundFmt, name)
	}

	// 创建插件上下文
	ctx := &PluginContext{
		PluginPath: pluginPath,
		PluginName: name,
		PluginDir:  m.pluginDir,
		EventBus:   m.eventBus,
	}

	m.contexts[name] = ctx

	// 调用初始化
	if err := p.OnInit(ctx); err != nil {
		return fmt.Errorf("插件 %s 初始化失败: %w", name, err)
	}

	logger.Info("[PluginManager] 插件已初始化", zap.String("name", name))
	return nil
}

// LoadPlugin 加载插件
func (m *Manager) LoadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf(common.ErrPluginNotFoundFmt, name)
	}

	ctx, exists := m.contexts[name]
	if !exists {
		return fmt.Errorf("插件 %s 未初始化", name)
	}

	// 调用加载
	if err := p.OnLoad(ctx); err != nil {
		return fmt.Errorf("插件 %s 加载失败: %w", name, err)
	}

	// 更新元数据
	if meta, ok := m.metadata[name]; ok {
		meta.Enabled = true
	}

	logger.Info("[PluginManager] 插件已加载", zap.String("name", name))
	return nil
}

// UnloadPlugin 卸载插件
func (m *Manager) UnloadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf(common.ErrPluginNotFoundFmt, name)
	}

	ctx, exists := m.contexts[name]
	if !exists {
		return nil
	}

	// 调用卸载
	if err := p.OnUnload(ctx); err != nil {
		return fmt.Errorf("插件 %s 卸载失败: %w", name, err)
	}

	// 更新元数据
	if meta, ok := m.metadata[name]; ok {
		meta.Enabled = false
	}

	logger.Info("[PluginManager] 插件已卸载", zap.String("name", name))
	return nil
}

// ReloadPlugin 重载插件
func (m *Manager) ReloadPlugin(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.plugins[name]
	if !exists {
		return false
	}

	ctx, exists := m.contexts[name]
	if !exists {
		return false
	}

	// 先卸载
	_ = p.OnUnload(ctx)

	// 重新加载
	if err := p.OnLoad(ctx); err != nil {
		logger.Warn("[PluginManager] 插件重载失败",
			zap.String("name", name),
			zap.Error(err))
		return false
	}

	logger.Info("[PluginManager] 插件已重载", zap.String("name", name))
	return true
}

// GetPlugin 获取插件实例
func (m *Manager) GetPlugin(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[name]
	return p, ok
}

// GetMetadata 获取插件元数据
func (m *Manager) GetMetadata(name string) (*Metadata, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	meta, ok := m.metadata[name]
	return meta, ok
}

// ListPlugins 列出所有插件
func (m *Manager) ListPlugins() map[string]*Metadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Metadata)
	for name, meta := range m.metadata {
		result[name] = meta
	}
	return result
}

// ListEnabledPlugins 列出已启用的插件
func (m *Manager) ListEnabledPlugins() []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Plugin
	for name, p := range m.plugins {
		if meta, ok := m.metadata[name]; ok && meta.Enabled {
			result = append(result, p)
		}
	}

	// 按优先级排序（优先级越高越先执行）
	sort.Slice(result, func(i, j int) bool {
		nameI := result[i].Name()
		nameJ := result[j].Name()
		priI := m.metadata[nameI].Priority
		priJ := m.metadata[nameJ].Priority
		return priI > priJ
	})

	return result
}

// EnablePlugin 启用插件
func (m *Manager) EnablePlugin(name string) (bool, string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	meta, exists := m.metadata[name]
	if !exists {
		return false, common.ErrPluginNotExist
	}

	if meta.Enabled {
		return true, "插件已处于启用状态"
	}

	p := m.plugins[name]
	ctx, hasCtx := m.contexts[name]

	if hasCtx {
		if err := p.OnLoad(ctx); err != nil {
			return false, "插件启用失败: " + err.Error()
		}
	}

	meta.Enabled = true
	logger.Info("[PluginManager] 插件已启用", zap.String("name", name))
	return true, "插件已启用"
}

// DisablePlugin 禁用插件
func (m *Manager) DisablePlugin(name string) (bool, string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	meta, exists := m.metadata[name]
	if !exists {
		return false, common.ErrPluginNotExist
	}

	if !meta.Enabled {
		return true, "插件已处于禁用状态"
	}

	p := m.plugins[name]
	ctx, hasCtx := m.contexts[name]

	if hasCtx {
		if err := p.OnUnload(ctx); err != nil {
			logger.Warn("[PluginManager] 插件禁用时卸载失败",
				zap.String("name", name),
				zap.Error(err))
		}
	}

	meta.Enabled = false
	logger.Info("[PluginManager] 插件已禁用", zap.String("name", name))
	return true, "插件已禁用"
}

// SetPluginPriority 设置插件优先级
func (m *Manager) SetPluginPriority(name string, priority int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	meta, exists := m.metadata[name]
	if !exists {
		return false
	}

	meta.Priority = priority
	logger.Info("[PluginManager] 插件优先级已设置",
		zap.String("name", name),
		zap.Int("priority", priority))
	return true
}

// ScanPlugins 扫描插件目录
func (m *Manager) ScanPlugins() []*Metadata {
	m.mu.Lock()
	defer m.mu.Unlock()

	var newPlugins []*Metadata

	// 检查插件目录是否存在
	if _, err := os.Stat(m.pluginDir); os.IsNotExist(err) {
		logger.Warn("[PluginManager] 插件目录不存在", zap.String("dir", m.pluginDir))
		return newPlugins
	}

	// 遍历插件目录
	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		logger.Warn("[PluginManager] 读取插件目录失败", zap.Error(err))
		return newPlugins
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginName := entry.Name()
		pluginPath := filepath.Join(m.pluginDir, pluginName)

		// 检查是否已注册
		if _, exists := m.plugins[pluginName]; exists {
			continue
		}

		// 检查插件配置文件
		configPath := filepath.Join(pluginPath, "config.json")
		if _, err := os.Stat(configPath); err == nil {
			meta := &Metadata{
				Name:    pluginName,
				Path:    pluginPath,
				Enabled: false,
			}
			m.metadata[pluginName] = meta
			newPlugins = append(newPlugins, meta)

			logger.Info("[PluginManager] 发现新插件",
				zap.String("name", pluginName),
				zap.String("path", pluginPath))
		}
	}

	return newPlugins
}

// PublishEvent 发布事件到所有启用的插件
func (m *Manager) PublishEvent(event Event, ec *EventContext) error {
	// ListEnabledPlugins 自带 RLock 保护，无需在此额外加锁
	plugins := m.ListEnabledPlugins()

	for _, p := range plugins {
		if err := p.OnEvent(event, ec); err != nil {
			logger.Warn("[PluginManager] 插件事件处理失败",
				zap.String("plugin", p.Name()),
				zap.String("event", event.String()),
				zap.Error(err))
		}

		// 检查是否需要中断事件链
		if ec.IsBreak() {
			break
		}
	}

	return nil
}

// PluginCount 返回插件数量
func (m *Manager) PluginCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins)
}

// EnabledPluginCount 返回已启用插件数量
func (m *Manager) EnabledPluginCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, meta := range m.metadata {
		if meta.Enabled {
			count++
		}
	}
	return count
}
