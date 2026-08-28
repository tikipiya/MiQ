package main

import (
	"context"
	"image/color"
	"log"
	"os"

	miq "github.com/tikipiya/MiQ"
	"github.com/tikipiya/MiQ/theme"
)

func main() {
	resolved, err := theme.Resolve(theme.Preset(theme.Portrait))
	if err != nil {
		log.Fatal(err)
	}
	resolved.Avatar.Shape = theme.Circle
	resolved.BackgroundGradient = &theme.BackgroundGradient{Type: theme.LinearGradient, Direction: theme.GradientDiagonal, Stops: []theme.ColorStop{{Color: color.NRGBA{R: 0xff, G: 0x7e, B: 0x5f, A: 0xff}, Offset: 0}, {Color: color.NRGBA{R: 0x6a, G: 0x30, B: 0x93, A: 0xff}, Offset: 1}}}
	engine, err := miq.NewEngine(miq.EngineOptions{})
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.Create("themed-quote.png")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	if err := engine.WriteQuote(context.Background(), file, miq.Quote{Text: "Goでテーマを組み立てる", DisplayName: "makeitaquote"}, miq.RenderOptions{Theme: theme.Exact(resolved)}, miq.EncodeOptions{Format: miq.PNG}); err != nil {
		log.Fatal(err)
	}
}
