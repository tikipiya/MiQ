package font

import (
	"os"
	"path/filepath"
)

func ResolveCacheDir(override string) string {
	if override != "" {
		return override
	}
	if value := os.Getenv("MIQ_FONT_CACHE_DIR"); value != "" {
		return value
	}
	dir, err := os.Getwd()
	if err != nil {
		return filepath.Join(".makeitaquote", "fonts")
	}
	start := dir
	for {
		if info, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil && !info.IsDir() {
			return filepath.Join(dir, ".makeitaquote", "fonts")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Join(start, ".makeitaquote", "fonts")
		}
		dir = parent
	}
}
