package theme

import (
	"fmt"
	"image/color"
)

var (
	darkBackground  = color.NRGBA{A: 0xff}
	darkText        = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	darkMuted       = color.NRGBA{R: 0x9a, G: 0x9a, B: 0x9a, A: 0xff}
	darkFaint       = color.NRGBA{R: 0x6e, G: 0x6e, B: 0x6e, A: 0xff}
	lightBackground = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	lightText       = color.NRGBA{R: 0x11, G: 0x11, B: 0x11, A: 0xff}
	lightMuted      = color.NRGBA{R: 0x66, G: 0x66, B: 0x66, A: 0xff}
	lightFaint      = color.NRGBA{R: 0x99, G: 0x99, B: 0x99, A: 0xff}
)

const defaultFont = "M PLUS Rounded 1c, Noto Sans JP, Yu Gothic, Meiryo, sans-serif"

func Resolve(input Input) (Theme, error) {
	var t Theme
	if input.Base != nil {
		t = *input.Base
		t.Gradient.Stops = append([]GradientStop(nil), input.Base.Gradient.Stops...)
		if input.Base.BackgroundGradient != nil {
			gradient := *input.Base.BackgroundGradient
			gradient.Stops = append([]ColorStop(nil), input.Base.BackgroundGradient.Stops...)
			t.BackgroundGradient = &gradient
		}
	} else {
		name := input.Extends
		if name == "" {
			name = Dark
		}
		var err error
		t, err = preset(name)
		if err != nil {
			return Theme{}, err
		}
	}
	if input.Layout != nil {
		t.Layout = *input.Layout
	}

	if input.Width != nil {
		t.Width = *input.Width
	}
	if input.Height != nil {
		t.Height = *input.Height
	}
	if input.Background != nil {
		t.Background = *input.Background
	}
	if input.BackgroundGradient != nil {
		gradient := *input.BackgroundGradient
		gradient.Stops = append([]ColorStop(nil), input.BackgroundGradient.Stops...)
		t.BackgroundGradient = &gradient
	}
	if p := input.Avatar; p != nil {
		if p.Grayscale != nil {
			t.Avatar.Grayscale = *p.Grayscale
		}
		if p.Position != nil {
			t.Avatar.Position = *p.Position
		}
		if p.WidthRatio != nil {
			t.Avatar.WidthRatio = *p.WidthRatio
		}
		if p.Fit != nil {
			t.Avatar.Fit = *p.Fit
		}
		if p.Shape != nil {
			t.Avatar.Shape = *p.Shape
		}
		if p.Fallback != nil {
			t.Avatar.Fallback = *p.Fallback
		}
		if p.FallbackText != nil {
			t.Avatar.FallbackText = *p.FallbackText
		}
		if p.HasFallback != nil {
			t.Avatar.HasFallback = *p.HasFallback
		}
	}
	if p := input.Gradient; p != nil {
		if p.Enabled != nil {
			t.Gradient.Enabled = *p.Enabled
		}
		if p.Vertical != nil {
			t.Gradient.Vertical = *p.Vertical
		}
		if p.StartRatio != nil {
			t.Gradient.StartRatio = *p.StartRatio
		}
		if p.EndRatio != nil {
			t.Gradient.EndRatio = *p.EndRatio
		}
		if p.Stops != nil {
			t.Gradient.Stops = append([]GradientStop(nil), (*p.Stops)...)
		}
	}
	if p := input.Text; p != nil {
		if p.Color != nil {
			t.Text.Color = *p.Color
		}
		if p.Font != nil {
			t.Text.Font = *p.Font
		}
		if p.Size != nil {
			t.Text.Size = *p.Size
		}
		if p.MinSize != nil {
			t.Text.MinSize = *p.MinSize
		}
		if p.LineHeight != nil {
			t.Text.LineHeight = *p.LineHeight
		}
		if p.Align != nil {
			t.Text.Align = *p.Align
		}
		if p.Overflow != nil {
			t.Text.Overflow = *p.Overflow
		}
		if p.Area != nil {
			t.Text.Area = *p.Area
			t.Text.AutoArea = false
		}
		if p.PhraseBreak != nil {
			t.Text.PhraseBreak = *p.PhraseBreak
		}
		if p.Locale != nil {
			t.Text.Locale = *p.Locale
		}
		if p.Weight != nil {
			t.Text.Weight = *p.Weight
		}
		if p.AutoArea != nil {
			t.Text.AutoArea = *p.AutoArea
		}
	}
	if p := input.QuoteMark; p != nil {
		if p.Display != nil {
			t.QuoteMark.Display = *p.Display
		}
		if p.Open != nil {
			t.QuoteMark.Open = *p.Open
		}
		if p.Close != nil {
			t.QuoteMark.Close = *p.Close
		}
		if p.Size != nil {
			t.QuoteMark.Size = *p.Size
		}
		if p.Color != nil {
			t.QuoteMark.Color = *p.Color
		}
		if p.InheritColor != nil {
			t.QuoteMark.InheritColor = *p.InheritColor
		}
		if p.Gap != nil {
			t.QuoteMark.Gap = *p.Gap
		}
		if p.Weight != nil {
			t.QuoteMark.Weight = *p.Weight
		}
	}
	if p := input.Divider; p != nil {
		if p.Enabled != nil {
			t.Divider.Enabled = *p.Enabled
		}
		if p.WidthRatio != nil {
			t.Divider.WidthRatio = *p.WidthRatio
		}
		if p.Thickness != nil {
			t.Divider.Thickness = *p.Thickness
		}
		if p.Gap != nil {
			t.Divider.Gap = *p.Gap
		}
		if p.Color != nil {
			t.Divider.Color = *p.Color
		}
		if p.InheritColor != nil {
			t.Divider.InheritColor = *p.InheritColor
		}
	}
	if p := input.Emoji; p != nil {
		if p.SideMarginRatio != nil {
			t.Emoji.SideMarginRatio = *p.SideMarginRatio
		}
		if p.TopMarginRatio != nil {
			t.Emoji.TopMarginRatio = *p.TopMarginRatio
		}
		if p.Size != nil {
			t.Emoji.Size = *p.Size
		}
	}
	applyLabel := func(target *Label, p *LabelInput) {
		if p == nil {
			return
		}
		if p.Color != nil {
			target.Color = *p.Color
		}
		if p.Font != nil {
			target.Font = *p.Font
		}
		if p.Size != nil {
			target.Size = *p.Size
		}
		if p.Prefix != nil {
			target.Prefix = *p.Prefix
		}
		if p.Position != nil {
			target.Position = *p.Position
		}
		if p.Weight != nil {
			target.Weight = *p.Weight
		}
	}
	applyLabel(&t.DisplayName, input.DisplayName)
	applyLabel(&t.Username, input.Username)
	applyLabel(&t.Watermark, input.Watermark)

	if err := validate(t); err != nil {
		return Theme{}, err
	}
	return t, nil
}

func preset(name Name) (Theme, error) {
	t := dark()
	switch name {
	case Dark:
	case Light:
		t.Name = Light
		t.Background = lightBackground
		t.Avatar.Fallback = color.NRGBA{R: 0xe8, G: 0xe8, B: 0xe8, A: 0xff}
		t.Avatar.FallbackText = color.NRGBA{R: 0x7a, G: 0x7a, B: 0x7a, A: 0xff}
		t.Text.Color = lightText
		t.DisplayName.Color = lightText
		t.Username.Color = lightMuted
		t.Watermark.Color = lightFaint
	case Color:
		t.Name = Color
		t.Avatar.Grayscale = false
	case Portrait, PortraitLight:
		t.Name = name
		t.Layout = Stacked
		t.Width, t.Height = 630, 790
		t.Avatar.WidthRatio = 1
		t.Gradient = Gradient{
			Enabled: true, Vertical: true, StartRatio: 0.38, EndRatio: 0.74,
			Stops: []GradientStop{{Offset: 0, Alpha: 0}, {Offset: 0.6, Alpha: 0.7}, {Offset: 1, Alpha: 1}},
		}
		t.Text.Size, t.Text.MinSize = 0.055, 0.028
		t.Text.Area = Rect{X: 0.08, Y: 0.56, Width: 0.84, Height: 0.18}
		t.QuoteMark.Display = QuoteBlock
		t.QuoteMark.Size, t.QuoteMark.Gap = 0.12, 0.03
		t.Divider.Enabled, t.Divider.WidthRatio, t.Divider.Thickness, t.Divider.Gap = true, 0.45, 0.004, 0.03
		t.DisplayName.Size, t.DisplayName.Prefix = 0.036, ""
		t.Username.Size = 0.022
		if name == PortraitLight {
			t.Background = lightBackground
			t.Text.Color = lightText
			t.DisplayName.Color = lightText
			t.Username.Color = lightMuted
			t.Watermark.Color = lightFaint
		}
	case Custom:
		t.Name = Custom
		t.Background = color.NRGBA{}
		t.Avatar.HasFallback = false
		t.Text.Color = color.NRGBA{}
		t.DisplayName.Color = color.NRGBA{}
		t.Username.Color = color.NRGBA{}
		t.Watermark.Color = color.NRGBA{}
	default:
		return Theme{}, fmt.Errorf("unknown theme %q", name)
	}
	return t, nil
}

func dark() Theme {
	return Theme{
		Name: Dark, Layout: Side, Width: 1200, Height: 630, Background: darkBackground,
		Avatar: Avatar{
			Grayscale: true, Position: Left, WidthRatio: 0.5, Fit: Cover,
			Fallback: color.NRGBA{R: 0x1e, G: 0x1e, B: 0x1e, A: 0xff}, FallbackText: color.NRGBA{R: 0x8a, G: 0x8a, B: 0x8a, A: 0xff}, HasFallback: true, Shape: Rectangle,
		},
		Gradient: Gradient{
			Enabled: true, StartRatio: 0.22, EndRatio: 0.5,
			Stops: []GradientStop{{Offset: 0, Alpha: 0}, {Offset: 0.55, Alpha: 0.8}, {Offset: 1, Alpha: 1}},
		},
		Text: Text{
			Color: darkText, Font: defaultFont, Size: 0.062, MinSize: 0.03,
			LineHeight: 1.35, Align: AlignCenter, Overflow: Ellipsis,
			Area: Rect{X: 0.54, Y: 0.1, Width: 0.42, Height: 0.68}, PhraseBreak: true, Locale: "ja", Weight: 400, AutoArea: true,
		},
		QuoteMark:   QuoteMark{Display: QuoteNone, Open: "“", Close: "”", Size: 0.12, InheritColor: true, Gap: 0.03, Weight: 700},
		Divider:     Divider{WidthRatio: 0.45, Thickness: 0.004, Gap: 0.03, InheritColor: true},
		Emoji:       Emoji{SideMarginRatio: 0.08, TopMarginRatio: 0.1, Size: 64},
		DisplayName: Label{Color: darkText, Font: defaultFont, Size: 0.04, Prefix: "- ", Weight: 400},
		Username:    Label{Color: darkMuted, Font: defaultFont, Size: 0.028, Prefix: "@", Weight: 400},
		Watermark:   Label{Color: darkFaint, Font: defaultFont, Size: 0.024, Position: WatermarkAuto, Weight: 400},
	}
}

func validate(t Theme) error {
	if t.Width <= 0 {
		return fmt.Errorf("theme.width must be positive")
	}
	if t.Height <= 0 {
		return fmt.Errorf("theme.height must be positive")
	}
	if t.Avatar.WidthRatio <= 0 || t.Avatar.WidthRatio > 1 {
		return fmt.Errorf("theme.avatar.widthRatio must be between 0 and 1")
	}
	if t.Layout != Side && t.Layout != Stacked {
		return fmt.Errorf("unknown layout %q", t.Layout)
	}
	if t.Avatar.Position != Left && t.Avatar.Position != Right {
		return fmt.Errorf("unknown avatar position %q", t.Avatar.Position)
	}
	if t.Avatar.Fit != Cover && t.Avatar.Fit != Contain {
		return fmt.Errorf("unknown avatar fit %q", t.Avatar.Fit)
	}
	if t.Avatar.Shape != Rectangle && t.Avatar.Shape != Circle {
		return fmt.Errorf("unknown avatar shape %q", t.Avatar.Shape)
	}
	if t.Gradient.Enabled {
		if t.Gradient.StartRatio < 0 || t.Gradient.EndRatio > 1 || t.Gradient.StartRatio >= t.Gradient.EndRatio {
			return fmt.Errorf("theme.gradient ratios are invalid")
		}
		if len(t.Gradient.Stops) < 2 {
			return fmt.Errorf("theme.gradient needs at least two stops")
		}
		for _, stop := range t.Gradient.Stops {
			if stop.Offset < 0 || stop.Offset > 1 || stop.Alpha < 0 || stop.Alpha > 1 {
				return fmt.Errorf("theme.gradient stops must be between 0 and 1")
			}
		}
	}
	if t.Text.Size <= 0 || t.Text.MinSize <= 0 || t.Text.LineHeight <= 0 {
		return fmt.Errorf("theme text sizes and line height must be positive")
	}
	for name, weight := range map[string]int{"text": t.Text.Weight, "quoteMark": t.QuoteMark.Weight, "displayName": t.DisplayName.Weight, "username": t.Username.Weight, "watermark": t.Watermark.Weight} {
		if weight < 100 || weight > 900 || weight%100 != 0 {
			return fmt.Errorf("theme.%s.weight must be 100 through 900", name)
		}
	}
	if pixels(t.Text.MinSize, t.Height) > pixels(t.Text.Size, t.Height) {
		return fmt.Errorf("theme.text.minSize must not exceed theme.text.size")
	}
	if t.Text.Align != AlignLeft && t.Text.Align != AlignCenter && t.Text.Align != AlignRight {
		return fmt.Errorf("unknown text alignment %q", t.Text.Align)
	}
	if t.Text.Overflow != Ellipsis && t.Text.Overflow != Shrink && t.Text.Overflow != OverflowError {
		return fmt.Errorf("unknown text overflow %q", t.Text.Overflow)
	}
	for _, value := range []float64{t.Text.Area.X, t.Text.Area.Y, t.Text.Area.Width, t.Text.Area.Height} {
		if value < 0 || value > 1 {
			return fmt.Errorf("theme.text.area values must be between 0 and 1")
		}
	}
	if t.Emoji.SideMarginRatio < 0 || t.Emoji.TopMarginRatio < 0 {
		return fmt.Errorf("theme emoji margins must not be negative")
	}
	if t.Emoji.Size != 64 && t.Emoji.Size != 72 && t.Emoji.Size != 128 {
		return fmt.Errorf("theme.emoji.size must be 64, 72, or 128")
	}
	if t.QuoteMark.Display != QuoteInline && t.QuoteMark.Display != QuoteBlock && t.QuoteMark.Display != QuoteNone {
		return fmt.Errorf("unknown quote mark display %q", t.QuoteMark.Display)
	}
	if t.Divider.WidthRatio < 0 || t.Divider.WidthRatio > 1 || t.Divider.Thickness < 0 || t.Divider.Gap < 0 {
		return fmt.Errorf("theme divider values are invalid")
	}
	for name, label := range map[string]Label{"displayName": t.DisplayName, "username": t.Username, "watermark": t.Watermark} {
		if label.Size <= 0 {
			return fmt.Errorf("theme.%s.size must be positive", name)
		}
	}
	if gradient := t.BackgroundGradient; gradient != nil {
		if gradient.Type != LinearGradient && gradient.Type != RadialGradient {
			return fmt.Errorf("unknown background gradient type %q", gradient.Type)
		}
		if gradient.Direction != GradientHorizontal && gradient.Direction != GradientVertical && gradient.Direction != GradientDiagonal && gradient.Direction != GradientDiagonalReverse {
			return fmt.Errorf("unknown background gradient direction %q", gradient.Direction)
		}
		if len(gradient.Stops) < 2 {
			return fmt.Errorf("theme.backgroundGradient needs at least two stops")
		}
		for _, stop := range gradient.Stops {
			if stop.Offset < 0 || stop.Offset > 1 {
				return fmt.Errorf("background gradient offsets must be between 0 and 1")
			}
		}
	}
	return nil
}

func Pixels(value float64, basis int) float64 { return pixels(value, basis) }

func pixels(value float64, basis int) float64 {
	if value > 0 && value <= 1 {
		return value * float64(basis)
	}
	return value
}
