# AGENTS.md — 插件系统

**目录:** pkg/plugin/ | **文件:** 15+ | **深度:** 5

---

## OVERVIEW

事件驱动插件架构。支持消息处理、敏感词过滤、命令扩展等。

---

## PLUGINS

| 插件 | 目录 | 说明 |
|------|------|------|
| tool | tool/ | 工具调用框架 (1654行) |
| linkai | linkai/ | LinkAI 集成 (1945行) |
| godcmd | godcmd/ | 管理员命令 (768行) |
| banwords | banwords/ | 敏感词过滤 |
| keyword | keyword/ | 关键词回复 |
| hello | hello/ | 示例插件 |
| dungeon | dungeon/ | 游戏 |
| finish | finish/ | 结束处理 |
| agent | agent/ | Agent 封装 |

---

## PLUGIN INTERFACE

```go
type Plugin interface {
    OnInit(config *Config) error
    OnLoad() error
    OnEvent(event *Event) EventAction
    OnUnload() error
}
```

---

## BASE PLUGIN

```go
type MyPlugin struct {
    *plugin.BasePlugin  // 嵌入获取默认实现
}
```

---

## EVENT ACTIONS

| 动作 | 说明 |
|------|------|
| `ActionContinue` | 继续下一个处理器 |
| `ActionBreak` | 停止并执行默认逻辑 |
| `ActionBreakPass` | 停止并跳过默认 |

---

## EVENT TYPES

```
EventOnReceiveMessage → EventOnHandleContext → EventOnDecorateReply → EventOnSendReply
```

---

## HOTSPOTS

| 文件 | 行数 | 建议 |
|------|------|------|
| `linkai/linkai.go` | 1945 | 拆分 knowledge/midjourney/summary |
| `tool/tool.go` | 1654 | 拆分 registry/executor/builtin |
| `godcmd/godcmd.go` | 768 | 提取权限管理 |
