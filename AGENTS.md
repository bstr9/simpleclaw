# AGENTS.md — SimpleClaw

**Generated:** 2025-03-25 | **Commit:** 7e6a635 | **Branch:** master
**Language:** Go 1.25 | **Module:** `github.com/bstr9/simpleclaw`

---

## OVERVIEW

多渠道 AI Agent 框架，支持 Function Calling、工具调用、记忆系统。企业定制化 UII-AI 平台核心组件。

---

## QUICK START

```bash
go build -o simpleclaw ./cmd/simpleclaw    # 编译
./simpleclaw -config config.json          # 运行
go test ./...                           # 测试
go vet ./...                            # 检查
deadcode ./...                          # 死代码检查
```

---

## STRUCTURE

```
simpleclaw/
├── cmd/simpleclaw/main.go     # 唯一入口，渠道启动
├── pkg/
│   ├── agent/               # Agent核心 → 见 pkg/agent/AGENTS.md
│   ├── bridge/              # 渠道-LLM桥接，消息路由
│   ├── channel/             # 10渠道 → 见 pkg/channel/AGENTS.md
│   ├── llm/                 # 15+ LLM → 见 pkg/llm/AGENTS.md
│   ├── plugin/              # 9插件 → 见 pkg/plugin/AGENTS.md
│   ├── voice/               # 9语音平台 → 见 pkg/voice/AGENTS.md
│   ├── types/               # 核心类型 (Context, Reply, Message)
│   ├── config/              # 配置加载，工具配置
│   ├── common/              # 工具函数，ExpireMap，TokenBucket
│   └── logger/              # zap 日志封装
└── data/workspace/          # Agent工作空间
```

---

## WHERE TO LOOK

| 任务 | 位置 | 说明 |
|------|------|------|
| 理解启动流程 | `cmd/simpleclaw/main.go` | 配置加载→渠道启动→信号处理 |
| 添加新渠道 | `pkg/channel/` | 复制 `terminal/` 模板 |
| 添加新LLM | `pkg/llm/` | 参考 `openai.go` |
| 添加新工具 | `pkg/agent/tools/` | 实现 `Tool` 接口 |
| 添加新插件 | `pkg/plugin/` | 嵌入 `BasePlugin` |
| 理解消息流 | `pkg/bridge/` | 渠道→LLM→Agent 桥接 |
| 配置工具 | `pkg/config/types.tools.go` | WebSearch/WebFetch 配置 |

---

## CODE MAP

### 核心接口

| 接口 | 位置 | 职责 |
|------|------|------|
| `channel.Channel` | `pkg/channel/channel.go` | 所有渠道统一接口 |
| `llm.Model` | `pkg/llm/model.go` | 所有LLM统一接口 |
| `agent.Tool` | `pkg/agent/tools/tools.go` | 工具接口定义 |
| `plugin.Plugin` | `pkg/plugin/plugin.go` | 插件生命周期 |
| `voice.VoiceEngine` | `pkg/voice/voice.go` | TTS/STT接口 |
| `memory.Storage` | `pkg/agent/memory/storage.go` | 记忆存储接口 |

### 工厂模式 (init自动注册)

| 工厂 | 注册函数 | 用途 |
|------|----------|------|
| `channel/factory.go` | `RegisterChannel()` | 渠道注册 |
| `llm/factory.go` | `RegisterProvider()` | LLM提供商注册 |
| `voice/factory.go` | `RegisterEngine()` | 语音引擎注册 |

### 复杂度热点 (>800行)

| 文件 | 行数 | 重构建议 |
|------|------|----------|
| `pkg/plugin/linkai/linkai.go` | 1853 | 拆分: knowledge/midjourney/summary |
| `pkg/plugin/tool/tool.go` | 1523 | 拆分: registry/executor/builtin |
| `pkg/channel/weixin/weixin_channel.go` | 1199 | 拆分: login/message/api |
| `pkg/agent/memory/long_term.go` | 1011 | 拆分: sqlite/vector/indexer |

---

## CODING CONVENTIONS

### 注释规范 (强制)

**所有注释必须使用中文**

```go
// 正确
// NewAgent 创建新的 Agent 实例
func NewAgent(opts ...Option) *Agent { ... }

// 错误
// NewAgent creates a new Agent instance
func NewAgent(opts ...Option) *Agent { ... }
```

### 代码风格

- `gofmt` 格式化
- `go vet` 检查
- 导出函数必须有中文注释
- 新代码用 `deadcode` 检查

---

## ANTI-PATTERNS

| 禁止 | 原因 | 位置 |
|------|------|------|
| 写入敏感信息 | API密钥、令牌禁止写入文件 | `pkg/agent/prompt/builder.go:211` |
| 技能当工具调用 | 技能不能直接调用，需解析 | `pkg/agent/prompt/builder.go:164` |
| 批量读取技能 | 永远不要一次读多个技能 | `pkg/agent/prompt/builder.go:164` |
| sed/批量替换字符串 | 容易破坏代码结构，必须逐文件手动修改 | 全项目 |

---

## CHANNELS (10)

terminal, web, feishu, dingtalk, weixin, wechatmp, wechatcom, wecombot, qq

---

## TOOLS (15)

read, write, edit, ls, bash, web_search, web_fetch, browser, memory, scheduler, vision, time, env_config, send, lark_cli

---

## FEISHU INTEGRATION

飞书扩展集成官方 lark-cli，提供：
- **lark_cli 工具**: 封装 200+ 命令
- **19 个官方 Skills**: 自动注册 `~/.agents/skills/`

详见 `extensions/AGENTS.md`

---

## INTEGRATION STATUS

所有核心功能已实现：
- ✅ Memory 系统 (短期/长期记忆，向量存储)
- ✅ Voice TTS/STT (9个语音平台)
- ✅ Skills 系统 (技能注册与加载)
- ✅ Web/Weixin 渠道 (9个渠道)
- ✅ BashTool (Shell 命令执行)
- ✅ Plugins (9个插件)
