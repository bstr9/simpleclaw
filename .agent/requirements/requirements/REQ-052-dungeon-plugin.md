---
id: REQ-052
title: "Dungeon 插件"
status: active
level: story
priority: P2
cluster: plugins
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-006]
  merged_from: []
  depends_on: []
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "文字冒险游戏插件，提供交互式 RPG 体验"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "细化验收标准，从源代码逆向补充实现细节"
    reason: "验收标准从4项扩展至19项，覆盖游戏会话管理、命令解析、故事生成、配置与清理等"
    snapshot: "Dungeon文字冒险游戏插件，StoryTeller会话管理(firstInteract/story/mu)，$开始冒险/$停止冒险命令，Action()首尾交互差异化prompt，games map并发安全，cleanupExpiredSessions协程，Config(Enabled/TriggerPrefix/DefaultStory/SessionTimeout)"
source_code:
  - pkg/plugin/dungeon/
---

# Dungeon 插件

## 描述
文字冒险游戏插件，提供交互式 RPG 体验。DungeonPlugin 嵌入 BasePlugin，priority=0，维护 games map 管理多会话的 StoryTeller 实例。支持 `$开始冒险 [背景故事]` 创建新游戏、`$停止冒险` 结束游戏、游戏内任意文本推进剧情。StoryTeller 首次交互生成完整开场 prompt（含故事背景+用户行动），后续交互使用续写 prompt（4-6 句节奏控制）。配置支持 Enabled 开关、TriggerPrefix 自定义前缀、DefaultStory 默认故事、SessionTimeout 超时。OnInit 启动会话清理协程。

## 代码参考

| 功能 | 文件 | 行号 |
|------|------|------|
| StoryTeller 结构体 | pkg/plugin/dungeon/dungeon.go | 15-26 |
| DungeonPlugin 结构体 | pkg/plugin/dungeon/dungeon.go | 75-81 |
| Config 配置结构体 | pkg/plugin/dungeon/dungeon.go | 84-96 |
| New() 构造函数 | pkg/plugin/dungeon/dungeon.go | 102-119 |
| OnInit 加载配置+启动清理协程 | pkg/plugin/dungeon/dungeon.go | 132-156 |
| OnLoad 注册事件处理器 | pkg/plugin/dungeon/dungeon.go | 159-164 |
| OnUnload 卸载清理 | pkg/plugin/dungeon/dungeon.go | 167-173 |
| onHandleContext 消息处理 | pkg/plugin/dungeon/dungeon.go | 181-263 |
| $停止冒险 命令处理 | pkg/plugin/dungeon/dungeon.go | 219-230 |
| $开始冒险 命令处理 | pkg/plugin/dungeon/dungeon.go | 233-247 |
| 游戏内消息处理 | pkg/plugin/dungeon/dungeon.go | 250-262 |
| StoryTeller.Action | pkg/plugin/dungeon/dungeon.go | 46-65 |
| StoryTeller.Reset | pkg/plugin/dungeon/dungeon.go | 39-43 |
| cleanupExpiredSessions | pkg/plugin/dungeon/dungeon.go | 266-274 |
| GetHelpText/HelpText | pkg/plugin/dungeon/dungeon.go | 277-301 |

## 验收标准
- [x] DungeonPlugin 嵌入 BasePlugin，实现 Plugin 接口（编译期校验 var _ plugin.Plugin）
- [x] 插件名称 "dungeon"，版本 "1.0.0"，priority=0
- [x] StoryTeller 结构体管理单会话状态：bot、sessionID、firstInteract、story、mu（读写锁）
- [x] StoryTeller.Reset() 重置 firstInteract 为 true
- [x] StoryTeller.Action() 首次交互生成完整开场 prompt（故事背景+用户行动，4-6句节奏描述），后续交互使用续写 prompt
- [x] StoryTeller.Action() 自动为用户行动补充句号结尾
- [x] Config 结构体定义 4 个字段：Enabled、TriggerPrefix、DefaultStory、SessionTimeout
- [x] 默认配置：Enabled=true、TriggerPrefix="$"、SessionTimeout=3600、DefaultStory 为树林冒险
- [x] OnInit 从 PluginContext.Config 加载配置（类型断言 float64→int），启动 cleanupExpiredSessions 协程
- [x] OnLoad 注册 EventOnHandleContext 事件处理器
- [x] OnUnload 清空 games map，调用 BasePlugin.OnUnload
- [x] onHandleContext 检查 Enabled 开关，非启用时直接返回
- [x] onHandleContext 只处理文本消息，获取 sessionID（缺省 "default"）
- [x] `$停止冒险` 命令：Reset 游戏状态 + 从 games map 删除 + 回复 "冒险结束!" + BreakPass
- [x] `$开始冒险 [背景故事]` 命令：创建新 StoryTeller，存入 games map，回复含故事背景的确认信息 + BreakPass
- [x] `$开始冒险` 支持自定义背景故事参数（arg），缺省使用 DefaultStory
- [x] 游戏内消息：查找 sessionID 对应的 StoryTeller，调用 Action() 生成 prompt，设为文本内容 + Break
- [x] games map 读写锁保护，所有访问均通过 p.mu.RLock/RLock
- [x] cleanupExpiredSessions 启动定时器协程（1分钟间隔），当前为 no-op 预留扩展
- [x] GetHelpText(verbose) 返回简短或详细帮助，HelpText() 返回简短版本
- [x] 详细帮助包含 $开始冒险/$停止冒险 命令说明和示例
