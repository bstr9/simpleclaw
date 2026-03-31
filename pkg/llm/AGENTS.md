# AGENTS.md — LLM 适配层

**目录:** pkg/llm/ | **文件:** 17 | **深度:** 3

---

## OVERVIEW

17个 LLM 提供商统一接口。OpenAI 兼容 API 为基础，特殊提供商独立实现。

---

## PROVIDERS

| 提供商 | 文件 | 特点 |
|--------|------|------|
| OpenAI | openai.go | 基础实现 |
| ChatGPT | chatgpt.go | OpenAI 变体 |
| Claude | claude.go | 流式 + 多模态 (785行) |
| Gemini | gemini.go | Google API (670行) |
| DashScope | dashscope.go | 通义千问 (886行) |
| DeepSeek | deepseek.go | 兼容 OpenAI |
| Qwen | qwen.go | 兼容 OpenAI |
| Zhipu | zhipu.go | GLM 系列 |
| Moonshot | moonshot.go | Kimi |
| Minimax | minimax.go | 独立 API |
| Baidu | baidu.go | 文心一言 |
| Xunfei | xunfei.go | WebSocket API |
| Doubao | doubao.go | 字节豆包 |
| ModelScope | modelscope.go | 阿里 ModelScope |
| LinkAI | linkai.go | LinkAI 集成 |

---

## MODEL INTERFACE

```go
type Model interface {
    Call(ctx context.Context, messages []Message, opts ...Option) (*Response, error)
    Stream(ctx context.Context, messages []Message, opts ...Option) (*Stream, error)
}
```

---

## PROVIDER DETECTION

```go
// factory.go
func NewModel(cfg *config.Config) (Model, error) {
    switch cfg.Model {
    case OpenAI, Azure, DeepSeek, Qwen:
        return NewOpenAIModel(cfg)
    case Claude:
        return NewClaudeModel(cfg)
    // ...
    }
}
```

---

## CONVENTIONS

- OpenAI 兼容提供商复用 `openai.go`
- 特殊 API (Claude, Gemini, Xunfei) 独立实现
- 使用 `llm.Option` 函数式选项

---

## HOTSPOTS

| 文件 | 行数 | 说明 |
|------|------|------|
| `dashscope.go` | 886 | 多模态 + 流式 |
| `claude.go` | 785 | Anthropic API |
| `gemini.go` | 670 | Google API |
