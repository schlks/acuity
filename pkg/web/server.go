// Package web integrates the
// handlers for the webclient
package web

import (
	"acuity/internal/config"
	"acuity/internal/gallery"
	"acuity/pkg/db"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/h2non/bimg"
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
	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "web"
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(webDir, "static")))))

	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("GET	/settings", s.getSettings)
	mux.HandleFunc("POST	/settings", s.setSettings)
	mux.HandleFunc("GET	/image", s.handleImage)
	mux.HandleFunc("POST /image/{id}/delete", s.deleteFile)
	mux.HandleFunc("GET	/image/{id}", s.getInfo)
	mux.HandleFunc("POST	/image/{id}", s.setRating)
	mux.HandleFunc("POST /images", s.deleteFiles)
	mux.HandleFunc("GET	/gallery/{name}/duplicates", s.handleDuplicates)
	mux.HandleFunc("POST	/gallery/{name}/search", s.handleSearch)
	mux.HandleFunc("POST	/gallery/{name}/scan", s.scanGallery)
	mux.HandleFunc("DELETE /gallery/{name}/scan", s.cancelScan)
	mux.HandleFunc("POST /gallery/{name}/edit", s.editGallery)
	mux.HandleFunc("GET	/gallery/{name}", s.getGallery)
	mux.HandleFunc("GET	/gallery/{name}/images", s.getGalleryImages)
	mux.HandleFunc("GET /api/progress", s.getGlobalProgress)
	mux.HandleFunc("POST	/gallery", s.createGallery)
	mux.HandleFunc("DELETE /gallery/{name}", s.deleteGallery)
	mux.HandleFunc("POST /gallery/batch", s.handleBatchAction)
	mux.HandleFunc("GET /api/browse", s.browseFiles)
	mux.HandleFunc("POST /api/browse/mkdir", s.mkdir)
	mux.HandleFunc("POST /image/{id}/flag", s.setFlag)
	mux.HandleFunc("POST /api/reset", s.resetDatabase)
}

type SubFolder struct {
	Name       string
	Path       string
	SubFolders []*SubFolder
}

type GalleryView struct {
	db.Gallery
	SubFolders []*SubFolder
}

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	galleries, err := s.Service.GetAllGalleries()
	if err != nil {
		slog.Error("Failed to load galleries", slog.Any("error", err))
	}

	var galleryViews []GalleryView
	for _, g := range galleries {
		gv := GalleryView{Gallery: g}
		nodeMap := make(map[string]*SubFolder)
		filepath.WalkDir(g.Path, func(path string, d fs.DirEntry, err error) error {
			if err != nil || path == g.Path {
				return nil
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				node := &SubFolder{
					Name: d.Name(),
					Path: path,
				}
				nodeMap[path] = node

				parentPath := filepath.Dir(path)
				if parentPath == g.Path {
					gv.SubFolders = append(gv.SubFolders, node)
				} else if parentNode, ok := nodeMap[parentPath]; ok {
					parentNode.SubFolders = append(parentNode.SubFolders, node)
				}
			}
			return nil
		})
		galleryViews = append(galleryViews, gv)
	}

	data := struct {
		Galleries []GalleryView
		Config    *config.Config
	}{
		Galleries: galleryViews,
		Config:    s.Config,
	}

	if err = s.Template.ExecuteTemplate(w, "index.html", data); err != nil {
		slog.Error("Internal server error during rendering", slog.Any("error", err))
		http.Error(w, "Internal server error during rendering", http.StatusInternalServerError)
	}
}

func (s *Server) resetDatabase(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	err := s.Service.ResetDatabase(ctx)
	if err != nil {
		slog.Error("Failed to reset database", slog.Any("error", err))
		http.Error(w, "Failed to reset database", http.StatusInternalServerError)
		return
	}

	// Close the settings dialog and optionally reload the page
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) setFlag(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	imageID := r.PathValue("id")
	flagStr := r.FormValue("flag")
	flag, _ := strconv.Atoi(flagStr)

	if err := s.Service.SetFlag(ctx, imageID, flag); err != nil {
		message := "Failed to set Flag"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	data := struct {
		ID   string
		Flag int
	}{ID: imageID, Flag: flag}
	if err := s.Template.ExecuteTemplate(w, "flag-set", data); err != nil {
		message := "Failed to update Flag"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) browseFiles(w http.ResponseWriter, r *http.Request) {
	dir := query.String(r, "dir", "/data")
	if dir == "" {
		dir = "/data"
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

	target := query.String(r, "target", "")

	data := struct {
		CurrentDir string
		ParentDir  string
		Folders    []string
		Target     string
	}{
		CurrentDir: dir,
		ParentDir:  parentDir,
		Folders:    folders,
		Target:     target,
	}

	if err := s.Template.ExecuteTemplate(w, "file-browser", data); err != nil {
		message := "file browser cannot be loaded"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) mkdir(w http.ResponseWriter, r *http.Request) {
	dir := r.FormValue("dir")
	newFolder := r.FormValue("new_folder")
	target := r.FormValue("target")

	if dir != "" && newFolder != "" {
		newPath := filepath.Join(dir, newFolder)
		err := os.MkdirAll(newPath, 0755)
		if err != nil {
			slog.Error("Failed to create folder", slog.String("path", newPath), slog.Any("error", err))
		}
	}

	urlStr := fmt.Sprintf("/api/browse?dir=%s", url.QueryEscape(dir))
	if target != "" {
		urlStr += fmt.Sprintf("&target=%s", url.QueryEscape(target))
	}

	r.Method = "GET"
	http.Redirect(w, r, urlStr, http.StatusSeeOther)
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

func isRawExtension(ext string) bool {
	rawExts := map[string]bool{
		".nef": true, ".cr2": true, ".cr3": true, ".arw": true,
		".dng": true, ".raf": true, ".orf": true, ".rw2": true, ".srw": true,
	}
	return rawExts[strings.ToLower(ext)]
}

func extractRawPreview(path string) ([]byte, error) {
	var bestPreview []byte

	for _, tag := range []string{"-JpgFromRaw", "-PreviewImage", "-OtherImage", "-ThumbnailImage"} {
		cmd := exec.Command("exiftool", "-b", tag, path)
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			// If we found a high-res preview (> 100KB), return it immediately to save time
			if len(out) > 100000 {
				return out, nil
			}
			// Otherwise, keep it if it's the largest we've seen so far, but keep searching
			if len(out) > len(bestPreview) {
				bestPreview = out
			}
		}
	}

	if len(bestPreview) > 0 {
		return bestPreview, nil
	}
	return nil, fmt.Errorf("no preview found")
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	image := query.String(r, "path", "")
	if image == "" {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	if isRawExtension(filepath.Ext(image)) {
		// First try extracting the embedded high-quality JPEG
		if previewBytes, err := extractRawPreview(image); err == nil {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(previewBytes)
			return
		}

		// Fallback to bimg if exiftool fails
		buffer, err := os.ReadFile(image)
		if err == nil {
			options := bimg.Options{
				Type:     bimg.PNG,
				Quality:  100,
				Lossless: true,
			}
			if newImage, err := bimg.NewImage(buffer).Process(options); err == nil {
				w.Header().Set("Content-Type", "image/png")
				w.Write(newImage)
				return
			}
		}
	}

	ext := strings.ToLower(filepath.Ext(image))
	if ext == ".avif" || ext == ".avis" || ext == ".avifs" {
		w.Header().Set("Content-Type", "image/avif")
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
	flagFilter := r.FormValue("flagFilter")
	images, err := s.Service.GetAllImages(ctx, id, s.Config.DefaultSortBy, s.Config.DefaultSortOrder, 0, s.Config.ImagesPerPage, flagFilter, "")
	if err != nil {
		message := "Failed to fetch images for new gallery"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	count, _ := s.Service.GetGalleryCount(ctx, id)

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)

	galleries, _ := s.Service.GetAllGalleries()
	data := map[string]any{
		"Images":    images,
		"Count":     count,
		"Name":      name,
		"Galleries": galleries,
	}

	if err = s.Template.ExecuteTemplate(w, "gallery.html", data); err != nil {
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

type Progress struct {
	GalleryName string
	Current     int
	Expected    int
	Percent     int
}

func progressesEqual(p1, p2 []Progress) bool {
	if len(p1) != len(p2) {
		return false
	}
	for i := range p1 {
		if p1[i].GalleryName != p2[i].GalleryName || p1[i].Current != p2[i].Current || p1[i].Expected != p2[i].Expected {
			return false
		}
	}
	return true
}

func (s *Server) getGlobalProgress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	galleries, err := s.Service.GetAllGalleries()
	if err != nil {
		http.Error(w, "Failed to get galleries", http.StatusInternalServerError)
		return
	}

	var progresses []Progress
	refreshImages := false
	hasActive := false

	check := func() (bool, []Progress, bool) {
		var p []Progress
		var hasAct bool
		var refresh bool
		for _, g := range galleries {
			expected := s.Service.ExpectedCount[g.ID]
			if expected > 0 {
				hasAct = true
				current, _ := s.Service.GetGalleryCount(ctx, g.ID)
				if current < expected {
					p = append(p, Progress{
						GalleryName: g.Name,
						Current:     current,
						Expected:    expected,
						Percent:     int(float64(current) / float64(expected) * 100),
					})
					if current > 0 {
						refresh = true
					}
				}
			}
		}
		return hasAct, p, refresh
	}

	hasActive, progresses, refreshImages = check()

	if hasActive {
		// Long polling: wait up to 10 seconds for a change
		for i := 0; i < 20; i++ {
			time.Sleep(500 * time.Millisecond)
			newActive, newProgresses, newRefresh := check()
			if !progressesEqual(progresses, newProgresses) || !newActive {
				// Wenn ein Import gerade abgeschlossen wurde (!newActive),
				// erzwingen wir ein letztes Refresh der Galerie, damit die neuen Bilder sichtbar sind.
				if hasActive && !newActive {
					newRefresh = true
				}

				hasActive = newActive
				progresses = newProgresses
				refreshImages = newRefresh
				break
			}
		}
		var triggers []string
		if hasActive {
			triggers = append(triggers, "check-progress")
		}
		if refreshImages {
			triggers = append(triggers, "refresh-images")
		}
		if len(triggers) > 0 {
			w.Header().Set("HX-Trigger", strings.Join(triggers, ", "))
		}
	} else {
		if refreshImages {
			w.Header().Set("HX-Trigger", "refresh-images")
		}
	}

	if err := s.Template.ExecuteTemplate(w, "progress.html", progresses); err != nil {
		slog.Error("progress cannot be loaded", slog.Any("error", err))
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

func (s *Server) cancelScan(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if id, err, ok := s.Service.GetGalleryID(name); err == nil && ok {
		s.Service.CancelImport(id)
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
		files, err := s.Service.GetGalleryFiles(ctx, name, "", "", -1, 10_000, "", "")
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
	name := r.PathValue("name")
	if imageID == "" {
		if name == "global" {
			s.handleGlobalDuplicates(w, r)
			return
		} else {
			http.Error(w, "Failed to get Image ID", http.StatusBadRequest)
		}
	}

	page := query.Int(r, "page", 1)

	galleryID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	thresholdStr := query.String(r, "threshold", "0.9")
	thresholdFloat, _ := strconv.ParseFloat(thresholdStr, 32)
	threshold := float32(thresholdFloat)

	images, err := s.Service.FindDublicates(ctx, imageID, galleryID, page-1, s.Config.ImagesPerPage, threshold)
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
		"Threshold":   thresholdFloat,
	}

	err = s.Template.ExecuteTemplate(w, "search-results.html", data)
	if err != nil {
		message := "Internal server error during rendering"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleGlobalDuplicates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	thresholdStr := query.String(r, "threshold", "0.1")
	thresholdFloat, _ := strconv.ParseFloat(thresholdStr, 64)

	_ = r.ParseForm()
	var galleryIDs []int
	selectedGalleries := make(map[int]bool)
	for _, idStr := range r.Form["galleries"] {
		if id, err := strconv.Atoi(idStr); err == nil {
			galleryIDs = append(galleryIDs, id)
			selectedGalleries[id] = true
		}
	}

	groups, err := s.Service.FindGlobalDuplicates(ctx, thresholdFloat, galleryIDs)
	if err != nil {
		message := "Failed to find duplicate Images"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	galleries, _ := s.Service.GetAllGalleries()

	data := map[string]any{
		"Groups":            groups,
		"Threshold":         thresholdFloat,
		"Galleries":         galleries,
		"SelectedGalleries": selectedGalleries,
	}

	if err := s.Template.ExecuteTemplate(w, "global-duplicates.html", data); err != nil {
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

	deleteDisk := true

	if err := s.Service.DeleteImage(ctx, image, deleteDisk); err != nil {
		message := "Failed to delete File"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"refresh-sidebar": "", "refresh-images": ""}`)
	w.WriteHeader(http.StatusOK)
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

	// Go's r.ParseForm() doesn't parse the body for DELETE requests.
	// If HTMX sends hx-vals in the body, we need to parse it manually.
	/*if r.Method == http.MethodDelete && r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
		body, _ := io.ReadAll(r.Body)
		parsedBody, _ := url.ParseQuery(string(body))
		for k, v := range parsedBody {
			r.Form[k] = v
		}
	}*/

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

	w.Header().Set("HX-Trigger", `{"refresh-sidebar": "", "refresh-images": ""}`)
	w.WriteHeader(http.StatusOK)
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

	folder := r.URL.Query().Get("folder")
	heading := name
	if folder != "" {
		heading = name + " / " + filepath.Base(folder)
	}

	galleries, _ := s.Service.GetAllGalleries()
	data := map[string]any{
		"Images":    nil,
		"Count":     count,
		"Name":      name,
		"Heading":   heading,
		"Folder":    folder,
		"Galleries": galleries,
	}

	// Bilder werden auf "nil" gesetzt, da sie erst später lazy geladen werden!
	if err = s.Template.ExecuteTemplate(w, "gallery.html", data); err != nil {
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

	flagFilter := r.FormValue("flagFilter")
	folderFilter := r.FormValue("folder")
	images, err := s.Service.GetAllImages(ctx, galleryID, sortBy, sortOrder, page-1, s.Config.ImagesPerPage, flagFilter, folderFilter)
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
		Count       int
		Folder      string
	}{
		GalleryName: name,
		Images:      images,
		CurrentPage: page,
		PrevPage:    page - 1,
		NextPage:    nextPage,
		HasNext:     nextPage != -1,
		LastPage:    lastPage,
		Count:       count,
		Folder:      folderFilter,
	}

	// Rendert nur die Bilder-Kacheln aus dem neuen Template
	if err := s.Template.ExecuteTemplate(w, "gallery-images.html", data); err != nil {
		slog.Error("Failed to render gallery images", slog.Any("error", err))
		http.Error(w, "Failed to render", http.StatusInternalServerError)
	}
}

func (s *Server) handleBatchAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := r.ParseForm()
	if err != nil {
		message := "Failed to parse Form"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	criteria := r.FormValue("criteria")
	action := r.FormValue("batch_action")
	sourceGalleryName := r.FormValue("source_gallery")

	var images []db.Image

	if criteria == "selected" {
		imageIDsStr := r.FormValue("image_ids")
		if imageIDsStr != "" {
			for _, id := range strings.Split(imageIDsStr, ",") {
				images = append(images, db.Image{ID: id})
			}
		}
	} else {
		flagFilter := "any"
		if strings.HasPrefix(criteria, "flag_") {
			flagFilter = strings.TrimPrefix(criteria, "flag_")
		}

		files, err := s.Service.GetGalleryFiles(ctx, sourceGalleryName, "", "", 0, 100_000, flagFilter, "")
		if err != nil {
			message := "Failed to get files for batch action"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
		images = files
	}

	if len(images) == 0 {
		w.Header().Set("HX-Trigger", `{"refresh-sidebar": "", "refresh-images": ""}`)
		w.WriteHeader(http.StatusOK)
		return
	}

	if action == "delete" || action == "delete_disk" {
		deleteDisk := action == "delete_disk"
		if err := s.Service.DeleteImages(ctx, images, deleteDisk); err != nil {
			message := "Failed to delete images in batch"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
	} else if action == "move" || action == "copy" {
		isNew := r.FormValue("is_new") == "true"
		var targetGalleryName string

		if isNew {
			targetGalleryName = r.FormValue("new_name")
			path := r.FormValue("path")

			if err := s.Service.CreateGallery(ctx, targetGalleryName, path); err != nil {
				message := "Failed to create new Gallery"
				slog.Error(message, slog.Any("error", err))
				http.Error(w, message, http.StatusInternalServerError)
				return
			}
		} else {
			targetGalleryName = r.FormValue("name")
		}

		targetID, err, ok := s.Service.GetGalleryID(targetGalleryName)
		if err != nil || !ok {
			message := "Failed to get target Gallery ID"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}

		if action == "move" {
			if err := s.Service.ChangeGallery(ctx, targetID, images); err != nil {
				message := "Failed to move images"
				slog.Error(message, slog.Any("error", err))
				http.Error(w, message, http.StatusInternalServerError)
				return
			}
		} else if action == "copy" {
			if err := s.Service.CopyToGallery(ctx, targetID, images); err != nil {
				message := "Failed to copy images"
				slog.Error(message, slog.Any("error", err))
				http.Error(w, message, http.StatusInternalServerError)
				return
			}
		}
	}

	w.Header().Set("HX-Trigger", `{"refresh-sidebar": "", "refresh-images": ""}`)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		if errors.Is(err, http.ErrNotMultipart) {
			if err := r.ParseForm(); err != nil {
				message := "Failed to parse form"
				slog.Error(message, slog.Any("error", err))
				http.Error(w, message, http.StatusBadRequest)
				return
			}
			s.textSearch(w, r)
			return
		}
		message := "Failed to parse form"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	_, _, err := r.FormFile("file")

	if err == nil {
		s.imageSearch(w, r)
		return
	}

	if errors.Is(err, http.ErrMissingFile) {
		s.textSearch(w, r)
		return
	}

	http.Error(w, "Failed to parse form", http.StatusBadRequest)
}

func (s *Server) textSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	search := r.FormValue("q")
	sortBy := r.FormValue("sortBy")
	if sortBy == "" {
		sortBy = s.Config.DefaultSortBy
	}
	sortOrder := r.FormValue("sortOrder")
	if sortOrder == "" {
		sortOrder = s.Config.DefaultSortOrder
	}
	page, _ := strconv.Atoi(r.FormValue("page"))
	if page < 1 {
		page = 1
	}
	// options := query.Strings(r, "op")

	name := r.PathValue("name")
	galleryID, err, ok := s.Service.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	thresholdStr := r.FormValue("threshold")
	if thresholdStr == "" {
		thresholdStr = "0.9"
	}
	thresholdFloat, _ := strconv.ParseFloat(thresholdStr, 32)
	threshold := float32(thresholdFloat)
	flagFilter := r.FormValue("flagFilter")

	images, err := s.Service.SearchImages(ctx, search, galleryID, sortBy, sortOrder, page-1, s.Config.ImagesPerPage, threshold, flagFilter)
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
		"Threshold":   thresholdFloat,
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

func (s *Server) imageSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	sortBy := r.FormValue("sortBy")
	if sortBy == "" {
		sortBy = s.Config.DefaultSortBy
	}
	sortOrder := r.FormValue("sortOrder")
	if sortOrder == "" {
		sortOrder = s.Config.DefaultSortOrder
	}
	page, _ := strconv.Atoi(r.FormValue("page"))
	if page < 1 {
		page = 1
	}
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

	thresholdStr := r.FormValue("threshold")
	if thresholdStr == "" {
		thresholdStr = "0.9"
	}
	thresholdFloat, _ := strconv.ParseFloat(thresholdStr, 32)
	threshold := float32(thresholdFloat)

	flagFilter := r.FormValue("flagFilter")
	images, err := s.Service.SearchImages64(ctx, file64, galleryID, sortBy, sortOrder, page-1, s.Config.ImagesPerPage, threshold, flagFilter)
	if err != nil {
		message := "Error during database query"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	returnTo := query.String(r, "returnTo", name)
	data := map[string]any{
		"Images":      images,
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
