// Package linkai 提供 LinkAI 集成插件，支持知识库、Midjourney绘画、文档总结等功能。
// 可实现群聊应用管理、文章摘要、文件处理等高级功能。
package linkai

import (
	"bytes"
	"github.com/bstr9/simpleclaw/pkg/common"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/plugin"
	"github.com/bstr9/simpleclaw/pkg/types"
)

const (
	sumIDFlag   = "sum_id"
	fileIDFlag  = "file_id"
	errLinkAI   = "LinkAI error: %s"
	errAdminReq = "admin request failed: %s"

	// 重复字符串常量 (go:S1192)
	msgSummaryGenerating   = "正在为你加速生成摘要，请稍后"
	msgOpenOrCloseRequired = "请指定 open 或 close"
	actionOpen             = "开启"
	actionClose            = "关闭"
	msgTryLater            = "请稍后再试"
	cmdStartChat           = "开启对话"
	cmdExitChat            = "退出对话"
	msgStartChatHint       = "\n\n💬 发送 \"开启对话\" 可以开启与%s内容的对话"
	msgExitChatHint        = "发送 \"退出对话\" 可以关闭与文章的对话"

	// API 路径
	apiSummaryFile       = "/v1/summary/file"
	apiSummaryURL        = "/v1/summary/url"
	apiSummaryChat       = "/v1/summary/chat"
	apiMJGenerate        = "/v1/img/midjourney/generate"
	apiMJOperate         = "/v1/img/midjourney/operate"
	apiMJTasks           = "/v1/img/midjourney/tasks/"
	apiAppInfo           = "/v1/app/info"
	apiChat              = "/v1/chat"
	defaultLinkAIBaseURL = "https://api.link-ai.tech"
)

// MidjourneyConfig Midjourney 绘画配置
type MidjourneyConfig struct {
	// Enabled 是否启用 Midjourney
	Enabled bool `json:"enabled"`
	// AutoTranslate 是否自动翻译中文提示词
	AutoTranslate bool `json:"auto_translate"`
	// ImgProxy 是否使用图片代理
	ImgProxy bool `json:"img_proxy"`
	// MaxTasks 最大任务数
	MaxTasks int `json:"max_tasks"`
	// MaxTasksPerUser 每用户最大任务数
	MaxTasksPerUser int `json:"max_tasks_per_user"`
	// UseImageCreatePrefix 是否使用图片创建前缀
	UseImageCreatePrefix bool `json:"use_image_create_prefix"`
	// Mode 绘画模式 (fast/relax)
	Mode string `json:"mode"`
}

// SummaryConfig 文档总结配置
type SummaryConfig struct {
	// Enabled 是否启用总结功能
	Enabled bool `json:"enabled"`
	// GroupEnabled 群聊是否启用
	GroupEnabled bool `json:"group_enabled"`
	// MaxFileSize 最大文件大小(KB)
	MaxFileSize int `json:"max_file_size"`
	// Type 支持的文件类型
	Type []string `json:"type"`
}

// Config 表示 linkai 插件的配置
type Config struct {
	// GroupAppMap 群聊应用映射（群名 -> 应用编码）
	GroupAppMap map[string]string `json:"group_app_map"`
	// Midjourney Midjourney 配置
	Midjourney MidjourneyConfig `json:"midjourney"`
	// Summary 文档总结配置
	Summary SummaryConfig `json:"summary"`
}

// LinkAIPlugin 实现 LinkAI 集成插件
type LinkAIPlugin struct {
	*plugin.BasePlugin

	mu      sync.RWMutex
	config  *Config
	client  *LinkAIClient
	mjBot   *MJBot
	summary *SummaryService
	userMap *ExpiredMap // 用户会话映射
}

// 确保 LinkAIPlugin 实现了 Plugin 接口
var _ plugin.Plugin = (*LinkAIPlugin)(nil)

// New 创建新的 LinkAIPlugin 实例
func New() *LinkAIPlugin {
	bp := plugin.NewBasePlugin("linkai", "0.1.0")
	bp.SetDescription("LinkAI 集成插件，支持知识库、Midjourney绘画、文档总结")
	bp.SetAuthor("https://link-ai.tech")
	bp.SetPriority(99)

	return &LinkAIPlugin{
		BasePlugin: bp,
		config: &Config{
			GroupAppMap: make(map[string]string),
			Midjourney: MidjourneyConfig{
				Enabled:              false,
				AutoTranslate:        true,
				ImgProxy:             true,
				MaxTasks:             3,
				MaxTasksPerUser:      1,
				UseImageCreatePrefix: true,
				Mode:                 "fast",
			},
			Summary: SummaryConfig{
				Enabled:      false,
				GroupEnabled: true,
				MaxFileSize:  5000,
				Type:         []string{"FILE", "SHARING"},
			},
		},
		userMap: NewExpiredMap(30 * time.Minute),
	}
}

// Name 返回插件名称
func (p *LinkAIPlugin) Name() string {
	return "linkai"
}

// Version 返回插件版本
func (p *LinkAIPlugin) Version() string {
	return "0.1.0"
}

// OnInit 初始化插件
func (p *LinkAIPlugin) OnInit(ctx *plugin.PluginContext) error {
	ctx.Debug("[linkai] 正在初始化 LinkAI 插件")

	// 加载配置文件
	configPath := filepath.Join(ctx.PluginPath, "config.json")
	if err := p.loadConfig(configPath); err != nil {
		if os.IsNotExist(err) {
			// 尝试加载模板配置
			templatePath := filepath.Join(ctx.PluginPath, "config.json.template")
			if templateErr := p.loadConfig(templatePath); templateErr != nil {
				ctx.Warn("[linkai] 加载配置失败，使用默认配置")
			}
		} else {
			ctx.Warn("[linkai] 加载配置失败: " + err.Error())
		}
	}

	// 初始化 LinkAI 客户端
	cfg := config.Get()
	p.client = NewLinkAIClient(
		cfg.LinkAIAPIKey,
		cfg.LinkAIAPIBase,
	)

	// 初始化 Midjourney 服务
	p.mjBot = NewMJBot(&p.config.Midjourney, p.fetchGroupAppCode)

	// 初始化总结服务
	p.summary = NewSummaryService(p.client, &p.config.Summary)

	ctx.Info("[linkai] 插件初始化完成")
	return nil
}

// OnLoad 加载插件
func (p *LinkAIPlugin) OnLoad(ctx *plugin.PluginContext) error {
	ctx.Debug("[linkai] 正在加载 LinkAI 插件")

	// 注册事件处理器
	p.RegisterHandler(plugin.EventOnHandleContext, p.onHandleContext)

	ctx.Info("[linkai] 插件加载成功")
	return nil
}

// OnUnload 卸载插件
func (p *LinkAIPlugin) OnUnload(ctx *plugin.PluginContext) error {
	ctx.Debug("[linkai] 正在卸载 LinkAI 插件")
	return p.BasePlugin.OnUnload(ctx)
}

// OnEvent 处理插件事件
func (p *LinkAIPlugin) OnEvent(event plugin.Event, ec *plugin.EventContext) error {
	return p.BasePlugin.OnEvent(event, ec)
}

// onHandleContext 处理 ON_HANDLE_CONTEXT 事件
func (p *LinkAIPlugin) onHandleContext(ec *plugin.EventContext) error {
	// 检查配置是否加载
	if p.config == nil {
		return nil
	}

	// 获取消息类型
	contextType, ok := ec.GetInt("type")
	if !ok {
		return nil
	}

	// 只处理特定类型消息
	switch types.ContextType(contextType) {
	case types.ContextText:
		return p.handleTextMessage(ec)
	case types.ContextImage:
		return p.handleImageMessage(ec)
	case types.ContextImageCreate:
		return p.handleImageCreateMessage(ec)
	case types.ContextFile:
		return p.handleFileMessage(ec)
	case types.ContextSharing:
		return p.handleSharingMessage(ec)
	}

	return nil
}

// handleTextMessage 处理文本消息
func (p *LinkAIPlugin) handleTextMessage(ec *plugin.EventContext) error {
	content, ok := ec.GetString("content")
	if !ok {
		return nil
	}

	triggerPrefix := p.getTriggerPrefix()

	// 检查是否是 linkai 管理命令
	if strings.HasPrefix(content, triggerPrefix+"linkai") {
		return p.handleAdminCommand(ec)
	}

	// 检查是否是 Midjourney 命令
	if handled := p.tryHandleMidjourneyCommand(ec, content, triggerPrefix); handled {
		return nil
	}

	// 处理总结对话功能
	if p.isSummaryEnabled(ec) {
		if handled := p.handleSummaryFeature(ec, content); handled {
			return nil
		}
	}

	// 处理群聊应用管理
	return p.handleGroupAppIfNeeded(ec)
}

// tryHandleMidjourneyCommand 尝试处理 Midjourney 命令，返回是否已处理。
func (p *LinkAIPlugin) tryHandleMidjourneyCommand(ec *plugin.EventContext, content, triggerPrefix string) bool {
	if !p.config.Midjourney.Enabled {
		return false
	}

	mjCmds := []string{triggerPrefix + "mj ", triggerPrefix + "mju ", triggerPrefix + "mjv ", triggerPrefix + "mjr "}
	for _, cmd := range mjCmds {
		if strings.HasPrefix(content, cmd) {
			p.handleMidjourneyCommand(ec)
			return true
		}
	}

	// 处理 mj 帮助命令
	if content == triggerPrefix+"mj" || content == triggerPrefix+"mj help" {
		p.handleMidjourneyCommand(ec)
		return true
	}

	return false
}

// handleSummaryFeature 处理总结对话功能，返回是否已处理。
func (p *LinkAIPlugin) handleSummaryFeature(ec *plugin.EventContext, content string) bool {
	userID := p.findUserID(ec)

	// 开启对话
	if content == cmdStartChat {
		sumID := p.userMap.Get(userID + sumIDFlag)
		if sumID != "" {
			p.openSummaryChat(ec, sumID.(string))
			return true
		}
	}

	// 退出对话
	if content == cmdExitChat {
		fileID := p.userMap.Get(userID + fileIDFlag)
		if fileID != nil && fileID != "" {
			p.userMap.Delete(userID + fileIDFlag)
			reply := types.NewInfoReply("对话已退出")
			ec.Set("reply", reply)
			ec.BreakPass("linkai")
			return true
		}
	}

	// 总结对话
	fileID := p.userMap.Get(userID + fileIDFlag)
	if fileID != nil && fileID != "" {
		p.handleSummaryChat(ec, content, fileID.(string))
		return true
	}

	// 检查是否是 URL 需要总结
	if p.summary.CheckURL(content) {
		p.handleURLSummary(ec, content)
		return true
	}

	return false
}

// handleGroupAppIfNeeded 处理群聊应用消息（如果需要）。
func (p *LinkAIPlugin) handleGroupAppIfNeeded(ec *plugin.EventContext) error {
	isGroup, _ := ec.GetBool("is_group")
	if isGroup && len(p.config.GroupAppMap) > 0 {
		return p.handleGroupAppMessage(ec)
	}
	return nil
}

// handleImageMessage 处理图片消息 - 实现图片总结功能
func (p *LinkAIPlugin) handleImageMessage(ec *plugin.EventContext) error {
	// 检查是否启用总结功能
	if !p.isSummaryEnabled(ec) {
		return nil
	}

	// 获取图片路径
	imagePath, ok := ec.GetString("content")
	if !ok || imagePath == "" {
		return nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return nil
	}

	// 检查文件大小和类型
	if !p.summary.CheckFile(imagePath) {
		return nil
	}

	// 获取应用编码
	appCode := p.fetchAppCode(ec)

	// 获取会话 ID
	sessionID := p.getSessionID(ec)

	result, err := p.summary.SummaryFile(imagePath, appCode, sessionID)
	if err != nil {
		return nil
	}

	if result == nil || result.Summary == "" {
		return nil
	}

	// 保存 summary_id 用于后续对话
	userID := p.findUserID(ec)
	if userID != "" && result.SummaryID != "" {
		p.userMap.Set(userID+sumIDFlag, result.SummaryID)
	}

	// 返回总结结果
	reply := types.NewTextReply(result.Summary)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")

	return nil
}

// handleImageCreateMessage 处理图片生成消息 - 使用 Midjourney 生成图片
func (p *LinkAIPlugin) handleImageCreateMessage(ec *plugin.EventContext) error {
	if !p.config.Midjourney.Enabled {
		return nil
	}

	// 检查是否使用图片创建前缀
	if !p.config.Midjourney.UseImageCreatePrefix {
		return nil
	}

	// 检查 MJ 是否开启（本地或远程）
	if !p.isMJOpen(ec) {
		return nil
	}

	// 获取提示词
	prompt, ok := ec.GetString("content")
	if !ok || prompt == "" {
		return nil
	}

	// 获取会话 ID
	sessionID := p.getSessionID(ec)

	// 检查速率限制
	if !p.mjBot.CheckRateLimit(sessionID, ec, p.config.Midjourney.MaxTasks, p.config.Midjourney.MaxTasksPerUser) {
		return nil
	}

	// 生成图片
	reply := p.mjBot.Generate(prompt, sessionID, ec)
	if reply != nil {
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
	}

	return nil
}

// handleFileMessage 处理文件消息 - 实现文件总结功能
func (p *LinkAIPlugin) handleFileMessage(ec *plugin.EventContext) error {
	// 检查是否启用总结功能
	if !p.isSummaryEnabled(ec) {
		return nil
	}

	// 获取文件路径
	filePath, ok := ec.GetString("content")
	if !ok || filePath == "" {
		return nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}

	// 检查文件大小和类型
	if !p.summary.CheckFile(filePath) {
		return nil
	}

	// 发送处理中提示
	p.sendInfoReply(ec, msgSummaryGenerating)

	// 获取应用编码
	appCode := p.fetchAppCode(ec)

	// 获取会话 ID
	sessionID := p.getSessionID(ec)

	// 调用文件总结 API
	result, err := p.summary.SummaryFile(filePath, appCode, sessionID)
	if err != nil {
		reply := types.NewErrorReply("因为神秘力量无法获取内容，请稍后再试吧")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	if result == nil || result.Summary == "" {
		reply := types.NewErrorReply("无法生成文件摘要")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	// 保存 summary_id 用于后续对话
	userID := p.findUserID(ec)
	if userID != "" && result.SummaryID != "" {
		p.userMap.Set(userID+sumIDFlag, result.SummaryID)
	}

	// 返回总结结果
	summaryText := result.Summary + fmt.Sprintf(msgStartChatHint, "文件")
	reply := types.NewTextReply(summaryText)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")

	// 清理临时文件
	os.Remove(filePath)

	return nil
}

// handleSharingMessage 处理分享消息 - 实现链接总结功能
func (p *LinkAIPlugin) handleSharingMessage(ec *plugin.EventContext) error {
	// 检查是否启用总结功能
	if !p.isSummaryEnabled(ec) {
		return nil
	}

	// 获取分享 URL
	url, ok := ec.GetString("content")
	if !ok || url == "" {
		return nil
	}

	return p.handleURLSummary(ec, url)
}

// handleURLSummary 处理 URL 总结
func (p *LinkAIPlugin) handleURLSummary(ec *plugin.EventContext, url string) error {
	// 解码 URL
	url = html.UnescapeString(url)

	// 检查 URL 是否支持
	if !p.summary.CheckURL(url) {
		return nil
	}

	// 发送处理中提示
	p.sendInfoReply(ec, msgSummaryGenerating)

	// 获取应用编码
	appCode := p.fetchAppCode(ec)

	// 调用 URL 总结 API
	result, err := p.summary.SummaryURL(url, appCode)
	if err != nil {
		reply := types.NewErrorReply("因为神秘力量无法获取文章内容，请稍后再试吧~")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	if result == nil || result.Summary == "" {
		reply := types.NewErrorReply("无法生成文章摘要")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	// 保存 summary_id 用于后续对话
	userID := p.findUserID(ec)
	if userID != "" && result.SummaryID != "" {
		p.userMap.Set(userID+sumIDFlag, result.SummaryID)
	}

	// 返回总结结果
	summaryText := result.Summary + fmt.Sprintf(msgStartChatHint, "文章")
	reply := types.NewTextReply(summaryText)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")

	return nil
}

// openSummaryChat 开启总结对话
func (p *LinkAIPlugin) openSummaryChat(ec *plugin.EventContext, sumID string) error {
	p.sendInfoReply(ec, "正在为你开启对话，请稍后")

	result, err := p.summary.SummaryChat(sumID)
	if err != nil || result == nil {
		reply := types.NewErrorReply("开启对话失败，请稍后再试吧")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	// 保存 file_id 用于后续对话
	userID := p.findUserID(ec)
	if userID != "" && result.FileID != "" {
		p.userMap.Set(userID+fileIDFlag, result.FileID)
	}

	// 返回提示信息
	helpText := "💡你可以问我关于这篇文章的任何问题，例如：\n\n" + result.Questions + "\n\n发送 \"退出对话\" 可以关闭与文章的对话"
	reply := types.NewTextReply(helpText)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")

	return nil
}

// handleSummaryChat 处理总结对话
func (p *LinkAIPlugin) handleSummaryChat(ec *plugin.EventContext, query, fileID string) error {
	// 调用 LinkAI 对话 API
	resp, err := p.client.Chat(query, fileID)
	if err != nil {
		reply := types.NewErrorReply("对话失败: " + err.Error())
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	reply := types.NewTextReply(resp)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")

	return nil
}

// handleAdminCommand 处理管理命令
func (p *LinkAIPlugin) handleAdminCommand(ec *plugin.EventContext) error {
	content, _ := ec.GetString("content")
	parts := strings.Fields(content)

	if len(parts) == 1 || (len(parts) == 2 && parts[1] == "help") {
		reply := types.NewInfoReply(p.HelpText())
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	if len(parts) < 2 {
		return nil
	}

	switch parts[1] {
	case "open", "close":
		return p.handleAdminToggleCmd(ec, parts[1])
	case "app":
		return p.handleAdminAppCmd(ec, parts)
	case "sum":
		return p.handleAdminSumCmd(ec, parts)
	case "mj":
		return p.handleAdminMJCmd(ec, parts)
	}

	return nil
}

// handleAdminToggleCmd 处理 linkai open/close 命令
func (p *LinkAIPlugin) handleAdminToggleCmd(ec *plugin.EventContext, cmd string) error {
	isAdmin, _ := ec.GetBool("is_admin")
	if !isAdmin {
		reply := types.NewErrorReply(errAdminReq)
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}
	action := actionOpen
	if cmd == "close" {
		action = actionClose
	}
	reply := types.NewInfoReply("LinkAI对话功能" + action)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")
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

// handleAdminSumCmd 处理 linkai sum 命令
func (p *LinkAIPlugin) handleAdminSumCmd(ec *plugin.EventContext, parts []string) error {
	isAdmin, _ := ec.GetBool("is_admin")

	if len(parts) < 3 {
		reply := types.NewErrorReply(msgOpenOrCloseRequired)
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
	action := actionOpen
	if parts[2] == "close" {
		action = actionClose
	}
	p.mu.Lock()
	p.config.Summary.Enabled = parts[2] == "open"
	p.mu.Unlock()
	reply := types.NewInfoReply("文章总结功能" + action)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")
	return nil
}

// handleAdminMJCmd 处理 linkai mj 命令
func (p *LinkAIPlugin) handleAdminMJCmd(ec *plugin.EventContext, parts []string) error {
	isAdmin, _ := ec.GetBool("is_admin")

	if len(parts) < 3 {
		reply := types.NewErrorReply(msgOpenOrCloseRequired)
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
	action := actionOpen
	if parts[2] == "close" {
		action = actionClose
	}
	p.mu.Lock()
	p.config.Midjourney.Enabled = parts[2] == "open"
	p.mu.Unlock()
	reply := types.NewInfoReply("Midjourney绘画已" + action)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")
	return nil
}

// handleMidjourneyCommand 处理 Midjourney 命令
func (p *LinkAIPlugin) handleMidjourneyCommand(ec *plugin.EventContext) error {
	content, _ := ec.GetString("content")
	triggerPrefix := p.getTriggerPrefix()

	// 解析命令
	parts := strings.SplitN(content, " ", 2)
	cmd := parts[0]

	// 处理帮助命令
	if len(parts) == 1 && cmd == triggerPrefix+"mj" {
		reply := types.NewInfoReply(p.mjBot.GetHelpText(true))
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	// 处理开关命令
	if len(parts) == 2 && (parts[1] == "open" || parts[1] == "close") {
		return p.handleMJToggleCmd(ec, parts[1])
	}

	// 检查 MJ 是否开启
	if !p.isMJOpen(ec) {
		reply := types.NewInfoReply("Midjourney绘画未开启")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	// 获取会话 ID
	sessionID := p.getSessionID(ec)

	// 检查速率限制
	if !p.mjBot.CheckRateLimit(sessionID, ec, p.config.Midjourney.MaxTasks, p.config.Midjourney.MaxTasksPerUser) {
		return nil
	}

	// 执行 MJ 操作
	reply := p.executeMJCommand(content, triggerPrefix, sessionID, ec)
	if reply != nil {
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
	}

	return nil
}

// handleMJToggleCmd 处理 Midjourney 开关命令
func (p *LinkAIPlugin) handleMJToggleCmd(ec *plugin.EventContext, cmd string) error {
	isAdmin, _ := ec.GetBool("is_admin")
	if !isAdmin {
		reply := types.NewErrorReply(errAdminReq)
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return nil
	}

	action := actionOpen
	if cmd == "close" {
		action = actionClose
	}
	p.mu.Lock()
	p.config.Midjourney.Enabled = cmd == "open"
	p.mu.Unlock()

	reply := types.NewInfoReply("Midjourney绘画已" + action)
	ec.Set("reply", reply)
	ec.BreakPass("linkai")
	return nil
}

// executeMJCommand 执行 Midjourney 命令
func (p *LinkAIPlugin) executeMJCommand(content, triggerPrefix, sessionID string, ec *plugin.EventContext) *types.Reply {
	switch {
	case strings.HasPrefix(content, triggerPrefix+"mj "):
		prompt := strings.TrimPrefix(content, triggerPrefix+"mj ")
		return p.mjBot.Generate(prompt, sessionID, ec)

	case strings.HasPrefix(content, triggerPrefix+"mju "):
		params := strings.TrimPrefix(content, triggerPrefix+"mju ")
		return p.handleMJOperate(MJTaskUpscale, params, sessionID, ec)

	case strings.HasPrefix(content, triggerPrefix+"mjv "):
		params := strings.TrimPrefix(content, triggerPrefix+"mjv ")
		return p.handleMJOperate(MJTaskVariation, params, sessionID, ec)

	case strings.HasPrefix(content, triggerPrefix+"mjr "):
		params := strings.TrimPrefix(content, triggerPrefix+"mjr ")
		return p.handleMJOperate(MJTaskReset, params, sessionID, ec)
	}
	return nil
}

// handleMJOperate 处理 Midjourney 操作命令（放大、变换、重置）
func (p *LinkAIPlugin) handleMJOperate(taskType MJTaskType, params string, sessionID string, ec *plugin.EventContext) *types.Reply {
	parts := strings.Fields(params)

	switch taskType {
	case MJTaskUpscale, MJTaskVariation:
		if len(parts) < 2 {
			return types.NewErrorReply("命令缺少参数，格式: " + p.getTriggerPrefix() + "mj[u/v] 图片ID 图片序号(1-4)")
		}
		imgID := parts[0]
		index, err := strconv.Atoi(parts[1])
		if err != nil || index < 1 || index > 4 {
			return types.NewErrorReply("图片序号错误，应在 1 至 4 之间")
		}
		return p.mjBot.DoOperate(taskType, sessionID, imgID, index, ec)

	case MJTaskReset:
		if len(parts) < 1 {
			return types.NewErrorReply("命令缺少参数，格式: " + p.getTriggerPrefix() + "mjr 图片ID")
		}
		imgID := parts[0]
		return p.mjBot.DoOperate(taskType, sessionID, imgID, 0, ec)
	}

	return types.NewErrorReply("暂不支持该命令")
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

// isSummaryEnabled 检查总结功能是否启用
func (p *LinkAIPlugin) isSummaryEnabled(ec *plugin.EventContext) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.config.Summary.Enabled {
		return false
	}

	isGroup, _ := ec.GetBool("is_group")
	if isGroup && !p.config.Summary.GroupEnabled {
		return false
	}

	return true
}

// isMJOpen 检查 Midjourney 是否开启（本地配置或远程插件）
func (p *LinkAIPlugin) isMJOpen(ec *plugin.EventContext) bool {
	// 本地配置
	if p.config.Midjourney.Enabled {
		return true
	}

	// 检查远程应用插件状态
	appCode := p.fetchAppCode(ec)
	if appCode != "" {
		enabled, _ := p.client.FetchAppPlugin(appCode, "Midjourney")
		return enabled
	}

	return false
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

// findUserID 获取用户 ID
func (p *LinkAIPlugin) findUserID(ec *plugin.EventContext) string {
	isGroup, _ := ec.GetBool("is_group")
	if isGroup {
		// 群聊使用实际用户 ID
		if actualUserID, ok := ec.GetString("actual_user_id"); ok {
			return actualUserID
		}
	}
	// 非群聊使用接收者
	if receiver, ok := ec.GetString("receiver"); ok {
		return receiver
	}
	return ""
}

// getSessionID 获取会话 ID
func (p *LinkAIPlugin) getSessionID(ec *plugin.EventContext) string {
	if sessionID, ok := ec.GetString("session_id"); ok {
		return sessionID
	}
	return p.findUserID(ec)
}

// getTriggerPrefix 获取触发前缀
func (p *LinkAIPlugin) getTriggerPrefix() string {
	cfg := config.Get()
	if cfg.PluginTriggerPrefix != "" {
		return cfg.PluginTriggerPrefix
	}
	return "$"
}

// sendInfoReply 发送信息回复（用于异步通知）
func (p *LinkAIPlugin) sendInfoReply(ec *plugin.EventContext, content string) {
	// 获取 channel 并发送消息
	if channel, ok := ec.Get("channel"); ok && channel != nil {
		// 使用反射调用 channel 的 send 方法
		// 这里简化处理，直接设置回复
		ec.Set("reply", types.NewInfoReply(content))
	}
}

// loadConfig 加载配置文件
func (p *LinkAIPlugin) loadConfig(path string) error {
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

// HelpText 返回插件帮助文本
func (p *LinkAIPlugin) HelpText() string {
	var helpText strings.Builder
	triggerPrefix := p.getTriggerPrefix()

	helpText.WriteString("用于集成 LinkAI 提供的知识库、Midjourney绘画、文档总结、联网搜索等能力。\n\n")

	helpText.WriteString("📖 知识库\n")
	helpText.WriteString(" - 群聊中指定应用: " + triggerPrefix + "linkai app 应用编码\n")
	helpText.WriteString(" - " + triggerPrefix + "linkai open: 开启对话\n")
	helpText.WriteString(" - " + triggerPrefix + "linkai close: 关闭对话\n\n")

	helpText.WriteString("🎨 绘画\n")
	helpText.WriteString(" - 生成: " + triggerPrefix + "mj 描述词1, 描述词2..\n")
	helpText.WriteString(" - 放大: " + triggerPrefix + "mju 图片ID 图片序号\n")
	helpText.WriteString(" - 变换: " + triggerPrefix + "mjv 图片ID 图片序号\n")
	helpText.WriteString(" - 重置: " + triggerPrefix + "mjr 图片ID\n")
	helpText.WriteString(" - 开关: " + triggerPrefix + "mj open/close\n\n")

	helpText.WriteString("💡 文档总结和对话\n")
	helpText.WriteString(" - 开启: " + triggerPrefix + "linkai sum open\n")
	helpText.WriteString(" - 使用: 发送文件、公众号文章等可生成摘要，并与内容对话\n")

	return helpText.String()
}

// ============ LinkAI API 客户端 ============

// LinkAIClient LinkAI API 客户端
type LinkAIClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewLinkAIClient 创建新的 LinkAI 客户端
func NewLinkAIClient(apiKey, baseURL string) *LinkAIClient {
	if baseURL == "" {
		baseURL = defaultLinkAIBaseURL
	}

	return &LinkAIClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

// SummaryFileResponse 文件总结响应
type SummaryFileResponse struct {
	Code int `json:"code"`
	Data struct {
		Summary   string `json:"summary"`
		SummaryID string `json:"summary_id"`
		FileID    string `json:"file_id"`
	} `json:"data"`
	Message string `json:"message"`
}

// SummaryURLResponse URL 总结响应
type SummaryURLResponse struct {
	Code int `json:"code"`
	Data struct {
		Summary   string `json:"summary"`
		SummaryID string `json:"summary_id"`
	} `json:"data"`
	Message string `json:"message"`
}

// SummaryChatResponse 总结对话响应
type SummaryChatResponse struct {
	Code int `json:"code"`
	Data struct {
		Questions string `json:"questions"`
		FileID    string `json:"file_id"`
	} `json:"data"`
	Message string `json:"message"`
}

// MJGenerateResponse MJ 生成响应
type MJGenerateResponse struct {
	Code int `json:"code"`
	Data struct {
		TaskID     string `json:"task_id"`
		RealPrompt string `json:"real_prompt"`
	} `json:"data"`
	Message string `json:"message"`
}

// MJOperateResponse MJ 操作响应
type MJOperateResponse struct {
	Code int `json:"code"`
	Data struct {
		TaskID string `json:"task_id"`
	} `json:"data"`
	Message string `json:"message"`
}

// MJTaskResponse MJ 任务状态响应
type MJTaskResponse struct {
	Code int `json:"code"`
	Data struct {
		Status string `json:"status"`
		ImgID  string `json:"img_id"`
		ImgURL string `json:"img_url"`
	} `json:"data"`
	Message string `json:"message"`
}

// AppInfoResponse 应用信息响应
type AppInfoResponse struct {
	Code int `json:"code"`
	Data struct {
		Plugins []struct {
			Name string `json:"name"`
		} `json:"plugins"`
	} `json:"data"`
	Message string `json:"message"`
}

// SummaryFile 调用文件总结 API
func (c *LinkAIClient) SummaryFile(filePath, appCode, sessionID string) (*SummaryFileResponse, error) {
	url := c.baseURL + apiSummaryFile

	// 创建 multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 添加文件
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	// 添加其他字段
	if appCode != "" {
		writer.WriteField("app_code", appCode)
	}
	if sessionID != "" {
		writer.WriteField("session_id", sessionID)
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	// 创建请求
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set(common.HeaderContentType, writer.FormDataContentType())
	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 解析响应
	var result SummaryFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SummaryURL 调用 URL 总结 API
func (c *LinkAIClient) SummaryURL(urlStr, appCode string) (*SummaryURLResponse, error) {
	url := c.baseURL + apiSummaryURL

	body := map[string]string{
		"url":      urlStr,
		"app_code": appCode,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SummaryURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SummaryChat 调用总结对话 API
func (c *LinkAIClient) SummaryChat(summaryID string) (*SummaryChatResponse, error) {
	url := c.baseURL + apiSummaryChat

	body := map[string]string{
		"summary_id": summaryID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SummaryChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// MJGenerate 调用 MJ 生成 API
func (c *LinkAIClient) MJGenerate(prompt, mode string, autoTranslate, imgProxy bool) (*MJGenerateResponse, error) {
	url := c.baseURL + apiMJGenerate

	body := map[string]any{
		"prompt":         prompt,
		"mode":           mode,
		"auto_translate": autoTranslate,
		"img_proxy":      imgProxy,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result MJGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// MJOperate 调用 MJ 操作 API（放大、变换、重置）
func (c *LinkAIClient) MJOperate(taskType, imgID string, index int, imgProxy bool) (*MJOperateResponse, error) {
	url := c.baseURL + apiMJOperate

	body := map[string]any{
		"type":      taskType,
		"img_id":    imgID,
		"img_proxy": imgProxy,
	}
	if index > 0 {
		body["index"] = index
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result MJOperateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// MJTaskStatus 获取 MJ 任务状态
func (c *LinkAIClient) MJTaskStatus(taskID string) (*MJTaskResponse, error) {
	url := c.baseURL + apiMJTasks + taskID

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result MJTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// FetchAppPlugin 获取应用插件状态
func (c *LinkAIClient) FetchAppPlugin(appCode, pluginName string) (bool, error) {
	url := c.baseURL + apiAppInfo + "?app_code=" + appCode

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var result AppInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	for _, plugin := range result.Data.Plugins {
		if plugin.Name == pluginName {
			return true, nil
		}
	}

	return false, nil
}

// Chat 调用对话 API（带 file_id）
func (c *LinkAIClient) Chat(query, fileID string) (string, error) {
	url := c.baseURL + apiChat

	body := map[string]any{
		"query":   query,
		"file_id": fileID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set(common.HeaderContentType, common.ContentTypeJSON)
	req.Header.Set("Authorization", common.AuthPrefixBearer+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Data    string `json:"data"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.Code != 200 {
		return "", fmt.Errorf(errLinkAI, result.Message)
	}

	return result.Data, nil
}

// ============ 总结服务 ============

// SummaryService 总结服务
type SummaryService struct {
	client *LinkAIClient
	config *SummaryConfig
}

// NewSummaryService 创建总结服务
func NewSummaryService(client *LinkAIClient, config *SummaryConfig) *SummaryService {
	return &SummaryService{
		client: client,
		config: config,
	}
}

// SummaryResult 总结结果
type SummaryResult struct {
	Summary   string
	SummaryID string
	FileID    string
}

// ChatResult 对话结果
type ChatResult struct {
	Questions string
	FileID    string
}

// SummaryFile 文件总结
func (s *SummaryService) SummaryFile(filePath, appCode, sessionID string) (*SummaryResult, error) {
	resp, err := s.client.SummaryFile(filePath, appCode, sessionID)
	if err != nil {
		return nil, err
	}

	if resp.Code != 200 {
		return nil, fmt.Errorf(errLinkAI, resp.Message)
	}

	return &SummaryResult{
		Summary:   resp.Data.Summary,
		SummaryID: resp.Data.SummaryID,
		FileID:    resp.Data.FileID,
	}, nil
}

// SummaryURL URL 总结
func (s *SummaryService) SummaryURL(url, appCode string) (*SummaryResult, error) {
	resp, err := s.client.SummaryURL(url, appCode)
	if err != nil {
		return nil, err
	}

	if resp.Code != 200 {
		return nil, fmt.Errorf(errLinkAI, resp.Message)
	}

	return &SummaryResult{
		Summary:   resp.Data.Summary,
		SummaryID: resp.Data.SummaryID,
	}, nil
}

// SummaryChat 总结对话
func (s *SummaryService) SummaryChat(summaryID string) (*ChatResult, error) {
	resp, err := s.client.SummaryChat(summaryID)
	if err != nil {
		return nil, err
	}

	if resp.Code != 200 {
		return nil, fmt.Errorf(errLinkAI, resp.Message)
	}

	return &ChatResult{
		Questions: resp.Data.Questions,
		FileID:    resp.Data.FileID,
	}, nil
}

// CheckFile 检查文件是否符合总结要求
func (s *SummaryService) CheckFile(filePath string) bool {
	// 检查文件大小
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	fileSizeKB := int(fileInfo.Size() / 1024)
	maxSize := s.config.MaxFileSize
	if maxSize == 0 {
		maxSize = 5000 // 默认 5MB
	}
	if fileSizeKB > maxSize || fileSizeKB > 15000 {
		return false
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(filePath))
	ext = strings.TrimPrefix(ext, ".")
	supportedTypes := []string{"txt", "csv", "docx", "pdf", "md", "jpg", "jpeg", "png", "gif", "webp"}
	for _, t := range supportedTypes {
		if t == ext {
			return true
		}
	}

	return false
}

// CheckURL 检查 URL 是否支持总结
func (s *SummaryService) CheckURL(url string) bool {
	if url == "" {
		return false
	}

	// 支持的 URL 前缀
	supportedPrefixes := []string{
		"http://mp.weixin.qq.com",
		"https://mp.weixin.qq.com",
	}

	// 黑名单 URL 前缀
	blacklistPrefixes := []string{
		"https://mp.weixin.qq.com/mp/waerrpage",
	}

	url = strings.TrimSpace(url)

	// 检查黑名单
	for _, prefix := range blacklistPrefixes {
		if strings.HasPrefix(url, prefix) {
			return false
		}
	}

	// 检查支持列表
	for _, prefix := range supportedPrefixes {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}

	return false
}

// ============ Midjourney 服务 ============

// MJTaskType MJ 任务类型
type MJTaskType string

const (
	MJTaskGenerate  MJTaskType = "generate"
	MJTaskUpscale   MJTaskType = "upscale"
	MJTaskVariation MJTaskType = "variation"
	MJTaskReset     MJTaskType = "reset"
)

// MJTaskStatus MJ 任务状态
type MJTaskStatus string

const (
	MJStatusPending  MJTaskStatus = "pending"
	MJStatusFinished MJTaskStatus = "finished"
	MJStatusExpired  MJTaskStatus = "expired"
	MJStatusAborted  MJTaskStatus = "aborted"
)

// MJTask MJ 任务
type MJTask struct {
	ID         string
	UserID     string
	TaskType   MJTaskType
	RawPrompt  string
	Status     MJTaskStatus
	ImgURL     string
	ImgID      string
	ExpiryTime time.Time
}

// MJBot Midjourney 机器人
type MJBot struct {
	client        *LinkAIClient
	config        *MidjourneyConfig
	fetchGroupApp func(string) string
	tasks         map[string]*MJTask
	tempDict      map[string]bool // 防止重复操作
	mu            sync.RWMutex
}

// NewMJBot 创建 MJ 机器人
func NewMJBot(config *MidjourneyConfig, fetchGroupApp func(string) string) *MJBot {
	return &MJBot{
		client:        nil, // 将在插件初始化时设置
		config:        config,
		fetchGroupApp: fetchGroupApp,
		tasks:         make(map[string]*MJTask),
		tempDict:      make(map[string]bool),
	}
}

// SetClient 设置客户端
func (b *MJBot) SetClient(client *LinkAIClient) {
	b.client = client
}

// CheckRateLimit 检查速率限制
func (b *MJBot) CheckRateLimit(userID string, ec *plugin.EventContext, maxTasks, maxTasksPerUser int) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// 检查用户任务数
	userTaskCount := 0
	for _, task := range b.tasks {
		if task.UserID == userID && task.Status == MJStatusPending {
			userTaskCount++
		}
	}
	if userTaskCount >= maxTasksPerUser {
		reply := types.NewInfoReply("您的Midjourney作图任务数已达上限，请稍后再试")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return false
	}

	// 检查总任务数
	totalTaskCount := 0
	for _, task := range b.tasks {
		if task.Status == MJStatusPending {
			totalTaskCount++
		}
	}
	if totalTaskCount >= maxTasks {
		reply := types.NewInfoReply("Midjourney作图任务数已达上限，请稍后再试")
		ec.Set("reply", reply)
		ec.BreakPass("linkai")
		return false
	}

	return true
}

// Generate 生成图片
func (b *MJBot) Generate(prompt, userID string, ec *plugin.EventContext) *types.Reply {
	// 确定模式
	mode := b.config.Mode
	if strings.Contains(prompt, "--relax") {
		mode = "relax"
	}

	// 调用 API
	resp, err := b.client.MJGenerate(prompt, mode, b.config.AutoTranslate, b.config.ImgProxy)
	if err != nil {
		return types.NewErrorReply("图片生成失败，请稍后再试")
	}

	if resp.Code != 200 {
		return types.NewErrorReply("图片生成失败，请检查提示词参数或内容")
	}

	// 创建任务
	task := &MJTask{
		ID:         resp.Data.TaskID,
		UserID:     userID,
		TaskType:   MJTaskGenerate,
		RawPrompt:  prompt,
		Status:     MJStatusPending,
		ExpiryTime: time.Now().Add(10 * time.Minute),
	}

	b.mu.Lock()
	b.tasks[task.ID] = task
	b.mu.Unlock()

	// 启动后台检查
	go b.checkTask(task, ec)

	// 返回提示信息
	timeStr := "1分钟"
	if mode == "relax" {
		timeStr = "1~10分钟"
	}

	content := fmt.Sprintf("🚀您的作品将在%s左右完成，请耐心等待\n- - - - - - - - -\n", timeStr)
	if resp.Data.RealPrompt != "" {
		content += fmt.Sprintf("初始prompt: %s\n转换后prompt: %s", prompt, resp.Data.RealPrompt)
	} else {
		content += "prompt: " + prompt
	}

	return types.NewInfoReply(content)
}

// DoOperate 执行操作（放大、变换、重置）
func (b *MJBot) DoOperate(taskType MJTaskType, userID, imgID string, index int, ec *plugin.EventContext) *types.Reply {
	// 检查是否已经操作过
	key := fmt.Sprintf("%s_%s_%d", taskType, imgID, index)
	b.mu.RLock()
	_, exists := b.tempDict[key]
	b.mu.RUnlock()

	if exists {
		taskNames := map[MJTaskType]string{
			MJTaskUpscale:   "放大",
			MJTaskVariation: "变换",
			MJTaskReset:     "重新生成",
		}
		return types.NewErrorReply(fmt.Sprintf("该图片已经%s过了", taskNames[taskType]))
	}

	// 调用 API
	resp, err := b.client.MJOperate(string(taskType), imgID, index, b.config.ImgProxy)
	if err != nil {
		return types.NewErrorReply("操作失败，请稍后再试")
	}

	if resp.Code != 200 {
		return types.NewErrorReply("请输入正确的图片ID")
	}

	// 创建任务
	task := &MJTask{
		ID:         resp.Data.TaskID,
		UserID:     userID,
		TaskType:   taskType,
		Status:     MJStatusPending,
		ExpiryTime: time.Now().Add(10 * time.Minute),
	}

	b.mu.Lock()
	b.tasks[task.ID] = task
	b.tempDict[key] = true
	b.mu.Unlock()

	// 启动后台检查
	go b.checkTask(task, ec)

	// 返回提示信息
	icons := map[MJTaskType]string{
		MJTaskUpscale:   "🔎",
		MJTaskVariation: "🪄",
		MJTaskReset:     "🔄",
	}
	taskNames := map[MJTaskType]string{
		MJTaskUpscale:   "放大",
		MJTaskVariation: "变换",
		MJTaskReset:     "重新生成",
	}

	content := fmt.Sprintf("%s图片正在%s中，请耐心等待", icons[taskType], taskNames[taskType])
	return types.NewInfoReply(content)
}

// checkTask 检查任务状态
func (b *MJBot) checkTask(task *MJTask, ec *plugin.EventContext) {
	maxRetry := 90
	for i := 0; i < maxRetry; i++ {
		time.Sleep(10 * time.Second)

		resp, err := b.client.MJTaskStatus(task.ID)
		if err != nil {
			continue
		}

		if resp.Code == 200 && resp.Data.Status == string(MJStatusFinished) {
			// 更新任务状态
			b.mu.Lock()
			if t, ok := b.tasks[task.ID]; ok {
				t.Status = MJStatusFinished
				t.ImgID = resp.Data.ImgID
				t.ImgURL = resp.Data.ImgURL
			}
			b.mu.Unlock()

			// 发送结果（这里需要通过 channel 发送，简化处理）
			// 实际实现中应该通过 channel 的 send 方法发送图片和提示信息
			return
		}
	}

	// 超时
	b.mu.Lock()
	if t, ok := b.tasks[task.ID]; ok {
		t.Status = MJStatusExpired
	}
	b.mu.Unlock()
}

// GetHelpText 获取帮助文本
func (b *MJBot) GetHelpText(verbose bool) string {
	triggerPrefix := "$" // 默认前缀
	helpText := "🎨利用Midjourney进行画图\n\n"

	if !verbose {
		return helpText
	}

	helpText += fmt.Sprintf(" - 生成: %smj 描述词1, 描述词2.. \n", triggerPrefix)
	helpText += fmt.Sprintf(" - 放大: %smju 图片ID 图片序号\n", triggerPrefix)
	helpText += fmt.Sprintf(" - 变换: %smjv 图片ID 图片序号\n", triggerPrefix)
	helpText += fmt.Sprintf(" - 重置: %smjr 图片ID\n", triggerPrefix)
	helpText += fmt.Sprintf("\n例如：\n\"%smj a little cat, white --ar 9:16\"\n", triggerPrefix)
	helpText += fmt.Sprintf("\"%smju 11055927171882 2\"\n", triggerPrefix)
	helpText += fmt.Sprintf("\"%smjv 11055927171882 2\"\n", triggerPrefix)
	helpText += fmt.Sprintf("\"%smjr 11055927171882\"", triggerPrefix)

	return helpText
}

// ============ 过期 Map ============

// ExpiredMap 带过期时间的 Map
type ExpiredMap struct {
	data map[string]expiredItem
	mu   sync.RWMutex
	ttl  time.Duration
}

type expiredItem struct {
	value      any
	expiryTime time.Time
}

// NewExpiredMap 创建过期 Map
func NewExpiredMap(ttl time.Duration) *ExpiredMap {
	m := &ExpiredMap{
		data: make(map[string]expiredItem),
		ttl:  ttl,
	}

	// 启动清理协程
	go m.cleanup()

	return m
}

// Set 设置值
func (m *ExpiredMap) Set(key string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = expiredItem{
		value:      value,
		expiryTime: time.Now().Add(m.ttl),
	}
}

// Get 获取值
func (m *ExpiredMap) Get(key string) any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.data[key]
	if !ok || time.Now().After(item.expiryTime) {
		return nil
	}
	return item.value
}

// Delete 删除值
func (m *ExpiredMap) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

// cleanup 定期清理过期项
func (m *ExpiredMap) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for k, v := range m.data {
			if now.After(v.expiryTime) {
				delete(m.data, k)
			}
		}
		m.mu.Unlock()
	}
}
