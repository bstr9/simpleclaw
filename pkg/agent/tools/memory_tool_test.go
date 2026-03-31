package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bstr9/simpleclaw/pkg/agent"
	"github.com/bstr9/simpleclaw/pkg/common"
)

func TestMemoryTool_Name(t *testing.T) {
	tool := NewMemoryTool("")
	if tool.Name() != "memory" {
		t.Errorf("Expected name 'memory', got '%s'", tool.Name())
	}
}

func TestMemoryTool_Description(t *testing.T) {
	tool := NewMemoryTool("")
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestMemoryTool_Parameters(t *testing.T) {
	tool := NewMemoryTool("")
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Error("Expected type to be 'object'")
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	if _, exists := props["action"]; !exists {
		t.Error("Expected 'action' property to exist")
	}
}

func TestMemoryTool_Stage(t *testing.T) {
	tool := NewMemoryTool("")
	if tool.Stage() != agent.ToolStagePostProcess {
		t.Error("Expected stage to be ToolStagePostProcess")
	}
}

func TestMemoryTool_Execute_MissingAction(t *testing.T) {
	tool := NewMemoryTool("")
	result, err := tool.Execute(map[string]any{})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing action")
	}
}

func TestMemoryTool_Execute_InvalidAction(t *testing.T) {
	tool := NewMemoryTool("")
	result, err := tool.Execute(map[string]any{"action": "invalid"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for invalid action")
	}
}

func TestMemoryTool_Execute_SearchMissingQuery(t *testing.T) {
	tool := NewMemoryTool("")
	result, err := tool.Execute(map[string]any{"action": "search"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing query")
	}
}

func TestMemoryTool_Execute_SearchNoMemory(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewMemoryTool(tmpDir)

	result, err := tool.Execute(map[string]any{
		"action": "search",
		"query":  "test",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}
}

func TestMemoryTool_Execute_SearchWithMemory(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)

	testFile := filepath.Join(memoryDir, "2024-01-01.md")
	if err := os.WriteFile(testFile, []byte("# Test Memory\nThis is a test memory about golang programming."), 0644); err != nil {
		t.Fatalf("Failed to create test memory file: %v", err)
	}

	tool := NewMemoryTool(tmpDir)
	result, err := tool.Execute(map[string]any{
		"action": "search",
		"query":  "golang",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap := result.Result.(map[string]any)
	results := resultMap["results"].([]map[string]any)

	if len(results) == 0 {
		t.Error("Expected to find search results")
	}
}

func TestMemoryTool_Execute_GetMissingPath(t *testing.T) {
	tool := NewMemoryTool("")
	result, err := tool.Execute(map[string]any{"action": "get"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing path")
	}
}

func TestMemoryTool_Execute_GetFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewMemoryTool(tmpDir)

	result, err := tool.Execute(map[string]any{
		"action": "get",
		"path":   "nonexistent.md",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for file not found")
	}
}

func TestMemoryTool_Execute_GetFile(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)

	testFile := filepath.Join(memoryDir, "test.md")
	testContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewMemoryTool(tmpDir)
	result, err := tool.Execute(map[string]any{
		"action": "get",
		"path":   "test.md",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap := result.Result.(map[string]any)
	content := resultMap["content"].(string)

	if !containsSubstring(content, "Line 1") {
		t.Error("Expected content to contain 'Line 1'")
	}
}

func TestMemoryTool_Execute_GetFileWithStartLine(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)

	testFile := filepath.Join(memoryDir, "offset.md")
	testContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewMemoryTool(tmpDir)
	result, err := tool.Execute(map[string]any{
		"action":     "get",
		"path":       "offset.md",
		"start_line": float64(3),
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap := result.Result.(map[string]any)
	content := resultMap["content"].(string)

	if containsSubstring(content, "Line 1") {
		t.Error("Expected content to not contain 'Line 1' when start_line is 3")
	}

	if !containsSubstring(content, "Line 3") {
		t.Error("Expected content to contain 'Line 3'")
	}
}

func TestMemoryTool_GetMemoryDir(t *testing.T) {
	tool := NewMemoryTool("/workspace")
	dir := tool.getMemoryDir()

	if dir != "/workspace/memory" {
		t.Errorf("Expected '/workspace/memory', got '%s'", dir)
	}
}

func TestMemoryTool_ResolvePath(t *testing.T) {
	tool := NewMemoryTool("/workspace")

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

func TestMemoryTool_SearchInMemory(t *testing.T) {
	tmpDir := t.TempDir()
	memoryDir := filepath.Join(tmpDir, "memory")
	os.MkdirAll(memoryDir, 0755)

	testFile := filepath.Join(memoryDir, "2024-01-15.md")
	if err := os.WriteFile(testFile, []byte("This is a test memory about testing"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	rootMemoryFile := filepath.Join(tmpDir, common.MemoryFileName)
	if err := os.WriteFile(rootMemoryFile, []byte("Root memory content about testing"), 0644); err != nil {
		t.Fatalf("Failed to create root memory file: %v", err)
	}

	tool := NewMemoryTool(tmpDir)
	results := tool.searchInMemory("testing", memoryDir, 10)

	if len(results) == 0 {
		t.Error("Expected to find search results")
	}
}

func TestMemoryTool_ExtractSnippet(t *testing.T) {
	tool := NewMemoryTool("")

	tests := []struct {
		name    string
		content string
		query   string
		maxLen  int
		wantLen int
	}{
		{"short content", "Hello World", "World", 100, 11},
		{"long content", "This is a very long content with the keyword test somewhere in the middle", "test", 20, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snippet := tool.extractSnippet(tt.content, tt.query, tt.maxLen)
			if len(snippet) == 0 {
				t.Error("Expected non-empty snippet")
			}
		})
	}
}
