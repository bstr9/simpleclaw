package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

func TestReadTool_Name(t *testing.T) {
	tool := NewReadTool("")
	if tool.Name() != "read" {
		t.Errorf("Expected name 'read', got '%s'", tool.Name())
	}
}

func TestReadTool_Description(t *testing.T) {
	tool := NewReadTool("")
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestReadTool_Parameters(t *testing.T) {
	tool := NewReadTool("")
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Error("Expected type to be 'object'")
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	if _, exists := props["path"]; !exists {
		t.Error("Expected 'path' property to exist")
	}

	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("Expected required to be a string slice")
	}

	if len(required) != 1 || required[0] != "path" {
		t.Error("Expected 'path' to be required")
	}
}

func TestReadTool_Stage(t *testing.T) {
	tool := NewReadTool("")
	if tool.Stage() != agent.ToolStagePostProcess {
		t.Error("Expected stage to be ToolStagePostProcess")
	}
}

func TestReadTool_Execute_MissingPath(t *testing.T) {
	tool := NewReadTool("")
	result, err := tool.Execute(map[string]any{})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing path")
	}
}

func TestReadTool_Execute_FileNotFound(t *testing.T) {
	tool := NewReadTool("")
	result, err := tool.Execute(map[string]any{"path": "nonexistent_file.txt"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for file not found")
	}
}

func TestReadTool_Execute_ReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!\nLine 2\nLine 3"

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadTool(tmpDir)
	result, err := tool.Execute(map[string]any{"path": "test.txt"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap, ok := result.Result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	content, ok := resultMap["content"].(string)
	if !ok {
		t.Fatal("Expected content to be a string")
	}

	if !containsString(content, "Hello, World!") {
		t.Error("Expected content to contain 'Hello, World!'")
	}
}

func TestReadTool_Execute_ReadWithOffset(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "offset_test.txt")
	testContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadTool(tmpDir)
	result, err := tool.Execute(map[string]any{
		"path":   "offset_test.txt",
		"offset": float64(2),
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap := result.Result.(map[string]any)
	content := resultMap["content"].(string)

	if containsString(content, "Line 1") {
		t.Error("Expected content to not contain 'Line 1' when offset is 2")
	}

	if !containsString(content, "Line 2") {
		t.Error("Expected content to contain 'Line 2'")
	}
}

func TestReadTool_Execute_ReadWithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "limit_test.txt")
	testContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"

	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadTool(tmpDir)
	result, err := tool.Execute(map[string]any{
		"path":  "limit_test.txt",
		"limit": float64(2),
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap := result.Result.(map[string]any)
	content := resultMap["content"].(string)

	if !containsString(content, "Line 1") {
		t.Error("Expected content to contain 'Line 1'")
	}

	if containsString(content, "Line 3") {
		t.Error("Expected content to not contain 'Line 3' when limit is 2")
	}
}

func TestReadTool_Execute_ReadDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	tool := NewReadTool(tmpDir)
	result, err := tool.Execute(map[string]any{"path": "."})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result when reading a directory")
	}
}

func TestReadTool_ResolvePath(t *testing.T) {
	tool := NewReadTool("/workspace")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"relative path", "file.txt", "/workspace/file.txt"},
		{"absolute path", "/tmp/file.txt", "/tmp/file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.resolvePath(tt.input)
			if result != tt.expected {
				t.Errorf("resolvePath(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
