package main

import (
	"acuity/pkg/config"
	"acuity/pkg/db"
	"acuity/pkg/web"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

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
	wClient, err := db.NewWeaviateClient(a.config.WeaviateHost + a.config.WeaviatePort)
	if err != nil {
		slog.Error("Failed to create Weaviate client", slog.Any("error", err))
		os.Exit(1)
	}

	if err := wClient.WaitForReady(120 * time.Second); err != nil {
		slog.Error("Failed to wait for Weaviate server", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Connected to Weaviate server")

	sClient, err := db.NewSqliteDB(a.config.DBPath)
	if err != nil {
		slog.Error("Failed to open SQLite database", slog.Any("error", err))
		os.Exit(1)
	}

	if err := sClient.InitTable(); err != nil {
		slog.Error("Failed to initialize table", slog.Any("error", err))
		os.Exit(1)
	}

	if err := wClient.InitSchema(); err != nil {
		slog.Error("Error during schema initialization", slog.Any("error", err))
		os.Exit(1)
	}

	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "web"
	}
	// templates := template.Must(template.New("").Funcs(funcMap).ParseGlob(filepath.Join(webDir, "templates/*.html")))

	webServer := web.NewServer(sClient, wClient, a.config)

	webServer.RegisterRoutes(a.mux)
	slog.Info(fmt.Sprintf("Acuity Web-Interface listening on http://localhost:%s", a.config.Port))
	go func() {
		err := http.ListenAndServe(":"+a.config.Port, a.mux)
		if err != nil {

		}
	}()
}

func (a *App) SelectDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Gallery Directory",
	})
}
