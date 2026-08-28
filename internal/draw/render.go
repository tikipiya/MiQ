package draw

import (
	"cmp"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"math"
	"slices"
	"strings"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
	internalemoji "github.com/tikipiya/MiQ/internal/emoji"
	"github.com/tikipiya/MiQ/internal/layout"
	"github.com/tikipiya/MiQ/theme"
	xdraw "golang.org/x/image/draw"
)

const pointsPerPixel = 72.0 / 25.4

type Quote struct {
	Text              string
	Segments          []internalemoji.Segment
	EmojiImages       map[string]image.Image
	Username          string
	DisplayName       string
	Watermark         string
	Background        image.Image
	BackgroundFit     theme.Fit
	BackgroundOpacity float64
	AvatarMissing     bool
}

func Render(q Quote, avatar image.Image, t theme.Theme, fonts *FontRegistry) (*image.NRGBA, error) {
	if t.Width <= 0 || t.Height <= 0 {
		return nil, fmt.Errorf("invalid canvas size: %w", fmt.Errorf("%dx%d", t.Width, t.Height))
	}
	out := image.NewNRGBA(image.Rect(0, 0, t.Width, t.Height))
	fill(out, t.Background)
	drawBackgroundGradient(out, t.BackgroundGradient)
	if q.Background != nil {
		drawBackgroundImage(out, q.Background, q.BackgroundFit, q.BackgroundOpacity)
	}

	boxes := layout.Compute(t)
	if err := drawAvatar(out, avatar, boxes.Avatar, t); err != nil {
		return nil, err
	}
	q.AvatarMissing = avatar == nil
	if fonts == nil {
		return nil, fmt.Errorf("font registry is nil")
	}
	if err := drawText(out, q, boxes, t, fonts); err != nil {
		return nil, err
	}
	return out, nil
}

func drawBackgroundImage(dst *image.NRGBA, src image.Image, fit theme.Fit, opacity float64) {
	resized := resize(src, dst.Bounds().Dx(), dst.Bounds().Dy(), fit)
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			c := color.NRGBAModel.Convert(resized.At(x, y)).(color.NRGBA)
			blend(dst, x, y, c, opacity)
		}
	}
}

func fill(dst *image.NRGBA, c color.NRGBA) {
	stddraw.Draw(dst, dst.Bounds(), &image.Uniform{C: c}, image.Point{}, stddraw.Src)
}

func drawBackgroundGradient(dst *image.NRGBA, gradient *theme.BackgroundGradient) {
	if gradient == nil || len(gradient.Stops) < 2 {
		return
	}
	stops := append([]theme.ColorStop(nil), gradient.Stops...)
	slices.SortFunc(stops, func(a, b theme.ColorStop) int { return cmp.Compare(a.Offset, b.Offset) })
	b := dst.Bounds()
	cx, cy := float64(b.Dx()-1)/2, float64(b.Dy()-1)/2
	radius := math.Hypot(cx, cy)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			var progress float64
			if gradient.Type == theme.RadialGradient {
				progress = math.Hypot(float64(x)-cx, float64(y)-cy) / radius
			} else {
				switch gradient.Direction {
				case theme.GradientVertical:
					progress = float64(y) / math.Max(1, float64(b.Dy()-1))
				case theme.GradientDiagonal:
					progress = (float64(x)/math.Max(1, float64(b.Dx()-1)) + float64(y)/math.Max(1, float64(b.Dy()-1))) / 2
				case theme.GradientDiagonalReverse:
					progress = ((1 - float64(x)/math.Max(1, float64(b.Dx()-1))) + float64(y)/math.Max(1, float64(b.Dy()-1))) / 2
				default:
					progress = float64(x) / math.Max(1, float64(b.Dx()-1))
				}
			}
			c := interpolateColor(stops, math.Max(0, math.Min(1, progress)))
			blend(dst, x, y, c, 1)
		}
	}
}

func interpolateColor(stops []theme.ColorStop, progress float64) color.NRGBA {
	if progress <= stops[0].Offset {
		return stops[0].Color
	}
	for i := 1; i < len(stops); i++ {
		if progress <= stops[i].Offset {
			span := stops[i].Offset - stops[i-1].Offset
			if span <= 0 {
				return stops[i].Color
			}
			p := (progress - stops[i-1].Offset) / span
			mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a) + (float64(b)-float64(a))*p)) }
			return color.NRGBA{mix(stops[i-1].Color.R, stops[i].Color.R), mix(stops[i-1].Color.G, stops[i].Color.G), mix(stops[i-1].Color.B, stops[i].Color.B), mix(stops[i-1].Color.A, stops[i].Color.A)}
		}
	}
	return stops[len(stops)-1].Color
}

func drawAvatar(dst *image.NRGBA, src image.Image, box layout.Rect, t theme.Theme) error {
	w, h := int(math.Round(box.Width)), int(math.Round(box.Height))
	if w <= 0 || h <= 0 {
		return nil
	}
	if src == nil {
		if t.Avatar.HasFallback {
			if t.Avatar.Shape == theme.Circle {
				cx, cy, radius := float64(w)/2, float64(h)/2, math.Min(float64(w), float64(h))/2
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						if math.Pow(float64(x)+0.5-cx, 2)+math.Pow(float64(y)+0.5-cy, 2) <= radius*radius {
							dst.SetNRGBA(int(box.X)+x, int(box.Y)+y, t.Avatar.Fallback)
						}
					}
				}
			} else {
				rect := image.Rect(int(box.X), int(box.Y), int(box.X)+w, int(box.Y)+h)
				stddraw.Draw(dst, rect, &image.Uniform{C: t.Avatar.Fallback}, image.Point{}, stddraw.Src)
			}
		}
		return nil
	}

	avatar := resize(src, w, h, t.Avatar.Fit)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if t.Avatar.Shape == theme.Circle {
				cx, cy, radius := float64(w)/2, float64(h)/2, math.Min(float64(w), float64(h))/2
				if math.Pow(float64(x)+0.5-cx, 2)+math.Pow(float64(y)+0.5-cy, 2) > radius*radius {
					continue
				}
			}
			c := color.NRGBAModel.Convert(avatar.At(x, y)).(color.NRGBA)
			if t.Avatar.Grayscale {
				gray := uint8((299*uint32(c.R) + 587*uint32(c.G) + 114*uint32(c.B) + 500) / 1000)
				c.R, c.G, c.B = gray, gray, gray
			}
			opacity := avatarOpacity(float64(x)+box.X, float64(y)+box.Y, t)
			blend(dst, int(box.X)+x, int(box.Y)+y, c, opacity)
		}
	}
	return nil
}

func resize(src image.Image, width, height int, fit theme.Fit) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return dst
	}

	if fit == theme.Contain {
		scale := math.Min(float64(width)/float64(sw), float64(height)/float64(sh))
		dw, dh := max(1, int(math.Round(float64(sw)*scale))), max(1, int(math.Round(float64(sh)*scale)))
		x, y := (width-dw)/2, (height-dh)/2
		xdraw.CatmullRom.Scale(dst, image.Rect(x, y, x+dw, y+dh), src, b, stddraw.Over, nil)
		return dst
	}

	sourceAspect := float64(sw) / float64(sh)
	targetAspect := float64(width) / float64(height)
	crop := b
	if sourceAspect > targetAspect {
		cropWidth := int(math.Round(float64(sh) * targetAspect))
		left := b.Min.X + (sw-cropWidth)/2
		crop = image.Rect(left, b.Min.Y, left+cropWidth, b.Max.Y)
	} else if sourceAspect < targetAspect {
		cropHeight := int(math.Round(float64(sw) / targetAspect))
		top := b.Min.Y + (sh-cropHeight)/2
		crop = image.Rect(b.Min.X, top, b.Max.X, top+cropHeight)
	}
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, stddraw.Src, nil)
	return dst
}

func avatarOpacity(x, y float64, t theme.Theme) float64 {
	if !t.Gradient.Enabled || t.Background.A == 0 {
		return 1
	}
	position := x
	start := float64(t.Width) * t.Gradient.StartRatio
	end := float64(t.Width) * t.Gradient.EndRatio
	if t.Gradient.Vertical {
		position = y
		start = float64(t.Height) * t.Gradient.StartRatio
		end = float64(t.Height) * t.Gradient.EndRatio
	} else if t.Layout == theme.Side && t.Avatar.Position == theme.Right {
		start = float64(t.Width) * (1 - t.Gradient.StartRatio)
		end = float64(t.Width) * (1 - t.Gradient.EndRatio)
	}
	progress := 1.0
	if end != start {
		progress = (position - start) / (end - start)
	}
	progress = math.Max(0, math.Min(1, progress))
	return 1 - interpolateAlpha(t.Gradient.Stops, progress)
}

func interpolateAlpha(stops []theme.GradientStop, progress float64) float64 {
	if len(stops) == 0 {
		return progress
	}
	if progress <= stops[0].Offset {
		return stops[0].Alpha
	}
	for i := 1; i < len(stops); i++ {
		if progress <= stops[i].Offset {
			span := stops[i].Offset - stops[i-1].Offset
			if span <= 0 {
				return stops[i].Alpha
			}
			t := (progress - stops[i-1].Offset) / span
			return stops[i-1].Alpha + (stops[i].Alpha-stops[i-1].Alpha)*t
		}
	}
	return stops[len(stops)-1].Alpha
}

func blend(dst *image.NRGBA, x, y int, src color.NRGBA, opacity float64) {
	if !image.Pt(x, y).In(dst.Bounds()) || opacity <= 0 || src.A == 0 {
		return
	}
	d := dst.NRGBAAt(x, y)
	sa := float64(src.A) / 255 * opacity
	da := float64(d.A) / 255
	outA := sa + da*(1-sa)
	if outA <= 0 {
		dst.SetNRGBA(x, y, color.NRGBA{})
		return
	}
	blendChannel := func(s, old uint8) uint8 {
		value := (float64(s)*sa + float64(old)*da*(1-sa)) / outA
		return uint8(math.Round(math.Max(0, math.Min(255, value))))
	}
	dst.SetNRGBA(x, y, color.NRGBA{
		R: blendChannel(src.R, d.R), G: blendChannel(src.G, d.G), B: blendChannel(src.B, d.B),
		A: uint8(math.Round(outA * 255)),
	})
}

func drawText(dst *image.NRGBA, q Quote, boxes layout.Layout, t theme.Theme, fonts *FontRegistry) error {
	family := fonts.resolve(t.Text.Font)
	segments := append([]internalemoji.Segment(nil), q.Segments...)
	if t.QuoteMark.Display == theme.QuoteInline {
		segments = append([]internalemoji.Segment{{Text: t.QuoteMark.Open}}, segments...)
		segments = append(segments, internalemoji.Segment{Text: t.QuoteMark.Close})
	}
	fontSize, lines, face := fitQuote(family, segments, boxes.Text, t)
	if len(lines) == 0 {
		return fmt.Errorf("quote has no renderable lines")
	}
	lineHeight := fontSize * t.Text.LineHeight
	displaySize := theme.Pixels(t.DisplayName.Size, t.Height)
	usernameSize := theme.Pixels(t.Username.Size, t.Height)
	labelsHeight := 0.0
	if q.DisplayName != "" {
		labelsHeight += displaySize * 1.25
	}
	if q.Username != "" {
		labelsHeight += usernameSize * 1.25
	}
	gap := 0.03 * float64(t.Height)
	if labelsHeight == 0 {
		gap = 0
	}
	quoteMarkHeight := 0.0
	if t.QuoteMark.Display == theme.QuoteBlock {
		quoteMarkHeight = theme.Pixels(t.QuoteMark.Size, t.Height)*0.75 + theme.Pixels(t.QuoteMark.Gap, t.Height)
	}
	dividerHeight := 0.0
	if t.Divider.Enabled {
		dividerHeight = 2*theme.Pixels(t.Divider.Gap, t.Height) + math.Max(1, theme.Pixels(t.Divider.Thickness, t.Height))
	}
	blockHeight := quoteMarkHeight + float64(len(lines))*lineHeight + dividerHeight + gap + labelsHeight
	top := boxes.Text.Y + math.Max(0, (boxes.Text.Height-blockHeight)/2)

	c := canvas.New(float64(t.Width), float64(t.Height))
	ctx := canvas.NewContext(c)
	ctx.SetCoordSystem(canvas.CartesianIV)
	if q.AvatarMissing && t.Avatar.HasFallback {
		name := strings.TrimSpace(q.DisplayName)
		if name == "" {
			name = strings.TrimSpace(q.Username)
		}
		initial := ""
		for _, r := range name {
			initial = strings.ToUpper(string(r))
			break
		}
		if initial != "" {
			size := math.Min(boxes.Avatar.Width, boxes.Avatar.Height) * 0.25
			fallbackFace := family.Face(size*pointsPerPixel, t.Avatar.FallbackText, canvas.FontRegular)
			ctx.DrawText(boxes.Avatar.X+boxes.Avatar.Width/2, boxes.Avatar.Y+boxes.Avatar.Height/2+fallbackFace.Metrics().Ascent/3, canvas.NewTextLine(fallbackFace, initial, canvas.Center))
		}
	}

	if t.QuoteMark.Display == theme.QuoteBlock {
		size := theme.Pixels(t.QuoteMark.Size, t.Height)
		markColor := t.QuoteMark.Color
		if t.QuoteMark.InheritColor {
			markColor = t.Text.Color
		}
		markFace := family.Face(size*pointsPerPixel, markColor, fontStyle(t.QuoteMark.Weight))
		ctx.DrawText(boxes.CenterX, top+size*0.75, canvas.NewTextLine(markFace, t.QuoteMark.Open+t.QuoteMark.Close, canvas.Center))
		top += quoteMarkHeight
	}
	baseline := top + face.Metrics().Ascent
	scaledEmoji := make(map[string]image.Image)
	for _, line := range lines {
		lineWidth := internalemoji.MeasureSegments(line, emojiAdvanceFor(fontSize, t.Emoji.SideMarginRatio), face.TextWidth)
		lineX := alignedSegmentX(t.Text.Align, boxes.Text, lineWidth)
		drawSegmentLine(ctx, lineX, baseline, line, q.EmojiImages, scaledEmoji, face, fontSize, t.Emoji.SideMarginRatio, t.Emoji.TopMarginRatio)
		baseline += lineHeight
	}

	labelY := top + float64(len(lines))*lineHeight
	if t.Divider.Enabled {
		dividerGap := theme.Pixels(t.Divider.Gap, t.Height)
		thickness := math.Max(1, theme.Pixels(t.Divider.Thickness, t.Height))
		width := boxes.Text.Width * t.Divider.WidthRatio
		dividerColor := t.Divider.Color
		if t.Divider.InheritColor {
			dividerColor = t.DisplayName.Color
		}
		rect := image.Rect(int(math.Round(boxes.CenterX-width/2)), int(math.Round(labelY+dividerGap)), int(math.Round(boxes.CenterX+width/2)), int(math.Round(labelY+dividerGap+thickness)))
		stddraw.Draw(dst, rect, &image.Uniform{C: dividerColor}, image.Point{}, stddraw.Over)
		labelY += dividerHeight
	}
	labelY += gap
	if q.DisplayName != "" {
		labelFace := family.Face(displaySize*pointsPerPixel, t.DisplayName.Color, fontStyle(t.DisplayName.Weight))
		ctx.DrawText(boxes.CenterX, labelY+labelFace.Metrics().Ascent,
			canvas.NewTextLine(labelFace, t.DisplayName.Prefix+q.DisplayName, canvas.Center))
		labelY += displaySize * 1.25
	}
	if q.Username != "" {
		labelFace := family.Face(usernameSize*pointsPerPixel, t.Username.Color, fontStyle(t.Username.Weight))
		ctx.DrawText(boxes.CenterX, labelY+labelFace.Metrics().Ascent,
			canvas.NewTextLine(labelFace, t.Username.Prefix+strings.TrimPrefix(q.Username, "@"), canvas.Center))
	}
	if q.Watermark != "" {
		size := theme.Pixels(t.Watermark.Size, t.Height)
		watermarkFace := family.Face(size*pointsPerPixel, t.Watermark.Color, fontStyle(t.Watermark.Weight))
		x, align := float64(t.Width)-0.025*float64(t.Width), canvas.Right
		switch t.Watermark.Position {
		case theme.WatermarkBottomLeft:
			x, align = 0.025*float64(t.Width), canvas.Left
		case theme.WatermarkBottomCenter:
			x, align = float64(t.Width)/2, canvas.Center
		case theme.WatermarkAuto:
			if t.Layout == theme.Side && t.Avatar.Position == theme.Right {
				x, align = 0.025*float64(t.Width), canvas.Left
			}
		}
		ctx.DrawText(x, float64(t.Height)-0.025*float64(t.Height), canvas.NewTextLine(watermarkFace, q.Watermark, align))
	}

	// canvas' rasterizer currently advertises draw.Image but its scanner only
	// handles premultiplied RGBA safely. Keep that backend detail inside this
	// package and convert back to the public NRGBA representation afterwards.
	rgba := image.NewRGBA(dst.Bounds())
	stddraw.Draw(rgba, rgba.Bounds(), dst, dst.Bounds().Min, stddraw.Src)
	ras := rasterizer.FromImage(rgba, canvas.DPMM(1), canvas.LinearColorSpace{})
	c.RenderTo(ras)
	ras.Close()
	stddraw.Draw(dst, dst.Bounds(), rgba, rgba.Bounds().Min, stddraw.Src)
	return nil
}

func fitQuote(family *canvas.FontFamily, segments []internalemoji.Segment, box layout.Rect, t theme.Theme) (float64, [][]internalemoji.Segment, *canvas.FontFace) {
	maxSize := theme.Pixels(t.Text.Size, t.Height)
	minSize := theme.Pixels(t.Text.MinSize, t.Height)
	for size := maxSize; size >= minSize; size -= 1 {
		face := family.Face(size*pointsPerPixel, t.Text.Color, fontStyle(t.Text.Weight))
		lines := internalemoji.WrapSegmentsWithOptions(segments, box.Width, emojiAdvanceFor(size, t.Emoji.SideMarginRatio), face.TextWidth, internalemoji.WrapOptions{PhraseBreak: t.Text.PhraseBreak, Locale: t.Text.Locale})
		maxLines := max(1, int(math.Floor(box.Height/(size*t.Text.LineHeight))))
		if len(lines) <= maxLines {
			return size, lines, face
		}
	}

	face := family.Face(minSize*pointsPerPixel, t.Text.Color, fontStyle(t.Text.Weight))
	lines := internalemoji.WrapSegmentsWithOptions(segments, box.Width, emojiAdvanceFor(minSize, t.Emoji.SideMarginRatio), face.TextWidth, internalemoji.WrapOptions{PhraseBreak: t.Text.PhraseBreak, Locale: t.Text.Locale})
	maxLines := max(1, int(math.Floor(box.Height/(minSize*t.Text.LineHeight))))
	if len(lines) > maxLines {
		if t.Text.Overflow == theme.OverflowError {
			return minSize, nil, face
		}
		if t.Text.Overflow == theme.Shrink {
			return minSize, lines, face
		}
		lines = lines[:maxLines]
		lines[maxLines-1] = internalemoji.EllipsizeLine(lines[maxLines-1], box.Width, emojiAdvanceFor(minSize, t.Emoji.SideMarginRatio), face.TextWidth)
	}
	return minSize, lines, face
}

func emojiAdvance(fontSize float64) float64 { return fontSize * 1.16 }
func fontStyle(weight int) canvas.FontStyle {
	if weight >= 600 {
		return canvas.FontBold
	}
	return canvas.FontRegular
}
func emojiAdvanceFor(fontSize, sideMarginRatio float64) float64 {
	return fontSize * (1 + 2*sideMarginRatio)
}

func alignedSegmentX(value theme.Align, box layout.Rect, width float64) float64 {
	switch value {
	case theme.AlignLeft:
		return box.X
	case theme.AlignRight:
		return box.X + box.Width - width
	default:
		return box.X + (box.Width-width)/2
	}
}

func drawSegmentLine(
	ctx *canvas.Context,
	x, baseline float64,
	line []internalemoji.Segment,
	images map[string]image.Image,
	scaled map[string]image.Image,
	face *canvas.FontFace,
	fontSize float64,
	sideMarginRatio, topMarginRatio float64,
) {
	cursor := x
	margin := fontSize * sideMarginRatio
	for _, segment := range line {
		if !segment.IsEmoji() {
			ctx.DrawText(cursor, baseline, canvas.NewTextLine(face, segment.Text, canvas.Left))
			cursor += face.TextWidth(segment.Text)
			continue
		}
		img := images[segment.URL]
		if img == nil {
			continue
		}
		resized := scaled[segment.URL]
		if resized == nil || resized.Bounds().Dx() != int(math.Round(fontSize)) {
			size := max(1, int(math.Round(fontSize)))
			resized = resize(img, size, size, theme.Contain)
			scaled[segment.URL] = resized
		}
		ctx.DrawImage(cursor+margin, baseline-fontSize+fontSize*topMarginRatio, resized, canvas.DPMM(1))
		cursor += emojiAdvanceFor(fontSize, sideMarginRatio)
	}
}

func textAlign(value theme.Align, box layout.Rect) (canvas.TextAlign, float64) {
	switch value {
	case theme.AlignLeft:
		return canvas.Left, box.X
	case theme.AlignRight:
		return canvas.Right, box.X + box.Width
	default:
		return canvas.Center, box.X + box.Width/2
	}
}
