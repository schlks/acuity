package db_test

import (
	"acuity/pkg/db"
	"slices"
	"testing"
)

func pendingVectorPaths(t *testing.T, sDB *db.SQLiteClient) []string {
	t.Helper()
	list, err := sDB.GetImagesWithoutVectors()
	if err != nil {
		t.Fatalf("GetImagesWithoutVectors failed: %v", err)
	}
	paths := make([]string, 0, len(list))
	for _, img := range list {
		paths = append(paths, img.FilePath)
	}
	slices.Sort(paths)
	return paths
}

func TestGetImagesWithoutVectorsSkipsFailed(t *testing.T) {
	sDB := setupTestDB(t)
	vDB := db.NewVectorClient(sDB, nil)
	if err := vDB.InitSchema(); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}
	if err := sDB.InsertGallery("g", "/"); err != nil {
		t.Fatal(err)
	}
	if err := sDB.InsertImage([]db.SImage{
		{GalleryID: 1, FilePath: "/embedded.jpg", Blurhash: "x"},
		{GalleryID: 1, FilePath: "/import_failed.avis", Blurhash: "x", VectorFailed: true},
		{GalleryID: 1, FilePath: "/backfill_failed.avis", Blurhash: "x"},
		{GalleryID: 1, FilePath: "/pending.jpg", Blurhash: "x"},
	}); err != nil {
		t.Fatal(err)
	}

	embedded, err := sDB.GetImageByPath("/embedded.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if err := vDB.InsertVector(embedded.ID, generateNormalizedVector(1)); err != nil {
		t.Fatal(err)
	}

	want := []string{"/backfill_failed.avis", "/pending.jpg"}
	if got := pendingVectorPaths(t, sDB); !slices.Equal(got, want) {
		t.Fatalf("before mark: got %v, want %v", got, want)
	}

	failed, err := sDB.GetImageByPath("/backfill_failed.avis")
	if err != nil {
		t.Fatal(err)
	}
	if err := sDB.MarkVectorFailed(failed.ID); err != nil {
		t.Fatalf("MarkVectorFailed failed: %v", err)
	}

	want = []string{"/pending.jpg"}
	if got := pendingVectorPaths(t, sDB); !slices.Equal(got, want) {
		t.Fatalf("after mark: got %v, want %v", got, want)
	}

	if err := sDB.InsertImage([]db.SImage{
		{GalleryID: 1, FilePath: "/import_failed.avis", Blurhash: "x"},
	}); err != nil {
		t.Fatal(err)
	}

	want = []string{"/import_failed.avis", "/pending.jpg"}
	if got := pendingVectorPaths(t, sDB); !slices.Equal(got, want) {
		t.Fatalf("after rescan: got %v, want %v", got, want)
	}
}
