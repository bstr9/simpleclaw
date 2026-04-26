// Package weixin 提供微信个人号渠道实现
// markdown.go 实现 Markdown 语法过滤器，将 Markdown 转换为纯文本
// 微信个人号不支持 Markdown 渲染，需要在发送前移除格式标记
package weixin

import (
	"regexp"
	"strings"
)

// MarkdownFilter 过滤 Markdown 语法，保留纯文本内容
// 微信个人号不支持 Markdown 渲染，需要移除格式标记
type MarkdownFilter struct {
	enabled bool // 是否启用过滤
}

// NewMarkdownFilter 创建 Markdown 过滤器
func NewMarkdownFilter(enabled bool) *MarkdownFilter {
	return &MarkdownFilter{enabled: enabled}
}

// IsEnabled 返回过滤器是否启用
func (f *MarkdownFilter) IsEnabled() bool {
	return f.enabled
}

// markdownPatterns 预编译的正则表达式，用于匹配各种 Markdown 语法
// 按处理顺序排列：先处理嵌套的复杂模式，再处理简单的
var markdownPatterns = []struct {
	re      *regexp.Regexp
	replace string // 使用 $1 引用捕获组
}{
	// 粗斜体 (***text***) — 必须在粗体和斜体之前处理
	{regexp.MustCompile(`\*\*\*(.+?)\*\*\*`), "$1"},
	// 粗体 (**text**)
	{regexp.MustCompile(`\*\*(.+?)\*\*`), "$1"},
	// 粗体替代语法 (__text__)
	{regexp.MustCompile(`__(.+?)__`), "$1"},
	// 斜体 (*text*)
	{regexp.MustCompile(`\*(.+?)\*`), "$1"},
	// 斜体替代语法 (_text_)
	{regexp.MustCompile(`_(.+?)_`), "$1"},
	// 行内代码 (`code`)
	{regexp.MustCompile("`(.+?)`"), "$1"},
	// 图片 (![alt](url)) — 保留 alt 文本
	{regexp.MustCompile(`!\[([^\]]*)\]\([^\)]+\)`), "$1"},
	// 链接 ([text](url)) — 保留显示文本
	{regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`), "$1"},
}

// 行级正则表达式，针对每行开头的语法
var (
	// 标题前缀 (# H1 ~ ###### H6)
	reHeading = regexp.MustCompile(`^#{1,6}\s+`)
	// 引用前缀 (> text)
	reBlockquote = regexp.MustCompile(`^>\s?`)
	// 无序列表前缀 (- item, * item, + item)
	reUnorderedList = regexp.MustCompile(`^[-*+]\s+`)
	// 有序列表前缀 (1. item)
	reOrderedList = regexp.MustCompile(`^\d+\.\s+`)
	// 水平线 (---, ***, ___)
	reHorizontalRule = regexp.MustCompile(`^[-*_]{3,}\s*$`)
	// HTML 标签
	reHTMLTag = regexp.MustCompile(`<[^>]+>`)
	// 代码块开始/结束标记
	reCodeBlockMarker = regexp.MustCompile("^```")
)

// Filter 对输入文本执行 Markdown 过滤
// 如果过滤器未启用，直接返回原文
// 处理逻辑：逐行扫描，保留代码块内容，移除其他 Markdown 格式标记
func (f *MarkdownFilter) Filter(text string) string {
	if !f.enabled || text == "" {
		return text
	}

	lines := strings.Split(text, "\n")
	var builder strings.Builder
	inCodeBlock := false

	for i, line := range lines {
		// 处理代码块边界
		if reCodeBlockMarker.MatchString(line) {
			if inCodeBlock {
				// 代码块结束：移除结束标记，保留代码块内容不变
				inCodeBlock = false
			} else {
				// 代码块开始：移除开始标记和语言标识
				inCodeBlock = true
			}
			// 代码块标记行本身不输出
			if i < len(lines)-1 {
				builder.WriteString("\n")
			}
			continue
		}

		if inCodeBlock {
			// 代码块内部：保留原始内容不做任何过滤
			builder.WriteString(line)
			if i < len(lines)-1 {
				builder.WriteString("\n")
			}
			continue
		}

		// 代码块外部：应用 Markdown 过滤规则
		filtered := f.filterLine(line)
		builder.WriteString(filtered)
		if i < len(lines)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// filterLine 对单行文本应用所有 Markdown 过滤规则
func (f *MarkdownFilter) filterLine(line string) string {
	// 先检查水平线，整行替换为空行
	if reHorizontalRule.MatchString(line) {
		return ""
	}

	// 处理行首语法（标题、引用、列表）
	line = reHeading.ReplaceAllString(line, "")
	line = reBlockquote.ReplaceAllString(line, "")
	line = reUnorderedList.ReplaceAllString(line, "")
	line = reOrderedList.ReplaceAllString(line, "")

	// 处理行内语法（粗斜体、粗体、斜体、代码、链接、图片）
	for _, pattern := range markdownPatterns {
		line = pattern.re.ReplaceAllString(line, pattern.replace)
	}

	// 移除 HTML 标签
	line = reHTMLTag.ReplaceAllString(line, "")

	return line
}
