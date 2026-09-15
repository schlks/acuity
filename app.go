package main

import (
	"acuity/pkg/ai"
	"acuity/pkg/config"
	"acuity/pkg/db"
	"acuity/pkg/version"
	"acuity/pkg/web"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx    context.Context
	config *config.Config
	mux    *http.ServeMux
}

func NewApp(cfg *config.Config) *App {
	return &App{
		config: cfg,
		mux:    http.NewServeMux(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	sClient, err := db.NewSqliteDB(a.config.DBPath)
	if err != nil {
		slog.Error("Failed to open SQLite database", slog.Any("error", err))
		os.Exit(1)
	}

	if err := sClient.InitTable(); err != nil {
		slog.Error("Failed to initialize table", slog.Any("error", err))
		os.Exit(1)
	}

	vClient := db.NewVectorClient(sClient, nil)
	if err := vClient.InitSchema(); err != nil {
		slog.Error("Failed to initialize vector schema", slog.Any("error", err))
		os.Exit(1)
	}

	bridge := db.NewBridge(sClient, vClient)

	// Initialize AI models in background (so UI opens instantly and shows setup modal if downloading)
	go func() {
		modelPaths, err := ai.EnsureModels()
		if err != nil {
			slog.Error("Failed to ensure AI models", slog.Any("error", err))
			return
		}

		embedder, err := ai.NewCLIPEmbedder(modelPaths.VisualModelPath, modelPaths.TextualModelPath, modelPaths.OnnxLibPath)
		if err != nil {
			slog.Error("Failed to initialize CLIP AI embedder", slog.Any("error", err))
			return
		}

		vClient.SetEmbedder(embedder)
		slog.Info("Local CLIP AI Engine successfully initialized")

		go func() {
			if err := bridge.IndexMissingVectors(context.Background()); err != nil {
				slog.Error("Vector backfill error", slog.Any("error", err))
			}
		}()
	}()

	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "web"
	}

	webServer := web.NewServer(sClient, vClient, a.config)
	webServer.RegisterRoutes(a.mux)

	// The webview loads the UI through Wails' own wails:// scheme, which routes
	// requests to a.mux in-process (no socket, no external access) via the
	// AssetServer.Handler set up in main.go. A real TCP listener is only needed
	// during `wails dev`, where the frontend runs in a separate Vite dev server
	// that proxies /api requests to this port.
	if runtime.Environment(ctx).BuildType == "dev" {
		slog.Info(fmt.Sprintf("Acuity Web-Interface listening on http://localhost:%s", a.config.Port))
		go func() {
			err := http.ListenAndServe("127.0.0.1:"+a.config.Port, a.mux)
			if err != nil {
				slog.Error("Web server error", slog.Any("error", err))
			}
		}()
	}
}

func (a *App) SelectDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Gallery Directory",
	})
}

func (a *App) GetVersion() string {
	return version.Version
}
