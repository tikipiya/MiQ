package asset

import (
	"bytes"
	"context"
	"fmt"
	"golang.org/x/image/font/gofont/goregular"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestInstallAndInfo(t *testing.T) {
	var encoded bytes.Buffer
	pixel := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	pixel.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	if err := png.Encode(&encoded, pixel); err != nil {
		t.Fatal(err)
	}
	pngData := encoded.Bytes()
	var fontDownloads, emojiDownloads atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/css":
			fmt.Fprintf(w, "@font-face{src:url(%s/font.ttf)}", server.URL)
		case "/font.ttf":
			fontDownloads.Add(1)
			w.Write(goregular.TTF)
		case "/list":
			w.Write([]byte(`{"files":[{"name":"/assets/72x72/1f600.png"}]}`))
		case "/emoji/1f600.png":
			emojiDownloads.Add(1)
			w.Write(pngData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	manager, err := New(Options{Root: t.TempDir(), Client: server.Client(), FontCSSEndpoint: server.URL + "/css", TwemojiListEndpoint: server.URL + "/list", TwemojiCDN: server.URL + "/emoji"})
	if err != nil {
		t.Fatal(err)
	}
	fonts, err := manager.InstallFonts(context.Background(), []string{"Test Font"})
	if err != nil {
		t.Fatal(err)
	}
	if fonts[0].Downloaded != 1 {
		t.Fatalf("unexpected result: %#v", fonts)
	}
	emoji, err := manager.InstallTwemoji(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if emoji.Downloaded != 1 {
		t.Fatalf("unexpected result: %#v", emoji)
	}
	info, err := manager.Info()
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Fonts) != 1 || info.Twemoji == nil {
		t.Fatalf("unexpected info: %#v", info)
	}
	again, err := manager.InstallTwemoji(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again.Skipped != 1 {
		t.Fatalf("install did not resume: %#v", again)
	}
	againFonts, err := manager.InstallFonts(context.Background(), []string{"Test Font"})
	if err != nil {
		t.Fatal(err)
	}
	if againFonts[0].Skipped != 1 || fontDownloads.Load() != 1 || emojiDownloads.Load() != 1 {
		t.Fatalf("resume fetched assets again: fonts=%d emoji=%d result=%#v", fontDownloads.Load(), emojiDownloads.Load(), againFonts)
	}
}

func TestInstallRejectsCorruptAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/list":
			w.Write([]byte(`{"files":[{"name":"/assets/72x72/bad.png"}]}`))
		case "/bad.png":
			w.Write([]byte("not png"))
		case "/css":
			fmt.Fprintf(w, "@font-face{src:url(%s/bad.ttf)}", serverURL(r))
		case "/bad.ttf":
			w.Write([]byte("not font"))
		}
	}))
	defer server.Close()
	manager, err := New(Options{Root: t.TempDir(), Client: server.Client(), TwemojiListEndpoint: server.URL + "/list", TwemojiCDN: server.URL, FontCSSEndpoint: server.URL + "/css"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.InstallTwemoji(context.Background()); err == nil {
		t.Fatal("accepted corrupt PNG")
	}
	if _, err := manager.InstallFonts(context.Background(), []string{"Broken"}); err == nil {
		t.Fatal("accepted corrupt font")
	}
}

func serverURL(r *http.Request) string { return "http://" + r.Host }

func TestPruneAndUninstall(t *testing.T) {
	root := t.TempDir()
	manager, _ := New(Options{Root: root})
	dir := filepath.Join(root, "fonts", "test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(dir, Manifest{Schema: 1, Kind: "font", Name: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old.ttf"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	count, _, err := manager.Prune()
	if err != nil || count != 1 {
		t.Fatalf("prune: %d %v", count, err)
	}
	removed, err := manager.UninstallFonts(nil)
	if err != nil || removed != 1 {
		t.Fatalf("uninstall: %d %v", removed, err)
	}
}

func TestLegacyCacheCompatibility(t *testing.T) {
	root := t.TempDir()
	fonts := filepath.Join(root, "fonts")
	twemoji := filepath.Join(root, "twemoji")
	if err := os.MkdirAll(fonts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(twemoji, 0o755); err != nil {
		t.Fatal(err)
	}
	fontName := "noto-sans-jp-v30-400-test.ttf"
	if err := os.WriteFile(filepath.Join(fonts, fontName), goregular.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, solidPixel(color.NRGBA{R: 255, A: 255})); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(twemoji, "1f600.png"), encoded.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyManifest := `{"version":"` + TwemojiVersion + `","count":1,"installedAt":"2025-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(twemoji, "manifest.json"), []byte(legacyManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := New(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	info, err := manager.Info()
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Fonts) != 1 || info.Fonts[0].Name != "Noto Sans JP" || info.Fonts[0].Version != "v30" {
		t.Fatalf("legacy fonts=%#v", info.Fonts)
	}
	if info.Twemoji == nil || info.Twemoji.Version != TwemojiVersion || len(info.Twemoji.Files) != 1 {
		t.Fatalf("legacy twemoji=%#v", info.Twemoji)
	}
	removed, err := manager.UninstallFonts([]string{"Noto Sans JP"})
	if err != nil || removed != 1 {
		t.Fatalf("remove legacy font=%d,%v", removed, err)
	}
}

func TestLegacyTwemojiInstallResumesWithoutAssetDownload(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "twemoji")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, solidPixel(color.NRGBA{G: 255, A: 255})); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1f600.png"), encoded.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"version":"`+TwemojiVersion+`","count":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var downloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/list" {
			_, _ = w.Write([]byte(`{"files":[{"name":"/assets/72x72/1f600.png"}]}`))
			return
		}
		downloads.Add(1)
		_, _ = w.Write(encoded.Bytes())
	}))
	defer server.Close()
	manager, _ := New(Options{Root: root, Client: server.Client(), TwemojiListEndpoint: server.URL + "/list", TwemojiCDN: server.URL})
	result, err := manager.InstallTwemoji(context.Background())
	if err != nil || result.Skipped != 1 || downloads.Load() != 0 {
		t.Fatalf("legacy resume=%#v downloads=%d err=%v", result, downloads.Load(), err)
	}
}

func TestResolveRootFindsNearestGoModule(t *testing.T) {
	t.Setenv("MIQ_ASSET_DIR", "")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/assets\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	if got, want := ResolveRoot(), filepath.Join(root, ".makeitaquote"); got != want {
		t.Fatalf("ResolveRoot=%q want %q", got, want)
	}
}

func solidPixel(value color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, value)
	return img
}
