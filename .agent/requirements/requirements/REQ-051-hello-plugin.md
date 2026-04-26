---
id: REQ-051
title: "Hello 插件"
status: active
level: story
priority: P2
cluster: plugins
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-006]
  merged_from: []
  depends_on: []
  refined_by: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "示例/欢迎插件，展示插件开发模式"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "细化验收标准，从源代码逆向补充实现细节"
    reason: "验收标准从3项扩展至19项，覆盖插件结构、事件分发、消息处理、配置管理等"
    snapshot: "Hello 示例插件，嵌入BasePlugin，priority=-1/hidden=true，支持Hello/Hi/End文本分发、入群欢迎(固定/AI)、退群提示、拍一拍响应，config.json+template回退加载，{nickname}占位符替换"
source_code:
  - pkg/plugin/hello/
---

# Hello 插件

## 描述
示例/欢迎插件，展示插件开发模式。HelloPlugin 嵌入 BasePlugin，priority=-1、hidden=true，作为最低优先级的隐藏插件处理各类社交事件。支持 Hello/Hi/End 文本消息分发、入群欢迎（固定消息或 AI 生成）、退群提示、拍一拍响应。配置通过 config.json 加载，回退到 config.json.template 模板。提供 {nickname} 占位符替换机制和自定义 replaceAll/findIndex 工具函数。

## 代码参考

| 功能 | 文件 | 行号 |
|------|------|------|
| HelloPlugin 结构体 | pkg/plugin/hello/hello.go | 30-35 |
| Config 配置结构体 | pkg/plugin/hello/hello.go | 16-27 |
| New() 构造函数 | pkg/plugin/hello/hello.go | 41-58 |
| OnInit 配置加载 | pkg/plugin/hello/hello.go | 71-90 |
| OnLoad 注册事件处理器 | pkg/plugin/hello/hello.go | 93-101 |
| onHandleContext 事件分发 | pkg/plugin/hello/hello.go | 115-135 |
| handleTextMessage 文本消息 | pkg/plugin/hello/hello.go | 138-167 |
| handleJoinGroup 入群欢迎 | pkg/plugin/hello/hello.go | 170-193 |
| handleExitGroup 退群处理 | pkg/plugin/hello/hello.go | 196-208 |
| handlePatPat 拍一拍 | pkg/plugin/hello/hello.go | 211-221 |
| buildHelloReply 构建回复 | pkg/plugin/hello/hello.go | 224-238 |
| replacePlaceholder 占位符替换 | pkg/plugin/hello/hello.go | 241-248 |
| replaceAll/findIndex 工具函数 | pkg/plugin/hello/hello.go | 251-273 |
| loadConfig 配置文件加载 | pkg/plugin/hello/hello.go | 276-292 |
| HelpText 帮助文本 | pkg/plugin/hello/hello.go | 295-297 |

## 验收标准
- [x] HelloPlugin 嵌入 BasePlugin，实现 Plugin 接口（编译期校验 var _ plugin.Plugin）
- [x] 插件名称 "hello"，版本 "0.1.0"，priority=-1，hidden=true
- [x] Config 结构体定义 5 个字段：GroupWelcomeFixedMsg、GroupWelcomePrompt、GroupExitPrompt、PatPatPrompt、UseCharacterDesc
- [x] OnInit 从 config.json 加载配置，文件不存在时回退到 config.json.template，均失败使用默认配置
- [x] OnLoad 注册 EventOnHandleContext 事件处理器
- [x] onHandleContext 按消息类型分发：ContextText→handleTextMessage、ContextJoinGroup→handleJoinGroup、ContextExitGroup→handleExitGroup、ContextPatPat→handlePatPat
- [x] handleTextMessage 处理 "Hello"：构建含昵称+群名的回复，调用 BreakPass 阻断后续处理器
- [x] handleTextMessage 处理 "Hi"：返回 "Hi" 文本回复，调用 Break 阻断事件链
- [x] handleTextMessage 处理 "End"：将消息类型转换为 ContextImageCreate，内容设为 "The World"，继续事件链
- [x] handleJoinGroup 优先使用 GroupWelcomeFixedMsg 固定欢迎语，无固定语时使用 AI 提示词（GroupWelcomePrompt）
- [x] handleExitGroup 使用 GroupExitPrompt 提示词，替换 {nickname} 占位符后设为文本内容
- [x] handlePatPat 使用 PatPatPrompt 提示词，Break 阻断事件链
- [x] buildHelloReply 根据是否群聊格式化回复：群聊含 nickname+groupName，私聊仅含 nickname
- [x] replacePlaceholder 替换模板中的 {nickname} 占位符，昵称为空时默认 "用户"
- [x] 自定义 replaceAll 和 findIndex 函数实现全局字符串替换和子串查找
- [x] loadConfig 读写锁保护，JSON 反序列化到 Config 结构体
- [x] HelpText 返回中文帮助文本："输入Hello，我会回复你的名字\n输入End，我会回复你世界的图片"
- [x] config.json.template 模板文件存在于插件目录
- [x] 默认配置提供群欢迎/退群/拍一拍提示词模板
