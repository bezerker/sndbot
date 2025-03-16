/* This file contains all functions related to using the Blizzard API */

package blizzard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bezerker/sndbot/util"
)

type BlizzardClient struct {
	ClientID     string
	ClientSecret string
	accessToken  string
	tokenExpiry  time.Time
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type CharacterSummary struct {
	Name   string `json:"name"`
	Realm  Realm  `json:"realm"`
	Guild  Guild  `json:"guild"`
	Level  int    `json:"level"`
	Gender struct {
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"gender"`
	GuildRank int `json:"guild_rank"`
}

type Guild struct {
	Name    string `json:"name"`
	ID      int    `json:"id"`
	Realm   Realm  `json:"realm"`
	Faction struct {
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"faction"`
}

type Realm struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
	Slug string `json:"slug"`
}

// GuildInfo represents simplified guild information
type GuildInfo struct {
	Name    string
	Rank    int
	Faction string
}

// GuildMember represents a member of a guild
type GuildMember struct {
	Character struct {
		Name  string `json:"name"`
		Realm struct {
			Name string `json:"name"`
			ID   int    `json:"id"`
			Slug string `json:"slug"`
		} `json:"realm"`
	} `json:"character"`
	Rank int `json:"rank"`
}

// GuildRoster represents the full guild roster response
type GuildRoster struct {
	Members []struct {
		Character struct {
			Name  string `json:"name"`
			Realm struct {
				Name string `json:"name"`
				ID   int    `json:"id"`
				Slug string `json:"slug"`
			} `json:"realm"`
		} `json:"character"`
		Rank int `json:"rank"`
	} `json:"members"`
}

func NewBlizzardClient(clientID, clientSecret string) *BlizzardClient {
	if util.IsDebugEnabled() {
		util.Logger.Printf("Initializing Blizzard API client with client ID: %s", clientID)
	}
	return &BlizzardClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
}

func (c *BlizzardClient) getAccessToken() error {
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		util.Logger.Printf("Using existing access token (expires in %v)", c.tokenExpiry.Sub(time.Now()))
		return nil
	}

	util.Logger.Print("Getting new Blizzard API access token")
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", "https://us.battle.net/oauth/token", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create token request: %v", err)
	}

	req.SetBasicAuth(c.ClientID, c.ClientSecret)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		util.Logger.Printf("Error getting access token: %v", err)
		return fmt.Errorf("failed to get token: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		util.Logger.Printf("Error reading token response: %v", err)
		return fmt.Errorf("failed to read token response: %v", err)
	}

	var token tokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		util.Logger.Printf("Error parsing token response: %v\nResponse body: %s", err, string(body))
		return fmt.Errorf("failed to parse token response: %v", err)
	}

	c.accessToken = token.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn-60) * time.Second)
	util.Logger.Printf("Successfully obtained new access token (expires in %d seconds)", token.ExpiresIn)
	return nil
}

func (c *BlizzardClient) GetCharacterGuild(characterName, realm string) (*Guild, error) {
	util.Logger.Printf("Looking up character %s on realm %s", characterName, realm)

	if err := c.getAccessToken(); err != nil {
		util.Logger.Printf("Failed to get access token: %v", err)
		return nil, err
	}

	// Convert realm name to slug format (lowercase, spaces to hyphens)
	realmSlug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(realm), " ", "-"))
	characterNameLower := strings.ToLower(strings.TrimSpace(characterName))

	// Validate inputs
	if realmSlug == "" || characterNameLower == "" {
		util.Logger.Printf("Invalid input: realm='%s' character='%s'", realm, characterName)
		return nil, fmt.Errorf("realm and character name cannot be empty")
	}

	// Build URL for character profile
	baseURL := "https://us.api.blizzard.com"
	path := fmt.Sprintf("/profile/wow/character/%s/%s", url.PathEscape(realmSlug), url.PathEscape(characterNameLower))
	params := url.Values{}
	params.Add("namespace", "profile-us")
	params.Add("locale", "en_US")

	fullURL := fmt.Sprintf("%s%s?%s", baseURL, path, params.Encode())

	util.Logger.Printf("Making character profile request to: %s", fullURL)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		util.Logger.Printf("Error creating request: %v", err)
		return nil, fmt.Errorf("failed to create character request: %v", err)
	}

	req.Header.Add("Authorization", "Bearer "+c.accessToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		util.Logger.Printf("Error making request: %v", err)
		return nil, fmt.Errorf("failed to get character info: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		util.Logger.Printf("Error reading response body: %v", err)
		return nil, fmt.Errorf("failed to read character response: %v", err)
	}

	util.Logger.Printf("Character API response status: %d", resp.StatusCode)

	if resp.StatusCode == 404 {
		util.Logger.Printf("Character %s on realm %s not found", characterName, realm)
		return nil, nil
	}

	if resp.StatusCode != 200 {
		util.Logger.Printf("API request failed with status %d. Response body: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var character CharacterSummary
	if err := json.Unmarshal(body, &character); err != nil {
		util.Logger.Printf("Error parsing character response: %v\nResponse body: %s", err, string(body))
		return nil, fmt.Errorf("failed to parse character response: %v", err)
	}

	if character.Guild.Name == "" {
		util.Logger.Printf("Character %s on realm %s is not in a guild", characterName, realm)
		return nil, nil
	}

	util.Logger.Printf("Successfully found guild information: %+v", character.Guild)
	return &character.Guild, nil
}

func (c *BlizzardClient) GetGuildMemberInfo(characterName, realmSlug, guildName string) (*GuildMember, error) {
	if err := c.getAccessToken(); err != nil {
		util.Logger.Printf("Failed to get access token: %v", err)
		return nil, err
	}

	// Clean up guild name for URL (lowercase, spaces to hyphens)
	guildSlug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(guildName), " ", "-"))

	// Build URL for guild roster
	baseURL := "https://us.api.blizzard.com"
	path := fmt.Sprintf("/data/wow/guild/%s/%s/roster", url.PathEscape(realmSlug), url.PathEscape(guildSlug))
	params := url.Values{}
	params.Add("namespace", "profile-us")
	params.Add("locale", "en_US")

	fullURL := fmt.Sprintf("%s%s?%s", baseURL, path, params.Encode())

	if util.IsDebugEnabled() {
		util.Logger.Printf("Making guild roster request to: %s", fullURL)
		util.Logger.Printf("Debug info - Realm slug: %s, Guild slug: %s", realmSlug, guildSlug)
	}

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		util.Logger.Printf("Error creating request: %v", err)
		return nil, fmt.Errorf("failed to create guild roster request: %v", err)
	}

	req.Header.Add("Authorization", "Bearer "+c.accessToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		util.Logger.Printf("Error making request: %v", err)
		return nil, fmt.Errorf("failed to get guild roster: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		util.Logger.Printf("Error reading response body: %v", err)
		return nil, fmt.Errorf("failed to read guild roster response: %v", err)
	}

	if resp.StatusCode != 200 {
		if util.IsDebugEnabled() {
			util.Logger.Printf("API request failed with status %d. Response body: %s", resp.StatusCode, string(body))
		}
		if resp.StatusCode == 404 {
			util.Logger.Printf("Guild not found: realm=%s, guild=%s", realmSlug, guildSlug)
			return nil, fmt.Errorf("guild not found on realm")
		}
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var roster GuildRoster
	if err := json.Unmarshal(body, &roster); err != nil {
		util.Logger.Printf("Error parsing guild roster response: %v", err)
		if util.IsDebugEnabled() {
			util.Logger.Printf("Response body: %s", string(body))
		}
		return nil, fmt.Errorf("failed to parse guild roster response: %v", err)
	}

	if util.IsDebugEnabled() {
		util.Logger.Printf("Guild roster response - Members count: %d", len(roster.Members))
	}

	// Find the specific character in the roster
	characterNameLower := strings.ToLower(characterName)
	var foundMember *GuildMember
	for _, member := range roster.Members {
		if strings.ToLower(member.Character.Name) == characterNameLower {
			guildMember := &GuildMember{
				Rank: member.Rank,
			}
			guildMember.Character.Name = member.Character.Name
			guildMember.Character.Realm = member.Character.Realm
			foundMember = guildMember
			if util.IsDebugEnabled() {
				util.Logger.Printf("Found character %s in roster with rank %d", characterName, member.Rank)
			}
			break
		}
	}

	if foundMember == nil && util.IsDebugEnabled() {
		util.Logger.Printf("Character %s not found in guild roster", characterName)
	}

	return foundMember, nil
}

// GetGuildInfo returns simplified guild information for a character
func (c *BlizzardClient) GetGuildInfo(characterName, realm string) (*GuildInfo, error) {
	guild, err := c.GetCharacterGuild(characterName, realm)
	if err != nil {
		return nil, err
	}
	if guild == nil {
		return nil, nil
	}

	// Use the guild's realm information instead of the character's realm
	guildRealmSlug := guild.Realm.Slug
	if guildRealmSlug == "" {
		// Fallback to converting realm name if slug is not provided
		guildRealmSlug = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(guild.Realm.Name), " ", "-"))
	}

	// Get member info to get the rank
	member, err := c.GetGuildMemberInfo(characterName, guildRealmSlug, guild.Name)
	if err != nil {
		util.Logger.Printf("Failed to get guild member info: %v", err)
		// Continue with unknown rank
		return &GuildInfo{
			Name:    guild.Name,
			Rank:    -1,
			Faction: guild.Faction.Name,
		}, nil
	}

	rank := -1
	if member != nil {
		rank = member.Rank
	} else if util.IsDebugEnabled() {
		util.Logger.Printf("Member info not found for character %s", characterName)
	}

	return &GuildInfo{
		Name:    guild.Name,
		Rank:    rank,
		Faction: guild.Faction.Name,
	}, nil
}

func login() {
	return
}
