//go:build !windows

package main

// getWorkArea returns the usable screen rectangle (left, top, right, bottom).
// On non-Windows platforms, return reasonable defaults based on common screen sizes.
// The Wails runtime will handle positioning within the actual display.
func getWorkArea() (int, int, int, int) {
	// Return a reasonable default work area.
	// On Linux/macOS, window managers handle work area differently,
	// and Wails runtime will clamp positions appropriately.
	return 0, 0, 1920, 1080
}
