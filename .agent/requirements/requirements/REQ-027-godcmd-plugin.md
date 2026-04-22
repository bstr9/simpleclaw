---
id: REQ-027
title: "Godcmd 插件：管理员认证、插件管理与配置重载"
status: active
level: story
priority: P0
cluster: plugins
created_at: "2026-04-23T16:00:00"
updated_at: "2026-04-23T16:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-006]
  merged_from: []
  depends_on: [REQ-004]
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/plugin/godcmd/godcmd.go (768行)"
    reason: "逆向代码生成需求"
    snapshot: "管理员命令插件：密码认证+白名单、12条命令（服务控制/插件管理/模型切换）、临时口令生成"
---

# Godcmd 插件：管理员认证、插件管理与配置重载

## 描述
Godcmd 插件提供管理员命令框架，以 "#" 前缀触发。核心功能：(1) 认证系统，支持配置密码和临时口令两种方式，通过 AdminUsers 白名单管理权限；(2) 12 条管理命令，涵盖服务暂停/恢复、配置重载、插件生命周期管理、模型切换；(3) 双层命令体系，通用命令（auth/help/model/id/reset）无需管理员权限，管理命令需先认证。

## 验收标准
- [x] GodcmdPlugin 结构体：嵌入 `BasePlugin`，持有 `config *Config`、`tempPassword string`、`isRunning bool`、`rng *rand.Rand`、`debugMode bool` (godcmd.go:124-134)
- [x] 配置结构体 `Config`：`Password string`、`AdminUsers []string` (godcmd.go:118-122)
- [x] 优先级 999，`SetHidden(true)`，确保在其他插件之后执行 (godcmd.go:145-146)
- [x] 通用命令表 `commands`：`help`（帮助）、`helpp`（插件帮助）、`auth`（认证）、`model`（模型查看/设置）、`id`（获取用户ID）、`reset`（重置会话），含中英文别名 (godcmd.go:29-57)
- [x] 管理员命令表 `adminCommands`：`resume`/`stop`（服务控制）、`reconf`（重载配置）、`resetall`（重置所有会话）、`scanp`（扫描插件）、`plist`（插件列表）、`setpri`（设置优先级）、`reloadp`（重载插件）、`enablep`/`disablep`（启禁用插件）、`debug`（调试模式） (godcmd.go:60-109)
- [x] 命令信息结构 `commandInfo`：`alias []string`、`args []string`、`desc string` (godcmd.go:112-116)
- [x] 消息触发：`onHandleContext` 检查 `"#"` 前缀，调用 `parseCommand` 解析命令和参数 (godcmd.go:214-246)
- [x] 命令分发 `dispatchCommand`：先查 `commands`（通用），再查 `adminCommands`（管理），管理命令需权限检查 (godcmd.go:279-289)
- [x] 认证流程 `authenticate`：禁止群聊认证 → 检查 AdminUsers 白名单 → 比对 Password → 比对 tempPassword → 成功后追加到 AdminUsers (godcmd.go:596-630)
- [x] 临时口令 `generateTempPassword`：4 位随机数字，在 Password 为空时自动生成并通过日志输出 (godcmd.go:717-725)
- [x] 权限检查 `isAdmin(userID)`：遍历 `config.AdminUsers` 匹配 (godcmd.go:633-642)
- [x] 管理员命令权限守卫 `handleAdminCommandWithCheck`：`!isAdmin` 返回"需要管理员权限"，`isGroup` 返回"群聊不可执行管理员指令" (godcmd.go:293-299)
- [x] 服务控制 `handleStop`/`handleResume`：设置 `isRunning` 标志，`breakPassIfNeeded` 在非运行状态拦截消息 (godcmd.go:352-366, 258-262)
- [x] 配置重载 `handleReconf`：调用 `config.Reload()` + `bridge.GetBridge().Reset()` (godcmd.go:470-479)
- [x] 插件扫描 `handleScanPlugins`：调用 `mgr.ScanPlugins()` 返回新发现插件列表 (godcmd.go:531-546)
- [x] 插件列表 `handlePluginList`：调用 `mgr.ListPlugins()` 展示名称、版本、优先级、启用状态 (godcmd.go:507-528)
- [x] 优先级设置 `handleSetPriority`：调用 `mgr.SetPluginPriority(pluginName, priority)` (godcmd.go:549-561)
- [x] 插件重载 `handleReloadPlugin`：调用 `mgr.ReloadPlugin(pluginName)` (godcmd.go:564-571)
- [x] 插件启禁用 `handleEnablePlugin`/`handleDisablePlugin`：调用 `mgr.EnablePlugin`/`mgr.DisablePlugin` (godcmd.go:574-593)
- [x] 模型管理 `handleModel`：无参数显示当前模型，有参数设置 `cfg.Model` 并调用 `bridge.GetBridge().Reset()` (godcmd.go:434-455)
- [x] 调试模式 `handleDebug`：切换 `debugMode`，调用 `logger.SetLevel(zapcore.DebugLevel/InfoLevel)` (godcmd.go:489-504)
- [x] 帮助文本 `HelpText()`：展示通用命令 + 可用插件列表 + 管理员命令（仅管理员可见），支持 `HelpTextProvider` 接口 (godcmd.go:766-768)
- [x] 命令查找 `findCommand`：遍历命令表的 `alias` 数组，使用 `strings.EqualFold` 大小写不敏感匹配 (godcmd.go:645-654)
