package emoji

import (
	"bytes"
	"context"
	"fmt"
	internalnet "github.com/tikipiya/MiQ/internal/netpolicy"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/gen2brain/avif"
	_ "github.com/gen2brain/webp"
)

type LoaderOptions struct {
	Client              *http.Client
	Offline             bool
	TwemojiDir          string
	MaxAssetBytes       int64
	MaxEntries          int
	TTL                 time.Duration
	NegativeTTL         time.Duration
	Concurrency         int
	AllowPrivateNetwork bool
}

type Loader struct {
	client              *http.Client
	offline             bool
	twemojiDir          string
	maxAssetBytes       int64
	maxEntries          int
	ttl                 time.Duration
	negativeTTL         time.Duration
	concurrency         int
	allowPrivateNetwork bool

	mu       sync.Mutex
	images   map[string]imageEntry
	failures map[string]time.Time
	inFlight map[string]*imageCall
}

type imageEntry struct {
	image   image.Image
	used    time.Time
	expires time.Time
}

type imageCall struct {
	done chan struct{}
	img  image.Image
}

func NewLoader(opts LoaderOptions) (*Loader, error) {
	if opts.Client == nil {
		return nil, fmt.Errorf("emoji loader requires HTTP client")
	}
	if opts.MaxAssetBytes <= 0 {
		return nil, fmt.Errorf("emoji byte limit must be positive")
	}
	if opts.MaxEntries == 0 {
		opts.MaxEntries = 256
	}
	if opts.TTL == 0 {
		opts.TTL = time.Hour
	}
	if opts.NegativeTTL == 0 {
		opts.NegativeTTL = time.Minute
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 8
	}
	return &Loader{
		client: opts.Client, offline: opts.Offline, twemojiDir: opts.TwemojiDir,
		maxAssetBytes: opts.MaxAssetBytes, maxEntries: opts.MaxEntries, ttl: opts.TTL,
		negativeTTL: opts.NegativeTTL, concurrency: opts.Concurrency, allowPrivateNetwork: opts.AllowPrivateNetwork,
		images: make(map[string]imageEntry), failures: make(map[string]time.Time), inFlight: make(map[string]*imageCall),
	}, nil
}

func (l *Loader) Prefetch(ctx context.Context, segments []Segment) map[string]image.Image {
	candidates := make(map[string][]string)
	for _, segment := range segments {
		if !segment.IsEmoji() {
			continue
		}
		if _, exists := candidates[segment.URL]; !exists {
			candidates[segment.URL] = append([]string{segment.URL}, segment.AlternativeURLs...)
		}
	}
	images := make(map[string]image.Image)
	var mu sync.Mutex
	sem := make(chan struct{}, l.concurrency)
	var wg sync.WaitGroup
	for key, urls := range candidates {
		key, urls := key, urls
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			for _, candidate := range urls {
				img := l.Load(ctx, candidate)
				if img != nil {
					mu.Lock()
					images[key] = img
					mu.Unlock()
					return
				}
			}
		}()
	}
	wg.Wait()
	return images
}

func (l *Loader) Load(ctx context.Context, source string) image.Image {
	now := time.Now()
	l.mu.Lock()
	if entry, ok := l.images[source]; ok && now.Before(entry.expires) {
		entry.used = now
		l.images[source] = entry
		l.mu.Unlock()
		return entry.image
	}
	if expires, ok := l.failures[source]; ok && now.Before(expires) {
		l.mu.Unlock()
		return nil
	}
	if call := l.inFlight[source]; call != nil {
		l.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil
		case <-call.done:
			return call.img
		}
	}
	call := &imageCall{done: make(chan struct{})}
	l.inFlight[source] = call
	l.mu.Unlock()

	call.img = l.load(ctx, source)
	l.mu.Lock()
	delete(l.inFlight, source)
	if call.img == nil {
		l.failures[source] = now.Add(l.negativeTTL)
	} else {
		l.evictIfNeeded()
		l.images[source] = imageEntry{image: call.img, used: now, expires: now.Add(l.ttl)}
	}
	close(call.done)
	l.mu.Unlock()
	return call.img
}

func (l *Loader) load(ctx context.Context, source string) image.Image {
	if path := l.localTwemoji(source); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			if img, _, decodeErr := image.Decode(bytes.NewReader(data)); decodeErr == nil {
				return img
			}
		}
	}
	if l.offline {
		return nil
	}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil
	}
	if internalnet.Validate(ctx, parsed, l.allowPrivateNetwork) != nil {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return nil
	}
	if internalnet.Validate(ctx, resp.Request.URL, l.allowPrivateNetwork) != nil {
		resp.Body.Close()
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil
	}
	limited := &io.LimitedReader{R: resp.Body, N: l.maxAssetBytes + 1}
	data, err := io.ReadAll(limited)
	if err != nil || int64(len(data)) > l.maxAssetBytes {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return img
}

func (l *Loader) localTwemoji(source string) string {
	if l.twemojiDir == "" || !strings.Contains(source, "/twemoji@") {
		return ""
	}
	parsed, err := url.Parse(source)
	if err != nil {
		return ""
	}
	name := filepath.Base(parsed.Path)
	if !strings.HasSuffix(name, ".png") || strings.Contains(name, "..") {
		return ""
	}
	for _, path := range []string{filepath.Join(l.twemojiDir, name), filepath.Join(l.twemojiDir, "72x72", name)} {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return path
		}
	}
	return ""
}

func (l *Loader) evictIfNeeded() {
	for len(l.images) >= l.maxEntries && len(l.images) > 0 {
		var oldestKey string
		var oldest time.Time
		for key, entry := range l.images {
			if oldestKey == "" || entry.used.Before(oldest) {
				oldestKey, oldest = key, entry.used
			}
		}
		delete(l.images, oldestKey)
	}
}

func ResolveTwemojiDir(override, fontCacheDir string) string {
	if override != "" {
		return override
	}
	if value := os.Getenv("MIQ_TWEMOJI_CACHE_DIR"); value != "" {
		return value
	}
	return filepath.Join(filepath.Dir(fontCacheDir), "twemoji")
}
