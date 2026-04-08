package tools

import (
	"strings"
	"testing"
)

func TestShellExecName(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewShellExec(manager)

	if tool.Name() != "shell_exec" {
		t.Errorf("Expected name 'shell_exec', got '%s'", tool.Name())
	}
}

func TestShellExecDescription(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewShellExec(manager)

	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestShellExecParameters(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewShellExec(manager)

	params := tool.Parameters()
	if len(params) == 0 {
		t.Error("Parameters should not be empty")
	}
}

func TestShellExecExecute(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewShellExec(manager)

	tests := []struct {
		name       string
		command    string
		wantErr    bool
		wantOutput string
	}{
		{
			name:       "echo command",
			command:    "echo hello",
			wantErr:    false,
			wantOutput: "hello",
		},
		{
			name:       "echo with spaces",
			command:    "echo hello world",
			wantErr:    false,
			wantOutput: "hello world",
		},
		{
			name:    "pwd command",
			command: "pwd",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(map[string]any{
				"command": tt.command,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantOutput != "" && !strings.Contains(result, tt.wantOutput) {
				t.Errorf("Execute() result = %v, want output containing %v", result, tt.wantOutput)
			}
		})
	}
}

func TestShellExecExecuteErrors(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewShellExec(manager)

	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{
			name:    "missing command parameter",
			args:    map[string]any{},
			wantErr: true,
		},
		{
			name: "invalid command type",
			args: map[string]any{
				"command": 123,
			},
			wantErr: true,
		},
		{
			name: "empty command",
			args: map[string]any{
				"command": "",
			},
			wantErr: true,
		},
		{
			name: "whitespace only command",
			args: map[string]any{
				"command": "   ",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShellExecCommandBlacklist(t *testing.T) {
	blacklist := []string{"rm", "del", "format", "shutdown"}
	manager := NewManager([]string{}, blacklist)
	tool := NewShellExec(manager)

	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{
			name:    "blacklisted command rm",
			command: "rm -rf /tmp/test",
			wantErr: true,
		},
		{
			name:    "blacklisted command del",
			command: "del test.txt",
			wantErr: true,
		},
		{
			name:    "blacklisted command format",
			command: "format c:",
			wantErr: true,
		},
		{
			name:    "blacklisted command shutdown",
			command: "shutdown now",
			wantErr: true,
		},
		{
			name:    "safe command echo",
			command: "echo test",
			wantErr: false,
		},
		{
			name:    "safe command ls",
			command: "ls",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(map[string]any{
				"command": tt.command,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShellExecCommandBlacklistCaseInsensitive(t *testing.T) {
	blacklist := []string{"rm"}
	manager := NewManager([]string{}, blacklist)
	tool := NewShellExec(manager)

	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{
			name:    "lowercase rm",
			command: "rm test",
			wantErr: true,
		},
		{
			name:    "uppercase RM",
			command: "RM test",
			wantErr: true,
		},
		{
			name:    "mixed case Rm",
			command: "Rm test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(map[string]any{
				"command": tt.command,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShellExecCustomTimeout(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewShellExec(manager)

	// Test with custom timeout (1 second)
	result, err := tool.Execute(map[string]any{
		"command": "echo quick",
		"timeout": float64(1),
	})
	if err != nil {
		t.Errorf("Execute with custom timeout failed: %v", err)
	}
	if !strings.Contains(result, "quick") {
		t.Errorf("Expected output containing 'quick', got '%s'", result)
	}
}

func TestShellExecTimeoutBounds(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewShellExec(manager)

	// Test with timeout exceeding max (60 seconds)
	result, err := tool.Execute(map[string]any{
		"command": "echo test",
		"timeout": float64(120), // Exceeds max
	})
	if err != nil {
		t.Errorf("Execute with excessive timeout failed: %v", err)
	}
	// Should still work (timeout capped at 60)
	if !strings.Contains(result, "test") {
		t.Errorf("Expected output containing 'test', got '%s'", result)
	}

	// Test with timeout below min (0 seconds)
	result, err = tool.Execute(map[string]any{
		"command": "echo test2",
		"timeout": float64(0), // Below min
	})
	if err != nil {
		t.Errorf("Execute with zero timeout failed: %v", err)
	}
	// Should still work (timeout defaults to 30)
	if !strings.Contains(result, "test2") {
		t.Errorf("Expected output containing 'test2', got '%s'", result)
	}
}
