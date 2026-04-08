package tools

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	allowedDirs := []string{"/tmp", "/home/test"}
	blacklist := []string{"rm", "del"}

	manager := NewManager(allowedDirs, blacklist)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	// Register built-in tools
	RegisterBuiltInTools(manager)

	// Check that tools are registered
	tools := manager.GetAllTools()
	if len(tools) != 3 {
		t.Errorf("Expected 3 registered tools, got %d", len(tools))
	}

	// Check that built-in tools are registered
	toolNames := []string{}
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name())
	}

	expectedTools := []string{"file_read", "file_write", "shell_exec"}
	for _, expected := range expectedTools {
		found := false
		for _, name := range toolNames {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool %s not found", expected)
		}
	}
}

func TestGetBuiltInToolNames(t *testing.T) {
	names := GetBuiltInToolNames()

	if len(names) != 3 {
		t.Errorf("Expected 3 built-in tool names, got %d", len(names))
	}

	expected := []string{"file_read", "file_write", "shell_exec"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected tool name %s at index %d, got %s", expected[i], i, name)
		}
	}
}

func TestManagerIsCommandBlacklisted(t *testing.T) {
	blacklist := []string{"rm", "del", "format"}
	manager := NewManager([]string{}, blacklist)

	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{"blacklisted command", "rm", true},
		{"blacklisted command uppercase", "RM", true},
		{"safe command", "ls", false},
		{"safe command", "echo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.IsCommandBlacklisted(tt.command)
			if result != tt.expected {
				t.Errorf("IsCommandBlacklisted(%s) = %v, want %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestDefaultCommandBlacklist(t *testing.T) {
	blacklist := DefaultCommandBlacklist()

	if len(blacklist) == 0 {
		t.Error("DefaultCommandBlacklist returned empty list")
	}

	// Check for some expected dangerous commands
	dangerousCmds := map[string]bool{
		"rm":       false,
		"shutdown": false,
		"format":   false,
	}

	for _, cmd := range blacklist {
		if _, exists := dangerousCmds[cmd]; exists {
			dangerousCmds[cmd] = true
		}
	}

	for cmd, found := range dangerousCmds {
		if !found {
			t.Errorf("Expected dangerous command %s not in default blacklist", cmd)
		}
	}
}
