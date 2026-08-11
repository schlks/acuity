package db

import (
	"bytes"
	"context"
	"encoding/base64"
	"image/jpeg"
	"fmt"
	"io/fs"
	"log/slog"
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

type Bridge struct {
	sDB           *SQLiteClient
	wDB           *WeaviateClient
	ExpectedCount map[int]int
	CurrentCount  map[int]int
	cancelFuncs   map[int]context.CancelFunc
	mu            sync.Mutex
}

type ImagePayload struct {
	Weaviate WImage
	SQLite   SImage
}

type ImageInfo struct {
	SImage  SImage `json:"SImage"`
	Gallery string `json:"gallery,omitempty"`
	Name string 	 `json:"name,omitempty"`
}

func NewBridge(sDB *SQLiteClient, wDB *WeaviateClient) *Bridge {
	return &Bridge{
		sDB:           sDB,
		wDB:           wDB,
		ExpectedCount: make(map[int]int),
		CurrentCount:  make(map[int]int),
		cancelFuncs:   make(map[int]context.CancelFunc),
	}
}

func (b *Bridge) CancelImport(galleryID int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if cancel, exists := b.cancelFuncs[galleryID]; exists {
		cancel()
		delete(b.cancelFuncs, galleryID)
	}
}

func (b *Bridge) GetAllGalleries() ([]Gallery, error) {
	return b.sDB.GetAllGalleries()
}

func (b *Bridge) getSeenFilePaths(known []string, folderPath string) ([]string, []string, error) {
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

func (b *Bridge) CreateGallery(ctx context.Context, name string, folderPath string) error {
	if err := os.MkdirAll(folderPath, 0o755); err != nil {
		return err
	}

	if err := b.sDB.InsertGallery(folderPath, name); err != nil {
		return nil
	}

	if err := b.UpdateFolder(ctx, name); err != nil {
		return err
	}

	return nil
}

func (b *Bridge) DeleteGallery(ctx context.Context, id int) error {
	if err := b.sDB.RemoveGallery(id); err != nil {
		return err
	}

	if err := b.wDB.RemoveGalleryImages(ctx, id); err != nil {
		return err
	}

	return nil
}

func (b *Bridge) UpdateFolder(ctx context.Context, name string) error {
	gallery, err := b.sDB.GetGalleryByName(name)
	if err != nil {
		return err
	}

	b.mu.Lock()
	if b.ExpectedCount[gallery.ID] != 0 {
		b.mu.Unlock()
		return nil
	}
	b.ExpectedCount[gallery.ID] = -1
	b.mu.Unlock()

	go func() {
		defer func() {
			b.mu.Lock()
			b.ExpectedCount[gallery.ID] = 0
			delete(b.cancelFuncs, gallery.ID)
			b.mu.Unlock()
		}()

		known, err := b.sDB.GetKnownPaths(gallery.ID)
		if err != nil {
			slog.Error("Failed to get file paths", slog.Any("error", err))
		}

		filePaths, missingPaths, err := b.getSeenFilePaths(known, gallery.Path)
		if err != nil {
			slog.Error("Failed to get file paths", slog.Any("error", err))
			return
		}

		currentCount, _ := b.sDB.GetGalleryCount(gallery.ID)
		importCtx, cancel := context.WithCancel(context.Background())

		b.mu.Lock()
		b.ExpectedCount[gallery.ID] = currentCount + len(filePaths) - len(missingPaths)
		b.cancelFuncs[gallery.ID] = cancel

		b.CurrentCount[gallery.ID] = currentCount - len(missingPaths)
		b.mu.Unlock()

		defer cancel()

		if len(missingPaths) > 0 {
			var imagesToDelete []SImage
			for _, path := range missingPaths {
				imagesToDelete = append(imagesToDelete, SImage{FilePath: path, GalleryID: gallery.ID})
			}
			_ = b.DeleteImages(importCtx, imagesToDelete)
		}
		if len(filePaths) > 0 {
			b.ImportImages(importCtx, filePaths, gallery.ID, func(count int) {
				b.mu.Lock()
				if _, ok := b.CurrentCount[gallery.ID]; ok {
					b.CurrentCount[gallery.ID] += count
				}
				b.mu.Unlock()
			})
		}
	}()

	return nil
}

func (b *Bridge) ImportImages(ctx context.Context, filePaths []string, galleryID int, progressCallback func(count int)) {
	batchSize, threads := 100, runtime.NumCPU()
	maxWorkers := max(1, threads-2)

	resultChan, done := make(chan ImagePayload, batchSize), make(chan struct{})

	go func() {
		var wBatch []*models.Object
		var sBatch []SImage

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
				failed := b.wDB.WriteBatchDB(ctx, wBatch)
				if err := b.sDB.InsertImage(sBatch); err != nil {
					slog.Error("Failed to insert SQLite batch", slog.Any("error", err))
				}
				amount = len(wBatch) - failed
				if progressCallback != nil {
					progressCallback(amount)
				}

				wBatch, sBatch = make([]*models.Object, 0, batchSize), make([]SImage, 0, batchSize)
				slog.Info("Wrote Images into the Databases", slog.Int("amount", amount))
			}
		}
		if len(wBatch) > 0 {
			failed := b.wDB.WriteBatchDB(ctx, wBatch)
			amount = len(wBatch) - failed
			if err := b.sDB.InsertImage(sBatch); err != nil {
				slog.Error("Failed to insert SQLite batch", slog.Any("error", err))
			}
			if progressCallback != nil {
				progressCallback(amount)
			}
		}
		gallery, err := b.sDB.GetGalleryByID(galleryID)
		if err != nil {
			slog.Error("Failed to gallery name", slog.Any("error", err))
		}

		count, err := b.sDB.GetGalleryCount(galleryID)
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

			var sImage SImage
			var wImage WImage

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
				if size, imgErr := bimgImg.Size(); imgErr == nil {
					width = size.Width
					heigth = size.Height
				}
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

func (b *Bridge) GetGalleryID(name string) (int, error, bool) {
	if name == "global" {
		return -1, nil, true
	}

	gallery, err := b.sDB.GetGalleryByName(name)
	if err != nil {
		return 0, err, false
	}

	return gallery.ID, nil, true
}

func (b *Bridge) GetGalleryFiles(name string, sortBy string, sortOrder string, page int, imagesPerPage int, flagFilter string, folderFilter string) ([]SImage, error) {
	galleryID, err, ok := b.GetGalleryID(name)
	if err != nil && !ok {
		return nil, err
	}
	files, err := b.sDB.GetAllImages(galleryID, sortBy, sortOrder, page, imagesPerPage, flagFilter, folderFilter)
	if err != nil {
		return nil, err
	}

	return files, err
}

func (b *Bridge) SearchImages(ctx context.Context, search string, galleryID int, threshold float32) ([]SImage, error) {
	count, err := b.sDB.GetGalleryCount(galleryID)
	if err != nil {
		return nil, err
	}
	images, err := b.wDB.SearchImage(ctx, search, galleryID, count, threshold)
	if err != nil {
		return nil, err
	}
	var sImages []SImage
	for _, img := range images {
		sImage, err := b.sDB.GetImageByPath(img.Path)
		if err != nil {
			slog.Error("Somthing went wrong", slog.Any("Error", err))
			continue
		}
		dist := img.Distance
		sImage.Distance = &dist
		sImages = append(sImages, sImage)
	}
	return sImages, nil
}

func (b *Bridge) SearchImages64(ctx context.Context, search string, galleryID int, threshold float32) ([]SImage, error) {
	count, err := b.sDB.GetGalleryCount(galleryID)
	if err != nil {
		return nil, err
	}
	images, err := b.wDB.SearchImage64(ctx, search, galleryID, count, threshold)
	if err != nil {
		return nil, err
	}
	var sImages []SImage
	for _, img := range images {
		sImage, err := b.sDB.GetImageByPath(img.Path)
		if err != nil {
			slog.Error("Somthing went wrong", slog.Any("Error", err))
			continue
		}
		dist := img.Distance
		sImage.Distance = &dist
		sImages = append(sImages, sImage)
	}
	return sImages, nil
}

func (b *Bridge) GetAllImages(ctx context.Context, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int, flagFilter string, folderFilter string) ([]SImage, error) {
	return b.sDB.GetAllImages(galleryID, sortBy, sortOrder, page, imagesPerPage, flagFilter, folderFilter)
}

func (b *Bridge) ConvertImage(file []byte) (string, error) {
	bimgImg := bimg.NewImage(file)
	jpegBuffer, err := bimgImg.Convert(bimg.JPEG)
	if err != nil {
		return "", err
	}

	base64Image := base64.StdEncoding.EncodeToString(jpegBuffer)

	return base64Image, nil
}

func (b *Bridge) DeleteImage(ctx context.Context, inputImg SImage) error {
	image, err := b.sDB.GetImageByID(inputImg.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch image: %w", err)
	}

	slog.Info("Deleting image", slog.String("file", image.FilePath))

	uuidStr := uuid.NewMD5(uuid.NameSpaceURL, []byte(image.FilePath+strconv.Itoa(image.GalleryID))).String()
	wImage := WImage{ID: uuidStr, Path: image.FilePath}

	if err := b.sDB.RemoveImage(image.ID); err != nil {
		return err
	}

	if err := b.wDB.RemoveImage(ctx, wImage); err != nil {
		return err
	}

	if image.FilePath != "" {
		if err := os.Remove(image.FilePath); err != nil {
			slog.Warn("Failed to delete physical file", slog.String("path", image.FilePath), slog.Any("error", err))
		}
	}
	return nil
}

func (b *Bridge) DeleteImages(ctx context.Context, images []SImage) error {
	var paths []string
	var wImages []WImage

	for _, inputImg := range images {
		img, err := b.sDB.GetImageByID(inputImg.ID)
		if err != nil {
			slog.Warn("Failed to fetch image for deletion, skipping", slog.Int("id", inputImg.ID), slog.Any("error", err))
			continue
		}

		slog.Info("Removing Image from Databases", slog.String("Image", img.FilePath))
		uuidStr := uuid.NewMD5(uuid.NameSpaceURL, []byte(img.FilePath+strconv.Itoa(img.GalleryID))).String()
		wImages = append(wImages, WImage{ID: uuidStr, Path: img.FilePath})

		paths = append(paths, img.FilePath)

		if err := b.sDB.RemoveImage(img.ID); err != nil {
			return err
		}
	}

	if err := b.wDB.RemoveImages(ctx, wImages); err != nil {
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

func (b *Bridge) FindDublicates(ctx context.Context, imageID int, galleryID int, threshold float32) ([]SImage, error) {
	sImage, err := b.sDB.GetImageByID(imageID)
	if err != nil {
		return nil, err
	}
	uuidStr := uuid.NewMD5(uuid.NameSpaceURL, []byte(sImage.FilePath+strconv.Itoa(sImage.GalleryID))).String()
	count, err := b.sDB.GetGalleryCount(galleryID)
	if err != nil {
		return nil, err
	}
	if count <= 0 {
		count = 1000
	}
	var sImages []SImage
	images, err := b.wDB.FindDuplicates(ctx, uuidStr, galleryID, count, threshold)
	if err != nil {
		return nil, err
	}
	for _, img := range images {
		image, err := b.sDB.GetImageByPath(img.Path)
		if err != nil {
			slog.Error("Failed to get Image", slog.String("Image", img.Path), slog.Any("error", err))
			continue
		}
		dist := img.Distance
		image.Distance = &dist
		sImages = append(sImages, image)
	}
	return sImages, nil
}

func (b *Bridge) ChangeGallery(ctx context.Context, newID int, sImages []SImage) error {
	targetGallery, err := b.sDB.GetGalleryByID(newID)
	if err != nil {
		return err
	}
	var wImages []WImage

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
			wImages = append(wImages, WImage{ID: uuidStr, Path: img.FilePath})
		}
	}

	if err := b.sDB.ChangeGallery(newID, sImages); err != nil {
		return err
	}
	if err := b.wDB.ChangeGallery(ctx, newID, wImages); err != nil {
		return err
	}
	return nil
}

func (b *Bridge) CopyToGallery(ctx context.Context, newID int, sImages []SImage) error {
	targetGallery, err := b.sDB.GetGalleryByID(newID)
	if err != nil {
		return err
	}
	var wImages []WImage

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
			wImages = append(wImages, WImage{ID: uuidStr, Path: img.FilePath})
		}
	}

	if err := b.sDB.InsertImage(sImages); err != nil {
		return err
	}
	if err := b.wDB.CopyToGallery(ctx, newID, wImages); err != nil {
		return err
	}
	return nil
}

func (b *Bridge) GetImageInfo(imageID int) (ImageInfo, error) {
	sImage, err := b.sDB.GetImageByID(imageID)
	if err != nil {
		return ImageInfo{}, err
	}

	name := strings.TrimSuffix(filepath.Base(sImage.FilePath), filepath.Ext(sImage.FilePath))

	gallery, err := b.sDB.GetGalleryByID(sImage.GalleryID)
	if err != nil {
		return ImageInfo{}, err
	}

	imageInfo := ImageInfo{
		SImage: sImage,
		Gallery: gallery.Name,
		Name: name,
	}

	return imageInfo, nil
}

func (b *Bridge) GetGalleryCount(galleryID int) (int, error) {
	b.mu.Lock()
	expected := b.ExpectedCount[galleryID]
	current, hasCurrent := b.CurrentCount[galleryID]
	b.mu.Unlock()

	if expected > 0 && hasCurrent {
		return current, nil
	}
	return b.sDB.GetGalleryCount(galleryID)
}

func (b *Bridge) SetRating(imageID int, rating int) error {
	return b.sDB.UpdateImageRating(imageID, rating)
}

func (b *Bridge) SetFlag(imageID int, flag int) error {
	return b.sDB.UpdateImageFlag(imageID, flag)
}

func (b *Bridge) GetExpectedCount(galleryID int) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ExpectedCount[galleryID]
}

func (b *Bridge) EditGalleryName(id int, newName string) error {
	return b.sDB.UpdateGallery(id, newName)
}

func (b *Bridge) FindGlobalDuplicates(ctx context.Context, threshold float64, galleryIDs []int) ([][]SImage, error) {
	allImages, err := b.wDB.GetVectors(ctx, galleryIDs)
	if err != nil {
		return nil, err
	}

	var validImages []ImageVector
	for _, img := range allImages {
		if len(img.Vector) > 0 {
			norm := floats.Norm(img.Vector, 2)
			if norm > 0 {
				floats.Scale(1.0/norm, img.Vector)
				validImages = append(validImages, img)
			}
		}
	}

	n := len(validImages)
	if n == 0 {
		return nil, nil
	}

	type matchPair struct {
		i, j int
		dist float64
	}

	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}

	chunkSize := (n + numWorkers - 1) / numWorkers
	var wg sync.WaitGroup
	matchesChan := make(chan []matchPair, numWorkers)

	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if start >= n {
			break
		}
		if end > n {
			end = n
		}

		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			var localMatches []matchPair
			for i := s; i < e; i++ {
				vecA := validImages[i].Vector
				lenA := len(vecA)
				for j := i + 1; j < n; j++ {
					vecB := validImages[j].Vector
					if lenA != len(vecB) {
						continue
					}
					var dot float64
					for k := 0; k < lenA; k++ {
						dot += vecA[k] * vecB[k]
					}
					dist := 1.0 - dot
					if dist <= threshold {
						localMatches = append(localMatches, matchPair{i: i, j: j, dist: dist})
					}
				}
			}
			matchesChan <- localMatches
		}(start, end)
	}

	wg.Wait()
	close(matchesChan)

	adj := make(map[int][]matchPair)
	for chunk := range matchesChan {
		for _, m := range chunk {
			adj[m.i] = append(adj[m.i], m)
		}
	}

	var groups [][]SImage
	visited := make(map[int]bool)
	imageMap := make(map[string]SImage)

	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}
		matches, ok := adj[i]
		if !ok || len(matches) == 0 {
			continue
		}

		visited[i] = true
		groupIndices := []int{i}
		for _, m := range matches {
			if !visited[m.j] {
				visited[m.j] = true
				groupIndices = append(groupIndices, m.j)
			}
		}

		if len(groupIndices) > 1 {
			var currentGroup []SImage
			rootVec := validImages[i].Vector
			for idx, gIdx := range groupIndices {
				imgVec := validImages[gIdx]
				sImg, ok := imageMap[imgVec.Path]
				if !ok {
					var fetchErr error
					sImg, fetchErr = b.sDB.GetImageByPath(imgVec.Path)
					if fetchErr != nil {
						continue
					}
					imageMap[imgVec.Path] = sImg
				}
				dist := 0.0
				if idx > 0 {
					var dot float64
					for k := 0; k < len(rootVec); k++ {
						dot += rootVec[k] * imgVec.Vector[k]
					}
					dist = 1.0 - dot
				}
				sImg.Distance = &dist
				currentGroup = append(currentGroup, sImg)
			}
			if len(currentGroup) > 1 {
				groups = append(groups, currentGroup)
			}
		}
	}

	return groups, nil
}

func (b *Bridge) Unflag(galleryName string) error {
	gallery, err := b.sDB.GetGalleryByName(galleryName)
	if err != nil {
		return err
	}
	return b.sDB.Unflag(gallery.ID)
}

func (b *Bridge) ResetDatabase(ctx context.Context) error {
	if err := b.sDB.ResetDatabase(); err != nil {
		return err
	}
	if err := b.wDB.ResetDatabase(ctx); err != nil {
		return err
	}
	return nil
}
