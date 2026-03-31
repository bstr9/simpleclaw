package tools

import (
	"testing"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

func TestBashTool_Name(t *testing.T) {
	tool := NewBashTool()
	if tool.Name() != "bash" {
		t.Errorf("Expected name 'bash', got '%s'", tool.Name())
	}
}

func TestBashTool_Description(t *testing.T) {
	tool := NewBashTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestBashTool_Parameters(t *testing.T) {
	tool := NewBashTool()
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Error("Expected type to be 'object'")
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	if _, exists := props["command"]; !exists {
		t.Error("Expected 'command' property to exist")
	}
}

func TestBashTool_Stage(t *testing.T) {
	tool := NewBashTool()
	if tool.Stage() != agent.ToolStagePostProcess {
		t.Error("Expected stage to be ToolStagePostProcess")
	}
}

func TestBashTool_Execute_MissingCommand(t *testing.T) {
	tool := NewBashTool()
	result, err := tool.Execute(map[string]any{})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing command")
	}
}

func TestBashTool_Execute_SimpleCommand(t *testing.T) {
	tool := NewBashTool()
	result, err := tool.Execute(map[string]any{"command": "echo hello"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	output := result.Result.(string)
	if !containsSubstring(output, "hello") {
		t.Errorf("Expected output to contain 'hello', got: %s", output)
	}
}

func TestBashTool_Execute_CommandWithExitCode(t *testing.T) {
	tool := NewBashTool()
	result, err := tool.Execute(map[string]any{"command": "ls /nonexistent_dir_12345"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for failed command")
	}
}

func TestBashTool_Execute_DeniedCommand(t *testing.T) {
	tool := NewBashTool()
	result, err := tool.Execute(map[string]any{"command": "rm -rf /"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for denied command")
	}
}

func TestBashTool_Execute_WithTimeout(t *testing.T) {
	tool := NewBashTool()
	result, err := tool.Execute(map[string]any{
		"command": "echo test",
		"timeout": float64(5),
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}
}

func TestBashTool_WithOptions(t *testing.T) {
	tool := NewBashTool(
		WithBashWorkingDir("/tmp"),
		WithBashTimeout(10),
	)

	if tool.workingDir != "/tmp" {
		t.Errorf("Expected workingDir '/tmp', got '%s'", tool.workingDir)
	}

	if tool.timeout != 10 {
		t.Errorf("Expected timeout 10, got %v", tool.timeout)
	}
}

func TestBashTool_CheckCommand(t *testing.T) {
	tool := NewBashTool()

	tests := []struct {
		command string
		wantErr bool
	}{
		{"ls", false},
		{"echo test", false},
		{"rm file", true},
		{"dd if=/dev/zero", true},
		{"shutdown now", true},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			err := tool.checkCommand(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkCommand(%s) error = %v, wantErr %v", tt.command, err, tt.wantErr)
			}
		})
	}
}

func TestContainsCommand(t *testing.T) {
	tests := []struct {
		command string
		target  string
		want    bool
	}{
		{"ls", "ls", true},
		{"ls -la", "ls", true},
		{"rm file", "rm", true},
		{"cat file", "cat", true},
		{"echo test", "echo", true},
		{"ls", "ls -la", false},
		{"cat file", "ls", false},
	}

	for _, tt := range tests {
		t.Run(tt.command+"_"+tt.target, func(t *testing.T) {
			if got := containsCommand(tt.command, tt.target); got != tt.want {
				t.Errorf("containsCommand(%s, %s) = %v, want %v", tt.command, tt.target, got, tt.want)
			}
		})
	}
}
