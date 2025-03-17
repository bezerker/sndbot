package bot

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/bezerker/sndbot/blizzard"
	config "github.com/bezerker/sndbot/config"
	"github.com/bezerker/sndbot/database"
	"github.com/bezerker/sndbot/util"
	"github.com/bwmarrin/discordgo"
)

func init() {
	// Initialize the util logger for tests
	util.Logger = log.New(os.Stdout, "TEST: ", log.LstdFlags)
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
	db = setupTestDB(t)
	defer db.Close()

	ts := NewTestSession()
	mockAPI := NewMockBlizzardAPI()
	blizzardAPI = mockAPI

	// Initialize with test config
	Initialize(config.Config{
		CommunityRoleID:    "test-community-role",
		GuildMemberRoleIDs: []string{"test-guild-role-1", "test-guild-role-2"},
	})

	// Add a test character that exists and is in the guild
	addMockCharacter("testchar", "testrealm", true)

	msg := createTestMessage("!register testchar testrealm", "testuser", "channel1")
	ts.SetChannelType(discordgo.ChannelTypeGuildText) // Set to guild channel for role management

	newMessage(ts, msg)

	messages := ts.GetMessages("channel1")
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	expectedResponse := "Successfully registered character testchar on server testrealm (Stand and Deliver member)"
	if messages[0] != expectedResponse {
		t.Errorf("Expected message '%s', got '%s'", expectedResponse, messages[0])
	}

	// Verify database entry
	reg, err := database.GetCharacter(db, "testuser")
	if err != nil {
		t.Errorf("Failed to get character: %v", err)
	}
	if reg == nil {
		t.Error("Expected registration to exist")
	} else {
		if reg.CharacterName != "testchar" || reg.Server != "testrealm" {
			t.Errorf("Wrong registration data. Got character=%s, server=%s", reg.CharacterName, reg.Server)
		}
	}

	// Verify role assignments
	roles := ts.GetUserRoles("test-user-id")
	hasCommunityRole := false
	hasGuildRole := false
	for _, role := range roles {
		if role == cfg.CommunityRoleID {
			hasCommunityRole = true
		}
		if role == cfg.GuildMemberRoleIDs[0] {
			hasGuildRole = true
		}
	}
	if !hasCommunityRole {
		t.Error("Expected user to have community role")
	}
	if !hasGuildRole {
		t.Error("Expected user to have guild role")
	}
}

func TestAdminCommands(t *testing.T) {
	db = setupTestDB(t)
	defer db.Close()

	ts := NewTestSession()
	adminUser := "admin"
	normalUser := "normal"

	// Add admin user
	err := database.AddAdmin(db, adminUser)
	if err != nil {
		t.Fatalf("Failed to add admin: %v", err)
	}

	tests := []struct {
		name          string
		user          string
		command       string
		wantMsg       string
		isDM          bool
		shouldRespond bool
	}{
		{
			name:          "Admin help in DM",
			user:          adminUser,
			command:       "!admin-help",
			wantMsg:       "Available admin commands (DM only):",
			isDM:          true,
			shouldRespond: true,
		},
		{
			name:          "Admin help in channel",
			user:          adminUser,
			command:       "!admin-help",
			isDM:          false,
			shouldRespond: false,
		},
		{
			name:          "Non-admin help",
			user:          normalUser,
			command:       "!admin-help",
			isDM:          true,
			shouldRespond: false,
		},
		{
			name:          "Admin list users",
			user:          adminUser,
			command:       "!list-users",
			wantMsg:       "No registered users found",
			isDM:          true,
			shouldRespond: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channelID := fmt.Sprintf("channel-%s", tt.name)
			ts.messages = make(map[string][]string) // Clear previous messages

			if tt.isDM {
				ts.SetChannelType(discordgo.ChannelTypeDM)
			} else {
				ts.SetChannelType(discordgo.ChannelTypeGuildText)
			}

			msg := createTestMessage(tt.command, tt.user, channelID)
			handleAdminCommands(ts, msg, strings.Fields(tt.command))

			messages := ts.GetMessages(channelID)
			if tt.shouldRespond {
				if len(messages) == 0 {
					t.Errorf("Expected response but got none")
				} else if tt.wantMsg != "" && !strings.Contains(messages[0], tt.wantMsg) {
					t.Errorf("Expected message containing '%s', got '%s'", tt.wantMsg, messages[0])
				}
			} else {
				if len(messages) > 0 {
					t.Errorf("Expected no response, got '%v'", messages)
				}
			}
		})
	}
}

func TestWhoamiCommand(t *testing.T) {
	db = setupTestDB(t)
	defer db.Close()

	ts := NewTestSession()
	username := "testuser"
	channelID := "channel1"

	// Test whoami before registration
	msg := createTestMessage("!whoami", username, channelID)
	newMessage(ts, msg)

	messages := ts.GetMessages(channelID)
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	if !strings.Contains(messages[0], "haven't registered") {
		t.Errorf("Expected 'haven't registered' message, got '%s'", messages[0])
	}

	// Register a character
	reg := database.CharacterRegistration{
		DiscordUsername: username,
		CharacterName:   "testchar",
		Server:          "testrealm",
	}
	err := database.RegisterCharacter(db, reg)
	if err != nil {
		t.Fatalf("Failed to register character: %v", err)
	}

	// Clear previous messages
	ts.messages = make(map[string][]string)

	// Test whoami after registration
	newMessage(ts, msg)

	messages = ts.GetMessages(channelID)
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	expectedResponse := fmt.Sprintf("Your registered character is testchar on server testrealm")
	if messages[0] != expectedResponse {
		t.Errorf("Expected message '%s', got '%s'", expectedResponse, messages[0])
	}
}

func TestHelpCommand(t *testing.T) {
	db = setupTestDB(t)
	defer db.Close()

	ts := NewTestSession()
	msg := createTestMessage("!help", "testuser", "channel1")
	newMessage(ts, msg)

	messages := ts.GetMessages("channel1")
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	expectedParts := []string{
		"Available commands:",
		"!help",
		"!register",
		"!whoami",
		"!guild",
	}

	for _, part := range expectedParts {
		if !strings.Contains(messages[0], part) {
			t.Errorf("Expected help message to contain '%s'", part)
		}
	}
}

func TestSimpleCommands(t *testing.T) {
	db = setupTestDB(t)
	defer db.Close()

	tests := []struct {
		command string
		want    string
	}{
		{"!ping", "Pong🏓"},
		{"!bye", "Good Bye👋"},
	}

	ts := NewTestSession()
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			ts.messages = make(map[string][]string) // Clear previous messages
			msg := createTestMessage(tt.command, "testuser", "channel1")
			newMessage(ts, msg)

			messages := ts.GetMessages("channel1")
			if len(messages) != 1 {
				t.Errorf("Expected 1 message, got %d", len(messages))
			}
			if messages[0] != tt.want {
				t.Errorf("Expected message '%s', got '%s'", tt.want, messages[0])
			}
		})
	}
}

// MockBlizzardAPI is a mock implementation of the Blizzard API client for testing
type MockBlizzardAPI struct {
	existingCharacters map[string]bool
	guildMembers       map[string]bool
}

var currentMock *MockBlizzardAPI

func NewMockBlizzardAPI() BlizzardAPI {
	mock := &MockBlizzardAPI{
		existingCharacters: make(map[string]bool),
		guildMembers:       make(map[string]bool),
	}
	currentMock = mock
	blizzardAPI = mock
	return mock
}

// CharacterExists mocks the character existence check
func (m *MockBlizzardAPI) CharacterExists(characterName, realm string) (bool, error) {
	key := fmt.Sprintf("%s-%s", strings.ToLower(characterName), strings.ToLower(realm))
	exists := m.existingCharacters[key]
	if util.IsDebugEnabled() {
		util.Logger.Printf("[MOCK] Character exists check for %s: %v", key, exists)
	}
	return exists, nil
}

// IsCharacterInGuild mocks the guild membership check
func (m *MockBlizzardAPI) IsCharacterInGuild(characterName, realm string, guildID int) (bool, error) {
	key := fmt.Sprintf("%s-%s", strings.ToLower(characterName), strings.ToLower(realm))
	inGuild := m.guildMembers[key]
	if util.IsDebugEnabled() {
		util.Logger.Printf("[MOCK] Guild membership check for %s (guild ID %d): %v", key, guildID, inGuild)
	}
	return inGuild, nil
}

// GetCharacterGuild mocks getting a character's guild information
func (m *MockBlizzardAPI) GetCharacterGuild(characterName, realm string) (*blizzard.Guild, error) {
	key := fmt.Sprintf("%s-%s", strings.ToLower(characterName), strings.ToLower(realm))
	if !m.guildMembers[key] {
		return nil, nil
	}
	return &blizzard.Guild{
		Name: "Stand and Deliver",
		ID:   70395110,
		Realm: blizzard.Realm{
			Name: "Cenarius",
			ID:   1,
			Slug: "cenarius",
		},
	}, nil
}

// GetGuildInfo mocks getting guild information
func (m *MockBlizzardAPI) GetGuildInfo(characterName, realm string) (*blizzard.GuildInfo, error) {
	key := fmt.Sprintf("%s-%s", strings.ToLower(characterName), strings.ToLower(realm))
	if !m.guildMembers[key] {
		return nil, nil
	}
	return &blizzard.GuildInfo{
		Name:    "Stand and Deliver",
		Rank:    3,
		Faction: "Alliance",
	}, nil
}

// GetGuildMemberInfo mocks getting guild member information
func (m *MockBlizzardAPI) GetGuildMemberInfo(characterName, realmSlug, guildName string) (*blizzard.GuildMember, error) {
	key := fmt.Sprintf("%s-%s", strings.ToLower(characterName), strings.ToLower(realmSlug))
	if !m.guildMembers[key] {
		return nil, nil
	}
	member := &blizzard.GuildMember{}
	member.Character.Name = characterName
	member.Character.Realm.Name = realmSlug
	member.Character.Realm.ID = 1
	member.Character.Realm.Slug = strings.ToLower(strings.ReplaceAll(realmSlug, " ", "-"))
	member.Rank = 3
	return member, nil
}

func addMockCharacter(name, realm string, inGuild bool) {
	if currentMock == nil {
		return
	}
	key := fmt.Sprintf("%s-%s", strings.ToLower(name), strings.ToLower(realm))
	currentMock.existingCharacters[key] = true
	if inGuild {
		currentMock.guildMembers[key] = true
	}
}

// Test registration with non-existent character
func TestRegisterNonExistentCharacter(t *testing.T) {
	db = setupTestDB(t)
	defer db.Close()

	ts := NewTestSession()
	mockAPI := NewMockBlizzardAPI()
	blizzardAPI = mockAPI

	msg := createTestMessage("!register fakechar testrealm", "testuser", "channel1")
	ts.SetChannelType(discordgo.ChannelTypeGuildText)

	newMessage(ts, msg)

	messages := ts.GetMessages("channel1")
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	expectedResponse := "Character fakechar was not found on realm testrealm. Please check the spelling and try again."
	if messages[0] != expectedResponse {
		t.Errorf("Expected message '%s', got '%s'", expectedResponse, messages[0])
	}

	// Verify no roles were assigned
	roles := ts.GetUserRoles("test-user-id")
	if len(roles) > 0 {
		t.Errorf("Expected no roles to be assigned, got %v", roles)
	}
}

// Test registration with existing character not in guild
func TestRegisterNonGuildCharacter(t *testing.T) {
	db = setupTestDB(t)
	defer db.Close()

	ts := NewTestSession()
	mockAPI := NewMockBlizzardAPI()
	blizzardAPI = mockAPI

	// Initialize with test config
	Initialize(config.Config{
		CommunityRoleID:    "test-community-role",
		GuildMemberRoleIDs: []string{"test-guild-role-1", "test-guild-role-2"},
	})

	// Add a test character that exists but is not in the guild
	addMockCharacter("testchar", "testrealm", false)

	msg := createTestMessage("!register testchar testrealm", "testuser", "channel1")
	ts.SetChannelType(discordgo.ChannelTypeGuildText)

	newMessage(ts, msg)

	messages := ts.GetMessages("channel1")
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	expectedResponse := "Successfully registered character testchar on server testrealm"
	if messages[0] != expectedResponse {
		t.Errorf("Expected message '%s', got '%s'", expectedResponse, messages[0])
	}

	// Verify only community role was assigned
	roles := ts.GetUserRoles("test-user-id")
	if len(roles) != 1 {
		t.Errorf("Expected 1 role, got %d", len(roles))
	}
	if len(roles) > 0 && roles[0] != cfg.CommunityRoleID {
		t.Errorf("Expected community role, got %s", roles[0])
	}
}
