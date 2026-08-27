package main

import (
	"acuity/pkg/config"
	"embed"
	"fmt"
	"log/slog"
	"os"

	"github.com/h2non/bimg"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:ui/build
var assets embed.FS

func main() {
	_ = os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
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
		Title:     "Acuity",
		Width:     1280,
		Height:    800,
		MinWidth:  1024,
		MinHeight: 700,

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
			WebviewGpuPolicy:    linux.WebviewGpuPolicyAlways,
		},

		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
			DisableWebViewDrop: true,
		},
	})

	if err != nil {
		slog.Error("Failed to start Desktop-App", slog.Any("error", err))
	}
}
