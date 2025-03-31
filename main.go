package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bezerker/sndbot/bot"
	"github.com/bezerker/sndbot/config"
	"github.com/bezerker/sndbot/database"
	"github.com/bezerker/sndbot/server"
	"github.com/bezerker/sndbot/util"
)

func main() {
	// Initialize logger
	if err := util.InitLogger(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer util.CloseLogger()

	util.Logger.Print("Starting bot...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := database.InitDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize bot
	bot.Initialize(cfg)

	// Start HTTP server for OAuth callback
	go func() {
		if err := server.StartCallbackServer(8080); err != nil {
			log.Printf("Failed to start callback server: %v", err)
		}
	}()

	// Run the bot
	bot.RunBot(cfg)

	// Wait for interrupt signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	<-sc

	// Cleanup
	util.Logger.Print("Shutting down...")
}
