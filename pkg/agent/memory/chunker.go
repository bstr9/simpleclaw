// Package memory 提供代理记忆管理功能。
// chunker.go 实现文本分块器，用于将长文本分割成小块以便索引和检索。
package memory

import (
	"strings"
)

// TextChunk 表示分块后的文本片段。
type TextChunk struct {
	// Text 是分块的文本内容。
	Text string `json:"text"`
	// StartLine 是分块在原始文本中的起始行号（从1开始）。
	StartLine int `json:"start_line"`
	// EndLine 是分块在原始文本中的结束行号。
	EndLine int `json:"end_line"`
}

// Chunker 定义了文本分块器的接口。
type Chunker interface {
	// ChunkText 将文本分割成多个重叠的分块。
	ChunkText(text string) []TextChunk
	// ChunkMarkdown 将 Markdown 文本分割，保留结构完整性。
	ChunkMarkdown(text string) []TextChunk
	// SetMaxTokens 设置每个分块的最大 token 数。
	SetMaxTokens(maxTokens int)
	// SetOverlapTokens 设置分块之间的重叠 token 数。
	SetOverlapTokens(overlapTokens int)
}

// TextChunker 实现 Chunker 接口，支持按 token 估算进行文本分块。
type TextChunker struct {
	maxTokens     int // 每个分块的最大 token 数
	overlapTokens int // 分块之间的重叠 token 数
	charsPerToken int // 每个 token 估算的字符数
}

// ChunkerOption 是 TextChunker 的函数式选项。
type ChunkerOption func(*TextChunker)

// WithChunkerMaxTokens 设置每个分块的最大 token 数。
func WithChunkerMaxTokens(maxTokens int) ChunkerOption {
	return func(c *TextChunker) {
		c.maxTokens = maxTokens
	}
}

// WithOverlapTokens 设置分块之间的重叠 token 数。
func WithOverlapTokens(overlapTokens int) ChunkerOption {
	return func(c *TextChunker) {
		c.overlapTokens = overlapTokens
	}
}

// WithCharsPerToken 设置每个 token 估算的字符数。
func WithCharsPerToken(charsPerToken int) ChunkerOption {
	return func(c *TextChunker) {
		c.charsPerToken = charsPerToken
	}
}

// NewTextChunker 创建一个新的 TextChunker 实例。
// maxTokens: 每个分块的最大 token 数，默认 500
// overlapTokens: 分块之间的重叠 token 数，默认 50
func NewTextChunker(maxTokens, overlapTokens int) *TextChunker {
	if maxTokens <= 0 {
		maxTokens = 500
	}
	if overlapTokens < 0 {
		overlapTokens = 0
	}

	return &TextChunker{
		maxTokens:     maxTokens,
		overlapTokens: overlapTokens,
		charsPerToken: 4, // 默认估算：英文/中文混合约 4 字符/token
	}
}

// NewTextChunkerWithOptions 使用选项创建 TextChunker。
func NewTextChunkerWithOptions(opts ...ChunkerOption) *TextChunker {
	c := &TextChunker{
		maxTokens:     500,
		overlapTokens: 50,
		charsPerToken: 4,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// ChunkText 将文本分割成重叠的分块。
func (c *TextChunker) ChunkText(text string) []TextChunk {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	maxChars := c.maxTokens * c.charsPerToken
	overlapChars := c.overlapTokens * c.charsPerToken

	return c.chunkLines(lines, maxChars, overlapChars)
}

// chunkLines 将行分割成分块
func (c *TextChunker) chunkLines(lines []string, maxChars, overlapChars int) []TextChunk {
	state := &chunkState{
		chunks:       []TextChunk{},
		currentChunk: []string{},
		currentChars: 0,
		startLine:    1,
	}

	for i, line := range lines {
		lineChars := len(line)

		if lineChars > maxChars {
			state = c.handleLongLine(state, line, i, maxChars)
			continue
		}

		c.processLine(state, line, lineChars, i, maxChars, overlapChars)
	}

	if len(state.currentChunk) > 0 {
		state.chunks = append(state.chunks, TextChunk{
			Text:      strings.Join(state.currentChunk, "\n"),
			StartLine: state.startLine,
			EndLine:   len(lines),
		})
	}

	return state.chunks
}

// handleLongLine 处理超过最大字符数的行
func (c *TextChunker) handleLongLine(state *chunkState, line string, lineIdx, maxChars int) *chunkState {
	if len(state.currentChunk) > 0 {
		state.chunks = append(state.chunks, TextChunk{
			Text:      strings.Join(state.currentChunk, "\n"),
			StartLine: state.startLine,
			EndLine:   lineIdx,
		})
	}

	lineChunks := c.splitLongLine(line, maxChars)
	for _, chunk := range lineChunks {
		state.chunks = append(state.chunks, TextChunk{
			Text:      chunk,
			StartLine: lineIdx + 1,
			EndLine:   lineIdx + 1,
		})
	}

	state.currentChunk = nil
	state.currentChars = 0
	state.startLine = lineIdx + 2
	return state
}

type chunkState struct {
	chunks       []TextChunk
	currentChunk []string
	currentChars int
	startLine    int
}

func (c *TextChunker) processLine(state *chunkState, line string, lineChars, lineIdx, maxChars, overlapChars int) {
	if state.currentChars+lineChars > maxChars && len(state.currentChunk) > 0 {
		state.chunks = append(state.chunks, TextChunk{
			Text:      strings.Join(state.currentChunk, "\n"),
			StartLine: state.startLine,
			EndLine:   lineIdx,
		})

		overlapLines := c.getOverlapLines(state.currentChunk, overlapChars)
		state.currentChunk = append(overlapLines, line)
		state.currentChars = 0
		for _, l := range state.currentChunk {
			state.currentChars += len(l)
		}
		state.startLine = lineIdx - len(overlapLines) + 1
	} else {
		state.currentChunk = append(state.currentChunk, line)
		state.currentChars += lineChars
	}
}

// ChunkMarkdown 将 Markdown 文本分割，保留结构完整性。
// 识别标题、代码块等结构，尽量在标题边界分割。
func (c *TextChunker) ChunkMarkdown(text string) []TextChunk {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	sections := c.parseMarkdownSections(lines)

	maxChars := c.maxTokens * c.charsPerToken
	chunks := []TextChunk{}

	currentChunk := []string{}
	currentChars := 0
	startLine := 1

	for _, section := range sections {
		sectionText := strings.Join(section.Lines, "\n")
		sectionChars := len(sectionText)

		if sectionChars > maxChars {
			if len(currentChunk) > 0 {
				chunks = append(chunks, TextChunk{
					Text:      strings.Join(currentChunk, "\n"),
					StartLine: startLine,
					EndLine:   section.StartLine - 1,
				})
				currentChunk = nil
				currentChars = 0
			}

			if section.IsCodeBlock {
				chunks = append(chunks, TextChunk{
					Text:      sectionText,
					StartLine: section.StartLine,
					EndLine:   section.EndLine,
				})
				startLine = section.EndLine + 1
				continue
			}

			subChunks := c.splitSectionByParagraph(section, maxChars)
			chunks = append(chunks, subChunks...)
			startLine = section.EndLine + 1
			continue
		}

		if currentChars+sectionChars+1 > maxChars && len(currentChunk) > 0 {
			chunks = append(chunks, TextChunk{
				Text:      strings.Join(currentChunk, "\n"),
				StartLine: startLine,
				EndLine:   section.StartLine - 1,
			})
			currentChunk = nil
			currentChars = 0
			startLine = section.StartLine
		}

		currentChunk = append(currentChunk, sectionText)
		currentChars += sectionChars + 1
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, TextChunk{
			Text:      strings.Join(currentChunk, "\n"),
			StartLine: startLine,
			EndLine:   len(lines),
		})
	}

	return chunks
}

type MarkdownSection struct {
	Lines        []string
	StartLine    int
	EndLine      int
	IsCodeBlock  bool
	HeadingLevel int
}

// parseMarkdownState 管理 Markdown 解析状态
type parseMarkdownState struct {
	sections       []MarkdownSection
	currentSection MarkdownSection
	inCodeBlock    bool
}

// parseMarkdownSections 将 Markdown 文本按结构分割成节
func (c *TextChunker) parseMarkdownSections(lines []string) []MarkdownSection {
	state := &parseMarkdownState{
		sections:       []MarkdownSection{},
		currentSection: MarkdownSection{StartLine: 1},
		inCodeBlock:    false,
	}

	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			state.handleCodeBlockToggle(i, line)
			continue
		}

		if state.inCodeBlock {
			state.currentSection.Lines = append(state.currentSection.Lines, line)
			continue
		}

		headingLevel := c.parseHeadingLevel(line)
		if headingLevel > 0 && len(state.currentSection.Lines) > 0 {
			state.startNewSection(i, headingLevel, line)
			continue
		}

		state.currentSection.Lines = append(state.currentSection.Lines, line)
	}

	state.finalize(len(lines))
	return state.sections
}

// handleCodeBlockToggle 处理代码块的开始和结束
func (s *parseMarkdownState) handleCodeBlockToggle(lineIdx int, line string) {
	if s.inCodeBlock {
		s.currentSection.Lines = append(s.currentSection.Lines, line)
		s.currentSection.EndLine = lineIdx + 1
		s.currentSection.IsCodeBlock = true
		s.sections = append(s.sections, s.currentSection)
		s.currentSection = MarkdownSection{StartLine: lineIdx + 2}
		s.inCodeBlock = false
		return
	}

	if len(s.currentSection.Lines) > 0 {
		s.currentSection.EndLine = lineIdx
		s.sections = append(s.sections, s.currentSection)
		s.currentSection = MarkdownSection{StartLine: lineIdx + 1}
	}
	s.currentSection.Lines = []string{line}
	s.inCodeBlock = true
}

// parseHeadingLevel 解析行的标题级别，返回 0 表示不是标题
func (c *TextChunker) parseHeadingLevel(line string) int {
	if !strings.HasPrefix(line, "#") {
		return 0
	}
	level := len(line) - len(strings.TrimLeft(line, "#"))
	if level > 6 {
		return 0
	}
	return level
}

// startNewSection 开始一个新的节
func (s *parseMarkdownState) startNewSection(lineIdx, headingLevel int, line string) {
	s.currentSection.EndLine = lineIdx
	s.sections = append(s.sections, s.currentSection)
	s.currentSection = MarkdownSection{
		StartLine:    lineIdx + 1,
		HeadingLevel: headingLevel,
	}
	s.currentSection.Lines = []string{line}
}

// finalize 完成解析，添加最后一个节
func (s *parseMarkdownState) finalize(totalLines int) {
	if len(s.currentSection.Lines) > 0 {
		s.currentSection.EndLine = totalLines
		s.sections = append(s.sections, s.currentSection)
	}
}

func (c *TextChunker) splitSectionByParagraph(section MarkdownSection, maxChars int) []TextChunk {
	chunks := []TextChunk{}
	currentChunk := []string{}
	currentChars := 0
	currentLine := section.StartLine

	paragraphs := strings.Split(strings.Join(section.Lines, "\n"), "\n\n")

	for _, para := range paragraphs {
		paraChars := len(para)

		if currentChars+paraChars+2 > maxChars && len(currentChunk) > 0 {
			chunks = append(chunks, TextChunk{
				Text:      strings.Join(currentChunk, "\n\n"),
				StartLine: section.StartLine,
				EndLine:   currentLine - 1,
			})
			currentChunk = nil
			currentChars = 0
			section.StartLine = currentLine
		}

		currentChunk = append(currentChunk, para)
		currentChars += paraChars + 2
		currentLine += strings.Count(para, "\n") + 1
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, TextChunk{
			Text:      strings.Join(currentChunk, "\n\n"),
			StartLine: section.StartLine,
			EndLine:   section.EndLine,
		})
	}

	return chunks
}

// ChunkByParagraph 按段落分割文本，优先在段落边界分割。
func (c *TextChunker) ChunkByParagraph(text string) []TextChunk {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	// 按空行分割段落
	paragraphs := strings.Split(text, "\n\n")
	chunks := []TextChunk{}

	maxChars := c.maxTokens * c.charsPerToken

	currentChunk := []string{}
	currentChars := 0
	startLine := 1
	currentLine := 1

	for _, para := range paragraphs {
		paraLines := strings.Split(para, "\n")
		paraChars := len(para)

		// 如果当前段落加上现有内容超过限制
		if currentChars+paraChars+2 > maxChars && len(currentChunk) > 0 {
			// 保存当前分块
			chunks = append(chunks, TextChunk{
				Text:      strings.Join(currentChunk, "\n\n"),
				StartLine: startLine,
				EndLine:   currentLine - 1,
			})

			currentChunk = nil
			currentChars = 0
			startLine = currentLine
		}

		// 添加段落到当前分块
		currentChunk = append(currentChunk, para)
		currentChars += paraChars + 2 // 加上 "\n\n"
		currentLine += len(paraLines)
	}

	// 保存最后一个分块
	if len(currentChunk) > 0 {
		chunks = append(chunks, TextChunk{
			Text:      strings.Join(currentChunk, "\n\n"),
			StartLine: startLine,
			EndLine:   currentLine - 1,
		})
	}

	return chunks
}

// getOverlapLines 返回最后一部分行，使其总字符数不超过 targetChars。
func (c *TextChunker) getOverlapLines(lines []string, targetChars int) []string {
	overlap := []string{}
	chars := 0

	// 从后向前遍历
	for i := len(lines) - 1; i >= 0; i-- {
		lineChars := len(lines[i])
		if chars+lineChars > targetChars {
			break
		}
		overlap = append([]string{lines[i]}, overlap...)
		chars += lineChars
	}

	return overlap
}

// splitLongLine 将长行分割成多个片段。
func (c *TextChunker) splitLongLine(line string, maxChars int) []string {
	chunks := []string{}
	for i := 0; i < len(line); i += maxChars {
		end := i + maxChars
		if end > len(line) {
			end = len(line)
		}
		chunks = append(chunks, line[i:end])
	}
	return chunks
}

// SetMaxTokens 设置每个分块的最大 token 数。
func (c *TextChunker) SetMaxTokens(maxTokens int) {
	if maxTokens > 0 {
		c.maxTokens = maxTokens
	}
}

// SetOverlapTokens 设置分块之间的重叠 token 数。
func (c *TextChunker) SetOverlapTokens(overlapTokens int) {
	if overlapTokens >= 0 {
		c.overlapTokens = overlapTokens
	}
}

// SetCharsPerToken 设置每个 token 估算的字符数。
func (c *TextChunker) SetCharsPerToken(charsPerToken int) {
	if charsPerToken > 0 {
		c.charsPerToken = charsPerToken
	}
}

// GetMaxTokens 返回每个分块的最大 token 数。
func (c *TextChunker) GetMaxTokens() int {
	return c.maxTokens
}

// GetOverlapTokens 返回分块之间的重叠 token 数。
func (c *TextChunker) GetOverlapTokens() int {
	return c.overlapTokens
}

// ChunkStats 包含分块统计信息。
type ChunkStats struct {
	TotalChunks     int // 总分块数
	TotalChars      int // 总字符数
	AvgChunkSize    int // 平均分块大小
	MaxChunkSize    int // 最大分块大小
	MinChunkSize    int // 最小分块大小
	EstimatedTokens int // 估算的总 token 数
}

// GetChunkStats 返回分块的统计信息。
func (c *TextChunker) GetChunkStats(chunks []TextChunk) ChunkStats {
	if len(chunks) == 0 {
		return ChunkStats{}
	}

	stats := ChunkStats{
		TotalChunks:  len(chunks),
		MinChunkSize: len(chunks[0].Text),
	}

	for _, chunk := range chunks {
		size := len(chunk.Text)
		stats.TotalChars += size
		if size > stats.MaxChunkSize {
			stats.MaxChunkSize = size
		}
		if size < stats.MinChunkSize {
			stats.MinChunkSize = size
		}
	}

	stats.AvgChunkSize = stats.TotalChars / stats.TotalChunks
	stats.EstimatedTokens = stats.TotalChars / c.charsPerToken

	return stats
}
