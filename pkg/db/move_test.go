package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newMoveTestEnv(t *testing.T) (*Bridge, *SQLiteClient) {
	t.Helper()
	sDB, err := NewSqliteDB(filepath.Join(t.TempDir(), "test_acuity.db"))
	if err != nil {
		t.Fatalf("NewSqliteDB failed: %v", err)
	}
	if err := sDB.InitTable(); err != nil {
		t.Fatalf("InitTable failed: %v", err)
	}
	return NewBridge(sDB, nil), sDB
}

func addTestGallery(t *testing.T, sDB *SQLiteClient, name string, path string) int {
	t.Helper()
	if err := sDB.InsertGallery(name, path); err != nil {
		t.Fatalf("InsertGallery failed: %v", err)
	}
	g, err := sDB.GetGalleryByName(name)
	if err != nil {
		t.Fatalf("GetGalleryByName failed: %v", err)
	}
	return g.ID
}

func addTestImages(t *testing.T, sDB *SQLiteClient, galleryID int, paths ...string) {
	t.Helper()
	var images []SImage
	for _, p := range paths {
		images = append(images, SImage{GalleryID: galleryID, FilePath: p, Blurhash: "x"})
	}
	if err := sDB.InsertImage(images); err != nil {
		t.Fatalf("InsertImage failed: %v", err)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
}

func TestMoveFolderImagesPrefixMatching(t *testing.T) {
	_, sDB := newMoveTestEnv(t)
	fromID := addTestGallery(t, sDB, "from", "/lib")
	toID := addTestGallery(t, sDB, "to", "/other")

	addTestImages(t, sDB, fromID,
		"/lib/Urlaub_%/1.jpg",
		"/lib/Urlaub_%/deep/er/2.jpg",
		"/lib/Urlaub_%2/3.jpg",
		"/lib/UrlaubXY/4.jpg",
		"/lib/Urlaub_%",
		"/lib/Über Größe/5.jpg",
	)

	if err := sDB.MoveFolderImages("/lib/Urlaub_%", "/other/Urlaub_%", fromID, toID); err != nil {
		t.Fatalf("MoveFolderImages failed: %v", err)
	}
	if err := sDB.MoveFolderImages("/lib/Über Größe", "/other/Über Größe", fromID, toID); err != nil {
		t.Fatalf("MoveFolderImages failed: %v", err)
	}

	moved := map[string]string{
		"/other/Urlaub_%/1.jpg":         "/lib/Urlaub_%/1.jpg",
		"/other/Urlaub_%/deep/er/2.jpg": "/lib/Urlaub_%/deep/er/2.jpg",
		"/other/Über Größe/5.jpg":       "/lib/Über Größe/5.jpg",
	}
	for newPath := range moved {
		img, err := sDB.GetImageByPath(newPath)
		if err != nil {
			t.Fatalf("expected image at %s: %v", newPath, err)
		}
		if img.GalleryID != toID {
			t.Errorf("image %s: expected gallery %d, got %d", newPath, toID, img.GalleryID)
		}
	}

	untouched := []string{
		"/lib/Urlaub_%2/3.jpg",
		"/lib/UrlaubXY/4.jpg",
		"/lib/Urlaub_%",
	}
	for _, p := range untouched {
		img, err := sDB.GetImageByPath(p)
		if err != nil {
			t.Fatalf("expected untouched image at %s: %v", p, err)
		}
		if img.GalleryID != fromID {
			t.Errorf("image %s: expected gallery %d, got %d", p, fromID, img.GalleryID)
		}
	}
}

func TestMoveFolder(t *testing.T) {
	b, sDB := newMoveTestEnv(t)
	root := t.TempDir()
	galleryA := filepath.Join(root, "a")
	galleryB := filepath.Join(root, "b")
	if err := os.MkdirAll(galleryB, 0o755); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(galleryA, "Trip", "1.jpg"), "one")
	writeTestFile(t, filepath.Join(galleryA, "Trip", "sub", "2.jpg"), "two")
	writeTestFile(t, filepath.Join(galleryA, "Other", "3.jpg"), "three")

	aID := addTestGallery(t, sDB, "A", galleryA)
	bID := addTestGallery(t, sDB, "B", galleryB)
	addTestImages(t, sDB, aID,
		filepath.Join(galleryA, "Trip", "1.jpg"),
		filepath.Join(galleryA, "Trip", "sub", "2.jpg"),
		filepath.Join(galleryA, "Other", "3.jpg"),
	)

	newPath, err := b.MoveFolder(filepath.Join(galleryA, "Trip"), aID, bID)
	if err != nil {
		t.Fatalf("MoveFolder failed: %v", err)
	}
	if newPath != filepath.Join(galleryB, "Trip") {
		t.Errorf("unexpected new path %s", newPath)
	}

	if _, err := os.Stat(filepath.Join(galleryA, "Trip")); !os.IsNotExist(err) {
		t.Errorf("source folder should be gone, stat err: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(galleryB, "Trip", "sub", "2.jpg"))
	if err != nil || string(content) != "two" {
		t.Errorf("moved file missing or wrong: %v %q", err, content)
	}

	for _, p := range []string{
		filepath.Join(galleryB, "Trip", "1.jpg"),
		filepath.Join(galleryB, "Trip", "sub", "2.jpg"),
	} {
		img, err := sDB.GetImageByPath(p)
		if err != nil {
			t.Fatalf("expected image at %s: %v", p, err)
		}
		if img.GalleryID != bID {
			t.Errorf("image %s: expected gallery %d, got %d", p, bID, img.GalleryID)
		}
	}
	other, err := sDB.GetImageByPath(filepath.Join(galleryA, "Other", "3.jpg"))
	if err != nil || other.GalleryID != aID {
		t.Errorf("unrelated image changed: %v %+v", err, other)
	}

	b.mu.Lock()
	if b.ExpectedCount[aID] != 0 || b.ExpectedCount[bID] != 0 {
		t.Errorf("ExpectedCount not reset: %d %d", b.ExpectedCount[aID], b.ExpectedCount[bID])
	}
	b.mu.Unlock()
}

func TestMoveFolderRejections(t *testing.T) {
	b, sDB := newMoveTestEnv(t)
	root := t.TempDir()
	galleryA := filepath.Join(root, "a")
	galleryB := filepath.Join(root, "b")
	writeTestFile(t, filepath.Join(galleryA, "Trip", "1.jpg"), "one")
	writeTestFile(t, filepath.Join(galleryA, "Trip", "nested", "2.jpg"), "two")
	writeTestFile(t, filepath.Join(galleryB, "Trip", "existing.jpg"), "x")
	writeTestFile(t, filepath.Join(root, "outside", "9.jpg"), "nine")

	aID := addTestGallery(t, sDB, "A", galleryA)
	bID := addTestGallery(t, sDB, "B", galleryB)
	missingID := addTestGallery(t, sDB, "Missing", filepath.Join(root, "unmounted"))

	cases := []struct {
		name   string
		folder string
		src    int
		dst    int
	}{
		{"gallery root itself", galleryA, aID, bID},
		{"already in target gallery", filepath.Join(galleryA, "Trip"), aID, aID},
		{"target already has name", filepath.Join(galleryA, "Trip"), aID, bID},
		{"outside of gallery", filepath.Join(root, "outside"), aID, bID},
		{"dotdot escape", filepath.Join(galleryA, "..", "outside"), aID, bID},
		{"unavailable target", filepath.Join(galleryA, "Trip"), aID, missingID},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := b.MoveFolder(c.folder, c.src, c.dst); err == nil {
				t.Errorf("expected error")
			}
		})
	}

	t.Run("gallery import running", func(t *testing.T) {
		b.mu.Lock()
		b.ExpectedCount[bID] = 5
		b.mu.Unlock()
		defer func() {
			b.mu.Lock()
			b.ExpectedCount[bID] = 0
			b.mu.Unlock()
		}()
		writeTestFile(t, filepath.Join(galleryA, "Busy", "1.jpg"), "b")
		if _, err := b.MoveFolder(filepath.Join(galleryA, "Busy"), aID, bID); err == nil {
			t.Errorf("expected error while import is running")
		}
		if _, err := os.Stat(filepath.Join(galleryA, "Busy", "1.jpg")); err != nil {
			t.Errorf("source must stay untouched: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(galleryA, "Trip", "nested", "2.jpg")); err != nil {
		t.Errorf("source must stay untouched after rejections: %v", err)
	}
}

func TestCopyDir(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	writeTestFile(t, filepath.Join(src, "a.jpg"), "aaa")
	writeTestFile(t, filepath.Join(src, "sub", "deep", "b.jpg"), "bbb")
	if err := os.MkdirAll(filepath.Join(src, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(src, "a.jpg"), old, old); err != nil {
		t.Fatal(err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "sub", "deep", "b.jpg"))
	if err != nil || string(got) != "bbb" {
		t.Errorf("nested file wrong: %v %q", err, got)
	}
	if info, err := os.Stat(filepath.Join(dst, "empty")); err != nil || !info.IsDir() {
		t.Errorf("empty dir not copied: %v", err)
	}
	info, err := os.Stat(filepath.Join(dst, "a.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Errorf("mod time not preserved: %v", info.ModTime())
	}
}

func TestMoveDirCopyFallbackAbortsOnSymlink(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	writeTestFile(t, filepath.Join(src, "a.jpg"), "aaa")
	if err := os.Symlink(filepath.Join(src, "a.jpg"), filepath.Join(src, "link.jpg")); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}
	dst := filepath.Join(root, "dst")

	if err := copyDir(src, dst); err == nil {
		t.Fatalf("expected copyDir to reject symlink")
	}
	if _, err := os.Stat(filepath.Join(src, "a.jpg")); err != nil {
		t.Errorf("source must stay untouched: %v", err)
	}
}
