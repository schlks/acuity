package db

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func writeStripesJPEG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 600, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 600; x++ {
			v := uint8(0)
			if (x/4)%2 == 0 {
				v = 255
			}
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBackfillSharpness(t *testing.T) {
	dir := t.TempDir()
	sDB, err := NewSqliteDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sDB.InitTable(); err != nil {
		t.Fatal(err)
	}

	good := filepath.Join(dir, "good.jpg")
	broken := filepath.Join(dir, "broken.jpg")
	missing := filepath.Join(dir, "missing.jpg")
	writeStripesJPEG(t, good)
	if err := os.WriteFile(broken, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}

	var images []SImage
	for _, p := range []string{good, broken, missing} {
		images = append(images, SImage{GalleryID: 1, FilePath: p, Blurhash: "x"})
	}
	if err := sDB.InsertImage(images); err != nil {
		t.Fatal(err)
	}

	if err := NewBridge(sDB, nil).BackfillSharpness(t.Context()); err != nil {
		t.Fatal(err)
	}

	get := func(p string) *float64 {
		img, err := sDB.GetImageByPath(p)
		if err != nil {
			t.Fatal(err)
		}
		return img.Sharpness
	}

	if s := get(good); s == nil || *s <= 0 {
		t.Fatalf("expected positive score for good image, got %v", s)
	}
	if s := get(broken); s == nil || *s != -1 {
		t.Fatalf("expected -1 for undecodable image, got %v", s)
	}
	if s := get(missing); s != nil {
		t.Fatalf("expected NULL for unreadable file so it is retried, got %f", *s)
	}

	remaining, err := sDB.GetImagesWithoutSharpness()
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].FilePath != missing {
		t.Fatalf("expected only the missing file to remain, got %+v", remaining)
	}
}
