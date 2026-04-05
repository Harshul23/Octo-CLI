package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// GitHubOAuthConfig holds GitHub OAuth configuration
type GitHubOAuthConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
}

// GitHubOAuthHandler handles GitHub OAuth flow
type GitHubOAuthHandler struct {
	config GitHubOAuthConfig
	db     *NeonDB
}

// NewGitHubOAuthHandler creates a new GitHub OAuth handler
func NewGitHubOAuthHandler(config GitHubOAuthConfig, db *NeonDB) *GitHubOAuthHandler {
	return &GitHubOAuthHandler{
		config: config,
		db:     db,
	}
}

// GetAuthorizationURL generates the GitHub OAuth authorization URL
func (h *GitHubOAuthHandler) GetAuthorizationURL(state string) string {
	params := url.Values{}
	params.Add("client_id", h.config.ClientID)
	params.Add("redirect_uri", h.config.CallbackURL)
	params.Add("scope", "read:user user:email")
	params.Add("state", state)

	return fmt.Sprintf("https://github.com/login/oauth/authorize?%s", params.Encode())
}

// ExchangeCodeForToken exchanges an authorization code for an access token
func (h *GitHubOAuthHandler) ExchangeCodeForToken(code string) (string, error) {
	data := map[string]string{
		"client_id":     h.config.ClientID,
		"client_secret": h.config.ClientSecret,
		"code":          code,
	}

	jsonData, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("github oauth error: %s - %s", result.Error, result.Description)
	}

	if result.AccessToken == "" {
		return "", fmt.Errorf("no access token returned from GitHub")
	}

	return result.AccessToken, nil
}

// GetGitHubUser fetches user information from GitHub API
func (h *GitHubOAuthHandler) GetGitHubUser(accessToken string) (*GitHubUserInfo, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error (status %d): %s", resp.StatusCode, string(body))
	}

	var userInfo GitHubUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// GetGitHubUserEmails fetches user email addresses from GitHub API
func (h *GitHubOAuthHandler) GetGitHubUserEmails(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch user emails")
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	// Return the primary verified email
	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}

	// Fallback to first verified email
	for _, email := range emails {
		if email.Verified {
			return email.Email, nil
		}
	}

	// Last resort: any email
	if len(emails) > 0 {
		return emails[0].Email, nil
	}

	return "", nil
}

// GitHubUserInfo represents GitHub user information
type GitHubUserInfo struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}
