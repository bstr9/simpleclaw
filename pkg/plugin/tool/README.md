# Tool 插件

这是一个能让 AI 机器人联网、搜索、数字运算的插件，赋予强大的扩展能力。

## 功能

- 支持多种工具注册和调用
- 可配置的工具列表
- 支持指定特定工具执行
- 工具重置功能

## 使用说明

```
$tool [命令]        - 根据命令选择使用哪些工具处理请求
$tool [工具名] [命令] - 使用指定工具处理请求
$tool reset         - 重置工具
```

## 配置说明

在 `config.json` 中配置：

```json
{
  "tools": ["url-get", "meteo"],
  "tool_configs": {
    "url-get": {
      "enabled": true,
      "config": {
        "timeout": 30
      }
    }
  },
  "kwargs": {
    "debug": false,
    "think_depth": 2
  },
  "trigger_prefix": "$"
}
```

### 配置项说明

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| tools | 要加载的工具列表 | ["url-get", "meteo"] |
| tool_configs | 工具特定配置 | {} |
| kwargs | 全局参数配置 | {} |
| trigger_prefix | 触发前缀 | "$" |
| debug | 是否开启调试模式 | false |
| think_depth | 一个问题最多使用多少次工具 | 2 |
| model_name | 使用的模型名称 | "gpt-3.5-turbo" |
| temperature | LLM 温度参数 | 0 |

## 内置工具

### url-get
获取 URL 内容

### meteo
查询天气信息

### calculator
执行数学计算

### search
网络搜索

## 扩展工具

可以通过实现 `Tool` 接口来创建自定义工具：

```go
type Tool interface {
    Name() string
    Description() string
    Run(query string, config map[string]any) (string, error)
}
```

然后在 `init()` 函数中注册：

```go
func init() {
    tool.RegisterTool(&MyCustomTool{})
}
```

## 作者

goldfishh

## 版本

0.5.0
