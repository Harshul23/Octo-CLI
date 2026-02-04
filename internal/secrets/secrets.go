package secrets

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// EnvVar represents a detected environment variable
type EnvVar struct {
	Name     string
	File     string // File where it was found
	Line     int    // Line number where it was found
	Language string // Language/pattern that matched
	Required bool   // Whether the variable is required (true by default)
}

// EnvStatus represents the status of environment variables
type EnvStatus struct {
	Required   []EnvVar        // All detected env vars from code
	Defined    map[string]bool // Vars defined in .env files
	Missing    []EnvVar        // Vars that are required but not defined
	EnvFile    string          // Path to the .env file
	HasEnvFile bool            // Whether .env file exists
}

// Patterns for detecting environment variable usage in different languages
var envPatterns = map[string]*regexp.Regexp{
	// JavaScript/TypeScript: process.env.VAR_NAME or process.env['VAR_NAME']
	"node": regexp.MustCompile(`process\.env\.([A-Z][A-Z0-9_]*)|process\.env\[['"]([A-Z][A-Z0-9_]*)['"]\]`),

	// Python: os.environ['VAR'], os.environ.get('VAR'), os.getenv('VAR')
	"python": regexp.MustCompile(`os\.environ(?:\.get)?\[?['\"]([A-Z][A-Z0-9_]*)['"]\]?|os\.getenv\(['\"]([A-Z][A-Z0-9_]*)['"]\)`),

	// Java: System.getenv("VAR")
	"java": regexp.MustCompile(`System\.getenv\(['\"]([A-Z][A-Z0-9_]*)['"]\)`),

	// Go: os.Getenv("VAR")
	"go": regexp.MustCompile(`os\.Getenv\(['\"]([A-Z][A-Z0-9_]*)['"]\)`),

	// Ruby: ENV['VAR'] or ENV["VAR"]
	"ruby": regexp.MustCompile(`ENV\[['\"]([A-Z][A-Z0-9_]*)['"]\]`),

	// Rust: std::env::var("VAR") or env::var("VAR")
	"rust": regexp.MustCompile(`(?:std::)?env::var\(['\"]([A-Z][A-Z0-9_]*)['"]\)`),

	// Generic .env reference pattern (for config files)
	"generic": regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)\}|\$([A-Z][A-Z0-9_]*)`),
}

// File extensions to scan for each language
var languageExtensions = map[string][]string{
	"node":   {".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs"},
	"python": {".py"},
	"java":   {".java"},
	"go":     {".go"},
	"ruby":   {".rb"},
	"rust":   {".rs"},
}

// Common env vars to ignore (usually system-provided)
var ignoredEnvVars = map[string]bool{
	"PATH":           true,
	"HOME":           true,
	"USER":           true,
	"NODE_ENV":       true,
	"LANG":           true,
	"SHELL":          true,
	"PWD":            true,
	"TERM":           true,
	"EDITOR":         true,
	"TMPDIR":         true,
	"TMP":            true,
	"TEMP":           true,
	"HOSTNAME":       true,
	"PORT":           true,
	"HOST":           true,
	"DEBUG":          true,
	"VERBOSE":        true,
	"CI":             true,
	"GITHUB_ACTIONS": true,
	"VERCEL":         true,
	"NETLIFY":        true,
}

// ScanForEnvVars scans the project directory for environment variable usage
func ScanForEnvVars(projectPath string, language string) ([]EnvVar, error) {
	var envVars []EnvVar
	seen := make(map[string]bool)

	// Determine which patterns to use based on language
	patterns := getPatterns(language)

	// Walk the directory
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip common non-source directories
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == "vendor" ||
				name == "target" || name == "build" || name == "dist" ||
				name == "__pycache__" || name == ".venv" || name == "venv" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if this file should be scanned
		ext := filepath.Ext(path)
		if !shouldScanFile(ext, language) {
			return nil
		}

		// Scan the file
		fileVars, err := scanFile(path, patterns)
		if err != nil {
			return nil // Skip files we can't read
		}

		// Add unique vars
		for _, v := range fileVars {
			if !seen[v.Name] && !ignoredEnvVars[v.Name] {
				seen[v.Name] = true
				envVars = append(envVars, v)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Check for defaults in .env.example or similar
	defaults := checkEnvExample(projectPath)
	hasKubeConfig := kubeConfigExists()

	// Post-process vars to determine if they are required
	for i := range envVars {
		// Default to optional - let user decide what's truly required
		envVars[i].Required = false

		// Only mark as required if it looks like a critical secret
		// (contains KEY, SECRET, TOKEN, PASSWORD, etc.) AND has no default
		if isCriticalEnvVar(envVars[i].Name) {
			// Check if it has a default in example file
			if hasDefault, ok := defaults[envVars[i].Name]; !ok || !hasDefault {
				envVars[i].Required = true
			}
		}

		// Heuristic: KUBECONFIG is always optional if local config exists
		if envVars[i].Name == "KUBECONFIG" && hasKubeConfig {
			envVars[i].Required = false
		}
	}

	// Sort by name for consistent output
	sort.Slice(envVars, func(i, j int) bool {
		return envVars[i].Name < envVars[j].Name
	})

	return envVars, nil
}

// checkEnvExample looks for example env files and returns vars with defaults
func checkEnvExample(root string) map[string]bool {
	defaults := make(map[string]bool)
	candidates := []string{".env.example", ".env.sample", ".env.template", ".env.defaults"}

	for _, name := range candidates {
		path := filepath.Join(root, name)
		vars, err := ReadEnvFile(path)
		if err == nil {
			for k, v := range vars {
				if v != "" {
					defaults[k] = true
				}
			}
		}
	}
	return defaults
}

// kubeConfigExists checks if ~/.kube/config exists
func kubeConfigExists() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	path := filepath.Join(home, ".kube", "config")
	_, err = os.Stat(path)
	return err == nil
}

// isCriticalEnvVar checks if the variable name suggests it's a critical secret
func isCriticalEnvVar(name string) bool {
	criticalPatterns := []string{
		"API_KEY", "APIKEY", "SECRET", "TOKEN", "PASSWORD", "PASSWD",
		"PRIVATE_KEY", "AUTH", "CREDENTIAL", "ACCESS_KEY",
	}
	upper := strings.ToUpper(name)
	for _, pattern := range criticalPatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

// getPatterns returns the appropriate regex patterns for a language
func getPatterns(language string) map[string]*regexp.Regexp {
	patterns := make(map[string]*regexp.Regexp)

	lang := strings.ToLower(language)
	if p, ok := envPatterns[lang]; ok {
		patterns[lang] = p
	}

	// Also add generic pattern
	patterns["generic"] = envPatterns["generic"]

	// For Node, also check for generic patterns in config files
	if lang == "node" || lang == "unknown" {
		patterns["node"] = envPatterns["node"]
	}

	return patterns
}

// shouldScanFile checks if a file should be scanned based on extension
func shouldScanFile(ext string, language string) bool {
	lang := strings.ToLower(language)

	// Always scan certain config files
	if ext == ".env" || ext == ".yaml" || ext == ".yml" || ext == ".json" {
		return true
	}

	if exts, ok := languageExtensions[lang]; ok {
		for _, e := range exts {
			if ext == e {
				return true
			}
		}
	}

	// For unknown language, scan common extensions
	if lang == "unknown" {
		for _, exts := range languageExtensions {
			for _, e := range exts {
				if ext == e {
					return true
				}
			}
		}
	}

	return false
}

// scanFile scans a single file for environment variable usage
func scanFile(path string, patterns map[string]*regexp.Regexp) ([]EnvVar, error) {
	var vars []EnvVar

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for lang, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				// Extract the variable name from capture groups
				varName := ""
				for i := 1; i < len(match); i++ {
					if match[i] != "" {
						varName = match[i]
						break
					}
				}
				if varName != "" {
					vars = append(vars, EnvVar{
						Name:     varName,
						File:     path,
						Line:     lineNum,
						Language: lang,
					})
				}
			}
		}
	}

	return vars, scanner.Err()
}

// ReadEnvFile reads an .env file and returns defined variables
func ReadEnvFile(envPath string) (map[string]string, error) {
	vars := make(map[string]string)

	file, err := os.Open(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return vars, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			vars[key] = value
		}
	}

	return vars, scanner.Err()
}

// CheckEnvStatus checks which required env vars are defined
func CheckEnvStatus(projectPath string, language string) (EnvStatus, error) {
	status := EnvStatus{
		Defined: make(map[string]bool),
		EnvFile: filepath.Join(projectPath, ".env"),
	}

	// Scan for required env vars
	required, err := ScanForEnvVars(projectPath, language)
	if err != nil {
		return status, err
	}
	status.Required = required

	// Check if .env file exists
	if _, err := os.Stat(status.EnvFile); err == nil {
		status.HasEnvFile = true
	}

	// Read existing .env file
	envVars, err := ReadEnvFile(status.EnvFile)
	if err != nil {
		return status, err
	}

	// Also check .env.local
	localEnvPath := filepath.Join(projectPath, ".env.local")
	localVars, _ := ReadEnvFile(localEnvPath)
	for k, v := range localVars {
		envVars[k] = v
	}

	// Mark which vars are defined
	for k := range envVars {
		status.Defined[k] = true
	}

	// Find missing vars
	for _, v := range required {
		if !status.Defined[v.Name] {
			status.Missing = append(status.Missing, v)
		}
	}

	return status, nil
}

// WriteEnvFile creates or updates an .env file with the provided values
func WriteEnvFile(envPath string, values map[string]string) error {
	// Read existing content if file exists
	existingVars, _ := ReadEnvFile(envPath)

	// Merge with new values
	for k, v := range values {
		existingVars[k] = v
	}

	// Create the file
	file, err := os.Create(envPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header comment
	fmt.Fprintln(file, "# Environment variables for this project")
	fmt.Fprintln(file, "# Generated by Octo CLI")
	fmt.Fprintln(file, "")

	// Sort keys for consistent output
	var keys []string
	for k := range existingVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Write variables
	for _, k := range keys {
		v := existingVars[k]
		// Quote values that contain spaces or special characters
		if strings.ContainsAny(v, " \t\n\"'") {
			v = fmt.Sprintf(`"%s"`, strings.ReplaceAll(v, `"`, `\"`))
		}
		fmt.Fprintf(file, "%s=%s\n", k, v)
	}

	return nil
}

// AppendToEnvFile appends new values to an existing .env file
func AppendToEnvFile(envPath string, values map[string]string) error {
	// Open file in append mode, create if not exists
	file, err := os.OpenFile(envPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check if file is empty or doesn't end with newline
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 0 {
		fmt.Fprintln(file, "") // Add newline before new entries
	}

	// Sort keys for consistent output
	var keys []string
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Append new variables
	for _, k := range keys {
		v := values[k]
		if strings.ContainsAny(v, " \t\n\"'") {
			v = fmt.Sprintf(`"%s"`, strings.ReplaceAll(v, `"`, `\"`))
		}
		fmt.Fprintf(file, "%s=%s\n", k, v)
	}

	return nil
}

// GetEnvVarDescription provides helpful descriptions for common env var patterns
func GetEnvVarDescription(varName string) string {
	varLower := strings.ToLower(varName)

	switch {
	case strings.Contains(varLower, "api_key") || strings.Contains(varLower, "apikey"):
		return "API key for external service"
	case strings.Contains(varLower, "secret"):
		return "Secret key (keep confidential)"
	case strings.Contains(varLower, "database") || strings.Contains(varLower, "db_"):
		return "Database connection string or credential"
	case strings.Contains(varLower, "password") || strings.Contains(varLower, "passwd"):
		return "Password (keep confidential)"
	case strings.Contains(varLower, "token"):
		return "Authentication token"
	case strings.Contains(varLower, "url") || strings.Contains(varLower, "uri"):
		return "URL or endpoint"
	case strings.Contains(varLower, "host"):
		return "Hostname or IP address"
	case strings.Contains(varLower, "port"):
		return "Port number"
	case strings.Contains(varLower, "email") || strings.Contains(varLower, "mail"):
		return "Email address or mail configuration"
	case strings.Contains(varLower, "aws"):
		return "AWS credential or configuration"
	case strings.Contains(varLower, "stripe"):
		return "Stripe payment API key"
	case strings.Contains(varLower, "twilio"):
		return "Twilio API credential"
	case strings.Contains(varLower, "sendgrid"):
		return "SendGrid email API key"
	case strings.Contains(varLower, "redis"):
		return "Redis connection configuration"
	case strings.Contains(varLower, "mongo"):
		return "MongoDB connection string"
	case strings.Contains(varLower, "postgres") || strings.Contains(varLower, "pg_"):
		return "PostgreSQL connection configuration"
	case strings.Contains(varLower, "mysql"):
		return "MySQL connection configuration"
	default:
		return "Environment variable"
	}
}
