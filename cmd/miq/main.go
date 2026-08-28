package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	miq "github.com/tikipiya/MiQ"
	"github.com/tikipiya/MiQ/asset"
	"github.com/tikipiya/MiQ/theme"
)

var version = "dev"

func main() { os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)) }

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stdout)
		return 0
	}
	command := args[0]
	aliases := map[string]string{"add": "install", "i": "install", "remove": "uninstall", "rm": "uninstall", "r": "uninstall", "un": "uninstall", "unlink": "uninstall", "list": "ls", "find": "search", "s": "search", "doctor": "env", "render": "generate"}
	if canonical := aliases[command]; canonical != "" {
		command = canonical
	}
	if command == "version" || command == "--version" || command == "-v" {
		fmt.Fprintln(stdout, version)
		return 0
	}
	if command == "help" || command == "--help" || command == "-h" {
		usage(stdout)
		return 0
	}
	if len(args) > 1 && (args[1] == "--help" || args[1] == "-h") {
		commandHelp(stdout, command)
		return 0
	}
	manager, err := asset.New(asset.Options{})
	if err != nil {
		return fail(stderr, err)
	}
	switch command {
	case "install":
		return install(ctx, manager, args[1:], stdout, stderr)
	case "uninstall":
		return uninstall(manager, args[1:], stdout, stderr)
	case "ls":
		return list(manager, args[1:], stdout, stderr)
	case "search":
		return search(args[1:], stdout)
	case "outdated":
		return outdated(manager, args[1:], stdout, stderr)
	case "update":
		return update(ctx, manager, stdout, stderr)
	case "prune":
		count, bytes, err := manager.Prune()
		if err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintf(stdout, "Pruned %d files (%d bytes)\n", count, bytes)
		return 0
	case "env":
		return environment(manager, args[1:], stdout, stderr)
	case "generate":
		return generate(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", command)
		return 1
	}
}

func install(ctx context.Context, m *asset.Manager, args []string, stdout, stderr io.Writer) int {
	twemoji, families := targets(args, true)
	if twemoji {
		result, err := m.InstallTwemoji(ctx)
		if err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintf(stdout, "Twemoji: %d downloaded, %d present\n", result.Downloaded, result.Skipped)
	}
	if families != nil {
		results, err := m.InstallFonts(ctx, families)
		if err != nil {
			return fail(stderr, err)
		}
		for _, result := range results {
			fmt.Fprintf(stdout, "%s: %d downloaded, %d present\n", result.Name, result.Downloaded, result.Skipped)
		}
	}
	return 0
}
func uninstall(m *asset.Manager, args []string, stdout, stderr io.Writer) int {
	twemoji, families := targets(args, false)
	if twemoji {
		if err := m.UninstallTwemoji(); err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintln(stdout, "Removed Twemoji")
	}
	if families != nil {
		count, err := m.UninstallFonts(families)
		if err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintf(stdout, "Removed %d font families\n", count)
	}
	return 0
}
func targets(args []string, defaults bool) (bool, []string) {
	if len(args) == 0 {
		if defaults {
			return true, append([]string(nil), asset.DefaultFonts...)
		}
		return true, []string{}
	}
	twemoji := false
	fonts := []string(nil)
	for i, arg := range args {
		switch strings.ToLower(arg) {
		case "twemoji", "emoji":
			twemoji = true
		case "all":
			twemoji = true
			fonts = append([]string(nil), asset.FontCatalogue...)
		case "fonts":
			if i == len(args)-1 {
				fonts = []string{}
			}
		default:
			fonts = append(fonts, arg)
		}
	}
	if fonts != nil && len(fonts) == 0 && defaults {
		fonts = append(fonts, asset.DefaultFonts...)
	}
	return twemoji, fonts
}

func list(m *asset.Manager, args []string, stdout, stderr io.Writer) int {
	jsonOutput := len(args) > 0 && args[0] == "--json"
	info, err := m.Info()
	if err != nil {
		return fail(stderr, err)
	}
	if jsonOutput {
		return writeJSON(stdout, info, stderr)
	}
	fmt.Fprintln(stdout, "Asset root:", info.Root)
	if info.Twemoji != nil {
		fmt.Fprintf(stdout, "Twemoji %s (%d files)\n", info.Twemoji.Version, len(info.Twemoji.Files))
	} else {
		fmt.Fprintln(stdout, "Twemoji: not installed")
	}
	if len(info.Fonts) == 0 {
		fmt.Fprintln(stdout, "Fonts: not installed")
	} else {
		for _, font := range info.Fonts {
			fmt.Fprintf(stdout, "Font: %s (%s)\n", font.Name, font.Version)
		}
	}
	return 0
}
func search(args []string, stdout io.Writer) int {
	query := strings.ToLower(strings.Join(args, " "))
	if resolved, ok := asset.ResolveFontAlias(query); ok {
		fmt.Fprintln(stdout, resolved)
		return 0
	}
	for _, name := range asset.FontCatalogue {
		if query == "" || strings.Contains(strings.ToLower(name), query) {
			fmt.Fprintln(stdout, name)
		}
	}
	if suggestion, ok := asset.SuggestionFor(query); ok {
		fmt.Fprintln(stdout, "Did you mean:", suggestion)
	}
	return 0
}
func outdated(m *asset.Manager, args []string, stdout, stderr io.Writer) int {
	info, err := m.Info()
	if err != nil {
		return fail(stderr, err)
	}
	result := map[string]any{"twemoji": map[string]any{"installed": nil, "latest": asset.TwemojiVersion}, "fonts": info.Fonts}
	if info.Twemoji != nil {
		result["twemoji"] = map[string]any{"installed": info.Twemoji.Version, "latest": asset.TwemojiVersion}
	}
	if len(args) > 0 && args[0] == "--json" {
		return writeJSON(stdout, result, stderr)
	}
	if info.Twemoji != nil && info.Twemoji.Version != asset.TwemojiVersion {
		fmt.Fprintf(stdout, "Twemoji %s -> %s\n", info.Twemoji.Version, asset.TwemojiVersion)
	} else {
		fmt.Fprintln(stdout, "Installed assets are current")
	}
	return 0
}
func update(ctx context.Context, m *asset.Manager, stdout, stderr io.Writer) int {
	info, err := m.Info()
	if err != nil {
		return fail(stderr, err)
	}
	if info.Twemoji != nil && info.Twemoji.Version != asset.TwemojiVersion {
		if err = m.UninstallTwemoji(); err != nil {
			return fail(stderr, err)
		}
		if _, err = m.InstallTwemoji(ctx); err != nil {
			return fail(stderr, err)
		}
	}
	families := make([]string, len(info.Fonts))
	for i, font := range info.Fonts {
		families[i] = font.Name
	}
	if len(families) > 0 {
		if _, err = m.InstallFonts(ctx, families); err != nil {
			return fail(stderr, err)
		}
	}
	if _, _, err = m.Prune(); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, "Assets updated")
	return 0
}
func environment(m *asset.Manager, args []string, stdout, stderr io.Writer) int {
	probe := filepath.Join(m.Root(), ".write-test")
	if err := os.MkdirAll(m.Root(), 0755); err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(probe, []byte("ok"), 0600); err != nil {
		return fail(stderr, err)
	}
	_ = os.Remove(probe)
	result := map[string]any{"root": m.Root(), "writable": true, "go": "1.24+"}
	if len(args) > 0 && args[0] == "--json" {
		return writeJSON(stdout, result, stderr)
	}
	fmt.Fprintln(stdout, "Asset root:", m.Root())
	fmt.Fprintln(stdout, "Writable: yes")
	return 0
}

func generate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	text := flags.String("text", "", "quote text")
	username := flags.String("username", "", "username")
	display := flags.String("display-name", "", "display name")
	watermark := flags.String("watermark", "", "watermark")
	avatar := flags.String("avatar", "", "avatar URL or file")
	output := flags.String("output", "", "output file")
	out := flags.String("out", "", "output file")
	format := flags.String("format", "png", "png, jpeg, webp or avif")
	themeName := flags.String("theme", "dark", "theme name")
	colorAvatar := flags.Bool("color", false, "keep avatar in color")
	scale := flags.Float64("scale", 1, "scale factor")
	quality := flags.Int("quality", 92, "quality")
	width := flags.Int("width", 0, "canvas width")
	height := flags.Int("height", 0, "canvas height")
	offline := flags.Bool("offline", false, "disable network")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if strings.TrimSpace(*text) == "" {
		fmt.Fprintln(stderr, "--text is required")
		return 2
	}
	quote := miq.Quote{Text: *text, Username: *username, DisplayName: *display, Watermark: *watermark}
	if *avatar != "" {
		if parsed, err := url.Parse(*avatar); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			quote.Avatar = miq.ImageURL(parsed)
		} else {
			quote.Avatar = miq.ImageFile(*avatar)
		}
	}
	selectedTheme := theme.Name(*themeName)
	if *colorAvatar {
		selectedTheme = theme.Color
	}
	input := theme.Input{Extends: selectedTheme}
	if *width > 0 {
		input.Width = width
	}
	if *height > 0 {
		input.Height = height
	}
	engine, err := miq.NewEngine(miq.EngineOptions{Offline: *offline})
	if err != nil {
		return fail(stderr, err)
	}
	target := *out
	if target == "" {
		target = *output
	}
	if target == "" {
		canonical, formatErr := miq.CanonicalFormat(miq.Format(*format))
		if formatErr != nil {
			return fail(stderr, formatErr)
		}
		extension := string(canonical)
		if canonical == miq.JPEG {
			extension = "jpg"
		}
		target = "quote." + extension
	}
	file, err := os.Create(target)
	if err != nil {
		return fail(stderr, err)
	}
	err = engine.WriteQuote(ctx, file, quote, miq.RenderOptions{Theme: input, Scale: *scale}, miq.EncodeOptions{Format: miq.Format(strings.ToLower(*format)), Quality: *quality})
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(target)
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, target)
	return 0
}

func fail(stderr io.Writer, err error) int { fmt.Fprintln(stderr, "error:", err); return 1 }
func writeJSON(stdout io.Writer, value any, stderr io.Writer) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fail(stderr, err)
	}
	return 0
}
func usage(w io.Writer) {
	fmt.Fprintln(w, "miq <install|uninstall|ls|search|outdated|update|prune|env|generate>\n\nUse `miq <command> --help` for command options.")
}

func commandHelp(w io.Writer, command string) {
	switch command {
	case "install":
		fmt.Fprintln(w, "miq install [twemoji|fonts|all|FONT ...]")
	case "uninstall":
		fmt.Fprintln(w, "miq uninstall [twemoji|fonts|all|FONT ...]")
	case "ls", "outdated", "env":
		fmt.Fprintf(w, "miq %s [--json]\n", command)
	case "search":
		fmt.Fprintln(w, "miq search [QUERY]")
	case "update":
		fmt.Fprintln(w, "miq update")
	case "prune":
		fmt.Fprintln(w, "miq prune")
	case "generate":
		fmt.Fprintln(w, "miq generate --text TEXT [--avatar URL|FILE] [--out FILE] [--format png|jpeg|webp|avif] [--theme NAME] [--scale FACTOR]")
	default:
		usage(w)
	}
}
