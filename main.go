package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"realMe/bot"
	"realMe/config"
	"realMe/llm"
)

func main() {
	cfg := config.LoadConfig()

	glmClient := llm.NewGLMClient(cfg)
	b := bot.NewBot(cfg, glmClient)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	log.Println("Starting bot...")
	if err := b.Run(ctx); err != nil {
		log.Fatalf("Bot stopped with error: %v", err)
	}
}
