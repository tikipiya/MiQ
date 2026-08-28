package layout

import (
	"github.com/tikipiya/MiQ/theme"
	"math"
	"testing"
)

func resolved(t *testing.T, input theme.Input) theme.Theme {
	t.Helper()
	value, err := theme.Resolve(input)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func closeTo(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSideLayoutTracksAvatar(t *testing.T) {
	base := resolved(t, theme.Preset(theme.Dark))
	layout := Compute(base)
	if layout.Avatar != (Rect{X: 0, Y: 0, Width: 600, Height: 630}) {
		t.Fatalf("avatar=%#v", layout.Avatar)
	}
	if !closeTo(layout.Text.X, 1200*0.54) || !closeTo(layout.Text.X+layout.Text.Width, 1200*0.96) {
		t.Fatalf("text=%#v", layout.Text)
	}
	right := theme.Right
	flipped := resolved(t, theme.Input{Avatar: &theme.AvatarInput{Position: &right}})
	rightLayout := Compute(flipped)
	if rightLayout.Avatar.X != 600 || !closeTo(rightLayout.Text.X, 1200*0.04) || !closeTo(rightLayout.Text.X+rightLayout.Text.Width, 1200*0.46) {
		t.Fatalf("right layout=%#v", rightLayout)
	}
	for _, position := range []theme.Position{theme.Left, theme.Right} {
		for _, ratio := range []float64{0.3, 0.5, 0.65} {
			candidate := resolved(t, theme.Input{Avatar: &theme.AvatarInput{Position: &position, WidthRatio: &ratio}})
			got := Compute(candidate)
			overlap := got.Text.X < got.Avatar.X+got.Avatar.Width && got.Text.X+got.Text.Width > got.Avatar.X
			if overlap {
				t.Fatalf("overlap for %s @ %.2f: %#v", position, ratio, got)
			}
		}
	}
}

func TestExplicitAndStackedLayout(t *testing.T) {
	area := theme.Rect{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4}
	explicit := resolved(t, theme.Input{Text: &theme.TextInput{Area: &area}})
	got := Compute(explicit)
	if got.Text != (Rect{X: 120, Y: 126, Width: 360, Height: 252}) {
		t.Fatalf("explicit=%#v", got.Text)
	}
	portrait := resolved(t, theme.Preset(theme.Portrait))
	stacked := Compute(portrait)
	if stacked.Avatar != (Rect{X: 0, Y: 0, Width: 630, Height: 790}) || stacked.Text.Y <= float64(portrait.Height)*0.5 || !closeTo(stacked.CenterX, float64(portrait.Width)/2) {
		t.Fatalf("stacked=%#v", stacked)
	}
	right := theme.Right
	portraitRight := resolved(t, theme.Input{Extends: theme.Portrait, Avatar: &theme.AvatarInput{Position: &right}})
	if Compute(portraitRight).Avatar != stacked.Avatar {
		t.Fatal("stacked layout should ignore avatar side")
	}
}
