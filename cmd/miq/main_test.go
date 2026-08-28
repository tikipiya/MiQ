package main

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	t.Setenv("MIQ_ASSET_DIR", filepath.Join(t.TempDir(), "assets"))
	output := filepath.Join(t.TempDir(), "quote.png")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"generate", "--text", "Hello", "--offline", "--output", output}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	info, err := os.Stat(output)
	if err != nil || info.Size() == 0 {
		t.Fatalf("output: %v", err)
	}
}
func TestListJSONAlias(t *testing.T) {
	t.Setenv("MIQ_ASSET_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"list", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	if !strings.Contains(stdout.String(), `"root"`) {
		t.Fatalf("output=%s", stdout.String())
	}
}
func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"wat"}, &stdout, &stderr); code != 1 {
		t.Fatalf("code=%d", code)
	}
}

func TestGenerateValidationAndFormats(t *testing.T) {
	t.Setenv("MIQ_ASSET_DIR", t.TempDir())
	for _, args := range [][]string{{"generate"}, {"generate", "--text", "hi", "--format", "bmp"}} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, &stdout, &stderr); code == 0 {
			t.Fatalf("args=%v unexpectedly succeeded", args)
		}
	}
	output := filepath.Join(t.TempDir(), "quote.jpg")
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"render", "--text", "Hi", "--format", "jpeg", "--quality", "75", "--theme", "light", "--width", "320", "--height", "180", "--offline", "--output", output}, &stdout, &stderr); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		t.Fatal("generate did not write JPEG")
	}
}

func TestHelpAndRemovalAliases(t *testing.T) {
	t.Setenv("MIQ_ASSET_DIR", t.TempDir())
	for _, command := range []string{"remove", "rm", "r", "un", "unlink"} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), []string{command, "twemoji"}, &stdout, &stderr); code != 0 {
			t.Fatalf("%s code=%d stderr=%s", command, code, stderr.String())
		}
	}
	for _, args := range [][]string{{"generate", "--help"}, {"install", "--help"}, {"help"}} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, &stdout, &stderr); code != 0 || stdout.Len() == 0 {
			t.Fatalf("args=%v code=%d output=%q", args, code, stdout.String())
		}
	}
}

func TestFontSearchCompatibility(t *testing.T) {
	for query, want := range map[string]string{
		"pop": "Hachi Maru Pop", "DOT": "DotGothic16", "dacing script": "Did you mean: Dancing Script",
	} {
		var output bytes.Buffer
		if code := search(strings.Fields(query), &output); code != 0 || strings.TrimSpace(output.String()) != want {
			t.Fatalf("search(%q)=%d,%q want %q", query, code, output.String(), want)
		}
	}
	var all bytes.Buffer
	if code := search(nil, &all); code != 0 || len(strings.Split(strings.TrimSpace(all.String()), "\n")) != 18 {
		t.Fatalf("catalogue output=%q", all.String())
	}
}

func TestGenerateLegacyCompatibleFlags(t *testing.T) {
	t.Setenv("MIQ_ASSET_DIR", t.TempDir())
	target := filepath.Join(t.TempDir(), "scaled.jpg")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"generate", "--text", "compatible", "--color", "--scale", "0.5",
		"--format", "jpg", "--out", target, "--offline",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	file, err := os.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	config, format, err := image.DecodeConfig(file)
	if err != nil || format != "jpeg" || config.Width != 600 || config.Height != 315 {
		t.Fatalf("generated=%s %dx%d err=%v", format, config.Width, config.Height, err)
	}
}
