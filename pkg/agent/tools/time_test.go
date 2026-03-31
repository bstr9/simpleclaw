package tools

import (
	"strings"
	"testing"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

func TestTimeTool_Name(t *testing.T) {
	tool := NewTimeTool()
	if tool.Name() != "time" {
		t.Errorf("Expected name 'time', got '%s'", tool.Name())
	}
}

func TestTimeTool_Description(t *testing.T) {
	tool := NewTimeTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestTimeTool_Parameters(t *testing.T) {
	tool := NewTimeTool()
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Error("Expected type to be 'object'")
	}
}

func TestTimeTool_Stage(t *testing.T) {
	tool := NewTimeTool()
	if tool.Stage() != agent.ToolStagePostProcess {
		t.Error("Expected stage to be ToolStagePostProcess")
	}
}

func TestTimeTool_Execute(t *testing.T) {
	tool := NewTimeTool()
	result, err := tool.Execute(map[string]any{})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	content, ok := result.Result.(string)
	if !ok {
		t.Fatal("Expected result to be a string")
	}

	if !strings.Contains(content, "Current time:") {
		t.Error("Expected result to contain 'Current time:'")
	}

	if !strings.Contains(content, "Date:") {
		t.Error("Expected result to contain 'Date:'")
	}

	if !strings.Contains(content, "Unix timestamp:") {
		t.Error("Expected result to contain 'Unix timestamp:'")
	}
}
