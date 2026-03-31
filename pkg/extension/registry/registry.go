// Package registry 提供扩展组件的全局注册表。
package registry

import (
	"sync"

	"github.com/bstr9/simpleclaw/pkg/agent"
)

var extensionTools = struct {
	mu    sync.RWMutex
	tools []agent.Tool
}{
	tools: make([]agent.Tool, 0),
}

var skillPaths = struct {
	mu    sync.RWMutex
	paths []string
}{
	paths: make([]string, 0),
}

func RegisterTool(tool agent.Tool) {
	extensionTools.mu.Lock()
	extensionTools.tools = append(extensionTools.tools, tool)
	extensionTools.mu.Unlock()
}

func GetTools() []agent.Tool {
	extensionTools.mu.RLock()
	defer extensionTools.mu.RUnlock()

	tools := make([]agent.Tool, len(extensionTools.tools))
	copy(tools, extensionTools.tools)
	return tools
}

func RegisterSkillPath(path string) {
	skillPaths.mu.Lock()
	skillPaths.paths = append(skillPaths.paths, path)
	skillPaths.mu.Unlock()
}

func GetSkillPaths() []string {
	skillPaths.mu.RLock()
	defer skillPaths.mu.RUnlock()

	paths := make([]string, len(skillPaths.paths))
	copy(paths, skillPaths.paths)
	return paths
}
