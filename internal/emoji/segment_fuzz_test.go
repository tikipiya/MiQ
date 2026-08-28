package emoji

import "testing"

func FuzzSegmentAndWrap(f *testing.F) {
	for _, seed := range []string{"今日は天気です", "👨‍👩‍👧‍👦", "<:cat:123456789012345678>", ":blob@example.test:", "a\n\nb"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		segments := SegmentText(input, SegmentOptions{})
		lines := WrapSegments(segments, 80, 16, func(value string) float64 { return float64(len([]rune(value))) * 8 })
		if len(lines) == 0 {
			t.Fatal("wrapper returned no lines")
		}
	})
}
