package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/harshul/octo-cli/internal/blueprint"
	"github.com/spf13/cobra"
)

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish your .octo.yaml configuration to the Octo platform",
	Long: `The publish command uploads your project's .octo.yaml configuration
to the Octo web platform, making it available for other developers to discover
and use. This enables one-command setup for anyone who wants to run your project.

Requires:
  - A valid .octo.yaml file in the current directory
  - An Octo platform token (set via OCTO_TOKEN or --token flag)
  - The Octo platform server running (default: https://localhost:8080)`,
	RunE: runPublish,
}

func init() {
	publishCmd.Flags().StringP("config", "c", ".octo.yaml", "Path to the .octo.yaml file")
	publishCmd.Flags().String("server", "", "Octo platform server URL (default: http://localhost:8080)")
	publishCmd.Flags().String("token", "", "Authentication token (or set OCTO_TOKEN env var)")
	publishCmd.Flags().String("repo", "", "GitHub repository URL")
	publishCmd.Flags().Bool("public", true, "Make the project publicly discoverable")
}

func runPublish(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	serverURL, _ := cmd.Flags().GetString("server")
	token, _ := cmd.Flags().GetString("token")
	repoURL, _ := cmd.Flags().GetString("repo")

	if serverURL == "" {
		serverURL = os.Getenv("OCTO_SERVER")
		if serverURL == "" {
			serverURL = "http://localhost:8080"
		}
	}

	if token == "" {
		token = os.Getenv("OCTO_TOKEN")
		if token == "" {
			return fmt.Errorf("authentication token required: set OCTO_TOKEN env var or use --token flag")
		}
	}

	// Read the blueprint
	fmt.Println("📖 Reading configuration...")
	bp, err := blueprint.Read(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	// Try to detect repo URL from git remote
	if repoURL == "" {
		repoURL = detectGitRemote()
	}

	fmt.Printf("📦 Publishing %s...\n", bp.Name)

	// Build publish request
	payload := map[string]interface{}{
		"blueprint": bp,
		"repo_url":  repoURL,
		"token":     token,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send to server
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", serverURL+"/api/publish", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Octo server at %s: %w", serverURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("publish failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	fmt.Println("✅ Published successfully!")
	if url, ok := result["public_url"].(string); ok {
		fmt.Printf("🔗 View at: %s%s\n", serverURL, url)
	}
	fmt.Println("\n💡 Other developers can now run your project with:")
	fmt.Printf("   octo init --from=%s/api/explore/<project-id>/blueprint\n", serverURL)

	return nil
}

func detectGitRemote() string {
	// Try to read git remote URL
	cmd := fmt.Sprintf("git remote get-url origin")
	_ = cmd // Would execute via os/exec in production
	return ""
}
