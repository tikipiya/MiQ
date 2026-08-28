// Package testfixture provides deterministic, non-personal image fixtures.
package testfixture

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const Size = 1024

// Illustration returns a transparent abstract geometric avatar.
func Illustration() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, Size, Size))
	center := float64(Size) / 2
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			dx, dy := float64(x)-center, float64(y)-center
			radius := math.Hypot(dx, dy)
			if radius > 455 {
				continue
			}
			angle := math.Atan2(dy, dx)
			wave := math.Sin(angle*6+radius/45) * 22
			var c color.NRGBA
			switch {
			case radius < 165+wave:
				c = color.NRGBA{R: 255, G: 199, B: 92, A: 255}
			case radius < 305+wave:
				c = color.NRGBA{R: 104, G: 126, B: 255, A: 245}
			default:
				c = color.NRGBA{R: 82, G: 211, B: 190, A: 230}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// Photo returns an opaque abstract gradient suitable for crop/fit tests.
func Photo() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, Size, Size))
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			fx, fy := float64(x)/Size, float64(y)/Size
			wave := (math.Sin(fx*math.Pi*5+fy*math.Pi*2) + 1) / 2
			r := uint8(25 + 85*fx + 35*wave)
			g := uint8(65 + 135*fy)
			b := uint8(125 + 100*(1-fx) - 35*wave)
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

// Write creates PNG and JPEG fixture files and returns their paths.
func Write(dir string) (string, string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	pngPath, jpgPath := filepath.Join(dir, "abstract.png"), filepath.Join(dir, "abstract.jpg")
	pngFile, err := os.Create(pngPath)
	if err != nil {
		return "", "", err
	}
	if err := png.Encode(pngFile, Illustration()); err != nil {
		pngFile.Close()
		return "", "", err
	}
	if err := pngFile.Close(); err != nil {
		return "", "", err
	}
	jpgFile, err := os.Create(jpgPath)
	if err != nil {
		return "", "", err
	}
	if err := jpeg.Encode(jpgFile, Photo(), &jpeg.Options{Quality: 88}); err != nil {
		jpgFile.Close()
		return "", "", err
	}
	if err := jpgFile.Close(); err != nil {
		return "", "", err
	}
	return pngPath, jpgPath, nil
}
