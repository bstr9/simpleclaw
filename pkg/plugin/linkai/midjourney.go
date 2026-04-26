// Package linkai 提供 LinkAI 集成插件，支持知识库、Midjourney绘画、文档总结等功能。
// 本文件包含 Midjourney 绘画相关的类型定义、API 客户端方法、MJBot 服务及插件处理器。
package linkai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bstr9/simpleclaw/pkg/common"
	"github.com/bstr9/simpleclaw/pkg/plugin"
	"github.com/bstr9/simpleclaw/pkg/types"
)

const (
	// API 路径 - Midjourney 相关
	apiMJGenerate = "/v1/img/midjourney/generate"
	apiMJOperate  = "/v1/img/midjourney/operate"
	apiMJTasks    = "/v1/img/midjourney/tasks/"
	apiAppInfo    = "/v1/app/info"
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

// ============ Midjourney 插件处理器 ============

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
