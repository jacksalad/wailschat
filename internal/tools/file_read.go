package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileRead reads the contents of a local file
type FileRead struct {
	manager *Manager
}

// NewFileRead creates a new file_read tool
func NewFileRead(manager *Manager) *FileRead {
	return &FileRead{manager: manager}
}

func (t *FileRead) Name() string {
	return "file_read"
}

func (t *FileRead) Description() string {
	return "Read the contents of a local file, or list the contents of a directory. For files, returns the file text. For directories, returns a listing of all immediate files and subdirectories (non-recursive)."
}

func (t *FileRead) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Absolute or relative path to the file or directory to read"
			},
			"max_size": {
				"type": "integer",
				"description": "Maximum file size to read in bytes (default: 102400, max: 1048576)",
				"default": 102400
			}
		},
		"required": ["path"]
	}`)
}

const (
	defaultMaxFileSize = 100 * 1024  // 100KB
	hardMaxFileSize    = 1024 * 1024 // 1MB
)

func (t *FileRead) Execute(args map[string]any) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path parameter is required and must be a string")
	}

	// Validate path is within allowed directories
	if err := t.manager.ValidatePath(path); err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}

	// Clean the path
	cleanPath := filepath.Clean(path)

	// Get file info to check size
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", cleanPath)
		}
		return "", fmt.Errorf("failed to access file: %w", err)
	}

	// If it's a directory, list its contents
	if fileInfo.IsDir() {
		return t.listDirectory(cleanPath)
	}

	// Get max size from args or use default
	maxSize := defaultMaxFileSize
	if mz, ok := args["max_size"].(float64); ok {
		maxSize = int(mz)
	}

	// Enforce hard limit
	if maxSize > hardMaxFileSize {
		maxSize = hardMaxFileSize
	}

	// Check file size
	if fileInfo.Size() > int64(maxSize) {
		return "", fmt.Errorf("file too large: %d bytes (max: %d bytes)", fileInfo.Size(), maxSize)
	}

	// Read file contents
	contents, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(contents), nil
}

// listDirectory returns a formatted listing of all immediate children in a directory
func (t *FileRead) listDirectory(dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Directory listing: %s\n", dirPath)
	fmt.Fprintf(&sb, "Total entries: %d\n\n", len(entries))

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		typeTag := "[FILE]"
		if entry.IsDir() {
			typeTag = "[DIR] "
		}
		fmt.Fprintf(&sb, "%s %-40s %10d  %s\n",
			typeTag,
			entry.Name(),
			info.Size(),
			info.ModTime().Format("2006-01-02 15:04:05"),
		)
	}

	return sb.String(), nil
}
