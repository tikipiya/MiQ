package theme

import (
	"image/color"
	"testing"
)

func TestResolveDark(t *testing.T) {
	got, err := Resolve(Input{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != Dark || got.Width != 1200 || got.Height != 630 {
		t.Fatalf("unexpected dark preset: %#v", got)
	}
	if got.Text.Area != (Rect{X: 0.54, Y: 0.1, Width: 0.42, Height: 0.68}) || !got.Text.AutoArea {
		t.Fatalf("unexpected text area: %#v", got.Text.Area)
	}
}

func TestResolvePartialOverride(t *testing.T) {
	width := 800
	keepColor := false
	background := color.NRGBA{R: 1, G: 2, B: 3, A: 4}
	got, err := Resolve(Input{
		Extends: Light, Width: &width, Background: &background,
		Avatar: &AvatarInput{Grayscale: &keepColor},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 800 || got.Height != 630 || got.Background != background || got.Avatar.Grayscale {
		t.Fatalf("override was not applied: %#v", got)
	}
}

func TestResolveRejectsInvalidTheme(t *testing.T) {
	if _, err := Resolve(Preset("unknown")); err == nil {
		t.Fatal("expected an error")
	}
	ratio := 2.0
	if _, err := Resolve(Input{Avatar: &AvatarInput{WidthRatio: &ratio}}); err == nil {
		t.Fatal("expected an invalid ratio error")
	}
}

func TestPortraitMatchesCurrentDimensions(t *testing.T) {
	got, err := Resolve(Preset(Portrait))
	if err != nil {
		t.Fatal(err)
	}
	if got.Layout != Stacked || got.Width != 630 || got.Height != 790 || !got.Gradient.Vertical {
		t.Fatalf("unexpected portrait preset: %#v", got)
	}
}

func TestResolveDeepCopyAndPartialNestedOverrides(t *testing.T) {
	display := QuoteBlock
	red := color.NRGBA{R: 255, A: 255}
	font := "Custom Font"
	resolved, err := Resolve(Input{QuoteMark: &QuoteMarkInput{Display: &display}, Text: &TextInput{Color: &red}, DisplayName: &LabelInput{Font: &font}})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.QuoteMark.Display != QuoteBlock || resolved.QuoteMark.Open == "" || resolved.QuoteMark.Weight != 700 || resolved.Text.Font == "" || resolved.DisplayName.Font != font {
		t.Fatalf("partial merge failed: %#v", resolved)
	}
	copyInput := Exact(resolved)
	copyOne, err := Resolve(copyInput)
	if err != nil {
		t.Fatal(err)
	}
	copyOne.Gradient.Stops[0].Alpha = 0.75
	copyTwo, err := Resolve(copyInput)
	if err != nil {
		t.Fatal(err)
	}
	if copyTwo.Gradient.Stops[0].Alpha == 0.75 {
		t.Fatal("Exact did not deep-copy gradient stops")
	}
}

func TestResolveValidationCompatibility(t *testing.T) {
	zero, negative := 0, -1
	if _, err := Resolve(Input{Width: &zero}); err == nil {
		t.Fatal("accepted zero width")
	}
	if _, err := Resolve(Input{Height: &negative}); err == nil {
		t.Fatal("accepted negative height")
	}
	maxSize, minSize := 0.05, 0.06
	if _, err := Resolve(Input{Text: &TextInput{Size: &maxSize, MinSize: &minSize}}); err == nil {
		t.Fatal("accepted min size above max")
	}
	hexagon := Shape("hexagon")
	if _, err := Resolve(Input{Avatar: &AvatarInput{Shape: &hexagon}}); err == nil {
		t.Fatal("accepted unknown shape")
	}
	conic := BackgroundGradientType("conic")
	upward := BackgroundGradientDirection("upward")
	stops := []ColorStop{{Color: color.NRGBA{A: 255}, Offset: 0}, {Color: color.NRGBA{R: 255, A: 255}, Offset: 1}}
	if _, err := Resolve(Input{BackgroundGradient: &BackgroundGradient{Type: conic, Direction: GradientHorizontal, Stops: stops}}); err == nil {
		t.Fatal("accepted conic gradient")
	}
	if _, err := Resolve(Input{BackgroundGradient: &BackgroundGradient{Type: LinearGradient, Direction: upward, Stops: stops}}); err == nil {
		t.Fatal("accepted unknown direction")
	}
	if _, err := Resolve(Input{BackgroundGradient: &BackgroundGradient{Type: LinearGradient, Direction: GradientHorizontal, Stops: stops[:1]}}); err == nil {
		t.Fatal("accepted one-stop gradient")
	}
}

func TestPixelsCompatibility(t *testing.T) {
	for _, test := range []struct {
		value float64
		basis int
		want  float64
	}{{0.5, 720, 360}, {1, 720, 720}, {45, 720, 45}} {
		if got := Pixels(test.value, test.basis); got != test.want {
			t.Fatalf("Pixels(%v,%d)=%v", test.value, test.basis, got)
		}
	}
}
