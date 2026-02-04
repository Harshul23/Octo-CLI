package analyzer

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// Analysis is a minimal representation of detected project info.
type Analysis struct {
	// Root is the analyzed directory
	Root string
	// Name is the project/app name derived from directory
	Name string
}

// PortConfig contains detected port configuration from the run command
type PortConfig struct {
	// Port is the detected port number
	Port int
	// Detected indicates if a port was found in the command
	Detected bool
	// FlagType indicates what type of flag was used (--port, -p, -Dserver.port, etc.)
	FlagType string
	// IsDefault indicates if this is a default port (not explicitly specified)
	IsDefault bool
}

// ProjectInfo contains detailed information about the analyzed project.
type ProjectInfo struct {
	// Name is the project/app name derived from directory
	Name string
	// Language is the detected programming language (Node, Java, Python, Unknown)
	Language string
	// Version is the detected version (if available)
	Version string
	// RunCommand is the best-guess command to run the project
	RunCommand string
	// PortConfig contains detected port information
	PortConfig PortConfig
}

// signalFile represents a file that signals a specific project type.
type signalFile struct {
	filename string
	language string
}

// Signal files for project detection
var signalFiles = []signalFile{
	{"package.json", "Node"},
	{"pom.xml", "Java"},
	{"build.gradle", "Java"},
	{"requirements.txt", "Python"},
	{"pyproject.toml", "Python"},
	{"go.mod", "Go"},
	{"Cargo.toml", "Rust"},
	{"Gemfile", "Ruby"},
}

// Analyze performs a minimal analysis of the provided directory.
// Currently, it derives the project name from the directory basename.
func Analyze(dir string) (Analysis, error) {
	abs := dir
	if !filepath.IsAbs(abs) {
		var err error
		abs, err = filepath.Abs(dir)
		if err != nil {
			return Analysis{}, err
		}
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Analysis{}, err
	}
	if !info.IsDir() {
		return Analysis{}, os.ErrInvalid
	}
	return Analysis{
		Root: abs,
		Name: filepath.Base(abs),
	}, nil
}

// AnalyzeProject scans the root directory for signal files and returns
// detailed project information including language, version, and run command.
func AnalyzeProject(path string) (ProjectInfo, error) {
	abs := path
	if !filepath.IsAbs(abs) {
		var err error
		abs, err = filepath.Abs(path)
		if err != nil {
			return ProjectInfo{}, err
		}
	}

	info, err := os.Stat(abs)
	if err != nil {
		return ProjectInfo{}, err
	}
	if !info.IsDir() {
		return ProjectInfo{}, os.ErrInvalid
	}

	projectInfo := ProjectInfo{
		Name:     filepath.Base(abs),
		Language: "Unknown",
		Version:  "",
		RunCommand: "",
	}

	// Scan for signal files
	for _, sf := range signalFiles {
		signalPath := filepath.Join(abs, sf.filename)
		if _, err := os.Stat(signalPath); err == nil {
			projectInfo.Language = sf.language

			switch sf.filename {
			case "package.json":
				projectInfo = analyzeNodeProject(abs, projectInfo)
			case "pom.xml":
				projectInfo = analyzeJavaProject(abs, projectInfo, "maven")
			case "build.gradle":
				projectInfo = analyzeJavaProject(abs, projectInfo, "gradle")
			case "requirements.txt":
				projectInfo = analyzePythonProject(abs, projectInfo, "requirements")
			case "pyproject.toml":
				projectInfo = analyzePythonProject(abs, projectInfo, "pyproject")
			case "go.mod":
				projectInfo = analyzeGoProject(abs, projectInfo)
			case "Cargo.toml":
				projectInfo = analyzeRustProject(abs, projectInfo)
			case "Gemfile":
				projectInfo = analyzeRubyProject(abs, projectInfo)
			}

			// Stop after first match (priority order)
			break
		}
	}

	// Detect port configuration from the run command
	projectInfo.PortConfig = DetectPortConfig(projectInfo.RunCommand, projectInfo.Language)

	// If no project was detected by signal files, try simple project detection
	if projectInfo.Language == "Unknown" || projectInfo.RunCommand == "" {
		simpleInfo, err := DetectSimpleProject(abs)
		if err == nil {
			projectInfo = simpleInfo
			projectInfo.PortConfig = DetectPortConfig(projectInfo.RunCommand, projectInfo.Language)
		}
	}

	return projectInfo, nil
}

// analyzeNodeProject extracts info from package.json
func analyzeNodeProject(projectPath string, info ProjectInfo) ProjectInfo {
	packagePath := filepath.Join(projectPath, "package.json")
	data, err := os.ReadFile(packagePath)
	if err != nil {
		info.RunCommand = "npm start"
		return info
	}

	var pkg struct {
		Name    string            `json:"name"`
		Version string            `json:"version"`
		Scripts map[string]string `json:"scripts"`
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		info.RunCommand = "npm start"
		return info
	}

	if pkg.Name != "" {
		info.Name = pkg.Name
	}
	if pkg.Engines.Node != "" {
		info.Version = pkg.Engines.Node
	}

	// Look for start script
	// Use npm/yarn commands instead of raw script content to ensure node_modules/.bin is in PATH
	if _, ok := pkg.Scripts["start"]; ok {
		info.RunCommand = "npm start"
	} else if _, ok := pkg.Scripts["dev"]; ok {
		info.RunCommand = "npm run dev"
	} else {
		info.RunCommand = "npm start"
	}

	return info
}

// analyzeJavaProject extracts info for Java projects
func analyzeJavaProject(projectPath string, info ProjectInfo, buildTool string) ProjectInfo {
	switch buildTool {
	case "maven":
		// Try to detect Java version and Spring Boot from pom.xml
		pomPath := filepath.Join(projectPath, "pom.xml")
		isSpringBoot := false
		if data, err := os.ReadFile(pomPath); err == nil {
			content := string(data)
			// Simple version detection (look for java.version property)
			if contains(content, "<java.version>") {
				info.Version = extractBetween(content, "<java.version>", "</java.version>")
			} else if contains(content, "<maven.compiler.source>") {
				info.Version = extractBetween(content, "<maven.compiler.source>", "</maven.compiler.source>")
			}
			// Detect Spring Boot indicators
			if contains(content, "org.springframework.boot") || 
			   contains(content, "spring-boot-starter") || 
			   contains(content, "spring-boot-maven-plugin") {
				isSpringBoot = true
			}
		}
		// Set run command based on Spring Boot detection
		if isSpringBoot {
			info.RunCommand = "mvn spring-boot:run"
		} else {
			info.RunCommand = "mvn package && java -jar target/*.jar"
		}
	case "gradle":
		// Check for gradlew
		gradlewPath := filepath.Join(projectPath, "gradlew")
		hasGradlew := true
		if _, err := os.Stat(gradlewPath); os.IsNotExist(err) {
			hasGradlew = false
		}
		
		// Try to detect Spring Boot from build.gradle
		buildGradlePath := filepath.Join(projectPath, "build.gradle")
		isSpringBoot := false
		if data, err := os.ReadFile(buildGradlePath); err == nil {
			content := string(data)
			// Detect Spring Boot indicators
			if contains(content, "org.springframework.boot") || 
			   contains(content, "spring-boot") {
				isSpringBoot = true
			}
		}
		
		// Set run command based on Spring Boot detection and wrapper presence
		if isSpringBoot {
			if hasGradlew {
				info.RunCommand = "./gradlew bootRun"
			} else {
				info.RunCommand = "gradle bootRun"
			}
		} else {
			if hasGradlew {
				info.RunCommand = "./gradlew build && java -jar build/libs/*.jar"
			} else {
				info.RunCommand = "gradle build && java -jar build/libs/*.jar"
			}
		}
	}

	return info
}

// analyzePythonProject extracts info for Python projects
func analyzePythonProject(projectPath string, info ProjectInfo, configType string) ProjectInfo {
	switch configType {
	case "requirements":
		// Default Python run command
		info.RunCommand = "python3 main.py"
		// Check for common entry points
		if _, err := os.Stat(filepath.Join(projectPath, "app.py")); err == nil {
			info.RunCommand = "python3 app.py"
		} else if _, err := os.Stat(filepath.Join(projectPath, "main.py")); err == nil {
			info.RunCommand = "python3 main.py"
		} else if _, err := os.Stat(filepath.Join(projectPath, "manage.py")); err == nil {
			info.RunCommand = "python3 manage.py runserver"
		}
	case "pyproject":
		info.RunCommand = "python3 -m app"
		// Check for poetry
		pyprojectPath := filepath.Join(projectPath, "pyproject.toml")
		if data, err := os.ReadFile(pyprojectPath); err == nil {
			content := string(data)
			if contains(content, "[tool.poetry]") {
				info.RunCommand = "poetry run python3 main.py"
			}
			// Try to extract Python version
			if contains(content, "python = ") {
				// Simple extraction
				info.Version = extractPythonVersion(content)
			}
		}
	}

	return info
}

// analyzeGoProject extracts info for Go projects
func analyzeGoProject(projectPath string, info ProjectInfo) ProjectInfo {
	goModPath := filepath.Join(projectPath, "go.mod")
	if data, err := os.ReadFile(goModPath); err == nil {
		content := string(data)
		// Extract Go version
		if contains(content, "go ") {
			info.Version = extractGoVersion(content)
		}
		// Extract module name for better project name
		if contains(content, "module ") {
			moduleName := extractGoModule(content)
			if moduleName != "" {
				// Use last part of module path as name
				parts := splitString(moduleName, "/")
				if len(parts) > 0 {
					info.Name = parts[len(parts)-1]
				}
			}
		}
	}
	
	// Check for common entry points
	if _, err := os.Stat(filepath.Join(projectPath, "main.go")); err == nil {
		info.RunCommand = "go run main.go"
	} else if _, err := os.Stat(filepath.Join(projectPath, "cmd")); err == nil {
		// Check for cmd directory structure
		info.RunCommand = "go run ./cmd/..."
	} else {
		info.RunCommand = "go run ."
	}
	
	return info
}

// analyzeRustProject extracts info for Rust projects
func analyzeRustProject(projectPath string, info ProjectInfo) ProjectInfo {
	cargoTomlPath := filepath.Join(projectPath, "Cargo.toml")
	if data, err := os.ReadFile(cargoTomlPath); err == nil {
		content := string(data)
		// Extract package name
		if contains(content, "name = ") {
			pkgName := extractTomlStringValue(content, "name = ")
			if pkgName != "" {
				info.Name = pkgName
			}
		}
		// Extract version
		if contains(content, "version = ") {
			info.Version = extractTomlStringValue(content, "version = ")
		}
	}
	
	// Default Rust run command
	info.RunCommand = "cargo run"
	
	return info
}

// analyzeRubyProject extracts info for Ruby projects
func analyzeRubyProject(projectPath string, info ProjectInfo) ProjectInfo {
	// Check for common Ruby frameworks and entry points
	if _, err := os.Stat(filepath.Join(projectPath, "config.ru")); err == nil {
		// Rack application
		info.RunCommand = "bundle exec rackup"
	} else if _, err := os.Stat(filepath.Join(projectPath, "config", "application.rb")); err == nil {
		// Rails application
		info.RunCommand = "bundle exec rails server"
	} else if _, err := os.Stat(filepath.Join(projectPath, "app.rb")); err == nil {
		// Sinatra or simple Ruby app
		info.RunCommand = "bundle exec ruby app.rb"
	} else if _, err := os.Stat(filepath.Join(projectPath, "main.rb")); err == nil {
		info.RunCommand = "bundle exec ruby main.rb"
	} else {
		// Generic Ruby execution
		info.RunCommand = "bundle exec ruby main.rb"
	}
	
	// Try to extract Ruby version from .ruby-version file
	rubyVersionPath := filepath.Join(projectPath, ".ruby-version")
	if data, err := os.ReadFile(rubyVersionPath); err == nil {
		info.Version = string(data)
		// Trim whitespace
		info.Version = trimWhitespace(info.Version)
	}
	
	return info
}

// Add these to your signalFiles or as a separate extension check
func DetectSimpleProject(abs string) (ProjectInfo, error) {
	files, err := os.ReadDir(abs)
	if err != nil {
		return ProjectInfo{}, err
	}

	// Track what we find
	var htmlFiles []string
	var pyFiles []string
	var mainPy, appPy, guiPy string

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		ext := filepath.Ext(name)

		switch ext {
		case ".html", ".htm":
			htmlFiles = append(htmlFiles, name)
			// Prefer index.html
			if name == "index.html" || name == "index.htm" {
				htmlFiles = []string{name} // Make it first priority
			}
		case ".py":
			pyFiles = append(pyFiles, name)
			nameLower := strings.ToLower(name)
			if nameLower == "main.py" {
				mainPy = name
			} else if nameLower == "app.py" {
				appPy = name
			} else if strings.Contains(nameLower, "gui") || strings.Contains(nameLower, "window") || strings.Contains(nameLower, "tk") {
				guiPy = name
			}
		}
	}

	// Priority 1: HTML project
	if len(htmlFiles) > 0 {
		targetFile := htmlFiles[0]
		// Use index.html if available
		for _, f := range htmlFiles {
			if f == "index.html" || f == "index.htm" {
				targetFile = f
				break
			}
		}
		return ProjectInfo{
			Name:       filepath.Base(abs),
			Language:   "HTML",
			RunCommand: GetBrowserOpenCommand(targetFile),
		}, nil
	}

	// Priority 2: Python project
	if len(pyFiles) > 0 {
		var targetFile string
		var isGUI bool

		// Check for GUI indicators in Python files
		if guiPy != "" {
			targetFile = guiPy
			isGUI = true
		} else if mainPy != "" {
			targetFile = mainPy
			isGUI = isPythonGUIApp(filepath.Join(abs, mainPy))
		} else if appPy != "" {
			targetFile = appPy
			isGUI = isPythonGUIApp(filepath.Join(abs, appPy))
		} else {
			targetFile = pyFiles[0]
			isGUI = isPythonGUIApp(filepath.Join(abs, targetFile))
		}

		runCmd := "python3 " + targetFile
		if isGUI {
			// For GUI apps, we might want to run differently on some platforms
			runCmd = "python3 " + targetFile
		}

		return ProjectInfo{
			Name:       filepath.Base(abs),
			Language:   "Python",
			RunCommand: runCmd,
		}, nil
	}

	return ProjectInfo{}, os.ErrInvalid
}

// GetBrowserOpenCommand returns the platform-specific command to open a file in browser
func GetBrowserOpenCommand(filename string) string {
	switch runtime.GOOS {
	case "darwin":
		return "open " + filename
	case "linux":
		return "xdg-open " + filename
	case "windows":
		return "start " + filename
	default:
		return "open " + filename
	}
}

// isPythonGUIApp checks if a Python file contains GUI framework imports
func isPythonGUIApp(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	content := string(data)
	guiIndicators := []string{
		"import tkinter",
		"from tkinter",
		"import Tkinter",
		"from Tkinter",
		"import PyQt",
		"from PyQt",
		"import PySide",
		"from PySide",
		"import wx",
		"from wx",
		"import kivy",
		"from kivy",
		"import pygame",
		"from pygame",
		"import turtle",
		"from turtle",
		"import customtkinter",
		"from customtkinter",
	}

	for _, indicator := range guiIndicators {
		if strings.Contains(content, indicator) {
			return true
		}
	}

	return false
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func extractBetween(s, start, end string) string {
	startIdx := findSubstring(s, start)
	if startIdx < 0 {
		return ""
	}
	startIdx += len(start)
	endIdx := findSubstring(s[startIdx:], end)
	if endIdx < 0 {
		return ""
	}
	return s[startIdx : startIdx+endIdx]
}

func extractPythonVersion(content string) string {
	// Look for python = "^3.x" or similar patterns
	idx := findSubstring(content, "python = ")
	if idx < 0 {
		return ""
	}
	// Skip past 'python = '
	idx += len("python = ")
	// Find the quoted version
	if idx < len(content) && (content[idx] == '"' || content[idx] == '\'') {
		quote := content[idx]
		idx++
		endIdx := idx
		for endIdx < len(content) && content[endIdx] != quote {
			endIdx++
		}
		if endIdx > idx {
			return content[idx:endIdx]
		}
	}
	return ""
}

func extractGoVersion(content string) string {
	// Look for "go 1.x" line
	lines := splitString(content, "\n")
	for _, line := range lines {
		line = trimWhitespace(line)
		if len(line) > 3 && line[:3] == "go " {
			return trimWhitespace(line[3:])
		}
	}
	return ""
}

func extractGoModule(content string) string {
	// Look for "module <name>" line
	lines := splitString(content, "\n")
	for _, line := range lines {
		line = trimWhitespace(line)
		if len(line) > 7 && line[:7] == "module " {
			return trimWhitespace(line[7:])
		}
	}
	return ""
}

func extractTomlStringValue(content, key string) string {
	// Look for key = "value" pattern
	idx := findSubstring(content, key)
	if idx < 0 {
		return ""
	}
	idx += len(key)
	// Find the quoted value
	for idx < len(content) && (content[idx] == ' ' || content[idx] == '\t') {
		idx++
	}
	if idx < len(content) && (content[idx] == '"' || content[idx] == '\'') {
		quote := content[idx]
		idx++
		endIdx := idx
		for endIdx < len(content) && content[endIdx] != quote {
			endIdx++
		}
		if endIdx > idx {
			return content[idx:endIdx]
		}
	}
	return ""
}

func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	if sep == "" {
		return []string{s}
	}
	
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i = start - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimWhitespace(s string) string {
	start := 0
	end := len(s)
	
	// Trim leading whitespace
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	
	// Trim trailing whitespace
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	
	return s[start:end]
}

// Port detection patterns for different frameworks
var portPatterns = []*regexp.Regexp{
	// Node.js / Generic: --port 3000, --port=3000
	regexp.MustCompile(`--port[=\s](\d+)`),
	// Short flag: -p 3000, -p=3000
	regexp.MustCompile(`-p[=\s](\d+)`),
	// Environment variable: PORT=3000
	regexp.MustCompile(`PORT=(\d+)`),
	// Java/Spring Boot: -Dserver.port=8080
	regexp.MustCompile(`-Dserver\.port=(\d+)`),
	// Host:port patterns
	regexp.MustCompile(`localhost:(\d+)`),
	regexp.MustCompile(`127\.0\.0\.1:(\d+)`),
	regexp.MustCompile(`0\.0\.0\.0:(\d+)`),
}

// Default ports for common frameworks
var defaultPortsByLanguage = map[string]int{
	"Node":   3000,
	"Python": 5000, // Flask default
	"Java":   8080, // Spring Boot default
	"Go":     8080,
	"Ruby":   3000, // Rails default
	"Rust":   8080,
}

// Default ports for specific commands
var defaultPortsByCommand = map[string]int{
	"npm start":                   3000,
	"npm run dev":                 3000,
	"yarn start":                  3000,
	"yarn dev":                    3000,
	"flask run":                   5000,
	"python manage.py runserver": 8000, // Django
	"rails server":                3000,
	"mvn spring-boot:run":        8080,
	"./gradlew bootRun":           8080,
	"gradle bootRun":              8080,
}

// DetectPortConfig scans a run command for port configuration
func DetectPortConfig(runCommand string, language string) PortConfig {
	config := PortConfig{
		Detected: false,
	}

	if runCommand == "" {
		return config
	}

	// Try to extract explicit port from the command
	for i, pattern := range portPatterns {
		matches := pattern.FindStringSubmatch(runCommand)
		if len(matches) >= 2 {
			port, err := strconv.Atoi(matches[1])
			if err == nil && port > 0 && port < 65536 {
				config.Port = port
				config.Detected = true
				config.IsDefault = false
				
				// Determine flag type based on pattern index
				switch i {
				case 0:
					config.FlagType = "--port"
				case 1:
					config.FlagType = "-p"
				case 2:
					config.FlagType = "PORT="
				case 3:
					config.FlagType = "-Dserver.port"
				default:
					config.FlagType = "host:port"
				}
				return config
			}
		}
	}

	// Check for default ports based on command patterns
	cmdLower := strings.ToLower(runCommand)
	for pattern, port := range defaultPortsByCommand {
		if strings.Contains(cmdLower, strings.ToLower(pattern)) {
			config.Port = port
			config.Detected = true
			config.IsDefault = true
			config.FlagType = "default"
			return config
		}
	}

	// Fall back to language defaults
	if port, ok := defaultPortsByLanguage[language]; ok {
		config.Port = port
		config.Detected = true
		config.IsDefault = true
		config.FlagType = "language-default"
	}

	return config
}

// ValidatePort checks if a port is available using net.Listen
func ValidatePort(port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	
	addr := ":" + strconv.Itoa(port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// GetPortFlagForLanguage returns the appropriate port flag for a given language
func GetPortFlagForLanguage(language string) string {
	switch language {
	case "Node":
		return "--port"
	case "Python":
		return "--port"
	case "Java":
		return "-Dserver.port="
	case "Ruby":
		return "-p"
	default:
		return "--port"
	}
}