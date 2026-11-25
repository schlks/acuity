package db_test

import (
	"acuity/pkg/db"
	"path/filepath"
	"testing"
)

func TestInitTableMigratesOldSchema(t *testing.T) {
	client, err := db.NewSqliteDB(filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatalf("NewSqliteDB failed: %v", err)
	}

	_, err = client.DB.Exec(`
		CREATE TABLE images (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			gallery_id INTEGER,
			filepath TEXT UNIQUE,
			file_hash TEXT,
			blurhash TEXT NOT NULL,
			rating INTEGER DEFAULT 0,
			flag INTEGER DEFAULT 0,
			date DATETIME
		);
		INSERT INTO images (gallery_id, filepath, file_hash, blurhash, date) VALUES (1, '/old.jpg', 'h', 'x', '2026-01-01T00:00:00Z');
	`)
	if err != nil {
		t.Fatalf("creating old schema failed: %v", err)
	}

	for i := range 2 {
		if err := client.InitTable(); err != nil {
			t.Fatalf("InitTable run %d failed: %v", i+1, err)
		}
	}

	img, err := client.GetImageByPath("/old.jpg")
	if err != nil {
		t.Fatalf("GetImageByPath after migration failed: %v", err)
	}
	if img.Sharpness != nil {
		t.Fatalf("expected NULL sharpness for migrated row, got %f", *img.Sharpness)
	}
	if img.CameraModel != nil || img.LensModel != nil {
		t.Fatalf("expected NULL camera/lens model for migrated row, got %v / %v", img.CameraModel, img.LensModel)
	}
}
