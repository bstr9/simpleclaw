package tools

import (
	"path/filepath"
	"testing"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

func TestEnvConfigTool_Name(t *testing.T) {
	tool := NewEnvConfigTool()
	if tool.Name() != "env_config" {
		t.Errorf("Expected name 'env_config', got '%s'", tool.Name())
	}
}

func TestEnvConfigTool_Description(t *testing.T) {
	tool := NewEnvConfigTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestEnvConfigTool_Parameters(t *testing.T) {
	tool := NewEnvConfigTool()
	params := tool.Parameters()

	if params["type"] != "object" {
		t.Error("Expected type to be 'object'")
	}

	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be a map")
	}

	for _, field := range []string{"action", "key", "value"} {
		if _, exists := props[field]; !exists {
			t.Errorf("Expected property '%s' to exist", field)
		}
	}
}

func TestEnvConfigTool_Stage(t *testing.T) {
	tool := NewEnvConfigTool()
	if tool.Stage() != agent.ToolStagePostProcess {
		t.Error("Expected stage to be ToolStagePostProcess")
	}
}

func TestEnvConfigTool_Execute_MissingAction(t *testing.T) {
	tool := NewEnvConfigTool()
	result, err := tool.Execute(map[string]any{})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing action")
	}
}

func TestEnvConfigTool_Execute_InvalidAction(t *testing.T) {
	tool := NewEnvConfigTool()
	result, err := tool.Execute(map[string]any{"action": "invalid"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for invalid action")
	}
}

func TestEnvConfigTool_Execute_Set(t *testing.T) {
	tool := NewEnvConfigTool()

	tmpDir := t.TempDir()
	tool.envDir = tmpDir
	tool.envPath = filepath.Join(tmpDir, ".env")

	result, err := tool.Execute(map[string]any{
		"action": "set",
		"key":    "TEST_KEY",
		"value":  "test_value_12345",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	envVars := tool.readEnvFile()
	if envVars["TEST_KEY"] != "test_value_12345" {
		t.Errorf("Expected TEST_KEY to be set, got '%s'", envVars["TEST_KEY"])
	}
}

func TestEnvConfigTool_Execute_SetMissingKey(t *testing.T) {
	tool := NewEnvConfigTool()
	result, err := tool.Execute(map[string]any{
		"action": "set",
		"value":  "test",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing key")
	}
}

func TestEnvConfigTool_Execute_SetMissingValue(t *testing.T) {
	tool := NewEnvConfigTool()
	result, err := tool.Execute(map[string]any{
		"action": "set",
		"key":    "TEST_KEY",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing value")
	}
}

func TestEnvConfigTool_Execute_Get(t *testing.T) {
	tool := NewEnvConfigTool()

	tmpDir := t.TempDir()
	tool.envDir = tmpDir
	tool.envPath = filepath.Join(tmpDir, ".env")

	tool.Execute(map[string]any{
		"action": "set",
		"key":    "GET_TEST_KEY",
		"value":  "get_test_value",
	})

	result, err := tool.Execute(map[string]any{
		"action": "get",
		"key":    "GET_TEST_KEY",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap := result.Result.(map[string]any)
	if !resultMap["exists"].(bool) {
		t.Error("Expected exists to be true")
	}
}

func TestEnvConfigTool_Execute_GetMissingKey(t *testing.T) {
	tool := NewEnvConfigTool()
	result, err := tool.Execute(map[string]any{"action": "get"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing key")
	}
}

func TestEnvConfigTool_Execute_GetNonExistent(t *testing.T) {
	tool := NewEnvConfigTool()

	tmpDir := t.TempDir()
	tool.envDir = tmpDir
	tool.envPath = filepath.Join(tmpDir, ".env")

	result, err := tool.Execute(map[string]any{
		"action": "get",
		"key":    "NON_EXISTENT_KEY_12345",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap := result.Result.(map[string]any)
	if resultMap["exists"].(bool) {
		t.Error("Expected exists to be false")
	}
}

func TestEnvConfigTool_Execute_List(t *testing.T) {
	tool := NewEnvConfigTool()

	tmpDir := t.TempDir()
	tool.envDir = tmpDir
	tool.envPath = filepath.Join(tmpDir, ".env")

	tool.Execute(map[string]any{
		"action": "set",
		"key":    "LIST_KEY_1",
		"value":  "value1",
	})
	tool.Execute(map[string]any{
		"action": "set",
		"key":    "LIST_KEY_2",
		"value":  "value2",
	})

	result, err := tool.Execute(map[string]any{"action": "list"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	resultMap := result.Result.(map[string]any)
	vars := resultMap["variables"].(map[string]any)

	if len(vars) < 2 {
		t.Errorf("Expected at least 2 variables, got %d", len(vars))
	}
}

func TestEnvConfigTool_Execute_ListEmpty(t *testing.T) {
	tool := NewEnvConfigTool()

	tmpDir := t.TempDir()
	tool.envDir = tmpDir
	tool.envPath = filepath.Join(tmpDir, ".env")

	result, err := tool.Execute(map[string]any{"action": "list"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}
}

func TestEnvConfigTool_Execute_Delete(t *testing.T) {
	tool := NewEnvConfigTool()

	tmpDir := t.TempDir()
	tool.envDir = tmpDir
	tool.envPath = filepath.Join(tmpDir, ".env")

	tool.Execute(map[string]any{
		"action": "set",
		"key":    "DELETE_KEY",
		"value":  "delete_value",
	})

	result, err := tool.Execute(map[string]any{
		"action": "delete",
		"key":    "DELETE_KEY",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}

	envVars := tool.readEnvFile()
	if _, exists := envVars["DELETE_KEY"]; exists {
		t.Error("Expected DELETE_KEY to be deleted")
	}
}

func TestEnvConfigTool_Execute_DeleteMissingKey(t *testing.T) {
	tool := NewEnvConfigTool()
	result, err := tool.Execute(map[string]any{"action": "delete"})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if result.IsSuccess() {
		t.Error("Expected error result for missing key")
	}
}

func TestEnvConfigTool_Execute_DeleteNonExistent(t *testing.T) {
	tool := NewEnvConfigTool()

	tmpDir := t.TempDir()
	tool.envDir = tmpDir
	tool.envPath = filepath.Join(tmpDir, ".env")

	result, err := tool.Execute(map[string]any{
		"action": "delete",
		"key":    "NON_EXISTENT_DELETE_KEY",
	})

	if err != nil {
		t.Errorf("Execute should not return error: %v", err)
	}

	if !result.IsSuccess() {
		t.Errorf("Expected success result, got: %v", result)
	}
}

func TestEnvConfigTool_MaskValue(t *testing.T) {
	tool := NewEnvConfigTool()

	tests := []struct {
		value    string
		expected string
	}{
		{"short", "***"},
		{"12345678901", "123456***8901"},
		{"very_long_api_key_12345", "very_l***2345"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			masked := tool.maskValue(tt.value)
			if masked != tt.expected {
				t.Errorf("maskValue(%s) = %s, want %s", tt.value, masked, tt.expected)
			}
		})
	}
}

func TestEnvConfigTool_ReadWriteEnvFile(t *testing.T) {
	tool := NewEnvConfigTool()

	tmpDir := t.TempDir()
	tool.envDir = tmpDir
	tool.envPath = filepath.Join(tmpDir, ".env")

	envVars := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	}

	tool.writeEnvFile(envVars)

	readVars := tool.readEnvFile()

	if readVars["KEY1"] != "value1" {
		t.Errorf("Expected KEY1=value1, got %s", readVars["KEY1"])
	}

	if readVars["KEY2"] != "value2" {
		t.Errorf("Expected KEY2=value2, got %s", readVars["KEY2"])
	}
}
