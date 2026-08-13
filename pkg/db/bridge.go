package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image/jpeg"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/buckket/go-blurhash"
	"github.com/evanoberholster/imagemeta"
	"github.com/h2non/bimg"
	"github.com/pillowskiy/imagesize"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/floats"
)

type Bridge struct {
	sDB           *SQLiteClient
	vDB           *VectorClient
	ExpectedCount map[int]int
	CurrentCount  map[int]int
	cancelFuncs   map[int]context.CancelFunc
	mu            sync.Mutex
}

type ImagePayload struct {
	SQLite    SImage
	Embedding []float32
}

type ImageInfo struct {
	SImage  SImage `json:"SImage"`
	Gallery string `json:"gallery,omitempty"`
	Name    string `json:"name,omitempty"`
}

func NewBridge(sDB *SQLiteClient, vDB *VectorClient) *Bridge {
	return &Bridge{
		sDB:           sDB,
		vDB:           vDB,
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
		b.ExpectedCount[galleryID] = 0
		delete(b.CurrentCount, galleryID)
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
	for path, exists := range seen {
		if !exists {
			missingPaths = append(missingPaths, path)
		}
	}

	return filePaths, missingPaths, nil
}

func (b *Bridge) ScanGalleries() error {
	galleries, err := b.sDB.GetAllGalleries()
	if err != nil {
		return err
	}

	for _, gallery := range galleries {
		err := b.ScanGallery(gallery.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Bridge) ScanGallery(name string) error {
	gallery, err := b.sDB.GetGalleryByName(name)
	if err != nil {
		return err
	}
	b.mu.Lock()
	if b.ExpectedCount[gallery.ID] != 0 {
		b.mu.Unlock()
		return fmt.Errorf("gallery is already being imported")
	}
	b.ExpectedCount[gallery.ID] = -1
	b.mu.Unlock()

	go func() {
		defer func() {
			b.mu.Lock()
			b.ExpectedCount[gallery.ID] = 0
			delete(b.CurrentCount, gallery.ID)
			delete(b.cancelFuncs, gallery.ID)
			b.mu.Unlock()
		}()

		importCtx, cancel := context.WithCancel(context.Background())

		knownPaths, err := b.sDB.GetKnownPaths(gallery.ID)
		if err != nil {
			cancel()
			return
		}

		filePaths, missingPaths, err := b.getSeenFilePaths(knownPaths, gallery.Path)
		if err != nil {
			cancel()
			return
		}

		currentCount, err := b.sDB.GetGalleryCount(gallery.ID)
		if err != nil {
			cancel()
			return
		}

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
		var sBatch []SImage
		var embBatch [][]float32

		for result := range resultChan {
			sBatch = append(sBatch, result.SQLite)
			embBatch = append(embBatch, result.Embedding)

			if len(sBatch) >= batchSize {
				if err := b.sDB.InsertImage(sBatch); err != nil {
					slog.Error("Failed to insert SQLite batch", slog.Any("error", err))
				}
				for i, sImg := range sBatch {
					if len(embBatch[i]) > 0 {
						if img, err := b.sDB.GetImageByPath(sImg.FilePath); err == nil {
							_ = b.vDB.InsertVector(img.ID, embBatch[i])
						}
					}
				}
				if progressCallback != nil {
					progressCallback(len(sBatch))
				}
				slog.Info("Wrote Images into SQLite and Vector Store", slog.Int("amount", len(sBatch)))
				sBatch, embBatch = make([]SImage, 0, batchSize), make([][]float32, 0, batchSize)
			}
		}
		if len(sBatch) > 0 {
			if err := b.sDB.InsertImage(sBatch); err != nil {
				slog.Error("Failed to insert SQLite batch", slog.Any("error", err))
			}
			for i, sImg := range sBatch {
				if len(embBatch[i]) > 0 {
					if img, err := b.sDB.GetImageByPath(sImg.FilePath); err == nil {
						_ = b.vDB.InsertVector(img.ID, embBatch[i])
					}
				}
			}
			if progressCallback != nil {
				progressCallback(len(sBatch))
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

	eg, gCtx := errgroup.WithContext(ctx)
	eg.SetLimit(maxWorkers)

	for _, path := range filePaths {
		select {
		case <-gCtx.Done():
			break
		default:
		}

		filePath := path
		eg.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}

			var sImage SImage

			sImage.GalleryID = galleryID
			sImage.FilePath = filePath
			sImage.Extension = filepath.Ext(filePath)

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			stat, err := os.Stat(path)

			bimgImg := bimg.NewImage(data)
			var width, height int
			if size, imgErr := bimgImg.Size(); imgErr == nil {
				width = size.Width
				height = size.Height
			} else if imageSize, err := imagesize.ExtractFileInfo(path); err == nil {
				width = imageSize.Width
				height = imageSize.Height
			}

			var resolution int
			var aspectRatio float64
			if width > 0 && height > 0 {
				resolution = width * height
				aspectRatio = float64(width) / float64(height)
			}

			// Generate blurhash by resizing & converting to JPEG
			bhash := ""
			smallImgOptions := bimg.Options{
				Width:  128,
				Height: 128,
				Crop:   false,
				Type:   bimg.JPEG,
			}
			if smallBuffer, err := bimgImg.Process(smallImgOptions); err == nil {
				if img, err := jpeg.Decode(bytes.NewReader(smallBuffer)); err == nil {
					if bh, err := blurhash.Encode(4, 3, img); err == nil {
						bhash = bh
					}
				}
			}
			sImage.Blurhash = bhash

			hash := sha256.Sum256(data)
			sImage.FileHash = hex.EncodeToString(hash[:])
			sImage.Width = width
			sImage.Height = height
			sImage.Tags = ""

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

			var embedding []float32
			if b.vDB != nil && b.vDB.embedder != nil {
				if emb, err := b.vDB.embedder.EmbedImage(data); err == nil {
					embedding = emb
				}
			}

			resultChan <- ImagePayload{
				SQLite:    sImage,
				Embedding: embedding,
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		slog.Error("Errors occurred while importing images", slog.Any("error", err))
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
	return gallery.ID, nil, false
}

func (b *Bridge) GetAllImages(galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int, flagFilter string, folderFilter string) ([]SImage, error) {
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

	if err := b.sDB.RemoveImage(image.ID); err != nil {
		return err
	}

	if err := b.vDB.RemoveImages(ctx, []int{image.ID}); err != nil {
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
	var imageIDs []int

	for _, inputImg := range images {
		img, err := b.sDB.GetImageByID(inputImg.ID)
		if err != nil {
			slog.Warn("Failed to fetch image for deletion, skipping", slog.Int("id", inputImg.ID), slog.Any("error", err))
			continue
		}

		slog.Info("Removing Image from Databases", slog.String("Image", img.FilePath))
		imageIDs = append(imageIDs, img.ID)
		paths = append(paths, img.FilePath)

		if err := b.sDB.RemoveImage(img.ID); err != nil {
			return err
		}
	}

	if err := b.vDB.RemoveImages(ctx, imageIDs); err != nil {
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

func (b *Bridge) FindDuplicates(ctx context.Context, imageID int, galleryID int, threshold float32) ([]SImage, error) {
	return b.vDB.FindDuplicates(ctx, imageID, galleryID, 1000, threshold)
}

func (b *Bridge) ChangeGallery(ctx context.Context, newID int, sImages []SImage) error {
	targetGallery, err := b.sDB.GetGalleryByID(newID)
	if err != nil {
		return err
	}

	for _, img := range sImages {
		if img.FilePath != "" {
			fileName := filepath.Base(img.FilePath)
			newPath := filepath.Join(targetGallery.Path, fileName)

			err := os.MkdirAll(targetGallery.Path, 0o755)
			if err != nil {
				return err
			}

			if err := os.Rename(img.FilePath, newPath); err == nil {
				img.FilePath = newPath
			} else {
				input, err := os.ReadFile(img.FilePath)
				if err == nil {
					if err := os.WriteFile(newPath, input, 0o644); err == nil {
						_ = os.Remove(img.FilePath)
						img.FilePath = newPath
					} else {
						return fmt.Errorf("failed to write file %s: %w", newPath, err)
					}
				} else {
					return fmt.Errorf("failed to read file %s: %w", img.FilePath, err)
				}
			}
		}
	}

	return b.sDB.ChangeGallery(newID, sImages)
}

func (b *Bridge) CopyToGallery(ctx context.Context, newID int, sImages []SImage) error {
	targetGallery, err := b.sDB.GetGalleryByID(newID)
	if err != nil {
		return err
	}

	for _, img := range sImages {
		if img.FilePath != "" {
			fileName := filepath.Base(img.FilePath)
			newPath := filepath.Join(targetGallery.Path, fileName)

			err := os.MkdirAll(targetGallery.Path, 0o755)
			if err != nil {
				return err
			}

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
		}
	}

	return b.sDB.InsertImage(sImages)
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
		SImage:  sImage,
		Gallery: gallery.Name,
		Name:    name,
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
	allImages, err := b.vDB.GetVectors(ctx, galleryIDs)
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

	numWorkers := max(runtime.NumCPU(), 1)

	chunkSize := (n + numWorkers - 1) / numWorkers
	var wg sync.WaitGroup
	matchesChan := make(chan []matchPair, numWorkers)

	for w := range numWorkers {
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
					for k := range lenA {
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

	go func() {
		wg.Wait()
		close(matchesChan)
	}()

	var allMatches []matchPair
	for matches := range matchesChan {
		allMatches = append(allMatches, matches...)
	}

	type unionFind struct {
		parent []int
	}
	uf := unionFind{parent: make([]int, n)}
	for i := range n {
		uf.parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if uf.parent[i] != i {
			uf.parent[i] = find(uf.parent[i])
		}
		return uf.parent[i]
	}
	union := func(i, j int) {
		rootI := find(i)
		rootJ := find(j)
		if rootI != rootJ {
			uf.parent[rootI] = rootJ
		}
	}

	for _, m := range allMatches {
		union(m.i, m.j)
	}

	groups := make(map[int][]int)
	for i := range n {
		root := find(i)
		groups[root] = append(groups[root], i)
	}

	var duplicateGroups [][]SImage
	for _, memberIndices := range groups {
		if len(memberIndices) > 1 {
			var group []SImage
			for _, idx := range memberIndices {
				image, err := b.sDB.GetImageByPath(validImages[idx].Path)
				if err != nil {
					slog.Error("Failed to get image by path for duplicate group", slog.String("path", validImages[idx].Path), slog.Any("error", err))
					continue
				}
				group = append(group, image)
			}
			if len(group) > 1 {
				duplicateGroups = append(duplicateGroups, group)
			}
		}
	}

	return duplicateGroups, nil
}

func (b *Bridge) CreateGallery(name string, path string) error {
	return b.sDB.InsertGallery(name, path)
}

func (b *Bridge) Unflag(galleryName string) error {
	id, err, _ := b.GetGalleryID(galleryName)
	if err != nil {
		return err
	}
	return b.sDB.Unflag(id)
}

func (b *Bridge) UpdateFolder(name string) error {
	return b.ScanGallery(name)
}

func (b *Bridge) GetGalleryFiles(name string, sortBy string, sortOrder string, page int, imagesPerPage int, flagFilter string, folderFilter string) ([]SImage, error) {
	id, err, isGlobal := b.GetGalleryID(name)
	if err != nil && !isGlobal {
		return nil, err
	}
	return b.sDB.GetAllImages(id, sortBy, sortOrder, page, imagesPerPage, flagFilter, folderFilter)
}

func (b *Bridge) ResetDatabase(ctx context.Context) error {
	if err := b.sDB.ResetDatabase(); err != nil {
		return err
	}
	return b.vDB.InitSchema()
}

func (b *Bridge) DeleteGallery(ctx context.Context, galleryID int) error {
	if err := b.vDB.RemoveGalleryImages(ctx, galleryID); err != nil {
		return err
	}
	return b.sDB.RemoveGallery(galleryID)
}

func (b *Bridge) SearchImages(ctx context.Context, search string, galleryID int, threshold float32) ([]SImage, error) {
	return b.vDB.SearchImages(ctx, search, galleryID, threshold)
}

func (b *Bridge) SearchImage(ctx context.Context, base64Image string, galleryID int, threshold float32) ([]SImage, error) {
	return b.vDB.SearchImage(ctx, base64Image, galleryID, threshold)
}
