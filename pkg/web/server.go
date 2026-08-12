// Package web integrates the
// handlers for the webclient
package web

import (
	"acuity/pkg/config"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

	"acuity/pkg/db"

	"github.com/h2non/bimg"
	"github.com/mallardduck/go-http-helpers/pkg/query"
	_ "github.com/weaviate/weaviate/entities/models"
)

type Server struct {
	Bridge *db.Bridge
	Config *config.Config
}

type Progress struct {
	GalleryName string
	Current     int
	Expected    int
	Percent     int
}

type searchResult struct {
	GalleryName      string       `json:"gallery_name,omitempty"`
	Images           []db.SImage  `json:"images,omitempty"`
	QueryImage       db.ImageInfo `json:"query_image"`
	Query            string       `json:"query,omitempty"`
	Threshold        float64      `json:"threshold,omitempty"`
	CurrentPage      int          `json:"current_page"`
	PrevPage         int          `json:"prev_page,omitempty"`
	NextPage         int          `json:"next_page,omitempty"`
	HasNext          bool         `json:"has_page,omitempty"`
	LastPage         int          `json:"last_page,omitempty"`
	Count            int          `json:"count,omitempty"`
	Folder           string       `json:"folder,omitempty"`
	IsInfiniteAppend bool         `json:"is_inf_append,omitempty"`
	ReturnTo         string       `json:"return_to,omitempty"`
	SearchType       string       `json:"search-type,omitempty"`
	Galleries        []db.Gallery `json:"galleries"`
}

// NewServer creates a new server
func NewServer(sdatabase *db.SQLiteClient, wdatabase *db.WeaviateClient, config *config.Config) *Server {
	service := db.NewBridge(sdatabase, wdatabase)
	server := &Server{
		Bridge: service,
		Config: config,
	}

	// Background task for periodic gallery scans (every hour)
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			galleries, err := service.GetAllGalleries()
			if err == nil {
				for _, g := range galleries {
					slog.Info("Running periodic scan for gallery", slog.String("name", g.Name))
					_ = service.UpdateFolder(g.Name)
				}
			}
		}
	}()

	return server
}

// RegisterRoutes registers all the routes used by the webui
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "web"
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(webDir, "static")))))
	mux.HandleFunc("GET	/api/root", s.handleRoot)
	mux.HandleFunc("GET	/api/settings", s.getSettings)
	mux.HandleFunc("POST	/api/settings", s.setSettings)
	mux.HandleFunc("GET	/api/image", s.handleImage)
	mux.HandleFunc("POST 	/api/image/{id}/delete", s.deleteFiles)
	mux.HandleFunc("GET	/api/image/{id}", s.getInfo)
	mux.HandleFunc("POST	/api/image/{id}", s.setRating)
	mux.HandleFunc("POST 	/api/image/{id}/flag", s.setFlag)
	mux.HandleFunc("POST 	/api/image/delete", s.deleteFiles)
	mux.HandleFunc("GET	/api/gallery/{name}/duplicates", s.handleDuplicates)
	mux.HandleFunc("POST	/api/gallery/{name}/search", s.handleSearch)
	mux.HandleFunc("POST	/api/gallery/{name}/scan", s.scanGallery)
	mux.HandleFunc("DELETE /api/gallery/{name}/scan", s.cancelScan)
	mux.HandleFunc("POST 	/api/gallery/{name}/edit", s.editGallery)
	mux.HandleFunc("POST 	/api/gallery/{name}/unflag", s.unflagGallery)
	mux.HandleFunc("GET	/api/gallery/{name}", s.getGallery)
	mux.HandleFunc("GET	/api/gallery/{name}/count", s.handleGalleryCount)
	mux.HandleFunc("GET	/api/gallery/{name}/images", s.getGalleryImages)
	mux.HandleFunc("POST	/api/gallery", s.createGallery)
	mux.HandleFunc("DELETE /api/gallery/{name}", s.deleteGallery)
	mux.HandleFunc("POST 	/api/gallery/batch", s.handleBatchAction)
	mux.HandleFunc("GET 	/api/progress", s.getGlobalProgress)
	mux.HandleFunc("GET 	/api/browse", s.browseFiles)
	mux.HandleFunc("POST 	/api/browse/mkdir", s.mkdir)
	mux.HandleFunc("POST 	/api/reset", s.resetDatabase)

	uiBuildDir := filepath.Join(webDir, "build")
	uiFS := http.FileServer(http.Dir(uiBuildDir))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(uiBuildDir, r.URL.Path)
		_, err := os.Stat(path)

		if os.IsNotExist(err) || r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(uiBuildDir, "index.html"))
			return
		}

		uiFS.ServeHTTP(w, r)
	})
}

type SubFolder struct {
	Name       string       `json:"name"`
	Path       string       `json:"path"`
	SubFolders []*SubFolder `json:"sub_folders"`
}

type GalleryView struct {
	db.Gallery
	SubFolders []*SubFolder `json:"sub_folders"`
}

func (s *Server) writeJSON(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		message := "Failed encoding JSON response"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
}

func (s *Server) getPaginationAndSort(r *http.Request) (int, string, string) {
	page, _ := strconv.Atoi(r.FormValue("page"))
	if page < 1 {
		page, _ = strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
	}
	sortBy := r.FormValue("sortBy")
	if sortBy == "" {
		sortBy = r.URL.Query().Get("sortBy")
		if sortBy == "" {
			sortBy = s.Config.DefaultSortBy
		}
	}
	sortOrder := r.FormValue("sortOrder")
	if sortOrder == "" {
		sortOrder = r.URL.Query().Get("sortOrder")
		if sortOrder == "" {
			sortOrder = s.Config.DefaultSortOrder
		}
	}
	return page, sortBy, sortOrder
}

func (s *Server) resolveGallery(r *http.Request) (string, int, error) {
	name := r.PathValue("name")
	if formTarget := r.FormValue("target_gallery"); formTarget != "" {
		name = formTarget
	}
	id, err, ok := s.Bridge.GetGalleryID(name)
	if err != nil || !ok {
		return name, 0, fmt.Errorf("failed to get gallery ID for %s", name)
	}
	return name, id, nil
}

func (s *Server) parseThreshold(r *http.Request, defaultVal string) (float32, float64) {
	tStr := r.FormValue("threshold")
	if tStr == "" {
		tStr = query.String(r, "threshold", defaultVal)
	}
	tFloat, _ := strconv.ParseFloat(tStr, 64)
	return float32(tFloat), tFloat
}

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	galleries, err := s.Bridge.GetAllGalleries()
	if err != nil {
		slog.Error("Failed to load galleries", slog.Any("error", err))
	}

	var galleryViews []GalleryView
	for _, g := range galleries {
		gv := GalleryView{Gallery: g}
		nodeMap := make(map[string]*SubFolder)
		err := filepath.WalkDir(g.Path, func(path string, d fs.DirEntry, err error) error {
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
		if err != nil {
			return
		}
		galleryViews = append(galleryViews, gv)
	}

	data := struct {
		Galleries []GalleryView  `json:"galleries"`
		Config    *config.Config `json:"config"`
	}{
		Galleries: galleryViews,
		Config:    s.Config,
	}

	s.writeJSON(w, data, http.StatusOK)
}

func (s *Server) resetDatabase(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	err := s.Bridge.ResetDatabase(ctx)
	if err != nil {
		slog.Error("Failed to reset database", slog.Any("error", err))
		http.Error(w, "Failed to reset database", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) setFlag(w http.ResponseWriter, r *http.Request) {
	imageIDStr := r.PathValue("id")
	imageID, err := strconv.Atoi(imageIDStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	flagStr := r.FormValue("flag")
	flag, _ := strconv.Atoi(flagStr)

	if err := s.Bridge.SetFlag(imageID, flag); err != nil {
		message := "Failed to set Flag"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	data := struct {
		ID   string `json:"id"`
		Flag int    `json:"flag"`
	}{ID: imageIDStr, Flag: flag}

	s.writeJSON(w, data, http.StatusOK)
}

func (s *Server) unflagGallery(w http.ResponseWriter, r *http.Request) {
	galleryName := r.PathValue("name")
	if err := s.Bridge.Unflag(galleryName); err != nil {
		message := "Failed to unflag all images in gallery"
		slog.Error(message, slog.String("gallery", galleryName), slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", "refresh-images")
	w.WriteHeader(http.StatusOK)
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
		CurrentDir string   `json:"current_dir"`
		ParentDir  string   `json:"parent_dir"`
		Folders    []string `json:"folders"`
		Target     string   `json:"target"`
	}{
		CurrentDir: dir,
		ParentDir:  parentDir,
		Folders:    folders,
		Target:     target,
	}

	s.writeJSON(w, data, http.StatusOK)
}

func (s *Server) mkdir(w http.ResponseWriter, r *http.Request) {
	dir := r.FormValue("dir")
	newFolder := r.FormValue("new_folder")
	target := r.FormValue("target")

	if dir != "" && newFolder != "" {
		newPath := filepath.Join(dir, newFolder)
		err := os.MkdirAll(newPath, 0o755)
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
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.Config); err != nil {
		slog.Error("Failed to encode settings to JSON", slog.Any("error", err))
		http.Error(w, "Failed to encode settings", http.StatusInternalServerError)
		return
	}
}

func (s *Server) setSettings(w http.ResponseWriter, r *http.Request) {
	newConfig := *s.Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		slog.Error("Failed to decode JSON settings", slog.Any("error", err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	s.Config = &newConfig

	if err := config.Save(s.Config); err != nil {
		slog.Error("failed to save config", slog.Any("error", err))
	}

	w.WriteHeader(http.StatusOK)
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
			if len(out) > 100000 {
				return out, nil
			}
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

var thumbSemaphore = make(chan struct{}, 4)

func (s *Server) getOrCreateThumbnail(imagePath string, targetWidth int) (string, error) {
	cacheBase, err := os.UserCacheDir()
	if err != nil {
		cacheBase = filepath.Dir(s.Config.DBPath)
	}
	cacheDir := filepath.Join(cacheBase, "acuity", "thumbs")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}

	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%d", imagePath, targetWidth)))
	hashStr := hex.EncodeToString(h.Sum(nil))
	thumbPath := filepath.Join(cacheDir, hashStr+".jpg")

	sourceStat, err := os.Stat(imagePath)
	if err != nil {
		return "", err
	}

	if thumbStat, err := os.Stat(thumbPath); err == nil {
		if thumbStat.ModTime().After(sourceStat.ModTime()) {
			return thumbPath, nil
		}
	}

	thumbSemaphore <- struct{}{}
	defer func() { <-thumbSemaphore }()

	var buffer []byte
	if isRawExtension(filepath.Ext(imagePath)) {
		preview, err := extractRawPreview(imagePath)
		if err == nil {
			buffer = preview
		}
	}
	if len(buffer) == 0 {
		buf, err := os.ReadFile(imagePath)
		if err != nil {
			return "", err
		}
		buffer = buf
	}

	options := bimg.Options{
		Width:   targetWidth,
		Quality: 80,
		Type:    bimg.JPEG,
	}

	newImage, err := bimg.NewImage(buffer).Process(options)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(thumbPath, newImage, 0o644); err != nil {
		return "", err
	}

	return thumbPath, nil
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	image := query.String(r, "path", "")
	if image == "" {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	isThumb := r.URL.Query().Get("thumb") == "true"
	if isThumb {
		thumbPath, err := s.getOrCreateThumbnail(image, 500)
		if err == nil && thumbPath != "" {
			w.Header().Set("Cache-Control", "public, max-age=604800")
			w.Header().Set("Content-Type", "image/jpeg")
			http.ServeFile(w, r, thumbPath)
			return
		}
	}

	if isRawExtension(filepath.Ext(image)) {
		if previewBytes, err := extractRawPreview(image); err == nil {
			mimeType := http.DetectContentType(previewBytes)

			if mimeType == "image/tiff" {
				options := bimg.Options{Type: bimg.JPEG, Quality: 90}
				if convertedBytes, err := bimg.NewImage(previewBytes).Process(options); err == nil {
					previewBytes = convertedBytes
					mimeType = "image/jpeg"
				}
			}

			w.Header().Set("Content-Type", mimeType)
			_, err := w.Write(previewBytes)
			if err != nil {
				return
			}
			return
		}

		buffer, err := os.ReadFile(image)
		if err == nil {
			options := bimg.Options{
				Type:     bimg.PNG,
				Quality:  100,
				Lossless: true,
			}
			if newImage, err := bimg.NewImage(buffer).Process(options); err == nil {
				w.Header().Set("Content-Type", "image/png")
				_, err := w.Write(newImage)
				if err != nil {
					return
				}
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
	name := r.FormValue("name")
	path := r.FormValue("path")

	if err := s.Bridge.CreateGallery(name, path); err != nil {
		message := "Failed to create Gallery"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	id, err, ok := s.Bridge.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	flagFilter := r.FormValue("flagFilter")
	images, err := s.Bridge.GetAllImages(id, s.Config.DefaultSortBy, s.Config.DefaultSortOrder, 0, s.Config.ImagesPerPage, flagFilter, "")
	if err != nil {
		message := "Failed to fetch images for new gallery"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	count, _ := s.Bridge.GetGalleryCount(id)

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)

	galleries, _ := s.Bridge.GetAllGalleries()
	data := searchResult{
		Images:      images,
		Count:       count,
		GalleryName: name,
		Galleries:   galleries,
	}

	s.writeJSON(w, data, http.StatusOK)
}

func (s *Server) editGallery(w http.ResponseWriter, r *http.Request) {
	oldName := r.PathValue("name")
	newName := r.FormValue("name")

	id, err, ok := s.Bridge.GetGalleryID(oldName)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	if ok {
		if err := s.Bridge.EditGalleryName(id, newName); err != nil {
			message := "Failed to update Gallery"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
	}

	s.writeJSON(w, map[string]any{"success": true, "name": newName}, http.StatusOK)
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

	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	galleries, err := s.Bridge.GetAllGalleries()
	if err != nil {
		http.Error(w, "Failed to get galleries", http.StatusInternalServerError)
		return
	}

	lastProgressStr := r.URL.Query().Get("last")
	var lastProgress []Progress
	if lastProgressStr != "" {
		decoded, err := base64.StdEncoding.DecodeString(lastProgressStr)
		if err == nil {
			_ = json.Unmarshal(decoded, &lastProgress)
		}
	}

	check := func() (bool, []Progress, []string) {
		var p []Progress
		var hasAct bool
		var refreshGalleries []string
		for _, g := range galleries {
			expected := s.Bridge.GetExpectedCount(g.ID)
			if expected != 0 {
				hasAct = true
				current, _ := s.Bridge.GetGalleryCount(g.ID)
				if expected == -1 {
					p = append(p, Progress{
						GalleryName: g.Name,
						Current:     0,
						Expected:    -1,
						Percent:     0,
					})
				} else if current < expected {
					p = append(p, Progress{
						GalleryName: g.Name,
						Current:     current,
						Expected:    expected,
						Percent:     int(float64(current) / float64(expected) * 100),
					})
					if current > 0 {
						refreshGalleries = append(refreshGalleries, g.Name)
					}
				}
			}
		}
		return hasAct, p, refreshGalleries
	}

	hasActive, progresses, refreshGalleries := check()

	if !hasActive {
		return
	}

	for progressesEqual(progresses, lastProgress) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
			hasActive, progresses, refreshGalleries = check()
			if !hasActive {
				w.Header().Set("HX-Trigger", "reload-main")
				slog.Error("progress cannot be loaded", slog.Any("error", err))
				return
			}
		}
	}

	var triggers []string
	for _, Name := range refreshGalleries {
		triggers = append(triggers, "refresh-images-"+Name)
	}
	if len(triggers) > 0 {
		w.Header().Set("HX-Trigger", strings.Join(triggers, ", "))
	}

	jsonData, _ := json.Marshal(progresses)
	newLastStr := base64.StdEncoding.EncodeToString(jsonData)

	data := struct {
		Progresses []Progress `json:"progresses"`
		LastStr    string     `json:"last_str"`
	}{
		Progresses: progresses,
		LastStr:    newLastStr,
	}

	s.writeJSON(w, data, http.StatusOK)
}

func (s *Server) scanGallery(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	if err := s.Bridge.UpdateFolder(name); err != nil {
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
	if id, err, ok := s.Bridge.GetGalleryID(name); err == nil && ok {
		s.Bridge.CancelImport(id)
	}
	w.Header().Set("HX-Trigger", "check-progress")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) deleteGallery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := r.PathValue("name")
	deleteGallery := query.Bool(r, "deleteGallery", false)

	id, err, ok := s.Bridge.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery Id"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	if err := s.Bridge.DeleteGallery(ctx, id); err != nil {
		message := "Failed to delete Gallery from Database"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	if deleteGallery {
		files, err := s.Bridge.GetGalleryFiles(name, "", "", -1, 10_000, "", "")
		if err != nil {
			message := "Failed to get Files in Gallery"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
		if err := s.Bridge.DeleteImages(ctx, files); err != nil {
			message := "Failed to delete Gallery"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Trigger", "refresh-sidebar")
	w.Header().Set("HX-Redirect", "/")
}

func (s *Server) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	imageIDStr := r.FormValue("imageID")
	if imageIDStr == "" {
		s.handleGlobalDuplicates(w, r)
		return
	}

	imageID, err := strconv.Atoi(imageIDStr)
	if err != nil {
		http.Error(w, "Invalid Image ID", http.StatusBadRequest)
		return
	}

	page, _, _ := s.getPaginationAndSort(r)
	name, galleryID, err := s.resolveGallery(r)
	if err != nil {
		slog.Error("Failed to get Gallery ID", slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	threshold, thresholdFloat := s.parseThreshold(r, "0.9")

	images, err := s.Bridge.FindDuplicates(ctx, imageID, galleryID, threshold)
	if err != nil {
		message := "Failed to find duplicate Images"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	queryImage, _ := s.Bridge.GetImageInfo(imageID)
	returnTo := query.String(r, "returnTo", name)
	galleries, _ := s.Bridge.GetAllGalleries()

	data := searchResult{
		Images:      images,
		QueryImage:  queryImage,
		Query:       strconv.Itoa(imageID),
		CurrentPage: page,
		PrevPage:    page - 1,
		NextPage:    page + 1,
		HasNext:     len(images) == s.Config.ImagesPerPage,
		GalleryName: name,
		ReturnTo:    returnTo,
		SearchType:  "duplicate",
		Threshold:   thresholdFloat,
		Galleries:   galleries,
	}

	s.writeJSON(w, data, http.StatusOK)
}

// TODO: change to json
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

	name := r.PathValue("name")
	if len(galleryIDs) == 0 && name != "" && name != "global" {
		if id, err, ok := s.Bridge.GetGalleryID(name); err == nil && ok {
			galleryIDs = append(galleryIDs, id)
			selectedGalleries[id] = true
		}
	}

	groups, err := s.Bridge.FindGlobalDuplicates(ctx, thresholdFloat, galleryIDs)
	if err != nil {
		message := "Failed to find duplicate Images"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	galleries, _ := s.Bridge.GetAllGalleries()

	data := map[string]any{
		"Groups":            groups,
		"Threshold":         thresholdFloat,
		"Galleries":         galleries,
		"SelectedGalleries": selectedGalleries,
	}

	s.writeJSON(w, data, http.StatusOK)
}

// Deprecated
func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var image db.SImage
	idstr := r.PathValue("id")
	id, err := strconv.Atoi(idstr)
	if err != nil {
		message := "Failed to convert id"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	image.ID = id
	if image.ID > 0 {
		idstr := r.FormValue("id")
		id, err := strconv.Atoi(idstr)
		if err != nil {
			message := "Failed to convert id"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
		image.ID = id
	}
	image.FilePath = r.FormValue("path")

	if err := s.Bridge.DeleteImage(ctx, image); err != nil {
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

	if r.Method == http.MethodPost {
		imageIDsStr := r.FormValue("image_id")
		var images []db.SImage

		for id := range strings.SplitSeq(imageIDsStr, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				idInt, _ := strconv.Atoi(id)
				images = append(images, db.SImage{ID: idInt})
			}
		}

		if err := s.Bridge.DeleteImages(ctx, images); err != nil {
			message := "Failed to delete Image"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Trigger", `{"refresh-sidebar": "", "refresh-images": ""}`)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) getGallery(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	galleryID, err, ok := s.Bridge.GetGalleryID(name)
	if err != nil && !ok {
		message := "Failed to get Gallery ID"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	count, err := s.Bridge.GetGalleryCount(galleryID)
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

	galleries, _ := s.Bridge.GetAllGalleries()
	data := struct {
		Count     int          `json:"count"`
		Name      string       `json:"name"`
		Heading   string       `json:"heading"`
		Folder    string       `json:"Folder"`
		Galleries []db.Gallery `json:"galleries"`
	}{
		Count:     count,
		Name:      name,
		Heading:   heading,
		Folder:    folder,
		Galleries: galleries,
	}

	s.writeJSON(w, data, http.StatusOK)
}

func (s *Server) getGalleryImages(w http.ResponseWriter, r *http.Request) {
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

	galleryID, err, ok := s.Bridge.GetGalleryID(name)
	if err != nil && !ok {
		http.Error(w, "Failed to get Gallery ID", http.StatusInternalServerError)
		return
	}

	flagFilter := r.FormValue("flagFilter")
	folderFilter := r.FormValue("folder")
	start := time.Now()
	images, err := s.Bridge.GetAllImages(galleryID, sortBy, sortOrder, page-1, s.Config.ImagesPerPage, flagFilter, folderFilter)
	slog.Info("GetAllImages took", slog.Duration("duration", time.Since(start)))
	if err != nil {
		slog.Error("Failed to fetch images", slog.Any("error", err))
		http.Error(w, "Images not found", http.StatusNotFound)
		return
	}

	nextPage := -1
	if len(images) == s.Config.ImagesPerPage {
		nextPage = page + 1
	}

	count := -1
	lastPage := 1
	if !s.Config.InfiniteScroll {
		count, _ = s.Bridge.GetGalleryCount(galleryID)
		lastPage = count / s.Config.ImagesPerPage
		if count%s.Config.ImagesPerPage != 0 {
			lastPage++
		}
		if lastPage == 0 {
			lastPage = 1
		}
	}

	data := searchResult{
		GalleryName:      name,
		Images:           images,
		CurrentPage:      page,
		PrevPage:         page - 1,
		NextPage:         nextPage,
		HasNext:          nextPage != -1,
		LastPage:         lastPage,
		Count:            count,
		Folder:           folderFilter,
		IsInfiniteAppend: s.Config.InfiniteScroll && r.URL.Query().Get("infinite") == "true",
	}

	s.writeJSON(w, data, http.StatusOK)
}

func (s *Server) handleGalleryCount(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	galleryID, err, ok := s.Bridge.GetGalleryID(name)
	if err != nil && !ok {
		switch name {
		case "Global":
			galleryID = 0
		case "duplicates":
			galleryID = -1
		default:
			http.Error(w, "Gallery not found", http.StatusNotFound)
			return
		}
	}

	count, _ := s.Bridge.GetGalleryCount(galleryID)
	data := struct {
		Count int `json:"count"`
	}{
		Count: count,
	}

	s.writeJSON(w, data, http.StatusOK)
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
	var targetGalleryName string

	var images []db.SImage

	if criteria == "selected" {
		imageIDsStr := r.FormValue("image_ids")
		if imageIDsStr != "" {
			for id := range strings.SplitSeq(imageIDsStr, ",") {
				id = strings.TrimSpace(id)
				if id != "" {
					idInt, _ := strconv.Atoi(id)
					images = append(images, db.SImage{ID: idInt})
				}
			}
		}
	} else {
		flagFilter, ok := strings.CutPrefix(criteria, "flag_")
		if !ok {
			message := "Failed to parse Form"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusBadRequest)
			return
		}

		files, err := s.Bridge.GetGalleryFiles(sourceGalleryName, "", "", 0, 100_000, flagFilter, "")
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
		if err := s.Bridge.DeleteImages(ctx, images); err != nil {
			message := "Failed to delete images in batch"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}
	} else if action == "move" || action == "copy" {
		targetGalleryName = r.FormValue("name")

		targetID, err, ok := s.Bridge.GetGalleryID(targetGalleryName)
		if err != nil || !ok {
			message := "Failed to get target Gallery ID"
			slog.Error(message, slog.Any("error", err))
			http.Error(w, message, http.StatusInternalServerError)
			return
		}

		if action == "move" {
			if err := s.Bridge.ChangeGallery(ctx, targetID, images); err != nil {
				message := "Failed to move images"
				slog.Error(message, slog.Any("error", err))
				http.Error(w, message, http.StatusInternalServerError)
				return
			}
		} else {
			if err := s.Bridge.CopyToGallery(ctx, targetID, images); err != nil {
				message := "Failed to copy images"
				slog.Error(message, slog.Any("error", err))
				http.Error(w, message, http.StatusInternalServerError)
				return
			}
		}
	}
	if err := s.Bridge.UpdateFolder(targetGalleryName); err != nil {
		message := "Failed to update target gallery"
		slog.Error(message, slog.String("gallery", targetGalleryName), slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

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

	data, _ := json.Marshal(struct{}{})
	s.writeJSON(w, data, http.StatusBadRequest)
}

func (s *Server) textSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	search := r.FormValue("q")
	page, _, _ := s.getPaginationAndSort(r)
	name, galleryID, err := s.resolveGallery(r)
	if err != nil {
		slog.Error("Failed to get Gallery ID", slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	threshold, thresholdFloat := s.parseThreshold(r, "0.9")

	images, err := s.Bridge.SearchImages(ctx, search, galleryID, threshold)
	if err != nil {
		message := "Error during database query"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	returnTo := query.String(r, "returnTo", name)
	galleries, _ := s.Bridge.GetAllGalleries()

	data := searchResult{
		GalleryName: name,
		Images:      images,
		Query:       search,
		Threshold:   thresholdFloat,
		CurrentPage: page,
		PrevPage:    page - 1,
		NextPage:    page + 1,
		HasNext:     len(images) == s.Config.ImagesPerPage,
		ReturnTo:    returnTo,
		SearchType:  "text",
		Galleries:   galleries,
	}

	s.writeJSON(w, data, http.StatusOK)
}

func (s *Server) imageSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	page, _, _ := s.getPaginationAndSort(r)
	name, galleryID, err := s.resolveGallery(r)
	if err != nil {
		slog.Error("Failed to get Gallery ID", slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	file64, err := s.Bridge.ConvertImage(fileBytes)
	if err != nil {
		message := "Failed to convert Image to Base64"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	threshold, thresholdFloat := s.parseThreshold(r, "0.9")

	images, err := s.Bridge.SearchImages(ctx, file64, galleryID, threshold)
	if err != nil {
		message := "Error during database query"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	returnTo := query.String(r, "returnTo", name)
	galleries, _ := s.Bridge.GetAllGalleries()
	data := searchResult{
		Images:      images,
		CurrentPage: page,
		PrevPage:    page - 1,
		NextPage:    page + 1,
		HasNext:     len(images) == s.Config.ImagesPerPage,
		GalleryName: name,
		ReturnTo:    returnTo,
		SearchType:  "duplicate",
		Threshold:   thresholdFloat,
		Galleries:   galleries,
	}

	s.writeJSON(w, data, http.StatusOK)
}

func (s *Server) getInfo(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	data, err := s.Bridge.GetImageInfo(id)
	if err != nil {
		message := "Failed to get Image Info"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, data, http.StatusOK)
}

func (s *Server) setRating(w http.ResponseWriter, r *http.Request) {
	imageIDStr := r.PathValue("id")
	imageID, err := strconv.Atoi(imageIDStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ratingString := r.FormValue("rating")
	rating, err := strconv.Atoi(ratingString)
	if err != nil {
		message := "Failed to get Image rating"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	if err := s.Bridge.SetRating(imageID, rating); err != nil {
		message := "Failed to set Image rating"
		slog.Error(message, slog.Any("error", err))
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	data := struct {
		ID     string `json:"id"`
		Rating int    `json:"rating"`
	}{
		ID:     imageIDStr,
		Rating: rating,
	}

	s.writeJSON(w, data, http.StatusOK)
}
