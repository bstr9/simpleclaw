// Package prompt 提示词构建器实现
package prompt

import (
	"fmt"
	"strings"
	"time"
)

// 核心工具摘要映射
var coreToolSummaries = map[string]string{
	"read":          "读取文件内容",
	"write":         "创建或覆盖文件",
	"edit":          "精确编辑文件",
	"ls":            "列出目录内容",
	"grep":          "搜索文件内容",
	"find":          "按模式查找文件",
	"bash":          "执行shell命令",
	"terminal":      "管理后台进程",
	"web_search":    "网络搜索",
	"web_fetch":     "获取URL内容",
	"browser":       "控制浏览器",
	"memory_search": "搜索记忆",
	"memory_get":    "读取记忆内容",
	"env_config":    "管理API密钥和技能配置",
	"cron":          "创建和管理定时任务、提醒",
	"send":          "发送本地文件给用户（仅限本地文件，URL直接放在回复文本中）",
	"lark_cli":      "飞书操作（创建文档/表格、发消息等）。当用户要求创建飞书文档或操作飞书时必须使用此工具",
}

// 工具显示顺序
var toolOrder = []string{
	"read", "write", "edit", "ls", "grep", "find",
	"bash", "terminal",
	"web_search", "web_fetch", "browser",
	"memory_search", "memory_get",
	"env_config", "cron", "send", "lark_cli",
}

// BuildSystemPrompt 构建完整的 Agent 系统提示词
//
// 构建顺序（按重要性和逻辑关系排列）:
// 1. 工具系统 - 核心能力，最先介绍
// 2. 技能系统 - 紧跟工具，因为技能需要用 read 工具读取
// 3. 记忆系统 - 独立的记忆能力
// 4. 工作空间 - 工作环境说明
// 5. 用户身份 - 用户信息（可选）
// 6. 项目上下文 - AGENT.md, USER.md, RULE.md, BOOTSTRAP.md
// 7. 运行时信息 - 元信息（时间、模型等）
func BuildSystemPrompt(opts *BuildOptions) string {
	if opts == nil {
		return ""
	}

	var sections []string

	// 1. 工具系统（最重要，放在最前面）
	if len(opts.Tools) > 0 {
		sections = append(sections, buildToolingSection(opts.Tools, opts.Language)...)
	}

	// 2. 技能系统（紧跟工具，因为需要用 read 工具）
	if opts.SkillsPrompt != "" {
		sections = append(sections, buildSkillsSection(opts.SkillsPrompt, opts.Language)...)
	}

	// 3. 记忆系统（独立的记忆能力）
	if opts.HasMemoryTools {
		sections = append(sections, buildMemorySection(opts.Language)...)
	}

	// 4. 工作空间（工作环境说明）
	if opts.WorkspaceDir != "" {
		sections = append(sections, buildWorkspaceSection(opts.WorkspaceDir, opts.Language)...)
	}

	// 5. 用户身份（如果有）
	if opts.UserIdentity != nil {
		sections = append(sections, buildUserIdentitySection(opts.UserIdentity, opts.Language)...)
	}

	// 6. 项目上下文文件（AGENT.md, USER.md, RULE.md - 定义人格）
	if len(opts.ContextFiles) > 0 {
		sections = append(sections, buildContextFilesSection(opts.ContextFiles, opts.Language)...)
	}

	// 7. 运行时信息（元信息，放在最后）
	if opts.Runtime != nil {
		sections = append(sections, buildRuntimeSection(opts.Runtime, opts.Language)...)
	}

	return strings.Join(sections, "\n")
}

// buildToolingSection 构建工具系统区块
func buildToolingSection(tools []*ToolInfo, _ string) []string {
	available := buildToolSummaryMap(tools)
	toolLines := buildOrderedToolLines(available)

	lines := []string{
		"## 工具系统",
		"",
		"可用工具（名称大小写敏感，严格按列表调用）:",
		strings.Join(toolLines, "\n"),
		"",
		"工具调用风格：",
		"",
		"- 在多步骤任务、敏感操作或用户要求时简要解释决策过程",
		"- 持续推进直到任务完成，完成后向用户报告结果。",
		"- 回复中涉及密钥、令牌等敏感信息必须脱敏。",
		"- URL链接直接放在回复文本中即可，系统会自动处理和渲染。无需下载后使用send工具发送",
		"- 创建飞书文档/表格、发送飞书消息等操作必须使用 lark_cli 工具，不要直接输出文档内容",
		"",
	}

	return lines
}

// buildToolSummaryMap 构建工具名称到摘要的映射
func buildToolSummaryMap(tools []*ToolInfo) map[string]string {
	available := make(map[string]string)
	for _, tool := range tools {
		available[tool.Name] = getToolSummary(tool)
	}
	return available
}

// getToolSummary 获取工具的摘要描述
func getToolSummary(tool *ToolInfo) string {
	if s, ok := coreToolSummaries[tool.Name]; ok {
		return s
	}
	if tool.Summary != "" {
		return tool.Summary
	}
	summary := tool.Description
	if len(summary) > 50 {
		summary = summary[:47] + "..."
	}
	return summary
}

// buildOrderedToolLines 按顺序生成工具行列表
func buildOrderedToolLines(available map[string]string) []string {
	var toolLines []string
	// 按预定义顺序添加工具
	for _, name := range toolOrder {
		if summary, ok := available[name]; ok {
			delete(available, name)
			toolLines = append(toolLines, formatToolLine(name, summary))
		}
	}
	// 添加剩余的工具
	for name, summary := range available {
		toolLines = append(toolLines, formatToolLine(name, summary))
	}
	return toolLines
}

// formatToolLine 格式化单个工具行
func formatToolLine(name, summary string) string {
	if summary != "" {
		return fmt.Sprintf("- %s: %s", name, summary)
	}
	return fmt.Sprintf("- %s", name)
}

// buildSkillsSection 构建技能系统区块
func buildSkillsSection(skillsPrompt string, _ string) []string {
	lines := []string{
		"## 技能系统（mandatory）",
		"",
		"在回复之前：扫描下方 <available_skills> 中每个技能的 <description>。",
		"",
		"- 如果有技能的描述与用户需求匹配：使用 `read` 工具读取其 <location> 路径的 SKILL.md 文件，然后严格遵循文件中的指令。当有匹配的技能时，应优先使用技能",
		"- 如果多个技能都适用则选择最匹配的一个，然后读取并遵循。",
		"- 如果没有技能明确适用：不要读取任何 SKILL.md，直接使用通用工具。",
		"",
		"**重要**: 技能不是工具，不能直接调用。使用技能的唯一方式是用 `read` 读取 SKILL.md 文件，然后按文件内容操作。永远不要一次性读取多个技能，只在选择后再读取。",
		"",
		"以下是可用技能：",
	}

	if skillsPrompt != "" {
		lines = append(lines, strings.TrimSpace(skillsPrompt))
		lines = append(lines, "")
	}

	return lines
}

// buildMemorySection 构建记忆系统区块
func buildMemorySection(_ string) []string {
	todayFile := time.Now().Format("2006-01-02") + ".md"

	lines := []string{
		"## 记忆系统",
		"",
		"### 检索记忆",
		"",
		"在回答关于以前的工作、决定、日期、人物、偏好或待办事项的任何问题之前：",
		"",
		"1. 不确定记忆文件位置 → 先用 `memory_search` 通过关键词和语义检索相关内容",
		"2. 已知文件位置 → 直接用 `memory_get` 读取相应的行 (例如：MEMORY.md, memory/YYYY-MM-DD.md)",
		"3. search 无结果 → 尝试用 `memory_get` 读取MEMORY.md及最近两天记忆文件",
		"",
		"**记忆文件结构**:",
		"- `MEMORY.md`: 长期记忆（核心信息、偏好、决策等）",
		fmt.Sprintf("- `memory/YYYY-MM-DD.md`: 每日记忆，今天是 `memory/%s`", todayFile),
		"",
		"### 写入记忆",
		"",
		"**主动存储**：遇到以下情况时，应主动将信息写入记忆文件（无需告知用户）：",
		"",
		"- 用户明确要求你记住某些信息",
		"- 用户分享了重要的个人偏好、习惯、决策",
		"- 对话中产生了重要的结论、方案、约定",
		"- 完成了复杂任务，值得记录关键步骤和结果",
		"- 发现了用户经常遇到的问题或解决方案",
		"",
		"**存储规则**:",
		"- 长期有效的核心信息 → `MEMORY.md`（文件保持精简，< 2000 tokens）",
		fmt.Sprintf("- 当天的事件、进展、笔记 → `memory/%s`", todayFile),
		"- 追加内容 → `edit` 工具，oldText 留空",
		"- 修改内容 → `edit` 工具，oldText 填写要替换的文本",
		"- **禁止写入敏感信息**：API密钥、令牌等敏感信息严禁写入记忆文件",
		"",
		"**使用原则**: 自然使用记忆，就像你本来就知道；不用刻意提起，除非用户问起。",
		"",
	}

	return lines
}

// buildWorkspaceSection 构建工作空间区块
func buildWorkspaceSection(workspaceDir string, _ string) []string {
	lines := []string{
		"## 工作空间",
		"",
		fmt.Sprintf("你的工作目录是: `%s`", workspaceDir),
		"",
		"**路径使用规则** (非常重要):",
		"",
		fmt.Sprintf("1. **相对路径的基准目录**: 所有相对路径都是相对于 `%s` 而言的", workspaceDir),
		"   - ✅ 正确: 访问工作空间内的文件用相对路径，如 `AGENT.md`",
		fmt.Sprintf("   - ❌ 错误: 用相对路径访问其他目录的文件 (如果它不在 `%s` 内)", workspaceDir),
		"",
		"2. **访问其他目录**: 如果要访问工作空间之外的目录（如项目代码、系统文件），**必须使用绝对路径**",
		"   - ✅ 正确: 例如 `~/chatgpt-on-wechat`、`/usr/local/`",
		"   - ❌ 错误: 假设相对路径会指向其他目录",
		"",
		"3. **路径解析示例**:",
		fmt.Sprintf("   - 相对路径 `memory/` → 实际路径 `%s/memory/`", workspaceDir),
		"   - 绝对路径 `~/chatgpt-on-wechat/docs/` → 实际路径 `~/chatgpt-on-wechat/docs/`",
		"",
		"4. **不确定时**: 先用 `bash pwd` 确认当前目录，或用 `ls .` 查看当前位置",
		"",
		"**重要说明 - 文件已自动加载**:",
		"",
		"以下文件在会话启动时**已经自动加载**到系统提示词的「项目上下文」section 中，你**无需再用 read 工具读取它们**：",
		"",
		"- ✅ `AGENT.md`: 已加载 - 你的人格和灵魂设定，请严格遵循。当你的名字、性格或交流风格发生变化时，主动用 `edit` 更新此文件",
		"- ✅ `USER.md`: 已加载 - 用户的身份信息。当用户修改称呼、姓名等身份信息时，用 `edit` 更新此文件",
		"- ✅ `RULE.md`: 已加载 - 工作空间使用指南和规则，请严格遵循",
		"",
		"**交流规范**:",
		"",
		"- 在对话中，无需直接输出工作空间中的技术细节，例如 AGENT.md、USER.md、MEMORY.md 等文件名称",
		"- 例如用自然表达例如「我已记住」而不是「已更新 MEMORY.md」",
		"",
	}

	return lines
}

// buildUserIdentitySection 构建用户身份区块
func buildUserIdentitySection(user *UserInfo, _ string) []string {
	if user == nil {
		return nil
	}

	lines := []string{
		"## 用户身份",
		"",
	}

	if user.Name != "" {
		lines = append(lines, fmt.Sprintf("**用户姓名**: %s", user.Name))
	}
	if user.Nickname != "" {
		lines = append(lines, fmt.Sprintf("**称呼**: %s", user.Nickname))
	}
	if user.Timezone != "" {
		lines = append(lines, fmt.Sprintf("**时区**: %s", user.Timezone))
	}
	if user.Notes != "" {
		lines = append(lines, fmt.Sprintf("**备注**: %s", user.Notes))
	}

	lines = append(lines, "")

	return lines
}

// buildContextFilesSection 构建项目上下文文件区块
func buildContextFilesSection(files []*ContextFile, _ string) []string {
	if len(files) == 0 {
		return nil
	}

	// 检查是否有 AGENT.md
	hasAgent := false
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f.Path), "agent.md") {
			hasAgent = true
			break
		}
	}

	lines := []string{
		"# 项目上下文",
		"",
		"以下项目上下文文件已被加载：",
		"",
	}

	if hasAgent {
		lines = append(lines, "**`AGENT.md` 是你的灵魂文件**：严格遵循其中定义的人格、规则、语气和设定，避免僵硬、模板化的回复。")
		lines = append(lines, "当用户通过对话透露了对你性格、风格、职责、能力边界的新期望，你应该主动用 `edit` 更新 AGENT.md 以反映这些演变。")
		lines = append(lines, "")
	}

	// 添加每个文件的内容
	for _, file := range files {
		lines = append(lines, fmt.Sprintf("## %s", file.Path))
		lines = append(lines, "")
		lines = append(lines, file.Content)
		lines = append(lines, "")
	}

	return lines
}

// buildRuntimeSection 构建运行时信息区块
func buildRuntimeSection(runtime *RuntimeInfo, _ string) []string {
	if runtime == nil {
		return nil
	}

	lines := []string{
		"## 运行时信息",
		"",
	}

	// 添加当前时间
	if runtime.CurrentTime != "" {
		timeLine := fmt.Sprintf("当前时间: %s", runtime.CurrentTime)
		if runtime.Weekday != "" {
			timeLine += " " + runtime.Weekday
		}
		if runtime.Timezone != "" {
			timeLine += " (" + runtime.Timezone + ")"
		}
		lines = append(lines, timeLine)
		lines = append(lines, "")
	}

	// 添加其他运行时信息
	var runtimeParts []string
	if runtime.Model != "" {
		runtimeParts = append(runtimeParts, fmt.Sprintf("模型=%s", runtime.Model))
	}
	if runtime.Workspace != "" {
		runtimeParts = append(runtimeParts, fmt.Sprintf("工作空间=%s", runtime.Workspace))
	}
	// 只有非默认 "web" 渠道才添加
	if runtime.Channel != "" && runtime.Channel != "web" {
		runtimeParts = append(runtimeParts, fmt.Sprintf("渠道=%s", runtime.Channel))
	}

	if len(runtimeParts) > 0 {
		lines = append(lines, "运行时: "+strings.Join(runtimeParts, " | "))
		lines = append(lines, "")
	}

	return lines
}
