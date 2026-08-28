// Command miq-gallery renders and optionally compares the Go visual gallery.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tikipiya/MiQ/internal/gallery"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "miq-gallery:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("miq-gallery", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root")
	out := flags.String("out", "", "output directory (default: <root>/docs/visual-go)")
	reference := flags.String("reference", "", "reference gallery (default: <root>/docs/visual)")
	site := flags.String("site", "", "write a JavaScript-free static gallery page")
	compare := flags.Bool("compare", false, "compare every generated image with the reference gallery")
	offline := flags.Bool("offline", false, "skip cases requiring network access")
	only := flags.String("only", "", "comma-separated group or case filters")
	threshold := flags.Float64("threshold", .08, "maximum perceptual block difference")
	version := flags.String("version", "", "gallery version (default: Go module version)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	start := time.Now()
	options := gallery.Options{Root: absRoot, Output: resolve(absRoot, *out), Reference: resolve(absRoot, *reference), Site: resolve(absRoot, *site), Version: *version, Compare: *compare, Offline: *offline, Only: split(*only), Threshold: *threshold, Stdout: os.Stdout}
	summary, err := gallery.Run(ctx, options)
	fmt.Printf("miq-gallery: %d/%d generated, %d compared, %d failed (%s)\n", summary.Generated, summary.Selected, summary.Compared, summary.Failed, gallery.Elapsed(start))
	return err
}

func resolve(root, value string) string {
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(root, value)
}

func split(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}
