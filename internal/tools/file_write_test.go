package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileWriteName(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewFileWrite(manager)

	if tool.Name() != "file_write" {
		t.Errorf("Expected name 'file_write', got '%s'", tool.Name())
	}
}

func TestFileWriteDescription(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewFileWrite(manager)

	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestFileWriteParameters(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewFileWrite(manager)

	params := tool.Parameters()
	if len(params) == 0 {
		t.Error("Parameters should not be empty")
	}
}

func TestFileWriteExecute(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager([]string{tmpDir}, []string{})
	tool := NewFileWrite(manager)

	// Test successful write
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"

	result, err := tool.Execute(map[string]any{
		"path":    testFile,
		"content": testContent,
	})
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "Successfully wrote") {
		t.Errorf("Expected success message, got '%s'", result)
	}

	// Verify file was created
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("Failed to read written file: %v", err)
	}
	if string(content) != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, string(content))
	}
}

func TestFileWriteExecuteOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager([]string{tmpDir}, []string{})
	tool := NewFileWrite(manager)

	// Create initial file
	testFile := filepath.Join(tmpDir, "test.txt")
	initialContent := "Initial content"
	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create initial file: %v", err)
	}

	// Overwrite with new content
	newContent := "New content"
	_, err = tool.Execute(map[string]any{
		"path":    testFile,
		"content": newContent,
	})
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	// Verify file was overwritten
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("Failed to read written file: %v", err)
	}
	if string(content) != newContent {
		t.Errorf("Expected content '%s', got '%s'", newContent, string(content))
	}
}

func TestFileWriteExecuteCreateDirs(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager([]string{tmpDir}, []string{})
	tool := NewFileWrite(manager)

	// Test with create_dirs=true
	nestedFile := filepath.Join(tmpDir, "subdir", "nested.txt")
	testContent := "Nested content"

	_, err := tool.Execute(map[string]any{
		"path":        nestedFile,
		"content":     testContent,
		"create_dirs": true,
	})
	if err != nil {
		t.Errorf("Execute with create_dirs failed: %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(nestedFile)
	if err != nil {
		t.Errorf("Failed to read nested file: %v", err)
	}
	if string(content) != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, string(content))
	}
}

func TestFileWriteExecuteErrors(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager([]string{tmpDir}, []string{})
	tool := NewFileWrite(manager)

	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{
			name: "missing path parameter",
			args: map[string]any{
				"content": "test",
			},
			wantErr: true,
		},
		{
			name: "missing content parameter",
			args: map[string]any{
				"path": "/tmp/test.txt",
			},
			wantErr: true,
		},
		{
			name: "invalid path type",
			args: map[string]any{
				"path":    123,
				"content": "test",
			},
			wantErr: true,
		},
		{
			name: "invalid content type",
			args: map[string]any{
				"path":    "/tmp/test.txt",
				"content": 123,
			},
			wantErr: true,
		},
		{
			name: "nonexistent parent directory without create_dirs",
			args: map[string]any{
				"path":        filepath.Join(tmpDir, "nonexistent", "file.txt"),
				"content":     "test",
				"create_dirs": false,
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

func TestFileWriteExecutePathValidation(t *testing.T) {
	// Create a restricted directory
	restrictedDir := t.TempDir()
	manager := NewManager([]string{restrictedDir}, []string{})
	tool := NewFileWrite(manager)

	// Test path validation - should fail for file outside allowed directory
	otherDir := "/tmp/other-dir-test"
	outsideFile := filepath.Join(otherDir, "outside.txt")
	_, err := tool.Execute(map[string]any{
		"path":    outsideFile,
		"content": "test",
	})
	if err == nil {
		t.Error("Expected error for file outside allowed directory")
	}

	// Test path validation - should succeed for file inside allowed directory
	insideFile := filepath.Join(restrictedDir, "inside.txt")
	result, err := tool.Execute(map[string]any{
		"path":    insideFile,
		"content": "inside content",
	})
	if err != nil {
		t.Errorf("Execute failed for allowed file: %v", err)
	}
	if !strings.Contains(result, "Successfully wrote") {
		t.Errorf("Expected success message, got '%s'", result)
	}
}

func TestFileWriteExecuteDangerousExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager([]string{tmpDir}, []string{})
	tool := NewFileWrite(manager)

	dangerousExts := []string{".exe", ".sh", ".bat", ".dll", ".so"}

	for _, ext := range dangerousExts {
		t.Run("dangerous_extension_"+ext, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "test"+ext)
			_, err := tool.Execute(map[string]any{
				"path":    testFile,
				"content": "test",
			})
			if err == nil {
				t.Errorf("Expected error for dangerous extension %s", ext)
			}
		})
	}
}

func TestFileWriteExecuteSafeExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager([]string{tmpDir}, []string{})
	tool := NewFileWrite(manager)

	safeExts := []string{".txt", ".md", ".json", ".csv", ".log"}

	for _, ext := range safeExts {
		t.Run("safe_extension_"+ext, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "test"+ext)
			_, err := tool.Execute(map[string]any{
				"path":    testFile,
				"content": "test content",
			})
			if err != nil {
				t.Errorf("Execute failed for safe extension %s: %v", ext, err)
			}
		})
	}
}
