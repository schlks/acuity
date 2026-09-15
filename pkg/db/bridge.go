package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image/jpeg"
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
	"github.com/coder/hnsw"
	"github.com/evanoberholster/imagemeta"
	"github.com/h2non/bimg"
	"github.com/pillowskiy/imagesize"
	"golang.org/x/sync/errgroup"
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
	_ = b.sDB.ClearAllDuplicateCache()
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
				img, err := b.sDB.GetImageByPath(path)
				if err != nil {
					slog.Error("Failed to get Image from Path", slog.Any("error", err))
					continue
				}
				imagesToDelete = append(imagesToDelete, img)
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

func (b *Bridge) insertProgress(sBatch []SImage, embBatch [][]float32, batchSize int, progressCallback func(count int)) ([]SImage, [][]float32) {
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
	return sBatch, embBatch
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
				sBatch, embBatch = b.insertProgress(sBatch, embBatch, batchSize, progressCallback)
			}
		}
		if len(sBatch) > 0 {
			_, _ = b.insertProgress(sBatch, embBatch, batchSize, progressCallback)
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
			if err != nil {
				return err
			}

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

func (b *Bridge) GetImagesByIDs(ids []int) ([]SImage, error) {
	imagesMap, err := b.sDB.GetImagesByIDs(ids)
	if err != nil {
		return nil, err
	}
	images := make([]SImage, 0, len(imagesMap))
	for _, id := range ids {
		if img, ok := imagesMap[id]; ok {
			images = append(images, img)
		}
	}
	return images, nil
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

func (b *Bridge) DeleteImages(ctx context.Context, images []SImage) error {
	var imageIDs []int

	for i := range images {
		if images[i].FilePath == "" {
			image, err := b.sDB.GetImageByID(images[i].ID)
			if err != nil {
				slog.Error("Failed to get Image", slog.Any("error", err))
				continue
			}
			images[i].FilePath = image.FilePath
		}
	}

	for _, img := range images {
		imageIDs = append(imageIDs, img.ID)
	}
	if err := b.sDB.RemoveImages(images); err != nil {
		slog.Error("Failed to remove Images from Database", slog.Any("error", err))
	}

	if err := b.sDB.RemoveImagesFromDuplicateCache(imageIDs); err != nil {
		slog.Error("Failed to update duplicates cache after deletion", slog.Any("error", err))
	}

	if err := b.vDB.RemoveImages(ctx, imageIDs); err != nil {
		return err
	}

	for _, img := range images {
		if _, err := os.Stat(img.FilePath); errors.Is(err, os.ErrNotExist) {
			slog.Info("Physical file alreaydy missing -> skipping", slog.Any("error", err))
			continue
		}
		slog.Info("Deleting physical file", slog.String("file", img.FilePath))
		err := os.Remove(img.FilePath)
		if err != nil {
			slog.Warn("Failed to delete physical file", slog.String("path", img.FilePath), slog.Any("error", err))
		}
	}
	return nil
}

func (b *Bridge) FindDuplicates(ctx context.Context, imageID int, galleryID int, threshold float32) ([]SImage, error) {
	return b.vDB.FindDuplicates(ctx, imageID, galleryID, 1000, threshold)
}

func (b *Bridge) ChangeGallery(newID int, sImages []SImage) error {
	targetGallery, err := b.sDB.GetGalleryByID(newID)
	if err != nil {
		return err
	}

	for i := range sImages {
		img := &sImages[i]
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

func (b *Bridge) CopyToGallery(newID int, sImages []SImage) error {
	targetGallery, err := b.sDB.GetGalleryByID(newID)
	if err != nil {
		return err
	}

	for i := range sImages {
		img := &sImages[i]
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
	// 1. Create Cache-Lookup
	sortedIDs := slices.Clone(galleryIDs)
	slices.Sort(sortedIDs)
	var idStrs []string
	for _, id := range sortedIDs {
		idStrs = append(idStrs, strconv.Itoa(id))
	}
	cacheKey := strings.Join(idStrs, ",")

	// 2. Try loading duplicate groups from database cache
	if cachedGroups, err := b.sDB.GetCachedDuplicates(cacheKey, threshold); err == nil && len(cachedGroups) > 0 {
		slog.Info("Loaded duplicate groups from database cache", slog.String("key", cacheKey), slog.Float64("threshold", threshold))

		// Batch fetch all SImage objects by their IDs
		var allIDs []int
		for _, g := range cachedGroups {
			for _, item := range g {
				allIDs = append(allIDs, item.ID)
			}
		}
		imagesMap, err := b.sDB.GetImagesByIDs(allIDs)
		if err != nil {
			return nil, err
		}

		var duplicateGroups [][]SImage
		for _, memberItems := range cachedGroups {
			var group []SImage
			for _, item := range memberItems {
				if image, ok := imagesMap[item.ID]; ok {
					image.Distance = item.Distance
					group = append(group, image)
				}
			}
			if len(group) > 1 {
				duplicateGroups = append(duplicateGroups, group)
			}
		}
		return duplicateGroups, nil
	}

	// 3. Cache miss: perform SIMD duplicates search
	allImages, err := b.vDB.GetVectors(ctx, galleryIDs)
	if err != nil {
		return nil, err
	}

	var validImages []ImageVector
	for _, img := range allImages {
		if len(img.Vector) > 0 {
			validImages = append(validImages, img)
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

	graph := hnsw.NewGraph[int]()
	for i, img := range validImages {
		graph.Add(hnsw.MakeNode(i, img.Vector))
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
				neighbors := graph.Search(validImages[i].Vector, 50)
				for _, n := range neighbors {
					j := n.Key
					dist := 1.0 - b.dot(validImages[i].Vector, n.Value)

					if j > i && dist <= threshold {
						localMatches = append(localMatches, matchPair{
							i:    i,
							j:    j,
							dist: dist,
						})
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

	var allDuplicateIDs []int
	for _, memberIndices := range groups {
		if len(memberIndices) > 1 {
			for _, idx := range memberIndices {
				id, _ := strconv.Atoi(validImages[idx].ID)
				allDuplicateIDs = append(allDuplicateIDs, id)
			}
		}
	}

	imagesMap, err := b.sDB.GetImagesByIDs(allDuplicateIDs)
	if err != nil {
		return nil, err
	}

	var duplicateGroups [][]SImage
	for _, memberIndices := range groups {
		if len(memberIndices) <= 1 {
			continue
		}

		var group []SImage
		var firstVec []float32

		for i, idx := range memberIndices {
			id, _ := strconv.Atoi(validImages[idx].ID)
			image, ok := imagesMap[id]
			if !ok {
				continue
			}

			if i == 0 {
				firstVec = validImages[idx].Vector
				group = append(group, image)
			} else if len(firstVec) > 0 {
				dist := 1.0 - b.dot(firstVec, validImages[idx].Vector)
				if dist <= threshold {
					image.Distance = &dist
					group = append(group, image)
				}
			}
		}

		if len(group) > 1 {
			duplicateGroups = append(duplicateGroups, group)
		}
	}

	// Save computed duplicate groups to cache
	var cacheGroups [][]CacheItem
	for _, group := range duplicateGroups {
		var items []CacheItem
		for _, img := range group {
			items = append(items, CacheItem{
				ID:       img.ID,
				Distance: img.Distance,
			})
		}
		cacheGroups = append(cacheGroups, items)
	}
	if err := b.sDB.SaveCachedDuplicates(cacheKey, threshold, cacheGroups); err != nil {
		slog.Error("Failed to save duplicate groups to cache", slog.Any("error", err))
	}

	return duplicateGroups, nil
}

func (b *Bridge) dot(a, c []float32) float64 {
	var sum float32
	for i := range a {
		sum += a[i] * c[i]
	}
	return float64(sum)
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

func (b *Bridge) ResetDatabase() error {
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

func (b *Bridge) IndexMissingVectors(ctx context.Context) error {
	if b.vDB == nil || b.vDB.embedder == nil {
		return fmt.Errorf("AI embedder not initialized")
	}

	images, err := b.sDB.GetImagesWithoutVectors()
	if err != nil || len(images) == 0 {
		return err
	}

	slog.Info("Starting vector backfill", slog.Int("count", len(images)))

	var count int
	for _, item := range images {
		img := item
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		data, err := os.ReadFile(img.FilePath)
		if err != nil {
			slog.Warn("Failed to read image for vector backfill", slog.String("path", img.FilePath), slog.Any("error", err))
			continue
		}

		emb, err := b.vDB.embedder.EmbedImage(data)
		if err != nil {
			slog.Warn("Failed to embed image", slog.Int("id", img.ID), slog.Any("error", err))
			continue
		}

		if err := b.vDB.InsertVector(img.ID, emb); err != nil {
			slog.Warn("Failed to insert vector for image", slog.Int("id", img.ID), slog.Any("error", err))
			continue
		}

		count++
		if count%50 == 0 || count == len(images) {
			slog.Info("Vector backfill progress", slog.Int("processed", count), slog.Int("total", len(images)))
		}
	}

	slog.Info("Vector backfill finished", slog.Int("indexed", count), slog.Int("total", len(images)))
	return nil
}
