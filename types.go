package miq

import (
	"image"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/tikipiya/MiQ/theme"
)

const (
	MaxTextLength      = 4000
	MaxNameLength      = 128
	MaxWatermarkLength = 64
	MaxScale           = 8
)

// Quote is the normalized input for one Make it a Quote image.
type Quote struct {
	Text        string
	Avatar      ImageSource
	Username    string
	DisplayName string
	Watermark   string
}

// ImageSource deliberately distinguishes URLs, files, bytes and already
// decoded images. A string is never guessed to be both a path and a URL.
type ImageSource interface {
	isImageSource()
}

type urlImageSource struct{ URL *url.URL }
type fileImageSource struct{ Path string }
type bytesImageSource struct{ Bytes []byte }
type valueImageSource struct{ Image image.Image }

func (urlImageSource) isImageSource()   {}
func (fileImageSource) isImageSource()  {}
func (bytesImageSource) isImageSource() {}
func (valueImageSource) isImageSource() {}

func ImageURL(value *url.URL) ImageSource { return urlImageSource{URL: value} }
func ImageFile(path string) ImageSource   { return fileImageSource{Path: path} }

// ImageBytes copies b so a render cannot race with a caller mutating it.
func ImageBytes(b []byte) ImageSource {
	return bytesImageSource{Bytes: slices.Clone(b)}
}

func ImageValue(value image.Image) ImageSource { return valueImageSource{Image: value} }

type RenderOptions struct {
	Theme             theme.Input
	Scale             float64
	Misskey           MisskeyOptions
	OnAssetError      MissingAssetBehavior
	BackgroundImage   ImageSource
	BackgroundFit     theme.Fit
	BackgroundOpacity *float64
	SizeToAvatar      AvatarSizeAxis
}

type AvatarSizeAxis string

const (
	AvatarNativeWidth  AvatarSizeAxis = "width"
	AvatarNativeHeight AvatarSizeAxis = "height"
)

type ConversationMessage struct {
	Text        string
	Username    string
	DisplayName string
	Avatar      ImageSource
}

type ConversationTheme string

const (
	ConversationDark  ConversationTheme = "dark"
	ConversationLight ConversationTheme = "light"
)

type ConversationOptions struct {
	Theme        ConversationTheme
	Width        int
	Misskey      MisskeyOptions
	OnAssetError MissingAssetBehavior
}

type MisskeyOptions struct {
	Instances []string
	Remote    *bool
}

type MissingAssetBehavior string

const (
	AssetAsText MissingAssetBehavior = "text"
	AssetIgnore MissingAssetBehavior = "ignore"
	AssetThrow  MissingAssetBehavior = "throw"
)

// EngineOptions controls shared I/O policy. More cache and font options will
// be added behind Engine without changing RenderQuote's signature.
type EngineOptions struct {
	HTTPClient          *http.Client
	Offline             bool
	MaxAssetBytes       int64
	Fonts               []FontFace
	FontCacheDir        string
	DisableAutoFont     bool
	StrictFonts         bool
	TwemojiCacheDir     string
	AllowPrivateNetwork bool
	MaxImagePixels      int64
	ImageCacheEntries   int
	ImageCacheTTL       time.Duration
	ImageFailureTTL     time.Duration
	DisableImageCache   bool
}

// FontFace registers one regular font face under Family for the lifetime of
// an Engine. Data is copied by NewEngine.
type FontFace struct {
	Family string
	Data   []byte
}

type Format string

const (
	PNG  Format = "png"
	JPEG Format = "jpeg"
	JPG  Format = "jpg"
	WebP Format = "webp"
	AVIF Format = "avif"
)

type EncodeOptions struct {
	Format  Format
	Quality int
}
