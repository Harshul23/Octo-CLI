package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config holds server configuration
type Config struct {
	Port           string
	SupabaseURL    string
	SupabaseKey    string
	SupabaseSecret string
	GitHubToken    string
	AllowedOrigins []string
}

// Server represents the Octo web API server
type Server struct {
	config Config
	mux    *http.ServeMux
	db     *SupabaseClient
	gh     *GitHubClient
}

// NewServer creates and configures a new server instance
func NewServer(cfg Config) *Server {
	s := &Server{
		config: cfg,
		mux:    http.NewServeMux(),
		db:     NewSupabaseClient(cfg.SupabaseURL, cfg.SupabaseKey, cfg.SupabaseSecret),
		gh:     NewGitHubClient(cfg.GitHubToken),
	}
	s.registerRoutes()
	return s
}

// LoadConfigFromEnv loads server configuration from environment variables
func LoadConfigFromEnv() Config {
	port := os.Getenv("OCTO_PORT")
	if port == "" {
		port = "8080"
	}

	origins := os.Getenv("OCTO_ALLOWED_ORIGINS")
	allowedOrigins := []string{"http://localhost:5173", "http://localhost:3000"}
	if origins != "" {
		allowedOrigins = strings.Split(origins, ",")
	}

	return Config{
		Port:           port,
		SupabaseURL:    os.Getenv("SUPABASE_URL"),
		SupabaseKey:    os.Getenv("SUPABASE_ANON_KEY"),
		SupabaseSecret: os.Getenv("SUPABASE_SERVICE_KEY"),
		GitHubToken:    os.Getenv("GITHUB_TOKEN"),
		AllowedOrigins: allowedOrigins,
	}
}

// registerRoutes sets up all API routes
func (s *Server) registerRoutes() {
	// Health check
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Analysis endpoints (no auth required — these just clone and analyze repos)
	s.mux.HandleFunc("POST /api/analyze", s.corsMiddleware(s.handleAnalyze))
	s.mux.HandleFunc("POST /api/analyze/github", s.corsMiddleware(s.handleAnalyzeGitHub))

	// Project CRUD endpoints
	s.mux.HandleFunc("GET /api/projects", s.corsMiddleware(s.authMiddleware(s.handleListProjects)))
	s.mux.HandleFunc("GET /api/projects/{id}", s.corsMiddleware(s.authMiddleware(s.handleGetProject)))
	s.mux.HandleFunc("POST /api/projects", s.corsMiddleware(s.authMiddleware(s.handleCreateProject)))
	s.mux.HandleFunc("PUT /api/projects/{id}", s.corsMiddleware(s.authMiddleware(s.handleUpdateProject)))
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.corsMiddleware(s.authMiddleware(s.handleDeleteProject)))

	// Blueprint endpoints
	s.mux.HandleFunc("GET /api/projects/{id}/blueprint", s.corsMiddleware(s.authMiddleware(s.handleGetBlueprint)))
	s.mux.HandleFunc("PUT /api/projects/{id}/blueprint", s.corsMiddleware(s.authMiddleware(s.handleUpdateBlueprint)))

	// Public explore endpoints (no auth required)
	s.mux.HandleFunc("GET /api/explore", s.corsMiddleware(s.handleExplore))
	s.mux.HandleFunc("GET /api/explore/{id}", s.corsMiddleware(s.handleExploreProject))

	// Publish endpoint (from CLI)
	s.mux.HandleFunc("POST /api/publish", s.corsMiddleware(s.handlePublish))

	// GitHub endpoints
	s.mux.HandleFunc("GET /api/github/repos", s.corsMiddleware(s.authMiddleware(s.handleGitHubRepos)))
	s.mux.HandleFunc("GET /api/github/repo/{owner}/{repo}", s.corsMiddleware(s.authMiddleware(s.handleGitHubRepoInfo)))

	// Auth callback
	s.mux.HandleFunc("POST /api/auth/github", s.corsMiddleware(s.handleGitHubAuth))

	// Serve preflight CORS requests for all API routes
	s.mux.HandleFunc("OPTIONS /api/", s.handleCORS)

	// Serve the web dashboard (static files)
	webDir := os.Getenv("OCTO_WEB_DIR")
	if webDir == "" {
		webDir = "./web/dist"
	}
	if _, err := os.Stat(webDir); err == nil {
		fs := http.FileServer(http.Dir(webDir))
		s.mux.Handle("GET /", fs)
	}
}

// Start begins listening for HTTP requests
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%s", s.config.Port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("🐙 Octo API server starting on http://localhost:%s", s.config.Port)
	log.Printf("   Dashboard: http://localhost:%s", s.config.Port)
	log.Printf("   API:       http://localhost:%s/api", s.config.Port)

	return srv.ListenAndServe()
}

// --- Middleware ---

func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		for _, o := range s.config.AllowedOrigins {
			if o == origin || o == "*" {
				allowed = true
				break
			}
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Octo-Token")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Missing authorization header"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid authorization format"})
			return
		}

		// Verify with Supabase
		user, err := s.db.VerifyToken(token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid or expired token"})
			return
		}

		// Inject user into request context
		r = r.WithContext(withUser(r.Context(), user))
		next(w, r)
	}
}

func (s *Server) handleCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	for _, o := range s.config.AllowedOrigins {
		if o == origin || o == "*" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			break
		}
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Octo-Token")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
"status":  "ok",
"version": "0.1.0",
"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
