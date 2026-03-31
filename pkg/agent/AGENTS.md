# AGENTS.md — Agent 核心模块

**目录:** pkg/agent/ | **文件:** 30+ | **深度:** 6

---

## OVERVIEW

Agent 执行引擎、工具系统、记忆管理、提示词构建、技能注册。核心抽象层。

---

## STRUCTURE

```
pkg/agent/
├── agent.go           # Agent 主结构
├── executor.go        # 执行循环
├── tools/             # 内置工具 → 见 AGENTS.md
├── memory/            # 记忆系统 → 见 AGENTS.md
├── skills/            # 技能注册表
├── prompt/            # 提示词构建器
├── protocol/          # 消息协议定义
└── chat/              # 会话管理
```

---

## WHERE TO LOOK

| 任务 | 位置 |
|------|------|
| 添加工具 | `tools/tools.go` → 实现 `Tool` 接口 |
| 工具执行 | `executor.go` → `Execute()` |
| 记忆存储 | `memory/storage.go` → `Storage` 接口 |
| 技能加载 | `skills/registry.go` → `LoadSkill()` |
| 提示词 | `prompt/builder.go` → `Build()` |

---

## CORE INTERFACES

### Tool 接口
```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any
    Execute(ctx context.Context, params map[string]any) (*ToolResult, error)
}
```

### Memory 接口
```go
type Storage interface {
    Save(ctx context.Context, memory *Memory) error
    Search(ctx context.Context, query string, opts ...SearchOption) ([]*Memory, error)
}
```

---

## CONVENTIONS

- 工具返回 `*ToolResult`，包含 `Success`, `Output`, `Error`
- 记忆三层作用域: `shared`, `user`, `session`
- 技能从 Markdown 文件加载，存放在 workspace
- 提示词禁止包含敏感信息

---

## HOTSPOTS

| 文件 | 行数 | 说明 |
|------|------|------|
| `memory/long_term.go` | 1011 | 向量搜索 + SQLite |
| `skills/registry.go` | 684 | 技能解析 |
| `chat/chat.go` | 539 | 对话管理 |
