package db

import (
	"io"
	"strings"

	"github.com/evanoberholster/imagemeta"
	"github.com/evanoberholster/imagemeta/meta/exif"
)

type cameraInfo struct {
	camera string
	lens   string
}

func cleanExifString(s string) string {
	return strings.TrimSpace(strings.Trim(s, "\x00"))
}

func cameraInfoFrom(x exif.Exif) cameraInfo {
	return cameraInfo{
		camera: cleanExifString(x.IFD0.Model),
		lens:   cleanExifString(x.ExifIFD.LensModel),
	}
}

func readCameraInfo(r io.ReadSeeker) cameraInfo {
	x, err := imagemeta.Decode(r)
	if err != nil {
		return cameraInfo{}
	}
	return cameraInfoFrom(x)
}
