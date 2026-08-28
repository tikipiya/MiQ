package main

import (
	"context"
	"log"
	"os"

	miq "github.com/tikipiya/MiQ"
	discordadapter "github.com/tikipiya/MiQ/adapter/discord"
)

func main() {
	quote, err := discordadapter.FromMessage(discordadapter.Message{
		Content:  "**hello** <@1>!",
		Author:   discordadapter.User{Username: "cat", GlobalName: "Cat"},
		Mentions: discordadapter.Mentions{Users: map[string]discordadapter.User{"1": {Username: "dog"}}},
	}, discordadapter.Options{StripMarkdown: true})
	if err != nil {
		log.Fatal(err)
	}
	engine, err := miq.NewEngine(miq.EngineOptions{})
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.Create("discord-quote.png")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	if err := engine.WriteQuote(context.Background(), file, quote, miq.RenderOptions{}, miq.EncodeOptions{Format: miq.PNG}); err != nil {
		log.Fatal(err)
	}
}
