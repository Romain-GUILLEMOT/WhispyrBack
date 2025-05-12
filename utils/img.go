package utils

import (
	"bytes"
	"fmt"
	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
)

func ConvertToWebP(file multipart.File, contentType string) (*bytes.Buffer, error) {
	buf, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	exifData, _ := exif.Decode(bytes.NewReader(buf))
	orientation := 1
	if exifData != nil {
		if tag, err := exifData.Get(exif.Orientation); err == nil {
			orientation, _ = tag.Int(0)
		}
	}

	var img image.Image
	switch contentType {
	case "image/jpeg", "image/jpg":
		img, err = jpeg.Decode(bytes.NewReader(buf))
	case "image/png":
		img, err = png.Decode(bytes.NewReader(buf))
	default:
		return nil, fmt.Errorf("format non supporté: %s", contentType)
	}
	if err != nil {
		return nil, err
	}

	switch orientation {
	case 3:
		img = imaging.Rotate180(img)
	case 6:
		img = imaging.Rotate270(img)
	case 8:
		img = imaging.Rotate90(img)
	}

	img = imaging.Resize(img, 1280, 0, imaging.Lanczos)

	out := new(bytes.Buffer)
	if err := webp.Encode(out, img, &webp.Options{
		Quality:  10,
		Lossless: false,
	}); err != nil {
		return nil, err
	}
	return out, nil
}
