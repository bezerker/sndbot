package bot

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/bezerker/sndbot/database"
	"github.com/bwmarrin/discordgo"
)

// TestSession is a custom session type for testing
type TestSession struct {
	messages    map[string][]string // channelID -> messages
	channelType discordgo.ChannelType
	state       *discordgo.State
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
	return &discordgo.Channel{
		ID:   channelID,
		Type: ts.channelType,
	}, nil
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
	msg := createTestMessage("!register testchar testrealm", "testuser", "channel1")
	newMessage(ts, msg)

	messages := ts.GetMessages("channel1")
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	expectedResponse := fmt.Sprintf("Successfully registered character testchar on server testrealm for testuser")
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
