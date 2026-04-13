package gui

import (
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func Run() {
	app := NewApp()

	if err := wails.Run(&options.App{
		Title:         "SnapVector",
		Width:         1480,
		Height:        940,
		MinWidth:      1200,
		MinHeight:     760,
		DisableResize: false,
		AssetServer:   &assetserver.Options{Assets: assets},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		Bind:          []interface{}{app},
	}); err != nil {
		panic(fmt.Errorf("run wails gui: %w", err))
	}
}
