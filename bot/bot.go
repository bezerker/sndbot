package bot

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/bezerker/sndbot/blizzard"
	config "github.com/bezerker/sndbot/config"
	database "github.com/bezerker/sndbot/database"
	util "github.com/bezerker/sndbot/util"
	"github.com/bwmarrin/discordgo"
)

var (
	db          *sql.DB
	cfg         config.Config
	oauthStates = make(map[string]string) // state -> discord user ID
	oauthMutex  sync.RWMutex
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
func updateMemberRoles(s DiscordSession, guildID string, member *discordgo.Member, characterExists bool, isInGuild bool) (string, error) {
	if !characterExists {
		return "", nil // Do nothing if character doesn't exist
	}

	var roleUpdates []string

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
			return "", fmt.Errorf("failed to add community role: %v", err)
		}
		roleUpdates = append(roleUpdates, "Community Role")
	}

	// If character is in guild and doesn't have any guild roles, add entry level role
	if isInGuild && !hasAnyRole(member, cfg.GuildMemberRoleIDs) {
		if util.IsDebugEnabled() {
			util.Logger.Printf("Adding guild member role to user %s", member.User.Username)
		}
		err := s.GuildMemberRoleAdd(guildID, member.User.ID, cfg.GuildMemberRoleIDs[0])
		if err != nil {
			return "", fmt.Errorf("failed to add guild role: %v", err)
		}
		roleUpdates = append(roleUpdates, "Guild Member Role")
	}

	if len(roleUpdates) == 0 {
		if isInGuild {
			return "No new roles needed - you already have all applicable roles (Community and Guild Member)", nil
		}
		return "No new roles needed - you already have all applicable roles (Community)", nil
	}

	return fmt.Sprintf("Granted roles: %s", strings.Join(roleUpdates, ", ")), nil
}

// DiscordSession is an interface that defines the methods we need from discordgo.Session
type DiscordSession interface {
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	GetState() *discordgo.State
	GuildMember(guildID, userID string) (*discordgo.Member, error)
	GuildMemberRoleAdd(guildID, userID, roleID string) error
	InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error
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

func (w *DiscordWrapper) InteractionRespond(interaction *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	return w.Session.InteractionRespond(interaction, resp)
}

// BlizzardAPI is an interface for the Blizzard API client
type BlizzardAPI interface {
	CharacterExists(characterName, realm string) (bool, error)
	IsCharacterInGuild(characterName, realm string, guildID int) (bool, error)
	GetCharacterGuild(characterName, realm string) (*blizzard.Guild, error)
	GetGuildInfo(characterName, realm string) (*blizzard.GuildInfo, error)
	GetGuildMemberInfo(characterName, realmSlug, guildName string) (*blizzard.GuildMember, error)
}

// Bot represents the Discord bot instance
type Bot struct {
	session     DiscordSession
	blizzardAPI BlizzardAPI
}

// NewBot creates a new bot instance
func NewBot(session DiscordSession, blizzardAPI BlizzardAPI) *Bot {
	return &Bot{
		session:     session,
		blizzardAPI: blizzardAPI,
	}
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

	BotToken := config.DiscordToken
	// create a session
	discord, err := discordgo.New("Bot " + BotToken)
	if err != nil {
		util.Logger.Printf("Failed to create Discord session: %v", err)
		return
	}

	wrapper := &DiscordWrapper{Session: discord}

	// Initialize Blizzard API client and create bot instance
	blizzardClient := blizzard.NewBlizzardClient(config.BlizzardClientID, config.BlizzardSecret)
	bot := NewBot(wrapper, blizzardClient)

	// Register slash commands
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "auth",
			Description: "Authenticate with Blizzard to register your character",
		},
		{
			Name:        "select-character",
			Description: "Select a character from your Blizzard account to register",
		},
		{
			Name:        "register",
			Description: "Register your character manually",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "character",
					Description: "Your character name",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "server",
					Description: "Your server name",
					Required:    true,
				},
			},
		},
		{
			Name:        "whoami",
			Description: "Show your registered character",
		},
		{
			Name:        "guild",
			Description: "Show your guild information",
		},
		{
			Name:        "checkguild",
			Description: "Check if a character is in Stand and Deliver",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "character",
					Description: "Character name to check",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "server",
					Description: "Character's server",
					Required:    true,
				},
			},
		},
	}

	// Register handlers
	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			handleSlashCommand(bot, i)
		case discordgo.InteractionMessageComponent:
			handleComponentInteraction(bot, i)
		}
	})

	// Add ready handler to register commands when bot is ready
	discord.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		util.Logger.Printf("Bot is ready! Logged in as %s#%s", s.State.User.Username, s.State.User.Discriminator)

		// Register commands globally
		util.Logger.Print("Registering slash commands globally...")
		for _, command := range commands {
			_, err := s.ApplicationCommandCreate(s.State.User.ID, "", command)
			if err != nil {
				util.Logger.Printf("Error creating command %s: %v", command.Name, err)
			} else {
				util.Logger.Printf("Successfully registered command: %s", command.Name)
			}
		}
	})

	// open the connection
	err = discord.Open()
	if err != nil {
		util.Logger.Printf("Failed to open Discord connection: %v", err)
		return
	}
	defer discord.Close()

	util.Logger.Print("Bot is now running. Press CTRL-C to exit.")

	// Wait for interrupt signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	<-sc

	util.Logger.Print("Shutting down...")
}

func handleSlashCommand(bot *Bot, i *discordgo.InteractionCreate) {
	util.Logger.Printf("Handling slash command: %s", i.ApplicationCommandData().Name)
	data := discordgo.InteractionResponseData{
		Flags: discordgo.MessageFlagsEphemeral,
	}

	switch i.ApplicationCommandData().Name {
	case "auth":
		// Generate state for OAuth
		state := generateState(i.Member.User.ID)
		oauthMutex.Lock()
		oauthStates[state] = i.Member.User.ID
		oauthMutex.Unlock()

		// Generate OAuth URL
		authURL := blizzard.GetAuthURL(cfg.BlizzardClientID, cfg.OAuthRedirectURL, state)

		data.Content = fmt.Sprintf("Please click the link below to authenticate with Blizzard:\n%s", authURL)

	case "select-character":
		util.Logger.Printf("Processing select-character command for user %s", i.Member.User.Username)
		// Get user's characters from database
		characters, err := database.GetOAuthCharacters(db, i.Member.User.ID)
		if err != nil {
			util.Logger.Printf("Error retrieving characters: %v", err)
			data.Content = fmt.Sprintf("Error retrieving characters: %v", err)
		} else if len(characters) == 0 {
			util.Logger.Printf("No characters found for user %s", i.Member.User.Username)
			data.Content = "No characters found. Please use /auth first to authenticate with Blizzard."
		} else {
			util.Logger.Printf("Found %d characters for user %s", len(characters), i.Member.User.Username)
			// Create the character selection menu
			err = bot.handleSelectCharacter(i.Interaction, characters)
			if err != nil {
				data.Content = fmt.Sprintf("Error creating character selection menu: %v", err)
			}
		}

	case "register":
		options := i.ApplicationCommandData().Options
		characterName := options[0].StringValue()
		server := options[1].StringValue()

		// First, check if the character exists
		exists, err := bot.blizzardAPI.CharacterExists(characterName, server)
		if err != nil {
			data.Content = fmt.Sprintf("Error verifying character: %v", err)
		} else if !exists {
			data.Content = fmt.Sprintf("Character %s was not found on realm %s. Please check the spelling and try again.", characterName, server)
		} else {
			// Check if character is in Stand and Deliver
			isInGuild, err := bot.blizzardAPI.IsCharacterInGuild(characterName, server, 70395110)
			if err != nil {
				data.Content = fmt.Sprintf("Error checking guild membership: %v", err)
			} else {
				// Create registration
				reg := database.CharacterRegistration{
					DiscordUsername: i.Member.User.Username,
					CharacterName:   characterName,
					Server:          server,
				}

				// Register character
				err = database.RegisterCharacter(db, reg)
				if err != nil {
					data.Content = fmt.Sprintf("Failed to register character: %v", err)
				} else {
					// Update roles
					roleMsg, err := updateMemberRoles(bot.session, i.GuildID, i.Member, true, isInGuild)
					if err != nil {
						util.Logger.Printf("Error updating roles: %v", err)
					}

					if isInGuild {
						data.Content = fmt.Sprintf("Successfully registered character %s on server %s (Stand and Deliver member)", characterName, server)
					} else {
						data.Content = fmt.Sprintf("Successfully registered character %s on server %s", characterName, server)
					}

					if roleMsg != "" {
						data.Content += "\n" + roleMsg
					}
				}
			}
		}

	case "whoami":
		reg, err := database.GetCharacter(db, i.Member.User.Username)
		if err != nil {
			data.Content = fmt.Sprintf("Error: %v", err)
		} else if reg == nil {
			data.Content = "You haven't registered a character yet. Use /register to register."
		} else {
			data.Content = fmt.Sprintf("Your registered character is %s on server %s", reg.CharacterName, reg.Server)
		}

	case "guild":
		reg, err := database.GetCharacter(db, i.Member.User.Username)
		if err != nil {
			data.Content = fmt.Sprintf("Error: %v", err)
		} else if reg == nil {
			data.Content = "You haven't registered a character yet. Use /register to register."
		} else {
			guildInfo, err := bot.blizzardAPI.GetGuildInfo(reg.CharacterName, reg.Server)
			if err != nil {
				if strings.Contains(err.Error(), "guild not found") {
					data.Content = fmt.Sprintf("Could not find guild information. Please verify:\n1. The character %s exists on realm %s\n2. The character is in a guild\n3. The realm name is spelled correctly", reg.CharacterName, reg.Server)
				} else {
					data.Content = fmt.Sprintf("Failed to get guild info: %v", err)
				}
			} else if guildInfo == nil {
				data.Content = "Character is not in a guild"
			} else {
				rankStr := "Unknown"
				if guildInfo.Rank >= 0 {
					rankStr = fmt.Sprintf("%d", guildInfo.Rank)
				}
				data.Content = fmt.Sprintf("Guild: %s\nFaction: %s\nRank: %s", guildInfo.Name, guildInfo.Faction, rankStr)
			}
		}

	case "checkguild":
		options := i.ApplicationCommandData().Options
		character := options[0].StringValue()
		realm := options[1].StringValue()

		// Stand and Deliver guild ID on Cenarius
		guildID := 70395110

		isInGuild, err := bot.blizzardAPI.IsCharacterInGuild(character, realm, guildID)
		if err != nil {
			data.Content = fmt.Sprintf("Error checking guild membership: %v", err)
		} else if isInGuild {
			data.Content = fmt.Sprintf("%s-%s is in Stand and Deliver", character, realm)
		} else {
			data.Content = fmt.Sprintf("%s-%s is not in Stand and Deliver", character, realm)
		}
	}

	// Only send response if we haven't already sent one
	if data.Content != "" {
		util.Logger.Printf("Sending response for user %s: %s", i.Member.User.Username, data.Content)
		err := bot.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &data,
		})
		if err != nil {
			util.Logger.Printf("Error sending interaction response: %v", err)
		}
	}
}

// HandleOAuthCallback handles the OAuth callback from Blizzard
func HandleOAuthCallback(code, state string) error {
	// Get Discord user ID from state
	oauthMutex.RLock()
	discordUserID, exists := oauthStates[state]
	oauthMutex.RUnlock()

	if !exists {
		return fmt.Errorf("invalid OAuth state")
	}

	// Exchange code for token
	token, err := blizzard.ExchangeCodeForToken(code, cfg.BlizzardClientID, cfg.BlizzardSecret, cfg.OAuthRedirectURL)
	if err != nil {
		return fmt.Errorf("failed to exchange code for token: %v", err)
	}

	// Get user's characters
	characters, err := blizzard.GetUserCharacters(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to get characters: %v", err)
	}

	// Store characters in database for later selection
	err = database.StoreOAuthCharacters(db, discordUserID, characters)
	if err != nil {
		return fmt.Errorf("failed to store characters: %v", err)
	}

	// Clean up state
	oauthMutex.Lock()
	delete(oauthStates, state)
	oauthMutex.Unlock()

	return nil
}

// handleComponentInteraction handles interactions with message components (like select menus)
func handleComponentInteraction(bot *Bot, i *discordgo.InteractionCreate) {
	data := &discordgo.InteractionResponseData{
		Flags: discordgo.MessageFlagsEphemeral,
	}

	customID := i.MessageComponentData().CustomID

	// Handle cancel button
	if customID == "cancel_character_select" {
		data.Content = "Character selection cancelled."
	} else if strings.HasPrefix(customID, "select_character_") {
		if len(i.MessageComponentData().Values) == 0 {
			data.Content = "No character selected. Please try again."
		} else {
			// Parse selected value (format: "characterName-realmSlug")
			parts := strings.Split(i.MessageComponentData().Values[0], "-")
			if len(parts) != 2 {
				data.Content = "Invalid character selection. Please try again."
			} else {
				characterName := parts[0]
				realmSlug := parts[1]

				// Check if character is in Stand and Deliver
				isInGuild, err := bot.blizzardAPI.IsCharacterInGuild(characterName, realmSlug, 70395110)
				if err != nil {
					data.Content = fmt.Sprintf("Error checking guild membership: %v", err)
				} else {
					// Create registration
					reg := database.CharacterRegistration{
						DiscordUsername: i.Member.User.Username,
						CharacterName:   characterName,
						Server:          realmSlug,
					}

					// Register character
					err = database.RegisterCharacter(db, reg)
					if err != nil {
						data.Content = fmt.Sprintf("Failed to register character: %v", err)
					} else {
						// Update roles
						roleMsg, err := updateMemberRoles(bot.session, i.GuildID, i.Member, true, isInGuild)
						if err != nil {
							util.Logger.Printf("Error updating roles: %v", err)
						}

						if isInGuild {
							data.Content = fmt.Sprintf("Successfully registered character %s on server %s (Stand and Deliver member)", characterName, realmSlug)
						} else {
							data.Content = fmt.Sprintf("Successfully registered character %s on server %s", characterName, realmSlug)
						}

						if roleMsg != "" {
							data.Content += "\n" + roleMsg
						}
					}
				}
			}
		}
	}

	// Send response
	err := bot.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
	if err != nil {
		util.Logger.Printf("Error sending interaction response: %v", err)
	}
}

// generateState creates a secure random state for OAuth
func generateState(userID string) string {
	// Create a random state using the user ID and a random number
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	return fmt.Sprintf("%x-%s", randBytes, userID)
}

func (b *Bot) handleSelectCharacter(interaction *discordgo.Interaction, characters []blizzard.CharacterSummary) error {
	util.Logger.Printf("Creating character selection menu with %d characters", len(characters))

	// Split characters into chunks of 25 (Discord's limit)
	const maxOptions = 25
	var selectMenus []discordgo.MessageComponent

	for i := 0; i < len(characters); i += maxOptions {
		end := i + maxOptions
		if end > len(characters) {
			end = len(characters)
		}

		chunk := characters[i:end]
		util.Logger.Printf("Creating select menu %d with %d characters", i/maxOptions, len(chunk))

		options := make([]discordgo.SelectMenuOption, len(chunk))
		for j, char := range chunk {
			// Create a simple label and value without any special characters
			label := fmt.Sprintf("%s - %s", char.Name, char.Realm.Slug)
			if len(label) > 100 {
				label = label[:100]
			}

			value := fmt.Sprintf("%s-%s", char.Name, char.Realm.Slug)
			if len(value) > 100 {
				value = value[:100]
			}

			options[j] = discordgo.SelectMenuOption{
				Label:       label,
				Value:       value,
				Description: fmt.Sprintf("Level %d", char.Level),
			}
		}

		selectMenus = append(selectMenus, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    fmt.Sprintf("select_character_%d", i/maxOptions),
					Placeholder: "Select a character",
					Options:     options,
				},
			},
		})
	}

	// Add a cancel button
	selectMenus = append(selectMenus, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Cancel",
				Style:    discordgo.DangerButton,
				CustomID: "cancel_character_select",
			},
		},
	})

	util.Logger.Printf("Sending character selection menu with %d rows", len(selectMenus))
	err := b.session.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    "Please select a character:",
			Components: selectMenus,
		},
	})
	if err != nil {
		util.Logger.Printf("Error sending character selection menu: %v", err)
		return err
	}
	util.Logger.Printf("Successfully sent character selection menu")
	return nil
}
