//go:build linux

package fonts

import (
	"bufio"
	"os/exec"
	"sort"
	"strings"
)

// GetSystemFonts returns a sorted list of installed font family names on Linux
// using fontconfig's fc-list command.
func GetSystemFonts() ([]string, error) {
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
