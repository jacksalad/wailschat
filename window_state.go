package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// windowState holds persistent window geometry.
type windowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximised bool `json:"maximised"`
}

// windowStateFile returns the path to the window state JSON file.
// Placed next to the SQLite database in the user config directory.
func windowStateFile() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "wailschat", "window_state.json")
}

// loadWindowStateFromFile reads the saved window state from disk.
// Returns nil if no state file exists or it's invalid.
func loadWindowStateFromFile() *windowState {
	path := windowStateFile()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var state windowState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	// Clamp to reasonable minimums
	if state.Width < 800 {
		state.Width = 800
	}
	if state.Height < 600 {
		state.Height = 600
	}
	return &state
}

// saveWindowState reads the current window geometry and writes it to disk.
// Called from OnBeforeClose while the window is still alive.
func saveWindowState(ctx context.Context) {
	path := windowStateFile()
	if path == "" {
		return
	}

	x, y := wailsRuntime.WindowGetPosition(ctx)
	w, h := wailsRuntime.WindowGetSize(ctx)
	maximised := wailsRuntime.WindowIsMaximised(ctx)

	state := windowState{X: x, Y: y, Width: w, Height: h, Maximised: maximised}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal window state: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("Failed to save window state: %v", err)
	}
}
