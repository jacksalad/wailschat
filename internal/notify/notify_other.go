//go:build !windows

package notify

import "log"

// Show displays a desktop notification.
// On non-Windows platforms this is a no-op placeholder.
func Show(title, body string) {
	log.Printf("[notify] %s: %s", title, body)
}
