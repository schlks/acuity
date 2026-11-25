package raw

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanoberholster/imagemeta"
	"github.com/evanoberholster/imagemeta/meta"
)

var extensions = map[string]bool{
	".nef": true, ".cr2": true, ".cr3": true, ".arw": true,
	".dng": true, ".raf": true, ".orf": true, ".rw2": true, ".srw": true,
}

type source interface {
	io.ReadSeeker
	io.ReaderAt
}

func IsRawExtension(ext string) bool {
	return extensions[strings.ToLower(ext)]
}

func ExtractPreview(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return extractPreview(file, filepath.Ext(path))
}

func ExtractPreviewFromBytes(data []byte, ext string) ([]byte, error) {
	return extractPreview(bytes.NewReader(data), ext)
}

func extractPreview(r source, ext string) ([]byte, error) {
	if strings.ToLower(ext) == ".cr3" {
		return imagemeta.PreviewCR3(r)
	}

	type preview struct {
		Offset, Length uint32
		Compression    meta.Compression
	}

	x, err := imagemeta.Decode(r)
	if err != nil {
		return nil, err
	}

	candidates := []preview{
		{x.IFD0.ThumbnailOffset, x.IFD0.ThumbnailLength, meta.Compression(7)},
	}
	if x.IFD1 != nil {
		candidates = append(candidates, preview{x.IFD1.ImageOffset, x.IFD1.ImageLength, x.IFD1.Compression})
	}
	if x.IFD2 != nil {
		candidates = append(candidates, preview{x.IFD2.ImageOffset, x.IFD2.ImageLength, x.IFD2.Compression})
	}

	var offset, length uint32
	for _, c := range candidates {
		if c.Compression != 6 && c.Compression != 7 {
			continue
		}
		if c.Length > length {
			length = c.Length
			offset = c.Offset
		}
	}
	if length == 0 {
		return nil, fmt.Errorf("no preview found")
	}

	return io.ReadAll(io.NewSectionReader(r, int64(offset), int64(length)))
}
