package emoji

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWrapSegmentsKinsoku(t *testing.T) {
	measure := func(value string) float64 { return float64(utf8.RuneCountInString(value)) }
	lines := WrapSegments([]Segment{{Text: "これは（テスト）です。"}}, 5, 1, measure)
	for _, line := range lines {
		var value strings.Builder
		for _, segment := range line {
			value.WriteString(segment.Text)
		}
		text := value.String()
		if strings.HasPrefix(text, "）") || strings.HasPrefix(text, "。") || strings.HasSuffix(text, "（") {
			t.Fatalf("illegal Japanese line break: %q", text)
		}
	}
}

func TestWrapSegmentsPrefersJapanesePhraseBoundary(t *testing.T) {
	measure := func(value string) float64 { return float64(utf8.RuneCountInString(value)) }
	segments := []Segment{{Text: "今日は天気です"}}
	withPhrases := WrapSegmentsWithOptions(segments, 4, 1, measure, WrapOptions{PhraseBreak: true, Locale: "ja"})
	withoutPhrases := WrapSegmentsWithOptions(segments, 4, 1, measure, WrapOptions{PhraseBreak: false, Locale: "ja"})
	plain := func(line []Segment) string {
		var out strings.Builder
		for _, segment := range line {
			out.WriteString(segment.Text)
		}
		return out.String()
	}
	if got := plain(withPhrases[0]); got != "今日は" {
		t.Fatalf("phrase line=%q", got)
	}
	if got := plain(withoutPhrases[0]); got != "今日は天" {
		t.Fatalf("character fallback=%q", got)
	}
}

func TestWrapSegmentsBreaksAfterHyphenAndProtectsGraphemes(t *testing.T) {
	measure := func(value string) float64 { return float64(utf8.RuneCountInString(value)) }
	lines := WrapSegments([]Segment{{Text: "e-mail-box"}}, 7, 1, measure)
	if got := segmentsFromAtoms(atomize(lines[0]))[0].Text; got != "e-mail-" {
		t.Fatalf("hyphen line=%q", got)
	}
	emojiLines := WrapSegments([]Segment{{Text: "A👨‍👩‍👧‍👦B"}}, 1, 1, func(value string) float64 {
		if value == "👨‍👩‍👧‍👦" {
			return 1
		}
		return measure(value)
	})
	if len(emojiLines) != 3 {
		t.Fatalf("grapheme lines=%d", len(emojiLines))
	}
}
