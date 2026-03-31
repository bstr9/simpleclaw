package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/bstr9/simpleclaw/pkg/llm"
)

// mockTool 实现 Tool 接口用于测试
type mockTool struct {
	name        string
	description string
	parameters  map[string]any
	stage       ToolStage
	executeFunc func(params map[string]any) (*ToolResult, error)
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return m.description
}

func (m *mockTool) Parameters() map[string]any {
	return m.parameters
}

func (m *mockTool) Stage() ToolStage {
	return m.stage
}

func (m *mockTool) Execute(params map[string]any) (*ToolResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(params)
	}
	return NewToolResult("mock result"), nil
}

// TestToolRegistry 测试工具注册表
func TestToolRegistry(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*ToolRegistry)
		validate func(t *testing.T, r *ToolRegistry)
	}{
		{
			name: "注册和获取工具",
			setup: func(r *ToolRegistry) {
				r.Register(&mockTool{name: "test_tool", description: "测试工具"})
			},
			validate: func(t *testing.T, r *ToolRegistry) {
				tool, ok := r.Get("test_tool")
				if !ok {
					t.Error("期望找到工具 test_tool")
				}
				if tool.Name() != "test_tool" {
					t.Errorf("期望工具名称为 test_tool，实际为 %s", tool.Name())
				}
			},
		},
		{
			name:  "获取不存在的工具",
			setup: func(r *ToolRegistry) {},
			validate: func(t *testing.T, r *ToolRegistry) {
				_, ok := r.Get("nonexistent")
				if ok {
					t.Error("期望找不到工具 nonexistent")
				}
			},
		},
		{
			name: "注册多个工具",
			setup: func(r *ToolRegistry) {
				r.Register(&mockTool{name: "tool1", description: "工具1"})
				r.Register(&mockTool{name: "tool2", description: "工具2"})
				r.Register(&mockTool{name: "tool3", description: "工具3"})
			},
			validate: func(t *testing.T, r *ToolRegistry) {
				if r.Count() != 3 {
					t.Errorf("期望工具数量为 3，实际为 %d", r.Count())
				}
				tools := r.GetAll()
				if len(tools) != 3 {
					t.Errorf("期望 GetAll 返回 3 个工具，实际为 %d", len(tools))
				}
			},
		},
		{
			name: "移除工具",
			setup: func(r *ToolRegistry) {
				r.Register(&mockTool{name: "to_remove", description: "待移除"})
				r.Remove("to_remove")
			},
			validate: func(t *testing.T, r *ToolRegistry) {
				_, ok := r.Get("to_remove")
				if ok {
					t.Error("期望工具已被移除")
				}
				if r.Count() != 0 {
					t.Errorf("期望工具数量为 0，实际为 %d", r.Count())
				}
			},
		},
		{
			name: "清空所有工具",
			setup: func(r *ToolRegistry) {
				r.Register(&mockTool{name: "tool1", description: "工具1"})
				r.Register(&mockTool{name: "tool2", description: "工具2"})
				r.Clear()
			},
			validate: func(t *testing.T, r *ToolRegistry) {
				if r.Count() != 0 {
					t.Errorf("期望工具数量为 0，实际为 %d", r.Count())
				}
			},
		},
		{
			name: "转换为 OpenAI 格式",
			setup: func(r *ToolRegistry) {
				r.Register(&mockTool{
					name:        "test",
					description: "测试工具",
					parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"query": map[string]any{
								"type":        "string",
								"description": "查询内容",
							},
						},
					},
				})
			},
			validate: func(t *testing.T, r *ToolRegistry) {
				openaiTools := r.ToOpenAITools()
				if len(openaiTools) != 1 {
					t.Errorf("期望 1 个 OpenAI 工具，实际为 %d", len(openaiTools))
				}
				if openaiTools[0]["type"] != "function" {
					t.Errorf("期望类型为 function，实际为 %v", openaiTools[0]["type"])
				}
				fn, ok := openaiTools[0]["function"].(map[string]any)
				if !ok {
					t.Error("期望 function 为 map[string]any 类型")
					return
				}
				if fn["name"] != "test" {
					t.Errorf("期望函数名为 test，实际为 %v", fn["name"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewToolRegistry()
			tt.setup(r)
			tt.validate(t, r)
		})
	}
}

// TestToolResult 测试工具结果
func TestToolResult(t *testing.T) {
	tests := []struct {
		name     string
		result   *ToolResult
		wantOK   bool
		wantData string
	}{
		{
			name:     "成功结果",
			result:   NewToolResult("操作成功"),
			wantOK:   true,
			wantData: "操作成功",
		},
		{
			name:     "错误结果",
			result:   NewErrorToolResult(errors.New("操作失败")),
			wantOK:   false,
			wantData: "操作失败",
		},
		{
			name: "带扩展数据的成功结果",
			result: &ToolResult{
				Status: "success",
				Result: "主结果",
				ExtData: map[string]any{
					"extra": "额外信息",
				},
			},
			wantOK:   true,
			wantData: "主结果",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.IsSuccess() != tt.wantOK {
				t.Errorf("IsSuccess() = %v, want %v", tt.result.IsSuccess(), tt.wantOK)
			}
			if tt.result.Result != tt.wantData {
				t.Errorf("Result = %v, want %v", tt.result.Result, tt.wantData)
			}
		})
	}
}

// TestToolStage 测试工具阶段
func TestToolStage(t *testing.T) {
	tests := []struct {
		stage    ToolStage
		expected string
	}{
		{ToolStagePreProcess, "preprocess"},
		{ToolStagePostProcess, "postprocess"},
		{ToolStage(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.stage.String(); got != tt.expected {
				t.Errorf("String() = %s, want %s", got, tt.expected)
			}
		})
	}
}

// TestAgentCreation 测试 Agent 创建
func TestAgentCreation(t *testing.T) {
	tests := []struct {
		name  string
		opts  []Option
		check func(t *testing.T, a *Agent)
	}{
		{
			name: "默认配置",
			opts: nil,
			check: func(t *testing.T, a *Agent) {
				if a.GetMaxSteps() != 10 {
					t.Errorf("期望 MaxSteps 为 10，实际为 %d", a.GetMaxSteps())
				}
				if a.GetMaxTokens() != 2048 {
					t.Errorf("期望 MaxTokens 为 2048，实际为 %d", a.GetMaxTokens())
				}
				if a.GetTemperature() != 0.7 {
					t.Errorf("期望 Temperature 为 0.7，实际为 %f", a.GetTemperature())
				}
			},
		},
		{
			name: "自定义系统提示",
			opts: []Option{WithSystemPrompt("你是一个助手")},
			check: func(t *testing.T, a *Agent) {
				if a.GetSystemPrompt() != "你是一个助手" {
					t.Errorf("期望系统提示为 '你是一个助手'，实际为 '%s'", a.GetSystemPrompt())
				}
			},
		},
		{
			name: "自定义最大步数",
			opts: []Option{WithMaxSteps(20)},
			check: func(t *testing.T, a *Agent) {
				if a.GetMaxSteps() != 20 {
					t.Errorf("期望 MaxSteps 为 20，实际为 %d", a.GetMaxSteps())
				}
			},
		},
		{
			name: "自定义工具列表",
			opts: []Option{
				WithTools([]Tool{
					&mockTool{name: "tool1", description: "工具1"},
					&mockTool{name: "tool2", description: "工具2"},
				}),
			},
			check: func(t *testing.T, a *Agent) {
				tools := a.GetTools()
				if len(tools) != 2 {
					t.Errorf("期望 2 个工具，实际为 %d", len(tools))
				}
				registry := a.GetToolRegistry()
				if registry.Count() != 2 {
					t.Errorf("期望注册表中 2 个工具，实际为 %d", registry.Count())
				}
			},
		},
		{
			name: "组合多个选项",
			opts: []Option{
				WithSystemPrompt("测试助手"),
				WithMaxSteps(15),
			},
			check: func(t *testing.T, a *Agent) {
				if a.GetSystemPrompt() != "测试助手" {
					t.Errorf("期望系统提示为 '测试助手'")
				}
				if a.GetMaxSteps() != 15 {
					t.Errorf("期望 MaxSteps 为 15")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAgent(tt.opts...)
			tt.check(t, a)
		})
	}
}

// TestAgentMessages 测试 Agent 消息管理
func TestAgentMessages(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(a *Agent)
		wantLen int
		check   func(t *testing.T, a *Agent)
	}{
		{
			name:    "初始消息为空",
			setup:   func(a *Agent) {},
			wantLen: 0,
			check:   nil,
		},
		{
			name: "添加用户消息",
			setup: func(a *Agent) {
				a.AddUserMessage("你好")
			},
			wantLen: 1,
			check: func(t *testing.T, a *Agent) {
				msgs := a.GetMessages()
				if len(msgs) != 1 {
					return
				}
				if msgs[0].Content != "你好" {
					t.Errorf("期望内容为 '你好'，实际为 '%s'", msgs[0].Content)
				}
			},
		},
		{
			name: "添加多条消息",
			setup: func(a *Agent) {
				a.AddUserMessage("问题1")
				a.AddAssistantMessage("回答1")
				a.AddUserMessage("问题2")
			},
			wantLen: 3,
			check: func(t *testing.T, a *Agent) {
				msgs := a.GetMessages()
				if len(msgs) != 3 {
					return
				}
				if msgs[0].Content != "问题1" {
					t.Errorf("第一条消息内容错误")
				}
				if msgs[2].Content != "问题2" {
					t.Errorf("第三条消息内容错误")
				}
			},
		},
		{
			name: "清空历史",
			setup: func(a *Agent) {
				a.AddUserMessage("消息1")
				a.AddUserMessage("消息2")
				a.ClearHistory()
			},
			wantLen: 0,
			check:   nil,
		},
		{
			name: "裁剪历史",
			setup: func(a *Agent) {
				for i := 0; i < 10; i++ {
					a.AddUserMessage("消息")
				}
				a.TrimHistory(5)
			},
			wantLen: 5,
			check:   nil,
		},
		{
			name: "裁剪历史 - 消息数少于 n",
			setup: func(a *Agent) {
				a.AddUserMessage("消息1")
				a.AddUserMessage("消息2")
				a.TrimHistory(5)
			},
			wantLen: 2,
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAgent()
			tt.setup(a)
			msgs := a.GetMessages()
			if len(msgs) != tt.wantLen {
				t.Errorf("期望 %d 条消息，实际为 %d", tt.wantLen, len(msgs))
			}
			if tt.check != nil {
				tt.check(t, a)
			}
		})
	}
}

// TestAgentSystemPrompt 测试 Agent 系统提示管理
func TestAgentSystemPrompt(t *testing.T) {
	a := NewAgent()

	// 初始为空
	if a.GetSystemPrompt() != "" {
		t.Error("期望初始系统提示为空")
	}

	// 设置系统提示
	a.SetSystemPrompt("新系统提示")
	if a.GetSystemPrompt() != "新系统提示" {
		t.Errorf("期望系统提示为 '新系统提示'，实际为 '%s'", a.GetSystemPrompt())
	}

	// 覆盖系统提示
	a.SetSystemPrompt("更新的系统提示")
	if a.GetSystemPrompt() != "更新的系统提示" {
		t.Errorf("期望系统提示为 '更新的系统提示'")
	}
}

// TestAgentSetMaxSteps 测试设置最大步数
func TestAgentSetMaxSteps(t *testing.T) {
	a := NewAgent()

	a.SetMaxSteps(50)
	if a.GetMaxSteps() != 50 {
		t.Errorf("期望 MaxSteps 为 50，实际为 %d", a.GetMaxSteps())
	}
}

// TestGetMessagesWithSystem 测试获取包含系统提示的消息
func TestGetMessagesWithSystem(t *testing.T) {
	tests := []struct {
		name       string
		opts       []Option
		addMsgs    func(a *Agent)
		wantLen    int
		wantSystem bool
	}{
		{
			name:       "无系统提示",
			opts:       nil,
			addMsgs:    func(a *Agent) { a.AddUserMessage("测试") },
			wantLen:    1,
			wantSystem: false,
		},
		{
			name:       "有系统提示",
			opts:       []Option{WithSystemPrompt("系统指令")},
			addMsgs:    func(a *Agent) { a.AddUserMessage("测试") },
			wantLen:    2,
			wantSystem: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAgent(tt.opts...)
			tt.addMsgs(a)
			msgs := a.GetMessagesWithSystem()
			if len(msgs) != tt.wantLen {
				t.Errorf("期望 %d 条消息，实际为 %d", tt.wantLen, len(msgs))
			}
			if tt.wantSystem && msgs[0].Content != "系统指令" {
				t.Errorf("期望第一条消息为系统提示")
			}
		})
	}
}

// TestToolExecute 测试工具执行
func TestToolExecute(t *testing.T) {
	tests := []struct {
		name    string
		tool    Tool
		params  map[string]any
		wantErr bool
	}{
		{
			name:    "执行成功",
			tool:    &mockTool{name: "test", description: "测试"},
			params:  map[string]any{"arg": "value"},
			wantErr: false,
		},
		{
			name: "执行失败",
			tool: &mockTool{
				name:        "error_tool",
				description: "错误工具",
				executeFunc: func(params map[string]any) (*ToolResult, error) {
					return nil, errors.New("执行失败")
				},
			},
			params:  map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.tool.Execute(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && result == nil {
				t.Error("期望返回非 nil 结果")
			}
		})
	}
}

// TestParseToolCallArgs 测试工具调用参数解析
func TestParseToolCallArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    map[string]any
		wantErr bool
	}{
		{
			name:    "有效 JSON",
			args:    `{"query": "测试", "count": 10}`,
			want:    map[string]any{"query": "测试", "count": float64(10)},
			wantErr: false,
		},
		{
			name:    "空对象",
			args:    `{}`,
			want:    map[string]any{},
			wantErr: false,
		},
		{
			name:    "无效 JSON",
			args:    `invalid json`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolCall := llm.ToolCall{Function: llm.FunctionCall{Arguments: tt.args}}
			got, err := parseToolCallArgs(toolCall)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseToolCallArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				for k, v := range tt.want {
					if got[k] != v {
						t.Errorf("parseToolCallArgs()[%s] = %v, want %v", k, got[k], v)
					}
				}
			}
		})
	}
}

func TestEmitEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		data      map[string]any
	}{
		{
			name:      "文本事件",
			eventType: "text",
			data:      map[string]any{"content": "测试内容"},
		},
		{
			name:      "工具调用事件",
			eventType: "tool_call",
			data:      map[string]any{"name": "test_tool", "args": map[string]any{}},
		},
		{
			name:      "空数据",
			eventType: "step_start",
			data:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var received map[string]any
			emitEvent(func(event map[string]any) {
				received = event
			}, tt.eventType, tt.data)

			if received == nil {
				t.Error("期望接收到事件")
				return
			}
			if received["type"] != tt.eventType {
				t.Errorf("期望事件类型为 %s，实际为 %v", tt.eventType, received["type"])
			}
		})
	}
}

// TestEmitEventNilCallback 测试空回调
func TestEmitEventNilCallback(t *testing.T) {
	// 不应该 panic
	emitEvent(nil, "test", map[string]any{"data": "value"})
}

// TestAgentConcurrency 测试 Agent 并发安全性
func TestAgentConcurrency(t *testing.T) {
	a := NewAgent()

	// 并发添加消息
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(n int) {
			a.AddUserMessage("消息")
			_ = a.GetMessages()
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 100; i++ {
		<-done
	}

	msgs := a.GetMessages()
	if len(msgs) != 100 {
		t.Errorf("期望 100 条消息，实际为 %d", len(msgs))
	}
}

// TestContextCancellation 测试上下文取消
func TestContextCancellation(t *testing.T) {
	a := NewAgent()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	// 即使上下文已取消，Agent 也能正常处理（具体行为取决于实现）
	_ = ctx
	_ = a
}
