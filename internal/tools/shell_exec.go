package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
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
	DefaultTimeout = 5 * time.Minute  // 5 minutes default - enough for most commands
	MaxTimeout     = 30 * time.Minute // 30 minutes max - allows npm install, gcc编译等
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

	// Trim whitespace
	commandStr = strings.TrimSpace(commandStr)
	if commandStr == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	// Parse the command to get the executable name
	parts := strings.Fields(commandStr)
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid command")
	}

	executable := parts[0]

	// Check if command is blacklisted
	if t.manager.IsCommandBlacklisted(executable) {
		return "", fmt.Errorf("command '%s' is not allowed for security reasons", executable)
	}

	// Get timeout from args or use default
	timeout := t.timeout
	if to, ok := args["timeout"].(float64); ok {
		customTimeout := time.Duration(to) * time.Second
		// Enforce min 10 seconds and max 30 minutes
		if customTimeout >= 10*time.Second && customTimeout <= MaxTimeout {
			timeout = customTimeout
		}
	}

	// Get working directory from args (optional)
	workingDir := ""
	if wd, ok := args["working_dir"].(string); ok {
		workingDir = strings.TrimSpace(wd)
	}

	// Execute based on OS
	startTime := time.Now()
	var output []byte
	var err error

	switch runtime.GOOS {
	case "windows":
		output, err = t.executeWindows(commandStr, workingDir, timeout)
	default:
		output, err = t.executeUnix(commandStr, workingDir, timeout)
	}

	duration := time.Since(startTime)

	// Build result
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

// executeUnix executes a command on Unix-like systems (Linux, macOS)
func (t *ShellExec) executeUnix(commandStr string, workingDir string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.CommandContext(ctx, "bash", "-c", commandStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", commandStr)
	}

	// Set working directory if specified
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	return cmd.CombinedOutput()
}

// executeWindows executes a command on Windows without showing a console window
func (t *ShellExec) executeWindows(commandStr string, workingDir string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Use cmd /c to run the command - this is the standard way on Windows
	// The sysprocattr below will hide any console window
	cmd := exec.CommandContext(ctx, "cmd", "/c", commandStr)

	// Set process attributes to hide the window on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	// Set working directory if specified
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// Try to run and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, err
	}

	return output, nil
}
