package textlayout

import (
	"strings"

	"github.com/rivo/uniseg"
)

type Measure func(string) float64

// Wrap performs deterministic greedy wrapping on grapheme cluster boundaries.
// Spaces are preferred for Latin text; CJK and oversized tokens always retain
// a grapheme-level escape hatch.
func Wrap(text string, maxWidth float64, measure Measure) []string {
	paragraphs := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapParagraph(paragraph, maxWidth, measure)...)
	}
	return lines
}

func wrapParagraph(paragraph string, maxWidth float64, measure Measure) []string {
	clusters := graphemes(paragraph)
	var lines []string
	start := 0
	for start < len(clusters) {
		end := start
		lastSpace := -1
		for end < len(clusters) {
			candidate := strings.Join(clusters[start:end+1], "")
			if measure(candidate) > maxWidth && end > start {
				// A trailing separator does not have to fit on the painted line.
				// Treat it as the preferred break and consume it below.
				if strings.TrimSpace(clusters[end]) == "" {
					lastSpace = end
				}
				break
			}
			if strings.TrimSpace(clusters[end]) == "" {
				lastSpace = end
			}
			end++
			if measure(candidate) > maxWidth {
				break
			}
		}

		if end >= len(clusters) {
			lines = append(lines, strings.TrimSpace(strings.Join(clusters[start:], "")))
			break
		}
		if lastSpace >= start {
			line := strings.TrimSpace(strings.Join(clusters[start:lastSpace], ""))
			if line != "" {
				lines = append(lines, line)
			}
			start = lastSpace + 1
			for start < len(clusters) && strings.TrimSpace(clusters[start]) == "" {
				start++
			}
			continue
		}

		if end == start {
			end++
		}
		lines = append(lines, strings.Join(clusters[start:end], ""))
		start = end
	}
	return lines
}

func Ellipsize(line string, maxWidth float64, measure Measure) string {
	const mark = "…"
	if measure(line) <= maxWidth {
		return line
	}
	clusters := graphemes(line)
	for len(clusters) > 0 && measure(strings.Join(clusters, "")+mark) > maxWidth {
		clusters = clusters[:len(clusters)-1]
	}
	if measure(mark) > maxWidth {
		return ""
	}
	return strings.TrimSpace(strings.Join(clusters, "")) + mark
}

func graphemes(value string) []string {
	g := uniseg.NewGraphemes(value)
	out := make([]string, 0, len(value))
	for g.Next() {
		out = append(out, g.Str())
	}
	return out
}
