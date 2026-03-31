package tools

import (
	"testing"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

type mockTool struct {
	name        string
	description string
	parameters  map[string]any
	stage       agent.ToolStage
}

func (m *mockTool) Name() string               { return m.name }
func (m *mockTool) Description() string        { return m.description }
func (m *mockTool) Parameters() map[string]any { return m.parameters }
func (m *mockTool) Stage() agent.ToolStage     { return m.stage }
func (m *mockTool) Execute(params map[string]any) (*agent.ToolResult, error) {
	return agent.NewToolResult("mock result"), nil
}

func TestToolRegistry(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*agent.ToolRegistry)
		validate func(t *testing.T, r *agent.ToolRegistry)
	}{
		{
			name: "注册单个工具",
			setup: func(r *agent.ToolRegistry) {
				r.Register(&mockTool{name: "test_tool", description: "测试工具"})
			},
			validate: func(t *testing.T, r *agent.ToolRegistry) {
				if r.Count() != 1 {
					t.Errorf("期望工具数量为 1，实际为 %d", r.Count())
				}
				tool, ok := r.Get("test_tool")
				if !ok {
					t.Error("期望找到工具 test_tool")
					return
				}
				if tool.Name() != "test_tool" {
					t.Errorf("期望工具名称为 test_tool，实际为 %s", tool.Name())
				}
			},
		},
		{
			name: "注册多个工具",
			setup: func(r *agent.ToolRegistry) {
				r.Register(&mockTool{name: "tool1", description: "工具1"})
				r.Register(&mockTool{name: "tool2", description: "工具2"})
				r.Register(&mockTool{name: "tool3", description: "工具3"})
			},
			validate: func(t *testing.T, r *agent.ToolRegistry) {
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
			name: "获取不存在的工具",
			setup: func(r *agent.ToolRegistry) {
				r.Register(&mockTool{name: "existing", description: "存在"})
			},
			validate: func(t *testing.T, r *agent.ToolRegistry) {
				_, ok := r.Get("nonexistent")
				if ok {
					t.Error("期望找不到工具 nonexistent")
				}
			},
		},
		{
			name: "移除工具",
			setup: func(r *agent.ToolRegistry) {
				r.Register(&mockTool{name: "to_remove", description: "待移除"})
				r.Remove("to_remove")
			},
			validate: func(t *testing.T, r *agent.ToolRegistry) {
				_, ok := r.Get("to_remove")
				if ok {
					t.Error("期望工具已被移除")
				}
			},
		},
		{
			name: "清空工具",
			setup: func(r *agent.ToolRegistry) {
				r.Register(&mockTool{name: "tool1", description: "工具1"})
				r.Register(&mockTool{name: "tool2", description: "工具2"})
				r.Clear()
			},
			validate: func(t *testing.T, r *agent.ToolRegistry) {
				if r.Count() != 0 {
					t.Errorf("期望工具数量为 0，实际为 %d", r.Count())
				}
			},
		},
		{
			name: "转换为 OpenAI 格式",
			setup: func(r *agent.ToolRegistry) {
				r.Register(&mockTool{
					name:        "search",
					description: "搜索工具",
					parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"query": map[string]any{
								"type":        "string",
								"description": "搜索查询",
							},
						},
					},
				})
			},
			validate: func(t *testing.T, r *agent.ToolRegistry) {
				openaiTools := r.ToOpenAITools()
				if len(openaiTools) != 1 {
					t.Errorf("期望 1 个 OpenAI 工具，实际为 %d", len(openaiTools))
					return
				}
				if openaiTools[0]["type"] != "function" {
					t.Errorf("期望类型为 function")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := agent.NewToolRegistry()
			tt.setup(r)
			tt.validate(t, r)
		})
	}
}

func TestToolResult(t *testing.T) {
	tests := []struct {
		name       string
		result     *agent.ToolResult
		wantStatus string
		wantOK     bool
	}{
		{
			name:       "成功结果",
			result:     agent.NewToolResult("操作成功"),
			wantStatus: "success",
			wantOK:     true,
		},
		{
			name:       "带错误的结果",
			result:     &agent.ToolResult{Status: "error", Result: "失败"},
			wantStatus: "error",
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Status != tt.wantStatus {
				t.Errorf("Status = %s, want %s", tt.result.Status, tt.wantStatus)
			}
			if tt.result.IsSuccess() != tt.wantOK {
				t.Errorf("IsSuccess() = %v, want %v", tt.result.IsSuccess(), tt.wantOK)
			}
		})
	}
}

func TestToolStage(t *testing.T) {
	tests := []struct {
		stage    agent.ToolStage
		expected string
	}{
		{agent.ToolStagePreProcess, "preprocess"},
		{agent.ToolStagePostProcess, "postprocess"},
		{agent.ToolStage(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.stage.String(); got != tt.expected {
				t.Errorf("String() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestWithWorkingDir(t *testing.T) {
	dir := "/test/workspace"
	opt := WithWorkingDir(dir)

	cfg := &toolConfig{}
	opt(cfg)

	if cfg.workingDir != dir {
		t.Errorf("期望 workingDir 为 %s，实际为 %s", dir, cfg.workingDir)
	}
}

func TestRegisterBuiltInTools(t *testing.T) {
	tests := []struct {
		name       string
		opts       []ToolOption
		wantMinCnt int
		checkTools []string
	}{
		{
			name:       "无选项注册",
			opts:       nil,
			wantMinCnt: 10,
			checkTools: []string{"read", "write", "time"},
		},
		{
			name:       "带工作目录选项",
			opts:       []ToolOption{WithWorkingDir("/custom/dir")},
			wantMinCnt: 10,
			checkTools: []string{"read", "write", "edit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := agent.NewToolRegistry()
			RegisterBuiltInTools(r, tt.opts...)

			if r.Count() < tt.wantMinCnt {
				t.Errorf("期望至少 %d 个工具，实际为 %d", tt.wantMinCnt, r.Count())
			}

			for _, toolName := range tt.checkTools {
				_, ok := r.Get(toolName)
				if !ok {
					t.Errorf("期望找到工具 %s", toolName)
				}
			}
		})
	}
}

func TestToolParameters(t *testing.T) {
	tests := []struct {
		name       string
		tool       agent.Tool
		wantParams bool
	}{
		{
			name: "有参数定义",
			tool: &mockTool{
				name:        "param_tool",
				description: "有参数的工具",
				parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"input": map[string]any{"type": "string"},
					},
				},
			},
			wantParams: true,
		},
		{
			name: "无参数定义",
			tool: &mockTool{
				name:        "no_param_tool",
				description: "无参数的工具",
				parameters:  nil,
			},
			wantParams: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := tt.tool.Parameters()
			hasParams := len(params) > 0
			if hasParams != tt.wantParams {
				t.Errorf("期望参数存在 = %v，实际为 %v", tt.wantParams, hasParams)
			}
		})
	}
}

func TestToolStageValues(t *testing.T) {
	tool := &mockTool{
		name:        "stage_test",
		description: "阶段测试工具",
		stage:       agent.ToolStagePreProcess,
	}

	if tool.Stage() != agent.ToolStagePreProcess {
		t.Errorf("期望阶段为 ToolStagePreProcess")
	}
}
