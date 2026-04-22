---
id: REQ-010
title: "提示词构建器"
status: active
level: story
priority: P0
cluster: agent-core
created_at: "2026-04-23T10:10:00"
updated_at: "2026-04-23T18:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-001]
  merged_from: []
  depends_on: [REQ-002, REQ-003]
  related_to: []
versions:
  - version: 1
    date: "2026-04-23T10:10:00"
    author: ai
    context: "从 REQ-001 拆分，来源: pkg/agent/prompt/builder.go"
    reason: "Epic 拆分为 Story"
    snapshot: "提示词构建：系统提示词、工具描述注入、记忆检索注入、技能描述注入"
  - version: 2
    date: "2026-04-23T18:00:00"
    author: ai
    context: "代码逆向分析，扩展验收标准"
    reason: "基于代码分析扩展验收标准至20条"
    snapshot: "提示词构建器：7段式构建流程、17工具有序注入、模板引擎、敏感信息脱敏、Builder流式API"
source_code:
  - pkg/agent/prompt/builder.go
  - pkg/agent/prompt/prompt.go
  - pkg/agent/prompt/template.go
---

# 提示词构建器

## 描述
构建 Agent 发送给 LLM 的完整提示词，包括系统提示词、可用工具描述、相关记忆片段、已加载技能描述。确保提示词不包含敏感信息。采用7段式严格顺序构建，Builder流式API组装，Go text/template模板引擎渲染。

## 验收标准
- [x] 系统提示词7段式严格顺序构建：1)工具系统 2)技能系统 3)记忆系统 4)工作空间 5)用户身份 6)项目上下文 7)运行时信息
- [x] Builder流式API：NewBuilder(workspaceDir, language)创建，支持WithTools()、WithUserIdentity()、WithContextFiles()、WithSkillsPrompt()、WithMemoryTools()、WithRuntime()链式调用，最终Build()生成系统提示词
- [x] Builder默认语言为"zh"（中文），通过Language字段控制输出语言
- [x] BuildOptions结构体字段：Language, WorkspaceDir, BasePersona(deprecated保留兼容), UserIdentity(*UserInfo), Tools([]*ToolInfo), ContextFiles([]*ContextFile), SkillsPrompt(string), HasMemoryTools(bool), Runtime(*RuntimeInfo)
- [x] 工具描述注入：将ToolInfo的Name/Description/Summary注入，Summary字段omitempty可省略
- [x] 17个核心工具中文摘要映射（coreToolSummaries）：read/写/编辑/ls/grep/find/bash/terminal/web_search/web_fetch/browser/memory_search/memory_get/env_config/cron/send/lark_cli各具固定中文描述
- [x] 工具摘要优先级：coreToolSummaries硬编码 > tool.Summary字段 > Description截断前50字符
- [x] 工具显示顺序（toolOrder）：read→write→edit→ls→grep→find→bash→terminal→web_search→web_fetch→browser→memory_search→memory_get→env_config→cron→send→lark_cli，不在toolOrder中的工具追加到末尾
- [x] 技能描述注入：标记为"强制性"部分，指令要求扫描技能描述后用read工具读取匹配的SKILL.md
- [x] 技能禁止批量读取：明确指示"永远不要一次读多个技能"，每次只读取一个SKILL.md
- [x] 技能不能作为工具调用：明确声明"技能不是工具"，仅作为提示词上下文注入
- [x] 记忆检索注入：包含当日日期文件路径、memory_search→memory_get→降级策略、MEMORY.md长期记忆、memory/YYYY-MM-DD.md每日记忆
- [x] 记忆存储规则：使用edit工具写入，追加时oldText留空
- [x] 敏感信息过滤（工具段）：密钥、令牌等敏感信息必须脱敏，禁止在工具输出中暴露
- [x] 敏感信息过滤（记忆段）：禁止写入敏感信息：API密钥、令牌等，明确写入提示词指令
- [x] 工作空间段：相对路径基于workspace dir，绝对路径用于外部文件，AGENT.md/USER.md/RULE.md自动加载无需重新读取
- [x] 交流规范：提示词中指示不要直接提及文件名，使用自然语言描述
- [x] ContextFiles段：AGENT.md作为"灵魂文件"处理，用户改变个性时应更新此文件
- [x] 运行时信息段：当前时间含星期和时区、模型/工作空间/渠道信息，渠道仅当非"web"时显示
- [x] 模板引擎系统：Template接口(Execute)、DefaultTemplate(Go text/template)、SimpleTemplate(变量替换)、TemplateManager(注册/获取/执行)
- [x] 4个内置模板：system_prompt、user_prompt、tool_prompt、skill_prompt
- [x] 模板错误类型：ErrTemplateNotFound、ErrInvalidTemplate、ErrMissingKey，MissingKeyAction支持Error/Default/Zero三种模式

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| 7段式顺序构建 | builder.go:BuildSystemPrompt() |
| Builder流式API | prompt.go:Builder结构体及WithXxx()方法 |
| 默认语言zh | prompt.go:NewBuilder()默认参数 |
| BuildOptions结构体 | prompt.go:BuildOptions定义 |
| 工具描述注入 | builder.go:buildToolSection() |
| coreToolSummaries映射 | builder.go:coreToolSummaries变量 |
| 工具摘要优先级 | builder.go:getToolSummary() |
| toolOrder显示顺序 | builder.go:toolOrder变量 |
| 技能描述注入 | builder.go:buildSkillSection() |
| 技能禁止批量读取 | builder.go:buildSkillSection()提示词文本 |
| 技能不能作为工具 | builder.go:buildSkillSection()提示词文本 |
| 记忆检索注入 | builder.go:buildMemorySection() |
| 记忆存储规则 | builder.go:buildMemorySection()提示词文本 |
| 敏感信息过滤(工具) | builder.go:buildToolSection()提示词文本 |
| 敏感信息过滤(记忆) | builder.go:buildMemorySection()提示词文本 |
| 工作空间段 | builder.go:buildWorkspaceSection() |
| 交流规范 | builder.go:buildWorkspaceSection()提示词文本 |
| ContextFiles/灵魂文件 | builder.go:buildContextSection() |
| 运行时信息 | builder.go:buildRuntimeSection() |
| 模板引擎系统 | template.go:Template/DefaultTemplate/SimpleTemplate/TemplateManager |
| 4个内置模板 | template.go:内置模板注册 |
| 模板错误类型 | template.go:ErrTemplateNotFound/ErrInvalidTemplate/ErrMissingKey/MissingKeyAction |
