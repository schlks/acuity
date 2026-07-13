// Package gallery contains the core business logic for managing
// photo galleries, including synchronizing local folders and
// orchestrating database updates.
package gallery

import (
	"acuity/pkg/db"
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/weaviate/weaviate/entities/models"

	"github.com/h2non/bimg"
)

//go:generate mockgen -source=service.go -destination=mock_test.go -package=gallery
type sService interface {
	InitTable() error
	InsertGallery(path string, name string) error
	RemoveGallery(id int) error
	GetGalleryByName(name string) (db.Gallery, error)
	GetGalleryByID(id int) (db.Gallery, error)
	GetAllGalleries() ([]db.Gallery, error)
	UpdateGallery(id int, name string) error
}

type wService interface {
	ImportImages(ctx context.Context, filePaths []string, galleryID int, progressCallback func(count int))
	RemoveGalleryImages(ctx context.Context, galleryID int) error
	RemoveImage(ctx context.Context, image db.Image) error
	WriteBatchDB(ctx context.Context, batch []*models.Object) int
	SearchImage(ctx context.Context, search string, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int, threshold float32, flagFilter string) ([]db.Image, error)
	SearchImage64(ctx context.Context, image string, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int, threshold float32, flagFilter string) ([]db.Image, error)
	FindDublicates(ctx context.Context, imageID string, galleryID int, page int, imagesPerPage int, threshold float32) ([]db.Image, error)
	ChangeGallery(ctx context.Context, newID int, images []db.Image) error
	CopyToGallery(ctx context.Context, newGalleryID int, images []db.Image) error
	RemoveImages(ctx context.Context, images []db.Image) error
	GetAll(ctx context.Context, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int, flagFilter string, folderFilter string) ([]db.Image, error)
	GetKnownPaths(ctx context.Context, galleryID int) (map[string]string, error)
	GetInfo(ctx context.Context, imageID string) (db.Image, error)
	GetGalleryCount(ctx context.Context, galleryID int) (int, error)
	GetVectors(ctx context.Context, galleryIDs []int) ([]db.ImageVector, error)
	ResetDatabase(ctx context.Context) error
	SetRating(ctx context.Context, imageID string, rating int) error
	SetFlag(ctx context.Context, imageID string, flag int) error
}

type GalleryService struct {
	sDB           sService
	wDB           wService
	ExpectedCount map[int]int
	CurrentCount  map[int]int
	cancelFuncs   map[int]context.CancelFunc
	mu            sync.Mutex
}

func NewService(sDB *db.SQLiteClient, wDB *db.WeaviateClient) *GalleryService {
	return &GalleryService{
		sDB:           sDB,
		wDB:           wDB,
		ExpectedCount: make(map[int]int),
		CurrentCount:  make(map[int]int),
		cancelFuncs:   make(map[int]context.CancelFunc),
	}
}

func (g *GalleryService) CancelImport(galleryID int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if cancel, exists := g.cancelFuncs[galleryID]; exists {
		cancel()
		delete(g.cancelFuncs, galleryID)
	}
}

func (g *GalleryService) GetAllGalleries() ([]db.Gallery, error) {
	return g.sDB.GetAllGalleries()
}

func (g *GalleryService) getKnownFilePaths(known map[string]string, folderPath string) ([]string, []string, error) {
	var filePaths []string

	// Create a copy of known to track which ones we've seen
	seen := make(map[string]bool)
	for path := range known {
		seen[path] = false
	}

	err := filepath.WalkDir(folderPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		if !d.IsDir() {
			if _, exists := known[absPath]; !exists {
				filePaths = append(filePaths, absPath)
			} else {
				seen[absPath] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	var missingIds []string
	for path, wasSeen := range seen {
		if !wasSeen {
			missingIds = append(missingIds, known[path])
		}
	}

	return filePaths, missingIds, nil
}

func (g *GalleryService) CreateGallery(ctx context.Context, name string, folderPath string) error {
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		return err
	}

	if err := g.sDB.InsertGallery(folderPath, name); err != nil {
		return err
	}
	if err := g.UpdateFolder(ctx, name); err != nil {
		return err
	}

	return nil
}

func (g *GalleryService) DeleteGallery(ctx context.Context, id int) error {
	if err := g.sDB.RemoveGallery(id); err != nil {
		return err
	}
	if err := g.wDB.RemoveGalleryImages(ctx, id); err != nil {
		return err
	}

	return nil
}

func (g *GalleryService) UpdateFolder(ctx context.Context, name string) error {
	gallery, err := g.sDB.GetGalleryByName(name)
	if err != nil {
		return err
	}

	known, err := g.wDB.GetKnownPaths(ctx, gallery.ID)
	if err != nil {
		return err
	}

	filePaths, missingIds, err := g.getKnownFilePaths(known, gallery.Path)
	if err != nil {
		return err
	}

	currentCount, _ := g.wDB.GetGalleryCount(ctx, gallery.ID)
	g.ExpectedCount[gallery.ID] = currentCount + len(filePaths) - len(missingIds)

	importCtx, cancel := context.WithCancel(context.Background())
	g.mu.Lock()
	g.cancelFuncs[gallery.ID] = cancel
	g.CurrentCount[gallery.ID] = currentCount
	g.mu.Unlock()

	go func() {
		defer func() {
			g.ExpectedCount[gallery.ID] = 0 // Reset when done to prevent infinite UI polling
			g.mu.Lock()
			delete(g.cancelFuncs, gallery.ID)
			g.mu.Unlock()
			cancel()
		}()
		if len(missingIds) > 0 {
			var imagesToDelete []db.Image
			for _, id := range missingIds {
				imagesToDelete = append(imagesToDelete, db.Image{ID: id})
			}
			// Delete from DB without touching the disk (since they are already missing)
			_ = g.DeleteImages(importCtx, imagesToDelete, false)
		}
		if len(filePaths) > 0 {
			g.wDB.ImportImages(importCtx, filePaths, gallery.ID, func(count int) {
				g.mu.Lock()
				if _, ok := g.CurrentCount[gallery.ID]; ok {
					g.CurrentCount[gallery.ID] += count
				}
				g.mu.Unlock()
			})
		}
	}()
	return nil
}

func (g *GalleryService) GetGalleryID(name string) (int, error, bool) {
	if name == "global" {
		return -1, nil, true
	}

	gallery, err := g.sDB.GetGalleryByName(name)
	if err != nil {
		return 0, err, false
	}

	return gallery.ID, nil, true
}

func (g *GalleryService) GetGalleryFiles(ctx context.Context, name string, sortBy string, sortOrder string, page int, imagesPerPage int, flagFilter string, folderFilter string) ([]db.Image, error) {
	gallery, err := g.sDB.GetGalleryByName(name)
	if err != nil {
		return nil, err
	}
	files, err := g.wDB.GetAll(ctx, gallery.ID, sortBy, sortOrder, page, imagesPerPage, flagFilter, folderFilter)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func (g *GalleryService) SearchImages(ctx context.Context, search string, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int, threshold float32, flagFilter string) ([]db.Image, error) {
	return g.wDB.SearchImage(ctx, search, galleryID, sortBy, sortOrder, page, imagesPerPage, threshold, flagFilter)
}

func (g *GalleryService) SearchImages64(ctx context.Context, search string, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int, threshold float32, flagFilter string) ([]db.Image, error) {
	return g.wDB.SearchImage64(ctx, search, galleryID, sortBy, sortOrder, page, imagesPerPage, threshold, flagFilter)
}

func (g *GalleryService) GetAllImages(ctx context.Context, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int, flagFilter string, folderFilter string) ([]db.Image, error) {
	return g.wDB.GetAll(ctx, galleryID, sortBy, sortOrder, page, imagesPerPage, flagFilter, folderFilter)
}

func (g *GalleryService) ConvertImage(file []byte) (string, error) {
	bimgImg := bimg.NewImage(file)
	jpegBuffer, err := bimgImg.Convert(bimg.JPEG)
	if err != nil {
		return "", err
	}

	base64Image := base64.StdEncoding.EncodeToString(jpegBuffer)

	return base64Image, nil
}

func (g *GalleryService) DeleteImage(ctx context.Context, file db.Image, deleteDisk bool) error {
	if deleteDisk && file.Path == "" {
		info, err := g.wDB.GetInfo(ctx, file.ID)
		if err == nil {
			file.Path = info.Path
		}
	}

	if err := g.wDB.RemoveImage(ctx, file); err != nil {
		return err
	}

	if deleteDisk && file.Path != "" {
		err := os.Remove(file.Path)
		if err != nil {
			slog.Warn("Failed to delete physical file", slog.String("path", file.Path), slog.Any("error", err))
		}
	}
	return nil
}

func (g *GalleryService) DeleteImages(ctx context.Context, images []db.Image, deleteDisk bool) error {
	slog.Info("Deleting from disk: %v", deleteDisk)
	var paths []string
	if deleteDisk {
		for _, img := range images {
			if img.Path != "" {
				paths = append(paths, img.Path)
			} else {
				info, err := g.wDB.GetInfo(ctx, img.ID)
				if err == nil && info.Path != "" {
					paths = append(paths, info.Path)
				}
			}
		}
	}

	if err := g.wDB.RemoveImages(ctx, images); err != nil {
		return err
	}

	if deleteDisk {
		for _, path := range paths {
			slog.Info("Deleting physical file: %s", path)
			err := os.Remove(path)
			if err != nil {
				slog.Warn("Failed to delete physical file", slog.String("path", path), slog.Any("error", err))
			}
		}
	}
	return nil
}

func (g *GalleryService) FindDublicates(ctx context.Context, imageID string, galleryID int, page int, imagesPerPage int, threshold float32) ([]db.Image, error) {
	return g.wDB.FindDublicates(ctx, imageID, galleryID, page, imagesPerPage, threshold)
}

func (g *GalleryService) ChangeGallery(ctx context.Context, newID int, images []db.Image) error {
	targetGallery, err := g.sDB.GetGalleryByID(newID)
	if err != nil {
		return err
	}

	for i, img := range images {
		info, err := g.wDB.GetInfo(ctx, img.ID)
		if err == nil && info.Path != "" {
			fileName := filepath.Base(info.Path)
			newPath := filepath.Join(targetGallery.Path, fileName)

			os.MkdirAll(targetGallery.Path, 0755)

			if err := os.Rename(info.Path, newPath); err == nil {
				images[i].Path = newPath
			} else {
				// cross-device fallback
				input, err := os.ReadFile(info.Path)
				if err == nil {
					if err := os.WriteFile(newPath, input, 0644); err == nil {
						os.Remove(info.Path)
						images[i].Path = newPath
					} else {
						return fmt.Errorf("failed to write file %s: %w", newPath, err)
					}
				} else {
					return fmt.Errorf("failed to read file %s: %w", info.Path, err)
				}
			}
		}
	}

	return g.wDB.ChangeGallery(ctx, newID, images)
}

func (g *GalleryService) CopyToGallery(ctx context.Context, newID int, images []db.Image) error {
	targetGallery, err := g.sDB.GetGalleryByID(newID)
	if err != nil {
		return err
	}

	for i, img := range images {
		info, err := g.wDB.GetInfo(ctx, img.ID)
		if err == nil && info.Path != "" {
			fileName := filepath.Base(info.Path)
			newPath := filepath.Join(targetGallery.Path, fileName)

			os.MkdirAll(targetGallery.Path, 0755)

			input, err := os.ReadFile(info.Path)
			if err == nil {
				if err := os.WriteFile(newPath, input, 0644); err == nil {
					images[i].Path = newPath
				} else {
					return fmt.Errorf("failed to write copy %s: %w", newPath, err)
				}
			} else {
				return fmt.Errorf("failed to read source %s: %w", info.Path, err)
			}
		}
	}

	return g.wDB.CopyToGallery(ctx, newID, images)
}

func (g *GalleryService) GetImageInfo(ctx context.Context, imageID string) (map[string]any, error) {
	wInfo, err := g.wDB.GetInfo(ctx, imageID)
	if err != nil {
		return nil, err
	}

	/*
		f, err := os.Open(wInfo.Path)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = f.Close()
		}()

		exif.RegisterParsers(mknote.All...)

		x, err := exif.Decode(f)
		if err != nil {
			return nil, err
		}

		camera, _ := x.Get(exif.Model)
	*/ // INFO: Maybe later

	baseName := filepath.Base(wInfo.Path)
	imageInfo := map[string]any{
		"ID":            wInfo.ID,
		"Name":          strings.TrimSuffix(baseName, filepath.Ext(baseName)),
		"Path":          wInfo.Path,
		"GalleryID":     wInfo.GalleryID,
		"Rating":        wInfo.Rating,
		"Time":          wInfo.Taken, // Genutzt von Weaviate
		"Size":          wInfo.Size,
		"Resolution":    wInfo.Resolution,
		"AspectRatio":   wInfo.AspectRatio,
		"Extension":     wInfo.Extension,
		"Date":          wInfo.Date,
		"Flag":          wInfo.Flag,
		"CameraDetails": wInfo.CameraDetails,
		"Gallery":       "",
	}
	galleryStr := imageInfo["GalleryID"].(string)
	gallery, err := strconv.Atoi(galleryStr)
	if err != nil {
		return nil, err
	}
	galleryName, err := g.sDB.GetGalleryByID(gallery)
	if err != nil {
		imageInfo["Gallery"] = "Unknown"
	} else {
		imageInfo["Gallery"] = galleryName.Name
	}
	return imageInfo, nil
}

func (g *GalleryService) ResetDatabase(ctx context.Context) error {
	return g.wDB.ResetDatabase(ctx)
}

func (g *GalleryService) GetGalleryCount(ctx context.Context, galleryID int) (int, error) {
	g.mu.Lock()
	expected := g.ExpectedCount[galleryID]
	current, hasCurrent := g.CurrentCount[galleryID]
	g.mu.Unlock()

	if expected > 0 && hasCurrent {
		return current, nil
	}
	return g.wDB.GetGalleryCount(ctx, galleryID)
}

func (g *GalleryService) SetRating(ctx context.Context, imageID string, rating int) error {
	return g.wDB.SetRating(ctx, imageID, rating)
}

func (g *GalleryService) SetFlag(ctx context.Context, imageID string, rating int) error {
	return g.wDB.SetFlag(ctx, imageID, rating)
}

func (g *GalleryService) EditGalleryName(id int, newName string) error {
	return g.sDB.UpdateGallery(id, newName)
}

func (g *GalleryService) FindGlobalDuplicates(ctx context.Context, threshold float64, galleryIDs []int) ([][]db.ImageVector, error) {
	allImages, err := g.wDB.GetVectors(ctx, galleryIDs)
	if err != nil {
		return nil, err
	}

	var groups [][]db.ImageVector
	visited := make(map[string]bool)

	for i, imgA := range allImages {
		if visited[imgA.ID] {
			continue
		}

		var currentGroup []db.ImageVector

		for j := i + 1; j < len(allImages); j++ {
			imgB := allImages[j]
			if visited[imgB.ID] {
				continue
			}

			distance := 1.0 - cosineSimilarity(imgA.Vector, imgB.Vector)
			if distance <= threshold {
				if len(currentGroup) == 0 {
					currentGroup = append(currentGroup, imgA)
				}
				currentGroup = append(currentGroup, imgB)
				visited[imgB.ID] = true
			}
		}

		if len(currentGroup) > 0 {
			visited[imgA.ID] = true
			groups = append(groups, currentGroup)
		}
	}

	return groups, nil
}

func cosineSimilarity(a, b []float64) float64 {
	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normB) * math.Sqrt(normB))
}
