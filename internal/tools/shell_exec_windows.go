//go:build windows

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ShellExec executes a shell command with timeout
type ShellExec struct {
	manager *Manager
	timeout time.Duration
}

// Default and max timeouts
const (
	DefaultTimeout = 5 * time.Minute
	MaxTimeout     = 30 * time.Minute
)

// NewShellExec creates a new shell_exec tool
func NewShellExec(manager *Manager) *ShellExec {
	return &ShellExec{
		manager: manager,
		timeout: DefaultTimeout,
	}
}

func (t *ShellExec) Name() string {
	return "shell_exec"
}

func (t *ShellExec) Description() string {
	return "Execute a shell command. Returns stdout and stderr. Commands are executed with a timeout. Default: 5 minutes, Max: 30 minutes."
}

func (t *ShellExec) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "Shell command to execute (e.g., 'ls -la', 'npm install', 'gcc main.c -o app')"
			},
			"working_dir": {
				"type": "string",
				"description": "Working directory for the command execution (optional). If not specified, uses the current working directory."
			},
			"timeout": {
				"type": "integer",
				"description": "Timeout in seconds (default: 300, max: 1800). Use higher values for long-running commands like npm install, gcc, etc.",
				"default": 300,
				"minimum": 10,
				"maximum": 1800
			}
		},
		"required": ["command"]
	}`)
}

func (t *ShellExec) Execute(args map[string]any) (string, error) {
	commandStr, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command parameter is required and must be a string")
	}

	commandStr = strings.TrimSpace(commandStr)
	if commandStr == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	parts := strings.Fields(commandStr)
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid command")
	}

	executable := parts[0]

	if t.manager.IsCommandBlacklisted(executable) {
		return "", fmt.Errorf("command '%s' is not allowed for security reasons", executable)
	}

	timeout := t.timeout
	if to, ok := args["timeout"].(float64); ok {
		customTimeout := time.Duration(to) * time.Second
		if customTimeout >= 10*time.Second && customTimeout <= MaxTimeout {
			timeout = customTimeout
		}
	}

	workingDir := ""
	if wd, ok := args["working_dir"].(string); ok {
		workingDir = strings.TrimSpace(wd)
	}

	startTime := time.Now()
	output, err := t.executeWindows(commandStr, workingDir, timeout)
	duration := time.Since(startTime)

	result := fmt.Sprintf("Command: %s\nDuration: %v\nExit code: ", commandStr, duration.Round(time.Millisecond))

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result += fmt.Sprintf("%d\n", exitError.ExitCode())
		} else {
			result += fmt.Sprintf("Error: %v\n", err)
		}
	} else {
		result += "0\n"
	}

	result += fmt.Sprintf("\nOutput:\n%s", string(output))

	return result, nil
}

// executeWindows executes a command on Windows without showing a console window
func (t *ShellExec) executeWindows(commandStr string, workingDir string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cmd", "/c", commandStr)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	if workingDir != "" {
		cmd.Dir = workingDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, err
	}

	return output, nil
}
