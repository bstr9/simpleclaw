# AGENTS.md — 工具系统

**目录:** pkg/agent/tools/ | **文件:** 18 | **深度:** 4

---

## OVERVIEW

内置工具实现：文件操作、网络请求、浏览器控制、记忆管理、定时任务。

---

## BUILTIN TOOLS

| 工具 | 文件 | 说明 |
|------|------|------|
| read | read.go | 读取文件内容 |
| write | write.go | 创建/覆盖文件 (限10KB) |
| edit | edit.go | 精确字符串替换 |
| ls | ls.go | 目录列表 |
| bash | bash.go | Shell 命令 (含禁止列表) |
| web_search | web_search.go | 多提供商搜索 |
| web_fetch | web_fetch.go | 网页内容获取 |
| browser | browser.go | 浏览器自动化 |
| memory | memory_tool.go | 记忆读写 |
| scheduler | scheduler.go | 定时任务 |
| vision | vision.go | 图像识别 |
| time | time.go | 时间工具 |

---

## TOOL INTERFACE

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]any  // JSON Schema
    Execute(ctx context.Context, params map[string]any) (*ToolResult, error)
}

type ToolResult struct {
    Success bool
    Output  string
    Error   string
}
```

---

## REGISTER PATTERN

```go
// tools/tools.go
func RegisterBuiltInTools(registry *ToolRegistry) {
    registry.Register(NewReadTool())
    registry.Register(NewWriteTool())
    // ...
}
```

---

## CONVENTIONS

- 参数使用 JSON Schema 定义
- 返回 `*ToolResult`，不返回 Go error
- Bash 工具支持 `WithBashDenyList` 禁止危险命令
- Write 工具单次写入限 10KB
