package font

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	internaldraw "github.com/tikipiya/MiQ/internal/draw"
	"golang.org/x/image/font/gofont/goregular"
)

func TestManagerUsesInstalledNestedFontOffline(t *testing.T) {
	dir := t.TempDir()
	installed := filepath.Join(dir, "m-plus-rounded-1c")
	if err := os.MkdirAll(installed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed, "font.ttf"), goregular.TTF, 0o644); err != nil {
		t.Fatal(err)
	}
	registry, err := internaldraw.NewFontRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(Options{Client: &http.Client{}, Registry: registry, CacheDir: dir, Offline: true, MaxAssetBytes: 4 << 20})
	if err != nil {
		t.Fatal(err)
	}
	ok, err := manager.EnsureFamily(context.Background(), "M PLUS Rounded 1c")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestParseCSS(t *testing.T) {
	css := `@font-face {
  font-family: 'Noto Sans JP';
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.example/noto.ttf) format('truetype');
}`
	faces := parseCSS(css, "requested")
	if len(faces) != 1 {
		t.Fatalf("faces = %#v", faces)
	}
	if faces[0].Family != "Noto Sans JP" || faces[0].Weight != 400 || faces[0].URL != "https://fonts.example/noto.ttf" {
		t.Fatalf("unexpected face: %#v", faces[0])
	}
}

func TestManagerDownloadsCoalescesAndCaches(t *testing.T) {
	var cssRequests atomic.Int32
	var fontRequests atomic.Int32
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/css2":
			cssRequests.Add(1)
			if got := r.URL.Query().Get("family"); got != "Test Family" {
				t.Errorf("family query = %q", got)
			}
			fmt.Fprintf(w, "@font-face { font-family: 'Test Family'; font-style: normal; font-weight: 400; src: url(%s/font.ttf); }", server.URL)
		case "/font.ttf":
			fontRequests.Add(1)
			_, _ = w.Write(goregular.TTF)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	registry, err := internaldraw.NewFontRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	manager, err := NewManager(Options{
		Client: server.Client(), Registry: registry, CacheDir: cacheDir,
		MaxAssetBytes: 2 << 20, CSSEndpoint: server.URL + "/css2",
	})
	if err != nil {
		t.Fatal(err)
	}

	const workers = 4
	errs := make(chan error, workers)
	for range workers {
		go func() {
			ok, ensureErr := manager.EnsureFamily(context.Background(), "Test Family")
			if ensureErr == nil && !ok {
				ensureErr = fmt.Errorf("font was not made ready")
			}
			errs <- ensureErr
		}()
	}
	for range workers {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if cssRequests.Load() != 1 || fontRequests.Load() != 1 {
		t.Fatalf("requests: css=%d font=%d", cssRequests.Load(), fontRequests.Load())
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil || len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".ttf") {
		t.Fatalf("cache entries = %#v, err=%v", entries, err)
	}

	offlineRegistry, err := internaldraw.NewFontRegistry(nil)
	if err != nil {
		t.Fatal(err)
	}
	offline, err := NewManager(Options{
		Client: server.Client(), Registry: offlineRegistry, CacheDir: cacheDir,
		Offline: true, MaxAssetBytes: 2 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := offline.EnsureFamily(context.Background(), "Test Family"); err != nil || !ok {
		t.Fatalf("offline cache load: ok=%v err=%v", ok, err)
	}
	if cssRequests.Load() != 1 || fontRequests.Load() != 1 {
		t.Fatal("offline manager made a network request")
	}
}

func TestResolveCacheDirUsesEnvironment(t *testing.T) {
	t.Setenv("MIQ_FONT_CACHE_DIR", t.TempDir())
	if got := ResolveCacheDir(""); got != os.Getenv("MIQ_FONT_CACHE_DIR") {
		t.Fatalf("ResolveCacheDir = %q", got)
	}
}

func TestResolveCacheDirProjectLocalCompatibility(t *testing.T) {
	t.Setenv("MIQ_FONT_CACHE_DIR", "")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/cache\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	if got, want := ResolveCacheDir(""), filepath.Join(root, ".makeitaquote", "fonts"); got != want {
		t.Fatalf("ResolveCacheDir=%q want %q", got, want)
	}
	override := filepath.Join(t.TempDir(), "explicit")
	if got := ResolveCacheDir(override); got != override {
		t.Fatalf("override=%q", got)
	}
}
