// Package asset manages persistent fonts and Twemoji used by the renderer.
package asset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/image/font/sfnt"
	"image"
	_ "image/png"
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
	"time"
)

const TwemojiVersion = "17.0.2"

type Options struct {
	Root                string
	Client              *http.Client
	FontCSSEndpoint     string
	TwemojiListEndpoint string
	TwemojiCDN          string
	MaxBytes            int64
}
type Manager struct {
	root                           string
	client                         *http.Client
	cssEndpoint, listEndpoint, cdn string
	maxBytes                       int64
}

type File struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type Manifest struct {
	Schema      int       `json:"schema"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installedAt"`
	Files       []File    `json:"files"`
}
type Info struct {
	Root    string     `json:"root"`
	Fonts   []Manifest `json:"fonts"`
	Twemoji *Manifest  `json:"twemoji,omitempty"`
}
type InstallResult struct {
	Name                string
	Downloaded, Skipped int
	Bytes               int64
}

func New(options Options) (*Manager, error) {
	root := options.Root
	if root == "" {
		root = ResolveRoot()
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	max := options.MaxBytes
	if max == 0 {
		max = 32 << 20
	}
	if max < 1 {
		return nil, fmt.Errorf("max bytes must be positive")
	}
	css := options.FontCSSEndpoint
	if css == "" {
		css = "https://fonts.googleapis.com/css2"
	}
	list := options.TwemojiListEndpoint
	if list == "" {
		list = "https://data.jsdelivr.com/v1/package/gh/jdecked/twemoji@" + TwemojiVersion + "/flat"
	}
	cdn := options.TwemojiCDN
	if cdn == "" {
		cdn = "https://cdn.jsdelivr.net/gh/jdecked/twemoji@" + TwemojiVersion + "/assets/72x72"
	}
	return &Manager{root: abs, client: client, cssEndpoint: css, listEndpoint: list, cdn: strings.TrimSuffix(cdn, "/"), maxBytes: max}, nil
}

func ResolveRoot() string {
	if value := os.Getenv("MIQ_ASSET_DIR"); value != "" {
		return value
	}
	dir, err := os.Getwd()
	if err != nil {
		return ".makeitaquote"
	}
	start := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, ".makeitaquote")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Join(start, ".makeitaquote")
}
func (m *Manager) Root() string { return m.root }

var fontURL = regexp.MustCompile(`url\((?:"|')?([^)'" ]+)(?:"|')?\)`)
var legacyFontName = regexp.MustCompile(`^(.+)-(v\d+)-(\d+)(?:-italic)?-(.+)\.(?:ttf|otf|woff2?)$`)

func (m *Manager) InstallFonts(ctx context.Context, families []string) ([]InstallResult, error) {
	if len(families) == 0 {
		families = DefaultFonts
	}
	results := make([]InstallResult, 0, len(families))
	for _, family := range families {
		result, err := m.installFont(ctx, family)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}
func (m *Manager) installFont(ctx context.Context, family string) (InstallResult, error) {
	family = strings.TrimSpace(family)
	if family == "" {
		return InstallResult{}, fmt.Errorf("font family is empty")
	}
	if resolved, ok := ResolveFontAlias(family); ok {
		family = resolved
	}
	if reason, ok := UnavailableReason(family); ok {
		return InstallResult{}, fmt.Errorf("font %q is %s", family, reason)
	}
	endpoint := m.cssEndpoint + "?family=" + url.QueryEscape(family)
	data, err := m.get(ctx, endpoint, m.maxBytes)
	if err != nil {
		if suggestion, ok := SuggestionFor(family); ok {
			return InstallResult{}, fmt.Errorf("font CSS %q: %w; did you mean %q?", family, err, suggestion)
		}
		return InstallResult{}, fmt.Errorf("font CSS %q: %w", family, err)
	}
	match := fontURL.FindSubmatch(data)
	if match == nil {
		if suggestion, ok := SuggestionFor(family); ok {
			return InstallResult{}, fmt.Errorf("font %q was not found; did you mean %q?", family, suggestion)
		}
		return InstallResult{}, fmt.Errorf("font %q was not found", family)
	}
	dir := filepath.Join(m.root, "fonts", safeName(family))
	name := "font" + filepath.Ext(strings.Split(string(match[1]), "?")[0])
	if name == "font" {
		name = "font.ttf"
	}
	version := versionFromURL(string(match[1]))
	if existing, readErr := readManifest(dir); readErr == nil && existing.Version == version && len(existing.Files) == 1 && existing.Files[0].Name == name && fileMatches(filepath.Join(dir, name), existing.Files[0]) {
		return InstallResult{Name: family, Skipped: 1, Bytes: existing.Files[0].Bytes}, nil
	}
	fontData, err := m.get(ctx, string(match[1]), m.maxBytes)
	if err != nil {
		return InstallResult{}, fmt.Errorf("font %q: %w", family, err)
	}
	if _, err := sfnt.Parse(fontData); err != nil {
		return InstallResult{}, fmt.Errorf("font %q is invalid: %w", family, err)
	}
	file, downloaded, err := writeAsset(dir, name, fontData)
	if err != nil {
		return InstallResult{}, err
	}
	manifest := Manifest{Schema: 1, Kind: "font", Name: family, Version: version, InstalledAt: time.Now().UTC(), Files: []File{file}}
	if err := writeManifest(dir, manifest); err != nil {
		return InstallResult{}, err
	}
	result := InstallResult{Name: family, Bytes: file.Bytes}
	if downloaded {
		result.Downloaded = 1
	} else {
		result.Skipped = 1
	}
	return result, nil
}

type flatListing struct {
	Files []struct {
		Name string `json:"name"`
	} `json:"files"`
}

func (m *Manager) InstallTwemoji(ctx context.Context) (InstallResult, error) {
	data, err := m.get(ctx, m.listEndpoint, m.maxBytes)
	if err != nil {
		return InstallResult{}, err
	}
	var listing flatListing
	if err = json.Unmarshal(data, &listing); err != nil {
		return InstallResult{}, fmt.Errorf("decode Twemoji listing: %w", err)
	}
	names := make([]string, 0)
	for _, file := range listing.Files {
		if strings.HasPrefix(file.Name, "/assets/72x72/") && strings.HasSuffix(file.Name, ".png") {
			names = append(names, filepath.Base(file.Name))
		}
	}
	if len(names) == 0 {
		return InstallResult{}, fmt.Errorf("Twemoji listing contains no PNG files")
	}
	sort.Strings(names)
	dir := filepath.Join(m.root, "twemoji", "72x72")
	files := make([]File, len(names))
	downloaded := make([]bool, len(names))
	pending := make([]int, 0, len(names))
	if old, readErr := readManifest(filepath.Join(m.root, "twemoji")); readErr == nil && old.Version == TwemojiVersion {
		known := make(map[string]File, len(old.Files))
		for _, file := range old.Files {
			known[file.Name] = file
		}
		for i, name := range names {
			if file, ok := known[name]; ok {
				if path := matchingTwemojiPath(filepath.Join(m.root, "twemoji"), name, file); path != "" {
					files[i] = file
					continue
				}
			}
			{
				pending = append(pending, i)
			}
		}
	} else {
		for i := range names {
			pending = append(pending, i)
		}
	}
	jobs := make(chan int)
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	workers := 16
	if len(pending) < workers {
		workers = len(pending)
	}
	if workers == 0 {
		workers = 1
	}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				payload, err := m.get(ctx, m.cdn+"/"+names[index], m.maxBytes)
				if err != nil {
					select {
					case errs <- err:
					default:
					}
					return
				}
				if _, _, decodeErr := image.DecodeConfig(bytes.NewReader(payload)); decodeErr != nil {
					select {
					case errs <- fmt.Errorf("invalid Twemoji %s: %w", names[index], decodeErr):
					default:
					}
					return
				}
				files[index], downloaded[index], err = writeAsset(dir, names[index], payload)
				if err != nil {
					select {
					case errs <- err:
					default:
					}
					return
				}
			}
		}()
	}
	for _, i := range pending {
		select {
		case jobs <- i:
		case err := <-errs:
			close(jobs)
			wg.Wait()
			return InstallResult{}, err
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return InstallResult{}, ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	select {
	case err := <-errs:
		return InstallResult{}, err
	default:
	}
	manifest := Manifest{Schema: 1, Kind: "twemoji", Name: "Twemoji", Version: TwemojiVersion, InstalledAt: time.Now().UTC(), Files: files}
	if err := writeManifest(filepath.Join(m.root, "twemoji"), manifest); err != nil {
		return InstallResult{}, err
	}
	result := InstallResult{Name: "Twemoji"}
	for i, file := range files {
		result.Bytes += file.Bytes
		if downloaded[i] {
			result.Downloaded++
		} else {
			result.Skipped++
		}
	}
	return result, nil
}

func (m *Manager) Info() (Info, error) {
	info := Info{Root: m.root}
	entries, _ := os.ReadDir(filepath.Join(m.root, "fonts"))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest, err := readManifest(filepath.Join(m.root, "fonts", entry.Name()))
		if err == nil {
			info.Fonts = append(info.Fonts, manifest)
		}
	}
	legacy := map[string]*Manifest{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := legacyFontName.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		family := familyFromSlug(match[1])
		manifest := legacy[family]
		if manifest == nil {
			manifest = &Manifest{Schema: 0, Kind: "font", Name: family, Version: match[2]}
			legacy[family] = manifest
		}
		if versionNumber(match[2]) > versionNumber(manifest.Version) {
			manifest.Version = match[2]
		}
		file, fileErr := describeFile(filepath.Join(m.root, "fonts", entry.Name()), entry.Name())
		if fileErr == nil {
			manifest.Files = append(manifest.Files, file)
		}
	}
	for _, manifest := range legacy {
		info.Fonts = append(info.Fonts, *manifest)
	}
	sort.Slice(info.Fonts, func(i, j int) bool { return info.Fonts[i].Name < info.Fonts[j].Name })
	if manifest, err := readManifest(filepath.Join(m.root, "twemoji")); err == nil {
		info.Twemoji = &manifest
	}
	return info, nil
}
func (m *Manager) UninstallFonts(families []string) (int, error) {
	entries, _ := os.ReadDir(filepath.Join(m.root, "fonts"))
	targets := map[string]bool{}
	for _, f := range families {
		if resolved, ok := ResolveFontAlias(f); ok {
			f = resolved
		}
		targets[safeName(f)] = true
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			if len(targets) > 0 && !targets[entry.Name()] {
				continue
			}
			if err := os.RemoveAll(filepath.Join(m.root, "fonts", entry.Name())); err != nil {
				return removed, err
			}
			removed++
			continue
		}
		match := legacyFontName.FindStringSubmatch(entry.Name())
		if match == nil || len(targets) > 0 && !targets[match[1]] {
			continue
		}
		if err := os.Remove(filepath.Join(m.root, "fonts", entry.Name())); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
func (m *Manager) UninstallTwemoji() error { return os.RemoveAll(filepath.Join(m.root, "twemoji")) }
func (m *Manager) Prune() (int, int64, error) {
	info, err := m.Info()
	if err != nil {
		return 0, 0, err
	}
	keep := map[string]bool{}
	for _, manifest := range info.Fonts {
		dir := filepath.Join(m.root, "fonts", safeName(manifest.Name))
		for _, f := range manifest.Files {
			keep[filepath.Join(dir, f.Name)] = true
		}
	}
	removed := 0
	var bytes int64
	_ = filepath.Walk(filepath.Join(m.root, "fonts"), func(path string, entry os.FileInfo, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Base(path) == "manifest.json" || keep[path] {
			return nil
		}
		bytes += entry.Size()
		if err := os.Remove(path); err == nil {
			removed++
		}
		return nil
	})
	return removed, bytes, nil
}

func (m *Manager) get(ctx context.Context, address string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "makeitaquote-go/1")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("asset exceeds %d bytes", limit)
	}
	return data, nil
}
func writeAsset(dir, name string, data []byte) (File, bool, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return File{}, false, err
	}
	hash := sha256.Sum256(data)
	encoded := hex.EncodeToString(hash[:])
	path := filepath.Join(dir, name)
	if existing, err := os.ReadFile(path); err == nil {
		old := sha256.Sum256(existing)
		if old == hash {
			return File{Name: name, SHA256: encoded, Bytes: int64(len(data))}, false, nil
		}
	}
	tmp, err := os.CreateTemp(dir, ".miq-*")
	if err != nil {
		return File{}, false, err
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
		return File{}, false, err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return File{}, false, err
	}
	return File{Name: name, SHA256: encoded, Bytes: int64(len(data))}, true, nil
}
func writeManifest(dir string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	_, _, err = writeAsset(dir, "manifest.json", append(data, '\n'))
	return err
}
func fileMatches(path string, file File) bool {
	data, err := os.ReadFile(path)
	if err != nil || int64(len(data)) != file.Bytes {
		return false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == file.SHA256
}
func readManifest(dir string) (Manifest, error) {
	var result Manifest
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	if result.Schema == 0 && result.Version != "" && result.Kind == "" && filepath.Base(dir) == "twemoji" {
		result.Kind, result.Name = "twemoji", "Twemoji"
		for _, candidateDir := range []string{dir, filepath.Join(dir, "72x72")} {
			entries, _ := os.ReadDir(candidateDir)
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".png") {
					continue
				}
				file, fileErr := describeFile(filepath.Join(candidateDir, entry.Name()), entry.Name())
				if fileErr == nil {
					result.Files = append(result.Files, file)
				}
			}
		}
	}
	return result, nil
}

func describeFile(path, name string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	sum := sha256.Sum256(data)
	return File{Name: name, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data))}, nil
}

func matchingTwemojiPath(root, name string, file File) string {
	for _, path := range []string{filepath.Join(root, "72x72", name), filepath.Join(root, name)} {
		if fileMatches(path, file) {
			return path
		}
	}
	return ""
}

func familyFromSlug(slug string) string {
	for _, family := range FontCatalogue {
		if safeName(family) == slug {
			return family
		}
	}
	return strings.ReplaceAll(slug, "-", " ")
}

func versionNumber(version string) int {
	value, _ := strconv.Atoi(strings.TrimPrefix(version, "v"))
	return value
}
func safeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out.WriteRune(r)
		} else if out.Len() > 0 && !strings.HasSuffix(out.String(), "-") {
			out.WriteByte('-')
		}
	}
	result := strings.Trim(out.String(), "-")
	if result == "" {
		sum := sha256.Sum256([]byte(value))
		result = "font-" + hex.EncodeToString(sum[:6])
	}
	return result
}
func versionFromURL(value string) string {
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "v") && len(part) > 1 {
			return part
		}
	}
	return "unknown"
}
