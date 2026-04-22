---
id: REQ-011
title: "技能系统"
status: active
level: story
priority: P1
cluster: agent-core
created_at: "2026-04-23T10:10:00"
updated_at: "2026-04-23T18:00:00"
relations:
  supersedes: []
  conflicts_with: []
  refines: [REQ-001]
  merged_from: []
  depends_on: []
  related_to: [REQ-002]
versions:
  - version: 1
    date: "2026-04-23T10:10:00"
    author: ai
    context: "从 REQ-001 拆分，来源: pkg/agent/skills/registry.go"
    reason: "Epic 拆分为 Story"
    snapshot: "从 Markdown 文件加载技能，存放在 workspace，支持技能描述解析和注册"
  - version: 2
    date: "2026-04-23T18:00:00"
    author: ai
    context: "代码逆向分析，扩展验收标准"
    reason: "基于代码分析扩展验收标准至22条"
    snapshot: "技能系统：Skill接口、Registry注册表、Loader递归加载、Parser frontmatter解析、Filter过滤、Snapshot快照、OS/依赖检查"
source_code:
  - pkg/agent/skills/skill.go
  - pkg/agent/skills/registry.go
---

# 技能系统

## 描述
Agent 技能注册与加载系统，从 Markdown 文件解析技能描述和触发词，注册到技能表中供提示词构建器注入。技能存放在 Agent workspace 目录下。支持递归目录加载、frontmatter元数据解析、OS兼容性检查、二进制/环境变量依赖检测、技能快照生成。

## 验收标准
- [x] 技能文件格式：Markdown，frontmatter使用简单key:value行解析（非YAML解析器），以"---"分隔
- [x] Skill接口定义：Name()、Description()、Execute(ctx, input) (any, error)，SkillInfo提供默认Execute返回input不变
- [x] SkillInfo结构体：name, description, FilePath, BaseDir, Source(builtin/custom), Content(完整markdown), DisableModelInvocation(bool), Frontmatter(map[string]any)
- [x] 技能来源枚举：SourceBuiltin="builtin"，SourceCustom="custom"
- [x] 技能注册表（Registry）：线程安全（sync.RWMutex），支持Register/RegisterEntry/Get/GetSkill/Remove/List/ListEnabled/Execute/Enable/Disable/IsEnabled/SetConfig操作
- [x] LoadFromDir(dirs...)：按目录顺序加载，后加载目录的同名技能覆盖先加载的，加载诊断信息以Debug级别日志记录
- [x] Refresh(dirs...)：清空所有技能后从目录重新加载
- [x] Loader.LoadFromDir(dir, source)：递归查找子目录中的SKILL.md或直接.md文件，每个技能设置Source和BaseDir
- [x] SkillParser.ParseFile(filePath)：读取文件→parseFrontmatter→extractName(frontmatter "name"键或去除.md的文件名)→extractDescription(frontmatter "description"键或正文前100字符)
- [x] parseFrontmatter：简单行级key:value解析器（非YAML），按"---\n"分割，跳过#注释行，值去除引号
- [x] Metadata结构体：Always(bool), SkillKey, PrimaryEnv, Emoji, Homepage, OS([]string), Requires(*Requirements), Install([]*InstallSpec)
- [x] Requirements结构体：Bins(全部必须存在), AnyBins(至少一个存在), Env(全部必须设置), AnyEnv(至少一个设置)
- [x] InstallSpec结构体：Kind(brew/pip/npm/download), ID, Label, Bins, OS, Formula, Package, Module, URL, Archive, Extract, StripComponents, TargetDir
- [x] Entry结构体：Skill(接口), *SkillInfo, *Metadata, UserInvocable(bool，从frontmatter "user-invocable"键解析)
- [x] Filter结构体：Names([]string)名称过滤, IncludeDisabled(bool)包含已禁用, shouldInclude检查：禁用过滤→名称过滤→OS兼容性→依赖检查
- [x] OS兼容性检查：runtime.GOOS映射（windows→"win32"），OS列表为空时兼容所有系统，否则当前OS必须匹配列表中某项
- [x] 依赖检查：Bins通过exec.LookPath验证, AnyBins至少一个存在, Env通过os.LookupEnv非空验证, AnyEnv至少一个设置
- [x] hasBinary/hasEnvVar：使用可替换变量execLookPath（便于测试）和os.LookupEnv
- [x] FormatForPrompt(entries)：输出格式"可用技能：\n\n" + 每个条目"## {name}\n{description}\n\n"
- [x] BuildSnapshot(filter, version)：创建Snapshot包含Prompt、Skills摘要(name+primary_env)、ResolvedSkills、Version
- [x] Snapshot结构体：Prompt(string), Skills([]map[string]string), ResolvedSkills([]*SkillInfo), Version(int)
- [x] LoadResult结构体：Skills([]*SkillInfo), Diagnostics([]string)
- [x] 技能不能直接调用：技能作为提示词上下文注入，不是工具
- [x] 逐个加载：禁止批量读取技能，每次只加载一个

## 代码参考

| 验收标准 | 代码位置 |
|---------|---------|
| frontmatter格式解析 | registry.go:parseFrontmatter() |
| Skill接口 | skill.go:Skill接口定义 |
| SkillInfo结构体 | skill.go:SkillInfo结构体 |
| Source枚举 | skill.go:SourceBuiltin/SourceCustom常量 |
| Registry线程安全 | registry.go:Registry结构体(mu sync.RWMutex) |
| LoadFromDir覆盖 | registry.go:LoadFromDir()目录顺序逻辑 |
| Refresh重载 | registry.go:Refresh() |
| Loader递归加载 | registry.go:Loader.LoadFromDir() |
| ParseFile解析 | registry.go:SkillParser.ParseFile() |
| extractName/extractDescription | registry.go:extractName()/extractDescription() |
| Metadata结构体 | skill.go:Metadata结构体 |
| Requirements结构体 | skill.go:Requirements结构体 |
| InstallSpec结构体 | skill.go:InstallSpec结构体 |
| Entry/UserInvocable | registry.go:Entry结构体及createEntry() |
| Filter过滤 | registry.go:Filter结构体及shouldInclude() |
| OS兼容性检查 | registry.go:checkOSCompatibility() |
| 依赖检查 | registry.go:checkRequirements() |
| hasBinary/hasEnvVar | registry.go:hasBinary()/hasEnvVar() |
| FormatForPrompt | registry.go:FormatForPrompt() |
| BuildSnapshot | registry.go:BuildSnapshot() |
| Snapshot结构体 | skill.go:Snapshot结构体 |
| LoadResult结构体 | skill.go:LoadResult结构体 |
