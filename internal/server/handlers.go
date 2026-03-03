package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/harshul/octo-cli/internal/analyzer"
	"github.com/harshul/octo-cli/internal/blueprint"
)

// --- Request/Response Types ---

type AnalyzeRequest struct {
	Path string `json:"path"`
}

type AnalyzeGitHubRequest struct {
	RepoURL   string `json:"repo_url"`
	Branch    string `json:"branch"`
	UserToken string `json:"user_token,omitempty"` // GitHub OAuth token for private repos
}

type CreateProjectRequest struct {
	Name        string              `json:"name"`
	RepoURL     string              `json:"repo_url"`
	Description string              `json:"description"`
	IsPublic    bool                `json:"is_public"`
	Blueprint   *blueprint.Blueprint `json:"blueprint,omitempty"`
}

type UpdateProjectRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsPublic    *bool   `json:"is_public,omitempty"`
}

type UpdateBlueprintRequest struct {
	Blueprint blueprint.Blueprint `json:"blueprint"`
}

type PublishRequest struct {
	RepoURL   string              `json:"repo_url"`
	Blueprint blueprint.Blueprint `json:"blueprint"`
	Token     string              `json:"token"`
}

type AnalysisResponse struct {
	Success   bool                 `json:"success"`
	Project   *analyzer.ProjectInfo `json:"project,omitempty"`
	Blueprint *blueprint.Blueprint  `json:"blueprint,omitempty"`
	Error     string               `json:"error,omitempty"`
	Duration  string               `json:"duration,omitempty"`
}

// --- Analysis Handlers ---

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Path is required"})
		return
	}

	start := time.Now()

	// Use the existing analyzer
	info, err := analyzer.AnalyzeProject(req.Path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, AnalysisResponse{
			Success: false,
			Error:   fmt.Sprintf("Analysis failed: %v", err),
		})
		return
	}

	// Convert to blueprint
	bp := blueprint.FromProjectInfo(info)

	writeJSON(w, http.StatusOK, AnalysisResponse{
		Success:   true,
		Project:   &info,
		Blueprint: &bp,
		Duration:  time.Since(start).String(),
	})
}

func (s *Server) handleAnalyzeGitHub(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeGitHubRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.RepoURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repo_url is required"})
		return
	}

	branch := req.Branch
	if branch == "" {
		branch = "main"
	}

	start := time.Now()

	// Clone the repo to a temp directory
	tmpDir, err := os.MkdirTemp("", "octo-analyze-*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, AnalysisResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create temp directory: %v", err),
		})
		return
	}
	defer os.RemoveAll(tmpDir)

	// If a user token is provided, inject it into the URL for private repo access
	cloneURL := req.RepoURL
	if req.UserToken != "" && strings.Contains(cloneURL, "github.com") {
		cloneURL = strings.Replace(cloneURL, "https://github.com", "https://"+req.UserToken+"@github.com", 1)
	}

	// Shallow clone for speed
	cloneCmd := exec.Command("git", "clone", "--depth", "1", "--branch", branch, cloneURL, tmpDir)
	if output, err := cloneCmd.CombinedOutput(); err != nil {
		// Try without branch (maybe default is not "main")
		tmpDir2, _ := os.MkdirTemp("", "octo-analyze-*")
		defer os.RemoveAll(tmpDir2)
		cloneCmd2 := exec.Command("git", "clone", "--depth", "1", cloneURL, tmpDir2)
		if output2, err2 := cloneCmd2.CombinedOutput(); err2 != nil {
			writeJSON(w, http.StatusBadRequest, AnalysisResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to clone repo: %s %s", string(output), string(output2)),
			})
			return
		}
		tmpDir = tmpDir2
	}

	// Run analysis on the cloned repo
	info, err := analyzer.AnalyzeProject(tmpDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, AnalysisResponse{
			Success: false,
			Error:   fmt.Sprintf("Analysis failed: %v", err),
		})
		return
	}

	// Fix the name from the repo URL
	info.Name = extractRepoName(req.RepoURL)

	bp := blueprint.FromProjectInfo(info)
	bp.Name = info.Name

	writeJSON(w, http.StatusOK, AnalysisResponse{
		Success:   true,
		Project:   &info,
		Blueprint: &bp,
		Duration:  time.Since(start).String(),
	})
}

// --- Project CRUD Handlers ---

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	projects, err := s.db.ListProjects(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"projects": projects,
		"total":    len(projects),
	})
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user := userFromContext(r.Context())

	project, err := s.db.GetProject(id, user.ID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Project not found"})
		return
	}

	writeJSON(w, http.StatusOK, project)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Name is required"})
		return
	}

	project := &Project{
		Name:        req.Name,
		RepoURL:     req.RepoURL,
		Description: req.Description,
		IsPublic:    req.IsPublic,
		UserID:      user.ID,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	if req.Blueprint != nil {
		project.Blueprint = req.Blueprint
	}

	created, err := s.db.CreateProject(project)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user := userFromContext(r.Context())

	var req UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	updated, err := s.db.UpdateProject(id, user.ID, req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user := userFromContext(r.Context())

	if err := s.db.DeleteProject(id, user.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Project deleted"})
}

// --- Blueprint Handlers ---

func (s *Server) handleGetBlueprint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user := userFromContext(r.Context())

	project, err := s.db.GetProject(id, user.ID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Project not found"})
		return
	}

	if project.Blueprint == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "No blueprint found for this project"})
		return
	}

	writeJSON(w, http.StatusOK, project.Blueprint)
}

func (s *Server) handleUpdateBlueprint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user := userFromContext(r.Context())

	var req UpdateBlueprintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	err := s.db.UpdateBlueprint(id, user.ID, &req.Blueprint)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Blueprint updated"})
}

// --- Explore Handlers (Public) ---

func (s *Server) handleExplore(w http.ResponseWriter, r *http.Request) {
	language := r.URL.Query().Get("language")
	search := r.URL.Query().Get("search")
	sortBy := r.URL.Query().Get("sort")

	projects, err := s.db.ListPublicProjects(language, search, sortBy)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"projects": projects,
		"total":    len(projects),
	})
}

func (s *Server) handleExploreProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	project, err := s.db.GetPublicProject(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Project not found"})
		return
	}

	writeJSON(w, http.StatusOK, project)
}

// --- Publish Handler (from CLI) ---

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	var req PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Token is required"})
		return
	}

	// Verify token
	user, err := s.db.VerifyToken(req.Token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
		return
	}

	// Create or update the project
	project := &Project{
		Name:        req.Blueprint.Name,
		RepoURL:     req.RepoURL,
		Blueprint:   &req.Blueprint,
		UserID:      user.ID,
		IsPublic:    true,
		Language:    req.Blueprint.Language,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	created, err := s.db.UpsertProject(project)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Published successfully",
		"project":    created,
		"public_url": fmt.Sprintf("/explore/%s", created.ID),
	})
}

// --- GitHub Handlers ---

func (s *Server) handleGitHubRepos(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	if user == nil || user.GitHubToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "GitHub not connected"})
		return
	}

	repos, err := s.gh.ListUserRepos(user.GitHubToken)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"repos": repos,
	})
}

func (s *Server) handleGitHubRepoInfo(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")

	info, err := s.gh.GetRepoInfo(owner, repo)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleGitHubAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	log.Printf("GitHub auth callback received (code exchange handled by Supabase)")
	writeJSON(w, http.StatusOK, map[string]string{"message": "Auth handled by Supabase client"})
}

// --- Helpers ---

func extractRepoName(repoURL string) string {
	// Extract "repo-name" from "https://github.com/user/repo-name.git"
	name := filepath.Base(repoURL)
	name = strings.TrimSuffix(name, ".git")
	if name == "" || name == "." {
		return "unknown-project"
	}
	return name
}
