package bot

import (
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/bezerker/sndbot/blizzard"
	config "github.com/bezerker/sndbot/config"
	database "github.com/bezerker/sndbot/database"
	util "github.com/bezerker/sndbot/util"
	"github.com/bwmarrin/discordgo"
)

var (
	db          *sql.DB
	blizzardAPI *blizzard.BlizzardClient
)

func RunBot(config config.Config) {
	// Initialize logger
	if err := util.InitLogger(); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		return
	}
	defer util.CloseLogger()

	util.Logger.Print("Starting bot...")

	var err error
	// Initialize database
	db, err = database.InitDB("characters.db")
	if err != nil {
		util.Logger.Printf("Failed to initialize database: %v", err)
		return
	}
	defer db.Close()

	// Initialize Blizzard API client
	blizzardAPI = blizzard.NewBlizzardClient(config.BlizzardClientID, config.BlizzardSecret)

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
	util.Logger.Print("Bot is now running. Press Ctrl+C to exit.")
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

	case strings.HasPrefix(message.Content, "!guild"):
		util.Logger.Printf("Guild lookup requested by user %s", message.Author.Username)

		registration, err := database.GetCharacter(db, message.Author.Username)
		if err != nil {
			util.Logger.Printf("Error looking up registration for %s: %v", message.Author.Username, err)
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Error looking up registration: %v", err))
			return
		}
		if registration == nil {
			util.Logger.Printf("No character registration found for user %s", message.Author.Username)
			discord.ChannelMessageSend(message.ChannelID, "You haven't registered a character yet. Use !register <character_name> <server> to register.")
			return
		}

		util.Logger.Printf("Looking up guild for character %s on server %s", registration.CharacterName, registration.Server)
		guild, err := blizzardAPI.GetCharacterGuild(registration.CharacterName, registration.Server)
		if err != nil {
			util.Logger.Printf("Error looking up guild information: %v", err)
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Error looking up guild information: %v", err))
			return
		}
		if guild == nil {
			util.Logger.Printf("Character %s on %s is not in a guild", registration.CharacterName, registration.Server)
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Character %s on %s is not in a guild", registration.CharacterName, registration.Server))
			return
		}

		response := fmt.Sprintf("%s is in the guild <%s> on %s (%s)",
			registration.CharacterName, guild.Name, guild.Realm.Name, guild.Faction.Name)
		util.Logger.Printf("Guild lookup successful: %s", response)
		discord.ChannelMessageSend(message.ChannelID, response)

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
!guild - Show your character's guild information
!bye - Say goodbye
!ping - Ping the bot`
		discord.ChannelMessageSend(message.ChannelID, helpMessage)

	case strings.Contains(message.Content, "!bye"):
		discord.ChannelMessageSend(message.ChannelID, "Good Bye👋")

	case strings.Contains(message.Content, "!ping"):
		discord.ChannelMessageSend(message.ChannelID, "Pong🏓")
	}
}
