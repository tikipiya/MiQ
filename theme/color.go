package theme

import (
	"fmt"
	"image/color"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/mazznoer/csscolorparser"
)

var srgbFunction = regexp.MustCompile(`(?i)^color\(\s*srgb\s+([^)]*)\)$`)
var labFunction = regexp.MustCompile(`(?i)^(lab|lch)\(\s*([^)]*)\)$`)

// ParseColor accepts CSS Color Level 4 strings, 0xRRGGBB/0xRRGGBBAA integer
// values, and []number values containing RGB plus an optional alpha channel.
func ParseColor(input any, field ...string) (color.NRGBA, error) {
	name := "color"
	if len(field) > 0 && strings.TrimSpace(field[0]) != "" {
		name = field[0]
	}
	switch value := input.(type) {
	case string:
		return parseColorString(value, name)
	case int:
		if value < 0 {
			return color.NRGBA{}, colorInputError(name)
		}
		return parseColorNumber(uint64(value), name)
	case int64:
		if value < 0 {
			return color.NRGBA{}, colorInputError(name)
		}
		return parseColorNumber(uint64(value), name)
	case uint:
		return parseColorNumber(uint64(value), name)
	case uint32:
		return parseColorNumber(uint64(value), name)
	case uint64:
		return parseColorNumber(value, name)
	case []int:
		channels := make([]float64, len(value))
		for index := range value {
			channels[index] = float64(value[index])
		}
		return parseColorChannels(channels, name)
	case []float64:
		return parseColorChannels(value, name)
	default:
		return color.NRGBA{}, colorInputError(name)
	}
}

func parseColorNumber(value uint64, field string) (color.NRGBA, error) {
	if value > 0xffffffff {
		return color.NRGBA{}, colorInputError(field)
	}
	if value > 0xffffff {
		return color.NRGBA{R: byte(value >> 24), G: byte(value >> 16), B: byte(value >> 8), A: byte(value)}, nil
	}
	return color.NRGBA{R: byte(value >> 16), G: byte(value >> 8), B: byte(value), A: 0xff}, nil
}

func parseColorChannels(value []float64, field string) (color.NRGBA, error) {
	if len(value) < 3 || len(value) > 4 {
		return color.NRGBA{}, colorInputError(field)
	}
	for _, channel := range value {
		if math.IsNaN(channel) || math.IsInf(channel, 0) {
			return color.NRGBA{}, colorInputError(field)
		}
	}
	alpha := 255.0
	if len(value) == 4 {
		alpha = value[3]
		if alpha <= 1 {
			alpha *= 255
		}
	}
	return color.NRGBA{
		R: roundedByte(value[0]), G: roundedByte(value[1]), B: roundedByte(value[2]), A: roundedByte(alpha),
	}, nil
}

func parseColorString(input, field string) (color.NRGBA, error) {
	value := strings.TrimSpace(input)
	if strings.HasPrefix(strings.ToLower(value), "0x") {
		digits := value[2:]
		if len(digits) != 6 && len(digits) != 8 {
			return color.NRGBA{}, colorParseError(field, input)
		}
		number, err := strconv.ParseUint(digits, 16, 32)
		if err != nil {
			return color.NRGBA{}, colorParseError(field, input)
		}
		if len(digits) == 8 {
			return color.NRGBA{R: byte(number >> 24), G: byte(number >> 16), B: byte(number >> 8), A: byte(number)}, nil
		}
		return color.NRGBA{R: byte(number >> 16), G: byte(number >> 8), B: byte(number), A: 0xff}, nil
	}
	if match := srgbFunction.FindStringSubmatch(value); match != nil {
		return parseSRGBFunction(match[1], field, input)
	}
	if match := labFunction.FindStringSubmatch(value); match != nil {
		return parseLabFunction(strings.ToLower(match[1]), match[2], field, input)
	}
	parsed, err := csscolorparser.Parse(strings.ToLower(value))
	if err != nil {
		return color.NRGBA{}, colorParseError(field, input)
	}
	r, g, b, a := parsed.RGBA255()
	return color.NRGBA{R: r, G: g, B: b, A: a}, nil
}

func parseLabFunction(kind, body, field, original string) (color.NRGBA, error) {
	parts := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(body, ",", " "), "/", " / "))
	if len(parts) != 3 && len(parts) != 5 {
		return color.NRGBA{}, colorParseError(field, original)
	}
	if len(parts) == 5 && parts[3] != "/" {
		return color.NRGBA{}, colorParseError(field, original)
	}
	read := func(value string, percentScale float64) (float64, error) {
		if strings.HasSuffix(value, "%") {
			parsed, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
			return parsed * percentScale / 100, err
		}
		return strconv.ParseFloat(value, 64)
	}
	l, err := read(parts[0], 100)
	if err != nil {
		return color.NRGBA{}, colorParseError(field, original)
	}
	second, err := read(parts[1], 125)
	if err != nil {
		return color.NRGBA{}, colorParseError(field, original)
	}
	third, err := read(parts[2], 125)
	if err != nil {
		return color.NRGBA{}, colorParseError(field, original)
	}
	a, b := second, third
	if kind == "lch" {
		radians := third * math.Pi / 180
		a, b = second*math.Cos(radians), second*math.Sin(radians)
	}
	alpha := 1.0
	if len(parts) == 5 {
		alpha, err = parseAlpha(parts[4])
		if err != nil {
			return color.NRGBA{}, colorParseError(field, original)
		}
	}
	r, g, blue := labToSRGB(l, a, b)
	return color.NRGBA{R: roundedByte(r * 255), G: roundedByte(g * 255), B: roundedByte(blue * 255), A: roundedByte(alpha * 255)}, nil
}

func labToSRGB(l, a, b float64) (float64, float64, float64) {
	const epsilon = 216.0 / 24389.0
	const kappa = 24389.0 / 27.0
	f1 := (l + 16) / 116
	f0 := a/500 + f1
	f2 := f1 - b/200
	finv := func(value float64) float64 {
		cube := value * value * value
		if cube > epsilon {
			return cube
		}
		return (116*value - 16) / kappa
	}
	// CIELAB is relative to D50.
	x50, y50, z50 := finv(f0)*0.96422, finv(f1), finv(f2)*0.82521
	// Bradford adaptation from D50 to D65, as used by CSS Color 4.
	x65 := x50*0.9555766 + y50*-0.0230393 + z50*0.0631636
	y65 := x50*-0.0282895 + y50*1.0099416 + z50*0.0210077
	z65 := x50*0.0122982 + y50*-0.0204830 + z50*1.3299098
	rLinear := x65*3.2406 + y65*-1.5372 + z65*-0.4986
	gLinear := x65*-0.9689 + y65*1.8758 + z65*0.0415
	bLinear := x65*0.0557 + y65*-0.2040 + z65*1.0570
	encode := func(value float64) float64 {
		if value <= 0.0031308 {
			return math.Max(0, math.Min(1, 12.92*value))
		}
		return math.Max(0, math.Min(1, 1.055*math.Pow(value, 1/2.4)-0.055))
	}
	return encode(rLinear), encode(gLinear), encode(bLinear)
}

func parseSRGBFunction(body, field, original string) (color.NRGBA, error) {
	parts := strings.Fields(strings.ReplaceAll(body, "/", " / "))
	if len(parts) != 3 && len(parts) != 5 {
		return color.NRGBA{}, colorParseError(field, original)
	}
	if len(parts) == 5 && parts[3] != "/" {
		return color.NRGBA{}, colorParseError(field, original)
	}
	channels := make([]float64, 4)
	channels[3] = 1
	for index := 0; index < 3; index++ {
		parsed, err := strconv.ParseFloat(parts[index], 64)
		if err != nil {
			return color.NRGBA{}, colorParseError(field, original)
		}
		channels[index] = parsed * 255
	}
	if len(parts) == 5 {
		alpha, err := parseAlpha(parts[4])
		if err != nil {
			return color.NRGBA{}, colorParseError(field, original)
		}
		channels[3] = alpha
	}
	return parseColorChannels(channels, field)
}

func parseAlpha(value string) (float64, error) {
	if strings.HasSuffix(value, "%") {
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
		return parsed / 100, err
	}
	return strconv.ParseFloat(value, 64)
}

func roundedByte(value float64) uint8 {
	return uint8(math.Round(math.Max(0, math.Min(255, value))))
}

func colorInputError(field string) error {
	return fmt.Errorf("%s must be a CSS color, an integer like 0xRRGGBBAA, or [r, g, b, a]", field)
}

func colorParseError(field, input string) error {
	return fmt.Errorf("%s: could not read %q as a color; use a CSS color, 0xRRGGBBAA, or [r, g, b, a]", field, input)
}

func ColorCSS(value color.NRGBA) string {
	if value.A == 0xff {
		return fmt.Sprintf("rgb(%d, %d, %d)", value.R, value.G, value.B)
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %s)", value.R, value.G, value.B, formatAlpha(float64(value.A)/255))
}

func WithAlpha(value color.NRGBA, alpha float64) string {
	combined := math.Max(0, math.Min(1, float64(value.A)/255*alpha))
	return fmt.Sprintf("rgba(%d, %d, %d, %s)", value.R, value.G, value.B, formatAlpha(combined))
}

func IsTransparent(value color.NRGBA) bool { return value.A == 0 }

func ColorHex(value color.NRGBA) string {
	return fmt.Sprintf("#%02X%02X%02X%02X", value.R, value.G, value.B, value.A)
}

func formatAlpha(value float64) string {
	rounded := math.Round(value*1000) / 1000
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}
