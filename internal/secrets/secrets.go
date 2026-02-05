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
	Name         string
	File         string // File where it was found
	Line         int    // Line number where it was found
	Language     string // Language/pattern that matched
	Required     bool   // Whether the variable is required (true by default)
	DefaultValue string // Default or suggested value from README/example files
	TargetDir    string // Target directory for the .env file (e.g., "apps/client")
}

// ReadmeEnvConfig represents environment variable configuration from README
type ReadmeEnvConfig struct {
	Name         string
	Value        string
	TargetDir    string // Where to write this env var (e.g., "apps/client", "apps/server")
	Description  string // Optional description from README context
}

// EnvFileTarget represents a target .env file with its variables
type EnvFileTarget struct {
	Path      string            // Relative path to the .env file
	AbsPath   string            // Absolute path to the .env file
	Variables []ReadmeEnvConfig // Variables to write to this file
	Exists    bool              // Whether the file already exists
}

// EnvStatus represents the status of environment variables
type EnvStatus struct {
	Required      []EnvVar              // All detected env vars from code
	Defined       map[string]bool       // Vars defined in .env files
	Missing       []EnvVar              // Vars that are required but not defined
	EnvFile       string                // Path to the .env file
	HasEnvFile    bool                  // Whether .env file exists
	ReadmeDefaults map[string]ReadmeEnvConfig // Defaults scraped from README
	EnvTargets    []EnvFileTarget       // Target .env files for monorepo support
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

// Critical env vars that will cause runtime errors if missing
var criticalEnvVarPatterns = []string{
	"DATABASE_URL",
	"API_KEY",
	"SECRET_KEY",
	"JWT_SECRET",
	"AUTH0",
	"STRIPE",
	"CLERK",
	"SUPABASE",
	"FIREBASE",
	"NEXT_PUBLIC_API",
}

// isCriticalEnvVar checks if a variable is critical for the app to run
func isCriticalEnvVar(name string) bool {
	nameLower := strings.ToLower(name)
	for _, pattern := range criticalEnvVarPatterns {
		if strings.Contains(nameLower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// isValidEnvVarName checks if a name looks like a valid environment variable
// Filters out false positives like single letters or common non-env patterns
func isValidEnvVarName(name string) bool {
	// Must be at least 3 characters
	if len(name) < 3 {
		return false
	}
	
	// Must contain at least one underscore OR be a known prefix pattern
	knownPrefixes := []string{"API", "AWS", "DATABASE", "DB", "JWT", "NEXT", "NODE", "REACT", "REDIS", "S3", "VITE"}
	for _, prefix := range knownPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	
	// Otherwise require an underscore (like SOME_VAR)
	return strings.Contains(name, "_")
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
				// Skip single-letter or too-short variable names (likely false positives)
				if varName != "" && len(varName) >= 3 && isValidEnvVarName(varName) {
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

	// Read existing .env files from root and common subdirectories
	envVars := make(map[string]string)
	
	// Read root .env and .env.local
	rootVars, _ := ReadEnvFile(status.EnvFile)
	for k, v := range rootVars {
		envVars[k] = v
	}
	localEnvPath := filepath.Join(projectPath, ".env.local")
	localVars, _ := ReadEnvFile(localEnvPath)
	for k, v := range localVars {
		envVars[k] = v
	}

	// Also check common monorepo subdirectories for .env files
	subDirs := []string{"apps/client", "apps/server", "apps/web", "apps/api", "packages/web", "client", "server", "frontend", "backend"}
	for _, subDir := range subDirs {
		subEnvPath := filepath.Join(projectPath, subDir, ".env")
		subVars, _ := ReadEnvFile(subEnvPath)
		for k, v := range subVars {
			envVars[k] = v
		}
		subLocalPath := filepath.Join(projectPath, subDir, ".env.local")
		subLocalVars, _ := ReadEnvFile(subLocalPath)
		for k, v := range subLocalVars {
			envVars[k] = v
		}
	}

	// Mark which vars are defined
	for k := range envVars {
		status.Defined[k] = true
	}

	// Find missing vars and determine their target directories
	for _, v := range required {
		if !status.Defined[v.Name] {
			// Determine target directory based on where the var was found
			v.TargetDir = determineTargetDirFromFile(v.File, projectPath)
			status.Missing = append(status.Missing, v)
		}
	}

	return status, nil
}

// determineTargetDirFromFile extracts the target directory from the file path where a var was found
func determineTargetDirFromFile(filePath string, projectPath string) string {
	// Get relative path from project root
	relPath, err := filepath.Rel(projectPath, filePath)
	if err != nil {
		return ""
	}
	
	// Check for common monorepo patterns
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) >= 2 {
		// apps/client/src/... -> apps/client
		// packages/web/lib/... -> packages/web
		if parts[0] == "apps" || parts[0] == "packages" {
			return filepath.Join(parts[0], parts[1])
		}
		// client/src/... -> client
		// server/src/... -> server
		commonDirs := []string{"client", "server", "frontend", "backend", "web", "api"}
		for _, dir := range commonDirs {
			if parts[0] == dir {
				return parts[0]
			}
		}
	}
	
	return "" // Root directory
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

// ============================================================================
// README-Driven Secret Provisioning
// ============================================================================

// ParseReadmeForEnvVars scans README.md for code blocks containing environment
// variable assignments and extracts them with their suggested values and target directories.
func ParseReadmeForEnvVars(projectPath string) ([]ReadmeEnvConfig, error) {
	var configs []ReadmeEnvConfig

	// Look for README files in various formats
	readmeFiles := []string{"README.md", "README.MD", "readme.md", "Readme.md", "README.txt", "README"}
	
	var readmePath string
	for _, name := range readmeFiles {
		path := filepath.Join(projectPath, name)
		if _, err := os.Stat(path); err == nil {
			readmePath = path
			break
		}
	}

	if readmePath == "" {
		return configs, nil // No README found, not an error
	}

	content, err := os.ReadFile(readmePath)
	if err != nil {
		return nil, err
	}

	contentStr := string(content)

	// Parse code blocks and extract env vars
	configs = extractEnvVarsFromReadme(contentStr, projectPath)

	return configs, nil
}

// extractEnvVarsFromReadme parses README content for environment variable definitions
func extractEnvVarsFromReadme(content string, projectPath string) []ReadmeEnvConfig {
	var configs []ReadmeEnvConfig
	seen := make(map[string]bool)

	// Pattern to match code blocks (```...``` or indented blocks)
	codeBlockPattern := regexp.MustCompile("(?s)```[^`]*```")
	
	// Pattern to match env var assignments: KEY=value or KEY="value" or KEY='value'
	envPattern := regexp.MustCompile(`^([A-Z][A-Z0-9_]*)=["']?([^"'\n]*)["']?`)
	
	// Pattern to detect directory context (e.g., "in apps/client" or "apps/client/.env")
	dirContextPattern := regexp.MustCompile(`(?i)(?:in\s+|cd\s+|create\s+|add\s+to\s+)?([a-z0-9._-]+(?:/[a-z0-9._-]+)*)(?:/\.env)?`)

	// Find all code blocks
	codeBlocks := codeBlockPattern.FindAllString(content, -1)
	
	// Also look for inline env assignments in the text
	lines := strings.Split(content, "\n")
	
	currentDir := "" // Track directory context
	
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		
		// Check for directory context hints
		if dirMatch := dirContextPattern.FindStringSubmatch(trimmedLine); len(dirMatch) > 1 {
			potentialDir := dirMatch[1]
			// Validate it looks like a directory path and exists
			if isValidSubdirectory(projectPath, potentialDir) {
				currentDir = potentialDir
			}
		}
		
		// Look for env var pattern in context lines (lines starting with # or containing =)
		if envMatch := envPattern.FindStringSubmatch(trimmedLine); len(envMatch) >= 2 {
			varName := envMatch[1]
			varValue := ""
			if len(envMatch) > 2 {
				varValue = envMatch[2]
			}
			
			if !seen[varName] && !ignoredEnvVars[varName] {
				seen[varName] = true
				
				config := ReadmeEnvConfig{
					Name:      varName,
					Value:     varValue,
					TargetDir: determineTargetDir(varName, currentDir, projectPath),
				}
				
				// Try to extract description from surrounding context
				if i > 0 {
					prevLine := strings.TrimSpace(lines[i-1])
					if strings.HasPrefix(prevLine, "#") || strings.HasPrefix(prevLine, "//") {
						config.Description = strings.TrimLeft(prevLine, "# /")
					}
				}
				
				configs = append(configs, config)
			}
		}
	}

	// Also parse code blocks
	for _, block := range codeBlocks {
		// Remove the ``` markers
		blockContent := strings.Trim(block, "`")
		// Remove language identifier if present (e.g., ```bash)
		if idx := strings.Index(blockContent, "\n"); idx > 0 {
			firstLine := blockContent[:idx]
			if !strings.Contains(firstLine, "=") {
				blockContent = blockContent[idx+1:]
			}
		}
		
		blockLines := strings.Split(blockContent, "\n")
		blockDir := currentDir
		
		for _, line := range blockLines {
			trimmedLine := strings.TrimSpace(line)
			
			// Check for cd command or directory context
			if strings.HasPrefix(trimmedLine, "cd ") {
				dir := strings.TrimPrefix(trimmedLine, "cd ")
				if isValidSubdirectory(projectPath, dir) {
					blockDir = dir
				}
			}
			
			// Look for env var assignments
			if envMatch := envPattern.FindStringSubmatch(trimmedLine); len(envMatch) >= 2 {
				varName := envMatch[1]
				varValue := ""
				if len(envMatch) > 2 {
					varValue = envMatch[2]
				}
				
				if !seen[varName] && !ignoredEnvVars[varName] {
					seen[varName] = true
					configs = append(configs, ReadmeEnvConfig{
						Name:      varName,
						Value:     varValue,
						TargetDir: determineTargetDir(varName, blockDir, projectPath),
					})
				}
			}
		}
	}

	return configs
}

// determineTargetDir determines the target directory for an env var based on:
// 1. Explicit directory context from README
// 2. Variable name patterns (NEXT_PUBLIC_* -> frontend)
// 3. Monorepo package detection
func determineTargetDir(varName string, contextDir string, projectPath string) string {
	// If context directory is specified and valid, use it
	if contextDir != "" && isValidSubdirectory(projectPath, contextDir) {
		return contextDir
	}

	// Check for NEXT_PUBLIC_ prefix (always goes to frontend/client)
	if strings.HasPrefix(varName, "NEXT_PUBLIC_") || strings.HasPrefix(varName, "VITE_") || strings.HasPrefix(varName, "REACT_APP_") {
		// Look for common frontend directories
		frontendDirs := []string{"apps/client", "apps/web", "apps/frontend", "packages/web", "client", "frontend", "web"}
		for _, dir := range frontendDirs {
			if isValidSubdirectory(projectPath, dir) {
				return dir
			}
		}
	}

	// Check for server-side patterns
	if strings.Contains(varName, "DATABASE") || strings.Contains(varName, "DB_") ||
		strings.Contains(varName, "REDIS") || strings.Contains(varName, "SECRET") ||
		strings.HasPrefix(varName, "AWS_") || strings.HasPrefix(varName, "S3_") {
		// Look for common backend directories
		backendDirs := []string{"apps/server", "apps/api", "apps/backend", "packages/api", "server", "api", "backend"}
		for _, dir := range backendDirs {
			if isValidSubdirectory(projectPath, dir) {
				return dir
			}
		}
	}

	// Default to root
	return ""
}

// isValidSubdirectory checks if a path is a valid subdirectory of the project
func isValidSubdirectory(projectPath string, subDir string) bool {
	if subDir == "" || subDir == "." {
		return false
	}
	
	fullPath := filepath.Join(projectPath, subDir)
	info, err := os.Stat(fullPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// GroupEnvVarsByTarget groups environment variables by their target .env file
func GroupEnvVarsByTarget(configs []ReadmeEnvConfig, projectPath string) []EnvFileTarget {
	targetMap := make(map[string]*EnvFileTarget)

	for _, config := range configs {
		targetDir := config.TargetDir
		if targetDir == "" {
			targetDir = "." // Root directory
		}

		var envPath string
		if targetDir == "." {
			envPath = filepath.Join(projectPath, ".env")
		} else {
			envPath = filepath.Join(projectPath, targetDir, ".env")
		}

		if target, exists := targetMap[targetDir]; exists {
			target.Variables = append(target.Variables, config)
		} else {
			_, fileExists := os.Stat(envPath)
			targetMap[targetDir] = &EnvFileTarget{
				Path:      filepath.Join(targetDir, ".env"),
				AbsPath:   envPath,
				Variables: []ReadmeEnvConfig{config},
				Exists:    fileExists == nil,
			}
		}
	}

	// Convert map to slice
	var targets []EnvFileTarget
	for _, target := range targetMap {
		targets = append(targets, *target)
	}

	// Sort by path for consistent ordering
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Path < targets[j].Path
	})

	return targets
}

// WriteEnvFilesToTargets writes environment variables to their respective .env files
func WriteEnvFilesToTargets(targets []EnvFileTarget, values map[string]string) error {
	for _, target := range targets {
		// Collect values for this target
		targetValues := make(map[string]string)
		for _, v := range target.Variables {
			if val, ok := values[v.Name]; ok && val != "" {
				targetValues[v.Name] = val
			}
		}

		if len(targetValues) == 0 {
			continue
		}

		// Ensure directory exists
		dir := filepath.Dir(target.AbsPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Write to .env file
		if err := AppendToEnvFile(target.AbsPath, targetValues); err != nil {
			return fmt.Errorf("failed to write to %s: %w", target.Path, err)
		}
	}

	return nil
}

// ValidateEnvFilesExist checks if all required .env files exist before running
func ValidateEnvFilesExist(projectPath string, targets []EnvFileTarget) []string {
	var missing []string

	for _, target := range targets {
		if !target.Exists {
			if _, err := os.Stat(target.AbsPath); os.IsNotExist(err) {
				missing = append(missing, target.Path)
			}
		}
	}

	return missing
}

// CheckEnvStatusWithReadme extends CheckEnvStatus with README-sourced defaults
func CheckEnvStatusWithReadme(projectPath string, language string) (EnvStatus, error) {
	// First, get the basic env status
	status, err := CheckEnvStatus(projectPath, language)
	if err != nil {
		return status, err
	}

	// Parse README for defaults
	readmeConfigs, err := ParseReadmeForEnvVars(projectPath)
	if err != nil {
		// Non-fatal - continue without README defaults
		return status, nil
	}

	// Build defaults map
	status.ReadmeDefaults = make(map[string]ReadmeEnvConfig)
	for _, config := range readmeConfigs {
		status.ReadmeDefaults[config.Name] = config
	}

	// Group by target directories
	status.EnvTargets = GroupEnvVarsByTarget(readmeConfigs, projectPath)

	// Update missing vars with defaults and target directories
	for i, v := range status.Missing {
		if config, ok := status.ReadmeDefaults[v.Name]; ok {
			status.Missing[i].DefaultValue = config.Value
			status.Missing[i].TargetDir = config.TargetDir
		}
	}

	return status, nil
}

// GetEnvVarSuggestion returns a suggested value for an env var based on README or heuristics
func GetEnvVarSuggestion(varName string, readmeDefaults map[string]ReadmeEnvConfig) string {
	// Check README defaults first
	if readmeDefaults != nil {
		if config, ok := readmeDefaults[varName]; ok && config.Value != "" {
			return config.Value
		}
	}

	// Provide smart defaults for common patterns
	varLower := strings.ToLower(varName)
	varUpper := strings.ToUpper(varName)

	switch {
	// Next.js public API URL - typically points to backend server
	case strings.HasPrefix(varUpper, "NEXT_PUBLIC_API"):
		return "http://localhost:8080"
	case strings.HasPrefix(varUpper, "NEXT_PUBLIC_WS"):
		return "ws://localhost:8080"
	case strings.HasPrefix(varUpper, "NEXT_PUBLIC_"):
		// Generic NEXT_PUBLIC_ - leave empty, too specific
		return ""

	// Vite public vars
	case strings.HasPrefix(varUpper, "VITE_API"):
		return "http://localhost:8080"
	case strings.HasPrefix(varUpper, "VITE_"):
		return ""

	// React app vars
	case strings.HasPrefix(varUpper, "REACT_APP_API"):
		return "http://localhost:8080"

	// Generic API/URL patterns
	case strings.Contains(varLower, "api_url") || strings.Contains(varLower, "api_base"):
		return "http://localhost:8080"
	case strings.Contains(varLower, "url") && strings.Contains(varLower, "ws"):
		return "ws://localhost:8080"
	case strings.Contains(varLower, "base_url"):
		return "http://localhost:8080"

	// Port configurations
	case varLower == "port" || strings.HasSuffix(varLower, "_port"):
		return "3000"

	// Host configurations
	case strings.Contains(varLower, "host") && !strings.Contains(varLower, "db"):
		return "localhost"

	// Database URLs
	case strings.Contains(varLower, "database_url") || strings.Contains(varLower, "db_url"):
		return "postgresql://localhost:5432/mydb"
	case strings.Contains(varLower, "redis_url"):
		return "redis://localhost:6379"
	case strings.Contains(varLower, "mongo"):
		return "mongodb://localhost:27017/mydb"

	// Environment
	case varLower == "node_env":
		return "development"
	case varLower == "env" || varLower == "environment":
		return "development"
	}

	return ""
}

// PreRunEnvValidation performs pre-run validation to ensure .env files are properly configured
func PreRunEnvValidation(projectPath string, language string) (bool, []string) {
	var issues []string

	// Check env status with README context
	status, err := CheckEnvStatusWithReadme(projectPath, language)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Failed to check environment status: %v", err))
		return false, issues
	}

	// Check for missing .env files in target directories
	if len(status.EnvTargets) > 0 {
		missing := ValidateEnvFilesExist(projectPath, status.EnvTargets)
		for _, path := range missing {
			issues = append(issues, fmt.Sprintf("Missing .env file: %s", path))
		}
	}

	// Check for critical missing variables
	for _, v := range status.Missing {
		if v.Required && isCriticalEnvVar(v.Name) {
			if v.TargetDir != "" {
				issues = append(issues, fmt.Sprintf("Missing required variable %s in %s/.env", v.Name, v.TargetDir))
			} else {
				issues = append(issues, fmt.Sprintf("Missing required variable: %s", v.Name))
			}
		}
	}

	return len(issues) == 0, issues
}

// AutoProvisionResult contains the result of auto-provisioning env files
type AutoProvisionResult struct {
	CreatedFiles    []string          // .env files that were created
	ProvisionedVars map[string]string // Variables that were auto-provisioned with their values
	SkippedVars     []string          // Variables that had no default value
}

// AutoProvisionEnvFiles automatically creates missing .env files with defaults from README
// Returns information about what was created and which variables still need values
func AutoProvisionEnvFiles(projectPath string, language string) (*AutoProvisionResult, error) {
	result := &AutoProvisionResult{
		CreatedFiles:    []string{},
		ProvisionedVars: make(map[string]string),
		SkippedVars:     []string{},
	}

	// Get env status with README defaults
	status, err := CheckEnvStatusWithReadme(projectPath, language)
	if err != nil {
		return result, err
	}

	// If no README defaults found, try to use smart suggestions
	if len(status.ReadmeDefaults) == 0 && len(status.Missing) > 0 {
		// Build suggestions for missing vars
		for _, v := range status.Missing {
			suggestion := GetEnvVarSuggestion(v.Name, nil)
			if suggestion != "" {
				status.ReadmeDefaults[v.Name] = ReadmeEnvConfig{
					Name:      v.Name,
					Value:     suggestion,
					TargetDir: v.TargetDir,
				}
			}
		}
	}

	// Group variables by target directory
	targetVars := make(map[string]map[string]string) // targetDir -> varName -> value

	for _, v := range status.Missing {
		targetDir := v.TargetDir
		if targetDir == "" {
			targetDir = "."
		}

		if _, ok := targetVars[targetDir]; !ok {
			targetVars[targetDir] = make(map[string]string)
		}

		// Get value from README defaults or suggestions
		value := ""
		if config, ok := status.ReadmeDefaults[v.Name]; ok && config.Value != "" {
			value = config.Value
		} else {
			value = GetEnvVarSuggestion(v.Name, status.ReadmeDefaults)
		}

		if value != "" {
			targetVars[targetDir][v.Name] = value
			result.ProvisionedVars[v.Name] = value
		} else {
			result.SkippedVars = append(result.SkippedVars, v.Name)
		}
	}

	// Write to .env files
	for targetDir, vars := range targetVars {
		if len(vars) == 0 {
			continue
		}

		var envPath string
		var displayPath string
		if targetDir == "." {
			envPath = filepath.Join(projectPath, ".env")
			displayPath = ".env"
		} else {
			envPath = filepath.Join(projectPath, targetDir, ".env")
			displayPath = filepath.Join(targetDir, ".env")
		}

		// Check if file existed before
		_, existedBefore := os.Stat(envPath)
		fileExisted := existedBefore == nil

		// Ensure directory exists
		dir := filepath.Dir(envPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return result, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Write/append to .env file
		if err := AppendToEnvFile(envPath, vars); err != nil {
			return result, fmt.Errorf("failed to write to %s: %w", displayPath, err)
		}

		if !fileExisted {
			result.CreatedFiles = append(result.CreatedFiles, displayPath)
		}
	}

	return result, nil
}
