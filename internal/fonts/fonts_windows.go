//go:build windows

package fonts

import (
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// GetSystemFonts returns a sorted list of installed font family names on Windows
// by reading the system and user font registry keys.
func GetSystemFonts() ([]string, error) {
	fontSet := make(map[string]bool)

	readFontRegistryKey(registry.LOCAL_MACHINE, fontSet)
	readFontRegistryKey(registry.CURRENT_USER, fontSet)

	var fonts []string
	for name := range fontSet {
		fonts = append(fonts, name)
	}
	sort.Strings(fonts)
	return fonts, nil
}

func readFontRegistryKey(k registry.Key, fontSet map[string]bool) {
	key, err := registry.OpenKey(k, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer key.Close()

	names, err := key.ReadValueNames(0)
	if err != nil {
		return
	}

	for _, name := range names {
		clean := cleanFontName(name)
		if clean != "" {
			fontSet[clean] = true
		}
	}
}

func cleanFontName(name string) string {
	s := name
	// Remove type suffixes like "(TrueType)", "(OpenType)", etc.
	for _, suffix := range []string{
		" (TrueType)", " (OpenType)", " (Type 1)",
		" (All res)", " (Bitmap)", " (Vector)",
	} {
		s = strings.TrimSuffix(s, suffix)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Skip entries that look like file paths or are too short
	if strings.Contains(s, "\\") || strings.Contains(s, "/") {
		return ""
	}
	return s
}
