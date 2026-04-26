---
id: REQ-054
title: "配置 Schema 覆盖"
status: active
level: story
priority: P1
cluster: bridge
created_at: "2026-04-26T10:00:00"
updated_at: "2026-04-26T12:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-008]
  merged_from: []
  depends_on: [REQ-008]
  refined_by: []
  related_to: [REQ-056, REQ-060]
versions:
  - version: 1
    date: "2026-04-26T10:00:00"
    author: ai
    context: "从审查报告创建缺失需求"
    reason: "审查发现缺失的需求文档"
    snapshot: "系统配置管理，70+ 配置字段覆盖所有子系统"
  - version: 2
    date: "2026-04-26T12:00:00"
    author: ai
    context: "细化验收标准，从源代码逆向补充实现细节"
    reason: "验收标准从6项扩展至25项，覆盖核心/Agent/渠道/语音/工具/搜索/管理后台/加载/脱敏等完整配置体系"
    snapshot: "Config主结构体70+字段，全局单例sync.Once+RWMutex，viper JSON加载+环境变量覆盖+syncToEnv，5搜索提供商(Brave/Gemini/Grok/Kimi/Perplexity)+DuckDuckGo默认，WebFetch(Readability/缓存/重定向)，AdminConfig(认证/静态目录)，APIServerConfig(CORS/限流)，MaskSensitive脱敏"
source_code:
  - pkg/config/config.go
  - pkg/config/types.tools.go
---

# 配置 Schema 覆盖

## 描述
系统配置管理，70+ 配置字段覆盖 Agent、LLM、Channel、Voice、Tools、Admin、API 等所有子系统。Config 主结构体定义在 config.go（约 518 行），工具配置通过 types.tools.go 独立定义（约 181 行）。全局单例通过 sync.Once + sync.RWMutex 实现线程安全。支持 viper 加载 JSON 配置文件（回退 config-template.json）+ 环境变量覆盖（20+ 绑定键）+ 配置同步到环境变量（供子进程使用）。WebSearch 支持 5 个搜索提供商（Brave/Gemini/Grok/Kimi/Perplexity）+ DuckDuckGo 免费默认，自动按 API Key 检测提供商。WebFetch 支持 Readability 内容提取、缓存、重定向控制。AdminConfig 提供管理后台认证与静态目录配置。APIServerConfig 提供 CORS 和限流配置。MaskSensitive 脱敏 API Key（前3+*****+后3）。

## 代码参考

| 功能 | 文件 | 行号 |
|------|------|------|
| Config 主结构体 (70+字段) | pkg/config/config.go | 14-178 |
| AdminConfig 结构体 | pkg/config/config.go | 180-188 |
| APIServerConfig 结构体 | pkg/config/config.go | 465-472 |
| 全局单例 (sync.Once+RWMutex) | pkg/config/config.go | 191-195 |
| Load() 加载入口 | pkg/config/config.go | 199-206 |
| loadConfig viper 加载逻辑 | pkg/config/config.go | 209-254 |
| setDefaults 默认值设置 | pkg/config/config.go | - |
| bindEnvVars 环境变量绑定 | pkg/config/config.go | 257-303 |
| syncToEnv 配置→环境变量同步 | pkg/config/config.go | 306-336 |
| Get() 线程安全获取 | pkg/config/config.go | 339-347 |
| Reload() 热重载 | pkg/config/config.go | 350-360 |
| Set() 更新配置 | pkg/config/config.go | 363-368 |
| MaskSensitive 脱敏 | pkg/config/config.go | 449-462 |
| maskKey 脱敏算法 | pkg/config/config.go | 497-502 |
| IsAgentEnabled/GetModel 等方法 | pkg/config/config.go | 371-446 |
| GetAdminConfig/IsAdminEnabled | pkg/config/config.go | 504-518 |
| GetAPIServerConfig/IsAPIServerEnabled | pkg/config/config.go | 474-494 |
| ToolsConfig 结构体 | pkg/config/types.tools.go | 6-8 |
| WebToolsConfig 结构体 | pkg/config/types.tools.go | 11-14 |
| WebSearchConfig 结构体 | pkg/config/types.tools.go | 17-50 |
| WebFetchConfig 结构体 | pkg/config/types.tools.go | 104-128 |
| 5个搜索子配置 | pkg/config/types.tools.go | 53-101 |
| IsSearchEnabled/IsFetchEnabled | pkg/config/types.tools.go | 130-152 |
| GetSearchProvider 自动检测 | pkg/config/types.tools.go | 155-181 |

## 验收标准
- [x] Config 主结构体定义 70+ 字段，覆盖核心（Model/ModelName/BotType/ChannelType）、OpenAI、Agent、Tools、Service、APIServer 等子系统
- [x] Agent 模式配置：Agent(bool)、AgentWorkspace、AgentMaxContextTokens、AgentMaxContextTurns、AgentMaxSteps
- [x] ChatGPT API 参数配置：Temperature、TopP、FrequencyPenalty、PresencePenalty、RequestTimeout、Timeout
- [x] 单聊配置：SingleChatPrefix、SingleChatReplyPrefix、SingleChatReplySuffix
- [x] 群聊配置：GroupChatPrefix、GroupChatReplyPrefix/Suffix、GroupChatKeyword、GroupNameWhiteList、GroupNameKeywordWhiteList、NoNeedAt、GroupAtOff、GroupSharedSession
- [x] 10 个渠道配置：OpenAI、Claude、Gemini、DeepSeek、ZhipuAI、Moonshot、Minimax、Feishu、Dingtalk、Weixin、Wechatmp、QQ、Wecom
- [x] 语音配置：SpeechRecognition、VoiceReplyVoice、VoiceToText、TextToVoice、TextToVoiceModel、TTSVoiceID
- [x] 图片生成配置：TextToImage、ImageCreateSize、ImageProxy
- [x] AdminConfig 结构体：Enabled、Host、Port、Username、PasswordHash、SessionSecret、StaticDir
- [x] APIServerConfig 结构体：Enabled、Host、Port、APIKey、EnableCORS、RateLimit
- [x] Memory 配置：MemoryType、MemoryMaxTokens、MemorySummaryModel
- [x] 全局单例模式：sync.Once 初始化 + sync.RWMutex 读写锁保护
- [x] Load() 使用 viper 加载 JSON 配置文件，文件不存在时回退 config-template.json
- [x] bindEnvVars 绑定 20+ 环境变量键（snake_case → 大写下划线格式）
- [x] syncToEnv 将 20+ 配置值同步到环境变量（供子进程使用，不覆盖已有值）
- [x] Get() 线程安全获取配置，cfg 为 nil 时返回默认配置
- [x] Reload() 热重载配置（写锁保护），Set() 更新配置（用于测试）
- [x] MaskSensitive() 返回脱敏后的配置 map，maskKey 算法：长度≤6返回 ***，否则 前3+*****+后3
- [x] 便捷方法：IsAgentEnabled、GetModel、GetChannelType、GetWorkspace、IsDebugEnabled
- [x] 工具配置方法：GetTools、GetWebSearch、GetWebFetch、IsWebSearchEnabled、IsWebFetchEnabled
- [x] IsWebSearchEnabled 默认 true（DuckDuckGo 免费），IsWebFetchEnabled 默认 true
- [x] ToolsConfig 独立定义：Web（WebToolsConfig）含 Search + Fetch
- [x] WebSearchConfig 支持 5 个搜索提供商子配置：Brave（Mode）、Gemini（APIKey+Model）、Grok（APIKey+Model+InlineCitations）、Kimi（APIKey+BaseURL+Model）、Perplexity（APIKey+BaseURL+Model）
- [x] GetSearchProvider 自动检测逻辑：Provider 显式配置 → APIKey → Gemini → Grok → Kimi → Perplexity → DuckDuckGo 兜底
- [x] WebFetchConfig：Enabled、MaxChars、MaxCharsCap(50000)、TimeoutSeconds、CacheTTLMinutes、MaxRedirects(3)、UserAgent、UseReadability
- [x] APIServerConfig 默认值：Host=0.0.0.0、Port=8080、EnableCORS=true、RateLimit=60
- [x] AdminConfig 默认值：Enabled=true、Host=0.0.0.0、Port=31415
