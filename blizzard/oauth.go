package blizzard

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

type UserInfo struct {
	ID        int    `json:"id"`
	Battletag string `json:"battletag"`
}

// GenerateState generates a random state string for OAuth
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GetAuthURL generates the OAuth URL for Blizzard authentication
func GetAuthURL(clientID, redirectURI, state string) string {
	scopes := []string{
		"wow.profile",
		"wow.character",
		"wow.read",
	}
	scopeStr := strings.Join(scopes, " ")

	return fmt.Sprintf(
		"https://oauth.battle.net/authorize?client_id=%s&scope=%s&redirect_uri=%s&response_type=code&state=%s",
		clientID,
		url.QueryEscape(scopeStr),
		url.QueryEscape(redirectURI),
		state,
	)
}

// ExchangeCodeForToken exchanges the authorization code for an access token
func ExchangeCodeForToken(code, clientID, clientSecret, redirectURI string) (*OAuthToken, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest("POST", "https://oauth.battle.net/token", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %v", err)
	}

	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.URL.RawQuery = data.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var token OAuthToken
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %v", err)
	}

	return &token, nil
}

// GetUserInfo retrieves the user's information from Blizzard
func GetUserInfo(accessToken string) (*UserInfo, error) {
	req, err := http.NewRequest("GET", "https://oauth.battle.net/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create userinfo request: %v", err)
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read userinfo response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var userInfo UserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse userinfo response: %v", err)
	}

	return &userInfo, nil
}

// GetUserCharacters retrieves the user's characters from Blizzard
func GetUserCharacters(accessToken string) ([]CharacterSummary, error) {
	url := "https://us.api.blizzard.com/profile/user/wow?namespace=profile-us&locale=en_US"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create characters request: %v", err)
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get characters: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read characters response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("characters request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Accounts []struct {
			Characters []CharacterSummary `json:"characters"`
		} `json:"wow_accounts"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse characters response: %v", err)
	}

	if len(response.Accounts) == 0 {
		return nil, fmt.Errorf("no WoW accounts found")
	}

	return response.Accounts[0].Characters, nil
}
