// Package linkai 提供 LinkAI 集成插件，支持知识库、Midjourney绘画、文档总结等功能。
// 可实现群聊应用管理、文章摘要、文件处理等高级功能。
package linkai

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
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

	// API 路径和默认地址在各功能文件中定义
)

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
