package main

import (
	"context"
	"log"
	"os"

	miq "github.com/tikipiya/MiQ"
	"github.com/tikipiya/MiQ/theme"
)

func main() {
	engine, err := miq.NewEngine(miq.EngineOptions{})
	if err != nil {
		log.Fatal(err)
	}
	out, err := os.Create("quote.png")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	err = engine.WriteQuote(context.Background(), out, miq.Quote{
		Text:        "小さな工夫が、毎日の使いやすさをつくる。",
		Username:    "sample_user",
		DisplayName: "Sample User",
	}, miq.RenderOptions{Theme: theme.Preset(theme.Dark)}, miq.EncodeOptions{Format: miq.PNG})
	if err != nil {
		log.Fatal(err)
	}
}
