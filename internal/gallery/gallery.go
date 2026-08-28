// Package gallery generates and compares the repository's visual gallery.
package gallery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	_ "github.com/gen2brain/avif"
	_ "github.com/gen2brain/webp"
	miq "github.com/tikipiya/MiQ"
	"github.com/tikipiya/MiQ/internal/testfixture"
)

const DefaultVersion = "go-dev"

type Options struct {
	Root, Output, Reference, Site string
	Version                       string
	Offline, Compare              bool
	Only                          []string
	Stdout                        io.Writer
	Threshold                     float64
}

type Case struct {
	Group, Name, Note string
	Network           bool
	Format            miq.Format
	Quality           int
	ExpectWidth       int
	ExpectHeight      int
	Render            func(context.Context, *Runtime) (image.Image, error)
}

type Runtime struct {
	Root   string
	Engine *miq.Engine
	Assets Assets
}

type Assets struct {
	PNG, JPG, RemoteURL string
	PNGBytes            []byte
	DiscordEmoji        []string
	Misskey             struct {
		Instance string   `json:"instance"`
		Emoji    []string `json:"emoji"`
	}
}

type Result struct {
	Name       string   `json:"name"`
	File       string   `json:"file"`
	Note       *string  `json:"note"`
	Format     string   `json:"format"`
	Width      *int     `json:"width"`
	Height     *int     `json:"height"`
	Bytes      *int     `json:"bytes"`
	OK         bool     `json:"ok"`
	Error      *string  `json:"error"`
	Difference *float64 `json:"difference,omitempty"`
}

type GroupManifest struct {
	Name  string   `json:"name"`
	Title string   `json:"title"`
	Cases []Result `json:"cases"`
}

type IndexManifest struct {
	Version string       `json:"version"`
	Groups  []GroupIndex `json:"groups"`
}

type GroupIndex struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

type Summary struct {
	Selected, Generated, Compared, Failed int
	Results                               map[string][]Result
}

func Run(ctx context.Context, options Options) (Summary, error) {
	if ctx == nil {
		return Summary{}, fmt.Errorf("context must not be nil")
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return Summary{}, err
	}
	if options.Output == "" {
		options.Output = filepath.Join(root, "docs", "visual-go")
	}
	if options.Reference == "" {
		options.Reference = filepath.Join(root, "docs", "visual")
	}
	output, err := filepath.Abs(options.Output)
	if err != nil {
		return Summary{}, err
	}
	reference, err := filepath.Abs(options.Reference)
	if err != nil {
		return Summary{}, err
	}
	if options.Compare && samePath(output, reference) {
		return Summary{}, fmt.Errorf("output and reference must differ while comparing")
	}
	if options.Version == "" {
		options.Version = buildVersion()
	}
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	if options.Threshold == 0 {
		options.Threshold = .08
	}
	if options.Threshold < 0 || options.Threshold > 1 {
		return Summary{}, fmt.Errorf("threshold must be between 0 and 1")
	}
	assets, err := loadAssets(root)
	if err != nil {
		return Summary{}, err
	}
	defer os.RemoveAll(filepath.Dir(assets.PNG))
	engine, err := miq.NewEngine(miq.EngineOptions{
		Offline: options.Offline, FontCacheDir: filepath.Join(root, ".makeitaquote", "fonts"),
		TwemojiCacheDir: filepath.Join(root, ".makeitaquote", "twemoji"),
	})
	if err != nil {
		return Summary{}, err
	}
	runtime := &Runtime{Root: root, Engine: engine, Assets: assets}
	all := Cases()
	if err := validateCaseCount(all); err != nil {
		return Summary{}, err
	}
	selected := filterCases(all, options)
	if len(selected) == 0 {
		return Summary{}, fmt.Errorf("no gallery cases matched")
	}
	groups := uniqueGroups(all)
	selectedGroups := uniqueGroups(selected)
	if err := os.MkdirAll(output, 0o755); err != nil {
		return Summary{}, err
	}
	for _, group := range selectedGroups {
		dir := filepath.Join(output, group)
		if err := ensureWithin(output, dir); err != nil {
			return Summary{}, err
		}
		if err := os.RemoveAll(dir); err != nil {
			return Summary{}, err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Summary{}, err
		}
	}

	summary := Summary{Selected: len(selected), Results: make(map[string][]Result)}
	fmt.Fprintf(options.Stdout, "miq-gallery: rendering %d cases -> %s\n", len(selected), output)
	for _, testCase := range selected {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		result := renderCase(ctx, runtime, testCase, output, reference, options)
		summary.Results[testCase.Group] = append(summary.Results[testCase.Group], result)
		if result.OK {
			summary.Generated++
			if result.Difference != nil {
				summary.Compared++
			}
		} else {
			summary.Failed++
		}
		status := "ok"
		if !result.OK {
			status = "FAIL"
		}
		fmt.Fprintf(options.Stdout, "  %-4s %s/%s\n", status, testCase.Group, testCase.Name)
	}
	for _, group := range selectedGroups {
		manifest := GroupManifest{Name: group, Title: titleOf(group), Cases: summary.Results[group]}
		if err := writeJSON(filepath.Join(output, group, "manifest.json"), manifest); err != nil {
			return summary, err
		}
	}
	index := IndexManifest{Version: options.Version}
	for _, group := range groups {
		index.Groups = append(index.Groups, GroupIndex{Name: group, Title: titleOf(group)})
	}
	if err := writeJSON(filepath.Join(output, "manifest.json"), index); err != nil {
		return summary, err
	}
	if options.Site != "" {
		if err := writeSite(output, options.Site, index); err != nil {
			return summary, err
		}
	}
	if summary.Failed > 0 {
		return summary, fmt.Errorf("%d of %d gallery cases failed", summary.Failed, summary.Selected)
	}
	return summary, nil
}

func renderCase(ctx context.Context, runtime *Runtime, testCase Case, output, reference string, options Options) Result {
	format := testCase.Format
	if format == "" {
		format = miq.PNG
	}
	canonical, err := miq.CanonicalFormat(format)
	if err != nil {
		return failedResult(testCase, "", format, err)
	}
	extension := string(canonical)
	if canonical == miq.JPEG {
		extension = "jpg"
	}
	relative := filepath.ToSlash(filepath.Join(testCase.Group, slug(testCase.Name)+"."+extension))
	result := Result{Name: testCase.Name, File: relative, Note: optional(testCase.Note), Format: string(canonical), OK: true}
	img, err := testCase.Render(ctx, runtime)
	if err != nil {
		return failedResult(testCase, relative, canonical, err)
	}
	data, err := miq.EncodeBytes(img, miq.EncodeOptions{Format: canonical, Quality: testCase.Quality})
	if err != nil {
		return failedResult(testCase, relative, canonical, err)
	}
	if err := verifySignature(data, canonical); err != nil {
		return failedResult(testCase, relative, canonical, err)
	}
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	if testCase.ExpectWidth > 0 && width != testCase.ExpectWidth {
		return failedResult(testCase, relative, canonical, fmt.Errorf("width %d, expected %d", width, testCase.ExpectWidth))
	}
	if testCase.ExpectHeight > 0 && height != testCase.ExpectHeight {
		return failedResult(testCase, relative, canonical, fmt.Errorf("height %d, expected %d", height, testCase.ExpectHeight))
	}
	if canonical == miq.PNG {
		result.Width, result.Height = &width, &height
	}
	bytesCount := len(data)
	result.Bytes = &bytesCount
	target := filepath.Join(output, filepath.FromSlash(relative))
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return failedResult(testCase, relative, canonical, err)
	}
	if options.Compare {
		referencePath := filepath.Join(reference, filepath.FromSlash(relative))
		want, decodeErr := decodeImage(referencePath)
		if decodeErr != nil {
			return failedResult(testCase, relative, canonical, fmt.Errorf("reference: %w", decodeErr))
		}
		if img.Bounds().Size() != want.Bounds().Size() {
			return failedResult(testCase, relative, canonical, fmt.Errorf("dimensions %v, reference %v", img.Bounds().Size(), want.Bounds().Size()))
		}
		difference := BlockDifference(img, want, 30, 16)
		result.Difference = &difference
		if difference > options.Threshold {
			return failedResultWithDifference(testCase, relative, canonical, difference, options.Threshold)
		}
	}
	return result
}

func loadAssets(root string) (Assets, error) {
	temporary, err := os.MkdirTemp("", "miq-gallery-fixtures-")
	if err != nil {
		return Assets{}, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(temporary)
		}
	}()
	pngPath, jpgPath, err := testfixture.Write(temporary)
	if err != nil {
		return Assets{}, err
	}
	assets := Assets{PNG: pngPath, JPG: jpgPath}
	data, err := os.ReadFile(assets.PNG)
	if err != nil {
		return assets, err
	}
	assets.PNGBytes = data
	var urls []string
	if err := readJSON(filepath.Join(root, "assets", "imageurllist.json"), &urls); err != nil {
		return assets, fmt.Errorf("image URL fixture: %w", err)
	}
	if len(urls) == 0 {
		return assets, fmt.Errorf("image URL fixture is empty")
	}
	assets.RemoteURL = urls[0]
	if err := readJSON(filepath.Join(root, "assets", "discordemoji.json"), &assets.DiscordEmoji); err != nil {
		return assets, err
	}
	if err := readJSON(filepath.Join(root, "assets", "misskeycustomemoji.json"), &assets.Misskey); err != nil {
		return assets, err
	}
	keep = true
	return assets, nil
}

func filterCases(cases []Case, options Options) []Case {
	patterns := make([]string, 0, len(options.Only))
	for _, pattern := range options.Only {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern != "" {
			patterns = append(patterns, pattern)
		}
	}
	return slices.DeleteFunc(slices.Clone(cases), func(testCase Case) bool {
		if options.Offline && testCase.Network {
			return true
		}
		if len(patterns) == 0 {
			return false
		}
		value := strings.ToLower(testCase.Group + "\x00" + testCase.Name)
		return !slices.ContainsFunc(patterns, func(pattern string) bool { return strings.Contains(value, pattern) })
	})
}

func uniqueGroups(cases []Case) []string {
	seen := map[string]bool{}
	groups := make([]string, 0)
	for _, testCase := range cases {
		if !seen[testCase.Group] {
			seen[testCase.Group] = true
			groups = append(groups, testCase.Group)
		}
	}
	return groups
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slug(value string) string {
	value = nonSlug.ReplaceAllString(strings.ToLower(value), "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "case"
	}
	return value
}

func titleOf(group string) string {
	value := regexp.MustCompile(`^\d+-`).ReplaceAllString(group, "")
	value = strings.ReplaceAll(value, "-", " ")
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func verifySignature(data []byte, format miq.Format) error {
	valid := false
	switch format {
	case miq.PNG:
		valid = len(data) > 24 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10})
	case miq.JPEG:
		valid = len(data) > 4 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff
	case miq.WebP:
		valid = len(data) > 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	case miq.AVIF:
		valid = len(data) > 12 && string(data[4:8]) == "ftyp"
	}
	if !valid {
		return fmt.Errorf("bytes are not a valid %s", format)
	}
	return nil
}

func failedResult(testCase Case, file string, format miq.Format, err error) Result {
	message := err.Error()
	return Result{Name: testCase.Name, File: file, Note: optional(testCase.Note), Format: string(format), OK: false, Error: &message}
}

func failedResultWithDifference(testCase Case, file string, format miq.Format, difference, threshold float64) Result {
	message := fmt.Sprintf("visual difference %.4f exceeds %.4f", difference, threshold)
	return Result{Name: testCase.Name, File: file, Note: optional(testCase.Note), Format: string(format), OK: false, Error: &message, Difference: &difference}
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return DefaultVersion
}

func decodeImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	return decoded, err
}

func samePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func ensureWithin(root, target string) error {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("gallery path %q escapes output root %q", target, root)
	}
	return nil
}

func Elapsed(start time.Time) string { return time.Since(start).Round(100 * time.Millisecond).String() }
