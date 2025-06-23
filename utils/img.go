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
	"os"
	"os/exec"
	"path/filepath"
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

	var img image.Image
	switch contentType {
	case "image/jpeg", "image/jpg":
		img, err = jpeg.Decode(bytes.NewReader(buf))
	case "image/png":
		img, err = png.Decode(bytes.NewReader(buf))
	case "image/webp":
		img, err = webp.Decode(bytes.NewReader(buf))
	default:
		return nil, fmt.Errorf("format non supporté: %s", contentType)
	}
	if err != nil {
		return nil, err
	}

	img = imaging.Fill(img, 128, 128, imaging.Center, imaging.Lanczos)
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
				dst.Set(x, y, color.NRGBA{0, 0, 0, 0})
			}
		}
	}

	out := new(bytes.Buffer)
	if err := webp.Encode(out, dst, &webp.Options{
		Quality:  50,
		Lossless: false,
	}); err != nil {
		return nil, err
	}
	return out, nil
}

func ConvertToRoundedGIF(file multipart.File) (*bytes.Buffer, error) {
	input, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Temp GIF input
	inTmp, err := os.CreateTemp("", "*.gif")
	if err != nil {
		return nil, err
	}
	defer os.Remove(inTmp.Name())
	if _, err := inTmp.Write(input); err != nil {
		return nil, err
	}
	inTmp.Close()

	// Extract all frames as PNGs
	framePattern := filepath.Join(os.TempDir(), "frame_%04d.png")
	extractCmd := exec.Command("convert", inTmp.Name()+"[0-19]", "-coalesce", "-resize", "128x128^", "-gravity", "center", "-extent", "128x128", framePattern)
	var extractErr bytes.Buffer
	extractCmd.Stderr = &extractErr
	if err := extractCmd.Run(); err != nil {
		return nil, fmt.Errorf("extract: %v\n%s", err, extractErr.String())
	}

	// Apply mask to each frame
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "frame_*.png"))
	for _, f := range matches {
		maskCmd := exec.Command("convert", f,
			"(", "-size", "128x128", "xc:none", "-draw", "fill white circle 64,64 64,0", ")",
			"-compose", "DstIn", "-composite", f)
		if err := maskCmd.Run(); err != nil {
			return nil, fmt.Errorf("mask: %v", err)
		}
	}

	// Compose all frames into final GIF
	outGif, err := os.CreateTemp("", "*.gif")
	if err != nil {
		return nil, err
	}
	defer os.Remove(outGif.Name())
	outGif.Close()

	composeCmd := exec.Command("convert", append(matches, "-delay", "4", "-loop", "0", outGif.Name())...)
	var composeErr bytes.Buffer
	composeCmd.Stderr = &composeErr
	if err := composeCmd.Run(); err != nil {
		return nil, fmt.Errorf("compose: %v\n%s", err, composeErr.String())
	}

	// Clean temp PNGs
	for _, f := range matches {
		os.Remove(f)
	}

	// Read back the GIF
	out := new(bytes.Buffer)
	f, err := os.Open(outGif.Name())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	_, err = io.Copy(out, f)
	if err != nil {
		return nil, err
	}
	return out, nil
}
