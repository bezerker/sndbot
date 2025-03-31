package main

import (
	"fmt"
	"log"

	"github.com/bezerker/sndbot/config"
	"github.com/bwmarrin/discordgo"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create Discord session
	dg, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		log.Fatalf("Failed to create Discord session: %v", err)
	}

	// Open connection
	err = dg.Open()
	if err != nil {
		log.Fatalf("Failed to open connection: %v", err)
	}
	defer dg.Close()

	// Get all commands
	commands, err := dg.ApplicationCommands(dg.State.User.ID, "")
	if err != nil {
		log.Fatalf("Failed to get commands: %v", err)
	}

	fmt.Printf("Found %d commands:\n", len(commands))
	for _, cmd := range commands {
		fmt.Printf("- %s (ID: %s)\n", cmd.Name, cmd.ID)
	}

	// Ask for confirmation
	fmt.Print("\nDo you want to delete all commands? (yes/no): ")
	var response string
	fmt.Scanln(&response)

	if response == "yes" {
		// Delete all commands
		for _, cmd := range commands {
			err := dg.ApplicationCommandDelete(dg.State.User.ID, "", cmd.ID)
			if err != nil {
				log.Printf("Failed to delete command %s: %v", cmd.Name, err)
			} else {
				fmt.Printf("Deleted command: %s\n", cmd.Name)
			}
		}
		fmt.Println("All commands have been deleted. Restart your bot to register them again.")
	} else {
		fmt.Println("Operation cancelled.")
	}
}
