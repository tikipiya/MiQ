package emoji

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
	budoux "github.com/soundkitchen/go-budoux"
)

type Measure func(string) float64

type WrapOptions struct {
	PhraseBreak bool
	Locale      string
}

func MeasureSegments(segments []Segment, emojiWidth float64, measure Measure) float64 {
	width := 0.0
	for _, segment := range segments {
		if segment.IsEmoji() {
			width += emojiWidth
		} else {
			width += measure(segment.Text)
		}
	}
	return width
}

func WrapSegments(segments []Segment, maxWidth, emojiWidth float64, measure Measure) [][]Segment {
	return WrapSegmentsWithOptions(segments, maxWidth, emojiWidth, measure, WrapOptions{PhraseBreak: true, Locale: "ja"})
}

func WrapSegmentsWithOptions(segments []Segment, maxWidth, emojiWidth float64, measure Measure, options WrapOptions) [][]Segment {
	atoms := atomize(segments)
	phraseBoundaries := budouxBoundaries(atoms, options)
	var lines [][]Segment
	start := 0
	for start < len(atoms) {
		if atoms[start].newline {
			lines = append(lines, nil)
			start++
			continue
		}
		end := start
		lastPhrase := -1
		lastBreak := -1
		for end < len(atoms) && !atoms[end].newline {
			candidate := segmentsFromAtoms(atoms[start : end+1])
			if MeasureSegments(candidate, emojiWidth, measure) > maxWidth && end > start {
				if atoms[end].space {
					lastPhrase = end
				}
				break
			}
			if atoms[end].space {
				lastPhrase = end
			}
			if end+1 < len(atoms) {
				priority := breakPriority(atoms[end], atoms[end+1], options)
				if phraseBoundaries[end+1] {
					priority = 2
				}
				if priority > 0 {
					lastBreak = end + 1
				}
				if priority == 2 {
					lastPhrase = end + 1
				}
			}
			end++
			if MeasureSegments(candidate, emojiWidth, measure) > maxWidth {
				break
			}
		}
		if end == len(atoms) || end < len(atoms) && atoms[end].newline {
			lines = append(lines, trimLine(segmentsFromAtoms(atoms[start:end])))
			if end < len(atoms) && atoms[end].newline {
				end++
			}
			start = end
			continue
		}
		if lastPhrase > start && lastPhrase <= end {
			lines = append(lines, trimLine(segmentsFromAtoms(atoms[start:lastPhrase])))
			start = lastPhrase
			for start < len(atoms) && atoms[start].space {
				start++
			}
			continue
		}
		if lastBreak > start && lastBreak <= end {
			lines = append(lines, trimLine(segmentsFromAtoms(atoms[start:lastBreak])))
			start = lastBreak
			continue
		}
		if end == start {
			end++
		}
		lines = append(lines, segmentsFromAtoms(atoms[start:end]))
		start = end
	}
	if len(lines) == 0 {
		return [][]Segment{{}}
	}
	return lines
}

func breakPriority(left, right atom, options WrapOptions) int {
	if left.newline || right.newline || left.space {
		return 2
	}
	if left.segment.IsEmoji() || right.segment.IsEmoji() {
		return 2
	}
	lr, _ := utf8.DecodeLastRuneInString(left.segment.Text)
	rr, _ := utf8.DecodeRuneInString(right.segment.Text)
	if strings.ContainsRune("（［｛〈《「『【〔〘〖([{｢“‘", lr) {
		return 0
	}
	if strings.ContainsRune("、。，．,.）］｝〉》」』】〕〙〗)]}｡､･ーゝゞヽヾ々!?！？:;：；・…‥ぁぃぅぇぉっゃゅょゎゕゖァィゥェォッャュョヮヵヶ", rr) {
		return 0
	}
	if lr == '-' || lr == '/' {
		return 2
	}
	if isCJK(lr) || isCJK(rr) {
		return 1
	}
	return 0
}

func budouxBoundaries(atoms []atom, options WrapOptions) map[int]bool {
	boundaries := make(map[int]bool)
	if !options.PhraseBreak || options.Locale == "none" {
		return boundaries
	}
	parser := parserForLocale(options.Locale)
	if parser == nil {
		return boundaries
	}
	for start := 0; start < len(atoms); {
		for start < len(atoms) && (atoms[start].newline || atoms[start].segment.IsEmoji()) {
			start++
		}
		end := start
		var text strings.Builder
		for end < len(atoms) && !atoms[end].newline && !atoms[end].segment.IsEmoji() {
			text.WriteString(atoms[end].segment.Text)
			end++
		}
		if end == start {
			continue
		}
		chunks := parser.Parse(text.String())
		wanted := make(map[int]bool)
		cumulative := 0
		for _, chunk := range chunks[:max(0, len(chunks)-1)] {
			cumulative += len(chunk)
			wanted[cumulative] = true
		}
		consumed := 0
		for index := start; index < end; index++ {
			consumed += len(atoms[index].segment.Text)
			if wanted[consumed] {
				boundaries[index+1] = true
			}
		}
		start = end
	}
	return boundaries
}

func parserForLocale(locale string) *budoux.Parser {
	switch strings.ToLower(locale) {
	case "zh-hans":
		return simplifiedChineseParser
	case "zh-hant":
		return traditionalChineseParser
	case "ja", "":
		return japaneseParser
	default:
		return nil
	}
}

var (
	japaneseParser           = budoux.NewDefaultJapaneseParser()
	simplifiedChineseParser  = budoux.NewDefaultSimplifiedChineseParser()
	traditionalChineseParser = budoux.NewDefaultTraditionalChineseParser()
)

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

func EllipsizeLine(line []Segment, maxWidth, emojiWidth float64, measure Measure) []Segment {
	const mark = "…"
	atoms := atomize(line)
	for len(atoms) > 0 {
		candidate := append(segmentsFromAtoms(atoms), Segment{Text: mark})
		if MeasureSegments(candidate, emojiWidth, measure) <= maxWidth {
			return trimLine(candidate)
		}
		atoms = atoms[:len(atoms)-1]
	}
	if measure(mark) <= maxWidth {
		return []Segment{{Text: mark}}
	}
	return nil
}

type atom struct {
	segment Segment
	space   bool
	newline bool
}

func atomize(segments []Segment) []atom {
	var atoms []atom
	for _, segment := range segments {
		if segment.IsEmoji() {
			atoms = append(atoms, atom{segment: segment})
			continue
		}
		g := uniseg.NewGraphemes(strings.ReplaceAll(segment.Text, "\r\n", "\n"))
		for g.Next() {
			cluster := g.Str()
			atoms = append(atoms, atom{
				segment: Segment{Text: cluster}, newline: cluster == "\n",
				space: cluster != "\n" && strings.TrimSpace(cluster) == "",
			})
		}
	}
	return atoms
}

func segmentsFromAtoms(atoms []atom) []Segment {
	var out []Segment
	for _, item := range atoms {
		if item.newline {
			continue
		}
		push(&out, item.segment)
	}
	return out
}

func trimLine(line []Segment) []Segment {
	if len(line) == 0 {
		return line
	}
	if !line[0].IsEmoji() {
		line[0].Text = strings.TrimLeftFunc(line[0].Text, func(r rune) bool { return r == ' ' || r == '\t' })
	}
	if len(line) > 0 && !line[len(line)-1].IsEmoji() {
		line[len(line)-1].Text = strings.TrimRightFunc(line[len(line)-1].Text, func(r rune) bool { return r == ' ' || r == '\t' })
	}
	if len(line) > 0 && line[0].Text == "" && !line[0].IsEmoji() {
		line = line[1:]
	}
	if len(line) > 0 && line[len(line)-1].Text == "" && !line[len(line)-1].IsEmoji() {
		line = line[:len(line)-1]
	}
	return line
}
