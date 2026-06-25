package main

import (
	"log"
	"net/http"
	"html/template"
	//"context"
	//"io/fs"
	//"path/filepath"

	"acuity/pkg/db"
	"acuity/pkg/web"
	"acuity/internal/config"
)

func main() {
	Config, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	wClient, err := db.NewWeaviateClient(Config.WeaviateHost)
	if err != nil {
		log.Fatalf("Failed to create Weaviate client: %v", err)
	}

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

	templates := template.Must(template.ParseGlob("src/web/templates/*.html"))

	webServer := web.NewServer(sClient, wClient, templates)

	mux := http.NewServeMux()
	webServer.RegisterRoutes(mux)

	log.Printf("acuity Web-Interface listening on http://localhost:%s\n", Config.Port)
	err = http.ListenAndServe(Config.Port, mux)
	if err != nil {
		log.Fatalf("Server crashed: %v", err)
	}
}
