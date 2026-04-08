//go:build darwin

package fonts

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// GetSystemFonts returns a sorted list of installed font family names on macOS.
// It tries fc-list first, then falls back to scanning font directories.
func GetSystemFonts() ([]string, error) {
	fonts, err := getFontsFromFcList()
	if err == nil && len(fonts) > 0 {
		return fonts, nil
	}
	// Fallback: scan font directories
	return getFontsFromDirectories()
}

func getFontsFromFcList() ([]string, error) {
	cmd := exec.Command("fc-list", "--format=%{family}\n")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	fontSet := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			// fc-list may return comma-separated family names
			for _, f := range strings.Split(line, ",") {
				f = strings.TrimSpace(f)
				if f != "" {
					fontSet[f] = true
				}
			}
		}
	}

	var fonts []string
	for name := range fontSet {
		fonts = append(fonts, name)
	}
	sort.Strings(fonts)
	return fonts, nil
}

func getFontsFromDirectories() ([]string, error) {
	fontDirs := []string{
		"/Library/Fonts",
		"/System/Library/Fonts",
		filepath.Join(os.Getenv("HOME"), "Library", "Fonts"),
	}

	fontSet := make(map[string]bool)
	for _, dir := range fontDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := strings.ToLower(filepath.Ext(name))
			// Only include common font file types
			switch ext {
			case ".ttf", ".otf", ".ttc", ".dfont":
				base := strings.TrimSuffix(name, filepath.Ext(name))
				base = strings.TrimSpace(base)
				if base != "" {
					fontSet[base] = true
				}
			}
		}
	}

	var fonts []string
	for name := range fontSet {
		fonts = append(fonts, name)
	}
	sort.Strings(fonts)
	return fonts, nil
}
