package emoji

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoaderUsesLocalTwemojiOffline(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "72x72"), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(dir, "72x72", "1f600.png"))
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	loader, err := NewLoader(LoaderOptions{
		Client: &http.Client{}, Offline: true, TwemojiDir: dir,
		MaxAssetBytes: 1 << 20, TTL: time.Minute, NegativeTTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	segments := SegmentText("😀", SegmentOptions{})
	images := loader.Prefetch(context.Background(), segments)
	if images[segments[0].URL] == nil {
		t.Fatal("local Twemoji was not loaded")
	}
}
