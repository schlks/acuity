// Package web integrates the
// handlers for the webclient
package web

import (
	"acuity/internal/gallery"
	"acuity/pkg/db"
	"html/template"
	"net/http"
	"strings"
	"io"
	"log/slog"
	"strconv"

	"github.com/mallardduck/go-http-helpers/pkg/query"
	_ "github.com/weaviate/weaviate/entities/models"
)

type Server struct {
	Service		*gallery.GalleryService
	Template 	*template.Template
}

type SearchResult struct {
	Filepath string
	Distance float64
}

func NewServer(sdatabase *db.SQLiteClient, wdatabase *db.WeaviateClient, tmpl *template.Template) *Server {
	service := gallery.NewService(sdatabase, wdatabase)
	return &Server{
		Service:	service,
		Template:	tmpl,
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", s.handleRoot)										// INFO: GET
	mux.HandleFunc("/image", s.handleImage)								// INFO: GET
	mux.HandleFunc("/image", s.deleteFile)								// INFO: DELETE
	mux.HandleFunc("/gallery/{name}/duplicates", s.handleDuplicates)		// INFO: GET
	mux.HandleFunc("/gallery/{name}/search", s.handleTextSearch)			// INFO: GET
	mux.HandleFunc("/gallery/{name}/search/image", s.handleImageSearch)	// INFO: POST
	mux.HandleFunc("/gallery/{name}", s.handleGallery)					// INFO: GET
	mux.HandleFunc("/gallery/{name}", s.createGallery)					// INFO: POST
	mux.HandleFunc("/gallery/{name}", s.deleteGallery)					// INFO: DELETE
	mux.HandleFunc("/gallery/move", s.changeGallery)						// INFO: GET
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	err := s.Template.ExecuteTemplate(w, "landing.html", nil)
	if err != nil {
		message := "Index cannot be loaded"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	image := query.String(r, "path", "")
	if image == "" {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, image)
}

func (s *Server) createGallery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := r.PathValue("name")
	path := r.FormValue("path")

	if err := s.Service.CreateGallery(ctx, name, path); err != nil {
		message := "Failed to create Gallery"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	id, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	if err = s.Template.ExecuteTemplate(w, "gallery.html", id); err != nil {
		message := "Internal server error during rendering"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) deleteGallery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	delete := query.Bool(r, "deleteGallery", false)
	name := r.PathValue("name")

	id, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery Id"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	if err := s.Service.DeleteGallery(ctx, id); err != nil {
		message := "Failed to delete Gallery from Database"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	if delete {
		files, err := s.Service.GetGalleryFiles(ctx, name)
		if err != nil {
			message := "Failed to get Files in Gallery"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
		if err := s.Service.DeleteImages(files); err != nil {
			message := "Failed to delete Gallery"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
	}
}

func (s *Server) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	imageID, err := strconv.Atoi(r.FormValue("imageID"))
	if err != nil {
		message := "Failed to get Image ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	name := r.PathValue("name")
	galleryID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	images, err := s.Service.FindDublicates(ctx, imageID, galleryID)
	if err != nil {
		message := "Failed to find duplicate Images"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	
	err = s.Template.ExecuteTemplate(w, "result.html", images)
	if err != nil {
		message := "Internal server error during rendering"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var image db.Image
	image.ID = r.FormValue("id")
	image.Path = r.FormValue("path")

	if err := s.Service.DeleteImage(ctx, image); err != nil {
		message := "Failed to delete File"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) deleteFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := r.ParseForm()
	if err != nil {
		message := "Failed to parse Form"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	imageIDs := r.Form["image_id"]

	var images []db.Image
	for _, id := range imageIDs {
		images = append(images, db.Image{ID: id})
	}

	if err := s.Service.DeleteImages(ctx, images)
}

func (s *Server) handleGallery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// NOTE: strconv.ParseInt für mehr als base 10
	name := r.PathValue("name")


	galleryID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	images, err := s.Service.GetAllImages(ctx, galleryID)
	if err != nil {
		message := "Images not found"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusNotFound)
		return
	}
	if err = s.Template.ExecuteTemplate(w, "gallery.html", images); err != nil {
		message := "Internal server error during rendering"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) changeGallery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := r.ParseForm()
	if err != nil {
		message := "Failed to parse Form"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	newID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	imageIDs := r.Form["image_id"]

	var images []db.Image
	for _, id := range imageIDs {
		images = append(images, db.Image{ID: id})
	}

	if err := s.Service.ChangeGallery(ctx, newID, images); err != nil {
		message := "Failed to change Gallery"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleTextSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	search := query.String(r, "q", "")
	limit := query.Int(r, "limit", 100)
	//options := query.Strings(r, "op")

	name := r.PathValue("name")
	galleryID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	images, err := s.Service.SearchImages(ctx, search, limit, galleryID)
	if err != nil {
		message := "Error during database query"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	err = s.Template.ExecuteTemplate(w, "result.html", images)
	if err != nil {
		message := "Internal server error during rendering"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleImageSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := query.Int(r, "limit", 100)
	//options := query.Strings(r, "op")

	name := r.PathValue("name")
	galleryID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		message := "Error parsing the form"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		message := "Error reading the file"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	defer func() {
		_ = file.Close()
	}()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		message := "Error reading into memory"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	fileType := http.DetectContentType(fileBytes)
	if !strings.HasPrefix(fileType, "image/") {
		http.Error(w, "Only images are allowed", http.StatusBadRequest)
		return
	}

	file64, err := s.Service.ConvertImage(fileBytes)
	if err != nil {
		message := "Failed to convert Image to Base64"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	images, err := s.Service.SearchImages64(ctx, file64, limit, galleryID)
	if err != nil {
		message := "Error during database query"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	err = s.Template.ExecuteTemplate(w, "result.html", images)
	if err != nil {
		message := "Internal server error during rendering"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}
