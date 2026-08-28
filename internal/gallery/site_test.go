package gallery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSiteProducesStaticHTML(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "visual")
	groupDir := filepath.Join(output, "01-test")
	if err := os.MkdirAll(groupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	width, height, bytes := 1200, 630, 1024
	note := "A concise note"
	manifest := GroupManifest{Name: "01-test", Title: "Test", Cases: []Result{{Name: "<safe>", File: "01-test/safe.png", Format: "png", Width: &width, Height: &height, Bytes: &bytes, Note: &note, OK: true}}}
	if err := writeJSON(filepath.Join(groupDir, "manifest.json"), manifest); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "index.html")
	index := IndexManifest{Version: "1.2.3", Groups: []GroupIndex{{Name: "01-test", Title: "Test"}}}
	if err := writeSite(output, target, index); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, expected := range []string{"v1.2.3", "1 examples", "visual/01-test/safe.png", "&lt;safe&gt;", note} {
		if !strings.Contains(html, expected) {
			t.Errorf("site does not contain %q", expected)
		}
	}
	if strings.Contains(html, "<script") {
		t.Fatal("site unexpectedly contains JavaScript")
	}
}
