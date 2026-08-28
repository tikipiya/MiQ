package main

import (
	"context"
	"log"
	"os"

	miq "github.com/tikipiya/MiQ"
)

func main() {
	engine, err := miq.NewEngine(miq.EngineOptions{})
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.Create("conversation.png")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	messages := []miq.ConversationMessage{
		{Text: "こんにちは！", Username: "cat", DisplayName: "猫"},
		{Text: "同じ人の連続投稿はまとまります。", Username: "cat", DisplayName: "猫"},
		{Text: "絵文字も使えます 🐕", Username: "dog", DisplayName: "犬"},
	}
	if err := engine.WriteConversation(context.Background(), file, messages, miq.ConversationOptions{Theme: miq.ConversationDark, Width: 600}, miq.EncodeOptions{Format: miq.PNG}); err != nil {
		log.Fatal(err)
	}
}
