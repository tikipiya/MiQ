package font

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	internaldraw "github.com/tikipiya/MiQ/internal/draw"
)

const DefaultCSSEndpoint = "https://fonts.googleapis.com/css2"

var genericFamilies = map[string]bool{
	"sans-serif": true, "serif": true, "monospace": true, "system-ui": true,
}

type Options struct {
	Client        *http.Client
	Registry      *internaldraw.FontRegistry
	CacheDir      string
	Offline       bool
	MaxAssetBytes int64
	CSSEndpoint   string
}

type Manager struct {
	client        *http.Client
	registry      *internaldraw.FontRegistry
	cacheDir      string
	offline       bool
	maxAssetBytes int64
	cssEndpoint   string

	mu       sync.Mutex
	ready    map[string]bool
	inFlight map[string]*fontCall
}

type fontCall struct {
	done chan struct{}
	ok   bool
	err  error
}

type Face struct {
	Family string
	Weight int
	Italic bool
	URL    string
}

func NewManager(opts Options) (*Manager, error) {
	if opts.Client == nil || opts.Registry == nil {
		return nil, fmt.Errorf("font manager requires HTTP client and registry")
	}
	endpoint := opts.CSSEndpoint
	if endpoint == "" {
		endpoint = DefaultCSSEndpoint
	}
	return &Manager{
		client: opts.Client, registry: opts.Registry, cacheDir: opts.CacheDir,
		offline: opts.Offline, maxAssetBytes: opts.MaxAssetBytes, cssEndpoint: endpoint,
		ready: make(map[string]bool), inFlight: make(map[string]*fontCall),
	}, nil
}

func (m *Manager) EnsureStack(ctx context.Context, stack string) error {
	var lastErr error
	for _, candidate := range splitStack(stack) {
		if genericFamilies[strings.ToLower(candidate)] {
			continue
		}
		ok, err := m.EnsureFamily(ctx, candidate)
		if ok {
			return nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no font available for stack %q", stack)
	}
	return lastErr
}

func (m *Manager) EnsureFamily(ctx context.Context, family string) (bool, error) {
	name := strings.TrimSpace(family)
	if name == "" {
		return false, fmt.Errorf("font family is empty")
	}
	key := strings.ToLower(name)
	m.mu.Lock()
	if m.ready[key] || m.registry.Has(name) {
		m.ready[key] = true
		m.mu.Unlock()
		return true, nil
	}
	if call := m.inFlight[key]; call != nil {
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-call.done:
			return call.ok, call.err
		}
	}
	call := &fontCall{done: make(chan struct{})}
	m.inFlight[key] = call
	m.mu.Unlock()

	call.ok, call.err = m.ensure(ctx, name)
	m.mu.Lock()
	delete(m.inFlight, key)
	if call.ok {
		m.ready[key] = true
	}
	close(call.done)
	m.mu.Unlock()
	return call.ok, call.err
}

func (m *Manager) ensure(ctx context.Context, family string) (bool, error) {
	if m.registry.RegisterSystem(family) {
		return true, nil
	}
	if ok := m.registerCached(family); ok {
		return true, nil
	}
	if m.offline {
		return false, nil
	}
	faces, err := m.Resolve(ctx, family, []int{400})
	if err != nil {
		return false, err
	}
	for _, face := range faces {
		data, err := m.fetch(ctx, face.URL)
		if err != nil {
			return false, err
		}
		path, err := m.writeCached(face, data)
		if err != nil {
			return false, err
		}
		if err := m.registry.RegisterBytes(family, data); err != nil {
			return false, fmt.Errorf("register %s from %s: %w", family, path, err)
		}
		return true, nil
	}
	return false, fmt.Errorf("Google Fonts returned no usable face for %q", family)
}

func (m *Manager) Resolve(ctx context.Context, family string, weights []int) ([]Face, error) {
	if len(weights) == 0 {
		weights = []int{400}
	}
	weights = append([]int(nil), weights...)
	sort.Ints(weights)
	query := url.Values{}
	name := strings.Join(strings.Fields(family), " ")
	if len(weights) == 1 && weights[0] == 400 {
		query.Set("family", name)
	} else {
		parts := make([]string, len(weights))
		for i, weight := range weights {
			parts[i] = strconv.Itoa(weight)
		}
		query.Set("family", name+":wght@"+strings.Join(parts, ";"))
	}
	requestURL := m.cssEndpoint + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	// An unknown user agent makes the CSS API return a full TTF instead of
	// browser-specific unicode-range WOFF2 subsets.
	req.Header.Set("User-Agent", "makeitaquote-go/1")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("resolve Google Font %q: %w", family, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("Google Fonts answered HTTP %d for %q", resp.StatusCode, family)
	}
	body, err := readLimited(resp.Body, min(m.maxAssetBytes, 2<<20))
	if err != nil {
		return nil, err
	}
	faces := parseCSS(string(body), family)
	if len(faces) == 0 {
		return nil, fmt.Errorf("Google Fonts returned no usable font file for %q", family)
	}
	return faces, nil
}

func (m *Manager) fetch(ctx context.Context, source string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download font: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("font download answered HTTP %d", resp.StatusCode)
	}
	return readLimited(resp.Body, m.maxAssetBytes)
}

func (m *Manager) registerCached(family string) bool {
	pattern := filepath.Join(m.cacheDir, slug(family)+"-*")
	paths, _ := filepath.Glob(pattern)
	installed, _ := filepath.Glob(filepath.Join(m.cacheDir, slug(family), "font.*"))
	paths = append(paths, installed...)
	lowerInstalled, _ := filepath.Glob(filepath.Join(m.cacheDir, strings.ToLower(slug(family)), "font.*"))
	paths = append(paths, lowerInstalled...)
	sort.Strings(paths)
	for i := len(paths) - 1; i >= 0; i-- {
		info, err := os.Stat(paths[i])
		if err != nil || info.Size() <= 0 || info.Size() > m.maxAssetBytes {
			continue
		}
		data, err := os.ReadFile(paths[i])
		if err == nil && m.registry.RegisterBytes(family, data) == nil {
			return true
		}
	}
	return false
}

func (m *Manager) writeCached(face Face, data []byte) (string, error) {
	if err := os.MkdirAll(m.cacheDir, 0o755); err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	ext := filepath.Ext(strings.Split(face.URL, "?")[0])
	if ext == "" {
		ext = ".ttf"
	}
	name := fmt.Sprintf("%s-%d-%s%s", slug(face.Family), face.Weight, hex.EncodeToString(hash[:6]), ext)
	target := filepath.Join(m.cacheDir, name)
	tmp, err := os.CreateTemp(m.cacheDir, ".miq-font-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, target); err != nil {
		if info, statErr := os.Stat(target); statErr == nil && info.Size() > 0 {
			return target, nil
		}
		return "", err
	}
	return target, nil
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("asset byte limit must be positive")
	}
	limited := &io.LimitedReader{R: reader, N: limit + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("asset exceeds %d bytes", limit)
	}
	return data, nil
}

var (
	faceBlock     = regexp.MustCompile(`(?s)@font-face\s*\{(.*?)\}`)
	urlPattern    = regexp.MustCompile(`url\((?:'|")?(https://[^)'"\s]+\.(?:ttf|otf|woff2?)(?:\?[^)'"\s]*)?)(?:'|")?\)`)
	familyPattern = regexp.MustCompile(`font-family:\s*['"]([^'"]+)['"]`)
	weightPattern = regexp.MustCompile(`font-weight:\s*(\d+)`)
)

func parseCSS(css, requested string) []Face {
	var faces []Face
	seen := make(map[string]bool)
	for _, match := range faceBlock.FindAllStringSubmatch(css, -1) {
		block := match[1]
		urlMatch := urlPattern.FindStringSubmatch(block)
		if len(urlMatch) == 0 || seen[urlMatch[1]] {
			continue
		}
		seen[urlMatch[1]] = true
		family := requested
		if found := familyPattern.FindStringSubmatch(block); len(found) > 0 {
			family = found[1]
		}
		weight := 400
		if found := weightPattern.FindStringSubmatch(block); len(found) > 0 {
			weight, _ = strconv.Atoi(found[1])
		}
		faces = append(faces, Face{
			Family: family, Weight: weight, Italic: strings.Contains(block, "font-style: italic"), URL: urlMatch[1],
		})
	}
	return faces
}

func splitStack(stack string) []string {
	parts := strings.Split(stack, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), "'\"")
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func slug(value string) string {
	value = strings.TrimSpace(value)
	var out strings.Builder
	lastDash := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			out.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}
