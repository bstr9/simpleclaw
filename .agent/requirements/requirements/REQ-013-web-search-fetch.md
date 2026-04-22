---
id: REQ-013
title: "Web 搜索与抓取工具"
status: active
level: story
priority: P1
cluster: agent-tools
created_at: "2026-04-23T10:10:00"
updated_at: "2026-04-23T18:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-002]
  merged_from: []
  depends_on: []
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T10:10:00"
    author: ai
    context: "从 REQ-002 拆分，来源: pkg/agent/tools/web_search.go, web_fetch.go, pkg/config/types.tools.go"
    reason: "Epic 拆分为 Story"
    snapshot: "Web 搜索（5 提供商）和网页抓取工具，配置化启用，支持缓存和内容限制"
  - version: 2
    date: "2026-04-23T18:00:00"
    author: ai
    context: "逆向代码分析，扩展验收标准至代码级细节"
    reason: "代码逆向补充详细验收标准"
    snapshot: "Web 搜索（DuckDuckGo/SearXNG 双引擎+5 API 提供商）和网页抓取工具，HTML 解析管线、机器人检测、内容提取、配置化门控"
source_code:
  - pkg/agent/tools/web_search.go
  - pkg/agent/tools/web_fetch.go
  - pkg/agent/tools/tools.go
  - pkg/config/types.tools.go
---

# Web 搜索与抓取工具

## 描述
Agent 的网络搜索和网页内容抓取工具。web_search 支持 DuckDuckGo（默认，免费无需 API Key）和 SearXNG（自托管 JSON API）两个内置搜索引擎，配置层额外定义了 Brave/Gemini/Grok/Kimi/Perplexity 五个 API 提供商。web_fetch 获取网页内容并通过 HTML 解析管线提取可读文本。两者均可通过配置启用/禁用。

## 验收标准
- [x] web_search 工具：默认使用 DuckDuckGo HTML 端点搜索（provider="duckduckgo"），可选 SearXNG JSON API（provider="searxng"）
- [x] web_search 工具名称为 "web_search"，执行阶段为 ToolStagePostProcess
- [x] web_search 参数：query（string，必需）和 num_results（integer，可选，默认 5）
- [x] DuckDuckGo 搜索使用 https://html.duckduckgo.com/html 端点，通过正则解析 result__a 链接
- [x] DuckDuckGo URL 解码：通过 uddg 参数提取重定向后的真实 URL（decodeDuckDuckGoHTMLURL）
- [x] DuckDuckGo 摘要提取：在结果链接后搜索 result__snippet 类标签提取摘要（extractSnippet）
- [x] 机器人检测：isBotChallenge 检测 g-recaptcha、are you a human、challenge-form、challenge 标记，触发时返回错误
- [x] HTML 实体解码：decodeHTMLEntities 处理 &amp; &lt; &gt; &quot; &#39; &apos; &nbsp; &ndash; &mdash; &hellip;
- [x] SearXNG 搜索：默认端点 http://localhost:8080，请求 /search?q=...&format=json，解析 SearXNGResponse JSON
- [x] SearXNG 认证：apiKey 非空时设置 Authorization: Bearer 头
- [x] 搜索结果结构 SearchResult：包含 Title、URL、Snippet 三个字段
- [x] 搜索返回 ToolResult 包含 query、count、results；无结果时追加 message: "未找到结果"
- [x] 搜索超时默认 30 秒，User-Agent 为 Chrome 120 浏览器标识
- [x] 搜索工具支持上下文取消：Search(ctx, query, maxResults) 方法通过 goroutine+select 实现
- [x] 搜索配置 WebSearchConfig：Enabled(*bool)、Provider、APIKey、MaxResults(1-10)、TimeoutSeconds、CacheTTLMinutes
- [x] 搜索提供商特定配置：Brave（Mode: web/llm-context）、Gemini（APIKey, Model: gemini-2.5-flash）、Grok（APIKey, Model: grok-4-1-fast, InlineCitations）、Kimi（APIKey, BaseURL, Model: moonshot-v1-128k）、Perplexity（APIKey, BaseURL, Model）
- [x] 搜索启用判断 IsSearchEnabled()：DuckDuckGo 免费，默认启用（nil 配置也返回 true）
- [x] 搜索提供商自动检测 GetSearchProvider()：按 API Key 优先级选择 brave→gemini→grok→kimi→perplexity，无 Key 则 duckduckgo
- [x] web_fetch 工具名称为 "web_fetch"，执行阶段为 ToolStagePostProcess
- [x] web_fetch 参数：url（string，必需）和 output_file（string，可选）
- [x] web_fetch URL 安全校验：仅支持 http/https 协议，其他 scheme 返回错误
- [x] web_fetch 超时默认 60 秒，maxChars 默认 16000 字符，maxFetchSize 限制 32000 字节
- [x] web_fetch 输出模式：有 output_file 时保存原始 body 到文件（相对路径基于 workingDir）；无 output_file 时提取可读文本并截断至 maxChars
- [x] HTML 解析管线：removeTags(script,style,nav,header,footer,aside) → 提取 body → extractMainContent → stripHTMLTags → cleanWhitespace → decodeHTMLEntities
- [x] 主要内容提取 extractMainContent：按优先级匹配 article → main → div.content → div.article → div#content，内容需超过 500 字符
- [x] HTML 标签转换：br→换行, p→换行, li→"- ", h1-6→"## ", 其余标签替换为空格
- [x] 内容截断：超出 maxChars 时截断并追加 "... (truncated)"
- [x] 抓取配置 WebFetchConfig：Enabled(*bool)、MaxChars、MaxCharsCap(默认50000)、TimeoutSeconds、CacheTTLMinutes、MaxRedirects(默认3)、UserAgent、UseReadability(默认true)
- [x] 抓取启用判断 IsFetchEnabled()：默认启用（nil 配置也返回 true）
- [x] 工具注册门控：RegisterBuiltInTools 中根据 appConfig.IsWebSearchEnabled() 和 IsWebFetchEnabled() 条件注册
- [x] 工具注册传递配置：webSearchProvider、webSearchBaseURL、webSearchAPIKey 通过 ToolOption 传入
- [x] DuckDuckGo Lite 兼容解析器：parseDuckDuckGoLite 支持 result-link/result-snippet 类名（单引号和双引号）

## 代码参考
| 验收标准 | 代码位置 |
|---------|---------|
| DuckDuckGo 默认搜索 | web_search.go:60-70 (NewWebSearchTool 默认值), web_search.go:120-125 (provider 路由) |
| DuckDuckGo HTML 端点 | web_search.go:154 (duckDuckGoHTMLEndpoint), web_search.go:157-188 (searchDuckDuckGo) |
| URL 解码 uddg 参数 | web_search.go:240-257 (decodeDuckDuckGoHTMLURL) |
| 摘要提取 | web_search.go:260-281 (extractSnippet) |
| 机器人检测 | web_search.go:191-199 (isBotChallenge) |
| HTML 实体解码 | web_search.go:284-296 (decodeHTMLEntities) |
| SearXNG 搜索 | web_search.go:436-484 (searchSearXNG), web_search.go:487-493 (SearXNGResponse) |
| 上下文取消搜索 | web_search.go:496-525 (Search 方法) |
| 搜索参数定义 | web_search.go:83-98 (Parameters) |
| 搜索结果结构 | web_search.go:148-152 (SearchResult) |
| 搜索配置类型 | types.tools.go:17-50 (WebSearchConfig), types.tools.go:52-101 (提供商子配置) |
| 搜索启用判断 | types.tools.go:132-141 (IsSearchEnabled) |
| 搜索提供商自动检测 | types.tools.go:156-181 (GetSearchProvider) |
| 抓取工具结构 | web_fetch.go:25-29 (WebFetchTool) |
| URL 安全校验 | web_fetch.go:92-94 (http/https scheme 检查) |
| 输出文件模式 | web_fetch.go:118-129 (output_file 分支) |
| HTML 解析管线 | web_fetch.go:141-164 (extractReadableContent) |
| 主要内容提取 | web_fetch.go:174-194 (extractMainContent, 500 字符阈值) |
| 标签转换 | web_fetch.go:197-207 (stripHTMLTags) |
| 内容截断 | web_fetch.go:133-135 (maxChars 截断) |
| 抓取配置类型 | types.tools.go:103-128 (WebFetchConfig) |
| 抓取启用判断 | types.tools.go:144-152 (IsFetchEnabled) |
| 工具注册门控 | tools.go:25-40 (RegisterBuiltInTools 条件注册) |
| DuckDuckGo Lite 兼容 | web_search.go:299-433 (parseDuckDuckGoLite 及辅助函数) |
