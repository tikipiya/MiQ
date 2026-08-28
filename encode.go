package miq

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/tikipiya/MiQ/internal/codec"
)

func Encode(w io.Writer, img image.Image, opts EncodeOptions) error {
	if w == nil {
		return validationError("writer", "must not be nil")
	}
	if img == nil {
		return validationError("image", "must not be nil")
	}
	format := Format(strings.ToLower(string(opts.Format)))
	if format == "" {
		format = PNG
	}
	quality := opts.Quality
	if quality == 0 {
		quality = 92
	}
	if format != PNG && (quality < 1 || quality > 100) {
		return validationError("quality", "must be between 1 and 100")
	}

	switch format {
	case PNG:
		if err := png.Encode(w, img); err != nil {
			return fmt.Errorf("encode PNG: %w", err)
		}
	case JPEG, JPG:
		if err := jpeg.Encode(w, flatten(img), &jpeg.Options{Quality: quality}); err != nil {
			return fmt.Errorf("encode JPEG: %w", err)
		}
	case WebP:
		if err := codec.EncodeWebP(w, img, quality); err != nil {
			return fmt.Errorf("%w: %w", err, ErrRender)
		}
	case AVIF:
		if err := codec.EncodeAVIF(w, img, quality); err != nil {
			return fmt.Errorf("%w: %w", err, ErrRender)
		}
	default:
		return validationError("format", fmt.Sprintf("unknown output format %q", format))
	}
	return nil
}

func CanonicalFormat(format Format) (Format, error) {
	normalized := Format(strings.ToLower(string(format)))
	if normalized == "" {
		normalized = PNG
	}
	if normalized == JPG {
		normalized = JPEG
	}
	switch normalized {
	case PNG, JPEG, WebP, AVIF:
		return normalized, nil
	default:
		return "", validationError("format", fmt.Sprintf("unknown output format %q; use png, jpg, jpeg, webp, or avif", format))
	}
}
func MIMEType(format Format) (string, error) {
	canonical, err := CanonicalFormat(format)
	if err != nil {
		return "", err
	}
	switch canonical {
	case PNG:
		return "image/png", nil
	case JPEG:
		return "image/jpeg", nil
	case WebP:
		return "image/webp", nil
	case AVIF:
		return "image/avif", nil
	}
	panic("unreachable")
}
func EncodeDataURL(img image.Image, opts EncodeOptions) (string, error) {
	data, err := EncodeBytes(img, opts)
	if err != nil {
		return "", err
	}
	mime, err := MIMEType(opts.Format)
	if err != nil {
		return "", err
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func EncodeBytes(img image.Image, opts EncodeOptions) ([]byte, error) {
	var out bytes.Buffer
	if err := Encode(&out, img, opts); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func flatten(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}
