package main

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"acuity/internal/config"
	"acuity/pkg/db"
	"acuity/pkg/web"
)

func formatSize(b float64) string {
	const unit = 1024.0
	if b < unit {
		return fmt.Sprintf("%f B", b)
	}

	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", b/div, "KMGTPE"[exp])
}

func formatResolution(pixels int, aspectRatio float64) string {
	if pixels == 0 {
		return "Unkown"
	}

	if aspectRatio == 0 {
		return fmt.Sprintf("%.1fMP", float64(pixels)/1_000_000.0)
	}

	h := math.Round(math.Sqrt(float64(pixels) / aspectRatio))
	w := math.Round(float64(pixels) / h)

	resMap := map[int]string{
		720:  "HD",
		1080: "FHD",
		1440: "UHD",
		2160: "4K",
		4320: "8K",
	}

	if name, exists := resMap[int(h)]; exists {
		return name
	}

	return fmt.Sprintf("%.0f x %.0f", w, h)
}

func formatDate(dateStr string) string {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}

	return t.Format("02. Jan 2006, 15:04")
}

func formatExt(ext string) string {
	cleanExt := strings.TrimPrefix(ext, ".")
	return strings.ToUpper(cleanExt)
}

func formatAperture(a string) string {
	a = strings.TrimSpace(a)
	if a == "" || a == "0" || a == "0.0" || a == "0.00" {
		return ""
	}
	return strings.TrimPrefix(a, "f/")
}

func formatShutter(s string) string {
	s = strings.TrimSpace(strings.TrimSuffix(s, "s"))
	if s == "" || s == "0" || s == "0.0" || s == "0.00" {
		return ""
	}

	if !strings.Contains(s, "/") {
		if f, err := strconv.ParseFloat(s, 64); err == nil && f > 0 {
			if f < 1.0 {
				denominator := math.Round(1.0 / f)
				s = fmt.Sprintf("1/%.0f", denominator)
			} else {
				s = fmt.Sprintf("%v", f)
			}
		}
	}

	if !strings.HasSuffix(s, "s") {
		return s + "s"
	}
	return s
}

func formatFocalLength(f string) string {
	f = strings.TrimSpace(f)
	if f == "" || f == "0" || f == "0.0" || f == "0.00" || f == "0mm" || f == "0.0mm" || f == "0.00mm" {
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(f), "mm") {
		return f + "mm"
	}
	return f
}

func formatIso(iso string) string {
	iso = strings.TrimSpace(iso)
	if iso == "" || iso == "0" || iso == "0.0" || iso == "0.00" {
		return ""
	}
	return iso
}

var lensMetaRegex = regexp.MustCompile(`(?i)\b\d+(\.\d+)?mm\b|\bf/\d+(\.\d+)?\b`)
var spaceRegex = regexp.MustCompile(`\s+`)

func formatLens(make, model string) string {
	make = strings.TrimSpace(make)
	model = strings.TrimSpace(model)

	// Vermeide Redundanz, wenn der Hersteller schon im Modellnamen steht
	if make != "" && strings.HasPrefix(strings.ToLower(model), strings.ToLower(make)) {
		make = ""
	}

	res := strings.TrimSpace(make + " " + model)

	// Entferne Brennweite und Blende aus dem Objektivnamen (oft bei Smartphones der Fall)
	res = lensMetaRegex.ReplaceAllString(res, "")
	res = spaceRegex.ReplaceAllString(res, " ")

	return strings.TrimSpace(res)
}

func main() {
	// Logger so konfigurieren, dass Datei und Funktion ausgegeben werden
	opts := &slog.HandlerOptions{
		AddSource: true,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	Config, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	wClient, err := db.NewWeaviateClient(Config.WeaviateHost + Config.WeaviatePort)
	if err != nil {
		slog.Error("Failed to create Weaviate client", slog.Any("error", err))
		os.Exit(1)
	}

	if err := wClient.WaitForReady(120 * time.Second); err != nil {
		slog.Error("Failed to wait for Weaviate server", slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("Connected to Weaviate server")

	sClient, err := db.NewSqliteDB(Config.DBPath)
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

	funcMap := template.FuncMap{
		"formatSize":        formatSize,
		"formatRes":         formatResolution,
		"formatDate":        formatDate,
		"formatExt":         formatExt,
		"formatAperture":    formatAperture,
		"formatShutter":     formatShutter,
		"formatLens":        formatLens,
		"formatFocalLength": formatFocalLength,
		"formatIso":         formatIso,
		"formatDeepest": func(path string) string {
			if path == "" {
				return ""
			}
			return filepath.Base(filepath.Dir(path))
		},
		"getConfig": func() *config.Config { return Config },
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
	}

	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "web"
	}
	templates := template.Must(template.New("").Funcs(funcMap).ParseGlob(filepath.Join(webDir, "templates/*.html")))

	webServer := web.NewServer(sClient, wClient, Config, templates)

	mux := http.NewServeMux()
	webServer.RegisterRoutes(mux)

	slog.Info(fmt.Sprintf("Acuity Web-Interface listening on http://localhost:%s", Config.Port))
	err = http.ListenAndServe(":"+Config.Port, mux)
	if err != nil {
		slog.Error("Server crashed", slog.Any("error", err))
		os.Exit(1)
	}
}
