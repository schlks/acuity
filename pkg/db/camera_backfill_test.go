package db

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func ifdEntry(tag, typ uint16, count, value uint32) []byte {
	e := make([]byte, 12)
	binary.LittleEndian.PutUint16(e[0:], tag)
	binary.LittleEndian.PutUint16(e[2:], typ)
	binary.LittleEndian.PutUint32(e[4:], count)
	binary.LittleEndian.PutUint32(e[8:], value)
	return e
}

func exifSegment(model, lens string) []byte {
	modelBytes := append([]byte(model), 0)
	lensBytes := append([]byte(lens), 0)

	const ifd0Offset = 8
	const ifd0Size = 2 + 2*12 + 4
	const exifIFDOffset = ifd0Offset + ifd0Size
	const exifIFDSize = 2 + 1*12 + 4
	modelOffset := uint32(exifIFDOffset + exifIFDSize)
	lensOffset := modelOffset + uint32(len(modelBytes))

	tiff := []byte{'I', 'I', 0x2A, 0x00, ifd0Offset, 0, 0, 0}
	tiff = binary.LittleEndian.AppendUint16(tiff, 2)
	tiff = append(tiff, ifdEntry(0x0110, 2, uint32(len(modelBytes)), modelOffset)...)
	tiff = append(tiff, ifdEntry(0x8769, 4, 1, exifIFDOffset)...)
	tiff = binary.LittleEndian.AppendUint32(tiff, 0)
	tiff = binary.LittleEndian.AppendUint16(tiff, 1)
	tiff = append(tiff, ifdEntry(0xA434, 2, uint32(len(lensBytes)), lensOffset)...)
	tiff = binary.LittleEndian.AppendUint32(tiff, 0)
	tiff = append(tiff, modelBytes...)
	tiff = append(tiff, lensBytes...)

	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xFF, 0xE1}
	segment = binary.BigEndian.AppendUint16(segment, uint16(2+len(payload)))
	return append(segment, payload...)
}

func writeJPEG(t *testing.T, path string, app1 []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 16, 16)), nil); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	out := append([]byte{}, data[:2]...)
	out = append(out, app1...)
	out = append(out, data[2:]...)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBackfillCameraInfo(t *testing.T) {
	dir := t.TempDir()
	sDB, err := NewSqliteDB(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sDB.InitTable(); err != nil {
		t.Fatal(err)
	}

	withExif := filepath.Join(dir, "exif.jpg")
	withoutExif := filepath.Join(dir, "plain.jpg")
	missing := filepath.Join(dir, "missing.jpg")
	writeJPEG(t, withExif, exifSegment("EOS R6", "RF50mm F1.8 STM"))
	writeJPEG(t, withoutExif, nil)

	var images []SImage
	for _, p := range []string{withExif, withoutExif, missing} {
		images = append(images, SImage{GalleryID: 1, FilePath: p, Blurhash: "x"})
	}
	if err := sDB.InsertImage(images); err != nil {
		t.Fatal(err)
	}

	if err := NewBridge(sDB, nil).BackfillCameraInfo(t.Context()); err != nil {
		t.Fatal(err)
	}

	get := func(p string) SImage {
		img, err := sDB.GetImageByPath(p)
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	str := func(s *string) string {
		if s == nil {
			return "<nil>"
		}
		return *s
	}

	if img := get(withExif); str(img.CameraModel) != "EOS R6" || str(img.LensModel) != "RF50mm F1.8 STM" {
		t.Fatalf("expected EOS R6 / RF50mm F1.8 STM, got %q / %q", str(img.CameraModel), str(img.LensModel))
	}
	if img := get(withoutExif); str(img.CameraModel) != "" || str(img.LensModel) != "" {
		t.Fatalf("expected empty strings for image without EXIF so it is not retried, got %q / %q", str(img.CameraModel), str(img.LensModel))
	}
	if img := get(missing); img.CameraModel != nil || img.LensModel != nil {
		t.Fatalf("expected NULL for unreadable file so it is retried, got %q / %q", str(img.CameraModel), str(img.LensModel))
	}

	remaining, err := sDB.GetImagesWithoutCameraInfo()
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].FilePath != missing {
		t.Fatalf("expected only the missing file to remain, got %+v", remaining)
	}
}
