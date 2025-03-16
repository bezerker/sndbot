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
	blizzardAPI BlizzardAPI
	cfg         config.Config
)

// Initialize the bot with the given configuration
func Initialize(config config.Config) {
	cfg = config
}

// hasAnyRole checks if a member has any of the specified roles
func hasAnyRole(member *discordgo.Member, roles []string) bool {
	if member == nil {
		return false
	}

	memberRoleMap := make(map[string]bool)
	for _, role := range member.Roles {
		memberRoleMap[role] = true
	}

	for _, role := range roles {
		if memberRoleMap[role] {
			return true
		}
	}
	return false
}

// updateMemberRoles handles role assignments based on character verification and guild membership
func updateMemberRoles(s DiscordSession, guildID string, member *discordgo.Member, characterExists bool, isInGuild bool) error {
	if !characterExists {
		return nil // Do nothing if character doesn't exist
	}

	// Check if member already has the community role
	hasCommunityRole := false
	for _, role := range member.Roles {
		if role == cfg.CommunityRoleID {
			hasCommunityRole = true
			break
		}
	}

	// Add community role if character exists and user doesn't have it yet
	if !hasCommunityRole {
		if util.IsDebugEnabled() {
			util.Logger.Printf("Adding community role to user %s", member.User.Username)
		}
		err := s.GuildMemberRoleAdd(guildID, member.User.ID, cfg.CommunityRoleID)
		if err != nil {
			return fmt.Errorf("failed to add community role: %v", err)
		}
	} else if util.IsDebugEnabled() {
		util.Logger.Printf("User %s already has community role", member.User.Username)
	}

	// If character is in guild and doesn't have any guild roles, add entry level role
	if isInGuild && !hasAnyRole(member, cfg.GuildMemberRoleIDs) {
		if util.IsDebugEnabled() {
			util.Logger.Printf("Adding guild member role to user %s", member.User.Username)
		}
		err := s.GuildMemberRoleAdd(guildID, member.User.ID, cfg.GuildMemberRoleIDs[0])
		if err != nil {
			return fmt.Errorf("failed to add guild role: %v", err)
		}
	} else if util.IsDebugEnabled() && isInGuild {
		util.Logger.Printf("User %s already has a guild role", member.User.Username)
	}

	return nil
}

// DiscordSession is an interface that defines the methods we need from discordgo.Session
type DiscordSession interface {
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	GetState() *discordgo.State
	GuildMember(guildID, userID string) (*discordgo.Member, error)
	GuildMemberRoleAdd(guildID, userID, roleID string) error
}

// DiscordWrapper wraps a discordgo.Session to implement our interface
type DiscordWrapper struct {
	*discordgo.Session
}

func (w *DiscordWrapper) GetState() *discordgo.State {
	return w.State
}

func (w *DiscordWrapper) GuildMember(guildID, userID string) (*discordgo.Member, error) {
	return w.Session.GuildMember(guildID, userID)
}

func (w *DiscordWrapper) GuildMemberRoleAdd(guildID, userID, roleID string) error {
	return w.Session.GuildMemberRoleAdd(guildID, userID, roleID)
}

// BlizzardAPI is an interface for the Blizzard API client
type BlizzardAPI interface {
	CharacterExists(characterName, realm string) (bool, error)
	IsCharacterInGuild(characterName, realm string, guildID int) (bool, error)
	GetCharacterGuild(characterName, realm string) (*blizzard.Guild, error)
	GetGuildInfo(characterName, realm string) (*blizzard.GuildInfo, error)
	GetGuildMemberInfo(characterName, realmSlug, guildName string) (*blizzard.GuildMember, error)
}

func RunBot(config config.Config) {
	// Initialize logger
	if err := util.InitLogger(); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		return
	}
	defer util.CloseLogger()

	util.Logger.Print("Starting bot...")

	// Initialize configuration
	Initialize(config)

	// Initialize database
	var err error
	db, err = database.InitDB(config.DBPath)
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

		// First, check if the character exists
		exists, err := blizzardAPI.CharacterExists(characterName, server)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error verifying character: %v", err))
			return
		}

		if !exists {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Character %s was not found on realm %s. Please check the spelling and try again.", characterName, server))
			return
		}

		// Check if character is in Stand and Deliver
		isInGuild, err := blizzardAPI.IsCharacterInGuild(characterName, server, 70395110) // Stand and Deliver guild ID
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error checking guild membership: %v", err))
			return
		}

		// Create registration
		reg := database.CharacterRegistration{
			DiscordUsername: m.Author.Username,
			CharacterName:   characterName,
			Server:          server,
		}

		// Register character
		err = database.RegisterCharacter(db, reg)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Failed to register character: %v", err))
			return
		}

		// Get the Discord guild (server) ID from the message
		channel, err := s.Channel(m.ChannelID)
		if err != nil {
			util.Logger.Printf("Error getting channel info: %v", err)
			return
		}

		// Only process role updates if this is in a guild channel
		if channel.GuildID != "" {
			// Get member information
			member, err := s.GuildMember(channel.GuildID, m.Author.ID)
			if err != nil {
				util.Logger.Printf("Error getting member info: %v", err)
			} else {
				// Update roles
				err = updateMemberRoles(s, channel.GuildID, member, exists, isInGuild)
				if err != nil {
					util.Logger.Printf("Error updating roles: %v", err)
					s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Character registered successfully, but there was an error updating roles: %v", err))
					return
				}
			}
		}

		successMsg := fmt.Sprintf("Successfully registered character %s on server %s", characterName, server)
		if isInGuild {
			successMsg += " (Stand and Deliver member)"
		}
		s.ChannelMessageSend(m.ChannelID, successMsg)

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
			if strings.Contains(err.Error(), "guild not found") {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Could not find guild information. Please verify:\n1. The character %s exists on realm %s\n2. The character is in a guild\n3. The realm name is spelled correctly", reg.CharacterName, reg.Server))
			} else {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Failed to get guild info: %v", err))
			}
			return
		}

		if guildInfo == nil {
			s.ChannelMessageSend(m.ChannelID, "Character is not in a guild")
			return
		}

		rankStr := "Unknown"
		if guildInfo.Rank >= 0 {
			rankStr = fmt.Sprintf("%d", guildInfo.Rank)
		}

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Guild: %s\nFaction: %s\nRank: %s", guildInfo.Name, guildInfo.Faction, rankStr))

	case "!help":
		helpMessage := `Available commands:
!help - Show this help message
!register <character_name> <server> - Register your character
!whoami - Show your registered character
!guild - Show your guild information
!ping - Pong
!bye - Say goodbye
!checkguild <character> <realm> - Check if a character is in Stand and Deliver`
		s.ChannelMessageSend(m.ChannelID, helpMessage)

	case "!ping":
		s.ChannelMessageSend(m.ChannelID, "Pong🏓")

	case "!bye":
		s.ChannelMessageSend(m.ChannelID, "Good Bye👋")

	case "!checkguild":
		if len(args) < 3 {
			s.ChannelMessageSend(m.ChannelID, "Usage: !checkguild <character> <realm>")
			return
		}
		character := args[1]
		realm := args[2]

		// Stand and Deliver guild ID on Cenarius
		guildID := 70395110

		isInGuild, err := blizzardAPI.IsCharacterInGuild(character, realm, guildID)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error checking guild membership: %v", err))
			return
		}

		if isInGuild {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s-%s is in Stand and Deliver", character, realm))
		} else {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s-%s is not in Stand and Deliver", character, realm))
		}
	}
}
