package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

func TestLsTool_Name(t *testing.T) {
	tool := NewLsTool("")
	if tool.Name() != "ls" {
		t.Errorf("Expected name 'ls', got '%s'", tool.Name())
	}
}

func TestLsTool_Description(t *testing.T) {
	tool := NewLsTool("")
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestLsTool_Parameters(t *testing.T) {
	tool := NewLsTool("")
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Error("Expected type to be 'object'")
	}
}

func TestLsTool_Stage(t *testing.T) {
	tool := NewLsTool("")
	if tool.Stage() != agent.ToolStagePostProcess {
		t.Error("Expected stage to be ToolStagePostProcess")
	}
}

func TestLsTool_Execute_ListCurrentDir(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tool := NewLsTool(tmpDir)
	result, err := tool.Execute(map[string]any{})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap := result.Result.(map[string]any)
	output := resultMap["output"].(string)

	if !containsSubstring(output, "file1.txt") {
		t.Error("Expected output to contain 'file1.txt'")
	}

	if !containsSubstring(output, "subdir/") {
		t.Error("Expected output to contain 'subdir/'")
	}
}

func TestLsTool_Execute_ListSpecificDir(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")

	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(subDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewLsTool(tmpDir)
	result, err := tool.Execute(map[string]any{"path": "subdir"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap := result.Result.(map[string]any)
	output := resultMap["output"].(string)

	if !containsSubstring(output, "file.txt") {
		t.Error("Expected output to contain 'file.txt'")
	}
}

func TestLsTool_Execute_DirNotFound(t *testing.T) {
	tool := NewLsTool("")
	result, err := tool.Execute(map[string]any{"path": "nonexistent_dir"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for non-existent directory")
	}
}

func TestLsTool_Execute_ListFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewLsTool(tmpDir)
	result, err := tool.Execute(map[string]any{"path": "test.txt"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result when listing a file (not directory)")
	}
}

func TestLsTool_Execute_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	tool := NewLsTool(tmpDir)
	result, err := tool.Execute(map[string]any{})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap := result.Result.(map[string]any)
	entries := resultMap["entries"].([]string)

	if len(entries) != 0 {
		t.Errorf("Expected empty entries, got %d", len(entries))
	}
}
