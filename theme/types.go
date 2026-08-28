package theme

import "image/color"

type Name string

const (
	Dark          Name = "dark"
	Light         Name = "light"
	Color         Name = "color"
	Portrait      Name = "portrait"
	PortraitLight Name = "portrait-light"
	Custom        Name = "custom"
)

type LayoutMode string

const (
	Side    LayoutMode = "side"
	Stacked LayoutMode = "stacked"
)

type Position string

const (
	Left  Position = "left"
	Right Position = "right"
)

type Fit string

const (
	Cover   Fit = "cover"
	Contain Fit = "contain"
)

type Shape string

const (
	Rectangle Shape = "rectangle"
	Circle    Shape = "circle"
)

type Align string

const (
	AlignLeft   Align = "left"
	AlignCenter Align = "center"
	AlignRight  Align = "right"
)

type Overflow string

const (
	Ellipsis      Overflow = "ellipsis"
	Shrink        Overflow = "shrink"
	OverflowError Overflow = "error"
)

type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type Avatar struct {
	Grayscale    bool
	Position     Position
	WidthRatio   float64
	Fit          Fit
	Fallback     color.NRGBA
	FallbackText color.NRGBA
	HasFallback  bool
	Shape        Shape
}

type GradientStop struct {
	Offset float64
	Alpha  float64
}

type Gradient struct {
	Enabled    bool
	Vertical   bool
	StartRatio float64
	EndRatio   float64
	Stops      []GradientStop
}

type Text struct {
	Color       color.NRGBA
	Font        string
	Size        float64
	MinSize     float64
	LineHeight  float64
	Align       Align
	Overflow    Overflow
	Area        Rect
	PhraseBreak bool
	Locale      string
	Weight      int
	AutoArea    bool
}

type QuoteMarkDisplay string

const (
	QuoteInline QuoteMarkDisplay = "inline"
	QuoteBlock  QuoteMarkDisplay = "block"
	QuoteNone   QuoteMarkDisplay = "none"
)

type QuoteMark struct {
	Display      QuoteMarkDisplay
	Open, Close  string
	Size         float64
	Color        color.NRGBA
	InheritColor bool
	Gap          float64
	Weight       int
}
type Divider struct {
	Enabled                    bool
	WidthRatio, Thickness, Gap float64
	Color                      color.NRGBA
	InheritColor               bool
}
type WatermarkPosition string

const (
	WatermarkAuto         WatermarkPosition = "auto"
	WatermarkBottomRight  WatermarkPosition = "bottom-right"
	WatermarkBottomLeft   WatermarkPosition = "bottom-left"
	WatermarkBottomCenter WatermarkPosition = "bottom-center"
)

type Emoji struct {
	SideMarginRatio, TopMarginRatio float64
	Size                            int
}
type BackgroundGradientType string

const (
	LinearGradient BackgroundGradientType = "linear"
	RadialGradient BackgroundGradientType = "radial"
)

type BackgroundGradientDirection string

const (
	GradientHorizontal      BackgroundGradientDirection = "horizontal"
	GradientVertical        BackgroundGradientDirection = "vertical"
	GradientDiagonal        BackgroundGradientDirection = "diagonal"
	GradientDiagonalReverse BackgroundGradientDirection = "diagonal-reverse"
)

type ColorStop struct {
	Color  color.NRGBA
	Offset float64
}
type BackgroundGradient struct {
	Type      BackgroundGradientType
	Direction BackgroundGradientDirection
	Stops     []ColorStop
}

type Label struct {
	Color    color.NRGBA
	Font     string
	Size     float64
	Prefix   string
	Position WatermarkPosition
	Weight   int
}

type Theme struct {
	Name               Name
	Layout             LayoutMode
	Width              int
	Height             int
	Background         color.NRGBA
	BackgroundGradient *BackgroundGradient
	Avatar             Avatar
	Gradient           Gradient
	Text               Text
	QuoteMark          QuoteMark
	Divider            Divider
	Emoji              Emoji
	DisplayName        Label
	Username           Label
	Watermark          Label
}

// Input overlays a preset. Pointer fields preserve the difference between an
// omitted value and an intentional zero.
type Input struct {
	Base               *Theme
	Extends            Name
	Layout             *LayoutMode
	Width              *int
	Height             *int
	Background         *color.NRGBA
	BackgroundGradient *BackgroundGradient
	Avatar             *AvatarInput
	Gradient           *GradientInput
	Text               *TextInput
	QuoteMark          *QuoteMarkInput
	Divider            *DividerInput
	Emoji              *EmojiInput
	DisplayName        *LabelInput
	Username           *LabelInput
	Watermark          *LabelInput
}

type AvatarInput struct {
	Grayscale    *bool
	Position     *Position
	WidthRatio   *float64
	Fit          *Fit
	Shape        *Shape
	Fallback     *color.NRGBA
	FallbackText *color.NRGBA
	HasFallback  *bool
}

type TextInput struct {
	Color       *color.NRGBA
	Font        *string
	Size        *float64
	MinSize     *float64
	LineHeight  *float64
	Align       *Align
	Overflow    *Overflow
	Area        *Rect
	PhraseBreak *bool
	Locale      *string
	Weight      *int
	AutoArea    *bool
}

type GradientInput struct {
	Enabled              *bool
	Vertical             *bool
	StartRatio, EndRatio *float64
	Stops                *[]GradientStop
}
type QuoteMarkInput struct {
	Display      *QuoteMarkDisplay
	Open, Close  *string
	Size         *float64
	Color        *color.NRGBA
	InheritColor *bool
	Gap          *float64
	Weight       *int
}
type DividerInput struct {
	Enabled                    *bool
	WidthRatio, Thickness, Gap *float64
	Color                      *color.NRGBA
	InheritColor               *bool
}
type EmojiInput struct {
	SideMarginRatio, TopMarginRatio *float64
	Size                            *int
}
type LabelInput struct {
	Color    *color.NRGBA
	Font     *string
	Size     *float64
	Prefix   *string
	Position *WatermarkPosition
	Weight   *int
}

func Preset(name Name) Input  { return Input{Extends: name} }
func Exact(value Theme) Input { return Input{Base: &value} }
