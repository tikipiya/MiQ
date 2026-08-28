package miq

import (
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tikipiya/MiQ/internal/testfixture"
	"github.com/tikipiya/MiQ/theme"
)

func TestCanonicalGalleryVisualCompatibility(t *testing.T) {
	root := repositoryRoot(t)
	avatar := testfixture.Illustration()
	engine, err := NewEngine(EngineOptions{Offline: true, DisableAutoFont: true})
	if err != nil {
		t.Fatal(err)
	}
	const longJapanese = "伝えたい内容を丁寧に整えると、短い文章でも読みやすくなります。" +
		"文字の大きさや行間、余白の取り方を少し変えるだけで、同じ言葉から受ける印象も自然に変わります。"
	base := Quote{Text: longJapanese, Avatar: ImageValue(avatar), Username: "sample_user", DisplayName: "Sample User", Watermark: "Make it a Quote"}
	positionRight := theme.Right
	widthRatio := .32
	gradientOff := false
	tests := []struct {
		name      string
		reference string
		quote     Quote
		options   RenderOptions
	}{
		{"dark", "01-themes/dark-default.png", base, RenderOptions{Theme: theme.Preset(theme.Dark)}},
		{"light", "01-themes/light.png", base, RenderOptions{Theme: theme.Preset(theme.Light)}},
		{"portrait", "01-themes/portrait.png", Quote{Text: "余白が言葉を引き立てる", Avatar: ImageValue(avatar), Username: base.Username, DisplayName: base.DisplayName, Watermark: base.Watermark}, RenderOptions{Theme: theme.Preset(theme.Portrait)}},
		{"avatar-right", "02-layout/avatar-right-text-gradient-and-watermark-all-follow.png", base, RenderOptions{Theme: theme.Input{Avatar: &theme.AvatarInput{Position: &positionRight}}}},
		{"circle", "02-layout/circular-avatar.png", Quote{Text: "小さな工夫が、毎日の使いやすさをつくる。", Avatar: ImageValue(avatar), Username: base.Username, DisplayName: base.DisplayName, Watermark: base.Watermark}, RenderOptions{Theme: theme.Input{Avatar: &theme.AvatarInput{Shape: ptr(theme.Circle), WidthRatio: &widthRatio}, Gradient: &theme.GradientInput{Enabled: &gradientOff}}}},
		{"english", "03-text/english-wraps-at-spaces.png", Quote{Text: "Clear typography gives every sentence enough room to be read, understood, and remembered.", Avatar: ImageValue(avatar), Username: base.Username, DisplayName: base.DisplayName, Watermark: base.Watermark}, RenderOptions{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, renderErr := engine.RenderQuote(context.Background(), test.quote, test.options)
			if renderErr != nil {
				t.Fatal(renderErr)
			}
			want := decodeFixture(t, filepath.Join(root, "docs", "visual", filepath.FromSlash(test.reference)))
			if got.Bounds().Size() != want.Bounds().Size() {
				t.Fatalf("dimensions=%v want %v", got.Bounds().Size(), want.Bounds().Size())
			}
			difference := blockDifference(got, want, 30, 16)
			t.Logf("low-frequency difference %.4f", difference)
			if difference > .08 {
				t.Fatalf("visual structure differs from canonical gallery: %.4f > 0.08", difference)
			}
		})
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate repository")
	}
	return filepath.Dir(file)
}

func decodeFixture(t *testing.T, path string) image.Image {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return decoded
}

func blockDifference(left, right image.Image, columns, rows int) float64 {
	bounds := left.Bounds()
	total := 0.0
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			x0 := bounds.Min.X + column*bounds.Dx()/columns
			x1 := bounds.Min.X + (column+1)*bounds.Dx()/columns
			y0 := bounds.Min.Y + row*bounds.Dy()/rows
			y1 := bounds.Min.Y + (row+1)*bounds.Dy()/rows
			lr, lg, lb := blockAverage(left, x0, y0, x1, y1)
			rr, rg, rb := blockAverage(right, x0, y0, x1, y1)
			total += math.Abs(lr-rr) + math.Abs(lg-rg) + math.Abs(lb-rb)
		}
	}
	return total / float64(columns*rows*3)
}

func blockAverage(img image.Image, x0, y0, x1, y1 int) (float64, float64, float64) {
	r, g, b, count := 0.0, 0.0, 0.0, 0.0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			cr, cg, cb, ca := img.At(x, y).RGBA()
			alpha := float64(ca) / 65535
			r += float64(cr) / 65535 * alpha
			g += float64(cg) / 65535 * alpha
			b += float64(cb) / 65535 * alpha
			count++
		}
	}
	if count == 0 {
		return 0, 0, 0
	}
	return r / count, g / count, b / count
}

func ptr[T any](value T) *T { return &value }
