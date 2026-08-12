package main

import (
	"embed"
	"fmt"
	"log/slog"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"acuity/internal/config"

	"github.com/wailsapp/wails/v2"
        "github.com/wailsapp/wails/v2/pkg/options"
        "github.com/wailsapp/wails/v2/pkg/options/assetserver"
        "github.com/wailsapp/wails/v2/pkg/options/linux"
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

var (
	lensMetaRegex = regexp.MustCompile(`(?i)\b\d+(\.\d+)?mm\b|\bf/\d+(\.\d+)?\b`)
	spaceRegex    = regexp.MustCompile(`\s+`)
)

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

//go:embed all:ui/build
var assets embed.FS

func main() {
	_ = os.Setenv("WEBKIT_FORCE_COMPOSITING_MODE", "1")

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
		Title:		"Acuity",
		Width: 		1280,
		Height: 	800,
		MinWidth: 	1024,
		MinHeight: 	700,

		BackgroundColour: &options.RGBA{R: 9, G: 9, B: 9, A: 255},
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.mux,
		},

		OnStartup: 	app.startup,

		Bind: []any{
			app,
		},

		Linux: &linux.Options{
			WindowIsTranslucent: false,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyAlways,
		},
	})

	if err != nil {
		slog.Error("Failed to start Desktop-App", slog.Any("error", err))
	}
}
