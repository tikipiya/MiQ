package main

import (
	"context"
	"log"
	"os"

	"github.com/tikipiya/MiQ/api/voids"
)

func main() {
	client, err := voids.NewClient(voids.Options{})
	if err != nil {
		log.Fatal(err)
	}
	png, err := client.Direct(context.Background(), voids.Quote{Text: "Voids APIから生成", Username: "cat", DisplayName: "猫"})
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("voids-quote.png", png, 0o644); err != nil {
		log.Fatal(err)
	}
}
