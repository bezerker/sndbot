package main

import (
	"log"

	bot "github.com/bezerker/sndbot/bot"
	config "github.com/bezerker/sndbot/config"
)

func main() {
	cfg, err := config.LoadConfig() // call the LoadConfig function of config.go
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	bot.RunBot(cfg) // Run the bot passing in required arguments
}
