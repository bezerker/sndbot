package bot

import (
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"

	config "github.com/bezerker/sndbot/config"
	database "github.com/bezerker/sndbot/database"
	util "github.com/bezerker/sndbot/util"
	"github.com/bwmarrin/discordgo"
)

var db *sql.DB

func RunBot(config config.Config) {
	var err error
	// Initialize database
	db, err = database.InitDB("characters.db")
	if err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		return
	}
	defer db.Close()

	BotToken := config.DiscordToken
	// create a session
	discord, err := discordgo.New("Bot " + BotToken)
	util.CheckNilErr(err)

	// add a event handler
	discord.AddHandler(newMessage)

	// open session
	discord.Open()
	defer discord.Close() // close session, after function termination

	// keep bot running until there is NO os interruption (ctrl + C)
	fmt.Println("Bot running....")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func newMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {
	if message.Author.ID == discord.State.User.ID {
		return
	}

	// Split the message content into words
	args := strings.Fields(message.Content)

	// respond to user message if it contains commands
	switch {
	case strings.HasPrefix(message.Content, "!register"):
		if len(args) != 3 {
			discord.ChannelMessageSend(message.ChannelID, "Usage: !register <character_name> <server>")
			return
		}

		registration := database.CharacterRegistration{
			DiscordUsername: message.Author.Username,
			CharacterName:   args[1],
			Server:          args[2],
		}

		err := database.RegisterCharacter(db, registration)
		if err != nil {
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Failed to register character: %v", err))
			return
		}

		discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Successfully registered character %s on server %s for %s", args[1], args[2], message.Author.Username))

	case strings.HasPrefix(message.Content, "!whoami"):
		registration, err := database.GetCharacter(db, message.Author.Username)
		if err != nil {
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Error looking up registration: %v", err))
			return
		}
		if registration == nil {
			discord.ChannelMessageSend(message.ChannelID, "You haven't registered a character yet. Use !register <character_name> <server> to register.")
			return
		}
		discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Your registered character is %s on server %s", registration.CharacterName, registration.Server))

	case strings.Contains(message.Content, "!help"):
		helpMessage := `Available commands:
!help - Show this help message
!register <character_name> <server> - Register your character (or update existing registration)
!whoami - Show your current character registration
!bye - Say goodbye
!ping - Ping the bot`
		discord.ChannelMessageSend(message.ChannelID, helpMessage)

	case strings.Contains(message.Content, "!bye"):
		discord.ChannelMessageSend(message.ChannelID, "Good Bye👋")

	case strings.Contains(message.Content, "!ping"):
		discord.ChannelMessageSend(message.ChannelID, "Pong🏓")
	}
}
