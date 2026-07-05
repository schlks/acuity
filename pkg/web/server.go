// Package web integrates the
// handlers for the webclient
package web

import (
	"acuity"
	"acuity/internal/config"
	"acuity/internal/gallery"
	"acuity/pkg/db"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mallardduck/go-http-helpers/pkg/query"
	_ "github.com/weaviate/weaviate/entities/models"
)

type Server struct {
	Service  *gallery.GalleryService
	Template *template.Template
	Config   *config.Config
}

type SearchResult struct {
	Filepath string
	Distance float64
}

// NewServer creates a new server
func NewServer(sdatabase *db.SQLiteClient, wdatabase *db.WeaviateClient, config *config.Config, tmpl *template.Template) *Server {
	service := gallery.NewService(sdatabase, wdatabase)
	return &Server{
		Service:  service,
		Template: tmpl,
		Config:   config,
	}
}

// RegisterRoutes regusteres all the routes used by the webui
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	staticFS, _ := fs.Sub(acuity.WebFS, "web/static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("GET	/settings", s.getSettings)
	mux.HandleFunc("POST	/settings", s.setSettings)
	mux.HandleFunc("GET	/image", s.handleImage)
	mux.HandleFunc("DELETE /image/{id}", s.deleteFile)
	mux.HandleFunc("GET	/image/{id}", s.getInfo)
	mux.HandleFunc("POST	/image/{id}", s.setRating)
	mux.HandleFunc("DELETE /images", s.deleteFiles)
	mux.HandleFunc("GET	/gallery/{name}/duplicates", s.handleDuplicates)
	mux.HandleFunc("GET	/gallery/{name}/search", s.handleTextSearch)
	mux.HandleFunc("POST	/gallery/{name}/search/image", s.handleImageSearch)
	mux.HandleFunc("POST	/gallery/{name}/scan", s.scanGallery)
	mux.HandleFunc("POST /gallery/{name}/edit", s.editGallery)
	mux.HandleFunc("GET	/gallery/{name}", s.getGallery)
	mux.HandleFunc("GET	/gallery/{name}/images", s.getGalleryImages)
	mux.HandleFunc("GET /gallery/{name}/progress", s.getProgress)
	mux.HandleFunc("POST	/gallery", s.createGallery)
	mux.HandleFunc("DELETE /gallery/{name}", s.deleteGallery)
	mux.HandleFunc("POST	/gallery/move", s.changeGallery)
	mux.HandleFunc("POST	/gallery/copy", s.copyToGallery)
	mux.HandleFunc("GET	/api/browse", s.browseFiles)
}

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	galleries, err := s.Service.GetAllGalleries()
	if err != nil {
		slog.Error("Failed to load galleries", slog.Any("error", err))
	}

	data := struct {
		Galleries []db.Gallery
		Config    *config.Config
	}{
		Galleries: galleries,
		Config:    s.Config,
	}

	err = s.Template.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		message := "Index cannot be loaded"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) browseFiles(w http.ResponseWriter, r *http.Request) {
	dir := query.String(r, "dir", "/dir")
	if dir == "" {
		dir = "/"
	}

	dir = filepath.Clean(dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, "Kein Zugriff", http.StatusForbidden)
		return
	}

	var folders []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			folders = append(folders, e.Name())
		}
	}

	parentDir := filepath.Dir(dir)
	if dir == "/" {
		parentDir = ""
	}

	data := struct {
		CurrentDir string
		ParentDir  string
		Folders    []string
	}{
		CurrentDir: dir,
		ParentDir:  parentDir,
		Folders:    folders,
	}

	if err := s.Template.ExecuteTemplate(w, "file-browser", data); err != nil {
		message := "file browser cannot be loaded"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
	err := s.Template.ExecuteTemplate(w, "settings.html", s.Config)
	if err != nil {
		message := "Index cannot be loaded"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) setSettings(w http.ResponseWriter, r *http.Request) {
	var err error
	s.Config.ImagesPerPage, err = strconv.Atoi(r.FormValue("per_page"))
	if err != nil {
		message := "Failed to get the number for images per page"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	s.Config.DefaultSortBy = r.FormValue("sort_by")
	s.Config.DefaultSortOrder = r.FormValue("sort_order")
	s.Config.GridSize = r.FormValue("grid_size")

	if err := config.Save(s.Config); err != nil {
		slog.Error("Failed to save config", slog.Any("error", err))
	}

	w.Header().Set("HX-Trigger", "refresh-images")
	w.WriteHeader(http.StatusOK)

	// Return an Out-Of-Band update for the search-options-form so the UI reflects the new defaults without a full reload
	formTmpl := `
	<form id="search-options-form" hx-swap-oob="true" @change="let q = document.querySelector('input[name=\'q\']'); if(q && q.value.trim()){ htmx.trigger(q, 'keyup', {key: 'Enter'}); } else { htmx.trigger(document.body, 'refresh-images'); }">
		<div style="margin-bottom: 12px;">
			<div style="font-size: 0.8rem; color: var(--text-muted); margin-bottom: 4px;">Sort by</div>
			<select name="sortBy" class="input-clean" style="width: 100%; padding: 8px; border-radius: 8px;">
				<option value="name" {{if eq .DefaultSortBy "name"}}selected{{end}}>Name</option>
				<option value="date" {{if eq .DefaultSortBy "date"}}selected{{end}}>Date</option>
				<option value="size" {{if eq .DefaultSortBy "size"}}selected{{end}}>Size</option>
				<option value="rating" {{if eq .DefaultSortBy "rating"}}selected{{end}}>Rating</option>
			</select>
		</div>
		<div>
			<div style="font-size: 0.8rem; color: var(--text-muted); margin-bottom: 4px;">Order</div>
			<select name="sortOrder" class="input-clean" style="width: 100%; padding: 8px; border-radius: 8px;">
				<option value="desc" {{if eq .DefaultSortOrder "desc"}}selected{{end}}>Descending</option>
				<option value="asc" {{if eq .DefaultSortOrder "asc"}}selected{{end}}>Ascending</option>
			</select>
		</div>
	</form>`

	t := template.Must(template.New("form").Parse(formTmpl))
	t.Execute(w, s.Config)
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

	name := r.FormValue("name")
	path := r.FormValue("path")

	/*if !strings.HasPrefix(path, "/images/") {
		path = filepath.Join("/images", path)
	}*/

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
	images, err := s.Service.GetAllImages(ctx, id, s.Config.DefaultSortBy, s.Config.DefaultSortOrder, 0, s.Config.ImagesPerPage)
	if err != nil {
		message := "Failed to fetch images for new gallery"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	count, _ := s.Service.GetGalleryCount(ctx, id)

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)

	if err = s.Template.ExecuteTemplate(w, "gallery.html", []any{images, count, name}); err != nil {
		message := "Internal server error during rendering"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	return
}

func (s *Server) editGallery(w http.ResponseWriter, r *http.Request) {
	oldName := r.PathValue("name")
	newName := r.FormValue("name")

	id, err, ok := s.Service.GetGalleryID(oldName)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	if ok {
		if err := s.Service.EditGalleryName(id, newName); err != nil {
			message := "Failed to update Gallery"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Redirect", "/gallery/"+newName)
}

func (s *Server) getProgress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := r.PathValue("name")
	galleryID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	expected := s.Service.ExpectedCount[galleryID]
	current, _ := s.Service.GetGalleryCount(ctx, galleryID)

	if expected > 0 && current < expected {
		data := struct {
			GalleryName string
			Current     int
			Expected    int
			Percent     int
		}{
			GalleryName: name,
			Current:     current,
			Expected:    expected,
			Percent:     int(float64(current) / float64(expected) * 100),
		}
		if current > 0 {
			w.Header().Set("HX-Trigger", "refresh-images")
		}
		if err := s.Template.ExecuteTemplate(w, "progress.html", data); err != nil {
			message := "progress cannot be loaded"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
		}
	} else {
		w.Header().Set("HX-Trigger", "refresh-images")
		_, err := w.Write([]byte(``))
		if err != nil {
			return
		}
	}
}

func (s *Server) scanGallery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := r.PathValue("name")

	if err := s.Service.UpdateFolder(ctx, name); err != nil {
		message := "Failed to update gallery"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", "check-progress")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) deleteGallery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := r.PathValue("name")
	deleteGallery := query.Bool(r, "deleteGallery", false)

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
	if deleteGallery {
		files, err := s.Service.GetGalleryFiles(ctx, name, "", "", -1, 10_000)
		if err != nil {
			message := "Failed to get Files in Gallery"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
		if err := s.Service.DeleteImages(ctx, files, false); err != nil {
			message := "Failed to delete Gallery"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Redirect", "/")
}

func (s *Server) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	imageID := r.FormValue("imageID")
	if imageID == "" {
		message := "Failed to get Image ID"
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	name := r.PathValue("name")
	page := query.Int(r, "page", 1)

	galleryID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	images, err := s.Service.FindDublicates(ctx, imageID, galleryID, page-1, s.Config.ImagesPerPage)
	if err != nil {
		message := "Failed to find duplicate Images"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	queryImage, _ := s.Service.GetImageInfo(ctx, imageID)
	returnTo := query.String(r, "returnTo", name)

	data := map[string]any{
		"Images":      images,
		"QueryImage":  queryImage,
		"Query":       imageID,
		"Page":        page,
		"PrevPage":    page - 1,
		"NextPage":    page + 1,
		"HasNext":     len(images) == s.Config.ImagesPerPage,
		"GalleryName": name,
		"ReturnTo":    returnTo,
		"SearchType":  "duplicate",
	}

	err = s.Template.ExecuteTemplate(w, "search-results.html", data)
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
	image.ID = r.PathValue("id")
	if image.ID == "" {
		image.ID = r.FormValue("id")
	}
	image.Path = r.FormValue("path")

	deleteDiskStr := r.FormValue("delete_disk")
	deleteDisk := deleteDiskStr == "true"

	if err := s.Service.DeleteImage(ctx, image, deleteDisk); err != nil {
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

	imageIDsStr := r.FormValue("image_id")
	var imageIDs []string
	if imageIDsStr != "" {
		imageIDs = strings.Split(imageIDsStr, ",")
	}

	deleteDiskStr := r.FormValue("delete_disk")
	deleteDisk := deleteDiskStr == "true"

	var images []db.Image
	for _, id := range imageIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			images = append(images, db.Image{ID: id})
		}
	}

	if err := s.Service.DeleteImages(ctx, images, deleteDisk); err != nil {
		message := "Failed to delete Image"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) getGallery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := r.PathValue("name")

	galleryID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	count, err := s.Service.GetGalleryCount(ctx, galleryID)
	if err != nil {
		message := fmt.Sprintf("Failed to get count of images in Gallery: %d", galleryID)
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusNotFound)
		return
	}

	// Bilder werden auf "nil" gesetzt, da sie erst später lazy geladen werden!
	if err = s.Template.ExecuteTemplate(w, "gallery.html", []any{nil, count, name}); err != nil {
		message := "Internal server error during rendering"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) getGalleryImages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	sortBy := r.URL.Query().Get("sortBy")
	if sortBy == "" {
		sortBy = s.Config.DefaultSortBy
	}

	sortOrder := r.URL.Query().Get("sortOrder")
	if sortOrder == "" {
		sortOrder = s.Config.DefaultSortOrder
	}

	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	galleryID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		http.Error(w, "Failed to get Gallery ID", http.StatusInternalServerError)
		return
	}

	images, err := s.Service.GetAllImages(ctx, galleryID, sortBy, sortOrder, page-1, s.Config.ImagesPerPage)
	if err != nil {
		http.Error(w, "Images not found", http.StatusNotFound)
		return
	}

	nextPage := -1
	if len(images) == s.Config.ImagesPerPage {
		nextPage = page + 1
	}

	count, _ := s.Service.GetGalleryCount(ctx, galleryID)
	lastPage := count / s.Config.ImagesPerPage
	if count%s.Config.ImagesPerPage != 0 {
		lastPage++
	}
	if lastPage == 0 {
		lastPage = 1
	}

	data := struct {
		GalleryName string
		Images      []db.Image
		CurrentPage int
		PrevPage    int
		NextPage    int
		HasNext     bool
		LastPage    int
	}{
		GalleryName: name,
		Images:      images,
		CurrentPage: page,
		PrevPage:    page - 1,
		NextPage:    nextPage,
		HasNext:     nextPage != -1,
		LastPage:    lastPage,
	}

	// Rendert nur die Bilder-Kacheln aus dem neuen Template
	if err := s.Template.ExecuteTemplate(w, "gallery-images.html", data); err != nil {
		slog.Error("Failed to render gallery images", slog.Any("error", err))
		http.Error(w, "Failed to render", http.StatusInternalServerError)
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

func (s *Server) copyToGallery(w http.ResponseWriter, r *http.Request) {
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

	if err := s.Service.CopyToGallery(ctx, newID, images); err != nil {
		message := "Failed to change Gallery"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleTextSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	search := query.String(r, "q", "")
	sortBy := query.String(r, "sortBy", s.Config.DefaultSortBy)
	sortOrder := query.String(r, "sortOrder", s.Config.DefaultSortOrder)
	page := query.Int(r, "page", 1)
	// options := query.Strings(r, "op")

	name := r.PathValue("name")
	galleryID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	images, err := s.Service.SearchImages(ctx, search, galleryID, sortBy, sortOrder, page-1, s.Config.ImagesPerPage)
	if err != nil {
		message := "Error during database query"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	returnTo := query.String(r, "returnTo", name)

	data := map[string]any{
		"Images":      images,
		"Query":       search,
		"Page":        page,
		"PrevPage":    page - 1,
		"NextPage":    page + 1,
		"HasNext":     len(images) == s.Config.ImagesPerPage,
		"GalleryName": name,
		"ReturnTo":    returnTo,
		"SearchType":  "text",
	}

	if err := s.Template.ExecuteTemplate(w, "search-results.html", data); err != nil {
		message := "Internal server error during rendering"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleImageSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	sortBy := query.String(r, "sortBy", s.Config.DefaultSortBy)
	sortOrder := query.String(r, "sortOrder", s.Config.DefaultSortOrder)
	page := query.Int(r, "page", 0)
	// options := query.Strings(r, "op")

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

	images, err := s.Service.SearchImages64(ctx, file64, galleryID, sortBy, sortOrder, page-1, s.Config.ImagesPerPage)
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

func (s *Server) getInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	imageInfo, err := s.Service.GetImageInfo(ctx, id)
	if err != nil {
		message := "Failed to get Image Info"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	if err := s.Template.ExecuteTemplate(w, "info-sidebar", imageInfo); err != nil {
		message := "Internal server error during rendering"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) setRating(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	imageID := r.PathValue("id")

	ratingString := r.FormValue("rating")
	rating, err := strconv.Atoi(ratingString)
	if err != nil {
		message := "Failed to get Image rating"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	if err := s.Service.SetRating(ctx, imageID, rating); err != nil {
		message := "Failed to set Image rating"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	data := struct {
		ID     string
		Rating int
	}{
		ID:     imageID,
		Rating: rating,
	}

	w.Header().Set("HX-Trigger", "rating-updated")

	if err := s.Template.ExecuteTemplate(w, "rating-stars", data); err != nil {
		message := "Failed to render rating"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
	}
}
