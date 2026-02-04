package provisioner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PackageManager represents a detected package manager
type PackageManager string

const (
	NPM  PackageManager = "npm"
	PNPM PackageManager = "pnpm"
	Yarn PackageManager = "yarn"
	Bun  PackageManager = "bun"
)

// PackageManagerInfo contains details about the detected package manager
type PackageManagerInfo struct {
	Manager        PackageManager
	LockFile       string
	InstallCommand []string
	IsMonorepo     bool
	Installed      bool
	Version        string
}

// DetectPackageManager checks for lock files in the project root and returns
// the appropriate package manager. Priority: pnpm > yarn > npm
func DetectPackageManager(projectPath string) PackageManagerInfo {
	info := PackageManagerInfo{
		Manager:        NPM, // Default fallback
		InstallCommand: []string{"npm", "install"},
	}

	// Check for pnpm-lock.yaml first (highest priority)
	pnpmLockPath := filepath.Join(projectPath, "pnpm-lock.yaml")
	if _, err := os.Stat(pnpmLockPath); err == nil {
		info.Manager = PNPM
		info.LockFile = "pnpm-lock.yaml"
		info.IsMonorepo = detectPnpmWorkspace(projectPath)

		// Use recursive flag for monorepos
		if info.IsMonorepo {
			info.InstallCommand = []string{"pnpm", "install", "-r"}
		} else {
			info.InstallCommand = []string{"pnpm", "install"}
		}

		info.Installed, info.Version = checkManagerInstalled("pnpm")
		return info
	}

	// Check for pnpm-workspace.yaml (pnpm monorepo without lock file yet)
	pnpmWorkspacePath := filepath.Join(projectPath, "pnpm-workspace.yaml")
	if _, err := os.Stat(pnpmWorkspacePath); err == nil {
		info.Manager = PNPM
		info.LockFile = "pnpm-lock.yaml"
		info.IsMonorepo = true
		info.InstallCommand = []string{"pnpm", "install", "-r"}
		info.Installed, info.Version = checkManagerInstalled("pnpm")
		return info
	}

	// Check for workspace: protocol in package.json (pnpm-specific)
	// This handles cases where pnpm is used but lock file doesn't exist yet
	if usesPnpmWorkspaceProtocol(projectPath) {
		info.Manager = PNPM
		info.LockFile = "pnpm-lock.yaml"
		info.IsMonorepo = true
		info.InstallCommand = []string{"pnpm", "install", "-r"}
		info.Installed, info.Version = checkManagerInstalled("pnpm")
		return info
	}

	// Check for bun.lockb or bun.lock (Bun package manager)
	bunLockbPath := filepath.Join(projectPath, "bun.lockb")
	bunLockPath := filepath.Join(projectPath, "bun.lock")
	if _, err := os.Stat(bunLockbPath); err == nil {
		info.Manager = Bun
		info.LockFile = "bun.lockb"
		info.IsMonorepo = detectBunWorkspace(projectPath)
		info.InstallCommand = []string{"bun", "install"}
		info.Installed, info.Version = checkManagerInstalled("bun")
		return info
	}
	if _, err := os.Stat(bunLockPath); err == nil {
		info.Manager = Bun
		info.LockFile = "bun.lock"
		info.IsMonorepo = detectBunWorkspace(projectPath)
		info.InstallCommand = []string{"bun", "install"}
		info.Installed, info.Version = checkManagerInstalled("bun")
		return info
	}

	// Check for yarn.lock
	yarnLockPath := filepath.Join(projectPath, "yarn.lock")
	if _, err := os.Stat(yarnLockPath); err == nil {
		info.Manager = Yarn
		info.LockFile = "yarn.lock"
		info.IsMonorepo = detectYarnWorkspace(projectPath)
		info.InstallCommand = []string{"yarn", "install"}
		info.Installed, info.Version = checkManagerInstalled("yarn")
		return info
	}

	// Fallback to npm
	info.LockFile = "package-lock.json"
	info.Installed, info.Version = checkManagerInstalled("npm")
	return info
}

// usesPnpmWorkspaceProtocol checks if package.json uses workspace: protocol
// which is specific to pnpm and indicates the project requires pnpm
func usesPnpmWorkspaceProtocol(projectPath string) bool {
	packageJSONPath := filepath.Join(projectPath, "package.json")
	if data, err := os.ReadFile(packageJSONPath); err == nil {
		content := string(data)
		// workspace: protocol is pnpm-specific
		if strings.Contains(content, "\"workspace:") {
			return true
		}
	}
	return false
}

// detectPnpmWorkspace checks if this is a pnpm workspace/monorepo
func detectPnpmWorkspace(projectPath string) bool {
	// Check for pnpm-workspace.yaml
	workspacePath := filepath.Join(projectPath, "pnpm-workspace.yaml")
	if _, err := os.Stat(workspacePath); err == nil {
		return true
	}

	// Also check for workspace: protocol in package.json dependencies
	packageJSONPath := filepath.Join(projectPath, "package.json")
	if data, err := os.ReadFile(packageJSONPath); err == nil {
		content := string(data)
		if strings.Contains(content, "\"workspace:") {
			return true
		}
	}

	return false
}

// detectYarnWorkspace checks if this is a yarn workspace/monorepo
func detectYarnWorkspace(projectPath string) bool {
	packageJSONPath := filepath.Join(projectPath, "package.json")
	if data, err := os.ReadFile(packageJSONPath); err == nil {
		content := string(data)
		// Yarn workspaces are defined in package.json
		if strings.Contains(content, "\"workspaces\"") {
			return true
		}
	}

	return false
}

// detectBunWorkspace checks if this is a bun workspace/monorepo
func detectBunWorkspace(projectPath string) bool {
	packageJSONPath := filepath.Join(projectPath, "package.json")
	if data, err := os.ReadFile(packageJSONPath); err == nil {
		content := string(data)
		// Bun workspaces are defined in package.json similar to yarn
		if strings.Contains(content, "\"workspaces\"") {
			return true
		}
	}

	return false
}

// checkManagerInstalled checks if a package manager is installed and returns its version
func checkManagerInstalled(manager string) (bool, string) {
	cmd := exec.Command(manager, "--version")
	output, err := cmd.Output()
	if err != nil {
		return false, ""
	}
	return true, strings.TrimSpace(string(output))
}

// CheckResult represents the result of checking package manager availability
type CheckResult struct {
	Manager     PackageManager
	IsAvailable bool
	Version     string
	InstallHint string
	IsRequired  bool
	IsMonorepo  bool
}

// Check verifies if the required package manager is available
func Check(projectPath string) CheckResult {
	info := DetectPackageManager(projectPath)

	result := CheckResult{
		Manager:     info.Manager,
		IsAvailable: info.Installed,
		Version:     info.Version,
		IsMonorepo:  info.IsMonorepo,
		IsRequired:  info.Manager != NPM, // npm is typically pre-installed with node
	}

	// Provide installation hints for missing package managers
	if !info.Installed {
		switch info.Manager {
		case PNPM:
			result.InstallHint = "This project requires pnpm. Please run 'corepack enable pnpm' to continue."
		case Yarn:
			result.InstallHint = "This project requires yarn. Please run 'corepack enable yarn' to continue."
		case Bun:
			result.InstallHint = "This project requires bun. Please install it from https://bun.sh or run 'curl -fsSL https://bun.sh/install | bash'"
		case NPM:
			result.InstallHint = "npm is required. Please install Node.js from https://nodejs.org"
		}
	}

	return result
}

// InstallDependencies runs the appropriate install command for the detected package manager
func InstallDependencies(projectPath string) error {
	info := DetectPackageManager(projectPath)

	// First, verify the package manager is installed
	if !info.Installed {
		return fmt.Errorf("%s is not installed. %s", info.Manager, getInstallHint(info.Manager))
	}

	// Build the command
	if len(info.InstallCommand) == 0 {
		return fmt.Errorf("no install command configured for %s", info.Manager)
	}

	cmd := exec.Command(info.InstallCommand[0], info.InstallCommand[1:]...)
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// getInstallHint returns the installation hint for a package manager
func getInstallHint(manager PackageManager) string {
	switch manager {
	case PNPM:
		return "Please run 'corepack enable pnpm' to continue."
	case Yarn:
		return "Please run 'corepack enable yarn' to continue."
	case Bun:
		return "Please install bun from https://bun.sh or run 'curl -fsSL https://bun.sh/install | bash'"
	case NPM:
		return "Please install Node.js from https://nodejs.org"
	default:
		return ""
	}
}

// GetInstallCommand returns the install command for the detected package manager
func GetInstallCommand(projectPath string) []string {
	info := DetectPackageManager(projectPath)
	return info.InstallCommand
}

// GetManagerName returns a user-friendly name for the package manager
func GetManagerName(manager PackageManager) string {
	switch manager {
	case PNPM:
		return "pnpm"
	case Yarn:
		return "Yarn"
	case Bun:
		return "Bun"
	case NPM:
		return "npm"
	default:
		return string(manager)
	}
}
