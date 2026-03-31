# SimpleClaw

基于 Go 语言实现的 AI Agent 框架，支持多渠道接入和工具调用。

## 功能特性

- **Agent 模式**: 支持 Function Calling API，可调用多种工具
- **多渠道支持**: Terminal、Web、微信、飞书、钉钉、QQ 等
- **多模型支持**: OpenAI、Claude、Gemini、DeepSeek、Qwen、GLM、Minimax 等
- **工具系统**: 文件操作、网络请求、浏览器控制、定时任务等
- **记忆系统**: 短期记忆、长期记忆、会话存储
- **插件系统**: 支持自定义插件扩展
- **语音支持**: 多平台 TTS/STT（阿里、百度、Azure、Google 等）

## 快速开始

### 编译运行

```bash
# 编译
go build -o simpleclaw ./cmd/simpleclaw

# 运行
./simpleclaw

# 指定配置文件
./simpleclaw -config config.json
```

### 配置示例

创建 `config.json` 文件：

```json
{
  "channel_type": "terminal",
  "model": "openai",
  "model_name": "gpt-4o",
  "open_ai_api_key": "your-api-key",
  "open_ai_api_base": "https://api.openai.com/v1",
  "agent": true,
  "agent_workspace": "./workspace",
  "agent_max_steps": 15,
  "tools": {
    "web": {
      "search": {
        "enabled": false
      },
      "fetch": {
        "enabled": true
      }
    }
  }
}
```

## 工具配置

支持通过配置开关工具：

```json
{
  "tools": {
    "web": {
      "search": {
        "enabled": true,
        "provider": "brave",
        "api_key": "your-brave-api-key"
      },
      "fetch": {
        "enabled": true,
        "max_chars": 8000,
        "timeout_seconds": 60
      }
    }
  }
}
```

### 支持的搜索提供商

| Provider | 说明 |
|----------|------|
| brave | Brave Search API |
| gemini | Gemini Grounded Search |
| grok | xAI Grok Search |
| kimi | Moonshot Kimi Search |
| perplexity | Perplexity API |

## 项目结构

```
simpleclaw/
├── cmd/simpleclaw/          # 主程序入口
├── pkg/
│   ├── agent/             # Agent 核心
│   │   ├── executor.go    # 执行引擎
│   │   ├── tools/         # 内置工具
│   │   ├── memory/        # 记忆系统
│   │   ├── prompt/        # 提示词模板
│   │   └── skills/        # 技能系统
│   ├── bridge/            # 渠道-LLM 桥接
│   ├── channel/           # 渠道实现
│   │   ├── terminal/      # 终端渠道
│   │   ├── web/           # Web 渠道
│   │   ├── weixin/        # 微信渠道
│   │   ├── feishu/        # 飞书渠道
│   │   ├── dingtalk/      # 钉钉渠道
│   │   └── qq/            # QQ 渠道
│   ├── llm/               # LLM 模型适配
│   ├── config/            # 配置管理
│   ├── logger/            # 日志系统
│   ├── plugin/            # 插件系统
│   ├── voice/             # 语音处理
│   └── types/             # 公共类型
└── config.json            # 配置文件
```

## 内置工具

| 工具 | 说明 |
|------|------|
| read | 读取文件内容 |
| write | 创建或覆盖文件 |
| edit | 精确编辑文件 |
| ls | 列出目录内容 |
| bash | 执行 Shell 命令 |
| web_search | 网络搜索 |
| web_fetch | 获取网页内容 |
| browser | 浏览器控制 |
| memory | 管理记忆 |
| scheduler | 定时任务管理 |
| vision | 图像识别 |
| time | 获取当前时间 |
| env_config | 管理环境变量和API密钥 |
| send | 发送文件给用户 |

## 开发

### 运行测试

```bash
go test ./...
```

### 代码检查

```bash
go vet ./...
```

## License

MIT License
