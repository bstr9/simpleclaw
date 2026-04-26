// Package linkai 提供 LinkAI 集成插件，支持知识库、Midjourney绘画、文档总结等功能。
// 本文件包含知识库和群聊应用管理相关的逻辑。
package linkai

import (
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/plugin"
	"github.com/bstr9/simpleclaw/pkg/types"
)

// handleGroupAppIfNeeded 处理群聊应用消息（如果需要）。
func (p *LinkAIPlugin) handleGroupAppIfNeeded(ec *plugin.EventContext) error {
	isGroup, _ := ec.GetBool("is_group")
	if isGroup && len(p.config.GroupAppMap) > 0 {
		return p.handleGroupAppMessage(ec)
	}
	return nil
}

// handleGroupAppMessage 处理群聊应用消息
func (p *LinkAIPlugin) handleGroupAppMessage(ec *plugin.EventContext) error {
	groupName, _ := ec.GetString("group_name")

	// 查找群聊对应的应用编码
	p.mu.RLock()
	appCode, exists := p.config.GroupAppMap[groupName]
	p.mu.RUnlock()

	if !exists {
		// 检查是否有全局设置
		p.mu.RLock()
		appCode, exists = p.config.GroupAppMap["ALL_GROUP"]
		p.mu.RUnlock()
	}

	if exists && appCode != "" {
		// 设置应用编码到上下文
		ec.Set("app_code", appCode)
	}

	return nil
}

// handleAdminAppCmd 处理 linkai app 命令
func (p *LinkAIPlugin) handleAdminAppCmd(ec *plugin.EventContext, parts []string) error {
	isGroup, _ := ec.GetBool("is_group")
	isAdmin, _ := ec.GetBool("is_admin")

	if !isGroup {
		reply := types.NewErrorReply("该指令需在群聊中使用")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}
	if !isAdmin {
		reply := types.NewErrorReply(errAdminReq)
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}
	if len(parts) < 3 {
		reply := types.NewErrorReply("请提供应用编码")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}
	appCode := parts[2]
	groupName, _ := ec.GetString("group_name")
	p.mu.Lock()
	if p.config.GroupAppMap == nil {
		p.config.GroupAppMap = make(map[string]string)
	}
	p.config.GroupAppMap[groupName] = appCode
	p.mu.Unlock()
	reply := types.NewInfoReply("应用设置成功: " + appCode)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")
	return nil
}

// fetchGroupAppCode 获取群聊对应的应用编码
func (p *LinkAIPlugin) fetchGroupAppCode(groupName string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.config.GroupAppMap == nil {
		return ""
	}

	if appCode, ok := p.config.GroupAppMap[groupName]; ok {
		return appCode
	}

	// 检查全局设置
	if appCode, ok := p.config.GroupAppMap["ALL_GROUP"]; ok {
		return appCode
	}

	return ""
}

// fetchAppCode 获取应用编码
func (p *LinkAIPlugin) fetchAppCode(ec *plugin.EventContext) string {
	// 优先从上下文获取
	if appCode, ok := ec.GetString("app_code"); ok && appCode != "" {
		return appCode
	}

	isGroup, _ := ec.GetBool("is_group")
	if isGroup {
		groupName, _ := ec.GetString("group_name")
		if appCode := p.fetchGroupAppCode(groupName); appCode != "" {
			return appCode
		}
	}

	// 使用全局配置
	cfg := config.Get()
	return cfg.LinkAIAppCode
}
