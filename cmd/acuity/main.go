package main

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"time"

	//"context"
	//"io/fs"
	//"path/filepath"
	"acuity"
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

func main() {
	Config, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	wClient, err := db.NewWeaviateClient(Config.WeaviateHost + Config.WeaviatePort)
	if err != nil {
		log.Fatalf("Failed to create Weaviate client: %v", err)
	}

	if err := wClient.WaitForReady(120 * time.Second); err != nil {
		log.Fatalf("Failed to wait for Weaviate server: %v", err)
	}

	fmt.Println("Connected to Weaviate server")

	sClient, err := db.NewSqliteDB(Config.DBPath)
	if err != nil {
		log.Fatalf("Failed to open SQLite database")
	}

	if err := sClient.InitTable(); err != nil {
		log.Fatalf("Failed to initialize table")
	}

	if err := wClient.InitSchema(); err != nil {
		log.Fatalf("Error during schema initialization: %v", err)
	}

	funcMap := template.FuncMap{
		"formatSize": formatSize,
		"formatRes":  formatResolution,
		"formatDate": formatDate,
	}

	templates := template.Must(template.New("").Funcs(funcMap).ParseFS(acuity.WebFS, "web/templates/*.html"))

	webServer := web.NewServer(sClient, wClient, Config, templates)

	mux := http.NewServeMux()
	webServer.RegisterRoutes(mux)

	log.Printf("acuity Web-Interface listening on http://localhost:%s\n", Config.Port)
	err = http.ListenAndServe(":"+Config.Port, mux)
	if err != nil {
		log.Fatalf("Server crashed: %v", err)
	}
}
