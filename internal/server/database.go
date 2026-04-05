package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/harshul/octo-cli/internal/blueprint"
)

// --- Data Models ---

// User represents an authenticated user
type User struct {
	ID             string `json:"id"`
	GitHubID       string `json:"github_id"`
	Email          string `json:"email"`
	Name           string `json:"name"`
	AvatarURL      string `json:"avatar_url"`
	GitHubUsername string `json:"github_username"`
	GitHubToken    string `json:"github_token,omitempty"`
}

// Project represents a saved project configuration
type Project struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	RepoURL     string               `json:"repo_url"`
	Description string               `json:"description"`
	Language    string               `json:"language"`
	IsPublic    bool                 `json:"is_public"`
	UserID      string               `json:"user_id"`
	UserName    string               `json:"user_name,omitempty"`
	AvatarURL   string               `json:"avatar_url,omitempty"`
	Blueprint   *blueprint.Blueprint `json:"blueprint,omitempty"`
	Stars       int                  `json:"stars"`
	CreatedAt   string               `json:"created_at"`
	UpdatedAt   string               `json:"updated_at"`
}

// SupabaseClient handles all database operations via Supabase REST API
type SupabaseClient struct {
	url        string
	anonKey    string
	serviceKey string
	httpClient *http.Client
}

// NewSupabaseClient creates a new Supabase client
func NewSupabaseClient(url, anonKey, serviceKey string) *SupabaseClient {
	return &SupabaseClient{
		url:        url,
		anonKey:    anonKey,
		serviceKey: serviceKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// VerifyToken verifies a Supabase JWT token and returns the user
func (c *SupabaseClient) VerifyToken(token string) (*User, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/auth/v1/user", c.url), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("apikey", c.anonKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid token (status %d)", resp.StatusCode)
	}

	var result struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		UserMetadata  struct {
			FullName  string `json:"full_name"`
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user_metadata"`
		Identities []struct {
			Provider    string `json:"provider"`
			IdentityData struct {
				AccessToken string `json:"provider_token"`
			} `json:"identity_data"`
		} `json:"identities"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	name := result.UserMetadata.FullName
	if name == "" {
		name = result.UserMetadata.Name
	}

	user := &User{
		ID:        result.ID,
		Email:     result.Email,
		Name:      name,
		AvatarURL: result.UserMetadata.AvatarURL,
	}

	// Extract GitHub token if available
	for _, id := range result.Identities {
		if id.Provider == "github" {
			user.GitHubToken = id.IdentityData.AccessToken
			break
		}
	}

	return user, nil
}

// --- Project Operations ---

func (c *SupabaseClient) ListProjects(userID string) ([]Project, error) {
	url := fmt.Sprintf("%s/rest/v1/projects?user_id=eq.%s&order=updated_at.desc", c.url, userID)
	return c.queryProjects(url)
}

func (c *SupabaseClient) GetProject(id, userID string) (*Project, error) {
	url := fmt.Sprintf("%s/rest/v1/projects?id=eq.%s&user_id=eq.%s", c.url, id, userID)
	projects, err := c.queryProjects(url)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("project not found")
	}
	return &projects[0], nil
}

func (c *SupabaseClient) CreateProject(p *Project) (*Project, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/rest/v1/projects", c.url), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, true)
	req.Header.Set("Prefer", "return=representation")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create project (status %d): %s", resp.StatusCode, string(respBody))
	}

	var projects []Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("no project returned")
	}
	return &projects[0], nil
}

func (c *SupabaseClient) UpdateProject(id, userID string, update UpdateProjectRequest) (*Project, error) {
	body, err := json.Marshal(update)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/rest/v1/projects?id=eq.%s&user_id=eq.%s", c.url, id, userID)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, true)
	req.Header.Set("Prefer", "return=representation")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var projects []Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("project not found")
	}
	return &projects[0], nil
}

func (c *SupabaseClient) DeleteProject(id, userID string) error {
	url := fmt.Sprintf("%s/rest/v1/projects?id=eq.%s&user_id=eq.%s", c.url, id, userID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req, true)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *SupabaseClient) UpdateBlueprint(id, userID string, bp *blueprint.Blueprint) error {
	body, _ := json.Marshal(map[string]interface{}{
		"blueprint":  bp,
		"language":   bp.Language,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})

	url := fmt.Sprintf("%s/rest/v1/projects?id=eq.%s&user_id=eq.%s", c.url, id, userID)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	c.setHeaders(req, true)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *SupabaseClient) UpsertProject(p *Project) (*Project, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/rest/v1/projects", c.url), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, true)
	req.Header.Set("Prefer", "return=representation,resolution=merge-duplicates")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var projects []Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("no project returned")
	}
	return &projects[0], nil
}

// --- Public Project Operations ---

func (c *SupabaseClient) ListPublicProjects(language, search, sortBy string) ([]Project, error) {
	url := fmt.Sprintf("%s/rest/v1/projects?is_public=eq.true", c.url)

	if language != "" {
		url += fmt.Sprintf("&language=eq.%s", language)
	}
	if search != "" {
		url += fmt.Sprintf("&name=ilike.*%s*", search)
	}

	switch sortBy {
	case "stars":
		url += "&order=stars.desc"
	case "newest":
		url += "&order=created_at.desc"
	default:
		url += "&order=updated_at.desc"
	}

	url += "&limit=50"
	return c.queryProjects(url)
}

func (c *SupabaseClient) GetPublicProject(id string) (*Project, error) {
	url := fmt.Sprintf("%s/rest/v1/projects?id=eq.%s&is_public=eq.true", c.url, id)
	projects, err := c.queryProjects(url)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("project not found")
	}
	return &projects[0], nil
}

// --- Helpers ---

func (c *SupabaseClient) queryProjects(url string) ([]Project, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, false)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var projects []Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, err
	}
	return projects, nil
}

func (c *SupabaseClient) setHeaders(req *http.Request, useServiceKey bool) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.anonKey)
	if useServiceKey && c.serviceKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.anonKey)
	}
}
