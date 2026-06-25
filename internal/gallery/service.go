// Package gallery contains the core business logic for managing
// photo galleries, including synchronizing local folders and
// orchestrating database updates.
package gallery

import (
	"acuity/pkg/db"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"encoding/base64"

	"github.com/h2non/bimg"
)

type GalleryService struct {
	sDB *db.SQLiteClient
	wDB *db.WeaviateClient
}

// TODO: move to different Gallery

func NewService(sDB *db.SQLiteClient, wDB *db.WeaviateClient) *GalleryService {
	return &GalleryService{
		sDB: sDB,
		wDB: wDB,
	}
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

	go func() {
		ctx = context.Background()
		g.wDB.ImportImages(ctx, filePaths, gallery.ID)
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

func (g *GalleryService) GetGalleryFiles(ctx context.Context, name string) ([]db.Image, error) {
	gallery, err := g.sDB.GetGalleryByName(name)
	if err != nil {
		return nil, err
	}
	files, err := g.wDB.GetAll(ctx, gallery.ID)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (g *GalleryService) SearchImages(ctx context.Context, search string, limit int, galleryID int) ([]db.Image, error) {
	return g.wDB.SearchImage(ctx, search, limit, galleryID)
}

func (g *GalleryService) SearchImages64(ctx context.Context, search string, limit int, galleryID int) ([]db.Image, error) {
	return g.wDB.SearchImage64(ctx, search, limit, galleryID)
}

func (g *GalleryService) GetAllImages(ctx context.Context, galleryID int) ([]db.Image, error) {
	return g.wDB.GetAll(ctx, galleryID)
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

func (g *GalleryService) DeleteImages(files []db.Image) error {
	for _, file := range files {
		if err := os.Remove(file.Path); err != nil {
			return err
		}
	}
	return nil
}

func (g *GalleryService) DeleteImage(ctx context.Context, file db.Image) error {
	if err := os.Remove(file.Path); err != nil {
		return err
	}
	if err := g.wDB.RemoveImage(ctx, file); err != nil {
		return err
	}
	return nil
}

func (g *GalleryService) FindDublicates(ctx context.Context, imageID int, galleryID int) ([]db.Image, error) {
	images, err := g.wDB.FindDublicates(ctx, imageID, galleryID)
	if err != nil {
		return nil, err
	}
	return images, nil
}

func (g *GalleryService) ChangeGallery(ctx context.Context, newID int, images []db.Image) error {
	if err := g.wDB.ChangeGallery(ctx, newID, images); err != nil {
		return err
	}
	return nil
}
