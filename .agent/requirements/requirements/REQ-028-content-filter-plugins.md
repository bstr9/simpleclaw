---
id: REQ-028
title: "内容过滤插件：Banwords AC自动机 + Keyword 自动回复"
status: active
level: story
priority: P1
cluster: plugins
created_at: "2026-04-23T16:00:00"
updated_at: "2026-04-23T16:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-006]
  merged_from: []
  depends_on: [REQ-004]
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T16:00:00"
    author: ai
    context: "从代码逆向分析细化需求，来源: pkg/plugin/banwords/banwords.go (551行) + pkg/plugin/keyword/keyword.go (380行)"
    reason: "逆向代码生成需求"
    snapshot: "内容过滤双插件：Banwords（AC自动机+正则，双向过滤消息/回复，ignore/replace动作）+ Keyword（规则自动回复，精确/模糊/正则匹配，优先级排序，随机回复）"
---

# 内容过滤插件：Banwords AC自动机 + Keyword 自动回复

## 描述
两个内容过滤插件协同工作，构成消息安全与自动回复体系：

**Banwords 插件**（551 行）：使用 AC 自动机算法实现高性能敏感词匹配，支持关键词文件加载和正则表达式扩展。双向过滤——既检查用户输入消息（onHandleContext），也检查机器人回复（onDecorateReply）。动作策略支持"ignore"（拦截消息）和"replace"（星号替换），运行时支持动态添加关键词和正则模式。

**Keyword 插件**（380 行）：基于规则的关键词自动回复，支持精确匹配、模糊子字符串匹配、正则表达式三种模式。规则按 Priority 降序排列，多条回复时随机选择，支持 DefaultReply 兜底和动态规则管理。

## 验收标准

### Banwords 插件
- [x] BanwordsPlugin 结构体：嵌入 `BasePlugin`，持有 `config *Config`、`trie *ahoCorasick`、`patterns []*regexp.Regexp`、`keywords []string` (banwords.go:55-63)
- [x] 优先级 100，`SetHidden(true)` (banwords.go:74-75)
- [x] 配置 `Config`：`Action`（默认 "ignore"）、`ReplyFilter`（默认 true）、`ReplyAction`（默认 "ignore"）、`KeywordsFile`（默认 "banwords.txt"）、`RegexPatterns []string` (banwords.go:19-35)
- [x] AC 自动机 `ahoCorasick` 结构体：`root *trieNode`、`built bool` (banwords.go:378-382)
- [x] trieNode 结构体：`children map[rune]*trieNode`、`fail *trieNode`、`output []int`、`depth int` (banwords.go:385-390)
- [x] AC 自动机构建 `Build(keywords)`：`resetRoot` → `buildTrie`（逐字符插入） → `buildFailLinks`（BFS 构建失败指针） (banwords.go:403-408)
- [x] 失败链接构建 `buildFailLinks`：BFS 遍历，`initRootChildrenFailLinks` 初始化一层节点，`processChildrenFailLinks` 处理子节点 (banwords.go:435-476)
- [x] 查找首个匹配 `FindFirst(text)`：沿 trie 和 fail 指针匹配，返回 `*MatchResult{Keyword, Start, End}` (banwords.go:479-511)
- [x] 查找所有匹配 `FindAll(text)`：同 FindFirst 但收集所有 output (banwords.go:514-546)
- [x] 匹配结果 `MatchResult`：`Keyword string`、`Start int`、`End int` (banwords.go:48-52)
- [x] 快速检测 `ContainsAny(text)`：先检查 trie 匹配，再遍历正则 patterns (banwords.go:307-317)
- [x] 替换过滤 `Filter(text)`：trie 匹配和正则匹配的区间替换为 '*'，正则按逆序处理保持索引 (banwords.go:320-343)
- [x] 入站消息过滤 `onHandleContext`：Action="ignore" 时 `FindFirst` 匹配则 `BreakPass`；Action="replace" 时 `ContainsAny`+`Filter` 替换 content (banwords.go:198-222)
- [x] 出站回复过滤 `onDecorateReply`：`ReplyFilter=true` 时注册，ReplyAction 逻辑同 Action (banwords.go:225-249)
- [x] 关键词文件加载 `loadKeywords(path)`：逐行读取，跳过空行和 "#" 注释，调用 `trie.Build(words)` (banwords.go:144-167)
- [x] 默认关键词文件 `createDefaultKeywordsFile`：含注释模板 (banwords.go:170-182)
- [x] 正则编译 `compilePatterns`：遍历 `config.RegexPatterns`，`regexp.Compile` 编译 (banwords.go:185-195)
- [x] 动态添加关键词 `AddKeyword(keyword)`：追加到 keywords 并重建 trie (banwords.go:346-352)
- [x] 动态添加正则 `AddPattern(pattern)`：`regexp.Compile` 后追加到 patterns (banwords.go:355-366)
- [x] 公开检查方法 `Check(text) (bool, string)`：返回是否包含和匹配的关键词 (banwords.go:253-261)

### Keyword 插件
- [x] KeywordPlugin 结构体：嵌入 `BasePlugin`，持有 `config *Config`、`rules []Rule`、`rng *rand.Rand` (keyword.go:60-67)
- [x] 优先级 900，`SetHidden(true)` (keyword.go:77-78)
- [x] 规则结构 `Rule`：`Keyword`、`Reply []string`、`IsRegex bool`、`Priority int`、`FuzzyMatch bool`、`CaseSensitive bool`、`compiledRegex *regexp.Regexp`（缓存） (keyword.go:20-45)
- [x] 配置 `Config`：`Rules []Rule`、`DefaultReply string`（兜底）、`Enabled bool` (keyword.go:48-57)
- [x] 规则匹配 `Match(text)`：按优先级降序遍历规则，`matchRule` 逐条检查，匹配后 `selectReply` 返回 (keyword.go:182-204)
- [x] 精确匹配 `matchRule`：`CaseSensitive=false` 时转小写，`searchText == keyword` (keyword.go:207-237)
- [x] 模糊匹配：`FuzzyMatch=true` 时使用 `strings.Contains(searchText, keyword)` (keyword.go:231-233)
- [x] 正则匹配：`IsRegex=true` 时使用 `rule.compiledRegex.MatchString(text)`，首次匹配时编译并缓存 (keyword.go:218-228)
- [x] 随机回复 `selectReply(replies)`：单条直接返回，多条使用 `rng.Intn` 随机选择 (keyword.go:240-251)
- [x] 默认回复兜底：`DefaultReply != ""` 时在无规则匹配时返回 (keyword.go:195-203)
- [x] 规则预处理 `prepareRules`：编译正则表达式 + 按 Priority 降序排序 `sort.Slice` (keyword.go:307-325)
- [x] 动态规则管理：`AddRule(rule)` 追加并重新 prepare，`RemoveRule(keyword)` 过滤移除，`GetRules()` 返回副本 (keyword.go:328-359)
- [x] 运行时控制：`SetEnabled(bool)`、`IsEnabled() bool`、`SetDefaultReply(reply)` (keyword.go:362-380)
