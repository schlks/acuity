// Package gallery contains the core logic for managing
// photo galleries, including synchronizing local folders and
// orchestrating database updates
package gallery

import (
	"acuity/pkg/db"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/jpeg"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/buckket/go-blurhash"
	"github.com/evanoberholster/imagemeta"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/h2non/bimg"
	"github.com/pillowskiy/imagesize"
	"github.com/weaviate/weaviate/entities/models"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/floats"
)

type GalleryService struct {
	sDB           *db.SQLiteClient
	wDB           *db.WeaviateClient
	ExpectedCount map[int]int
	CurrentCount  map[int]int
	cancelFuncs   map[int]context.CancelFunc
	mu            sync.Mutex
}

type ImagePayload struct {
	Weaviate db.WImage
	SQLite   db.SImage
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

func (g *GalleryService) getSeenFilePaths(known []string, folderPath string) ([]string, []string, error) {
	var filePaths []string

	seen := make(map[string]bool)
	for _, path := range known {
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
			if slices.Contains(known, absPath) {
				seen[absPath] = true
			} else {
				filePaths = append(filePaths, absPath)
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	var missingPaths []string
	for path, wasSeen := range seen {
		if !wasSeen {
			missingPaths = append(missingPaths, path)
		}
	}

	return filePaths, missingPaths, nil
}

func (g *GalleryService) CreateGallery(ctx context.Context, name string, folderPath string) error {
	if err := os.MkdirAll(folderPath, 0o755); err != nil {
		return err
	}

	if err := g.sDB.InsertGallery(folderPath, name); err != nil {
		return nil
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

	g.mu.Lock()
	if g.ExpectedCount[gallery.ID] != 0 {
		g.mu.Unlock()
		return nil
	}
	g.ExpectedCount[gallery.ID] = -1
	g.mu.Unlock()

	go func() {
		defer func() {
			g.mu.Lock()
			g.ExpectedCount[gallery.ID] = 0
			delete(g.cancelFuncs, gallery.ID)
			g.mu.Unlock()
		}()

		known, err := g.sDB.GetKnownPaths(gallery.ID)
		if err != nil {
			slog.Error("Failed to get file paths", slog.Any("error", err))
		}

		filePaths, missingPaths, err := g.getSeenFilePaths(known, gallery.Path)
		if err != nil {
			slog.Error("Failed to get file paths", slog.Any("error", err))
			return
		}

		currentCount, _ := g.sDB.GetGalleryCount(gallery.ID)
		importCtx, cancel := context.WithCancel(context.Background())

		g.mu.Lock()
		g.ExpectedCount[gallery.ID] = currentCount + len(filePaths) - len(missingPaths)
		g.cancelFuncs[gallery.ID] = cancel

		g.CurrentCount[gallery.ID] = currentCount - len(missingPaths)
		g.mu.Unlock()

		defer cancel()

		if len(missingPaths) > 0 {
			var imagesToDelete []db.SImage
			for _, path := range missingPaths {
				imagesToDelete = append(imagesToDelete, db.SImage{FilePath: path, GalleryID: gallery.ID})
			}
			_ = g.DeleteImages(importCtx, imagesToDelete)
		}
		if len(filePaths) > 0 {
			g.ImportImages(importCtx, filePaths, gallery.ID, func(count int) {
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

func (g *GalleryService) ImportImages(ctx context.Context, filePaths []string, galleryID int, progressCallback func(count int)) {
	batchSize, threads := 100, runtime.NumCPU()
	maxWorkers := max(1, threads-2)

	resultChan, done := make(chan ImagePayload, batchSize), make(chan struct{})

	go func() {
		var wBatch []*models.Object
		var sBatch []db.SImage

		var amount int
		for result := range resultChan {
			id := uuid.NewMD5(uuid.NameSpaceURL, []byte(result.Weaviate.Path+strconv.Itoa(galleryID))).String()
			obj := &models.Object{
				ID:    strfmt.UUID(id),
				Class: "Image",
				Properties: map[string]any{
					"filepath":   result.Weaviate.Path,
					"image":      result.Weaviate.Base64,
					"gallery_id": galleryID,
				},
			}
			wBatch, sBatch = append(wBatch, obj), append(sBatch, result.SQLite)

			if len(wBatch) >= batchSize {
				failed := g.wDB.WriteBatchDB(ctx, wBatch)
				if err := g.sDB.InsertImage(sBatch); err != nil {
					slog.Error("Failed to insert SQLite batch", slog.Any("error", err))
				}
				amount = len(wBatch) - failed
				if progressCallback != nil {
					progressCallback(amount)
				}

				wBatch, sBatch = make([]*models.Object, 0, batchSize), make([]db.SImage, 0, batchSize)
				slog.Info("Wrote Images into the Databases", slog.Int("amount", amount))
			}
		}
		if len(wBatch) > 0 {
			failed := g.wDB.WriteBatchDB(ctx, wBatch)
			amount = len(wBatch) - failed
			if err := g.sDB.InsertImage(sBatch); err != nil {
				slog.Error("Failed to insert SQLite batch", slog.Any("error", err))
			}
			if progressCallback != nil {
				progressCallback(amount)
			}
		}
		gallery, err := g.sDB.GetGalleryByID(galleryID)
		if err != nil {
			slog.Error("Failed to gallery name", slog.Any("error", err))
		}

		count, err := g.sDB.GetGalleryCount(galleryID)
		if err != nil {
			slog.Error("Failed to gallery count", slog.Any("error", err))
		}

		slog.Info("Finished import", slog.String("gallery", gallery.Name), slog.Int("amount", count))
		close(done)
	}()

	eg := new(errgroup.Group)
	eg.SetLimit(maxWorkers)

	for _, path := range filePaths {
		eg.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			var sImage db.SImage
			var wImage db.WImage

			sImage.GalleryID = galleryID
			sImage.FilePath = path
			sImage.Extension = filepath.Ext(path)

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			stat, err := os.Stat(path)

			bimgImg := bimg.NewImage(data)
			var width, heigth int
			imageSize, err := imagesize.ExtractFileInfo(path)
			if err == nil {
				width = imageSize.Width
				heigth = imageSize.Height
			} else {
				slog.Error("failed to get image size with imagesize", slog.String("file", path), slog.Any("error", err))
			}

			var resolution int
			var aspectRatio float64
			if width > 0 && heigth > 0 {
				resolution = width * heigth
				aspectRatio = float64(width) / float64(heigth)
			}

			jpegBuffer, err := bimgImg.Convert(bimg.JPEG)
			if err != nil {
				return nil
			}

			smallImgOptions := bimg.Options{
				Width:  128,
				Height: 128,
				Crop:   false,
			}
			smallBuffer, err := bimgImg.Process(smallImgOptions)
			if err != nil {
				return nil
			}
			img, err := jpeg.Decode(bytes.NewReader(smallBuffer))
			if err != nil {
				return nil
			}
			bhash, _ := blurhash.Encode(4, 3, img)
			sImage.Blurhash = bhash

			sImage.Date = time.Now().Format(time.RFC3339)
			sImage.Taken = sImage.Date
			sImage.Size = float64(stat.Size())
			sImage.Resolution = resolution
			sImage.AspectRatio = aspectRatio

			if x, err := imagemeta.Decode(bytes.NewReader(data)); err == nil {
				tm := x.OriginalDate()
				if !tm.IsZero() {
					sImage.Taken = tm.Format(time.RFC3339)
				}
				sImage.CameraMake = x.CameraMake()
				sImage.LensMake = x.ExifIFD.LensMake
				sImage.FocalLength = x.ExifIFD.FocalLength.String()
				sImage.Aperture = x.ExifIFD.ApertureValue.String()
				sImage.ShutterSpeed = x.ExifIFD.ShutterSpeedValue.String()
				sImage.Iso = fmt.Sprintf("%v", x.ExifIFD.ISOSpeedRatings)
				sImage.Flash = x.ExifIFD.Flash.Fired()
			}

			wImage.Path = path
			wImage.Base64 = base64.StdEncoding.EncodeToString(jpegBuffer)

			resultChan <- ImagePayload{
				Weaviate: wImage,
				SQLite:   sImage,
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		slog.Error("Errors occured while importing images", slog.Any("error", err))
	}

	close(resultChan)
	<-done
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

func (g *GalleryService) GetGalleryFiles(name string, sortBy string, sortOrder string, page int, imagesPerPage int, flagFilter string, folderFilter string) ([]db.SImage, error) {
	galleryID, err, ok := g.GetGalleryID(name)
	if err != nil && !ok {
		return nil, err
	}
	files, err := g.sDB.GetAllImages(galleryID, sortBy, sortOrder, page, imagesPerPage, flagFilter, folderFilter)
	if err != nil {
		return nil, err
	}

	return files, err
}

func (g *GalleryService) SearchImages(ctx context.Context, search string, galleryID int, threshold float32) ([]db.SImage, error) {
	count, err := g.sDB.GetGalleryCount(galleryID)
	if err != nil {
		return nil, err
	}
	images, err := g.wDB.SearchImage(ctx, search, galleryID, count, threshold)
	if err != nil {
		return nil, err
	}
	var sImages []db.SImage
	for _, img := range images {
		sImage, err := g.sDB.GetImageByPath(img.Path)
		if err != nil {
			return nil, err
		}
		sImages = append(sImages, sImage)
	}
	return sImages, nil
}

func (g *GalleryService) SearchImages64(ctx context.Context, search string, galleryID int, threshold float32) ([]db.SImage, error) {
	count, err := g.sDB.GetGalleryCount(galleryID)
	if err != nil {
		return nil, err
	}
	images, err := g.wDB.SearchImage64(ctx, search, galleryID, count, threshold)
	if err != nil {
		return nil, err
	}
	var sImages []db.SImage
	for _, img := range images {
		sImage, err := g.sDB.GetImageByPath(img.Path)
		if err != nil {
			return nil, err
		}
		sImages = append(sImages, sImage)
	}
	return sImages, nil
}

func (g *GalleryService) GetAllImages(ctx context.Context, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int, flagFilter string, folderFilter string) ([]db.SImage, error) {
	return g.sDB.GetAllImages(galleryID, sortBy, sortOrder, page, imagesPerPage, flagFilter, folderFilter)
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

func (g *GalleryService) DeleteImage(ctx context.Context, inputImg db.SImage) error {
	image, err := g.sDB.GetImageByID(inputImg.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch image: %w", err)
	}

	slog.Info("Deleting image", slog.String("file", image.FilePath))

	uuidStr := uuid.NewMD5(uuid.NameSpaceURL, []byte(image.FilePath+strconv.Itoa(image.GalleryID))).String()
	wImage := db.WImage{ID: uuidStr, Path: image.FilePath}

	if err := g.sDB.RemoveImage(image.ID); err != nil {
		return err
	}

	if err := g.wDB.RemoveImage(ctx, wImage); err != nil {
		return err
	}

	if image.FilePath != "" {
		if err := os.Remove(image.FilePath); err != nil {
			slog.Warn("Failed to delete physical file", slog.String("path", image.FilePath), slog.Any("error", err))
		}
	}
	return nil
}

func (g *GalleryService) DeleteImages(ctx context.Context, images []db.SImage) error {
	var paths []string
	var wImages []db.WImage

	for _, inputImg := range images {
		img, err := g.sDB.GetImageByID(inputImg.ID)
		if err != nil {
			slog.Warn("Failed to fetch image for deletion, skipping", slog.Int("id", inputImg.ID), slog.Any("error", err))
			continue
		}

		slog.Info("Removing Image from Databases", slog.String("Image", img.FilePath))
		uuidStr := uuid.NewMD5(uuid.NameSpaceURL, []byte(img.FilePath+strconv.Itoa(img.GalleryID))).String()
		wImages = append(wImages, db.WImage{ID: uuidStr, Path: img.FilePath})

		paths = append(paths, img.FilePath)

		if err := g.sDB.RemoveImage(img.ID); err != nil {
			return err
		}
	}

	if err := g.wDB.RemoveImages(ctx, wImages); err != nil {
		return err
	}

	for _, path := range paths {
		slog.Info("Deleting physical file", slog.String("file", path))
		err := os.Remove(path)
		if err != nil {
			slog.Warn("Failed to delete physical file", slog.String("path", path), slog.Any("error", err))
		}
	}
	return nil
}

func (g *GalleryService) FindDublicates(ctx context.Context, imageID int, galleryID int, threshold float32) ([]db.WImage, error) {
	sImage, err := g.sDB.GetImageByID(imageID)
	if err != nil {
		return nil, err
	}
	uuidStr := uuid.NewMD5(uuid.NameSpaceURL, []byte(sImage.FilePath+strconv.Itoa(sImage.GalleryID))).String()
	count, err := g.sDB.GetGalleryCount(galleryID)
	if err != nil {
		return nil, err
	}
	return g.wDB.FindDublicates(ctx, uuidStr, galleryID, count, threshold)
}

func (g *GalleryService) ChangeGallery(ctx context.Context, newID int, sImages []db.SImage) error {
	targetGallery, err := g.sDB.GetGalleryByID(newID)
	if err != nil {
		return err
	}
	var wImages []db.WImage

	for _, img := range sImages {
		if img.FilePath != "" {
			fileName := filepath.Base(img.FilePath)
			newPath := filepath.Join(targetGallery.Path, fileName)

			os.MkdirAll(targetGallery.Path, 0o755)

			if err := os.Rename(img.FilePath, newPath); err == nil {
				img.FilePath = newPath
			} else {
				input, err := os.ReadFile(img.FilePath)
				if err == nil {
					if err := os.WriteFile(newPath, input, 0o644); err == nil {
						os.Remove(img.FilePath)
						img.FilePath = newPath
					} else {
						return fmt.Errorf("failed to write file %s: %w", newPath, err)
					}
				} else {
					return fmt.Errorf("failed to read file %s: %w", img.FilePath, err)
				}
			}
			uuidStr := uuid.NewMD5(uuid.NameSpaceURL, []byte(img.FilePath+strconv.Itoa(img.GalleryID))).String()
			wImages = append(wImages, db.WImage{ID: uuidStr, Path: img.FilePath})
		}
	}

	if err := g.sDB.ChangeGallery(newID, sImages); err != nil {
		return err
	}
	if err := g.wDB.ChangeGallery(ctx, newID, wImages); err != nil {
		return err
	}
	return nil
}

func (g *GalleryService) CopyToGallery(ctx context.Context, newID int, sImages []db.SImage) error {
	targetGallery, err := g.sDB.GetGalleryByID(newID)
	if err != nil {
		return err
	}
	var wImages []db.WImage

	for _, img := range sImages {
		if img.FilePath != "" {
			fileName := filepath.Base(img.FilePath)
			newPath := filepath.Join(targetGallery.Path, fileName)

			os.MkdirAll(targetGallery.Path, 0o755)

			input, err := os.ReadFile(img.FilePath)
			if err == nil {
				if err := os.WriteFile(newPath, input, 0o644); err == nil {
					img.FilePath = newPath
					img.GalleryID = newID
				} else {
					return fmt.Errorf("failed to write copy %s: %w", newPath, err)
				}
			} else {
				return fmt.Errorf("failed to read source %s: %w", img.FilePath, err)
			}
			uuidStr := uuid.NewMD5(uuid.NameSpaceURL, []byte(img.FilePath+strconv.Itoa(img.GalleryID))).String()
			wImages = append(wImages, db.WImage{ID: uuidStr, Path: img.FilePath})
		}
	}

	if err := g.sDB.InsertImage(sImages); err != nil {
		return err
	}
	if err := g.wDB.CopyToGallery(ctx, newID, wImages); err != nil {
		return err
	}
	return nil
}

func (g *GalleryService) GetImageInfo(imageID int) (map[string]any, error) {
	sImage, err := g.sDB.GetImageByID(imageID)
	if err != nil {
		return nil, err
	}

	baseName := filepath.Base(sImage.FilePath)

	cameraDetails := map[string]any{
		"make":          sImage.CameraMake,
		"lens_make":     sImage.LensMake,
		"lens_model":    sImage.LensMake,
		"focal_length":  sImage.FocalLength,
		"aperture":      sImage.Aperture,
		"shutter_speed": sImage.ShutterSpeed,
		"iso":           sImage.Iso,
		"flash":         sImage.Flash,
	}

	imageInfo := map[string]any{
		"ID":            strconv.Itoa(sImage.ID),
		"Name":          strings.TrimSuffix(baseName, filepath.Ext(baseName)),
		"Path":          sImage.FilePath,
		"FilePath":      sImage.FilePath,
		"GalleryID":     strconv.Itoa(sImage.GalleryID),
		"Rating":        sImage.Rating,
		"Size":          sImage.Size,
		"Resolution":    sImage.Resolution,
		"AspectRatio":   sImage.AspectRatio,
		"Extension":     sImage.Extension,
		"Date":          sImage.Date,
		"Taken":         sImage.Taken,
		"Flag":          sImage.Flag,
		"CameraDetails": cameraDetails,
		"Gallery":       "",
	}

	galleryName, err := g.sDB.GetGalleryByID(sImage.GalleryID)
	if err != nil {
		imageInfo["Gallery"] = "Unknown"
	} else {
		imageInfo["Gallery"] = galleryName.Name
	}
	return imageInfo, nil
}

func (g *GalleryService) GetGalleryCount(galleryID int) (int, error) {
	g.mu.Lock()
	expected := g.ExpectedCount[galleryID]
	current, hasCurrent := g.CurrentCount[galleryID]
	g.mu.Unlock()

	if expected > 0 && hasCurrent {
		return current, nil
	}
	return g.sDB.GetGalleryCount(galleryID)
}

func (g *GalleryService) SetRating(imageID int, rating int) error {
	return g.sDB.UpdateImageRating(imageID, rating)
}

func (g *GalleryService) SetFlag(imageID int, flag int) error {
	return g.sDB.UpdateImageFlag(imageID, flag)
}

func (g *GalleryService) GetExpectedCount(galleryID int) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ExpectedCount[galleryID]
}

func (g *GalleryService) EditGalleryName(id int, newName string) error {
	return g.sDB.UpdateGallery(id, newName)
}

func (g *GalleryService) FindGlobalDuplicates(ctx context.Context, threshold float64, galleryIDs []int) ([][]db.SImage, error) {
	allImages, err := g.wDB.GetVectors(ctx, galleryIDs)
	if err != nil {
		return nil, err
	}

	var groups [][]db.SImage
	visited := make(map[string]bool)

	for i, imgA := range allImages {
		if visited[imgA.ID] {
			continue
		}

		var currentGroup []db.SImage

		for j := i + 1; j < len(allImages); j++ {
			imgB := allImages[j]
			if visited[imgB.ID] {
				continue
			}

			distance := 1.0 - (floats.Dot(imgA.Vector, imgB.Vector) / (math.Sqrt(floats.Norm(imgA.Vector, 2)) * math.Sqrt(floats.Norm(imgB.Vector, 2))))
			if distance <= threshold {
				if len(currentGroup) == 0 {
					sImageA, err := g.sDB.GetImageByPath(imgA.Path)
					if err != nil {
						return nil, err
					}
					currentGroup = append(currentGroup, sImageA)
				}
				sImageB, err := g.sDB.GetImageByPath(imgB.Path)
				if err != nil {
					return nil, err
				}
				currentGroup = append(currentGroup, sImageB)
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

func (g *GalleryService) Unflag(galleryName string) error {
	gallery, err := g.sDB.GetGalleryByName(galleryName)
	if err != nil {
		return err
	}
	return g.sDB.Unflag(gallery.ID)
}

func (g *GalleryService) ResetDatabase(ctx context.Context) error {
	if err := g.sDB.ResetDatabase(); err != nil {
		return err
	}
	if err := g.wDB.ResetDatabase(ctx); err != nil {
		return err
	}
	return nil
}
