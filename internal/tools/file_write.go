package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileWrite creates or overwrites a file with content
type FileWrite struct {
	manager *Manager
}

// NewFileWrite creates a new file_write tool
func NewFileWrite(manager *Manager) *FileWrite {
	return &FileWrite{manager: manager}
}

func (t *FileWrite) Name() string {
	return "file_write"
}

func (t *FileWrite) Description() string {
	return "Create a new file or overwrite an existing file with text content."
}

func (t *FileWrite) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Absolute or relative path to the file to write"
			},
			"content": {
				"type": "string",
				"description": "Content to write to the file"
			},
			"create_dirs": {
				"type": "boolean",
				"description": "Create parent directories if they don't exist (default: false)",
				"default": false
			}
		},
		"required": ["path", "content"]
	}`)
}

func (t *FileWrite) Execute(args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path parameter is required and must be a string")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content parameter is required and must be a string")
	}

	// Validate path is within allowed directories
	if err := t.manager.ValidatePath(path); err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	// Clean the path
	cleanPath := filepath.Clean(path)

	// Check for suspicious file extensions
	if err := validateFileExtension(cleanPath); err != nil {
		return "", err
	}

	// Check if parent directories need to be created
	createDirs := false
	if cd, ok := args["create_dirs"].(bool); ok {
		createDirs = cd
	}

	// Get parent directory
	parentDir := filepath.Dir(cleanPath)

	// Check if parent directory exists
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		if createDirs {
			// Create parent directories with secure permissions
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return "", fmt.Errorf("failed to create parent directories: %w", err)
			}
		} else {
			return "", fmt.Errorf("parent directory does not exist: %s (set create_dirs=true to create)", parentDir)
		}
	}

	// Write the file
	if err := os.WriteFile(cleanPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), cleanPath), nil
}

// validateFileExtension checks for dangerous file extensions
func validateFileExtension(path string) error {
	// List of dangerous extensions
	dangerousExts := []string{
		".exe", ".dll", ".so", ".dylib",
		".bat", ".cmd", ".sh", ".ps1",
		".scr", ".vbs", ".js", ".jar",
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, dangerous := range dangerousExts {
		if ext == dangerous {
			return fmt.Errorf("writing executable files is not allowed for security reasons: %s", path)
		}
	}

	return nil
}
