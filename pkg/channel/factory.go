// Package channel 提供消息渠道的抽象和管理。
// factory.go 定义了渠道工厂和注册机制。
package channel

import (
	"fmt"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/logger"
	"go.uber.org/zap"
)

// ChannelCreator 渠道创建函数类型
type ChannelCreator func() (Channel, error)

// channelRegistry 渠道注册表
var channelRegistry = struct {
	mu       sync.RWMutex
	creators map[string]ChannelCreator
}{
	creators: make(map[string]ChannelCreator),
}

// RegisterChannel 注册渠道创建函数
// name: 渠道类型名称（如 "web", "terminal", "feishu" 等）
// creator: 渠道创建函数
func RegisterChannel(name string, creator ChannelCreator) {
	channelRegistry.mu.Lock()
	defer channelRegistry.mu.Unlock()

	if _, exists := channelRegistry.creators[name]; exists {
		logger.Warn("[ChannelFactory] Overwriting existing channel registration",
			zap.String("channel", name))
	}

	channelRegistry.creators[name] = creator
	logger.Debug("[ChannelFactory] Channel registered",
		zap.String("channel", name))
}

// CreateChannel 创建渠道实例
// channelType: 渠道类型名称
// 返回: 渠道实例和错误信息
func CreateChannel(channelType string) (Channel, error) {
	channelRegistry.mu.RLock()
	creator, exists := channelRegistry.creators[channelType]
	channelRegistry.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown channel type: %s", channelType)
	}

	ch, err := creator()
	if err != nil {
		return nil, fmt.Errorf("failed to create channel '%s': %w", channelType, err)
	}

	return ch, nil
}

// IsChannelRegistered 检查渠道是否已注册
func IsChannelRegistered(name string) bool {
	channelRegistry.mu.RLock()
	defer channelRegistry.mu.RUnlock()
	_, exists := channelRegistry.creators[name]
	return exists
}

// GetRegisteredChannelTypes 返回所有已注册的渠道类型名称
func GetRegisteredChannelTypes() []string {
	channelRegistry.mu.RLock()
	defer channelRegistry.mu.RUnlock()

	types := make([]string, 0, len(channelRegistry.creators))
	for name := range channelRegistry.creators {
		types = append(types, name)
	}
	return types
}

// channelCache 渠道实例缓存
// 用于存储已创建的渠道实例，支持重启时复用
var channelCache = struct {
	mu        sync.RWMutex
	instances map[string]Channel
}{
	instances: make(map[string]Channel),
}

// ClearChannelCache 清除渠道缓存
// 在重启渠道时调用，确保使用新的配置
func ClearChannelCache(name string) {
	channelCache.mu.Lock()
	defer channelCache.mu.Unlock()

	if _, exists := channelCache.instances[name]; exists {
		delete(channelCache.instances, name)
		logger.Debug("[ChannelFactory] Channel cache cleared",
			zap.String("channel", name))
	}
}

// 预定义的渠道类型常量
const (
	// ChannelWeb Web 渠道
	ChannelWeb = "web"
	// ChannelTerminal 终端渠道
	ChannelTerminal = "terminal"
	// ChannelFeishu 飞书渠道
	ChannelFeishu = "feishu"
	// ChannelDingtalk 钉钉渠道
	ChannelDingtalk = "dingtalk"
	// ChannelWeixin 微信渠道
	ChannelWeixin = "weixin"
	// ChannelWeixinAlias 微信渠道别名
	ChannelWeixinAlias = "wx"
	// ChannelWechatMP 微信公众号渠道
	ChannelWechatMP = "wechatmp"
	// ChannelWechatComApp 企业微信应用渠道
	ChannelWechatComApp = "wechatcom_app"
	// ChannelWecomBot 企业微信机器人渠道
	ChannelWecomBot = "wecom_bot"
	// ChannelQQ QQ渠道
	ChannelQQ = "qq"
)

// init 初始化默认渠道注册
func init() {
	// 注册占位符创建函数
	// 具体实现由各渠道包在导入时注册

	// 注册空渠道创建函数，防止未注册时直接报错
	// 各渠道包应该在 init() 中覆盖这些注册
	placeholders := []string{
		ChannelWeb,
		ChannelTerminal,
		ChannelFeishu,
		ChannelDingtalk,
		ChannelWeixin,
		ChannelWeixinAlias,
		ChannelWechatMP,
		ChannelWechatComApp,
		ChannelWecomBot,
		ChannelQQ,
	}

	for _, name := range placeholders {
		channelName := name // 捕获循环变量
		RegisterChannel(name, func() (Channel, error) {
			return nil, fmt.Errorf("channel '%s' is not implemented yet", channelName)
		})
	}
}
