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
	if startScript, ok := pkg.Scripts["start"]; ok {
		info.RunCommand = startScript
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
		info.RunCommand = "mvn spring-boot:run"
		// Try to detect Java version from pom.xml (simplified)
		pomPath := filepath.Join(projectPath, "pom.xml")
		if data, err := os.ReadFile(pomPath); err == nil {
			content := string(data)
			// Simple version detection (look for java.version property)
			if contains(content, "<java.version>") {
				info.Version = extractBetween(content, "<java.version>", "</java.version>")
			} else if contains(content, "<maven.compiler.source>") {
				info.Version = extractBetween(content, "<maven.compiler.source>", "</maven.compiler.source>")
			}
		}
	case "gradle":
		info.RunCommand = "./gradlew bootRun"
		// Check for gradlew
		gradlewPath := filepath.Join(projectPath, "gradlew")
		if _, err := os.Stat(gradlewPath); os.IsNotExist(err) {
			info.RunCommand = "gradle bootRun"
		}
	}

	return info
}

// analyzePythonProject extracts info for Python projects
func analyzePythonProject(projectPath string, info ProjectInfo, configType string) ProjectInfo {
	switch configType {
	case "requirements":
		// Default Python run command
		info.RunCommand = "python main.py"
		// Check for common entry points
		if _, err := os.Stat(filepath.Join(projectPath, "app.py")); err == nil {
			info.RunCommand = "python app.py"
		} else if _, err := os.Stat(filepath.Join(projectPath, "main.py")); err == nil {
			info.RunCommand = "python main.py"
		} else if _, err := os.Stat(filepath.Join(projectPath, "manage.py")); err == nil {
			info.RunCommand = "python manage.py runserver"
		}
	case "pyproject":
		info.RunCommand = "python -m app"
		// Check for poetry
		pyprojectPath := filepath.Join(projectPath, "pyproject.toml")
		if data, err := os.ReadFile(pyprojectPath); err == nil {
			content := string(data)
			if contains(content, "[tool.poetry]") {
				info.RunCommand = "poetry run python main.py"
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