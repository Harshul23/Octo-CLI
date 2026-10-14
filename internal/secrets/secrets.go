package secrets

import (
	"bufio"
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
	if len(name) < 3 {
		return false
	}

	knownPrefixes := []string{"API", "AWS", "DATABASE", "DB", "JWT", "NEXT", "NODE", "REACT", "REDIS", "S3", "VITE"}
	for _, prefix := range knownPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return strings.Contains(name, "_")
}

// ScanForEnvVars scans the project directory for environment variable usage
func ScanForEnvVars(projectPath string, language string) ([]EnvVar, error) {
	var envVars []EnvVar
	seen := make(map[string]bool)

	patterns := getPatterns(language)

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == ".github" || name == ".gitlab" ||
				name == ".circleci" || name == "vendor" ||
				name == "target" || name == "build" || name == "dist" ||
				name == "__pycache__" || name == ".venv" || name == "venv" ||
				name == "__tests__" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if !shouldScanFile(ext, language) {
			return nil
		}

		if isTestFile(path) {
			return nil
		}

		fileVars, err := scanFile(path, patterns)
		if err != nil {
			return nil
		}

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

	defaults := checkEnvExample(projectPath)
	hasKubeConfig := kubeConfigExists()

	for i := range envVars {
		envVars[i].Required = false

		if isCriticalEnvVar(envVars[i].Name) {
			if hasDefault, ok := defaults[envVars[i].Name]; !ok || !hasDefault {
				envVars[i].Required = true
			}
		}

		if envVars[i].Name == "KUBECONFIG" && hasKubeConfig {
			envVars[i].Required = false
		}
	}

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

	patterns["generic"] = envPatterns["generic"]

	if lang == "node" || lang == "unknown" {
		patterns["node"] = envPatterns["node"]
	}

	return patterns
}

// shouldScanFile checks if a file should be scanned based on extension
func shouldScanFile(ext string, language string) bool {
	lang := strings.ToLower(language)

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

// isTestFile returns true if the file is a test or spec file
func isTestFile(path string) bool {
	base := filepath.Base(path)
	lower := strings.ToLower(base)

	if strings.HasSuffix(lower, "_test.go") {
		return true
	}

	if strings.HasSuffix(lower, ".test.js") || strings.HasSuffix(lower, ".spec.js") ||
		strings.HasSuffix(lower, ".test.ts") || strings.HasSuffix(lower, ".spec.ts") ||
		strings.HasSuffix(lower, ".test.jsx") || strings.HasSuffix(lower, ".spec.jsx") ||
		strings.HasSuffix(lower, ".test.tsx") || strings.HasSuffix(lower, ".spec.tsx") ||
		strings.HasSuffix(lower, ".test.mjs") || strings.HasSuffix(lower, ".spec.mjs") ||
		strings.HasSuffix(lower, ".test.cjs") || strings.HasSuffix(lower, ".spec.cjs") {
		return true
	}

	if strings.HasSuffix(lower, ".py") && (strings.HasPrefix(lower, "test_") || strings.HasSuffix(lower, "_test.py")) {
		return true
	}

	if strings.HasSuffix(lower, ".rb") && (strings.HasSuffix(lower, "_spec.rb") || strings.HasSuffix(lower, "_test.rb") || strings.HasPrefix(lower, "test_")) {
		return true
	}

	if strings.HasSuffix(lower, "test.java") || strings.HasSuffix(lower, "tests.java") || strings.HasSuffix(lower, "testcase.java") ||
		strings.HasSuffix(lower, "test.kt") || strings.HasSuffix(lower, "tests.kt") ||
		strings.HasSuffix(lower, "test.cs") || strings.HasSuffix(lower, "tests.cs") {
		return true
	}

	if strings.HasSuffix(lower, ".rs") && (strings.HasSuffix(lower, "_test.rs") || lower == "test.rs" || lower == "tests.rs") {
		return true
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
				varName := ""
				for i := 1; i < len(match); i++ {
					if match[i] != "" {
						varName = match[i]
						break
					}
				}
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

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, `"'`)
			vars[key] = value
		}
	}

	return vars, scanner.Err()
}
