// Package bridge 提供消息处理的核心路由层
// agent_bridge_plugin.go 插件管理相关方法
package bridge

import (
	"github.com/bstr9/simpleclaw/pkg/plugin"
)

// GetPluginManager 返回插件管理器实例
func (ab *AgentBridge) GetPluginManager() *plugin.Manager {
	return ab.pluginMgr
}

// RegisterPlugin 向插件管理器注册插件
func (ab *AgentBridge) RegisterPlugin(p plugin.Plugin) error {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.Register(p)
}

// PublishEvent 向所有插件发布事件
func (ab *AgentBridge) PublishEvent(event plugin.Event, ctx *plugin.EventContext) error {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.PublishEvent(event, ctx)
}

// UnregisterPlugin 注销插件
func (ab *AgentBridge) UnregisterPlugin(name string) error {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.Unregister(name)
}

// LoadPlugin 加载插件
func (ab *AgentBridge) LoadPlugin(name string) error {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.LoadPlugin(name)
}

// UnloadPlugin 卸载插件
func (ab *AgentBridge) UnloadPlugin(name string) error {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.UnloadPlugin(name)
}

// ReloadPlugin 重载插件
func (ab *AgentBridge) ReloadPlugin(name string) bool {
	if ab.pluginMgr == nil {
		return false
	}
	return ab.pluginMgr.ReloadPlugin(name)
}

// GetPlugin 获取插件实例
func (ab *AgentBridge) GetPlugin(name string) (plugin.Plugin, bool) {
	if ab.pluginMgr == nil {
		return nil, false
	}
	return ab.pluginMgr.GetPlugin(name)
}

// ListPlugins 列出所有插件
func (ab *AgentBridge) ListPlugins() map[string]*plugin.Metadata {
	if ab.pluginMgr == nil {
		return nil
	}
	return ab.pluginMgr.ListPlugins()
}

// GetPluginMetadata 获取插件元数据
func (ab *AgentBridge) GetPluginMetadata(name string) (*plugin.Metadata, bool) {
	if ab.pluginMgr == nil {
		return nil, false
	}
	return ab.pluginMgr.GetMetadata(name)
}
