package miq

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "github.com/gen2brain/avif"
	_ "github.com/gen2brain/webp"
	internaldraw "github.com/tikipiya/MiQ/internal/draw"
	internalemoji "github.com/tikipiya/MiQ/internal/emoji"
	internalfont "github.com/tikipiya/MiQ/internal/font"
	internalnet "github.com/tikipiya/MiQ/internal/netpolicy"
	"github.com/tikipiya/MiQ/theme"
	_ "golang.org/x/image/webp"
)

const defaultMaxAssetBytes int64 = 16 << 20

// Engine owns shared rendering, network, font, and emoji policy and is safe
// for concurrent render calls.
type Engine struct {
	httpClient          *http.Client
	offline             bool
	maxAssetBytes       int64
	fonts               *internaldraw.FontRegistry
	fontManager         *internalfont.Manager
	autoFont            bool
	strictFonts         bool
	emojiLoader         *internalemoji.Loader
	allowPrivateNetwork bool
	maxImagePixels      int64
	imageCacheEnabled   bool
	imageCacheEntries   int
	imageCacheTTL       time.Duration
	imageFailureTTL     time.Duration
	imageMu             sync.Mutex
	imageCache          map[string]cachedImage
	imageFailures       map[string]cachedImageFailure
	imageInFlight       map[string]*imageLoadCall
}

type cachedImage struct {
	image   image.Image
	used    time.Time
	expires time.Time
}

type cachedImageFailure struct {
	err     error
	expires time.Time
}

type imageLoadCall struct {
	done  chan struct{}
	image image.Image
	err   error
}

func NewEngine(opts EngineOptions) (*Engine, error) {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	maxBytes := opts.MaxAssetBytes
	if maxBytes == 0 {
		maxBytes = defaultMaxAssetBytes
	}
	if maxBytes < 0 {
		return nil, validationError("maxAssetBytes", "must not be negative")
	}
	if opts.MaxImagePixels < 0 {
		return nil, validationError("maxImagePixels", "must not be negative")
	}
	cacheEntries := opts.ImageCacheEntries
	if cacheEntries == 0 {
		cacheEntries = 256
	}
	if cacheEntries < 0 {
		return nil, validationError("imageCacheEntries", "must not be negative")
	}
	cacheTTL := opts.ImageCacheTTL
	if cacheTTL == 0 {
		cacheTTL = time.Hour
	}
	failureTTL := opts.ImageFailureTTL
	if failureTTL == 0 {
		failureTTL = time.Minute
	}
	if cacheTTL < 0 || failureTTL < 0 {
		return nil, validationError("imageCacheTTL", "must not be negative")
	}
	fontData := make([]internaldraw.FontData, len(opts.Fonts))
	for i, face := range opts.Fonts {
		fontData[i] = internaldraw.FontData{Family: face.Family, Bytes: face.Data}
	}
	fonts, err := internaldraw.NewFontRegistry(fontData)
	if err != nil {
		return nil, &FieldError{Field: "fonts", Err: fmt.Errorf("%v: %w", err, ErrFont)}
	}
	fontManager, err := internalfont.NewManager(internalfont.Options{
		Client: client, Registry: fonts, CacheDir: internalfont.ResolveCacheDir(opts.FontCacheDir),
		Offline: opts.Offline, MaxAssetBytes: maxBytes,
	})
	if err != nil {
		return nil, &FieldError{Field: "fonts", Err: fmt.Errorf("%v: %w", err, ErrFont)}
	}
	fontCacheDir := internalfont.ResolveCacheDir(opts.FontCacheDir)
	emojiLoader, err := internalemoji.NewLoader(internalemoji.LoaderOptions{
		Client: client, Offline: opts.Offline,
		TwemojiDir:          internalemoji.ResolveTwemojiDir(opts.TwemojiCacheDir, fontCacheDir),
		MaxAssetBytes:       maxBytes,
		AllowPrivateNetwork: opts.AllowPrivateNetwork,
	})
	if err != nil {
		return nil, &FieldError{Field: "emoji", Err: fmt.Errorf("%v: %w", err, ErrAsset)}
	}
	return &Engine{
		httpClient: client, offline: opts.Offline, maxAssetBytes: maxBytes, fonts: fonts,
		fontManager: fontManager, autoFont: !opts.DisableAutoFont, strictFonts: opts.StrictFonts,
		emojiLoader: emojiLoader, allowPrivateNetwork: opts.AllowPrivateNetwork,
		imageCacheEnabled: !opts.DisableImageCache && cacheEntries > 0,
		imageCacheEntries: cacheEntries, imageCacheTTL: cacheTTL, imageFailureTTL: failureTTL,
		imageCache: make(map[string]cachedImage), imageFailures: make(map[string]cachedImageFailure), imageInFlight: make(map[string]*imageLoadCall),
		maxImagePixels: func() int64 {
			if opts.MaxImagePixels > 0 {
				return opts.MaxImagePixels
			}
			return 40_000_000
		}(),
	}, nil
}

func (e *Engine) RenderQuote(ctx context.Context, quote Quote, opts RenderOptions) (*image.NRGBA, error) {
	if ctx == nil {
		return nil, validationError("context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateQuote(quote); err != nil {
		return nil, err
	}

	t, err := theme.Resolve(opts.Theme)
	if err != nil {
		return nil, &FieldError{Field: "theme", Err: fmt.Errorf("%v: %w", err, ErrValidation)}
	}
	scale := opts.Scale
	if scale == 0 {
		scale = 1
	}
	if scale <= 0 || scale > MaxScale {
		return nil, validationError("scale", fmt.Sprintf("must be greater than zero and at most %d", MaxScale))
	}
	t.Width = max(1, int(float64(t.Width)*scale+0.5))
	t.Height = max(1, int(float64(t.Height)*scale+0.5))
	if e.autoFont {
		if err := e.fontManager.EnsureStack(ctx, t.Text.Font); err != nil && e.strictFonts {
			return nil, &FieldError{Field: "theme.text.font", Err: fmt.Errorf("%v: %w", err, ErrFont)}
		}
	}
	segments := internalemoji.SegmentText(quote.Text, internalemoji.SegmentOptions{
		Misskey: internalemoji.MisskeyOptions{
			Instances: opts.Misskey.Instances,
			Remote:    opts.Misskey.Remote,
		},
	})
	emojiImages := e.emojiLoader.Prefetch(ctx, segments)
	behavior := string(opts.OnAssetError)
	if behavior == "" {
		behavior = string(AssetAsText)
	}
	segments, err = internalemoji.ResolveMissing(segments, func(source string) bool {
		return emojiImages[source] != nil
	}, behavior)
	if err != nil {
		return nil, &AssetError{Source: "emoji", Err: fmt.Errorf("%v: %w", err, ErrAsset)}
	}

	avatar, err := e.resolveImage(ctx, quote.Avatar)
	if err != nil {
		return nil, err
	}
	if opts.SizeToAvatar != "" {
		if opts.SizeToAvatar != AvatarNativeWidth && opts.SizeToAvatar != AvatarNativeHeight {
			return nil, validationError("sizeToAvatar", "must be width or height")
		}
		t = sizeThemeToAvatar(t, avatar, opts.SizeToAvatar)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	background, err := e.resolveImage(ctx, opts.BackgroundImage)
	if err != nil {
		return nil, &FieldError{Field: "backgroundImage", Err: err}
	}
	backgroundFit := opts.BackgroundFit
	if backgroundFit == "" {
		backgroundFit = theme.Cover
	}
	if backgroundFit != theme.Cover && backgroundFit != theme.Contain {
		return nil, validationError("backgroundFit", "must be cover or contain")
	}
	backgroundOpacity := 1.0
	if opts.BackgroundOpacity != nil {
		backgroundOpacity = *opts.BackgroundOpacity
	}
	if backgroundOpacity < 0 || backgroundOpacity > 1 {
		return nil, validationError("backgroundOpacity", "must be between 0 and 1")
	}

	result, err := internaldraw.Render(internaldraw.Quote{
		Text: quote.Text, Segments: segments, EmojiImages: emojiImages, Username: quote.Username,
		DisplayName: quote.DisplayName, Watermark: quote.Watermark, Background: background, BackgroundFit: backgroundFit, BackgroundOpacity: backgroundOpacity,
	}, avatar, t, e.fonts)
	if err != nil {
		return nil, fmt.Errorf("render quote: %w: %w", err, ErrRender)
	}
	return result, nil
}

func sizeThemeToAvatar(t theme.Theme, avatar image.Image, axis AvatarSizeAxis) theme.Theme {
	if avatar == nil || avatar.Bounds().Dx() <= 0 || avatar.Bounds().Dy() <= 0 {
		return t
	}
	boxWidth := float64(t.Width) * t.Avatar.WidthRatio
	if t.Layout == theme.Stacked {
		boxWidth = float64(t.Width)
	}
	boxHeight := float64(t.Height)
	if boxWidth <= 0 || boxHeight <= 0 {
		return t
	}
	factor := float64(avatar.Bounds().Dy()) / boxHeight
	if axis == AvatarNativeWidth {
		factor = float64(avatar.Bounds().Dx()) / boxWidth
	}
	if math.IsNaN(factor) || math.IsInf(factor, 0) || factor <= 0 {
		return t
	}
	t.Width = max(1, int(math.Round(float64(t.Width)*factor)))
	t.Height = max(1, int(math.Round(float64(t.Height)*factor)))
	return t
}

func (e *Engine) WriteQuote(
	ctx context.Context,
	w io.Writer,
	quote Quote,
	opts RenderOptions,
	enc EncodeOptions,
) error {
	if w == nil {
		return validationError("writer", "must not be nil")
	}
	img, err := e.RenderQuote(ctx, quote, opts)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return Encode(w, img, enc)
}

func validateQuote(q Quote) error {
	if strings.TrimSpace(q.Text) == "" {
		return validationError("text", "must not be empty")
	}
	for _, field := range []struct {
		name  string
		value string
		max   int
	}{
		{"text", q.Text, MaxTextLength},
		{"username", q.Username, MaxNameLength},
		{"displayName", q.DisplayName, MaxNameLength},
		{"watermark", q.Watermark, MaxWatermarkLength},
	} {
		if utf16Length(field.value) > field.max {
			return validationError(field.name, fmt.Sprintf("must be at most %d UTF-16 code units", field.max))
		}
	}
	return nil
}

func utf16Length(value string) int {
	length := 0
	for len(value) > 0 {
		r, size := utf8.DecodeRuneInString(value)
		value = value[size:]
		if r > 0xffff {
			length += 2
		} else {
			length++
		}
	}
	return length
}

func (e *Engine) resolveImage(ctx context.Context, source ImageSource) (image.Image, error) {
	key := imageSourceCacheKey(source)
	if key == "" || !e.imageCacheEnabled {
		return e.loadImage(ctx, source)
	}
	now := time.Now()
	e.imageMu.Lock()
	if entry, ok := e.imageCache[key]; ok && (entry.expires.IsZero() || now.Before(entry.expires)) {
		entry.used = now
		e.imageCache[key] = entry
		e.imageMu.Unlock()
		return entry.image, nil
	}
	if failure, ok := e.imageFailures[key]; ok && now.Before(failure.expires) {
		e.imageMu.Unlock()
		return nil, failure.err
	}
	if call := e.imageInFlight[key]; call != nil {
		e.imageMu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-call.done:
			return call.image, call.err
		}
	}
	call := &imageLoadCall{done: make(chan struct{})}
	e.imageInFlight[key] = call
	e.imageMu.Unlock()

	call.image, call.err = e.loadImage(ctx, source)
	e.imageMu.Lock()
	delete(e.imageInFlight, key)
	if call.err != nil {
		if e.imageFailureTTL > 0 && !errors.Is(call.err, context.Canceled) && !errors.Is(call.err, context.DeadlineExceeded) {
			e.imageFailures[key] = cachedImageFailure{err: call.err, expires: now.Add(e.imageFailureTTL)}
		}
	} else if call.image != nil {
		e.evictImageCache()
		expires := time.Time{}
		if e.imageCacheTTL > 0 {
			expires = now.Add(e.imageCacheTTL)
		}
		e.imageCache[key] = cachedImage{image: call.image, used: now, expires: expires}
	}
	close(call.done)
	e.imageMu.Unlock()
	return call.image, call.err
}

func (e *Engine) loadImage(ctx context.Context, source ImageSource) (image.Image, error) {
	if source == nil {
		return nil, nil
	}
	var (
		reader io.Reader
		closer io.Closer
		label  string
	)

	switch value := source.(type) {
	case urlImageSource:
		if value.URL == nil {
			return nil, validationError("avatar", "URL must not be nil")
		}
		if e.offline {
			return nil, &AssetError{Source: value.URL.Redacted(), Err: fmt.Errorf("network disabled: %w", ErrAsset)}
		}
		if err := internalnet.Validate(ctx, value.URL, e.allowPrivateNetwork); err != nil {
			return nil, validationError("avatar", err.Error())
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, value.URL.String(), nil)
		if err != nil {
			return nil, &AssetError{Source: value.URL.Redacted(), Err: fmt.Errorf("create request: %w: %w", err, ErrAsset)}
		}
		req.Header.Set("User-Agent", "makeitaquote-go/dev")
		resp, err := e.httpClient.Do(req)
		if err != nil {
			return nil, &AssetError{Source: value.URL.Redacted(), Err: fmt.Errorf("fetch: %w: %w", err, ErrAsset)}
		}
		if err := internalnet.Validate(ctx, resp.Request.URL, e.allowPrivateNetwork); err != nil {
			resp.Body.Close()
			return nil, &AssetError{Source: value.URL.Redacted(), Err: fmt.Errorf("redirect target: %v: %w", err, ErrAsset)}
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			resp.Body.Close()
			return nil, &AssetError{Source: value.URL.Redacted(), Err: fmt.Errorf("HTTP %d: %w", resp.StatusCode, ErrAsset)}
		}
		reader, closer, label = resp.Body, resp.Body, value.URL.Redacted()
	case fileImageSource:
		if strings.TrimSpace(value.Path) == "" {
			return nil, validationError("avatar", "file path must not be empty")
		}
		file, err := os.Open(value.Path)
		if err != nil {
			return nil, &AssetError{Source: value.Path, Err: fmt.Errorf("open: %w: %w", err, ErrAsset)}
		}
		reader, closer, label = file, file, value.Path
	case bytesImageSource:
		reader, label = bytes.NewReader(value.Bytes), "bytes"
	case valueImageSource:
		if value.Image == nil {
			return nil, validationError("avatar", "image must not be nil")
		}
		if err := e.validateImageDimensions(value.Image.Bounds().Dx(), value.Image.Bounds().Dy()); err != nil {
			return nil, &AssetError{Source: "image", Err: err}
		}
		return cloneImage(value.Image), nil
	default:
		return nil, validationError("avatar", "unknown image source")
	}
	if closer != nil {
		defer closer.Close()
	}

	limited := &io.LimitedReader{R: reader, N: e.maxAssetBytes + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, &AssetError{Source: label, Err: fmt.Errorf("read: %w: %w", err, ErrAsset)}
	}
	if int64(len(data)) > e.maxAssetBytes {
		return nil, &AssetError{Source: label, Err: fmt.Errorf("exceeds %d bytes: %w", e.maxAssetBytes, ErrAsset)}
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, &AssetError{Source: label, Err: fmt.Errorf("decode header: %w: %w", err, ErrAsset)}
	}
	if err := e.validateImageDimensions(config.Width, config.Height); err != nil {
		return nil, &AssetError{Source: label, Err: err}
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, &AssetError{Source: label, Err: fmt.Errorf("decode: %w: %w", err, ErrAsset)}
	}
	return cloneImage(decoded), nil
}

func imageSourceCacheKey(source ImageSource) string {
	switch value := source.(type) {
	case urlImageSource:
		if value.URL != nil {
			return "url:" + value.URL.String()
		}
	case fileImageSource:
		if absolute, err := filepath.Abs(value.Path); err == nil {
			return "file:" + absolute
		}
	}
	return ""
}

func (e *Engine) evictImageCache() {
	for len(e.imageCache) >= e.imageCacheEntries && len(e.imageCache) > 0 {
		oldestKey := ""
		var oldest time.Time
		for key, entry := range e.imageCache {
			if oldestKey == "" || entry.used.Before(oldest) {
				oldestKey, oldest = key, entry.used
			}
		}
		delete(e.imageCache, oldestKey)
	}
}

func (e *Engine) validateImageDimensions(width, height int) error {
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || int64(width)*int64(height) > e.maxImagePixels {
		return fmt.Errorf("image dimensions %dx%d exceed limits: %w", width, height, ErrAsset)
	}
	return nil
}

func cloneImage(src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.SetNRGBA(x, y, color.NRGBAModel.Convert(src.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA))
		}
	}
	return dst
}
