package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

func TestEditTool_Name(t *testing.T) {
	tool := NewEditTool("")
	if tool.Name() != "edit" {
		t.Errorf("Expected name 'edit', got '%s'", tool.Name())
	}
}

func TestEditTool_Description(t *testing.T) {
	tool := NewEditTool("")
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestEditTool_Parameters(t *testing.T) {
	tool := NewEditTool("")
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Error("Expected type to be 'object'")
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	for _, field := range []string{"path", "oldText", "newText"} {
		if _, exists := props[field]; !exists {
			t.Errorf("Expected property '%s' to exist", field)
		}
	}
}

func TestEditTool_Stage(t *testing.T) {
	tool := NewEditTool("")
	if tool.Stage() != agent.ToolStagePostProcess {
		t.Error("Expected stage to be ToolStagePostProcess")
	}
}

func TestEditTool_Execute_MissingPath(t *testing.T) {
	tool := NewEditTool("")
	result, err := tool.Execute(map[string]any{"oldText": "old", "newText": "new"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing path")
	}
}

func TestEditTool_Execute_FileNotFound(t *testing.T) {
	tool := NewEditTool("")
	result, err := tool.Execute(map[string]any{
		"path":    "nonexistent.txt",
		"oldText": "old",
		"newText": "new",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for file not found")
	}
}

func TestEditTool_Execute_ReplaceText(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "replace.txt")

	if err := os.WriteFile(testFile, []byte("Hello World"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewEditTool(tmpDir)
	result, err := tool.Execute(map[string]any{
		"path":    "replace.txt",
		"oldText": "World",
		"newText": "Go",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	content, _ := os.ReadFile(testFile)
	if string(content) != "Hello Go" {
		t.Errorf("File content = %s, want 'Hello Go'", string(content))
	}
}

func TestEditTool_Execute_AppendText(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "append.txt")

	if err := os.WriteFile(testFile, []byte("Line 1"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewEditTool(tmpDir)
	result, err := tool.Execute(map[string]any{
		"path":    "append.txt",
		"oldText": "",
		"newText": "Line 2",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	content, _ := os.ReadFile(testFile)
	expected := "Line 1\nLine 2"
	if string(content) != expected {
		t.Errorf("File content = %s, want '%s'", string(content), expected)
	}
}

func TestEditTool_Execute_TextNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "notfound.txt")

	if err := os.WriteFile(testFile, []byte("Hello World"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewEditTool(tmpDir)
	result, err := tool.Execute(map[string]any{
		"path":    "notfound.txt",
		"oldText": "NotExist",
		"newText": "New",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for text not found")
	}
}

func TestEditTool_Execute_MultipleMatches(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "multiple.txt")

	if err := os.WriteFile(testFile, []byte("test test test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewEditTool(tmpDir)
	result, err := tool.Execute(map[string]any{
		"path":    "multiple.txt",
		"oldText": "test",
		"newText": "new",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for multiple matches")
	}
}

func TestEditTool_Execute_NoChange(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "nochange.txt")

	if err := os.WriteFile(testFile, []byte("Hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewEditTool(tmpDir)
	result, err := tool.Execute(map[string]any{
		"path":    "nochange.txt",
		"oldText": "Hello",
		"newText": "Hello",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result when no changes made")
	}
}
