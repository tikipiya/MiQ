package layout

import (
	"github.com/tikipiya/MiQ/theme"
	"math"
)

type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type Layout struct {
	Avatar  Rect
	Text    Rect
	CenterX float64
}

func Compute(t theme.Theme) Layout {
	w, h := float64(t.Width), float64(t.Height)
	avatarWidth := w * t.Avatar.WidthRatio
	if t.Layout == theme.Stacked {
		avatarWidth = w
	}
	avatarX := 0.0
	if t.Layout == theme.Side && t.Avatar.Position == theme.Right {
		avatarX = w - avatarWidth
	}

	area := t.Text.Area
	if t.Text.AutoArea {
		if t.Layout == theme.Stacked {
			area = theme.Rect{X: 0.08, Y: 0.56, Width: 0.84, Height: 0.18}
		} else {
			const gap = 0.04
			area = theme.Rect{X: t.Avatar.WidthRatio + gap, Y: 0.1, Width: math.Max(0.1, 1-t.Avatar.WidthRatio-gap*2), Height: 0.68}
			if t.Avatar.Position == theme.Right {
				area.X = gap
			}
		}
	}
	text := Rect{X: area.X * w, Y: area.Y * h, Width: area.Width * w, Height: area.Height * h}
	return Layout{
		Avatar:  Rect{X: avatarX, Y: 0, Width: avatarWidth, Height: h},
		Text:    text,
		CenterX: text.X + text.Width/2,
	}
}
