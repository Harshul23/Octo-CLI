package main

import (
	"fmt"

	"github.com/harshul/octo-cli/internal/server"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Octo web dashboard and API server",
	Long: `The serve command starts a local HTTP server that provides:
- A web-based dashboard for managing projects
- REST API endpoints for analyzing codebases
- GitHub integration for importing repositories
- Blueprint management and sharing

The server connects to Supabase for authentication and data storage.
Set environment variables before starting:
  SUPABASE_URL, SUPABASE_ANON_KEY, SUPABASE_SERVICE_KEY, GITHUB_TOKEN`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringP("port", "p", "8080", "Port to listen on")
	serveCmd.Flags().String("web-dir", "./web/dist", "Path to web dashboard static files")
}

func runServe(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetString("port")
	webDir, _ := cmd.Flags().GetString("web-dir")

	cfg := server.LoadConfigFromEnv()
	if port != "" {
		cfg.Port = port
	}

	if webDir != "" {
		// Set for the server to pick up
		fmt.Printf("🐙 Web dashboard directory: %s\n", webDir)
	}

	if cfg.DatabaseURL == "" {
		fmt.Println("⚠️  DATABASE_URL not set — database features will be unavailable")
		fmt.Println("   See NEON_SETUP.md for Neon configuration instructions")
		return fmt.Errorf("DATABASE_URL is required")
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	return srv.Start()
}
