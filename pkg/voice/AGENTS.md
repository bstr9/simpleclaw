# AGENTS.md — 语音系统

**目录:** pkg/voice/ | **文件:** 20+ | **深度:** 4

---

## OVERVIEW

9个 TTS/STT 平台适配。统一 `VoiceEngine` 接口。

---

## ENGINES

| 平台 | 目录 | TTS | STT |
|------|------|-----|-----|
| OpenAI | openai/ | ✓ | ✓ |
| Azure | azure/ | ✓ | ✓ |
| Baidu | baidu/ | ✓ | ✓ |
| Ali | ali/ | ✓ | ✓ |
| Tencent | tencent/ | ✓ | ✓ |
| Xunfei | xunfei/ | ✓ | ✓ |
| Google | google/ | ✓ | ✓ |
| Edge | edge/ | ✓ | ✗ |
| Eleven | elevent/ | ✓ | ✗ |

---

## ENGINE INTERFACE

```go
type VoiceEngine interface {
    TTS(ctx context.Context, text string, opts ...Option) ([]byte, error)
    STT(ctx context.Context, audio []byte, opts ...Option) (string, error)
}
```

---

## FACTORY PATTERN

```go
// factory.go
func NewEngineWithType(engineType string, opts ...Option) (VoiceEngine, error) {
    switch engineType {
    case EngineOpenAI:
        return openai.NewEngine(opts...)
    // ...
    }
}
```

---

## AUDIO CONVERT

`convert.go` 提供格式转换：
- 需要 `ffmpeg` 进行格式转换
- SILK 编解码需要 `go-silk` 库

---

## CONVENTIONS

- 在 `init()` 中调用 `voice.RegisterEngine()`
- 使用 `voice.Option` 函数式选项
- 不支持 STT 的引擎返回错误
