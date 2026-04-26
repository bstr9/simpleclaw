// Package channel 提供消息渠道的抽象和管理。
// manager.go 定义 ChannelManager 用于管理多个渠道的生命周期。
package channel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// ChannelManager 管理多个渠道的生命周期。
type ChannelManager struct {
	mu sync.RWMutex

	// channels 存储渠道实例，key 为渠道名称
	channels map[string]Channel

	// cancelFuncs 存储各渠道的取消函数
	cancelFuncs map[string]context.CancelFunc

	// primaryChannel 主渠道（第一个非 web 渠道）
	primaryChannel Channel

	// cloudMode 云模式标志
	cloudMode bool

	// wg 等待所有渠道退出
	wg sync.WaitGroup

	// shutdownSignals 关闭信号
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

	// processor 消息处理器，由 Bridge 层注入
	processor MessageProcessor

	// agentBridge Agent 桥接器，由 Bridge 层注入
	agentBridge AgentBridge
}

// NewChannelManager 创建渠道管理器
func NewChannelManager() *ChannelManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ChannelManager{
		channels:       make(map[string]Channel),
		cancelFuncs:    make(map[string]context.CancelFunc),
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
	}
}

var globalManager *ChannelManager
var globalManagerOnce sync.Once

// GetChannelManager 获取全局渠道管理器
func GetChannelManager() *ChannelManager {
	globalManagerOnce.Do(func() {
		if globalManager == nil {
			globalManager = NewChannelManager()
		}
	})
	return globalManager
}

// SetChannelManager 设置全局渠道管理器
func SetChannelManager(m *ChannelManager) {
	globalManagerOnce.Do(func() {
		globalManager = m
	})
}

// SetCloudMode 设置云模式
func (m *ChannelManager) SetCloudMode(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cloudMode = enabled
}

// IsCloudMode 检查是否为云模式
func (m *ChannelManager) IsCloudMode() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cloudMode
}

// PrimaryChannel 返回主渠道
func (m *ChannelManager) PrimaryChannel() Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.primaryChannel
}

// GetChannel 获取指定渠道
func (m *ChannelManager) GetChannel(name string) Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.channels[name]
}

// ListChannels 列出所有渠道名称
func (m *ChannelManager) ListChannels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.channels))
	for name := range m.channels {
		names = append(names, name)
	}
	return names
}

// ChannelCount 返回渠道数量
func (m *ChannelManager) ChannelCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.channels)
}

// StartOption 启动选项
type StartOption func(*startConfig)

type startConfig struct {
	firstStart    bool
	initPlugins   bool
	initCloud     bool
	processor     MessageProcessor
	agentBridge   AgentBridge
}

// WithFirstStart 设置首次启动标志
func WithFirstStart(first bool) StartOption {
	return func(c *startConfig) {
		c.firstStart = first
	}
}

// WithInitPlugins 设置是否初始化插件
func WithInitPlugins(init bool) StartOption {
	return func(c *startConfig) {
		c.initPlugins = init
	}
}

// WithMessageProcessor 设置消息处理器
func WithMessageProcessor(processor MessageProcessor) StartOption {
	return func(c *startConfig) {
		c.processor = processor
	}
}

// WithAgentBridge 设置 Agent 桥接器
func WithAgentBridge(ab AgentBridge) StartOption {
	return func(c *startConfig) {
		c.agentBridge = ab
	}
}

// Start 启动一个或多个渠道
// 渠道在独立的 goroutine 中运行
func (m *ChannelManager) Start(channelNames []string, opts ...StartOption) error {
	cfg := &startConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// 将注入的依赖设置到管理器
	if cfg.processor != nil {
		m.processor = cfg.processor
	}
	if cfg.agentBridge != nil {
		m.agentBridge = cfg.agentBridge
	}

	// 云服务初始化
	if cfg.initCloud && m.cloudMode {
		if err := m.initCloudServices(); err != nil {
			logger.Warn("[ChannelManager] Cloud initialization failed, continuing in local mode",
				zap.Error(err))
		} else {
			logger.Info("[ChannelManager] Cloud services initialized")
		}
	}

	// 创建所有渠道实例
	channels, err := m.createChannels(channelNames)
	if err != nil {
		return err
	}

	// 按 web 优先顺序启动渠道
	m.startOrderedChannels(channels)

	return nil
}

// createChannels 创建渠道实例并注册到管理器
func (m *ChannelManager) createChannels(channelNames []string) ([]struct {
	name    string
	channel Channel
}, error) {
	channels := make([]struct {
		name    string
		channel Channel
	}, 0, len(channelNames))

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, name := range channelNames {
		if _, exists := m.channels[name]; exists {
			return nil, fmt.Errorf("channel '%s' already exists", name)
		}

		ch, err := CreateChannel(name)
		if err != nil {
			return nil, fmt.Errorf("failed to create channel '%s': %w", name, err)
		}

		// 设置云模式
		if bc, ok := ch.(*BaseChannel); ok {
			bc.SetCloudMode(m.cloudMode)
		}

		m.channels[name] = ch
		channels = append(channels, struct {
			name    string
			channel Channel
		}{name: name, channel: ch})

		// 设置主渠道（第一个非 web 渠道）
		m.setPrimaryChannelIfNeeded(name, ch)
	}

	// 如果只有 web 渠道，则设置为主渠道
	if m.primaryChannel == nil && len(channels) > 0 {
		m.primaryChannel = channels[0].channel
	}

	return channels, nil
}

// setPrimaryChannelIfNeeded 设置主渠道（第一个非 web 渠道）
func (m *ChannelManager) setPrimaryChannelIfNeeded(name string, ch Channel) {
	if m.primaryChannel == nil && name != "web" {
		m.primaryChannel = ch
	}
}

// startOrderedChannels 按顺序启动渠道
func (m *ChannelManager) startOrderedChannels(channels []struct {
	name    string
	channel Channel
}) {
	ordered := m.orderChannels(channels)

	for i, entry := range ordered {
		// 非 web 渠道之间稍微延迟启动
		if i > 0 && entry.name != "web" {
			time.Sleep(100 * time.Millisecond)
		}

		// 启动渠道
		m.startChannel(entry.name, entry.channel)
		logger.Info("[ChannelManager] Channel started",
			zap.String("channel", entry.name))
	}
}

// orderChannels 排序渠道，web 渠道优先
func (m *ChannelManager) orderChannels(channels []struct {
	name    string
	channel Channel
}) []struct {
	name    string
	channel Channel
} {
	var webEntry *struct {
		name    string
		channel Channel
	}
	var others []struct {
		name    string
		channel Channel
	}

	for i := range channels {
		if channels[i].name == "web" {
			webEntry = &channels[i]
		} else {
			others = append(others, channels[i])
		}
	}

	ordered := make([]struct {
		name    string
		channel Channel
	}, 0, len(channels))
	if webEntry != nil {
		ordered = append(ordered, *webEntry)
	}
	ordered = append(ordered, others...)

	return ordered
}

// startChannel 在 goroutine 中启动渠道
func (m *ChannelManager) startChannel(name string, ch Channel) {
	ctx, cancel := context.WithCancel(m.shutdownCtx)

	m.mu.Lock()
	m.cancelFuncs[name] = cancel
	m.mu.Unlock()

	// 设置消息处理器（通过依赖注入）
	if setter, ok := ch.(MessageHandlerSetter); ok && m.processor != nil {
		setter.SetMessageHandler(m.processor)
	}

	// 设置 Agent 桥接器（通过依赖注入）
	if setter, ok := ch.(AgentBridgeSetter); ok && m.agentBridge != nil {
		setter.SetAgentBridge(m.agentBridge)
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				logger.Error("[ChannelManager] Channel panic",
					zap.String("channel", name),
					zap.Any("panic", r))
			}
		}()

		logger.Info("[ChannelManager] Starting channel",
			zap.String("channel", name))

		if err := ch.Startup(ctx); err != nil {
			logger.Error("[ChannelManager] Channel startup error",
				zap.String("channel", name),
				zap.Error(err))
		}

		logger.Info("[ChannelManager] Channel exited",
			zap.String("channel", name))
	}()
}

// MessageHandlerSetter 定义设置消息处理器的接口
type MessageHandlerSetter interface {
	SetMessageHandler(handler any)
}

// AgentBridgeSetter 定义设置 Agent 桥接器的接口
type AgentBridgeSetter interface {
	SetAgentBridge(ab AgentBridge)
}

// Stop 停止渠道
// 如果 name 为空，则停止所有渠道
func (m *ChannelManager) Stop(name string) {
	m.mu.Lock()

	var names []string
	if name != "" {
		names = []string{name}
	} else {
		names = make([]string, 0, len(m.channels))
		for n := range m.channels {
			names = append(names, n)
		}
	}

	toStop := make([]struct {
		name    string
		channel Channel
		cancel  context.CancelFunc
	}, 0, len(names))

	for _, n := range names {
		ch := m.channels[n]
		cancel := m.cancelFuncs[n]
		delete(m.channels, n)
		delete(m.cancelFuncs, n)

		if m.primaryChannel == ch {
			m.primaryChannel = nil
		}

		if ch != nil {
			toStop = append(toStop, struct {
				name    string
				channel Channel
				cancel  context.CancelFunc
			}{name: n, channel: ch, cancel: cancel})
		}
	}
	m.mu.Unlock()

	// 停止渠道
	for _, entry := range toStop {
		logger.Info("[ChannelManager] Stopping channel",
			zap.String("channel", entry.name))

		// 先取消上下文
		if entry.cancel != nil {
			entry.cancel()
		}

		// 调用 Stop 方法
		if err := entry.channel.Stop(); err != nil {
			logger.Warn("[ChannelManager] Channel stop error",
				zap.String("channel", entry.name),
				zap.Error(err))
		}
	}
}

// StopAll 停止所有渠道
func (m *ChannelManager) StopAll() {
	m.Stop("")
	m.wg.Wait()
}

// Shutdown 关闭管理器
// 停止所有渠道并等待退出
func (m *ChannelManager) Shutdown() {
	logger.Info("[ChannelManager] Shutting down...")
	m.shutdownCancel()
	m.StopAll()
	logger.Info("[ChannelManager] Shutdown complete")
}

// Restart 重启渠道
func (m *ChannelManager) Restart(name string) error {
	m.Stop(name)

	// 等待完全停止
	time.Sleep(time.Second)

	// 清除缓存（如果需要）
	ClearChannelCache(name)

	// 重新启动
	return m.Start([]string{name})
}

// AddChannel 动态添加渠道
func (m *ChannelManager) AddChannel(name string) error {
	m.mu.RLock()
	_, exists := m.channels[name]
	m.mu.RUnlock()

	if exists {
		logger.Info("[ChannelManager] Channel already exists, restarting",
			zap.String("channel", name))
		return m.Restart(name)
	}

	logger.Info("[ChannelManager] Adding channel",
		zap.String("channel", name))

	ClearChannelCache(name)
	return m.Start([]string{name})
}

// RemoveChannel 动态移除渠道
func (m *ChannelManager) RemoveChannel(name string) {
	m.mu.RLock()
	_, exists := m.channels[name]
	m.mu.RUnlock()

	if !exists {
		logger.Warn("[ChannelManager] Channel not found",
			zap.String("channel", name))
		return
	}

	logger.Info("[ChannelManager] Removing channel",
		zap.String("channel", name))
	m.Stop(name)
	logger.Info("[ChannelManager] Channel removed",
		zap.String("channel", name))
}

// Wait 等待所有渠道退出
func (m *ChannelManager) Wait() {
	m.wg.Wait()
}

// Context 获取关闭上下文
func (m *ChannelManager) Context() context.Context {
	return m.shutdownCtx
}

// initCloudServices 初始化云服务连接
// 包括：认证、配置同步、状态上报等
func (m *ChannelManager) initCloudServices() error {
	m.mu.RLock()
	cloudMode := m.cloudMode
	m.mu.RUnlock()

	if !cloudMode {
		return nil
	}

	logger.Info("[ChannelManager] Initializing cloud services...")

	// 待实现: 云服务初始化逻辑
	// 1. 云端认证
	// 2. 配置同步
	// 3. 状态上报

	return nil
}
