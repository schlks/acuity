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
)

func main() {
	wClient, err := db.NewWeaviateClient("localhost:8080")
	if err != nil {
		log.Fatalf("Failed to create Weaviate client: %v", err)
	}
	sClient, err := db.NewSqliteDB("acuity.db")
	if err != nil {
		log.Fatalf("Failed to open SQLite database")
	}
	err = sClient.InitTable()
	if err != nil {
		log.Fatalf("Failed to initialize table")
	}

	// ctx := context.Background()
	// wClient.ResetDatabase(ctx)

	err = wClient.InitSchema()
	if err != nil {
		log.Fatalf("Error during schema initialization: %v", err)
	}

	/*
	known, err := wClient.GetKnownPaths(ctx)
	if err != nil {
		log.Fatalf("Error retrieving existing files: %v", err)
	}
	var filePaths []string
	err = filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		// d.IsDir() filters out directories; remove this check to include folders
		if !d.IsDir() {
			if _, exists := known[absPath]; !exists {
				filePaths = append(filePaths, absPath)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	go func(){
		ctx = context.Background()
		wClient.ImportImages(ctx, filePaths)
	}()
	*/

	templates := template.Must(template.ParseGlob("src/web/templates/*.html"))

	webServer := web.NewServer(sClient, wClient, templates)

	mux := http.NewServeMux()
	webServer.RegisterRoutes(mux)

	log.Println("acuity Web-Interface listening on http://localhost:3000")
	err = http.ListenAndServe(":3000", mux)
	if err != nil {
		log.Fatalf("Server crashed: %v", err)
	}
}
