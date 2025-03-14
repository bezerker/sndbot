package database

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type CharacterRegistration struct {
	DiscordUsername string
	CharacterName   string
	Server          string
}

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create the characters table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS characters (
		discord_username TEXT PRIMARY KEY,
		character_name TEXT NOT NULL,
		server TEXT NOT NULL
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return nil, err
	}

	// Create admins table
	createAdminTableSQL := `
	CREATE TABLE IF NOT EXISTS admins (
		discord_username TEXT PRIMARY KEY
	);`

	_, err = db.Exec(createAdminTableSQL)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func RegisterCharacter(db *sql.DB, registration CharacterRegistration) error {
	// Using REPLACE to handle updates of existing registrations
	stmt := `
	REPLACE INTO characters (discord_username, character_name, server)
	VALUES (?, ?, ?)`

	_, err := db.Exec(stmt, registration.DiscordUsername, registration.CharacterName, registration.Server)
	return err
}

func GetCharacter(db *sql.DB, discordUsername string) (*CharacterRegistration, error) {
	stmt := `SELECT discord_username, character_name, server FROM characters WHERE discord_username = ?`

	registration := &CharacterRegistration{}
	err := db.QueryRow(stmt, discordUsername).Scan(&registration.DiscordUsername, &registration.CharacterName, &registration.Server)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return registration, nil
}

func IsAdmin(db *sql.DB, discordUsername string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM admins WHERE discord_username = ?", discordUsername).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func AddAdmin(db *sql.DB, discordUsername string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO admins (discord_username) VALUES (?)", discordUsername)
	return err
}

func RemoveAdmin(db *sql.DB, discordUsername string) error {
	_, err := db.Exec("DELETE FROM admins WHERE discord_username = ?", discordUsername)
	return err
}

func RemoveCharacterRegistration(db *sql.DB, discordUsername string) error {
	_, err := db.Exec("DELETE FROM characters WHERE discord_username = ?", discordUsername)
	return err
}

func GetAllRegistrations(db *sql.DB) ([]CharacterRegistration, error) {
	rows, err := db.Query("SELECT discord_username, character_name, server FROM characters")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var registrations []CharacterRegistration
	for rows.Next() {
		var reg CharacterRegistration
		err := rows.Scan(&reg.DiscordUsername, &reg.CharacterName, &reg.Server)
		if err != nil {
			return nil, err
		}
		registrations = append(registrations, reg)
	}
	return registrations, nil
}
