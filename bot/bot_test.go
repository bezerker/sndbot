package bot

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/bezerker/sndbot/blizzard"
	"github.com/bezerker/sndbot/config"
	"github.com/bezerker/sndbot/database"
	"github.com/bezerker/sndbot/util"
	"github.com/bwmarrin/discordgo"
)

var (
	session      *discordgo.Session
	lastResponse string
)

func init() {
	// Initialize the util logger for tests
	util.Logger = log.New(os.Stdout, "TEST: ", log.LstdFlags)

	// Initialize test session
	var err error
	session, err = discordgo.New("Bot " + "test-token")
	if err != nil {
		panic(err)
	}

	// Initialize test config
	cfg = config.Config{
		CommunityRoleID:    "test-community-role",
		GuildMemberRoleIDs: []string{"test-guild-role-1", "test-guild-role-2"},
	}

	// Initialize test database
	db, err = database.InitDB(":memory:")
	if err != nil {
		panic(err)
	}
}

// TestSession is a custom session type for testing
type TestSession struct {
	messages    map[string][]string // channelID -> messages
	channelType discordgo.ChannelType
	state       *discordgo.State
	roles       map[string][]string // userID -> roleIDs
	guildID     string
}

func NewTestSession() *TestSession {
	state := discordgo.NewState()
	state.User = &discordgo.User{
		ID: "bot-id",
	}

	return &TestSession{
		messages:    make(map[string][]string),
		channelType: discordgo.ChannelTypeDM,
		state:       state,
		roles:       make(map[string][]string),
		guildID:     "test-guild",
	}
}

func (ts *TestSession) ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if ts.messages[channelID] == nil {
		ts.messages[channelID] = make([]string, 0)
	}
	ts.messages[channelID] = append(ts.messages[channelID], content)
	return &discordgo.Message{
		Content:   content,
		ChannelID: channelID,
	}, nil
}

func (ts *TestSession) Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	channel := &discordgo.Channel{
		ID:   channelID,
		Type: ts.channelType,
	}
	if ts.channelType == discordgo.ChannelTypeGuildText {
		channel.GuildID = ts.guildID
	}
	return channel, nil
}

func (ts *TestSession) GetState() *discordgo.State {
	return ts.state
}

func (ts *TestSession) SetChannelType(channelType discordgo.ChannelType) {
	ts.channelType = channelType
}

func (ts *TestSession) GetMessages(channelID string) []string {
	return ts.messages[channelID]
}

func (ts *TestSession) GuildMember(guildID, userID string) (*discordgo.Member, error) {
	roles := ts.roles[userID]
	if roles == nil {
		roles = make([]string, 0)
	}
	return &discordgo.Member{
		User: &discordgo.User{
			ID:       userID,
			Username: "testuser",
		},
		Roles: roles,
	}, nil
}

func (ts *TestSession) GuildMemberRoleAdd(guildID, userID, roleID string) error {
	if ts.roles[userID] == nil {
		ts.roles[userID] = make([]string, 0)
	}
	// Check if role already exists
	for _, role := range ts.roles[userID] {
		if role == roleID {
			return nil
		}
	}
	ts.roles[userID] = append(ts.roles[userID], roleID)
	return nil
}

func (ts *TestSession) GetUserRoles(userID string) []string {
	return ts.roles[userID]
}

// Test helper functions
func setupTestDB(t *testing.T) *sql.DB {
	db, err := database.InitDB(":memory:") // Use SQLite in-memory database
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return db
}

func createTestMessage(content, username, channelID string) *discordgo.MessageCreate {
	return &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content: content,
			Author: &discordgo.User{
				Username: username,
				ID:       "test-user-id",
			},
			ChannelID: channelID,
		},
	}
}

// Tests
func TestRegisterCommand(t *testing.T) {
	ts := NewMockSession()
	mockAPI := NewMockBlizzardAPI()
	blizzardAPI = mockAPI

	// Set up test data
	characterName := "TestChar"
	realm := "TestRealm"
	key := fmt.Sprintf("%s-%s", characterName, realm)
	mockAPI.Characters[key] = true

	guild := &blizzard.Guild{
		Name: "Stand and Deliver",
		ID:   70395110,
		Realm: blizzard.Realm{
			Name: realm,
			ID:   1,
			Slug: strings.ToLower(strings.ReplaceAll(realm, " ", "-")),
		},
	}
	mockAPI.Guilds[key] = guild

	// Create test message
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content: fmt.Sprintf("!register %s %s", characterName, realm),
			Author: &discordgo.User{
				ID: "123456789",
			},
		},
	}

	// Process message
	newMessage(ts, msg)

	// Verify response
	expected := fmt.Sprintf("Successfully registered character %s on server %s (Stand and Deliver member)", characterName, realm)
	if !strings.Contains(lastResponse, expected) {
		t.Errorf("Expected response to contain '%s', got '%s'", expected, lastResponse)
	}

	// Verify role assignment
	roles := ts.GetUserRoles("123456789")
	if len(roles) == 0 {
		t.Error("Expected user to have roles assigned")
	}
	// Verify community role
	hasCommunityRole := false
	for _, role := range roles {
		if role == cfg.CommunityRoleID {
			hasCommunityRole = true
			break
		}
	}
	if !hasCommunityRole {
		t.Error("Expected user to have community role")
	}
	// Verify guild role
	hasGuildRole := false
	for _, role := range roles {
		if role == cfg.GuildMemberRoleIDs[0] {
			hasGuildRole = true
			break
		}
	}
	if !hasGuildRole {
		t.Error("Expected user to have guild role")
	}
}

func TestAdminCommands(t *testing.T) {
	ts := NewMockSession()
	mockAPI := NewMockBlizzardAPI()
	blizzardAPI = mockAPI

	// Set up admin user in database
	err := database.AddAdmin(db, "testadmin")
	if err != nil {
		t.Fatalf("Failed to add admin user: %v", err)
	}

	// Set channel type to DM for admin commands
	ts.SetChannelType(discordgo.ChannelTypeDM)

	// Test addadmin command
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content: "!addadmin 123456789",
			Author: &discordgo.User{
				ID:       "987654321", // Admin user ID
				Username: "testadmin",
			},
		},
	}

	newMessage(ts, msg)

	expected := "Successfully added 123456789 as admin"
	if !strings.Contains(lastResponse, expected) {
		t.Errorf("Expected response to contain '%s', got '%s'", expected, lastResponse)
	}

	// Test removeadmin command
	msg = &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content: "!removeadmin 123456789",
			Author: &discordgo.User{
				ID:       "987654321", // Admin user ID
				Username: "testadmin",
			},
		},
	}

	newMessage(ts, msg)

	expected = "Successfully removed 123456789 as admin"
	if !strings.Contains(lastResponse, expected) {
		t.Errorf("Expected response to contain '%s', got '%s'", expected, lastResponse)
	}
}

func TestWhoamiCommand(t *testing.T) {
	ts := NewMockSession()
	mockAPI := NewMockBlizzardAPI()
	blizzardAPI = mockAPI

	// Set up test data
	characterName := "TestChar"
	realm := "TestRealm"
	key := fmt.Sprintf("%s-%s", characterName, realm)
	mockAPI.Characters[key] = true

	guild := &blizzard.Guild{
		Name: "Stand and Deliver",
		ID:   70395110,
		Realm: blizzard.Realm{
			Name: realm,
			ID:   1,
			Slug: strings.ToLower(strings.ReplaceAll(realm, " ", "-")),
		},
	}
	mockAPI.Guilds[key] = guild

	// Register the character first
	reg := database.CharacterRegistration{
		DiscordUsername: "testuser",
		CharacterName:   characterName,
		Server:          realm,
	}
	err := database.RegisterCharacter(db, reg)
	if err != nil {
		t.Fatalf("Failed to register character: %v", err)
	}

	// Create test message
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content: "!whoami",
			Author: &discordgo.User{
				ID:       "123456789",
				Username: "testuser",
			},
		},
	}

	// Process message
	newMessage(ts, msg)

	// Verify response
	expected := fmt.Sprintf("Your registered character is %s on server %s", characterName, realm)
	if !strings.Contains(lastResponse, expected) {
		t.Errorf("Expected response to contain '%s', got '%s'", expected, lastResponse)
	}
}

func TestHelpCommand(t *testing.T) {
	ts := NewMockSession()
	mockAPI := NewMockBlizzardAPI()
	blizzardAPI = mockAPI

	// Create test message
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content: "!help",
			Author: &discordgo.User{
				ID: "123456789",
			},
		},
	}

	// Process message
	newMessage(ts, msg)

	// Verify response
	expected := "Available commands:"
	if !strings.Contains(lastResponse, expected) {
		t.Errorf("Expected response to contain '%s', got '%s'", expected, lastResponse)
	}
}

func TestSimpleCommands(t *testing.T) {
	ts := NewMockSession()
	mockAPI := NewMockBlizzardAPI()
	blizzardAPI = mockAPI

	// Test ping command
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content: "!ping",
			Author: &discordgo.User{
				ID: "123456789",
			},
		},
	}

	newMessage(ts, msg)

	expected := "Pong🏓"
	if !strings.Contains(lastResponse, expected) {
		t.Errorf("Expected response to contain '%s', got '%s'", expected, lastResponse)
	}

	// Test bye command
	msg = &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content: "!bye",
			Author: &discordgo.User{
				ID: "123456789",
			},
		},
	}

	newMessage(ts, msg)

	expected = "Good Bye👋"
	if !strings.Contains(lastResponse, expected) {
		t.Errorf("Expected response to contain '%s', got '%s'", expected, lastResponse)
	}
}

// MockBlizzardAPI implements BlizzardAPI for testing
type MockBlizzardAPI struct {
	Characters map[string]bool
	Guilds     map[string]*blizzard.Guild
	Members    map[string]*blizzard.GuildMember
}

func NewMockBlizzardAPI() *MockBlizzardAPI {
	return &MockBlizzardAPI{
		Characters: make(map[string]bool),
		Guilds:     make(map[string]*blizzard.Guild),
		Members:    make(map[string]*blizzard.GuildMember),
	}
}

func (m *MockBlizzardAPI) GetCharacterGuild(characterName, realm string) (*blizzard.Guild, error) {
	key := fmt.Sprintf("%s-%s", characterName, realm)
	if guild, ok := m.Guilds[key]; ok {
		return guild, nil
	}
	return nil, nil
}

func (m *MockBlizzardAPI) GetGuildInfo(characterName, realm string) (*blizzard.Guild, error) {
	return m.GetCharacterGuild(characterName, realm)
}

func (m *MockBlizzardAPI) GetGuildMemberInfo(characterName, realmSlug, guildName string) (*blizzard.GuildMember, error) {
	key := fmt.Sprintf("%s-%s-%s", characterName, realmSlug, guildName)
	if member, ok := m.Members[key]; ok {
		return member, nil
	}
	return nil, nil
}

func (m *MockBlizzardAPI) CharacterExists(characterName, realm string) (bool, error) {
	key := fmt.Sprintf("%s-%s", characterName, realm)
	return m.Characters[key], nil
}

func (m *MockBlizzardAPI) IsCharacterInGuild(characterName, realm string, guildID int) (bool, error) {
	guild, err := m.GetCharacterGuild(characterName, realm)
	if err != nil {
		return false, err
	}
	if guild == nil {
		return false, nil
	}
	return guild.ID == guildID, nil
}

func addMockCharacter(name, realm string, inGuild bool) {
	key := fmt.Sprintf("%s-%s", strings.ToLower(name), strings.ToLower(realm))
	mockAPI := NewMockBlizzardAPI()
	mockAPI.Characters[key] = true
	if inGuild {
		guild := &blizzard.Guild{
			Name: "TestGuild",
			ID:   12345,
			Realm: blizzard.Realm{
				Name: realm,
				ID:   1,
				Slug: strings.ToLower(strings.ReplaceAll(realm, " ", "-")),
			},
		}
		mockAPI.Guilds[key] = guild
	}
	blizzardAPI = mockAPI
}

// Test registration with non-existent character
func TestRegisterNonExistentCharacter(t *testing.T) {
	ts := NewMockSession()
	mockAPI := NewMockBlizzardAPI()
	blizzardAPI = mockAPI

	// Create test message for non-existent character
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content: "!register NonExistent TestRealm",
			Author: &discordgo.User{
				ID: "123456789",
			},
		},
	}

	// Process message
	newMessage(ts, msg)

	// Verify response
	expected := "Character NonExistent was not found on realm TestRealm. Please check the spelling and try again."
	if !strings.Contains(lastResponse, expected) {
		t.Errorf("Expected response to contain '%s', got '%s'", expected, lastResponse)
	}
}

// Test registration with existing character not in guild
func TestRegisterNonGuildCharacter(t *testing.T) {
	ts := NewMockSession()
	mockAPI := NewMockBlizzardAPI()
	blizzardAPI = mockAPI

	// Set up test data for character without guild
	characterName := "TestChar"
	realm := "TestRealm"
	key := fmt.Sprintf("%s-%s", characterName, realm)
	mockAPI.Characters[key] = true

	// Create test message
	msg := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			Content: fmt.Sprintf("!register %s %s", characterName, realm),
			Author: &discordgo.User{
				ID: "123456789",
			},
		},
	}

	// Process message
	newMessage(ts, msg)

	// Verify response
	expected := "Successfully registered character TestChar on server TestRealm"
	if !strings.Contains(lastResponse, expected) {
		t.Errorf("Expected response to contain '%s', got '%s'", expected, lastResponse)
	}
}

// MockDiscordSession implements the minimal Discord session interface needed for testing
type MockDiscordSession struct {
	*discordgo.Session
	channelType discordgo.ChannelType
	messages    map[string][]string
	userRoles   map[string][]string
	guildID     string
}

func NewMockSession() *MockDiscordSession {
	s := &MockDiscordSession{
		Session:     session,
		messages:    make(map[string][]string),
		userRoles:   make(map[string][]string),
		guildID:     "test-guild",
		channelType: discordgo.ChannelTypeGuildText,
	}
	s.State = discordgo.NewState()
	s.State.User = &discordgo.User{
		ID: "bot-id",
	}
	return s
}

func (s *MockDiscordSession) SetChannelType(channelType discordgo.ChannelType) {
	s.channelType = channelType
}

func (s *MockDiscordSession) ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	lastResponse = content
	if s.messages[channelID] == nil {
		s.messages[channelID] = make([]string, 0)
	}
	s.messages[channelID] = append(s.messages[channelID], content)
	return &discordgo.Message{Content: content}, nil
}

func (s *MockDiscordSession) Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	channel := &discordgo.Channel{
		ID:   channelID,
		Type: s.channelType,
	}
	if s.channelType == discordgo.ChannelTypeGuildText {
		channel.GuildID = s.guildID
	}
	return channel, nil
}

func (s *MockDiscordSession) GetMessages(channelID string) []string {
	return s.messages[channelID]
}

func (s *MockDiscordSession) GetUserRoles(userID string) []string {
	return s.userRoles[userID]
}

func (s *MockDiscordSession) GetState() *discordgo.State {
	return s.State
}

func (s *MockDiscordSession) GuildMember(guildID, userID string) (*discordgo.Member, error) {
	roles := s.userRoles[userID]
	if roles == nil {
		roles = make([]string, 0)
	}
	return &discordgo.Member{
		User: &discordgo.User{
			ID:       userID,
			Username: "testuser",
		},
		Roles: roles,
	}, nil
}

func (s *MockDiscordSession) GuildMemberRoleAdd(guildID, userID, roleID string) error {
	if s.userRoles[userID] == nil {
		s.userRoles[userID] = make([]string, 0)
	}
	// Check if role already exists
	for _, role := range s.userRoles[userID] {
		if role == roleID {
			return nil
		}
	}
	s.userRoles[userID] = append(s.userRoles[userID], roleID)
	return nil
}
