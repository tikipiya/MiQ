package miq

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/webp"
	"github.com/tikipiya/MiQ/theme"
	"golang.org/x/image/font/gofont/goregular"
)

func TestNewEngineRejectsNegativeAssetLimit(t *testing.T) {
	_, err := NewEngine(EngineOptions{MaxAssetBytes: -1})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestRenderQuoteValidatesUTF16Length(t *testing.T) {
	engine, err := NewEngine(EngineOptions{Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.RenderQuote(context.Background(), Quote{Text: strings.Repeat("😀", 2001)}, RenderOptions{})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	var field *FieldError
	if !errors.As(err, &field) || field.Field != "text" {
		t.Fatalf("expected text FieldError, got %#v", field)
	}
}

func TestRenderQuoteProducesExpectedCanvas(t *testing.T) {
	engine, err := NewEngine(EngineOptions{Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	img, err := engine.RenderQuote(context.Background(), Quote{
		Text:        "A quote rendered by Go.",
		Username:    "sample_user",
		DisplayName: "Make it a Quote",
	}, RenderOptions{Theme: theme.Preset(theme.Dark)})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := img.Bounds().Dx(), 1200; got != want {
		t.Fatalf("width = %d, want %d", got, want)
	}
	if got, want := img.Bounds().Dy(), 630; got != want {
		t.Fatalf("height = %d, want %d", got, want)
	}

	encoded, err := EncodeBytes(img, EncodeOptions{Format: PNG})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(encoded, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatal("output does not have a PNG signature")
	}
	decoded, err := png.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != img.Bounds() {
		t.Fatalf("decoded bounds = %v, want %v", decoded.Bounds(), img.Bounds())
	}
}

func TestSizeToAvatarCompatibility(t *testing.T) {
	resolved, err := theme.Resolve(theme.Preset(theme.Dark))
	if err != nil {
		t.Fatal(err)
	}
	avatar := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	byHeight := sizeThemeToAvatar(resolved, avatar, AvatarNativeHeight)
	if byHeight.Height != 1024 || math.Abs(float64(byHeight.Width)/float64(byHeight.Height)-float64(resolved.Width)/float64(resolved.Height)) > .002 {
		t.Fatalf("height sizing=%dx%d", byHeight.Width, byHeight.Height)
	}
	byWidth := sizeThemeToAvatar(resolved, avatar, AvatarNativeWidth)
	if got := float64(byWidth.Width) * byWidth.Avatar.WidthRatio; math.Abs(got-1024) > 1 {
		t.Fatalf("avatar width=%f canvas=%dx%d", got, byWidth.Width, byWidth.Height)
	}
	portrait, _ := theme.Resolve(theme.Preset(theme.Portrait))
	stacked := sizeThemeToAvatar(portrait, avatar, AvatarNativeWidth)
	if stacked.Width != 1024 {
		t.Fatalf("stacked width=%d", stacked.Width)
	}
	if got := sizeThemeToAvatar(resolved, nil, AvatarNativeHeight); got.Width != resolved.Width || got.Height != resolved.Height {
		t.Fatal("nil avatar changed theme")
	}
}

func TestRenderQuoteLoadsAndDesaturatesAvatarBytes(t *testing.T) {
	avatar := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			avatar.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var source bytes.Buffer
	if err := png.Encode(&source, avatar); err != nil {
		t.Fatal(err)
	}

	engine, err := NewEngine(EngineOptions{Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	img, err := engine.RenderQuote(context.Background(), Quote{
		Text: "avatar", Avatar: ImageBytes(source.Bytes()),
	}, RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pixel := img.NRGBAAt(10, img.Bounds().Dy()/2)
	if pixel.R != pixel.G || pixel.G != pixel.B {
		t.Fatalf("avatar pixel was not grayscale: %#v", pixel)
	}
}

func TestRenderQuoteHonorsOfflineMode(t *testing.T) {
	engine, err := NewEngine(EngineOptions{Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	avatarURL, err := url.Parse("https://example.com/avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.RenderQuote(context.Background(), Quote{
		Text: "offline", Avatar: ImageURL(avatarURL),
	}, RenderOptions{})
	if !errors.Is(err, ErrAsset) {
		t.Fatalf("expected asset error, got %v", err)
	}
}

func TestRenderQuoteRejectsPrivateNetwork(t *testing.T) {
	engine, err := NewEngine(EngineOptions{DisableAutoFont: true})
	if err != nil {
		t.Fatal(err)
	}
	privateURL, _ := url.Parse("http://127.0.0.1/avatar.png")
	_, err = engine.RenderQuote(context.Background(), Quote{Text: "private", Avatar: ImageURL(privateURL)}, RenderOptions{})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestRenderQuoteRejectsLargeImageValue(t *testing.T) {
	engine, err := NewEngine(EngineOptions{Offline: true, MaxImagePixels: 4})
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.RenderQuote(context.Background(), Quote{Text: "large", Avatar: ImageValue(image.NewNRGBA(image.Rect(0, 0, 3, 3)))}, RenderOptions{})
	if !errors.Is(err, ErrAsset) {
		t.Fatalf("expected asset error, got %v", err)
	}
}

func TestRenderQuoteHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	engine, err := NewEngine(EngineOptions{Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.RenderQuote(ctx, Quote{Text: "canceled"}, RenderOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestEncodeJPEG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var out bytes.Buffer
	if err := Encode(&out, img, EncodeOptions{Format: JPEG, Quality: 75}); err != nil {
		t.Fatal(err)
	}
	if _, err := jpeg.Decode(bytes.NewReader(out.Bytes())); err != nil {
		t.Fatalf("decode JPEG: %v", err)
	}
}

func TestOutputFormatCompatibility(t *testing.T) {
	for input, want := range map[Format]Format{PNG: PNG, JPEG: JPEG, JPG: JPEG, WebP: WebP, AVIF: AVIF, "JPG": JPEG, "PNG": PNG} {
		got, err := CanonicalFormat(input)
		if err != nil || got != want {
			t.Fatalf("CanonicalFormat(%q)=%q,%v want %q", input, got, err, want)
		}
	}
	if _, err := CanonicalFormat("bmp"); !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "jpg") {
		t.Fatalf("error=%v", err)
	}
	if mime, err := MIMEType(JPG); err != nil || mime != "image/jpeg" {
		t.Fatalf("mime=%q err=%v", mime, err)
	}
}

func TestEncodeDataURLUsesCanonicalMIME(t *testing.T) {
	value, err := EncodeDataURL(solidImage(2, 2, color.NRGBA{R: 1, A: 255}), EncodeOptions{Format: JPG, Quality: 80})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(value, "data:image/jpeg;base64,") {
		t.Fatalf("url=%q", value[:min(len(value), 40)])
	}
}

func TestEncodeCaseInsensitiveAndQuality(t *testing.T) {
	img := solidImage(16, 16, color.NRGBA{R: 0x33, G: 0x66, B: 0xff, A: 255})
	upper, err := EncodeBytes(img, EncodeOptions{Format: "JPG", Quality: 80})
	if err != nil {
		t.Fatal(err)
	}
	if len(upper) < 2 || upper[0] != 0xff || upper[1] != 0xd8 {
		t.Fatal("uppercase JPG did not encode JPEG")
	}
	low, err := EncodeBytes(img, EncodeOptions{Format: JPG, Quality: 10})
	if err != nil {
		t.Fatal(err)
	}
	high, err := EncodeBytes(img, EncodeOptions{Format: JPG, Quality: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(low) >= len(high) {
		t.Fatalf("low=%d high=%d", len(low), len(high))
	}
	for _, quality := range []int{-1, 101} {
		if _, err := EncodeBytes(img, EncodeOptions{Format: JPG, Quality: quality}); !errors.Is(err, ErrValidation) {
			t.Fatalf("quality %d error=%v", quality, err)
		}
	}
}

func TestEncodePNGIgnoresQuality(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	if _, err := EncodeBytes(img, EncodeOptions{Format: PNG, Quality: -100}); err != nil {
		t.Fatalf("PNG quality should be ignored: %v", err)
	}
}

func TestEncodeWebPRoundTrip(t *testing.T) {
	img := solidImage(8, 8, color.NRGBA{R: 220, G: 30, B: 40, A: 255})
	encoded, err := EncodeBytes(img, EncodeOptions{Format: WebP, Quality: 80})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(encoded, []byte("RIFF")) || !bytes.Equal(encoded[8:12], []byte("WEBP")) {
		t.Fatal("output does not have a WebP container signature")
	}
	decoded, err := webp.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != img.Bounds() {
		t.Fatalf("decoded bounds = %v, want %v", decoded.Bounds(), img.Bounds())
	}
}

func TestEncodeAVIFRoundTrip(t *testing.T) {
	img := solidImage(8, 8, color.NRGBA{R: 20, G: 130, B: 240, A: 255})
	encoded, err := EncodeBytes(img, EncodeOptions{Format: AVIF, Quality: 70})
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) < 12 || !bytes.Equal(encoded[4:12], []byte("ftypavif")) {
		t.Fatal("output does not have an AVIF container signature")
	}
	decoded, err := avif.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != img.Bounds() {
		t.Fatalf("decoded bounds = %v, want %v", decoded.Bounds(), img.Bounds())
	}
}

func TestEngineAcceptsCustomFont(t *testing.T) {
	engine, err := NewEngine(EngineOptions{Fonts: []FontFace{{Family: "Test Sans", Data: goregular.TTF}}})
	if err != nil {
		t.Fatal(err)
	}
	font := "Test Sans"
	img, err := engine.RenderQuote(context.Background(), Quote{Text: "custom font"}, RenderOptions{
		Theme: theme.Input{Text: &theme.TextInput{Font: &font}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 1200 {
		t.Fatalf("width = %d", img.Bounds().Dx())
	}
}

func TestEngineRejectsInvalidCustomFont(t *testing.T) {
	_, err := NewEngine(EngineOptions{Fonts: []FontFace{{Family: "broken", Data: []byte("not a font")}}})
	if !errors.Is(err, ErrFont) {
		t.Fatalf("expected font error, got %v", err)
	}
}

func TestEngineCanRenderConcurrently(t *testing.T) {
	engine, err := NewEngine(EngineOptions{Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 4
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			img, renderErr := engine.RenderQuote(context.Background(), Quote{Text: "parallel render"}, RenderOptions{})
			if renderErr == nil && img.Bounds() != image.Rect(0, 0, 1200, 630) {
				renderErr = errors.New("unexpected image bounds")
			}
			errs <- renderErr
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestEngineCachesAndCoalescesRemoteImages(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, solidImage(4, 4, color.NRGBA{R: 255, A: 255})); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		time.Sleep(40 * time.Millisecond)
		_, _ = w.Write(encoded.Bytes())
	}))
	defer server.Close()
	remote, _ := url.Parse(server.URL + "/avatar.png")
	engine, err := NewEngine(EngineOptions{HTTPClient: server.Client(), AllowPrivateNetwork: true, DisableAutoFont: true})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, renderErr := engine.RenderQuote(context.Background(), Quote{Text: "cache", Avatar: ImageURL(remote)}, RenderOptions{})
			errs <- renderErr
		}()
	}
	close(start)
	for range 2 {
		if renderErr := <-errs; renderErr != nil {
			t.Fatal(renderErr)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("coalesced requests=%d want 1", requests.Load())
	}
	if _, err := engine.RenderQuote(context.Background(), Quote{Text: "cached", Avatar: ImageURL(remote)}, RenderOptions{}); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatalf("cached requests=%d want 1", requests.Load())
	}
}

func TestEngineCachesRemoteImageFailuresAndCanDisableCache(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()
	remote, _ := url.Parse(server.URL + "/missing.png")
	engine, _ := NewEngine(EngineOptions{HTTPClient: server.Client(), AllowPrivateNetwork: true, DisableAutoFont: true})
	for range 2 {
		if _, err := engine.RenderQuote(context.Background(), Quote{Text: "failure", Avatar: ImageURL(remote)}, RenderOptions{}); !errors.Is(err, ErrAsset) {
			t.Fatalf("failure error=%v", err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("negative cache requests=%d want 1", requests.Load())
	}
	disabled, _ := NewEngine(EngineOptions{HTTPClient: server.Client(), AllowPrivateNetwork: true, DisableAutoFont: true, DisableImageCache: true})
	for range 2 {
		_, _ = disabled.RenderQuote(context.Background(), Quote{Text: "failure", Avatar: ImageURL(remote)}, RenderOptions{})
	}
	if requests.Load() != 3 {
		t.Fatalf("disabled cache requests=%d want 3", requests.Load())
	}
}

func TestEngineDrawsOfflineTwemoji(t *testing.T) {
	dir := t.TempDir()
	file, err := os.Create(filepath.Join(dir, "1f600.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, solidImage(16, 16, color.NRGBA{R: 255, A: 255})); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(EngineOptions{Offline: true, TwemojiCacheDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	img, err := engine.RenderQuote(context.Background(), Quote{Text: "😀"}, RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	red := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			c := img.NRGBAAt(x, y)
			if c.R > 200 && c.G < 30 && c.B < 30 {
				red++
			}
		}
	}
	if red == 0 {
		t.Fatal("rendered image contains no Twemoji pixels")
	}
}

func TestRenderQuoteBackgroundImageAndGradient(t *testing.T) {
	engine, err := NewEngine(EngineOptions{Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	width, height := 120, 80
	background := color.NRGBA{A: 255}
	textColor := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	size, minSize := 18.0, 10.0
	input := theme.Input{
		Extends: theme.Custom, Width: &width, Height: &height, Background: &background,
		Text: &theme.TextInput{Color: &textColor, Size: &size, MinSize: &minSize},
		BackgroundGradient: &theme.BackgroundGradient{Type: theme.LinearGradient, Direction: theme.GradientHorizontal, Stops: []theme.ColorStop{
			{Color: color.NRGBA{R: 255, A: 255}, Offset: 0}, {Color: color.NRGBA{B: 255, A: 255}, Offset: 1},
		}},
	}
	bg := solidImage(1, 1, color.NRGBA{G: 255, A: 128})
	opacity := 0.5
	img, err := engine.RenderQuote(context.Background(), Quote{Text: "x"}, RenderOptions{Theme: input, BackgroundImage: ImageValue(bg), BackgroundOpacity: &opacity})
	if err != nil {
		t.Fatal(err)
	}
	left, right := img.NRGBAAt(0, 0), img.NRGBAAt(width-1, 0)
	if left == right || left.G == 0 || right.G == 0 {
		t.Fatalf("background layers were not rendered: left=%v right=%v", left, right)
	}
}

func TestRenderConversation(t *testing.T) {
	engine, err := NewEngine(EngineOptions{Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	img, err := engine.RenderConversation(context.Background(), []ConversationMessage{
		{Text: "first message", Username: "a", DisplayName: "A"},
		{Text: "second message", Username: "a"},
		{Text: "third", Username: "b"},
	}, ConversationOptions{Theme: ConversationLight, Width: 320})
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 320 || img.Bounds().Dy() <= 40 {
		t.Fatalf("unexpected bounds %v", img.Bounds())
	}
}

func TestConversationValidationCompatibility(t *testing.T) {
	engine, err := NewEngine(EngineOptions{Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		messages []ConversationMessage
		field    string
	}{{"empty", nil, "messages"}, {"blank text", []ConversationMessage{{Text: "  ", Username: "a"}}, "messages[0].text"}, {"blank username", []ConversationMessage{{Text: "hi", Username: " "}}, "messages[0].username"}, {"long text", []ConversationMessage{{Text: strings.Repeat("x", MaxTextLength+1), Username: "a"}}, "messages[0].text"}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := engine.RenderConversation(context.Background(), test.messages, ConversationOptions{})
			if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), test.field) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func solidImage(width, height int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}
