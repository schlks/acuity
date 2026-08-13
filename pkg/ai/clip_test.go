package ai_test

import (
	"acuity/pkg/ai"
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestCLIPEmbedder(t *testing.T) {
	paths, err := ai.EnsureModels()
	if err != nil {
		t.Fatalf("Failed to ensure models: %v", err)
	}

	embedder, err := ai.NewCLIPEmbedder(paths.VisualModelPath, paths.TextualModelPath, paths.OnnxLibPath)
	if err != nil {
		t.Fatalf("Failed to initialize CLIP embedder: %v", err)
	}

	// 1. Test Text Embedding
	textVec, err := embedder.EmbedText("a photo of a blue square")
	if err != nil {
		t.Fatalf("Failed to embed text: %v", err)
	}
	if len(textVec) != 512 {
		t.Fatalf("Expected 512 dimensions for text vector, got %d", len(textVec))
	}

	// 2. Test Image Embedding with generated blue image
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 0, G: 0, B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("Failed to encode test jpeg: %v", err)
	}

	imgVec, err := embedder.EmbedImage(buf.Bytes())
	if err != nil {
		t.Fatalf("Failed to embed image: %v", err)
	}
	if len(imgVec) != 512 {
		t.Fatalf("Expected 512 dimensions for image vector, got %d", len(imgVec))
	}

	// 3. Check Cosine Similarity
	var similarity float32
	for i := 0; i < 512; i++ {
		similarity += textVec[i] * imgVec[i]
	}
	t.Logf("Cosine similarity between blue square image and prompt: %f", similarity)
	if similarity <= 0.15 {
		t.Errorf("Expected positive cosine similarity, got %f", similarity)
	}
}
