// Package banwords 提供敏感词过滤插件，使用 AC 自动机算法。
// 支持关键词匹配、正则表达式和消息过滤/替换。
package banwords

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/bstr9/simpleclaw/pkg/plugin"
)

// Config 表示 banwords 插件的配置。
type Config struct {
	// Action 定义如何处理包含敏感词的消息。
	// 选项: "ignore" (阻止消息), "replace" (用 * 替换词语)
	Action string `json:"action"`

	// ReplyFilter 启用机器人回复过滤。
	ReplyFilter bool `json:"reply_filter"`

	// ReplyAction 定义如何处理包含敏感词的机器人回复。
	ReplyAction string `json:"reply_action"`

	// KeywordsFile 是关键词文件的路径（相对于插件目录）。
	KeywordsFile string `json:"keywords_file"`

	// RegexPatterns 包含额外的正则表达式匹配模式。
	RegexPatterns []string `json:"regex_patterns"`
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		Action:       "ignore",
		ReplyFilter:  true,
		ReplyAction:  "ignore",
		KeywordsFile: "banwords.txt",
	}
}

// MatchResult 表示找到的敏感词匹配。
type MatchResult struct {
	Keyword string
	Start   int
	End     int
}

// BanwordsPlugin 实现 Plugin 接口用于敏感词过滤。
type BanwordsPlugin struct {
	*plugin.BasePlugin

	mu       sync.RWMutex
	config   *Config
	trie     *ahoCorasick
	patterns []*regexp.Regexp
	keywords []string
}

// New 创建一个新的 BanwordsPlugin 实例。
func New() *BanwordsPlugin {
	p := &BanwordsPlugin{
		BasePlugin: plugin.NewBasePlugin("banwords", "1.0.0"),
		config:     DefaultConfig(),
		trie:       newAhoCorasick(),
	}
	p.SetDescription("Filter sensitive words in messages and replies")
	p.SetAuthor("simpleclaw")
	p.SetPriority(100)
	p.SetHidden(true)
	return p
}

// OnInit 使用给定的上下文初始化插件。
func (p *BanwordsPlugin) OnInit(ctx *plugin.PluginContext) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 加载配置
	if err := p.loadConfig(ctx); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 加载关键词文件
	keywordsPath := filepath.Join(ctx.PluginDir, p.config.KeywordsFile)
	if err := p.loadKeywords(keywordsPath); err != nil {
		// 如果关键词文件不存在，创建默认文件
		if os.IsNotExist(err) {
			if createErr := p.createDefaultKeywordsFile(keywordsPath); createErr != nil {
				return fmt.Errorf("failed to create default keywords file: %w", createErr)
			}
		} else {
			return fmt.Errorf("failed to load keywords: %w", err)
		}
	}

	// 编译正则表达式
	if err := p.compilePatterns(); err != nil {
		return fmt.Errorf("failed to compile patterns: %w", err)
	}

	return nil
}

// OnLoad 当插件加载时注册事件处理器。
func (p *BanwordsPlugin) OnLoad(ctx *plugin.PluginContext) error {
	p.RegisterHandler(plugin.EventOnHandleContext, p.onHandleContext)
	if p.config.ReplyFilter {
		p.RegisterHandler(plugin.EventOnDecorateReply, p.onDecorateReply)
	}
	return nil
}

// OnUnload 当插件卸载时清理资源。
func (p *BanwordsPlugin) OnUnload(ctx *plugin.PluginContext) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.trie = newAhoCorasick()
	p.patterns = nil
	p.keywords = nil
	return nil
}

// loadConfig 从上下文加载插件配置或创建默认配置。
func (p *BanwordsPlugin) loadConfig(ctx *plugin.PluginContext) error {
	if ctx.Config != nil {
		data, err := json.Marshal(ctx.Config)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, p.config); err != nil {
			return err
		}
	}
	return nil
}

// loadKeywords 从指定文件加载关键词。
func (p *BanwordsPlugin) loadKeywords(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var words []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" && !strings.HasPrefix(word, "#") {
			words = append(words, word)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	p.keywords = words
	p.trie.Build(words)
	return nil
}

// createDefaultKeywordsFile 创建默认关键词文件。
func (p *BanwordsPlugin) createDefaultKeywordsFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	defaultContent := `# 敏感词列表
# 每行一个词，以 # 开头的行是注释
# 示例:
# 敏感词1
# 敏感词2
`
	return os.WriteFile(path, []byte(defaultContent), 0644)
}

// compilePatterns 从配置编译正则表达式。
func (p *BanwordsPlugin) compilePatterns() error {
	p.patterns = nil
	for _, pattern := range p.config.RegexPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
		}
		p.patterns = append(p.patterns, re)
	}
	return nil
}

// onHandleContext 处理 ON_HANDLE_CONTEXT 事件用于接收消息。
func (p *BanwordsPlugin) onHandleContext(ec *plugin.EventContext) error {
	content, ok := ec.GetString("content")
	if !ok || content == "" {
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	switch p.config.Action {
	case "ignore":
		if match := p.FindFirst(content); match != nil {
			ec.BreakPass(p.Name())
			return nil
		}
	case "replace":
		if p.ContainsAny(content) {
			filtered := p.Filter(content)
			ec.Set("content", filtered)
			ec.Set("banwords_replaced", true)
		}
	}

	return nil
}

// onDecorateReply 处理 ON_DECORATE_REPLY 事件用于机器人回复。
func (p *BanwordsPlugin) onDecorateReply(ec *plugin.EventContext) error {
	reply, ok := ec.GetString("reply")
	if !ok || reply == "" {
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	switch p.config.ReplyAction {
	case "ignore":
		if match := p.FindFirst(reply); match != nil {
			ec.Set("reply", "")
			ec.BreakPass(p.Name())
			return nil
		}
	case "replace":
		if p.ContainsAny(reply) {
			filtered := p.Filter(reply)
			ec.Set("reply", filtered)
		}
	}

	return nil
}

// Check 检查文本是否包含敏感词。
// 如果找到则返回 true 和匹配的关键词，否则返回 false 和空字符串。
func (p *BanwordsPlugin) Check(text string) (bool, string) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if match := p.FindFirst(text); match != nil {
		return true, match.Keyword
	}
	return false, ""
}

// FindFirst 在文本中查找第一个敏感词匹配。
func (p *BanwordsPlugin) FindFirst(text string) *MatchResult {
	// 首先检查基于 trie 的关键词
	if match := p.trie.FindFirst(text); match != nil {
		return match
	}

	// 检查正则表达式
	for _, re := range p.patterns {
		if loc := re.FindStringIndex(text); loc != nil {
			return &MatchResult{
				Keyword: text[loc[0]:loc[1]],
				Start:   loc[0],
				End:     loc[1],
			}
		}
	}

	return nil
}

// FindAll 在文本中查找所有敏感词匹配。
func (p *BanwordsPlugin) FindAll(text string) []*MatchResult {
	var results []*MatchResult

	// 查找基于 trie 的匹配
	results = append(results, p.trie.FindAll(text)...)

	// 查找正则表达式匹配
	for _, re := range p.patterns {
		matches := re.FindAllStringIndex(text, -1)
		for _, loc := range matches {
			results = append(results, &MatchResult{
				Keyword: text[loc[0]:loc[1]],
				Start:   loc[0],
				End:     loc[1],
			})
		}
	}

	return results
}

// ContainsAny 检查文本是否包含任何敏感词。
func (p *BanwordsPlugin) ContainsAny(text string) bool {
	if p.trie.ContainsAny(text) {
		return true
	}
	for _, re := range p.patterns {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// Filter 用星号替换文本中的敏感词。
func (p *BanwordsPlugin) Filter(text string) string {
	result := []rune(text)

	// 替换基于 trie 的匹配
	for _, match := range p.trie.FindAll(text) {
		for i := match.Start; i < match.End && i < len(result); i++ {
			result[i] = '*'
		}
	}

	// 替换正则表达式匹配
	for _, re := range p.patterns {
		matches := re.FindAllStringIndex(string(result), -1)
		// 按逆序处理以保持索引
		for i := len(matches) - 1; i >= 0; i-- {
			loc := matches[i]
			for j := loc[0]; j < loc[1] && j < len(result); j++ {
				result[j] = '*'
			}
		}
	}

	return string(result)
}

// AddKeyword 动态添加关键词。
func (p *BanwordsPlugin) AddKeyword(keyword string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.keywords = append(p.keywords, keyword)
	p.trie.Build(p.keywords)
}

// AddPattern 动态添加正则表达式模式。
func (p *BanwordsPlugin) AddPattern(pattern string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	p.patterns = append(p.patterns, re)
	return nil
}

// Keywords 返回所有已加载的关键词。
func (p *BanwordsPlugin) Keywords() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]string, len(p.keywords))
	copy(result, p.keywords)
	return result
}

// ahoCorasick 实现 AC 自动机字符串匹配算法。
type ahoCorasick struct {
	root  *trieNode
	built bool
}

// trieNode 表示 AC 自动机 trie 中的节点。
type trieNode struct {
	children map[rune]*trieNode
	fail     *trieNode
	output   []int
	depth    int
}

// newAhoCorasick 创建一个新的 AC 自动机。
func newAhoCorasick() *ahoCorasick {
	return &ahoCorasick{
		root: &trieNode{
			children: make(map[rune]*trieNode),
		},
		built: false,
	}
}

// Build 根据关键词构建 AC 自动机。
func (ac *ahoCorasick) Build(keywords []string) {
	ac.resetRoot()
	ac.buildTrie(keywords)
	ac.buildFailLinks()
	ac.built = true
}

// resetRoot 重置 trie 根节点。
func (ac *ahoCorasick) resetRoot() {
	ac.root = &trieNode{
		children: make(map[rune]*trieNode),
	}
}

// buildTrie 构建 trie 树结构。
func (ac *ahoCorasick) buildTrie(keywords []string) {
	for i, keyword := range keywords {
		node := ac.root
		for _, ch := range keyword {
			if _, exists := node.children[ch]; !exists {
				node.children[ch] = &trieNode{
					children: make(map[rune]*trieNode),
					depth:    node.depth + 1,
				}
			}
			node = node.children[ch]
		}
		node.output = append(node.output, i)
	}
}

// buildFailLinks 使用 BFS 构建失败链接。
func (ac *ahoCorasick) buildFailLinks() {
	queue := ac.initRootChildrenFailLinks()

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		queue = ac.processChildrenFailLinks(current, queue)
	}
}

// initRootChildrenFailLinks 初始化根节点子节点的失败链接。
func (ac *ahoCorasick) initRootChildrenFailLinks() []*trieNode {
	var queue []*trieNode
	for _, child := range ac.root.children {
		child.fail = ac.root
		queue = append(queue, child)
	}
	return queue
}

// processChildrenFailLinks 处理子节点的失败链接并返回更新后的队列。
func (ac *ahoCorasick) processChildrenFailLinks(current *trieNode, queue []*trieNode) []*trieNode {
	for ch, child := range current.children {
		queue = append(queue, child)
		ac.setChildFailLink(current, child, ch)
		child.output = append(child.output, child.fail.output...)
	}
	return queue
}

// setChildFailLink 设置子节点的失败链接。
func (ac *ahoCorasick) setChildFailLink(current, child *trieNode, ch rune) {
	fail := current.fail
	for fail != nil {
		if next, exists := fail.children[ch]; exists {
			child.fail = next
			return
		}
		fail = fail.fail
	}
	child.fail = ac.root
}

// FindFirst 在文本中查找第一个关键词匹配。
func (ac *ahoCorasick) FindFirst(text string) *MatchResult {
	if !ac.built {
		return nil
	}

	node := ac.root
	runes := []rune(text)

	for i, ch := range runes {
		for node != nil && node.children[ch] == nil {
			node = node.fail
		}

		if node == nil {
			node = ac.root
			continue
		}

		node = node.children[ch]

		if len(node.output) > 0 {
			_ = node.output[0] // keyword index (not used directly)
			keywordLen := node.depth
			return &MatchResult{
				Keyword: string(runes[i-keywordLen+1 : i+1]),
				Start:   i - keywordLen + 1,
				End:     i + 1,
			}
		}
	}

	return nil
}

// FindAll 在文本中查找所有关键词匹配。
func (ac *ahoCorasick) FindAll(text string) []*MatchResult {
	if !ac.built {
		return nil
	}

	var results []*MatchResult
	node := ac.root
	runes := []rune(text)

	for i, ch := range runes {
		for node != nil && node.children[ch] == nil {
			node = node.fail
		}

		if node == nil {
			node = ac.root
			continue
		}

		node = node.children[ch]

		for range node.output {
			keywordLen := node.depth
			results = append(results, &MatchResult{
				Keyword: string(runes[i-keywordLen+1 : i+1]),
				Start:   i - keywordLen + 1,
				End:     i + 1,
			})
		}
	}

	return results
}

// ContainsAny 检查文本是否包含任何关键词。
func (ac *ahoCorasick) ContainsAny(text string) bool {
	return ac.FindFirst(text) != nil
}
