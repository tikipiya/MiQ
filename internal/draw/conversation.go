package draw

import (
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"math"
	"strings"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
	internalemoji "github.com/tikipiya/MiQ/internal/emoji"
)

type ConversationMessage struct {
	Segments              []internalemoji.Segment
	Images                map[string]image.Image
	Username, DisplayName string
	Avatar                image.Image
}

type ConversationStyle struct {
	Width                                                    int
	Background, Text, Name, FallbackBackground, FallbackText color.NRGBA
}

const conversationFont = "M PLUS Rounded 1c, Noto Sans JP, Yu Gothic, Meiryo, sans-serif"

func RenderConversation(messages []ConversationMessage, style ConversationStyle, fonts *FontRegistry) (*image.NRGBA, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("conversation has no messages")
	}
	if style.Width <= 0 {
		return nil, fmt.Errorf("invalid conversation width")
	}
	const padding, avatarSize, avatarGap = 20.0, 40.0, 12.0
	const nameSize, textSize, lineHeight = 15.0, 16.0, 21.6
	contentX := padding + avatarSize + avatarGap
	columnWidth := math.Max(1, float64(style.Width)-contentX-padding)
	family := fonts.resolve(conversationFont)
	textFace := family.Face(textSize*pointsPerPixel, style.Text, canvas.FontRegular)
	nameFace := family.Face(nameSize*pointsPerPixel, style.Name, canvas.FontRegular)

	type laidOut struct {
		message           ConversationMessage
		lines             [][]internalemoji.Segment
		top, textBaseline float64
		grouped           bool
	}
	rows := make([]laidOut, 0, len(messages))
	cursor, previous := padding, ""
	for index, message := range messages {
		grouped := index > 0 && message.Username == previous
		gap := 16.0
		if grouped {
			gap = 4
		}
		if index == 0 {
			gap = 0
		}
		top := cursor + gap
		lines := internalemoji.WrapSegments(message.Segments, columnWidth, emojiAdvance(textSize), textFace.TextWidth)
		if len(lines) == 0 {
			lines = [][]internalemoji.Segment{{}}
		}
		// Keep these baselines numeric, matching Canvas 2D's alphabetic
		// coordinates in the reference renderer. Font ascent is useful for
		// glyph painting, but using it for layout changes the gallery height.
		baseline := top + textSize
		if !grouped {
			baseline = top + nameSize + 6 + textSize
		}
		bottom := baseline + float64(len(lines)-1)*lineHeight + textSize*0.3
		if !grouped {
			bottom = math.Max(bottom, top+avatarSize)
		}
		rows = append(rows, laidOut{message: message, lines: lines, top: top, textBaseline: baseline, grouped: grouped})
		cursor, previous = bottom, message.Username
	}
	height := max(1, int(math.Round(cursor+padding)))
	out := image.NewNRGBA(image.Rect(0, 0, style.Width, height))
	fill(out, style.Background)
	for _, row := range rows {
		if !row.grouped {
			drawConversationAvatar(out, row.message.Avatar, int(padding), int(row.top), int(avatarSize), style.FallbackBackground)
		}
	}

	c := canvas.New(float64(style.Width), float64(height))
	ctx := canvas.NewContext(c)
	ctx.SetCoordSystem(canvas.CartesianIV)
	scaled := make(map[string]image.Image)
	for _, row := range rows {
		if !row.grouped {
			if row.message.Avatar == nil {
				fallbackFace := family.Face(18*pointsPerPixel, style.FallbackText, canvas.FontRegular)
				ctx.DrawText(padding+avatarSize/2, row.top+avatarSize/2+fallbackFace.Metrics().Ascent/3, canvas.NewTextLine(fallbackFace, initial(row.message), canvas.Center))
			}
			name := row.message.DisplayName
			if name == "" {
				name = row.message.Username
			}
			ctx.DrawText(contentX, row.top+nameSize, canvas.NewTextLine(nameFace, name, canvas.Left))
		}
		baseline := row.textBaseline
		for _, line := range row.lines {
			drawSegmentLine(ctx, contentX, baseline, line, row.message.Images, scaled, textFace, textSize, 0.08, 0.1)
			baseline += lineHeight
		}
	}
	rgba := image.NewRGBA(out.Bounds())
	stddraw.Draw(rgba, rgba.Bounds(), out, image.Point{}, stddraw.Src)
	ras := rasterizer.FromImage(rgba, canvas.DPMM(1), canvas.LinearColorSpace{})
	c.RenderTo(ras)
	ras.Close()
	stddraw.Draw(out, out.Bounds(), rgba, image.Point{}, stddraw.Src)
	return out, nil
}

func initial(message ConversationMessage) string {
	name := strings.TrimSpace(message.DisplayName)
	if name == "" {
		name = strings.TrimSpace(message.Username)
	}
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return ""
}

func drawConversationAvatar(dst *image.NRGBA, src image.Image, x, y, size int, fallback color.NRGBA) {
	var avatar image.Image = &image.Uniform{C: fallback}
	if src != nil {
		avatar = resize(src, size, size, "cover")
	}
	cx, cy, radius := float64(x)+float64(size)/2, float64(y)+float64(size)/2, float64(size)/2
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			dx, dy := float64(x+px)+0.5-cx, float64(y+py)+0.5-cy
			if dx*dx+dy*dy <= radius*radius {
				dst.Set(x+px, y+py, avatar.At(avatar.Bounds().Min.X+px, avatar.Bounds().Min.Y+py))
			}
		}
	}
}
