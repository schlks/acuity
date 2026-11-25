package db_test

import (
	"acuity/pkg/db"
	"database/sql"
	"errors"
	"math"
	"testing"
)

func setupBlurryGallery(t *testing.T) (*db.SQLiteClient, *db.Bridge) {
	t.Helper()
	sDB := setupTestDB(t)
	if err := sDB.InsertGallery("g", "/"); err != nil {
		t.Fatal(err)
	}
	f := func(v float64) *float64 { return &v }
	images := []db.SImage{
		{GalleryID: 1, FilePath: "/a.jpg", Blurhash: "x", Sharpness: f(5)},
		{GalleryID: 1, FilePath: "/b.jpg", Blurhash: "x", Sharpness: f(10)},
		{GalleryID: 1, FilePath: "/kept.jpg", Blurhash: "x", Sharpness: f(1), Flag: 1},
		{GalleryID: 1, FilePath: "/d.jpg", Blurhash: "x", Sharpness: f(500)},
		{GalleryID: 1, FilePath: "/pending.jpg", Blurhash: "x"},
		{GalleryID: 1, FilePath: "/failed.jpg", Blurhash: "x", Sharpness: f(-1)},
		{GalleryID: 1, FilePath: "/sub/f.jpg", Blurhash: "x", Sharpness: f(2)},
		{GalleryID: 2, FilePath: "/other.jpg", Blurhash: "x", Sharpness: f(0)},
	}
	if err := sDB.InsertImage(images); err != nil {
		t.Fatal(err)
	}
	return sDB, db.NewBridge(sDB, nil)
}

func flags(t *testing.T, sDB *db.SQLiteClient) map[string]int {
	t.Helper()
	got := map[string]int{}
	for _, p := range []string{"/a.jpg", "/b.jpg", "/kept.jpg", "/d.jpg", "/pending.jpg", "/failed.jpg", "/sub/f.jpg", "/other.jpg"} {
		img, err := sDB.GetImageByPath(p)
		if err != nil {
			t.Fatal(err)
		}
		got[p] = img.Flag
	}
	return got
}

func TestGetSharpnessStats(t *testing.T) {
	sDB, _ := setupBlurryGallery(t)

	tests := []struct {
		folder string
		want   db.SharpnessStats
	}{
		{"", db.SharpnessStats{Scored: 5, Pending: 1}},
		{"/sub/", db.SharpnessStats{Scored: 1, Pending: 0}},
		{"/nope/", db.SharpnessStats{Scored: 0, Pending: 0}},
	}
	for _, tt := range tests {
		got, err := sDB.GetSharpnessStats(1, tt.folder)
		if err != nil {
			t.Fatalf("folder %q: %v", tt.folder, err)
		}
		if got != tt.want {
			t.Fatalf("folder %q: got %+v, want %+v", tt.folder, got, tt.want)
		}
	}
}

func TestSharpnessAtRank(t *testing.T) {
	sDB, _ := setupBlurryGallery(t)

	for rank, want := range map[int]float64{1: 1, 2: 2, 3: 5, 4: 10, 5: 500} {
		got, err := sDB.SharpnessAtRank(1, "", rank)
		if err != nil {
			t.Fatalf("rank %d: %v", rank, err)
		}
		if got != want {
			t.Fatalf("rank %d: got %v, want %v", rank, got, want)
		}
	}

	if _, err := sDB.SharpnessAtRank(1, "", 6); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("rank beyond scored images: expected sql.ErrNoRows, got %v", err)
	}
	if _, err := sDB.SharpnessAtRank(1, "", 0); err == nil {
		t.Fatal("rank 0: expected error")
	}
}

func TestRejectBelowSharpness(t *testing.T) {
	sDB, _ := setupBlurryGallery(t)

	n, err := sDB.RejectBelowSharpness(1, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 rejected, got %d", n)
	}

	want := map[string]int{
		"/a.jpg": -1, "/b.jpg": 0, "/kept.jpg": 1, "/d.jpg": 0,
		"/pending.jpg": 0, "/failed.jpg": 0, "/sub/f.jpg": -1, "/other.jpg": 0,
	}
	if got := flags(t, sDB); !mapsEqual(got, want) {
		t.Fatalf("flags:\n got  %v\n want %v", got, want)
	}

	if n, err := sDB.RejectBelowSharpness(1, "", 5); err != nil || n != 0 {
		t.Fatalf("second run: expected 0 rejected, got %d, %v", n, err)
	}
}

func TestRejectBlurriest(t *testing.T) {
	sDB, bridge := setupBlurryGallery(t)

	steps := []struct {
		folder  string
		percent float64
		want    int64
	}{
		{"/nope/", 10, 0},
		{"/sub/", 10, 1},
		{"", 40, 0},
		{"", 40, 0},
		{"", 100, 3},
	}
	for _, s := range steps {
		got, err := bridge.RejectBlurriest("g", s.folder, s.percent)
		if err != nil {
			t.Fatalf("RejectBlurriest(%q, %v): %v", s.folder, s.percent, err)
		}
		if got != s.want {
			t.Fatalf("RejectBlurriest(%q, %v): got %d, want %d", s.folder, s.percent, got, s.want)
		}
	}

	want := map[string]int{
		"/a.jpg": -1, "/b.jpg": -1, "/kept.jpg": 1, "/d.jpg": -1,
		"/pending.jpg": 0, "/failed.jpg": 0, "/sub/f.jpg": -1, "/other.jpg": 0,
	}
	if got := flags(t, sDB); !mapsEqual(got, want) {
		t.Fatalf("flags:\n got  %v\n want %v", got, want)
	}
}

func TestRejectBlurriestErrors(t *testing.T) {
	_, bridge := setupBlurryGallery(t)

	tests := []struct {
		gallery string
		percent float64
		want    error
	}{
		{"g", 0, db.ErrInvalidPercent},
		{"g", -5, db.ErrInvalidPercent},
		{"g", 100.1, db.ErrInvalidPercent},
		{"g", math.NaN(), db.ErrInvalidPercent},
		{"g", math.Inf(1), db.ErrInvalidPercent},
		{"global", 10, db.ErrGlobalGallery},
		{"nope", 10, sql.ErrNoRows},
	}
	for _, tt := range tests {
		if _, err := bridge.RejectBlurriest(tt.gallery, "", tt.percent); !errors.Is(err, tt.want) {
			t.Fatalf("RejectBlurriest(%q, %v): got %v, want %v", tt.gallery, tt.percent, err, tt.want)
		}
	}

	if _, err := bridge.GetSharpnessStats("global", ""); !errors.Is(err, db.ErrGlobalGallery) {
		t.Fatalf("GetSharpnessStats(global): got %v, want %v", err, db.ErrGlobalGallery)
	}
}

func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}
