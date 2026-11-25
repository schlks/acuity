package db_test

import (
	"acuity/pkg/db"
	"fmt"
	"slices"
	"testing"
)

func sortedPaths(t *testing.T, sDB *db.SQLiteClient, sortBy, order string, page, perPage int) []string {
	t.Helper()
	images, err := sDB.GetAllImages(1, sortBy, order, page, perPage, "", "")
	if err != nil {
		t.Fatalf("GetAllImages(%s, %s, page %d) failed: %v", sortBy, order, page, err)
	}
	paths := make([]string, 0, len(images))
	for _, img := range images {
		paths = append(paths, img.FilePath)
	}
	return paths
}

func TestSortBySharpness(t *testing.T) {
	sDB := setupTestDB(t)
	f := func(v float64) *float64 { return &v }

	images := []db.SImage{
		{GalleryID: 1, FilePath: "/a.jpg", Blurhash: "x", Sharpness: f(500)},
		{GalleryID: 1, FilePath: "/b.jpg", Blurhash: "x", Sharpness: f(-1)},
		{GalleryID: 1, FilePath: "/c.jpg", Blurhash: "x", Sharpness: f(20)},
		{GalleryID: 1, FilePath: "/d.jpg", Blurhash: "x", Sharpness: nil},
		{GalleryID: 1, FilePath: "/e.jpg", Blurhash: "x", Sharpness: f(9000)},
		{GalleryID: 1, FilePath: "/f.jpg", Blurhash: "x", Sharpness: f(0)},
	}
	if err := sDB.InsertImage(images); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		order string
		want  []string
	}{
		{"asc", []string{"/f.jpg", "/c.jpg", "/a.jpg", "/e.jpg", "/d.jpg", "/b.jpg"}},
		{"desc", []string{"/e.jpg", "/a.jpg", "/c.jpg", "/f.jpg", "/b.jpg", "/d.jpg"}},
	}
	for _, tt := range tests {
		t.Run(tt.order, func(t *testing.T) {
			if got := sortedPaths(t, sDB, "sharpness", tt.order, 1, 100); !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTiesAreBrokenByIDInSortDirection(t *testing.T) {
	sDB := setupTestDB(t)

	var images []db.SImage
	var ascending []string
	for i := range 11 {
		path := fmt.Sprintf("/img%02d.jpg", i)
		images = append(images, db.SImage{GalleryID: 1, FilePath: path, Blurhash: "x"})
		ascending = append(ascending, path)
	}
	if err := sDB.InsertImage(images); err != nil {
		t.Fatal(err)
	}
	descending := slices.Clone(ascending)
	slices.Reverse(descending)

	for _, sortBy := range []string{"sharpness", "rating", "flag"} {
		for _, tt := range []struct {
			order string
			want  []string
		}{{"asc", ascending}, {"desc", descending}} {
			t.Run(sortBy+"/"+tt.order, func(t *testing.T) {
				var paged []string
				for page := 1; ; page++ {
					chunk := sortedPaths(t, sDB, sortBy, tt.order, page, 3)
					if len(chunk) == 0 {
						break
					}
					paged = append(paged, chunk...)
				}
				if !slices.Equal(paged, tt.want) {
					t.Fatalf("paged order is not id order:\n got  %v\n want %v", paged, tt.want)
				}
			})
		}
	}
}
