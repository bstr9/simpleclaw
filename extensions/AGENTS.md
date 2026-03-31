# Extensions 扩展系统

## 目录结构

```
extensions/
├── AGENTS.md          # 本文档
├── feishu/            # 飞书扩展（参考模板）
│   ├── extension.go   # Extension 接口实现
│   └── tools/         # 扩展工具
│       └── doc.go
├── weixin/            # 微信扩展
├── dingtalk/          # 钉钉扩展
├── qq/                # QQ扩展
├── wechatmp/          # 微信公众号扩展
├── wechatcom/         # 企业微信扩展
├── wecombot/          # 企业微信机器人扩展
├── web/               # Web扩展
└── terminal/          # 终端扩展
```

## Extension 接口

所有扩展必须实现 `extension.Extension` 接口：

```go
type Extension interface {
    ID() string          // 唯一标识
    Name() string        // 显示名称
    Description() string // 描述
    Version() string     // 版本号
    Register(api ExtensionAPI) error    // 注册组件
    Startup(ctx context.Context) error  // 启动
    Shutdown(ctx context.Context) error // 关闭
}
```

## 注册流程

1. **init()** 中调用 `extension.RegisterExtension()` 注册到全局
2. **main.go** 调用 `mgr.LoadGlobalExtensions()` 加载
3. **mgr.RegisterAll()** 调用每个扩展的 `Register()`
4. **mgr.StartupAll()** 启动所有扩展

## 扩展模板

```go
package mychannel

import (
    "context"
    "path/filepath"
    "sync"

    "github.com/bstr9/simpleclaw/pkg/channel"
    "github.com/bstr9/simpleclaw/pkg/channel/mychannel"
    "github.com/bstr9/simpleclaw/pkg/extension"
    "github.com/bstr9/simpleclaw/pkg/logger"
    "go.uber.org/zap"
)

var defaultExtension *MyChannelExtension

func init() {
    defaultExtension = New()
    extension.RegisterExtension(defaultExtension)
}

type MyChannelExtension struct {
    mu      sync.RWMutex
    channel *mychannel.MyChannel
    api      extension.ExtensionAPI
    started  bool
}

func New() *MyChannelExtension {
    return &MyChannelExtension{}
}

func (e *MyChannelExtension) ID() string {
    return "mychannel"
}

func (e *MyChannelExtension) Name() string {
    return "MyChannel"
}

func (e *MyChannelExtension) Description() string {
    return "MyChannel 渠道扩展"
}

func (e *MyChannelExtension) Version() string {
    return "1.0.0"
}

func (e *MyChannelExtension) Register(api extension.ExtensionAPI) error {
    e.mu.Lock()
    e.api = api
    e.mu.Unlock()

    // 注册渠道
    api.RegisterChannel("mychannel", func() (channel.Channel, error) {
        return e.createChannel()
    })

    logger.Info("[MyChannelExtension] Extension registered")
    return nil
}

func (e *MyChannelExtension) Startup(ctx context.Context) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    if e.started {
        return nil
    }
    e.started = true
    return nil
}

func (e *MyChannelExtension) Shutdown(ctx context.Context) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    if !e.started {
        return nil
    }
    if e.channel != nil {
        e.channel.Stop()
    }
    e.started = false
    return nil
}

func (e *MyChannelExtension) createChannel() (*mychannel.MyChannel, error) {
    e.mu.Lock()
    defer e.mu.Unlock()
    if e.channel != nil {
        return e.channel, nil
    }
    // 创建渠道实例
    e.channel = mychannel.NewMyChannel(...)
    return e.channel, nil
}
```

## 注意事项

- 渠道实现保留在 `pkg/channel/xxx/` 目录，扩展只负责注册
- 扩展可选注册工具和技能目录
- 使用 `sync.RWMutex` 保护并发访问
