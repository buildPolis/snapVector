package gui

import (
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	linuxoptions "github.com/wailsapp/wails/v2/pkg/options/linux"
)

func Run() {
	app := NewApp()
	configureLinuxProgramIdentity()
	ensureLinuxDesktopIntegration()

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
		Linux: &linuxoptions.Options{
			Icon:        appIcon,
			ProgramName: "snapvector",
		},
	}); err != nil {
		panic(fmt.Errorf("run wails gui: %w", err))
	}
}
