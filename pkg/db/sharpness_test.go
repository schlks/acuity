package db

import (
	"image"
	"testing"
)

func stripes(w, h int) *image.Gray {
	g := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x/2)%2 == 0 {
				g.Pix[y*g.Stride+x] = 255
			}
		}
	}
	return g
}

func gradient(w, h int) *image.Gray {
	g := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			g.Pix[y*g.Stride+x] = uint8(x * 255 / w)
		}
	}
	return g
}

func paste(dst, src *image.Gray) {
	w := src.Bounds().Dx()
	for y := 0; y < src.Bounds().Dy(); y++ {
		copy(dst.Pix[y*dst.Stride:y*dst.Stride+w], src.Pix[y*src.Stride:y*src.Stride+w])
	}
}

func TestLaplacianScoreSharpBeatsSmooth(t *testing.T) {
	sharp := laplacianScore(stripes(256, 256))
	smooth := laplacianScore(gradient(256, 256))
	if sharp <= smooth {
		t.Fatalf("expected sharp > smooth, got %f <= %f", sharp, smooth)
	}
}

func TestLaplacianScoreFindsSharpRegion(t *testing.T) {
	full := laplacianScore(stripes(256, 256))

	g := gradient(256, 256)
	paste(g, stripes(64, 64))

	if got := laplacianScore(g); got < full/2 {
		t.Fatalf("small in-focus region should dominate the score, got %f, full sharp %f", got, full)
	}
}

func TestLaplacianScoreTinyImage(t *testing.T) {
	if got := laplacianScore(image.NewGray(image.Rect(0, 0, 2, 2))); got != 0 {
		t.Fatalf("expected 0 for images smaller than the kernel, got %f", got)
	}
}
