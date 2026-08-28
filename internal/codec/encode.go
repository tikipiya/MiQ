package codec

import (
	"fmt"
	"image"
	"io"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/webp"
)

func EncodeWebP(w io.Writer, img image.Image, quality int) error {
	if err := webp.Encode(w, img, webp.Options{
		Quality: quality,
		Method:  webp.DefaultMethod,
		Exact:   true,
	}); err != nil {
		return fmt.Errorf("encode WebP: %w", err)
	}
	return nil
}

func EncodeAVIF(w io.Writer, img image.Image, quality int) error {
	if err := avif.Encode(w, img, avif.Options{
		Quality:           quality,
		QualityAlpha:      quality,
		Speed:             8,
		ChromaSubsampling: image.YCbCrSubsampleRatio444,
	}); err != nil {
		return fmt.Errorf("encode AVIF: %w", err)
	}
	return nil
}
