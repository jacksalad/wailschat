package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileReadName(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewFileRead(manager)

	if tool.Name() != "file_read" {
		t.Errorf("Expected name 'file_read', got '%s'", tool.Name())
	}
}

func TestFileReadDescription(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewFileRead(manager)

	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestFileReadParameters(t *testing.T) {
	manager := NewManager([]string{}, []string{})
	tool := NewFileRead(manager)

	params := tool.Parameters()
	if len(params) == 0 {
		t.Error("Parameters should not be empty")
	}
}

func TestFileReadExecute(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test with allowed directory
	manager := NewManager([]string{tmpDir}, []string{})
	tool := NewFileRead(manager)

	// Test successful read
	result, err := tool.Execute(map[string]any{
		"path": testFile,
	})
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}
	if result != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, result)
	}

	// Test with custom max_size (smaller than file size - should fail)
	_, err2 := tool.Execute(map[string]any{
		"path":     testFile,
		"max_size": float64(10),
	})
	if err2 == nil {
		t.Error("Expected error for file exceeding custom max_size")
	}

	// Test with custom max_size (larger than file size - should succeed)
	result3, err3 := tool.Execute(map[string]any{
		"path":     testFile,
		"max_size": float64(100),
	})
	if err3 != nil {
		t.Errorf("Execute with larger max_size failed: %v", err3)
	}
	if result3 != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, result3)
	}
}

func TestFileReadExecuteErrors(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager([]string{tmpDir}, []string{})
	tool := NewFileRead(manager)

	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{
			name:    "missing path parameter",
			args:    map[string]any{},
			wantErr: true,
		},
		{
			name: "invalid path type",
			args: map[string]any{
				"path": 123,
			},
			wantErr: true,
		},
		{
			name: "file not found",
			args: map[string]any{
				"path": "/nonexistent/file.txt",
			},
			wantErr: true,
		},
		{
			name: "directory instead of file",
			args: map[string]any{
				"path": tmpDir,
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

func TestFileReadExecutePathValidation(t *testing.T) {
	// Create a restricted directory
	restrictedDir := t.TempDir()
	manager := NewManager([]string{restrictedDir}, []string{})
	tool := NewFileRead(manager)

	// Create a file outside the allowed directory
	otherDir := t.TempDir()
	outsideFile := filepath.Join(otherDir, "outside.txt")
	err := os.WriteFile(outsideFile, []byte("outside content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create outside file: %v", err)
	}

	// Test path validation - should fail for file outside allowed directory
	_, err = tool.Execute(map[string]any{
		"path": outsideFile,
	})
	if err == nil {
		t.Error("Expected error for file outside allowed directory")
	}

	// Create a file inside the allowed directory
	insideFile := filepath.Join(restrictedDir, "inside.txt")
	err = os.WriteFile(insideFile, []byte("inside content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create inside file: %v", err)
	}

	// Test path validation - should succeed for file inside allowed directory
	result, err := tool.Execute(map[string]any{
		"path": insideFile,
	})
	if err != nil {
		t.Errorf("Execute failed for allowed file: %v", err)
	}
	if result != "inside content" {
		t.Errorf("Expected 'inside content', got '%s'", result)
	}
}

func TestFileReadExecuteFileSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager([]string{tmpDir}, []string{})
	tool := NewFileRead(manager)

	// Create a file larger than default max size
	largeContent := make([]byte, defaultMaxFileSize+1000)
	for i := range largeContent {
		largeContent[i] = 'A'
	}
	largeFile := filepath.Join(tmpDir, "large.txt")
	err := os.WriteFile(largeFile, largeContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	// Test with default max size (should fail)
	_, err = tool.Execute(map[string]any{
		"path": largeFile,
	})
	if err == nil {
		t.Error("Expected error for file exceeding default max size")
	}

	// Test with custom max size
	_, err = tool.Execute(map[string]any{
		"path":     largeFile,
		"max_size": float64(hardMaxFileSize),
	})
	if err != nil {
		t.Errorf("Execute with custom max_size failed: %v", err)
	}
}
