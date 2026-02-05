package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/harshul/octo-cli/internal/provisioner"
)

// RuntimeStatus represents the status of a runtime check
type RuntimeStatus struct {
	Name      string
	Installed bool
	Version   string
	Path      string
}

// DependencyStatus represents the status of project dependencies
type DependencyStatus struct {
	Manager          string   // npm, pip, maven, etc.
	ConfigFile       string   // package.json, requirements.txt, etc.
	Installed        bool     // Are dependencies installed?
	MissingPackages  []string // List of missing packages (if detectable)
	InstallCommand   string   // Command to install dependencies
	ManagerInstalled bool     // Is the package manager itself installed?
	ManagerHint      string   // Hint for installing the package manager
	FixCommand       string   // One-liner command to fix the issue
	IsMonorepo       bool     // Is this a monorepo/workspace project?
}

// Diagnosis contains the full health check results
type Diagnosis struct {
	ProjectPath  string
	Language     string
	Runtime      RuntimeStatus
	Dependencies DependencyStatus
	Healthy      bool
	Issues       []string
}

// Diagnose checks the health of the project at the given path
func Diagnose(projectPath string, language string) Diagnosis {
	diagnosis := Diagnosis{
		ProjectPath: projectPath,
		Language:    language,
		Healthy:     true,
		Issues:      []string{},
	}

	// Check runtime based on detected language
	switch language {
	case "Node":
		diagnosis.Runtime = checkNodeRuntime()
		diagnosis.Dependencies = checkNodeDependencies(projectPath)
	case "Python":
		diagnosis.Runtime = checkPythonRuntime()
		diagnosis.Dependencies = checkPythonDependencies(projectPath)
	case "Java":
		diagnosis.Runtime = checkJavaRuntime()
		diagnosis.Dependencies = checkJavaDependencies(projectPath)
	case "Go":
		diagnosis.Runtime = checkGoRuntime()
		diagnosis.Dependencies = checkGoDependencies(projectPath)
	case "Ruby":
		diagnosis.Runtime = checkRubyRuntime()
		diagnosis.Dependencies = checkRubyDependencies(projectPath)
	case "Rust":
		diagnosis.Runtime = checkRustRuntime()
		diagnosis.Dependencies = checkRustDependencies(projectPath)
	case "HTML":
		// HTML projects don't need a runtime - they run in the browser
		diagnosis.Runtime = RuntimeStatus{Name: "Browser", Installed: true, Version: "default"}
		diagnosis.Dependencies = DependencyStatus{Installed: true}
	default:
		diagnosis.Runtime = RuntimeStatus{Name: "Unknown", Installed: false}
		diagnosis.Dependencies = DependencyStatus{}
	}

	// Determine if project is healthy
	if !diagnosis.Runtime.Installed {
		diagnosis.Healthy = false
		diagnosis.Issues = append(diagnosis.Issues, diagnosis.Runtime.Name+" runtime is not installed")
	}

	// Check if the required package manager is installed
	if !diagnosis.Dependencies.ManagerInstalled && diagnosis.Dependencies.ManagerHint != "" {
		diagnosis.Healthy = false
		diagnosis.Issues = append(diagnosis.Issues, diagnosis.Dependencies.ManagerHint)
	}

	if !diagnosis.Dependencies.Installed && diagnosis.Dependencies.ConfigFile != "" {
		diagnosis.Healthy = false
		diagnosis.Issues = append(diagnosis.Issues, "Dependencies are not installed")
	}

	return diagnosis
}

// checkNodeRuntime checks if Node.js is installed
func checkNodeRuntime() RuntimeStatus {
	status := RuntimeStatus{Name: "Node.js", Installed: false}

	cmd := exec.Command("node", "--version")
	output, err := cmd.Output()
	if err == nil {
		status.Installed = true
		status.Version = strings.TrimSpace(string(output))
	}

	// Find path
	pathCmd := exec.Command("which", "node")
	pathOutput, err := pathCmd.Output()
	if err == nil {
		status.Path = strings.TrimSpace(string(pathOutput))
	}

	return status
}

// checkPythonRuntime checks if Python is installed
func checkPythonRuntime() RuntimeStatus {
	status := RuntimeStatus{Name: "Python", Installed: false}

	// Try python3 first, then python
	for _, pythonCmd := range []string{"python3", "python"} {
		cmd := exec.Command(pythonCmd, "--version")
		output, err := cmd.Output()
		if err == nil {
			status.Installed = true
			status.Version = strings.TrimSpace(string(output))

			pathCmd := exec.Command("which", pythonCmd)
			pathOutput, err := pathCmd.Output()
			if err == nil {
				status.Path = strings.TrimSpace(string(pathOutput))
			}
			break
		}
	}

	return status
}

// checkJavaRuntime checks if Java is installed
func checkJavaRuntime() RuntimeStatus {
	status := RuntimeStatus{Name: "Java", Installed: false}

	cmd := exec.Command("java", "-version")
	// Java outputs version to stderr
	output, err := cmd.CombinedOutput()
	if err == nil {
		status.Installed = true
		// Parse first line for version
		lines := strings.Split(string(output), "\n")
		if len(lines) > 0 {
			status.Version = strings.TrimSpace(lines[0])
		}
	}

	pathCmd := exec.Command("which", "java")
	pathOutput, err := pathCmd.Output()
	if err == nil {
		status.Path = strings.TrimSpace(string(pathOutput))
	}

	return status
}

// checkGoRuntime checks if Go is installed
func checkGoRuntime() RuntimeStatus {
	status := RuntimeStatus{Name: "Go", Installed: false}

	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err == nil {
		status.Installed = true
		status.Version = strings.TrimSpace(string(output))
	}

	pathCmd := exec.Command("which", "go")
	pathOutput, err := pathCmd.Output()
	if err == nil {
		status.Path = strings.TrimSpace(string(pathOutput))
	}

	return status
}

// checkRubyRuntime checks if Ruby is installed
func checkRubyRuntime() RuntimeStatus {
	status := RuntimeStatus{Name: "Ruby", Installed: false}

	cmd := exec.Command("ruby", "--version")
	output, err := cmd.Output()
	if err == nil {
		status.Installed = true
		status.Version = strings.TrimSpace(string(output))
	}

	pathCmd := exec.Command("which", "ruby")
	pathOutput, err := pathCmd.Output()
	if err == nil {
		status.Path = strings.TrimSpace(string(pathOutput))
	}

	return status
}

// checkRustRuntime checks if Rust is installed
func checkRustRuntime() RuntimeStatus {
	status := RuntimeStatus{Name: "Rust", Installed: false}

	cmd := exec.Command("rustc", "--version")
	output, err := cmd.Output()
	if err == nil {
		status.Installed = true
		status.Version = strings.TrimSpace(string(output))
	}

	pathCmd := exec.Command("which", "rustc")
	pathOutput, err := pathCmd.Output()
	if err == nil {
		status.Path = strings.TrimSpace(string(pathOutput))
	}

	return status
}

// checkNodeDependencies checks if Node.js dependencies are installed
func checkNodeDependencies(projectPath string) DependencyStatus {
	status := DependencyStatus{Manager: "npm", ManagerInstalled: true}

	packageJsonPath := filepath.Join(projectPath, "package.json")
	if _, err := os.Stat(packageJsonPath); err != nil {
		return status // No package.json found
	}
	status.ConfigFile = "package.json"

	// Check if node_modules exists
	nodeModulesPath := filepath.Join(projectPath, "node_modules")
	if _, err := os.Stat(nodeModulesPath); err == nil {
		status.Installed = true
	}

	// Use provisioner to detect the correct package manager and check Corepack availability
	pmResult := provisioner.EnsurePackageManager(projectPath)
	status.Manager = string(pmResult.Manager)
	status.ManagerInstalled = pmResult.Available
	status.IsMonorepo = provisioner.DetectPackageManager(projectPath).IsMonorepo

	// Set appropriate hint and fix command based on the package manager status
	if !pmResult.Available {
		// Get the one-liner fix command
		status.FixCommand = provisioner.GetFixCommand(pmResult.Manager)

		switch pmResult.Manager {
		case provisioner.Bun:
			status.ManagerHint = "❌ bun is required but not installed."
			status.FixCommand = "curl -fsSL https://bun.sh/install | bash"
		case provisioner.PNPM:
			if provisioner.IsCommandAvailable("corepack") {
				status.ManagerHint = "❌ pnpm is required but not installed."
				status.FixCommand = "corepack enable pnpm"
			} else {
				status.ManagerHint = "❌ pnpm is required but not found. Please install Node.js (which includes Corepack) or install pnpm manually."
				status.FixCommand = "npm install -g pnpm"
			}
		case provisioner.Yarn:
			if provisioner.IsCommandAvailable("corepack") {
				status.ManagerHint = "❌ yarn is required but not installed."
				status.FixCommand = "corepack enable yarn"
			} else {
				status.ManagerHint = "❌ yarn is required but not found. Please install Node.js (which includes Corepack) or install yarn manually."
				status.FixCommand = "npm install -g yarn"
			}
		case provisioner.NPM:
			status.ManagerHint = "❌ npm is required but not found."
			status.FixCommand = "Install Node.js from https://nodejs.org"
		default:
			if pmResult.UserMessage != "" {
				status.ManagerHint = pmResult.UserMessage
			}
		}
	}

	// Get the install command from provisioner
	installCmd := provisioner.GetInstallCommand(projectPath)
	status.InstallCommand = strings.Join(installCmd, " ")

	return status
}

// checkPythonDependencies checks if Python dependencies are installed
func checkPythonDependencies(projectPath string) DependencyStatus {
	status := DependencyStatus{Manager: "pip"}

	// Check for requirements.txt
	reqPath := filepath.Join(projectPath, "requirements.txt")
	if _, err := os.Stat(reqPath); err == nil {
		status.ConfigFile = "requirements.txt"
		status.InstallCommand = "pip install -r requirements.txt"

		// Try to detect missing packages by reading requirements and checking imports
		status.MissingPackages = detectMissingPythonPackages(projectPath, reqPath)
		status.Installed = len(status.MissingPackages) == 0

		return status
	}

	// Check for pyproject.toml
	pyprojectPath := filepath.Join(projectPath, "pyproject.toml")
	if _, err := os.Stat(pyprojectPath); err == nil {
		status.ConfigFile = "pyproject.toml"

		// Check if it's a Poetry project
		if data, err := os.ReadFile(pyprojectPath); err == nil {
			content := string(data)
			if strings.Contains(content, "[tool.poetry]") {
				status.Manager = "poetry"
				status.InstallCommand = "poetry install"

				// Check for poetry.lock as indicator of installed deps
				if _, err := os.Stat(filepath.Join(projectPath, "poetry.lock")); err == nil {
					status.Installed = true
				}
			} else {
				status.InstallCommand = "pip install -e ."
			}
		}

		return status
	}

	return status
}

// checkJavaDependencies checks if Java dependencies are available
func checkJavaDependencies(projectPath string) DependencyStatus {
	status := DependencyStatus{}

	// Check for Maven (pom.xml)
	pomPath := filepath.Join(projectPath, "pom.xml")
	if _, err := os.Stat(pomPath); err == nil {
		status.Manager = "maven"
		status.ConfigFile = "pom.xml"
		status.InstallCommand = "mvn dependency:resolve"

		// Check for target directory as indicator
		targetPath := filepath.Join(projectPath, "target")
		if _, err := os.Stat(targetPath); err == nil {
			status.Installed = true
		}

		return status
	}

	// Check for Gradle (build.gradle)
	gradlePath := filepath.Join(projectPath, "build.gradle")
	if _, err := os.Stat(gradlePath); err == nil {
		status.Manager = "gradle"
		status.ConfigFile = "build.gradle"

		// Check for gradlew
		if _, err := os.Stat(filepath.Join(projectPath, "gradlew")); err == nil {
			status.InstallCommand = "./gradlew dependencies"
		} else {
			status.InstallCommand = "gradle dependencies"
		}

		// Check for build directory
		buildPath := filepath.Join(projectPath, "build")
		if _, err := os.Stat(buildPath); err == nil {
			status.Installed = true
		}

		return status
	}

	return status
}

// checkGoDependencies checks if Go dependencies are available
func checkGoDependencies(projectPath string) DependencyStatus {
	status := DependencyStatus{Manager: "go modules"}

	goModPath := filepath.Join(projectPath, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return status
	}

	status.ConfigFile = "go.mod"
	status.InstallCommand = "go mod download"

	// Check for go.sum as indicator of resolved dependencies
	goSumPath := filepath.Join(projectPath, "go.sum")
	if _, err := os.Stat(goSumPath); err == nil {
		status.Installed = true
	}

	return status
}

// checkRubyDependencies checks if Ruby dependencies are installed
func checkRubyDependencies(projectPath string) DependencyStatus {
	status := DependencyStatus{Manager: "bundler"}

	gemfilePath := filepath.Join(projectPath, "Gemfile")
	if _, err := os.Stat(gemfilePath); err != nil {
		return status
	}

	status.ConfigFile = "Gemfile"
	status.InstallCommand = "bundle install"

	// Check for Gemfile.lock as indicator
	lockPath := filepath.Join(projectPath, "Gemfile.lock")
	if _, err := os.Stat(lockPath); err == nil {
		status.Installed = true
	}

	return status
}

// checkRustDependencies checks if Rust dependencies are available
func checkRustDependencies(projectPath string) DependencyStatus {
	status := DependencyStatus{Manager: "cargo"}

	cargoPath := filepath.Join(projectPath, "Cargo.toml")
	if _, err := os.Stat(cargoPath); err != nil {
		return status
	}

	status.ConfigFile = "Cargo.toml"
	status.InstallCommand = "cargo fetch"

	// Check for Cargo.lock as indicator
	lockPath := filepath.Join(projectPath, "Cargo.lock")
	if _, err := os.Stat(lockPath); err == nil {
		status.Installed = true
	}

	return status
}

// detectMissingPythonPackages tries to detect missing Python packages
func detectMissingPythonPackages(projectPath string, reqPath string) []string {
	var missing []string

	// Read requirements.txt
	data, err := os.ReadFile(reqPath)
	if err != nil {
		return missing
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Extract package name (before ==, >=, <=, etc.)
		pkgName := line
		for _, sep := range []string{"==", ">=", "<=", "~=", "!=", ">"} {
			if idx := strings.Index(line, sep); idx > 0 {
				pkgName = line[:idx]
				break
			}
		}
		pkgName = strings.TrimSpace(pkgName)

		// Check if package is installed using pip show
		cmd := exec.Command("pip", "show", pkgName)
		if err := cmd.Run(); err != nil {
			missing = append(missing, pkgName)
		}
	}

	return missing
}

// InstallDependencies runs the installation command for the project
func InstallDependencies(projectPath string, installCommand string) error {
	parts := strings.Fields(installCommand)
	if len(parts) == 0 {
		return nil
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// VerifyInstallation re-runs diagnostics to verify installation was successful
func VerifyInstallation(projectPath string, language string) Diagnosis {
	return Diagnose(projectPath, language)
}
