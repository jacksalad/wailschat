package tools

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// Tool defines the interface for built-in tools
type Tool interface {
	// Name returns the unique identifier for this tool
	Name() string
	// Description returns a human-readable description of what this tool does
	Description() string
	// Parameters returns a JSON Schema object describing the tool's parameters
	Parameters() json.RawMessage
	// Execute runs the tool with the provided arguments and returns the result
	Execute(args map[string]any) (string, error)
}

// Manager manages available tools and their execution
type Manager struct {
	allowedDirs  []string
	cmdBlacklist []string
	mu           sync.RWMutex
	tools        map[string]Tool
}

// NewManager creates a new tool manager with security constraints
func NewManager(allowedDirs []string, cmdBlacklist []string) *Manager {
	// Normalize allowed directories
	cleaned := make([]string, 0, len(allowedDirs))
	for _, dir := range allowedDirs {
		abs, err := filepath.Abs(dir)
		if err == nil {
			cleaned = append(cleaned, abs)
		}
	}

	// Default blacklist if none provided
	blacklist := cmdBlacklist
	if len(blacklist) == 0 {
		blacklist = DefaultCommandBlacklist()
	}

	return &Manager{
		allowedDirs:  cleaned,
		cmdBlacklist: blacklist,
		tools:        make(map[string]Tool),
	}
}

// Register registers a tool with the manager
func (m *Manager) Register(tool Tool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools[tool.Name()] = tool
}

// GetTool returns a tool by name, or nil if not found
func (m *Manager) GetTool(name string) Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tools[name]
}

// GetAllTools returns all registered tools
func (m *Manager) GetAllTools() []Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tools := make([]Tool, 0, len(m.tools))
	for _, t := range m.tools {
		tools = append(tools, t)
	}
	return tools
}

// ExecuteTool executes a tool by name with the given arguments
func (m *Manager) ExecuteTool(name string, args map[string]any) (string, error) {
	m.mu.RLock()
	tool, exists := m.tools[name]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("tool not found: %s", name)
	}

	return tool.Execute(args)
}

// ValidatePath checks if a path is within the allowed directories
func (m *Manager) ValidatePath(path string) error {
	if len(m.allowedDirs) == 0 {
		return nil // No restrictions if no allowed dirs configured
	}

	// Clean the path and make it absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check for path traversal attempts
	if strings.Contains(absPath, "..") {
		// The path was cleaned, so ".." should be resolved
		// But double-check by comparing with the cleaned version
		cleaned := filepath.Clean(path)
		if strings.Contains(cleaned, "..") {
			return fmt.Errorf("path traversal detected: %s", path)
		}
	}

	// Check if path is within any allowed directory
	for _, allowedDir := range m.allowedDirs {
		if strings.HasPrefix(absPath, allowedDir+string(filepath.Separator)) || absPath == allowedDir {
			return nil
		}
	}

	return fmt.Errorf("path outside allowed directories: %s (allowed: %v)", path, m.allowedDirs)
}

// IsCommandBlacklisted checks if a command is in the blacklist
func (m *Manager) IsCommandBlacklisted(cmd string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, blocked := range m.cmdBlacklist {
		if strings.EqualFold(cmd, blocked) {
			return true
		}
	}
	return false
}

// DefaultCommandBlacklist returns a list of dangerous commands
func DefaultCommandBlacklist() []string {
	return []string{
		"rm", "rmdir", "del", "delete",
		"format", "fdisk", "mkfs",
		"shutdown", "reboot", "poweroff",
		"su", "sudo",
		"chmod", "chown",
		"dd",
		"kill", "killall",
	}
}

// tools stores registered tools by name
var tools = make(map[string]Tool)

// RegisterBuiltInTools registers all built-in tools with the manager.
// responder is used by provide_selection to communicate with the frontend.
func RegisterBuiltInTools(manager *Manager, responder SelectionResponder) {
	manager.Register(NewFileRead(manager))
	manager.Register(NewFileWrite(manager))
	manager.Register(NewShellExec(manager))
	manager.Register(NewProvideSelection(responder))
}

// GetBuiltInToolNames returns the names of all built-in tools
func GetBuiltInToolNames() []string {
	return []string{"file_read", "file_write", "shell_exec", "provide_selection"}
}
