// Package main is the entry point for simpleclaw.
// simpleclaw is a multi-channel AI assistant framework.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bstr9/simpleclaw/pkg/api"
	"github.com/bstr9/simpleclaw/pkg/bridge"
	"github.com/bstr9/simpleclaw/pkg/channel"
	"github.com/bstr9/simpleclaw/pkg/config"
	"github.com/bstr9/simpleclaw/pkg/extension"
	"github.com/bstr9/simpleclaw/pkg/logger"
	"github.com/bstr9/simpleclaw/pkg/scheduler"
	"github.com/bstr9/simpleclaw/pkg/types"
	_ "github.com/bstr9/simpleclaw/pkg/voice/ali"
	_ "github.com/bstr9/simpleclaw/pkg/voice/azure"
	_ "github.com/bstr9/simpleclaw/pkg/voice/baidu"
	_ "github.com/bstr9/simpleclaw/pkg/voice/edge"
	_ "github.com/bstr9/simpleclaw/pkg/voice/elevent"
	_ "github.com/bstr9/simpleclaw/pkg/voice/google"
	_ "github.com/bstr9/simpleclaw/pkg/voice/openai"
	_ "github.com/bstr9/simpleclaw/pkg/voice/tencent"
	_ "github.com/bstr9/simpleclaw/pkg/voice/xunfei"

	"github.com/bstr9/simpleclaw/pkg/plugin"
	"github.com/bstr9/simpleclaw/pkg/plugin/agent"
	"github.com/bstr9/simpleclaw/pkg/plugin/banwords"
	"github.com/bstr9/simpleclaw/pkg/plugin/dungeon"
	"github.com/bstr9/simpleclaw/pkg/plugin/finish"
	"github.com/bstr9/simpleclaw/pkg/plugin/godcmd"
	"github.com/bstr9/simpleclaw/pkg/plugin/hello"
	"github.com/bstr9/simpleclaw/pkg/plugin/keyword"
	plinkai "github.com/bstr9/simpleclaw/pkg/plugin/linkai"
	"github.com/bstr9/simpleclaw/pkg/plugin/tool"

	// 内建扩展（渠道扩展）
	_ "github.com/bstr9/simpleclaw/extensions/dingtalk"
	_ "github.com/bstr9/simpleclaw/extensions/feishu"
	_ "github.com/bstr9/simpleclaw/extensions/qq"
	_ "github.com/bstr9/simpleclaw/extensions/terminal"
	_ "github.com/bstr9/simpleclaw/extensions/web"
	_ "github.com/bstr9/simpleclaw/extensions/wechatcom"
	_ "github.com/bstr9/simpleclaw/extensions/wechatmp"
	_ "github.com/bstr9/simpleclaw/extensions/wecombot"
	_ "github.com/bstr9/simpleclaw/extensions/weixin"

	"go.uber.org/zap"
)

// 版本信息，通过 -ldflags 在编译时注入
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// 命令行参数
var (
	configPath  string
	showVersion bool
)

func init() {
	flag.StringVar(&configPath, "config", "./config.json", "配置文件路径")
	flag.StringVar(&configPath, "c", "./config.json", "配置文件路径（简写）")
	flag.BoolVar(&showVersion, "version", false, "显示版本信息")
	flag.BoolVar(&showVersion, "v", false, "显示版本信息（简写）")
}

func main() {
	flag.Parse()

	if showVersion {
		printVersion()
		os.Exit(0)
	}

	printBanner()

	if err := run(); err != nil {
		logger.Fatal("运行失败", zap.Error(err))
	}
}

// run 主运行逻辑
func run() error {
	if err := config.Load(configPath); err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	cfg := config.Get()
	channelTypes := parseChannelTypes(cfg.ChannelType)

	if err := initLogger(channelTypes, cfg.Debug); err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}

	logger.Info("Starting simpleclaw...",
		zap.String("version", version),
		zap.String("commit", commit),
		zap.String("date", date),
		zap.String("config", configPath),
	)

	extMgr := initExtensionSystem(cfg)
	logger.Info("扩展系统已初始化", zap.Int("extensions", len(extMgr.List())))

	if err := extMgr.RegisterAll(); err != nil {
		return fmt.Errorf("注册扩展组件失败: %w", err)
	}

	if err := extMgr.StartupAll(context.Background()); err != nil {
		return fmt.Errorf("启动扩展失败: %w", err)
	}
	logger.Info("扩展已启动")

	registerPlugins()
	logger.Debug("配置信息", zap.Any("config", cfg.MaskSensitive()))

	mgr := channel.NewChannelManager()
	channel.SetChannelManager(mgr)
	logger.Info("渠道类型", zap.Strings("types", channelTypes))

	if err := mgr.Start(channelTypes, channel.WithFirstStart(true)); err != nil {
		return fmt.Errorf("启动渠道失败: %w", err)
	}

	initScheduler()
	apiServer := startAPIServer(cfg)
	waitForShutdown(mgr, extMgr, apiServer)

	return nil
}

// initLogger 初始化日志
func initLogger(channelTypes []string, debug bool) error {
	writeFile := containsTerminal(channelTypes)
	return logger.InitWithOptions(logger.Options{
		Debug:     debug,
		WriteFile: writeFile,
	})
}

// containsTerminal 检查是否包含 terminal 渠道
func containsTerminal(channelTypes []string) bool {
	for _, ct := range channelTypes {
		if ct == "terminal" {
			return true
		}
	}
	return false
}

// registerPlugins 注册内置插件
func registerPlugins() {
	pluginMgr := plugin.GetManager()
	pluginMgr.Register(tool.New())
	pluginMgr.Register(plinkai.New())
	pluginMgr.Register(godcmd.New())
	pluginMgr.Register(banwords.New())
	pluginMgr.Register(keyword.New())
	pluginMgr.Register(hello.New())
	pluginMgr.Register(dungeon.New())
	pluginMgr.Register(agent.New())
	pluginMgr.Register(finish.New())
	logger.Info("内置插件已注册", zap.Int("count", 9))
}

// initScheduler 初始化调度器
func initScheduler() {
	s := scheduler.New(
		scheduler.WithRunner(
			scheduler.NewRunner(
				scheduler.WithExecutor(NewBridgeTaskExecutor()),
			),
		),
	)

	scheduler.SetScheduler(s)

	if err := s.Start(); err != nil {
		logger.Error("调度器启动失败", zap.Error(err))
		return
	}

	logger.Info("调度器已启动")
}

// startAPIServer 启动 API Server
func startAPIServer(cfg *config.Config) *api.Server {
	if !cfg.IsAPIServerEnabled() {
		return nil
	}

	apiCfg := cfg.GetAPIServerConfig()
	server := api.NewServer(&api.Config{
		Host:       apiCfg.Host,
		Port:       apiCfg.Port,
		APIKey:     apiCfg.APIKey,
		EnableCORS: apiCfg.EnableCORS,
		RateLimit:  apiCfg.RateLimit,
	})

	go func() {
		if err := server.Start(); err != nil {
			logger.Error("API Server 启动失败", zap.Error(err))
		}
	}()

	logger.Info("API Server 已启动",
		zap.String("host", apiCfg.Host),
		zap.Int("port", apiCfg.Port))

	return server
}

func waitForShutdown(mgr *channel.ChannelManager, extMgr *extension.Manager, apiServer *api.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info("收到退出信号，正在关闭...", zap.String("signal", sig.String()))

	shutdownAPIServer(apiServer)
	shutdownExtensions(extMgr)
	shutdownScheduler()
	mgr.Shutdown()

	logger.Sync()
	logger.Close()
}

func shutdownAPIServer(server *api.Server) {
	if server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("API Server 关闭失败", zap.Error(err))
	}
}

func shutdownExtensions(extMgr *extension.Manager) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := extMgr.ShutdownAll(ctx); err != nil {
		logger.Error("关闭扩展失败", zap.Error(err))
	}
}

func shutdownScheduler() {
	s := scheduler.GetScheduler()
	if s != nil {
		s.Stop()
		logger.Info("调度器已关闭")
	}
}

// printBanner 打印启动 banner
func printBanner() {
	banner := `
  _   _ _    _ _   _ _____ ___ ___  
 | | | | |  | | \ | |  __ |_ _|   \ 
 | | | | |  | |  \| | |_  || || |) |
 | |_| | |__| | |\  |  _| || ||  _ < 
  \___/|_____|_| \_|_|  |___|___|_\_|
                                      
  UAICLAW 
  Version: %s

`
	fmt.Printf(banner, version)
}

// printVersion 打印版本信息
func printVersion() {
	fmt.Printf("simpleclaw %s\n", version)
	fmt.Printf("  Commit: %s\n", commit)
	fmt.Printf("  Built:  %s\n", date)
}

// parseChannelTypes 解析渠道类型字符串
// 例如: "web,feishu" -> ["web", "feishu"]
func parseChannelTypes(raw string) []string {
	if raw == "" {
		return []string{"web"} // 默认使用 web 渠道
	}

	types := strings.Split(raw, ",")
	result := make([]string, 0, len(types))
	for _, t := range types {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, t)
		}
	}

	if len(result) == 0 {
		return []string{"web"}
	}

	return result
}

// initExtensionSystem 初始化扩展系统
func initExtensionSystem(cfg *config.Config) *extension.Manager {
	workspaceDir := "./data/workspace"
	if cfg.AgentWorkspace != "" {
		workspaceDir = cfg.AgentWorkspace
	}

	extDir := "./extensions"

	api := extension.NewAPI(
		extension.WithAPIWorkingDir(workspaceDir),
		extension.WithAPIExtensionDir(extDir),
	)

	mgr := extension.NewManager(
		extension.WithExtensionDir(extDir),
		extension.WithWorkingDir(workspaceDir),
		extension.WithAPI(api),
	)

	if err := mgr.LoadFromDir(extDir); err != nil {
		logger.Warn("加载扩展目录失败", zap.Error(err))
	}

	if err := mgr.LoadGlobalExtensions(); err != nil {
		logger.Warn("加载全局扩展失败", zap.Error(err))
	}

	return mgr
}

// BridgeTaskExecutor 桥接任务执行器
type BridgeTaskExecutor struct{}

// NewBridgeTaskExecutor 创建桥接任务执行器
func NewBridgeTaskExecutor() *BridgeTaskExecutor {
	return &BridgeTaskExecutor{}
}

// Execute 执行任务
func (e *BridgeTaskExecutor) Execute(ctx context.Context, task *scheduler.Task) (string, error) {
	switch task.Action.Type {
	case scheduler.ActionTypeSendMessage:
		return e.runSendMessage(task)
	case scheduler.ActionTypeAgentTask:
		return e.runAgentTask(task)
	default:
		return "", fmt.Errorf("未知的任务类型: %s", task.Action.Type)
	}
}

// runSendMessage 执行发送消息任务
func (e *BridgeTaskExecutor) runSendMessage(task *scheduler.Task) (string, error) {
	content := task.Action.Content
	if content == "" {
		return "", errors.New("消息内容为空")
	}

	notifyContent := fmt.Sprintf("[定时提醒] %s", content)
	kwargs := e.buildMessageKwargs(task)
	msgCtx := e.buildMessageContext(task, kwargs, notifyContent)
	reply := types.NewTextReply(notifyContent)

	// 尝试通过指定渠道发送
	if sent := e.sendViaSpecificChannel(task, reply, msgCtx); sent {
		return "消息已发送", nil
	}

	// 回退：使用主渠道发送
	if sent := e.sendViaPrimaryChannel(reply, msgCtx); sent {
		return "消息已发送", nil
	}

	return notifyContent, nil
}

// buildMessageKwargs 构建消息的 kwargs 参数
func (e *BridgeTaskExecutor) buildMessageKwargs(task *scheduler.Task) map[string]any {
	kwargs := map[string]any{
		"session_id": "scheduler_" + task.ID,
		"user_id":    "scheduler",
		"is_group":   false,
	}

	if task.Action.Context == nil {
		return kwargs
	}

	kwargs["session_id"] = task.Action.Context.SessionID
	kwargs["user_id"] = task.Action.Context.UserID
	kwargs["is_group"] = task.Action.Context.IsGroup

	if task.Action.Context.GroupID != "" {
		kwargs["group_id"] = task.Action.Context.GroupID
	}

	return kwargs
}

// buildMessageContext 构建消息上下文
func (e *BridgeTaskExecutor) buildMessageContext(task *scheduler.Task, kwargs map[string]any, notifyContent string) *types.Context {
	msgCtx := types.NewContextWithKwargs(types.ContextText, notifyContent, kwargs)

	if task.Action.Context == nil {
		return msgCtx
	}

	if task.Action.Context.Receiver != "" {
		msgCtx.Set("receiver", task.Action.Context.Receiver)
	}
	if task.Action.Context.ReceiveIDType != "" {
		msgCtx.Set("receive_id_type", task.Action.Context.ReceiveIDType)
	}

	return msgCtx
}

// sendViaSpecificChannel 通过指定渠道发送消息
func (e *BridgeTaskExecutor) sendViaSpecificChannel(task *scheduler.Task, reply *types.Reply, msgCtx *types.Context) bool {
	if task.Action.Context == nil || task.Action.Context.ChannelType == "" {
		return false
	}

	ch := channel.GetChannelManager().GetChannel(task.Action.Context.ChannelType)
	if ch == nil {
		return false
	}

	return ch.Send(reply, msgCtx) == nil
}

// sendViaPrimaryChannel 通过主渠道发送消息
func (e *BridgeTaskExecutor) sendViaPrimaryChannel(reply *types.Reply, msgCtx *types.Context) bool {
	ch := channel.GetChannelManager().PrimaryChannel()
	if ch == nil {
		return false
	}

	return ch.Send(reply, msgCtx) == nil
}

// runAgentTask 执行 AI 任务
func (e *BridgeTaskExecutor) runAgentTask(task *scheduler.Task) (string, error) {
	description := task.Action.TaskDescription
	if description == "" {
		return "", errors.New("任务描述为空")
	}

	ab := bridge.GetBridge().GetAgentBridge()
	if ab == nil {
		return "", errors.New("agent bridge 未初始化")
	}

	msgCtx := e.buildAgentTaskContext(task)
	reply, err := bridge.GetBridge().FetchAgentReply(description, msgCtx, nil)
	if err != nil {
		return "", fmt.Errorf("执行 AI 任务失败: %w", err)
	}

	return e.sendAgentTaskReply(task, reply, msgCtx)
}

// buildAgentTaskContext 构建任务上下文
func (e *BridgeTaskExecutor) buildAgentTaskContext(task *scheduler.Task) *types.Context {
	kwargs := map[string]any{
		"session_id": "scheduler_" + task.ID,
		"user_id":    "scheduler",
		"is_group":   false,
	}

	var receiver, receiveIDType string
	if task.Action.Context != nil {
		kwargs["session_id"] = task.Action.Context.SessionID
		kwargs["user_id"] = task.Action.Context.UserID
		kwargs["is_group"] = task.Action.Context.IsGroup
		if task.Action.Context.GroupID != "" {
			kwargs["group_id"] = task.Action.Context.GroupID
		}
		receiver = task.Action.Context.Receiver
		receiveIDType = task.Action.Context.ReceiveIDType
	}

	msgCtx := types.NewContextWithKwargs(types.ContextText, "", kwargs)
	if receiver != "" {
		msgCtx.Set("receiver", receiver)
	}
	if receiveIDType != "" {
		msgCtx.Set("receive_id_type", receiveIDType)
	}

	return msgCtx
}

// sendAgentTaskReply 发送任务回复
func (e *BridgeTaskExecutor) sendAgentTaskReply(task *scheduler.Task, reply *types.Reply, msgCtx *types.Context) (string, error) {
	if task.Action.Context != nil && task.Action.Context.ChannelType != "" {
		ch := channel.GetChannelManager().GetChannel(task.Action.Context.ChannelType)
		if ch != nil {
			if err := ch.Send(reply, msgCtx); err != nil {
				return "", fmt.Errorf("发送消息失败: %w", err)
			}
			return "消息已发送", nil
		}
	}

	ch := channel.GetChannelManager().PrimaryChannel()
	if ch != nil {
		if err := ch.Send(reply, msgCtx); err != nil {
			return "", fmt.Errorf("发送消息失败: %w", err)
		}
		return "消息已发送", nil
	}

	return reply.StringContent(), nil
}
