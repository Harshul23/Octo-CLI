package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Analysis is a minimal representation of detected project info.
type Analysis struct {
	// Root is the analyzed directory
	Root string
	// Name is the project/app name derived from directory
	Name string
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
	files, _ := os.ReadDir(abs)
	
	for _, file := range files {
		if file.IsDir() { continue }
		
		ext := filepath.Ext(file.Name())
		
		// 1. Detect Simple HTML
		if ext == ".html" {
			return ProjectInfo{
				Name:       filepath.Base(abs),
				Language:   "HTML",
				RunCommand: "open " + file.Name(), // MacOS specific, use 'xdg-open' for Linux
			}, nil
		}
		
		// 2. Detect Simple Python
		if ext == ".py" {
			return ProjectInfo{
				Name:       filepath.Base(abs),
				Language:   "Python",
				RunCommand: "python3 " + file.Name(),
			}, nil
		}
	}
	return ProjectInfo{}, os.ErrInvalid
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