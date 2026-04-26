---
id: REQ-049
title: "扩展系统"
status: active
level: story
priority: P2
cluster: bridge
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-008]
  merged_from: []
  depends_on: [REQ-001]
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "扩展管理框架，支持渠道迁移到 extensions/ 和工具注册"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "逆向代码分析，扩展验收标准至完整覆盖"
    reason: "从现有代码逆向补充详细验收标准"
    snapshot: "扩展系统：Extension接口（6方法）、ExtensionAPI接口（9方法）、Manager全局注册与生命周期管理、API工具/渠道/技能/事件注册、Go插件加载"
source_code:
  - pkg/extension/api.go
  - pkg/extension/manager.go
  - pkg/extension/extension.go
  - pkg/extension/extension_test.go
  - pkg/extension/registry/registry.go
---

# 扩展系统

## 描述
扩展管理框架，支持渠道从 pkg/channel 迁移到 extensions/。Extension Manager 加载和管理扩展，Registry 提供工具注册（GetTools 被 AgentInitializer.loadTools 消费）。通过扩展机制实现渠道的插件化加载，将核心框架与具体渠道实现解耦，支持第三方扩展开发。Extension 接口定义完整生命周期（注册→启动→关闭），ExtensionAPI 提供渠道注册、工具注册、技能路径、事件系统等能力，Manager 负责全局注册表管理和 Go 插件动态加载。

## 验收标准

### Extension 接口
- [x] Extension 接口定义 ID() string 方法，返回扩展唯一标识
- [x] Extension 接口定义 Name() string 方法，返回扩展名称
- [x] Extension 接口定义 Description() string 方法，返回扩展描述
- [x] Extension 接口定义 Version() string 方法，返回扩展版本号
- [x] Extension 接口定义 Register(api ExtensionAPI) error 方法，注册扩展能力
- [x] Extension 接口定义 Startup(ctx context.Context) error 方法，启动扩展
- [x] Extension 接口定义 Shutdown(ctx context.Context) error 方法，关闭扩展

### ExtensionAPI 接口
- [x] RegisterChannel(name, creator) 注册渠道创建器
- [x] RegisterTool(name, tool) 注册工具
- [x] RegisterSkillPath(path) 注册技能路径
- [x] RegisterEventHandler(name, handler) 注册事件处理器
- [x] EmitEvent(name string, data interface{}) 触发事件
- [x] Config(key string) interface{} 获取配置值
- [x] ConfigString(key string) string 获取字符串配置值
- [x] WorkingDir() string 获取工作目录
- [x] ExtensionDir() string 获取扩展目录
- [x] GetSkillPaths() []string 获取所有已注册技能路径

### 辅助类型
- [x] EventHandler 类型定义：func(data interface{}) error
- [x] ConfigSchema 结构体：Name + Properties map[string]PropertySchema + Required []string
- [x] PropertySchema 结构体：Type + Description + Default
- [x] ExtensionInfo 结构体：ID/Name/Description/Version + ConfigSchema 指针

### 全局注册表 (globalRegistry)
- [x] globalRegistry 结构体：sync.RWMutex + extensions map[string]Extension 并发安全
- [x] RegisterExtension(ext Extension) 注册扩展到全局注册表
- [x] GetGlobalExtensions() 获取所有全局扩展列表

### Manager 扩展管理器
- [x] Manager 结构体：extensions map + api + wg + mu + ctx 并发安全
- [x] Manager.Register(ext) 注册扩展到管理器
- [x] Manager.Unregister(id) 注销扩展
- [x] Manager.Get(id) 获取指定扩展
- [x] Manager.List() 列出所有扩展
- [x] Manager.ListInfo() 列出所有扩展的 ExtensionInfo
- [x] Manager.StartupAll() 启动所有已注册扩展
- [x] Manager.ShutdownAll() 关闭所有已注册扩展（使用 WaitGroup 等待）
- [x] Manager.RegisterAll(api) 批量注册所有扩展
- [x] Manager.LoadFromDir(dir) 从目录加载扩展（.so 插件）
- [x] Manager.LoadGlobalExtensions() 加载全局注册表中的扩展

### API 实现
- [x] API 结构体：channelCreators + toolRegistry + skillPaths + eventHandlers + config + workingDir + extensionDir
- [x] NewAPI(opts ...APIOption) 构造函数，支持函数选项模式
- [x] API.GetChannelCreators() 获取所有渠道创建器
- [x] API.ResolvePath(path) 解析扩展相对路径为绝对路径
- [x] GetChannelRegistry() 全局函数获取渠道注册表

### Go 插件加载
- [x] loadExtension(path) 加载 Go 插件（.so 文件）
- [x] loadPluginExtension(pluginPath) 从插件目录加载扩展

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| Extension 接口 (6方法) | `pkg/extension/extension.go` Extension 接口定义 |
| EventHandler 类型 | `pkg/extension/extension.go` EventHandler 类型定义 |
| ConfigSchema/PropertySchema | `pkg/extension/extension.go` 结构体定义 |
| ExtensionInfo | `pkg/extension/extension.go` ExtensionInfo 结构体 |
| ExtensionAPI 接口 (9方法) | `pkg/extension/extension.go` ExtensionAPI 接口定义 |
| 全局注册表 | `pkg/extension/manager.go` globalRegistry/RegisterExtension/GetGlobalExtensions |
| Manager 注册/注销/查询 | `pkg/extension/manager.go` Manager.Register/Unregister/Get/List/ListInfo |
| Manager 生命周期 | `pkg/extension/manager.go` Manager.StartupAll/ShutdownAll/RegisterAll |
| Manager 目录加载 | `pkg/extension/manager.go` Manager.LoadFromDir/LoadGlobalExtensions |
| Go 插件加载 | `pkg/extension/manager.go` loadExtension/loadPluginExtension |
| API 结构体与构造 | `pkg/extension/api.go` API/NewAPI/APIOption |
| API 渠道/工具注册 | `pkg/extension/api.go` RegisterChannel/RegisterTool |
| API 技能/事件 | `pkg/extension/api.go` RegisterSkillPath/RegisterEventHandler/EmitEvent |
| API 配置/路径 | `pkg/extension/api.go` Config/ConfigString/WorkingDir/ExtensionDir/ResolvePath |
| GetChannelRegistry | `pkg/extension/api.go` GetChannelRegistry 全局函数 |
