# AGENTS.md — 渠道系统

**目录:** pkg/channel/ | **文件:** 30+ | **深度:** 5

---

## OVERVIEW

9个消息渠道：终端、Web、飞书、钉钉、微信、QQ等。统一 `Channel` 接口。

---

## CHANNELS

| 渠道 | 目录 | 复杂度 |
|------|------|--------|
| terminal | terminal/ | 低 |
| web | web/ | 中 |
| feishu | feishu/ | 高 (850行) |
| dingtalk | dingtalk/ | 高 (758行) |
| weixin | weixin/ | 高 (1199行) |
| wechatmp | wechatmp/ | 高 (736行) |
| wechatcom | wechatcom/ | 中 |
| wecombot | wecombot/ | 高 (946行) |
| qq | qq/ | 高 (955行) |

---

## CHANNEL INTERFACE

```go
type Channel interface {
    Startup(ctx context.Context) error
    Stop() error
    Send(reply *types.Reply, ctx *types.Context) error
}
```

---

## BASE CHANNEL

所有渠道嵌入 `*BaseChannel`：

```go
type TerminalChannel struct {
    *channel.BaseChannel
    // ...
}
```

---

## REGISTER PATTERN

```go
// terminal/terminal_channel.go
func init() {
    channel.RegisterChannel("terminal", func() (channel.Channel, error) {
        return NewTerminalChannel(), nil
    })
}
```

---

## CONVENTIONS

- 在 `init()` 中调用 `channel.RegisterChannel()`
- 嵌入 `*BaseChannel` 获取通用方法
- 消息格式放 `message.go`
- 处理器放 `handler.go`

---

## HOTSPOTS

| 文件 | 行数 | 建议 |
|------|------|------|
| `weixin/weixin_channel.go` | 1199 | 拆分 login/message/api |
| `qq/qq_channel.go` | 955 | 提取 WebSocket 管理 |
| `wecombot/wecombot_channel.go` | 946 | 提取加解密模块 |
