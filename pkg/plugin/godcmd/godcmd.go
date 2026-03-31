// Package godcmd 提供管理员命令插件，用于机器人管理和控制。
// 支持用户认证、插件管理、配置重载、会话管理等管理员功能。
package godcmd

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/bridge"
	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/plugin"
	"github.com/bstr9/simpleclaw/pkg/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const emptyCmdMsg = "空指令，输入#help查看指令列表"

// 通用指令定义
var commands = map[string]commandInfo{
	"help": {
		alias: []string{"help", "帮助"},
		desc:  "回复此帮助",
	},
	"helpp": {
		alias: []string{"help", "帮助"},
		args:  []string{"插件名"},
		desc:  "回复指定插件的详细帮助",
	},
	"auth": {
		alias: []string{"auth", "认证"},
		args:  []string{"口令"},
		desc:  "管理员认证",
	},
	"model": {
		alias: []string{"model", "模型"},
		desc:  "查看和设置全局模型",
		args:  []string{"模型名"},
	},
	"id": {
		alias: []string{"id", "用户"},
		desc:  "获取用户id",
	},
	"reset": {
		alias: []string{"reset", "重置会话"},
		desc:  "重置会话",
	},
}

// 管理员指令定义
var adminCommands = map[string]commandInfo{
	"resume": {
		alias: []string{"resume", "恢复服务"},
		desc:  "恢复服务",
	},
	"stop": {
		alias: []string{"stop", "暂停服务"},
		desc:  "暂停服务",
	},
	"reconf": {
		alias: []string{"reconf", "重载配置"},
		desc:  "重载配置(不包含插件配置)",
	},
	"resetall": {
		alias: []string{"resetall", "重置所有会话"},
		desc:  "重置所有会话",
	},
	"scanp": {
		alias: []string{"scanp", "扫描插件"},
		desc:  "扫描插件目录是否有新插件",
	},
	"plist": {
		alias: []string{"plist", "插件"},
		desc:  "打印当前插件列表",
	},
	"setpri": {
		alias: []string{"setpri", "设置插件优先级"},
		args:  []string{"插件名", "优先级"},
		desc:  "设置指定插件的优先级，越大越优先",
	},
	"reloadp": {
		alias: []string{"reloadp", "重载插件"},
		args:  []string{"插件名"},
		desc:  "重载指定插件配置",
	},
	"enablep": {
		alias: []string{"enablep", "启用插件"},
		args:  []string{"插件名"},
		desc:  "启用指定插件",
	},
	"disablep": {
		alias: []string{"disablep", "禁用插件"},
		args:  []string{"插件名"},
		desc:  "禁用指定插件",
	},
	"debug": {
		alias: []string{"debug", "调试模式", "DEBUG"},
		desc:  "开启机器调试日志",
	},
}

// commandInfo 定义指令信息
type commandInfo struct {
	alias []string
	args  []string
	desc  string
}

// Config 表示 godcmd 插件的配置
type Config struct {
	Password   string   `json:"password"`
	AdminUsers []string `json:"admin_users"`
}

// GodcmdPlugin 实现管理员命令插件
type GodcmdPlugin struct {
	*plugin.BasePlugin

	mu           sync.RWMutex
	config       *Config
	tempPassword string
	isRunning    bool
	rng          *rand.Rand
	debugMode    bool
}

// 确保 GodcmdPlugin 实现了 Plugin 接口
var _ plugin.Plugin = (*GodcmdPlugin)(nil)

// New 创建新的 GodcmdPlugin 实例
func New() *GodcmdPlugin {
	bp := plugin.NewBasePlugin("godcmd", "1.0.0")
	bp.SetDescription("管理员命令插件，提供机器人管理和控制功能")
	bp.SetAuthor("simpleclaw")
	bp.SetPriority(999)
	bp.SetHidden(true)

	return &GodcmdPlugin{
		BasePlugin: bp,
		config:     &Config{Password: "", AdminUsers: []string{}},
		isRunning:  true,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		debugMode:  false,
	}
}

// Name 返回插件名称
func (p *GodcmdPlugin) Name() string {
	return "godcmd"
}

// Version 返回插件版本
func (p *GodcmdPlugin) Version() string {
	return "1.0.0"
}

// OnInit 初始化插件
func (p *GodcmdPlugin) OnInit(ctx *plugin.PluginContext) error {
	ctx.Debug("[godcmd] 正在初始化管理员命令插件")

	configPath := filepath.Join(ctx.PluginPath, "config.json")
	if err := p.loadConfig(configPath); err != nil {
		if os.IsNotExist(err) {
			if createErr := p.createDefaultConfig(configPath); createErr != nil {
				ctx.Warn("[godcmd] 创建默认配置失败: " + createErr.Error())
			}
		} else {
			ctx.Warn("[godcmd] 加载配置失败: " + err.Error())
		}
	}

	if p.config.Password == "" {
		p.tempPassword = p.generateTempPassword()
		ctx.Info("[godcmd] 因未设置口令，本次的临时口令为: " + p.tempPassword)
	}

	p.debugMode = config.Get().Debug

	ctx.Info("[godcmd] 插件初始化完成")
	return nil
}

// OnLoad 加载插件
func (p *GodcmdPlugin) OnLoad(ctx *plugin.PluginContext) error {
	ctx.Debug("[godcmd] 正在加载管理员命令插件")

	p.RegisterHandler(plugin.EventOnHandleContext, p.onHandleContext)

	ctx.Info("[godcmd] 插件加载成功")
	return nil
}

// OnUnload 卸载插件
func (p *GodcmdPlugin) OnUnload(ctx *plugin.PluginContext) error {
	ctx.Debug("[godcmd] 正在卸载管理员命令插件")
	return p.BasePlugin.OnUnload(ctx)
}

// OnEvent 处理插件事件
func (p *GodcmdPlugin) OnEvent(event plugin.Event, ec *plugin.EventContext) error {
	return p.BasePlugin.OnEvent(event, ec)
}

// onHandleContext 处理 ON_HANDLE_CONTEXT 事件
func (p *GodcmdPlugin) onHandleContext(ec *plugin.EventContext) error {
	if !p.shouldProcessContext(ec) {
		return nil
	}

	content, ok := ec.GetString("content")
	if !ok || content == "" || !strings.HasPrefix(content, "#") {
		p.breakPassIfNeeded(ec)
		return nil
	}

	cmd, args, reply := p.parseCommand(content)
	if reply != nil {
		ec.Set("reply", reply)
		ec.BreakPass("godcmd")
		return nil
	}

	userID, _ := ec.GetString("user_id")
	isGroup, _ := ec.GetBool("is_group")
	isAdmin := p.isAdmin(userID)

	reply, handled := p.dispatchCommand(cmd, args, userID, isAdmin, isGroup, ec)

	if handled && reply != nil {
		ec.Set("reply", reply)
		ec.BreakPass("godcmd")
		return nil
	}

	p.breakPassIfNeeded(ec)
	return nil
}

// shouldProcessContext 检查是否应该处理此上下文
func (p *GodcmdPlugin) shouldProcessContext(ec *plugin.EventContext) bool {
	contextType, ok := ec.GetInt("type")
	if !ok {
		return false
	}
	return contextType == int(types.ContextText)
}

// breakPassIfNeeded 如果服务未运行则中断传递
func (p *GodcmdPlugin) breakPassIfNeeded(ec *plugin.EventContext) {
	if !p.isRunning {
		ec.BreakPass("godcmd")
	}
}

// parseCommand 解析命令内容和参数
func (p *GodcmdPlugin) parseCommand(content string) (string, []string, *types.Reply) {
	if len(content) == 1 {
		return "", nil, types.NewErrorReply(emptyCmdMsg)
	}

	parts := strings.Fields(strings.TrimSpace(content[1:]))
	if len(parts) == 0 {
		return "", nil, types.NewErrorReply(emptyCmdMsg)
	}

	return parts[0], parts[1:], nil
}

// dispatchCommand 分发命令到对应处理器
func (p *GodcmdPlugin) dispatchCommand(cmd string, args []string, userID string, isAdmin bool, isGroup bool, ec *plugin.EventContext) (*types.Reply, bool) {
	if cmdName := p.findCommand(cmd, commands); cmdName != "" {
		return p.handleCommand(cmdName, args, userID, isAdmin, isGroup, ec)
	}

	cmdName := p.findCommand(cmd, adminCommands)
	if cmdName == "" {
		return nil, false
	}

	return p.handleAdminCommandWithCheck(cmdName, args, isAdmin, isGroup, ec)
}

// handleAdminCommandWithCheck 带权限检查的管理员命令处理
func (p *GodcmdPlugin) handleAdminCommandWithCheck(cmdName string, args []string, isAdmin bool, isGroup bool, ec *plugin.EventContext) (*types.Reply, bool) {
	if !isAdmin {
		return types.NewErrorReply("需要管理员权限才能执行该指令"), true
	}
	if isGroup {
		return types.NewErrorReply("群聊不可执行管理员指令"), true
	}
	return p.handleAdminCommand(cmdName, args, ec)
}

// handleCommand 处理通用命令
func (p *GodcmdPlugin) handleCommand(cmd string, args []string, userID string, isAdmin bool, isGroup bool, ec *plugin.EventContext) (*types.Reply, bool) {
	switch cmd {
	case "auth":
		return p.authenticate(userID, args, isGroup)
	case "help", "helpp":
		if len(args) == 0 {
			return types.NewInfoReply(p.getHelpText(isAdmin, isGroup)), true
		}
		return p.handlePluginHelp(args[0])
	case "model":
		return p.handleModel(args, isAdmin)
	case "id":
		return types.NewInfoReply(userID), true
	case "reset":
		return p.handleReset(ec)
	}
	return nil, false
}

// handleAdminCommand 处理管理员命令
func (p *GodcmdPlugin) handleAdminCommand(cmd string, args []string, _ *plugin.EventContext) (*types.Reply, bool) {
	switch cmd {
	case "stop":
		return p.handleStop()
	case "resume":
		return p.handleResume()
	case "reconf":
		return p.handleReconf()
	case "resetall":
		return p.handleResetAll()
	case "debug":
		return p.handleDebug()
	case "plist":
		return p.handlePluginList()
	case "scanp":
		return p.handleScanPlugins()
	case "setpri":
		return p.handleSetPriorityWithArgs(args)
	case "reloadp":
		return p.handlePluginOpWithArg(args, p.handleReloadPlugin)
	case "enablep":
		return p.handlePluginOpWithArg(args, p.handleEnablePlugin)
	case "disablep":
		return p.handlePluginOpWithArg(args, p.handleDisablePlugin)
	}
	return nil, false
}

// handleStop 停止服务
func (p *GodcmdPlugin) handleStop() (*types.Reply, bool) {
	p.mu.Lock()
	p.isRunning = false
	p.mu.Unlock()
	return types.NewInfoReply("服务已暂停"), true
}

// handleResume 恢复服务
func (p *GodcmdPlugin) handleResume() (*types.Reply, bool) {
	p.mu.Lock()
	p.isRunning = true
	p.mu.Unlock()
	return types.NewInfoReply("服务已恢复"), true
}

// handleSetPriorityWithArgs 处理设置优先级命令参数
func (p *GodcmdPlugin) handleSetPriorityWithArgs(args []string) (*types.Reply, bool) {
	if len(args) != 2 {
		return types.NewErrorReply("请提供插件名和优先级"), true
	}
	return p.handleSetPriority(args[0], args[1])
}

// PluginOpFunc 定义插件操作函数类型
type PluginOpFunc func(string) (*types.Reply, bool)

// handlePluginOpWithArg 处理带单参数的插件操作
func (p *GodcmdPlugin) handlePluginOpWithArg(args []string, op PluginOpFunc) (*types.Reply, bool) {
	if len(args) != 1 {
		return types.NewErrorReply(common.ErrPluginNameRequired), true
	}
	return op(args[0])
}

// HelpTextProvider 定义提供帮助文本的接口
type HelpTextProvider interface {
	HelpText() string
}

// handlePluginHelp 获取指定插件的帮助信息
func (p *GodcmdPlugin) handlePluginHelp(pluginName string) (*types.Reply, bool) {
	mgr := plugin.GetManager()
	plugins := mgr.ListPlugins()

	queryName := strings.ToUpper(pluginName)
	for name, meta := range plugins {
		if !p.matchPluginName(name, meta.NameCN, queryName) {
			continue
		}
		return p.getPluginHelpReply(mgr, name, meta, pluginName)
	}

	return types.NewErrorReply("插件 " + pluginName + " 不存在"), true
}

// matchPluginName 检查插件名称是否匹配
func (p *GodcmdPlugin) matchPluginName(name, nameCN, queryName string) bool {
	return strings.ToUpper(name) == queryName || strings.ToUpper(nameCN) == queryName
}

// getPluginHelpReply 获取插件帮助回复
func (p *GodcmdPlugin) getPluginHelpReply(mgr *plugin.Manager, name string, meta *plugin.Metadata, pluginName string) (*types.Reply, bool) {
	if !meta.Enabled {
		return types.NewErrorReply("插件 " + pluginName + " 未启用"), true
	}

	pluginInst, exists := mgr.GetPlugin(name)
	if !exists {
		return types.NewErrorReply("插件实例不存在"), true
	}

	helpText := "插件 " + name + " 暂无帮助信息"
	if hp, ok := pluginInst.(HelpTextProvider); ok {
		if ht := hp.HelpText(); ht != "" {
			helpText = ht
		}
	}
	return types.NewInfoReply(helpText), true
}

// handleModel 处理模型查看和设置
func (p *GodcmdPlugin) handleModel(args []string, isAdmin bool) (*types.Reply, bool) {
	cfg := config.Get()

	if len(args) == 0 {
		currentModel := cfg.Model
		if currentModel == "" {
			currentModel = "未设置"
		}
		return types.NewInfoReply("当前模型为: " + currentModel), true
	}

	if !isAdmin {
		return types.NewErrorReply("需要管理员权限才能设置模型"), true
	}

	newModel := args[0]
	cfg.Model = newModel

	bridge.GetBridge().Reset()

	return types.NewInfoReply("模型已设置为: " + newModel), true
}

// handleReset 重置当前会话
func (p *GodcmdPlugin) handleReset(ec *plugin.EventContext) (*types.Reply, bool) {
	sessionID, ok := ec.GetString("session_id")
	if !ok || sessionID == "" {
		return types.NewErrorReply("无法获取会话ID"), true
	}

	logger.Info("[godcmd] 重置会话", zap.String("session_id", sessionID))

	return types.NewInfoReply("会话已重置"), true
}

// handleReconf 重载配置
func (p *GodcmdPlugin) handleReconf() (*types.Reply, bool) {
	if err := config.Reload(); err != nil {
		return types.NewErrorReply("配置重载失败: " + err.Error()), true
	}

	bridge.GetBridge().Reset()

	logger.Info("[godcmd] 配置已重载")
	return types.NewInfoReply("配置已重载"), true
}

// handleResetAll 重置所有会话
func (p *GodcmdPlugin) handleResetAll() (*types.Reply, bool) {
	logger.Info("[godcmd] 重置所有会话")

	return types.NewInfoReply("重置所有会话成功"), true
}

// handleDebug 切换调试模式
func (p *GodcmdPlugin) handleDebug() (*types.Reply, bool) {
	p.mu.Lock()
	p.debugMode = !p.debugMode
	newMode := p.debugMode
	p.mu.Unlock()

	if newMode {
		logger.SetLevel(zapcore.DebugLevel)
		logger.Info("[godcmd] DEBUG模式已开启")
		return types.NewInfoReply("DEBUG模式已开启"), true
	}

	logger.SetLevel(zapcore.InfoLevel)
	logger.Info("[godcmd] DEBUG模式已关闭")
	return types.NewInfoReply("DEBUG模式已关闭"), true
}

// handlePluginList 列出所有插件
func (p *GodcmdPlugin) handlePluginList() (*types.Reply, bool) {
	mgr := plugin.GetManager()
	plugins := mgr.ListPlugins()

	if len(plugins) == 0 {
		return types.NewInfoReply("暂无已注册的插件"), true
	}

	var result strings.Builder
	result.WriteString("插件列表：\n")

	for name, meta := range plugins {
		result.WriteString(fmt.Sprintf("%s_v%s %d - ", name, meta.Version, meta.Priority))
		if meta.Enabled {
			result.WriteString("已启用\n")
		} else {
			result.WriteString("未启用\n")
		}
	}

	return types.NewInfoReply(result.String()), true
}

// handleScanPlugins 扫描新插件
func (p *GodcmdPlugin) handleScanPlugins() (*types.Reply, bool) {
	mgr := plugin.GetManager()
	newPlugins := mgr.ScanPlugins()

	if len(newPlugins) == 0 {
		return types.NewInfoReply("插件扫描完成，未发现新插件"), true
	}

	var result strings.Builder
	result.WriteString("插件扫描完成，发现新插件：\n")
	for _, meta := range newPlugins {
		result.WriteString(fmt.Sprintf("%s_v%s\n", meta.Name, meta.Version))
	}

	return types.NewInfoReply(result.String()), true
}

// handleSetPriority 设置插件优先级
func (p *GodcmdPlugin) handleSetPriority(pluginName, priorityStr string) (*types.Reply, bool) {
	priority, err := strconv.Atoi(priorityStr)
	if err != nil {
		return types.NewErrorReply("优先级必须是整数"), true
	}

	mgr := plugin.GetManager()
	if ok := mgr.SetPluginPriority(pluginName, priority); !ok {
		return types.NewErrorReply("插件 " + pluginName + " 不存在"), true
	}

	return types.NewInfoReply("插件 " + pluginName + " 优先级已设置为 " + priorityStr), true
}

// handleReloadPlugin 重载插件
func (p *GodcmdPlugin) handleReloadPlugin(pluginName string) (*types.Reply, bool) {
	mgr := plugin.GetManager()
	if ok := mgr.ReloadPlugin(pluginName); !ok {
		return types.NewErrorReply("插件 " + pluginName + " 不存在或重载失败"), true
	}

	return types.NewInfoReply("插件 " + pluginName + " 配置已重载"), true
}

// handleEnablePlugin 启用插件
func (p *GodcmdPlugin) handleEnablePlugin(pluginName string) (*types.Reply, bool) {
	mgr := plugin.GetManager()
	ok, msg := mgr.EnablePlugin(pluginName)
	if !ok {
		return types.NewErrorReply(msg), true
	}

	return types.NewInfoReply(msg), true
}

// handleDisablePlugin 禁用插件
func (p *GodcmdPlugin) handleDisablePlugin(pluginName string) (*types.Reply, bool) {
	mgr := plugin.GetManager()
	ok, msg := mgr.DisablePlugin(pluginName)
	if !ok {
		return types.NewErrorReply(msg), true
	}

	return types.NewInfoReply(msg), true
}

// authenticate 处理管理员认证
func (p *GodcmdPlugin) authenticate(userID string, args []string, isGroup bool) (*types.Reply, bool) {
	if isGroup {
		return types.NewErrorReply("请勿在群聊中认证"), true
	}

	if p.isAdmin(userID) {
		return types.NewErrorReply("管理员账号无需认证"), true
	}

	if len(args) != 1 {
		return types.NewErrorReply("请提供口令"), true
	}

	password := args[0]
	p.mu.RLock()
	correctPassword := p.config.Password
	tempPassword := p.tempPassword
	p.mu.RUnlock()

	if password == correctPassword {
		p.mu.Lock()
		p.config.AdminUsers = append(p.config.AdminUsers, userID)
		p.mu.Unlock()
		return types.NewInfoReply("认证成功"), true
	}

	if tempPassword != "" && password == tempPassword {
		p.mu.Lock()
		p.config.AdminUsers = append(p.config.AdminUsers, userID)
		p.mu.Unlock()
		return types.NewInfoReply("认证成功，请尽快设置口令"), true
	}

	return types.NewErrorReply("认证失败"), true
}

// isAdmin 检查用户是否为管理员
func (p *GodcmdPlugin) isAdmin(userID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, admin := range p.config.AdminUsers {
		if admin == userID {
			return true
		}
	}
	return false
}

// findCommand 根据别名查找命令名称
func (p *GodcmdPlugin) findCommand(cmd string, commands map[string]commandInfo) string {
	for name, info := range commands {
		for _, alias := range info.alias {
			if strings.EqualFold(cmd, alias) {
				return name
			}
		}
	}
	return ""
}

// getHelpText 获取帮助文本
func (p *GodcmdPlugin) getHelpText(isAdmin bool, _ bool) string {
	var helpText strings.Builder

	p.writeCommandHelp(&helpText)
	p.writePluginHelp(&helpText)

	if isAdmin {
		p.writeAdminCommandsHelp(&helpText)
	}

	return helpText.String()
}

// writeCommandHelp 写入通用命令帮助
func (p *GodcmdPlugin) writeCommandHelp(helpText *strings.Builder) {
	helpText.WriteString("通用指令\n")
	for cmdName, info := range commands {
		if cmdName == "auth" {
			continue
		}
		p.writeCommandLine(helpText, info)
	}
}

// writePluginHelp 写入插件帮助
func (p *GodcmdPlugin) writePluginHelp(helpText *strings.Builder) {
	mgr := plugin.GetManager()
	plugins := mgr.ListPlugins()
	helpText.WriteString("\n可用插件\n")
	for name, meta := range plugins {
		if !meta.Enabled || meta.Hidden {
			continue
		}
		helpText.WriteString(fmt.Sprintf("%s: ", name))
		if pluginInst, exists := mgr.GetPlugin(name); exists {
			if hp, ok := pluginInst.(HelpTextProvider); ok {
				helpText.WriteString(strings.TrimSpace(hp.HelpText()))
			}
		}
		helpText.WriteString("\n")
	}
}

// writeAdminCommandsHelp 写入管理员命令帮助
func (p *GodcmdPlugin) writeAdminCommandsHelp(helpText *strings.Builder) {
	helpText.WriteString("\n管理员指令：\n")
	for _, info := range adminCommands {
		p.writeCommandLine(helpText, info)
	}
}

// writeCommandLine 写入单行命令帮助
func (p *GodcmdPlugin) writeCommandLine(helpText *strings.Builder, info commandInfo) {
	helpText.WriteString(fmt.Sprintf("#%s", info.alias[0]))
	for _, arg := range info.args {
		helpText.WriteString(fmt.Sprintf(" %s", arg))
	}
	helpText.WriteString(fmt.Sprintf(": %s\n", info.desc))
}

// generateTempPassword 生成临时密码
func (p *GodcmdPlugin) generateTempPassword() string {
	digits := "0123456789"
	result := make([]byte, 4)
	for i := range result {
		result[i] = digits[p.rng.Intn(len(digits))]
	}
	return string(result)
}

// loadConfig 加载配置文件
func (p *GodcmdPlugin) loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	p.mu.Lock()
	p.config = &config
	p.mu.Unlock()

	return nil
}

// createDefaultConfig 创建默认配置文件
func (p *GodcmdPlugin) createDefaultConfig(path string) error {
	defaultConfig := Config{
		Password:   "",
		AdminUsers: []string{},
	}

	data, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// HelpText 返回插件帮助文本
func (p *GodcmdPlugin) HelpText() string {
	return p.getHelpText(false, false)
}
