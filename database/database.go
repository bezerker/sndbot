package database

import (
	"database/sql"
	"fmt"

	"github.com/bezerker/sndbot/blizzard"
	_ "github.com/mattn/go-sqlite3"
)

type CharacterRegistration struct {
	DiscordUsername string
	CharacterName   string
	Server          string
}

// InitDB initializes the database with required tables
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS character_registrations (
			discord_username TEXT PRIMARY KEY,
			character_name TEXT NOT NULL,
			server TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS admins (
			discord_username TEXT PRIMARY KEY
		);

		CREATE TABLE IF NOT EXISTS oauth_characters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			discord_user_id TEXT NOT NULL,
			character_name TEXT NOT NULL,
			realm_name TEXT NOT NULL,
			realm_slug TEXT NOT NULL,
			level INTEGER NOT NULL,
			UNIQUE(discord_user_id, character_name, realm_slug)
		);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return db, nil
}

// RegisterCharacter registers a character for a Discord user
func RegisterCharacter(db *sql.DB, registration CharacterRegistration) error {
	stmt := `
		REPLACE INTO character_registrations (discord_username, character_name, server)
		VALUES (?, ?, ?)`

	_, err := db.Exec(stmt, registration.DiscordUsername, registration.CharacterName, registration.Server)
	if err != nil {
		return fmt.Errorf("failed to register character: %v", err)
	}
	return nil
}

// GetCharacter retrieves a character registration for a Discord user
func GetCharacter(db *sql.DB, discordUsername string) (*CharacterRegistration, error) {
	stmt := `SELECT discord_username, character_name, server FROM character_registrations WHERE discord_username = ?`

	registration := &CharacterRegistration{}
	err := db.QueryRow(stmt, discordUsername).Scan(&registration.DiscordUsername, &registration.CharacterName, &registration.Server)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get character: %v", err)
	}
	return registration, nil
}

// IsAdmin checks if a Discord user is an admin
func IsAdmin(db *sql.DB, discordUsername string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM admins WHERE discord_username = ?", discordUsername).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check admin status: %v", err)
	}
	return count > 0, nil
}

// AddAdmin adds a Discord user as an admin
func AddAdmin(db *sql.DB, discordUsername string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO admins (discord_username) VALUES (?)", discordUsername)
	if err != nil {
		return fmt.Errorf("failed to add admin: %v", err)
	}
	return nil
}

// RemoveAdmin removes a Discord user's admin status
func RemoveAdmin(db *sql.DB, discordUsername string) error {
	_, err := db.Exec("DELETE FROM admins WHERE discord_username = ?", discordUsername)
	if err != nil {
		return fmt.Errorf("failed to remove admin: %v", err)
	}
	return nil
}

// RemoveCharacterRegistration removes a character registration for a Discord user
func RemoveCharacterRegistration(db *sql.DB, discordUsername string) error {
	_, err := db.Exec("DELETE FROM character_registrations WHERE discord_username = ?", discordUsername)
	if err != nil {
		return fmt.Errorf("failed to remove character registration: %v", err)
	}
	return nil
}

// GetAllRegistrations retrieves all character registrations
func GetAllRegistrations(db *sql.DB) ([]CharacterRegistration, error) {
	rows, err := db.Query("SELECT discord_username, character_name, server FROM character_registrations")
	if err != nil {
		return nil, fmt.Errorf("failed to get all registrations: %v", err)
	}
	defer rows.Close()

	var registrations []CharacterRegistration
	for rows.Next() {
		var reg CharacterRegistration
		err := rows.Scan(&reg.DiscordUsername, &reg.CharacterName, &reg.Server)
		if err != nil {
			return nil, fmt.Errorf("failed to scan registration: %v", err)
		}
		registrations = append(registrations, reg)
	}
	return registrations, nil
}

// StoreOAuthCharacters stores a user's characters from OAuth
func StoreOAuthCharacters(db *sql.DB, discordUserID string, characters []blizzard.CharacterSummary) error {
	// First, delete any existing characters for this user
	_, err := db.Exec("DELETE FROM oauth_characters WHERE discord_user_id = ?", discordUserID)
	if err != nil {
		return fmt.Errorf("failed to delete existing characters: %v", err)
	}

	// Insert new characters
	for _, char := range characters {
		_, err := db.Exec(`
			INSERT INTO oauth_characters (discord_user_id, character_name, realm_name, realm_slug, level)
			VALUES (?, ?, ?, ?, ?)
		`, discordUserID, char.Name, char.Realm.Name, char.Realm.Slug, char.Level)
		if err != nil {
			return fmt.Errorf("failed to insert character: %v", err)
		}
	}

	return nil
}

// GetOAuthCharacters retrieves a user's characters from OAuth
func GetOAuthCharacters(db *sql.DB, discordUserID string) ([]blizzard.CharacterSummary, error) {
	rows, err := db.Query(`
		SELECT character_name, realm_name, realm_slug, level
		FROM oauth_characters
		WHERE discord_user_id = ?
		ORDER BY level DESC
	`, discordUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query characters: %v", err)
	}
	defer rows.Close()

	var characters []blizzard.CharacterSummary
	for rows.Next() {
		var char blizzard.CharacterSummary
		err := rows.Scan(&char.Name, &char.Realm.Name, &char.Realm.Slug, &char.Level)
		if err != nil {
			return nil, fmt.Errorf("failed to scan character: %v", err)
		}
		characters = append(characters, char)
	}

	return characters, nil
}
