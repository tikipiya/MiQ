package emoji

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

const twemojiBase = "https://cdn.jsdelivr.net/gh/jdecked/twemoji@17.0.2/assets/72x72/"

type Source string

const (
	Twemoji Source = "twemoji"
	Discord Source = "discord"
	Misskey Source = "misskey"
)

type Segment struct {
	Text            string
	Source          Source
	URL             string
	Raw             string
	Name            string
	ID              string
	Animated        bool
	Host            string
	AlternativeURLs []string
}

func (s Segment) IsEmoji() bool { return s.URL != "" }

type MisskeyOptions struct {
	Instances []string
	Remote    *bool
}

type SegmentOptions struct {
	DiscordSize int
	Misskey     MisskeyOptions
}

var (
	discordPattern = regexp.MustCompile(`^<(a)?:([A-Za-z0-9_]{2,32}):(\d{17,20})>`)
	misskeyPattern = regexp.MustCompile(`^:([A-Za-z0-9_+\-]{2,64})(?:@([A-Za-z0-9.\-]+|\.))?:`)
	numericOnly    = regexp.MustCompile(`^\d+$`)
)

func SegmentText(value string, opts SegmentOptions) []Segment {
	size := opts.DiscordSize
	if size == 0 {
		size = 64
	}
	hosts := normalizeHosts(opts.Misskey.Instances)
	remote := true
	if opts.Misskey.Remote != nil {
		remote = *opts.Misskey.Remote
	}
	var out []Segment
	for offset := 0; offset < len(value); {
		rest := value[offset:]
		if match := discordPattern.FindStringSubmatch(rest); match != nil {
			animated := match[1] == "a"
			ext := "png"
			if animated {
				ext = "gif"
			}
			push(&out, Segment{
				Source: Discord, URL: fmt.Sprintf("https://cdn.discordapp.com/emojis/%s.%s?size=%d", match[3], ext, size),
				Raw: match[0], Name: match[2], ID: match[3], Animated: animated,
			})
			offset += len(match[0])
			continue
		}
		if (offset == 0 || !asciiWord(value[offset-1])) && strings.HasPrefix(rest, ":") {
			if match := misskeyPattern.FindStringSubmatch(rest); match != nil && !numericOnly.MatchString(match[1]) {
				federated := match[2] != "" && match[2] != "."
				candidates := hosts
				if federated {
					if !remote {
						candidates = nil
					} else {
						candidates = normalizeHosts([]string{match[2]})
					}
				}
				if len(candidates) > 0 {
					urls := make([]string, len(candidates))
					for i, host := range candidates {
						urls[i] = "https://" + host + "/emoji/" + url.PathEscape(match[1]) + ".webp"
					}
					push(&out, Segment{
						Source: Misskey, URL: urls[0], AlternativeURLs: urls[1:], Raw: match[0],
						Name: match[1], Host: candidates[0],
					})
					offset += len(match[0])
					continue
				}
			}
		}

		cluster, restAfter, _, _ := uniseg.FirstGraphemeClusterInString(rest, -1)
		if cluster == "" {
			break
		}
		if isEmojiCluster(cluster) {
			push(&out, Segment{Source: Twemoji, URL: twemojiURL(cluster), Raw: cluster})
		} else {
			push(&out, Segment{Text: cluster})
		}
		offset = len(value) - len(restAfter)
	}
	return out
}

func ResolveMissing(segments []Segment, hasImage func(string) bool, behavior string) ([]Segment, error) {
	out := make([]Segment, 0, len(segments))
	for _, segment := range segments {
		if !segment.IsEmoji() || hasImage(segment.URL) {
			push(&out, segment)
			continue
		}
		switch behavior {
		case "ignore":
		case "throw":
			return nil, fmt.Errorf("emoji %q could not be loaded", segment.Raw)
		default:
			push(&out, Segment{Text: segment.Raw})
		}
	}
	return out, nil
}

func push(out *[]Segment, segment Segment) {
	if segment.Text == "" && !segment.IsEmoji() {
		return
	}
	if segment.Text != "" && len(*out) > 0 && !(*out)[len(*out)-1].IsEmoji() {
		(*out)[len(*out)-1].Text += segment.Text
		return
	}
	*out = append(*out, segment)
}

func twemojiURL(cluster string) string {
	parts := make([]string, 0, len(cluster))
	for _, r := range cluster {
		if r == 0xfe0f {
			continue
		}
		parts = append(parts, strconv.FormatInt(int64(r), 16))
	}
	return twemojiBase + strings.Join(parts, "-") + ".png"
}

func isEmojiCluster(cluster string) bool {
	for _, r := range cluster {
		if r == 0x200d || r == 0x20e3 || r == 0xfe0f || r >= 0x1f000 && r <= 0x1faff ||
			r >= 0x2600 && r <= 0x27ff || r >= 0x1f1e6 && r <= 0x1f1ff ||
			r == 0x00a9 || r == 0x00ae || r == 0x203c || r == 0x2049 ||
			r == 0x2122 || r == 0x2139 || unicode.Is(unicode.Sk, r) && r > 0x1f000 {
			return true
		}
	}
	return false
}

func asciiWord(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' || value == '_'
}

func normalizeHosts(values []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err == nil && parsed.Host == "" {
			parsed, err = url.Parse("https://" + value)
		}
		if err != nil || parsed.Host == "" {
			continue
		}
		host := strings.ToLower(parsed.Host)
		if !seen[host] {
			seen[host] = true
			out = append(out, host)
		}
	}
	return out
}
