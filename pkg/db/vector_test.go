package db_test

import (
	"acuity/pkg/db"
	"context"
	"math"
	"testing"
)

func generateNormalizedVector(seed float32) []float32 {
	vec := make([]float32, 512)
	var sum float64
	for i := range vec {
		val := float32(math.Sin(float64(i)*float64(seed) + 0.1))
		vec[i] = val
		sum += float64(val * val)
	}
	norm := float32(math.Sqrt(sum))
	for i := range vec {
		vec[i] /= norm
	}
	return vec
}

func TestVectorClientOperations(t *testing.T) {
	sDB := setupTestDB(t)
	vDB := db.NewVectorClient(sDB, nil)

	// 1. Initialize vector virtual table
	if err := vDB.InitSchema(); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	// 2. Insert dummy gallery and images into SQLite
	_ = sDB.InsertGallery("VectorsTest", "/photos/vecs")
	g, _ := sDB.GetGalleryByName("VectorsTest")

	images := []db.SImage{
		{GalleryID: g.ID, FilePath: "/photos/vecs/img1.jpg", Blurhash: "L6PZfSi_"},
		{GalleryID: g.ID, FilePath: "/photos/vecs/img2.jpg", Blurhash: "L6PZfSi_"},
		{GalleryID: g.ID, FilePath: "/photos/vecs/img3.jpg", Blurhash: "L6PZfSi_"},
	}
	_ = sDB.InsertImage(images)

	img1, _ := sDB.GetImageByPath("/photos/vecs/img1.jpg")
	img2, _ := sDB.GetImageByPath("/photos/vecs/img2.jpg")
	img3, _ := sDB.GetImageByPath("/photos/vecs/img3.jpg")

	// 3. Insert vectors: img1 and img2 have almost identical vectors, img3 is different
	vecA := generateNormalizedVector(1.0)
	vecA_similar := make([]float32, 512)
	copy(vecA_similar, vecA)
	vecA_similar[0] += 0.001 // tiny perturbation
	// renormalize
	var s float64
	for _, v := range vecA_similar {
		s += float64(v * v)
	}
	for i := range vecA_similar {
		vecA_similar[i] /= float32(math.Sqrt(s))
	}

	vecB := generateNormalizedVector(5.0)

	if err := vDB.InsertVector(img1.ID, vecA); err != nil {
		t.Fatalf("InsertVector img1 failed: %v", err)
	}
	if err := vDB.InsertVector(img2.ID, vecA_similar); err != nil {
		t.Fatalf("InsertVector img2 failed: %v", err)
	}
	if err := vDB.InsertVector(img3.ID, vecB); err != nil {
		t.Fatalf("InsertVector img3 failed: %v", err)
	}

	// 4. Test GetVectors
	ctx := context.Background()
	allVecs, err := vDB.GetVectors(ctx, []int{g.ID})
	if err != nil {
		t.Fatalf("GetVectors failed: %v", err)
	}
	if len(allVecs) != 3 {
		t.Fatalf("Expected 3 vectors, got %d", len(allVecs))
	}
	if len(allVecs[0].Vector) != 512 {
		t.Errorf("Expected 512 vector dimensions, got %d", len(allVecs[0].Vector))
	}

	// 5. Test FindDuplicates (img1 should find img2 as close duplicate within threshold 0.1)
	duplicates, err := vDB.FindDuplicates(ctx, img1.ID, g.ID, 10, 0.1)
	if err != nil {
		t.Fatalf("FindDuplicates failed: %v", err)
	}
	if len(duplicates) != 1 {
		t.Fatalf("Expected exactly 1 duplicate for img1, got %d", len(duplicates))
	}
	if duplicates[0].ID != img2.ID {
		t.Errorf("Expected duplicate ID %d (img2), got %d", img2.ID, duplicates[0].ID)
	}
	if duplicates[0].Distance == nil || *duplicates[0].Distance > 0.05 {
		t.Errorf("Expected very small cosine distance, got %v", duplicates[0].Distance)
	}

	// 6. Test RemoveImages
	if err := vDB.RemoveImages(ctx, []int{img1.ID}); err != nil {
		t.Fatalf("RemoveImages failed: %v", err)
	}
	vecsAfter, _ := vDB.GetVectors(ctx, []int{g.ID})
	if len(vecsAfter) != 2 {
		t.Errorf("Expected 2 vectors after deletion, got %d", len(vecsAfter))
	}

	// 7. Test RemoveGalleryImages
	if err := vDB.RemoveGalleryImages(ctx, g.ID); err != nil {
		t.Fatalf("RemoveGalleryImages failed: %v", err)
	}
	vecsEmpty, _ := vDB.GetVectors(ctx, []int{g.ID})
	if len(vecsEmpty) != 0 {
		t.Errorf("Expected 0 vectors after gallery wipe, got %d", len(vecsEmpty))
	}
}
