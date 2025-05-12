package utils

import (
	"bytes"
	"fmt"
	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
	"image"
	"image/color"
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
		Quality:  25,
		Lossless: false,
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func ConvertToRoundedWebP(file multipart.File, contentType string) (*bytes.Buffer, error) {
	buf, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Lire l'image selon le type MIME
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

	// Redimensionner l'image à 128x128
	img = imaging.Fill(img, 128, 128, imaging.Center, imaging.Lanczos)

	// Créer un masque rond (cercle alpha)
	dst := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	centerX, centerY := 64, 64
	radius := 64

	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			dx := x - centerX
			dy := y - centerY
			if dx*dx+dy*dy <= radius*radius {
				dst.Set(x, y, img.At(x, y))
			} else {
				dst.Set(x, y, color.NRGBA{0, 0, 0, 0}) // transparent
			}
		}
	}

	// Encoder en WebP
	out := new(bytes.Buffer)
	if err := webp.Encode(out, dst, &webp.Options{
		Quality:  50,
		Lossless: false,
	}); err != nil {
		return nil, err
	}
	return out, nil
}
