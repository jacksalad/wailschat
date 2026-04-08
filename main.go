package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed frontend/src/assets/logo.png
var icon []byte

func main() {
	app := NewApp()

	// Load saved window state (file-based, works before wails.Run)
	savedState := loadWindowStateFromFile()

	// Default window size
	width := 1200
	height := 900
	var startState options.WindowStartState

	if savedState != nil {
		width = savedState.Width
		height = savedState.Height
		if savedState.Maximised {
			startState = options.Maximised
		}
	}

	err := wails.Run(&options.App{
		Title:            "WailsChat",
		Width:            width,
		Height:           height,
		MinWidth:         800,
		MinHeight:        600,
		WindowStartState: startState,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		Debug:            options.Debug{OpenInspectorOnStartup: false},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
