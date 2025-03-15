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

// DiscordSession is an interface that defines the methods we need from discordgo.Session
type DiscordSession interface {
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	GetState() *discordgo.State
}

// DiscordWrapper wraps a discordgo.Session to implement our interface
type DiscordWrapper struct {
	*discordgo.Session
}

func (w *DiscordWrapper) GetState() *discordgo.State {
	return w.State
}

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

	wrapper := &DiscordWrapper{Session: discord}

	// add a event handler
	discord.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		newMessage(wrapper, m)
	})

	// open the connection
	err = discord.Open()
	util.CheckNilErr(err)
	defer discord.Close()

	fmt.Println("Bot is running!")

	// Wait for a signal to quit
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	fmt.Println("Graceful shutdown")
}

func handleAdminCommands(discord DiscordSession, message *discordgo.MessageCreate, args []string) {
	// Only process admin commands in DMs
	channel, err := discord.Channel(message.ChannelID)
	if err != nil {
		util.Logger.Printf("Error getting channel info: %v", err)
		return
	}

	if channel.Type != discordgo.ChannelTypeDM {
		return
	}

	isAdmin, err := database.IsAdmin(db, message.Author.Username)
	if err != nil {
		util.Logger.Printf("Error checking admin status: %v", err)
		discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Error checking admin status: %v", err))
		return
	}

	if !isAdmin {
		return
	}

	switch args[0] {
	case "!addadmin":
		if len(args) != 2 {
			discord.ChannelMessageSend(message.ChannelID, "Usage: !addadmin <discord_username>")
			return
		}
		targetUser := args[1]
		err := database.AddAdmin(db, targetUser)
		if err != nil {
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Error adding admin: %v", err))
			return
		}
		discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Successfully added %s as admin", targetUser))

	case "!removeadmin":
		if len(args) != 2 {
			discord.ChannelMessageSend(message.ChannelID, "Usage: !removeadmin <discord_username>")
			return
		}
		targetUser := args[1]
		err := database.RemoveAdmin(db, targetUser)
		if err != nil {
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Error removing admin: %v", err))
			return
		}
		discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Successfully removed %s as admin", targetUser))

	case "!register-user":
		if len(args) != 4 {
			discord.ChannelMessageSend(message.ChannelID, "Usage: !register-user <discord_username> <character_name> <server>")
			return
		}
		registration := database.CharacterRegistration{
			DiscordUsername: args[1],
			CharacterName:   args[2],
			Server:          args[3],
		}
		err := database.RegisterCharacter(db, registration)
		if err != nil {
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Error registering character: %v", err))
			return
		}
		discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Successfully registered character %s on server %s for %s", args[2], args[3], args[1]))

	case "!remove-user":
		if len(args) != 2 {
			discord.ChannelMessageSend(message.ChannelID, "Usage: !remove-user <discord_username>")
			return
		}
		err := database.RemoveCharacterRegistration(db, args[1])
		if err != nil {
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Error removing registration: %v", err))
			return
		}
		discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Successfully removed registration for %s", args[1]))

	case "!list-users":
		registrations, err := database.GetAllRegistrations(db)
		if err != nil {
			discord.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Error getting registrations: %v", err))
			return
		}
		if len(registrations) == 0 {
			discord.ChannelMessageSend(message.ChannelID, "No registered users found")
			return
		}

		var response strings.Builder
		response.WriteString("Registered users:\n")
		for _, reg := range registrations {
			response.WriteString(fmt.Sprintf("- %s: %s on %s\n", reg.DiscordUsername, reg.CharacterName, reg.Server))
		}
		discord.ChannelMessageSend(message.ChannelID, response.String())

	case "!admin-help":
		helpMessage := `Available admin commands (DM only):
!admin-help - Show this help message
!addadmin <discord_username> - Add a new admin
!removeadmin <discord_username> - Remove an admin
!register-user <discord_username> <character_name> <server> - Register a character for a user
!remove-user <discord_username> - Remove a user's registration
!list-users - List all registered users`
		discord.ChannelMessageSend(message.ChannelID, helpMessage)
	}
}

func newMessage(s DiscordSession, m *discordgo.MessageCreate) {
	if m.Author.ID == s.GetState().User.ID {
		return
	}

	// Split the message content into words
	args := strings.Fields(m.Content)
	if len(args) == 0 {
		return
	}

	// Check for admin commands first
	if strings.HasPrefix(args[0], "!admin-") || args[0] == "!addadmin" || args[0] == "!removeadmin" || args[0] == "!register-user" || args[0] == "!remove-user" || args[0] == "!list-users" {
		handleAdminCommands(s, m, args)
		return
	}

	// Handle regular commands
	switch args[0] {
	case "!register":
		if len(args) != 3 {
			s.ChannelMessageSend(m.ChannelID, "Usage: !register <character_name> <server>")
			return
		}
		characterName := args[1]
		server := args[2]

		// Create registration
		reg := database.CharacterRegistration{
			DiscordUsername: m.Author.Username,
			CharacterName:   characterName,
			Server:          server,
		}

		// Register character
		err := database.RegisterCharacter(db, reg)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Failed to register character: %v", err))
			return
		}

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Successfully registered character %s on server %s for %s", characterName, server, m.Author.Username))

	case "!whoami":
		reg, err := database.GetCharacter(db, m.Author.Username)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error: %v", err))
			return
		}
		if reg == nil {
			s.ChannelMessageSend(m.ChannelID, "You haven't registered a character yet. Use !register <character_name> <server> to register.")
			return
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Your registered character is %s on server %s", reg.CharacterName, reg.Server))

	case "!guild":
		reg, err := database.GetCharacter(db, m.Author.Username)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error: %v", err))
			return
		}
		if reg == nil {
			s.ChannelMessageSend(m.ChannelID, "You haven't registered a character yet. Use !register <character_name> <server> to register.")
			return
		}

		guildInfo, err := blizzardAPI.GetGuildInfo(reg.CharacterName, reg.Server)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Failed to get guild info: %v", err))
			return
		}

		if guildInfo == nil {
			s.ChannelMessageSend(m.ChannelID, "Character is not in a guild")
			return
		}

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Guild: %s\nRank: %s", guildInfo.Name, guildInfo.Rank))

	case "!help":
		helpMessage := `Available commands:
!help - Show this help message
!register <character_name> <server> - Register your character
!whoami - Show your registered character
!guild - Show your guild information
!ping - Pong
!bye - Say goodbye`
		s.ChannelMessageSend(m.ChannelID, helpMessage)

	case "!ping":
		s.ChannelMessageSend(m.ChannelID, "Pong🏓")

	case "!bye":
		s.ChannelMessageSend(m.ChannelID, "Good Bye👋")
	}
}
