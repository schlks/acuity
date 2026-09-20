package main

import (
	"embed"
	"fmt"
	"log/slog"
	"os"

	"acuity/pkg/config"

	"github.com/h2non/bimg"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:ui/build
var assets embed.FS

func main() {
	bimg.Initialize()
	defer bimg.Shutdown()

	Config, err := config.Load()
	if err != nil {
		fmt.Println("Failed to load config")
		os.Exit(1)
	}

	if Config.Debug {
		opts := &slog.HandlerOptions{
			AddSource: true,
		}
		logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
		slog.SetDefault(logger)
	}

	app := NewApp(Config)
	err = wails.Run(&options.App{
		Title:            "Acuity",
		Width:            1280,
		Height:           800,
		MinWidth:         1024,
		MinHeight:        700,
		MaxWidth:         7680,
		MaxHeight:        4320,
		WindowStartState: options.Maximised,

		BackgroundColour: &options.RGBA{R: 9, G: 9, B: 9, A: 255},
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.mux,
		},

		OnStartup: app.startup,

		Bind: []any{
			app,
		},

		Linux: &linux.Options{
			WindowIsTranslucent: false,
			// Wails v2.15 maps this enum shifted by one, so OnDemand is
			// what actually yields WebKit's ACCELERATION_POLICY_ALWAYS.
			WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
		},

		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
	})
	if err != nil {
		slog.Error("Failed to start Desktop-App", slog.Any("error", err))
	}
}
