package db_test

import (
	"acuity/pkg/db"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) *db.SQLiteClient {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_acuity.db")
	client, err := db.NewSqliteDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test SQLite client: %v", err)
	}
	if err := client.InitTable(); err != nil {
		t.Fatalf("Failed to initialize database tables: %v", err)
	}
	return client
}

func TestSQLiteGalleries(t *testing.T) {
	sDB := setupTestDB(t)

	// 1. Insert galleries
	err := sDB.InsertGallery("Nature", "/photos/nature")
	if err != nil {
		t.Fatalf("InsertGallery failed: %v", err)
	}
	err = sDB.InsertGallery("Architecture", "/photos/architecture")
	if err != nil {
		t.Fatalf("InsertGallery failed: %v", err)
	}

	// 2. Get by name
	g1, err := sDB.GetGalleryByName("Nature")
	if err != nil {
		t.Fatalf("GetGalleryByName failed: %v", err)
	}
	if g1.Name != "Nature" || g1.Path != "/photos/nature" {
		t.Errorf("Unexpected gallery data: %+v", g1)
	}

	// 3. Get by ID
	g2, err := sDB.GetGalleryByID(g1.ID)
	if err != nil {
		t.Fatalf("GetGalleryByID failed: %v", err)
	}
	if g2.Name != g1.Name {
		t.Errorf("Expected name %s, got %s", g1.Name, g2.Name)
	}

	// 4. Get all galleries
	galleries, err := sDB.GetAllGalleries()
	if err != nil {
		t.Fatalf("GetAllGalleries failed: %v", err)
	}
	if len(galleries) != 2 {
		t.Errorf("Expected 2 galleries, got %d", len(galleries))
	}

	// 5. Update gallery name
	err = sDB.UpdateGallery(g1.ID, "Wild Nature")
	if err != nil {
		t.Fatalf("UpdateGallery failed: %v", err)
	}
	updated, _ := sDB.GetGalleryByID(g1.ID)
	if updated.Name != "Wild Nature" {
		t.Errorf("Expected updated name 'Wild Nature', got '%s'", updated.Name)
	}

	// 6. Remove gallery
	err = sDB.RemoveGallery(g1.ID)
	if err != nil {
		t.Fatalf("RemoveGallery failed: %v", err)
	}
	galleriesAfter, _ := sDB.GetAllGalleries()
	if len(galleriesAfter) != 1 {
		t.Errorf("Expected 1 gallery remaining, got %d", len(galleriesAfter))
	}
}

func TestSQLiteImagesCRUDAndSorting(t *testing.T) {
	sDB := setupTestDB(t)

	_ = sDB.InsertGallery("Holidays", "/photos/holidays")
	g, _ := sDB.GetGalleryByName("Holidays")

	// 1. Insert batch of images (with natural numbers to test NATSORT)
	images := []db.SImage{
		{
			GalleryID:  g.ID,
			FilePath:   "/photos/holidays/img10.jpg",
			Blurhash:   "L6PZfSi_.AyE_3t7t7R**0o#DgR4",
			Rating:     3,
			Flag:       1,
			Width:      1920,
			Height:     1080,
			Date:       "2026-08-01T10:00:00Z",
			Resolution: 1920 * 1080,
		},
		{
			GalleryID:  g.ID,
			FilePath:   "/photos/holidays/img2.jpg",
			Blurhash:   "L6PZfSi_.AyE_3t7t7R**0o#DgR4",
			Rating:     5,
			Flag:       2,
			Width:      3840,
			Height:     2160,
			Date:       "2026-08-02T10:00:00Z",
			Resolution: 3840 * 2160,
		},
		{
			GalleryID:  g.ID,
			FilePath:   "/photos/holidays/img1.jpg",
			Blurhash:   "L6PZfSi_.AyE_3t7t7R**0o#DgR4",
			Rating:     1,
			Flag:       0,
			Width:      1280,
			Height:     720,
			Date:       "2026-08-03T10:00:00Z",
			Resolution: 1280 * 720,
		},
	}

	err := sDB.InsertImage(images)
	if err != nil {
		t.Fatalf("InsertImage failed: %v", err)
	}

	// 2. Count verification
	count, err := sDB.GetGalleryCount(g.ID)
	if err != nil {
		t.Fatalf("GetGalleryCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}

	// 3. Natural Sort test (img1.jpg -> img2.jpg -> img10.jpg)
	sortedImages, err := sDB.GetAllImages(g.ID, "name", "asc", 1, 10, "", "")
	if err != nil {
		t.Fatalf("GetAllImages failed: %v", err)
	}
	if len(sortedImages) != 3 {
		t.Fatalf("Expected 3 images, got %d", len(sortedImages))
	}
	if sortedImages[0].FilePath != "/photos/holidays/img1.jpg" {
		t.Errorf("Natural sort failed: index 0 expected img1, got %s", sortedImages[0].FilePath)
	}
	if sortedImages[1].FilePath != "/photos/holidays/img2.jpg" {
		t.Errorf("Natural sort failed: index 1 expected img2, got %s", sortedImages[1].FilePath)
	}
	if sortedImages[2].FilePath != "/photos/holidays/img10.jpg" {
		t.Errorf("Natural sort failed: index 2 expected img10, got %s", sortedImages[2].FilePath)
	}

	// 4. Rating and Flag Updates
	img1 := sortedImages[0]
	err = sDB.UpdateImageRating(img1.ID, 4)
	if err != nil {
		t.Fatalf("UpdateImageRating failed: %v", err)
	}
	err = sDB.UpdateImageFlag(img1.ID, 2)
	if err != nil {
		t.Fatalf("UpdateImageFlag failed: %v", err)
	}

	reloaded, err := sDB.GetImageByID(img1.ID)
	if err != nil {
		t.Fatalf("GetImageByID failed: %v", err)
	}
	if reloaded.Rating != 4 || reloaded.Flag != 2 {
		t.Errorf("Expected rating 4 and flag 2, got rating %d, flag %d", reloaded.Rating, reloaded.Flag)
	}

	// 5. Flag filtering
	flaggedImages, err := sDB.GetAllImages(g.ID, "name", "asc", 1, 10, "2", "")
	if err != nil {
		t.Fatalf("Flag filter query failed: %v", err)
	}
	if len(flaggedImages) != 2 { // img2 had flag 2 originally, img1 was updated to 2
		t.Errorf("Expected 2 images with flag 2, got %d", len(flaggedImages))
	}

	// 6. Unflag Gallery
	err = sDB.Unflag(g.ID)
	if err != nil {
		t.Fatalf("Unflag failed: %v", err)
	}
	unflagged, _ := sDB.GetAllImages(g.ID, "name", "asc", 1, 10, "2", "")
	if len(unflagged) != 0 {
		t.Errorf("Expected 0 flagged images after Unflag, got %d", len(unflagged))
	}

	// 7. Remove Image
	err = sDB.RemoveImage(img1.ID)
	if err != nil {
		t.Fatalf("RemoveImage failed: %v", err)
	}
	countAfter, _ := sDB.GetGalleryCount(g.ID)
	if countAfter != 2 {
		t.Errorf("Expected 2 images after deletion, got %d", countAfter)
	}
}
