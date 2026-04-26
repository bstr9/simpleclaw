// Package agent 提供 AI Agent 核心实现
package agent

import "sync"

// ToolStage 定义工具执行阶段
type ToolStage int

const (
	// ToolStagePreProcess 预处理阶段，在 LLM 调用前执行
	ToolStagePreProcess ToolStage = iota
	// ToolStagePostProcess 后处理阶段，在 LLM 调用后执行
	ToolStagePostProcess
)

// String 返回 ToolStage 的字符串表示
func (s ToolStage) String() string {
	switch s {
	case ToolStagePreProcess:
		return "preprocess"
	case ToolStagePostProcess:
		return "postprocess"
	default:
		return "unknown"
	}
}

// Tool 定义代理工具接口
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Stage() ToolStage
	Execute(params map[string]any) (*ToolResult, error)
}

// ToolWithContext 支持上下文的工具接口
type ToolWithContext interface {
	Tool
	ExecuteWithContext(ctx *ToolContext, params map[string]any) (*ToolResult, error)
}

// ToolContext 工具执行上下文
type ToolContext struct {
	SessionID     string
	UserID        string
	GroupID       string
	IsGroup       bool
	ChannelType   string
	Receiver      string
	ReceiveIDType string
	Extra         map[string]any
}

// ToolResult 表示工具执行结果
type ToolResult struct {
	// Status 执行状态，"success" 或 "error"
	Status string `json:"status"`

	// Result 执行结果内容
	Result any `json:"result"`

	// ExtData 扩展数据，用于传递额外信息
	ExtData any `json:"ext_data,omitempty"`
}

// NewToolResult 创建成功的 ToolResult
func NewToolResult(result any) *ToolResult {
	return &ToolResult{
		Status: "success",
		Result: result,
	}
}

// NewErrorToolResult 创建错误的 ToolResult
func NewErrorToolResult(err error) *ToolResult {
	return &ToolResult{
		Status: "error",
		Result: err.Error(),
	}
}

// IsSuccess 检查是否执行成功
func (r *ToolResult) IsSuccess() bool {
	return r.Status == "success"
}

// ToolRegistry 工具注册表
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewToolRegistry 创建工具注册表
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register 注册工具
func (r *ToolRegistry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

// Get 获取工具
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// GetAll 获取所有工具
func (r *ToolRegistry) GetAll() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Remove 移除工具
func (r *ToolRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Clear 清空所有工具
func (r *ToolRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools = make(map[string]Tool)
}

// Count 返回工具数量
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// ToOpenAITools 转换为 OpenAI function calling 格式
func (r *ToolRegistry) ToOpenAITools() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]map[string]any, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name(),
				"description": tool.Description(),
				"parameters":  tool.Parameters(),
			},
		})
	}
	return tools
}
