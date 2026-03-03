package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GitHubRepo represents a GitHub repository
type GitHubRepo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	CloneURL    string `json:"clone_url"`
	Language    string `json:"language"`
	Stars       int    `json:"stargazers_count"`
	Forks       int    `json:"forks_count"`
	Private     bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	UpdatedAt   string `json:"updated_at"`
}

// GitHubClient handles GitHub API interactions
type GitHubClient struct {
	defaultToken string
	httpClient   *http.Client
}

// NewGitHubClient creates a new GitHub API client
func NewGitHubClient(defaultToken string) *GitHubClient {
	return &GitHubClient{
		defaultToken: defaultToken,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// ListUserRepos fetches repositories for the authenticated user
func (g *GitHubClient) ListUserRepos(token string) ([]GitHubRepo, error) {
	if token == "" {
		token = g.defaultToken
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user/repos?sort=updated&per_page=50&type=all", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error (status %d)", resp.StatusCode)
	}

	var repos []GitHubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, err
	}

	return repos, nil
}

// GetRepoInfo fetches details for a specific repository
func (g *GitHubClient) GetRepoInfo(owner, repo string) (*GitHubRepo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if g.defaultToken != "" {
		req.Header.Set("Authorization", "Bearer "+g.defaultToken)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error (status %d)", resp.StatusCode)
	}

	var ghRepo GitHubRepo
	if err := json.NewDecoder(resp.Body).Decode(&ghRepo); err != nil {
		return nil, err
	}

	return &ghRepo, nil
}
