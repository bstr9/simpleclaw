package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

func TestWriteTool_Name(t *testing.T) {
	tool := NewWriteTool("")
	if tool.Name() != "write" {
		t.Errorf("Expected name 'write', got '%s'", tool.Name())
	}
}

func TestWriteTool_Description(t *testing.T) {
	tool := NewWriteTool("")
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestWriteTool_Parameters(t *testing.T) {
	tool := NewWriteTool("")
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Error("Expected type to be 'object'")
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	for _, field := range []string{"path", "content"} {
		if _, exists := props[field]; !exists {
			t.Errorf("Expected property '%s' to exist", field)
		}
	}
}

func TestWriteTool_Stage(t *testing.T) {
	tool := NewWriteTool("")
	if tool.Stage() != agent.ToolStagePostProcess {
		t.Error("Expected stage to be ToolStagePostProcess")
	}
}

func TestWriteTool_Execute_MissingPath(t *testing.T) {
	tool := NewWriteTool("")
	result, err := tool.Execute(map[string]any{"content": "test"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing path")
	}
}

func TestWriteTool_Execute_CreateEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteTool(tmpDir)

	result, err := tool.Execute(map[string]any{
		"path":    "empty_file.txt",
		"content": "",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	filePath := filepath.Join(tmpDir, "empty_file.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}

	if string(content) != "" {
		t.Errorf("File content = %s, want empty", string(content))
	}
}

func TestWriteTool_Execute_CreateFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteTool(tmpDir)

	result, err := tool.Execute(map[string]any{
		"path":    "new_file.txt",
		"content": "Hello, World!",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	filePath := filepath.Join(tmpDir, "new_file.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}

	if string(content) != "Hello, World!" {
		t.Errorf("File content = %s, want 'Hello, World!'", string(content))
	}
}

func TestWriteTool_Execute_OverwriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "overwrite.txt")

	if err := os.WriteFile(testFile, []byte("original content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewWriteTool(tmpDir)
	result, err := tool.Execute(map[string]any{
		"path":    "overwrite.txt",
		"content": "new content",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != "new content" {
		t.Errorf("File content = %s, want 'new content'", string(content))
	}
}

func TestWriteTool_Execute_CreateNestedFile(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteTool(tmpDir)

	result, err := tool.Execute(map[string]any{
		"path":    "subdir/nested/file.txt",
		"content": "nested content",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	filePath := filepath.Join(tmpDir, "subdir", "nested", "file.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}

	if string(content) != "nested content" {
		t.Errorf("File content = %s, want 'nested content'", string(content))
	}
}
