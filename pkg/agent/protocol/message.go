// Package protocol 提供 Agent 交互协议的核心定义。
// message.go 定义了消息工具函数，用于修复和清理消息格式。
package protocol

import (
	"fmt"
	"strings"

	"github.com/bstr9/simpleclaw/pkg/logger"
)

// 合成的工具错误消息
const synthToolErrMsg = "Error: Missing tool_result adjacent to tool_use (session repair). The conversation history was inconsistent; continue from here."

// MessageBlock 表示消息中的一个内容块
type MessageBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   any    `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// Message 表示一条消息
type Message struct {
	Role    string         `json:"role"`
	Content []MessageBlock `json:"content,omitempty"`
}

// SanitizeMessages 验证并修复消息列表
// 1. tool_use 后必须紧跟包含 tool_result 的 user 消息
// 2. 移除开头的孤立 tool_result
// 3. 移除没有匹配 tool_use 的 tool_result
func SanitizeMessages(messages *[]Message) int {
	if messages == nil || len(*messages) == 0 {
		return 0
	}

	removed := 0

	// 1. 修复 tool_use/tool_result 邻接关系
	adjRepairs := repairToolUseAdjacency(messages)

	// 2. 移除开头的孤立 tool_result user 消息
	removed += removeLeadingOrphanToolResults(messages)

	// 3. 迭代移除不匹配的 tool_use/tool_result
	removed += removeMismatchedToolBlocks(messages)

	// 4. 如果有移除，重新修复邻接关系
	if removed > 0 {
		adjRepairs += repairToolUseAdjacency(messages)
	}

	if removed > 0 {
		logger.Info(fmt.Sprintf("消息验证: 移除了 %d 条损坏消息", removed))
	}
	if adjRepairs > 0 {
		logger.Info(fmt.Sprintf("消息验证: 邻接修复 %d 次", adjRepairs))
	}

	return removed + adjRepairs
}

// toolIDPair 保存 tool_use 和 tool_result ID 的配对信息
type toolIDPair struct {
	useIDs    map[string]bool
	resultIDs map[string]bool
}

// collectToolIDs 收集消息中所有的 tool_use 和 tool_result ID
func collectToolIDs(messages []Message) toolIDPair {
	useIDs := make(map[string]bool)
	resultIDs := make(map[string]bool)

	for _, msg := range messages {
		for _, block := range msg.Content {
			switch block.Type {
			case "tool_use":
				if block.ID != "" {
					useIDs[block.ID] = true
				}
			case "tool_result":
				if block.ToolUseID != "" {
					resultIDs[block.ToolUseID] = true
				}
			}
		}
	}

	return toolIDPair{useIDs: useIDs, resultIDs: resultIDs}
}

// findMismatchedIDs 找出不匹配的 tool_use 和 tool_result ID
func findMismatchedIDs(pair toolIDPair) (badUse, badResult map[string]bool) {
	badUse = make(map[string]bool)
	badResult = make(map[string]bool)

	for id := range pair.useIDs {
		if !pair.resultIDs[id] {
			badUse[id] = true
		}
	}
	for id := range pair.resultIDs {
		if !pair.useIDs[id] {
			badResult[id] = true
		}
	}

	return badUse, badResult
}

// removeLeadingOrphanToolResults 移除开头的孤立 tool_result user 消息
func removeLeadingOrphanToolResults(messages *[]Message) int {
	removed := 0

	for len(*messages) > 0 {
		first := (*messages)[0]
		if first.Role != "user" {
			break
		}
		if hasBlockType(first.Content, "tool_result") && !hasBlockType(first.Content, "text") {
			logger.Warn("移除开头的孤立 tool_result user 消息")
			*messages = (*messages)[1:]
			removed++
		} else {
			break
		}
	}

	return removed
}

// removeMismatchedToolBlocks 迭代移除不匹配的 tool_use/tool_result
func removeMismatchedToolBlocks(messages *[]Message) int {
	totalRemoved := 0

	for range 5 {
		pair := collectToolIDs(*messages)
		badUse, badResult := findMismatchedIDs(pair)

		if len(badUse) == 0 && len(badResult) == 0 {
			break
		}

		passRemoved := 0
		newMessages := make([]Message, 0, len(*messages))

		for _, msg := range *messages {
			shouldRemove, filteredMsg := processMessageForMismatch(msg, badUse, badResult, &passRemoved)
			if !shouldRemove {
				newMessages = append(newMessages, filteredMsg)
			}
		}

		*messages = newMessages
		totalRemoved += passRemoved

		if passRemoved == 0 {
			break
		}
	}

	return totalRemoved
}

// processMessageForMismatch 处理单条消息中的不匹配 tool_use/tool_result
func processMessageForMismatch(msg Message, badUse, badResult map[string]bool, removed *int) (shouldRemove bool, result Message) {
	result = msg

	if msg.Role == "assistant" && len(badUse) > 0 {
		if containsBadToolUse(msg.Content, badUse) {
			logger.Warn("移除包含不匹配 tool_use 的 assistant 消息")
			return true, msg
		}
	}

	if msg.Role == "user" && hasBlockType(msg.Content, "tool_result") && len(badResult) > 0 {
		hasBad := containsBadToolResult(msg.Content, badResult)
		if hasBad {
			if !hasBlockType(msg.Content, "text") {
				logger.Warn("移除包含不匹配 tool_result 的 user 消息")
				return true, msg
			}
			// 保留文本，移除不匹配的 tool_result
			result.Content = filterBadToolResults(msg.Content, badResult, removed)
		}
	}

	return false, result
}

// containsBadToolUse 检查消息是否包含不匹配的 tool_use
func containsBadToolUse(content []MessageBlock, badUse map[string]bool) bool {
	for _, block := range content {
		if block.Type == "tool_use" && badUse[block.ID] {
			return true
		}
	}
	return false
}

// containsBadToolResult 检查消息是否包含不匹配的 tool_result
func containsBadToolResult(content []MessageBlock, badResult map[string]bool) bool {
	for _, block := range content {
		if block.Type == "tool_result" && badResult[block.ToolUseID] {
			return true
		}
	}
	return false
}

// filterBadToolResults 过滤掉不匹配的 tool_result，返回新内容
func filterBadToolResults(content []MessageBlock, badResult map[string]bool, removed *int) []MessageBlock {
	newContent := make([]MessageBlock, 0, len(content))
	for _, block := range content {
		if block.Type == "tool_result" && badResult[block.ToolUseID] {
			*removed++
			continue
		}
		newContent = append(newContent, block)
	}
	return newContent
}

// repairToolUseAdjacency 修复 tool_use 和 tool_result 的邻接关系
func repairToolUseAdjacency(messages *[]Message) int {
	repairs := 0
	i := 0

	for i < len(*messages) {
		msg := (*messages)[i]
		if msg.Role != "assistant" {
			i++
			continue
		}

		required := collectToolUseIDsFromMessage(msg)
		if len(required) == 0 {
			i++
			continue
		}

		// 检查下一条消息是否存在
		if i+1 >= len(*messages) {
			repairs += appendSyntheticToolResults(messages, required)
			break
		}

		nxt := (*messages)[i+1]
		if nxt.Role != "user" {
			repairs += insertSyntheticToolResults(messages, i, required, nxt.Role)
			i += 2
			continue
		}

		// 检查是否所有 required 都存在
		present := collectToolResultIDsFromMessage(nxt)
		missing := findMissingIDs(required, present)

		if len(missing) == 0 {
			i++
			continue
		}

		// 在开头添加合成的 tool_result
		prependSyntheticToolResults(&(*messages)[i+1], missing)
		logger.Warn(fmt.Sprintf("为 Anthropic 邻接关系添加合成的 tool_result (缺失 IDs=%v)", missing))
		repairs += len(missing)
		i++
	}

	return repairs
}

// collectToolUseIDsFromMessage 从消息收集 tool_use ID
func collectToolUseIDsFromMessage(msg Message) []string {
	required := make([]string, 0)
	for _, block := range msg.Content {
		if block.Type == "tool_use" && block.ID != "" {
			required = append(required, block.ID)
		}
	}
	return required
}

// collectToolResultIDsFromMessage 从消息收集 tool_result ID
func collectToolResultIDsFromMessage(msg Message) map[string]bool {
	present := make(map[string]bool)
	for _, block := range msg.Content {
		if block.Type == "tool_result" && block.ToolUseID != "" {
			present[block.ToolUseID] = true
		}
	}
	return present
}

// findMissingIDs 找出缺失的 tool_result ID
func findMissingIDs(required []string, present map[string]bool) []string {
	missing := make([]string, 0)
	for _, tid := range required {
		if !present[tid] {
			missing = append(missing, tid)
		}
	}
	return missing
}

// createSyntheticToolResultBlocks 创建合成的 tool_result 块
func createSyntheticToolResultBlocks(ids []string) []MessageBlock {
	blocks := make([]MessageBlock, 0, len(ids))
	for _, tid := range ids {
		blocks = append(blocks, MessageBlock{
			Type:      "tool_result",
			ToolUseID: tid,
			Content:   synthToolErrMsg,
			IsError:   true,
		})
	}
	return blocks
}

// appendSyntheticToolResults 在末尾追加合成的 tool_result
func appendSyntheticToolResults(messages *[]Message, ids []string) int {
	blocks := createSyntheticToolResultBlocks(ids)
	*messages = append(*messages, Message{
		Role:    "user",
		Content: blocks,
	})
	logger.Warn("在末尾 assistant tool_use 后追加合成的 tool_result")
	return 1
}

// insertSyntheticToolResults 在指定位置插入合成的 tool_result
func insertSyntheticToolResults(messages *[]Message, index int, ids []string, nextRole string) int {
	blocks := createSyntheticToolResultBlocks(ids)
	newMsg := Message{Role: "user", Content: blocks}
	*messages = append((*messages)[:index+1], append([]Message{newMsg}, (*messages)[index+1:]...)...)
	logger.Warn(fmt.Sprintf("在 tool_use 后插入合成的 tool_result (下一条角色=%s)", nextRole))
	return 1
}

// prependSyntheticToolResults 在消息开头添加合成的 tool_result
func prependSyntheticToolResults(msg *Message, ids []string) {
	blocks := createSyntheticToolResultBlocks(ids)
	msg.Content = append(blocks, msg.Content...)
}

// hasBlockType 检查消息内容是否包含指定类型的块
func hasBlockType(content []MessageBlock, blockType string) bool {
	for _, block := range content {
		if block.Type == blockType {
			return true
		}
	}
	return false
}

// ExtractTextFromContent 从消息内容中提取纯文本
func ExtractTextFromContent(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []MessageBlock:
		return extractTextFromBlocks(c)
	case []map[string]any:
		return extractTextFromMaps(c)
	default:
		return fmt.Sprintf("%v", content)
	}
}

// extractTextFromBlocks 从 MessageBlock 切片提取文本
func extractTextFromBlocks(blocks []MessageBlock) string {
	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			texts = append(texts, block.Text)
		}
	}
	return joinTexts(texts)
}

// extractTextFromMaps 从 map 切片提取文本
func extractTextFromMaps(blocks []map[string]any) string {
	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if blockType, ok := block["type"].(string); ok && blockType == "text" {
			if text, ok := block["text"].(string); ok {
				texts = append(texts, text)
			}
		}
	}
	return joinTexts(texts)
}

// joinTexts 连接文本列表
func joinTexts(texts []string) string {
	return strings.Join(texts, "\n")
}

// CompressTurnToTextOnly 将完整的对话轮次压缩为轻量级的纯文本轮次
// 保留第一个用户文本和最后一个助手文本，移除中间的工具交互
func CompressTurnToTextOnly(messages []Message) []Message {
	var userText string
	var lastAssistantText string

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			if hasBlockType(msg.Content, "tool_result") {
				continue
			}
			if userText == "" {
				userText = ExtractTextFromContent(msg.Content)
			}
		case "assistant":
			text := ExtractTextFromContent(msg.Content)
			if text != "" {
				lastAssistantText = text
			}
		}
	}

	result := make([]Message, 0)
	if userText != "" {
		result = append(result, Message{
			Role: "user",
			Content: []MessageBlock{
				{Type: "text", Text: userText},
			},
		})
	}
	if lastAssistantText != "" {
		result = append(result, Message{
			Role: "assistant",
			Content: []MessageBlock{
				{Type: "text", Text: lastAssistantText},
			},
		})
	}

	return result
}
