package db

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"sort"

	"github.com/h2non/bimg"

	"acuity/pkg/raw"
)

const (
	sharpnessEdge = 1024
	sharpnessGrid = 8
	sharpnessTopK = 4
)

func sharpnessFor(data []byte, ext string) float64 {
	source := data
	if raw.IsRawExtension(ext) {
		preview, err := raw.ExtractPreviewFromBytes(data, ext)
		if err != nil {
			return -1
		}
		source = preview
	}
	score, err := computeSharpness(source)
	if err != nil {
		return -1
	}
	return score
}

func computeSharpness(data []byte) (float64, error) {
	buf, err := bimg.Resize(data, bimg.Options{
		Width:          sharpnessEdge,
		Height:         sharpnessEdge,
		Interpretation: bimg.InterpretationBW,
		Type:           bimg.JPEG,
		Quality:        95,
	})
	if err != nil {
		return 0, err
	}

	decoded, err := jpeg.Decode(bytes.NewReader(buf))
	if err != nil {
		return 0, err
	}

	return laplacianScore(toGray(decoded)), nil
}

func toGray(img image.Image) *image.Gray {
	if g, ok := img.(*image.Gray); ok {
		return g
	}
	b := img.Bounds()
	g := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(g, g.Bounds(), img, b.Min, draw.Src)
	return g
}

func laplacianScore(g *image.Gray) float64 {
	w, h := g.Bounds().Dx(), g.Bounds().Dy()
	if w < 3 || h < 3 {
		return 0
	}

	const tiles = sharpnessGrid * sharpnessGrid
	var sum, sq [tiles]float64
	var n [tiles]int

	pix, stride := g.Pix, g.Stride
	for y := 1; y < h-1; y++ {
		ty := y * sharpnessGrid / h
		row := y * stride
		for x := 1; x < w-1; x++ {
			i := row + x
			l := float64(int(pix[i-1]) + int(pix[i+1]) + int(pix[i-stride]) + int(pix[i+stride]) - 4*int(pix[i]))
			t := ty*sharpnessGrid + x*sharpnessGrid/w
			sum[t] += l
			sq[t] += l * l
			n[t]++
		}
	}

	vars := make([]float64, 0, tiles)
	for t := range n {
		if n[t] == 0 {
			continue
		}
		m := sum[t] / float64(n[t])
		vars = append(vars, sq[t]/float64(n[t])-m*m)
	}
	if len(vars) == 0 {
		return 0
	}

	sort.Sort(sort.Reverse(sort.Float64Slice(vars)))
	k := min(sharpnessTopK, len(vars))
	var total float64
	for _, v := range vars[:k] {
		total += v
	}
	return total / float64(k)
}
