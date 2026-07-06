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
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	ImportImages(ctx context.Context, filePaths []string, galleryID int)
	RemoveGalleryImages(ctx context.Context, galleryID int) error
	RemoveImage(ctx context.Context, image db.Image) error
	WriteBatchDB(ctx context.Context, batch []*models.Object)
	SearchImage(ctx context.Context, search string, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int) ([]db.Image, error)
	SearchImage64(ctx context.Context, image string, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int) ([]db.Image, error)
	FindDublicates(ctx context.Context, imageID string, galleryID int, page int, imagesPerPage int) ([]db.Image, error)
	ChangeGallery(ctx context.Context, newID int, images []db.Image) error
	CopyToGallery(ctx context.Context, newGalleryID int, images []db.Image) error
	RemoveImages(ctx context.Context, images []db.Image) error
	GetAll(ctx context.Context, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int) ([]db.Image, error)
	GetKnownPaths(ctx context.Context, galleryID int) (map[string]struct{}, error)
	GetInfo(ctx context.Context, imageID string) (db.Image, error)
	GetGalleryCount(ctx context.Context, galleryID int) (int, error)
	ResetDatabase(ctx context.Context) error
	SetRating(ctx context.Context, imageID string, rating int) error
}

type GalleryService struct {
	sDB           sService
	wDB           wService
	ExpectedCount map[int]int
}

func NewService(sDB *db.SQLiteClient, wDB *db.WeaviateClient) *GalleryService {
	return &GalleryService{
		sDB:           sDB,
		wDB:           wDB,
		ExpectedCount: make(map[int]int),
	}
}

func (g *GalleryService) GetAllGalleries() ([]db.Gallery, error) {
	return g.sDB.GetAllGalleries()
}

func (g *GalleryService) getKnownFilePaths(known map[string]struct{}, folderPath string) ([]string, error) {
	var filePaths []string
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
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return filePaths, nil
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

	filePaths, err := g.getKnownFilePaths(known, gallery.Path)
	if err != nil {
		return err
	}

	currentCount, _ := g.wDB.GetGalleryCount(ctx, gallery.ID)
	g.ExpectedCount[gallery.ID] = currentCount + len(filePaths)

	go func() {
		g.wDB.ImportImages(context.Background(), filePaths, gallery.ID)
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

func (g *GalleryService) GetGalleryFiles(ctx context.Context, name string, sortBy string, sortOrder string, page int, imagesPerPage int) ([]db.Image, error) {
	gallery, err := g.sDB.GetGalleryByName(name)
	if err != nil {
		return nil, err
	}
	files, err := g.wDB.GetAll(ctx, gallery.ID, sortBy, sortOrder, page, imagesPerPage)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func (g *GalleryService) SearchImages(ctx context.Context, search string, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int) ([]db.Image, error) {
	return g.wDB.SearchImage(ctx, search, galleryID, sortBy, sortOrder, page, imagesPerPage)
}

func (g *GalleryService) SearchImages64(ctx context.Context, search string, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int) ([]db.Image, error) {
	return g.wDB.SearchImage64(ctx, search, galleryID, sortBy, sortOrder, page, imagesPerPage)
}

func (g *GalleryService) GetAllImages(ctx context.Context, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int) ([]db.Image, error) {
	return g.wDB.GetAll(ctx, galleryID, sortBy, sortOrder, page, imagesPerPage)
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
			err := os.Remove(path)
			if err != nil {
				slog.Warn("Failed to delete physical file", slog.String("path", path), slog.Any("error", err))
			}
		}
	}
	return nil
}

func (g *GalleryService) FindDublicates(ctx context.Context, imageID string, galleryID int, page int, imagesPerPage int) ([]db.Image, error) {
	images, err := g.wDB.FindDublicates(ctx, imageID, galleryID, page, imagesPerPage)
	if err != nil {
		return nil, err
	}

	return images, nil
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
		"ID":          wInfo.ID,
		"Name":        strings.TrimSuffix(baseName, filepath.Ext(baseName)),
		"Path":        wInfo.Path,
		"GalleryID":   wInfo.GalleryID,
		"Rating":      wInfo.Rating,
		"Time":        wInfo.Taken, // Genutzt von Weaviate
		"Size":        wInfo.Size,
		"Resolution":  wInfo.Resolution,
		"AspectRatio": wInfo.AspectRatio,
		"Extension":   wInfo.Extension,
		"Date":        wInfo.Date,
		"Gallery":     "",
	}
	galleryStr := imageInfo["GalleryID"].(string)
	gallery, err := strconv.Atoi(galleryStr)
	if err != nil {
		return nil, err
	}
	galleryName, err := g.sDB.GetGalleryByID(gallery)
	if err != nil {
		return nil, err
	}
	imageInfo["Gallery"] = galleryName.Name

	return imageInfo, nil
}

func (g *GalleryService) GetGalleryCount(ctx context.Context, galleryID int) (int, error) {
	return g.wDB.GetGalleryCount(ctx, galleryID)
}

func (g *GalleryService) SetRating(ctx context.Context, imageID string, rating int) error {
	return g.wDB.SetRating(ctx, imageID, rating)
}

func (g *GalleryService) EditGalleryName(id int, newName string) error {
	return g.sDB.UpdateGallery(id, newName)
}
