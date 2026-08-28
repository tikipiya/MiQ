package theme

import (
	"image/color"
	"strings"
	"testing"
)

func TestParseColorCompatibility(t *testing.T) {
	tests := []struct {
		input any
		want  color.NRGBA
	}{
		{"#FF0000", color.NRGBA{R: 255, A: 255}},
		{"#F00", color.NRGBA{R: 255, A: 255}},
		{"#FF000080", color.NRGBA{R: 255, A: 128}},
		{"#F000", color.NRGBA{R: 255}},
		{"0xFF0000", color.NRGBA{R: 255, A: 255}},
		{0xff0000, color.NRGBA{R: 255, A: 255}},
		{uint32(0xff000080), color.NRGBA{R: 255, A: 128}},
		{[]int{300, -20, 0}, color.NRGBA{R: 255, A: 255}},
		{[]float64{255, 0, 0, .5}, color.NRGBA{R: 255, A: 128}},
		{"transparent", color.NRGBA{}},
		{"rebeccapurple", color.NRGBA{R: 102, G: 51, B: 153, A: 255}},
		{"RGB(255 0 0 / 50%)", color.NRGBA{R: 255, A: 128}},
		{"hsl(0, 100%, 50%)", color.NRGBA{R: 255, A: 255}},
		{"hwb(0 0% 0%)", color.NRGBA{R: 255, A: 255}},
		{"lab(50% 40 59.5)", color.NRGBA{R: 191, G: 87, A: 255}},
		{"lch(50% 60 30)", color.NRGBA{R: 202, G: 73, B: 72, A: 255}},
		{"oklab(59% 0.1 0.1)", color.NRGBA{R: 192, G: 93, B: 43, A: 255}},
		{"oklch(60% 0.15 30)", color.NRGBA{R: 202, G: 87, B: 71, A: 255}},
		{"color(srgb 1 0 0)", color.NRGBA{R: 255, A: 255}},
	}
	for _, test := range tests {
		got, err := ParseColor(test.input)
		if err != nil || got != test.want {
			t.Errorf("ParseColor(%v)=%v,%v want %v", test.input, got, err, test.want)
		}
	}
}

func TestParseColorErrorsAndFormatting(t *testing.T) {
	for _, input := range []any{"#GG0000", "not a color", -1, uint64(0x1_0000_0000), []int{1, 2}} {
		if _, err := ParseColor(input, "theme.background"); err == nil || !strings.Contains(err.Error(), "theme.background") {
			t.Errorf("ParseColor(%v) error=%v", input, err)
		}
	}
	red := color.NRGBA{R: 255, A: 255}
	if ColorCSS(red) != "rgb(255, 0, 0)" || ColorCSS(color.NRGBA{R: 255, A: 128}) != "rgba(255, 0, 0, 0.502)" {
		t.Fatal("CSS formatting mismatch")
	}
	if WithAlpha(red, .5) != "rgba(255, 0, 0, 0.5)" || ColorHex(red) != "#FF0000FF" || !IsTransparent(color.NRGBA{}) {
		t.Fatal("color helper mismatch")
	}
}
