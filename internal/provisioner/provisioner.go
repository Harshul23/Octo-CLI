package provisioner

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

// additionalPaths holds paths to newly installed binaries that should be added to PATH
var (
	additionalPaths   []string
	additionalPathsMu sync.RWMutex
)

// AddBinaryPath adds a path to the list of additional binary paths
// These paths will be injected into the environment when running commands
func AddBinaryPath(path string) {
	additionalPathsMu.Lock()
	defer additionalPathsMu.Unlock()

	// Avoid duplicates
	for _, p := range additionalPaths {
		if p == path {
			return
		}
	}
	additionalPaths = append(additionalPaths, path)
}

// GetAdditionalPaths returns all additional binary paths that have been registered
func GetAdditionalPaths() []string {
	additionalPathsMu.RLock()
	defer additionalPathsMu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]string, len(additionalPaths))
	copy(result, additionalPaths)
	return result
}

// ClearAdditionalPaths clears all registered additional paths
func ClearAdditionalPaths() {
	additionalPathsMu.Lock()
	defer additionalPathsMu.Unlock()
	additionalPaths = nil
}

// GetBinaryPaths returns common binary installation paths for various package managers
func GetBinaryPaths() map[PackageManager]string {
	home := os.Getenv("HOME")
	return map[PackageManager]string{
		Bun:  filepath.Join(home, ".bun", "bin"),
		PNPM: filepath.Join(home, ".local", "share", "pnpm"),
		Yarn: filepath.Join(home, ".yarn", "bin"),
		NPM:  filepath.Join(home, ".npm-global", "bin"),
	}
}

// GetBinaryPathForManager returns the typical binary path for a specific package manager
func GetBinaryPathForManager(manager PackageManager) string {
	paths := GetBinaryPaths()
	return paths[manager]
}

// BuildEnhancedEnvironment creates an environment slice with additional paths prepended to PATH
// This ensures newly installed binaries are immediately available
func BuildEnhancedEnvironment() []string {
	env := os.Environ()
	additionalPaths := GetAdditionalPaths()

	if len(additionalPaths) == 0 {
		return env
	}

	// Build the new PATH value
	newPathEntries := strings.Join(additionalPaths, string(os.PathListSeparator))
	currentPath := os.Getenv("PATH")
	newPath := newPathEntries + string(os.PathListSeparator) + currentPath

	// Replace or add PATH in the environment
	newEnv := make([]string, 0, len(env))
	pathFound := false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			newEnv = append(newEnv, "PATH="+newPath)
			pathFound = true
		} else {
			newEnv = append(newEnv, e)
		}
	}

	if !pathFound {
		newEnv = append(newEnv, "PATH="+newPath)
	}

	return newEnv
}

// BuildEnhancedEnvironmentWithTurbo creates an enhanced environment with Turbo-specific fixes
// It sets npm_config_user_agent to help Turbo detect the correct package manager
func BuildEnhancedEnvironmentWithTurbo(manager PackageManager, version string) []string {
	env := BuildEnhancedEnvironment()

	// Set npm_config_user_agent for Turbo compatibility
	// Format: <pm>/<version> npm/? node/<node-version> <os> <arch>
	if version == "" {
		version = "1.0.0" // fallback version
	}

	userAgent := fmt.Sprintf("%s/%s npm/? node/%s %s %s",
		string(manager),
		version,
		getNodeVersion(),
		getOS(),
		getArch(),
	)

	// Check if npm_config_user_agent already exists
	newEnv := make([]string, 0, len(env)+1)
	userAgentFound := false
	for _, e := range env {
		if strings.HasPrefix(e, "npm_config_user_agent=") {
			newEnv = append(newEnv, "npm_config_user_agent="+userAgent)
			userAgentFound = true
		} else {
			newEnv = append(newEnv, e)
		}
	}

	if !userAgentFound {
		newEnv = append(newEnv, "npm_config_user_agent="+userAgent)
	}

	return newEnv
}

// getNodeVersion returns the installed Node.js version or a fallback
func getNodeVersion() string {
	cmd := exec.Command("node", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	version := strings.TrimSpace(string(output))
	// Remove leading 'v' if present
	return strings.TrimPrefix(version, "v")
}

// getOS returns the current operating system name
func getOS() string {
	switch os := strings.ToLower(os.Getenv("GOOS")); os {
	case "darwin":
		return "darwin"
	case "linux":
		return "linux"
	case "windows":
		return "win32"
	default:
		// Try to detect from runtime
		return detectOS()
	}
}

func detectOS() string {
	// Check common indicators
	if _, err := os.Stat("/System/Library"); err == nil {
		return "darwin"
	}
	if _, err := os.Stat("/proc"); err == nil {
		return "linux"
	}
	return "unknown"
}

// getArch returns the current architecture
func getArch() string {
	arch := os.Getenv("GOARCH")
	if arch == "" {
		// Default fallback
		arch = "x64"
	}
	switch arch {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	case "386":
		return "ia32"
	default:
		return arch
	}
}

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

// isCommandAvailable checks if a command exists in the system PATH
func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// IsCommandAvailable is the exported version of isCommandAvailable
func IsCommandAvailable(name string) bool {
	return isCommandAvailable(name)
}

// PackageJSONConfig represents the relevant fields from package.json
type PackageJSONConfig struct {
	PackageManager string `json:"packageManager"`
}

// GetPackageManagerFromPackageJSON reads the packageManager field from package.json
// Returns the full string (e.g., "pnpm@9.1.4") or empty string if not found
func GetPackageManagerFromPackageJSON(projectPath string) string {
	packageJSONPath := filepath.Join(projectPath, "package.json")
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return ""
	}

	var config PackageJSONConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return ""
	}

	return config.PackageManager
}

// ParsePackageManagerSpec parses a packageManager string like "pnpm@9.1.4"
// Returns the manager name and version separately
func ParsePackageManagerSpec(spec string) (manager string, version string) {
	if spec == "" {
		return "", ""
	}

	// Match pattern like "pnpm@9.1.4" or "yarn@4.0.0"
	re := regexp.MustCompile(`^([a-z]+)@(.+)$`)
	matches := re.FindStringSubmatch(spec)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}

	// No version specified, just return the manager name
	return spec, ""
}

// CorepackResult represents the result of a corepack operation
type CorepackResult struct {
	Success           bool
	PermissionDenied  bool
	CorepackAvailable bool
	Error             error
	Message           string
}

// EnableCorepack attempts to enable a package manager via corepack
// It handles permission errors gracefully and provides user-friendly messages
func EnableCorepack(manager string) CorepackResult {
	result := CorepackResult{
		CorepackAvailable: isCommandAvailable("corepack"),
	}

	if !result.CorepackAvailable {
		result.Error = errors.New("corepack is not available")
		result.Message = fmt.Sprintf("❌ %s is required but not found. Please install Node.js (which includes Corepack) or install %s manually", manager, manager)
		return result
	}

	// Run corepack enable for the specific package manager
	cmd := exec.Command("corepack", "enable", manager)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it's a permission error (EACCES)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			outputStr := string(output)
			// Check for permission denied indicators
			if isPermissionError(err) || strings.Contains(outputStr, "EACCES") || strings.Contains(outputStr, "permission denied") || strings.Contains(outputStr, "Permission denied") {
				result.PermissionDenied = true
				result.Error = err
				result.Message = fmt.Sprintf("⚠️  Permission denied while enabling %s via Corepack.\n   Please run 'sudo corepack enable %s' once, then retry.", manager, manager)
				return result
			}
		}

		result.Error = fmt.Errorf("corepack enable %s failed: %w - %s", manager, err, string(output))
		result.Message = fmt.Sprintf("❌ Failed to enable %s via Corepack: %s", manager, strings.TrimSpace(string(output)))
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("✅ Successfully enabled %s via Corepack", manager)
	return result
}

// isPermissionError checks if an error is a permission denied error
func isPermissionError(err error) bool {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return errors.Is(pathErr.Err, syscall.EACCES)
	}
	return false
}

// EnsurePackageManagerResult contains the result of EnsurePackageManager
type EnsurePackageManagerResult struct {
	Manager            PackageManager
	Available          bool
	Version            string
	EnabledViaCorepack bool
	NeedsDownload      bool   // True if Corepack needs to download the PM on first use
	PinnedVersion      string // Version from packageManager field, if any
	Error              error
	UserMessage        string // Message to display to the user
}

// EnsurePackageManager checks if the required package manager is available,
// and attempts to enable it via Corepack if not. This allows Octo to bootstrap
// its environment on a fresh machine with only Node.js installed.
//
// The function:
// 1. Detects which package manager the project needs (based on lock files)
// 2. Checks if that package manager is already installed
// 3. If not, attempts to use Corepack as a fallback
// 4. Respects the packageManager field in package.json for version pinning
func EnsurePackageManager(projectPath string) EnsurePackageManagerResult {
	result := EnsurePackageManagerResult{}

	// Detect which package manager the project requires
	pmInfo := DetectPackageManager(projectPath)
	result.Manager = pmInfo.Manager

	// Check for packageManager field in package.json (version pinning)
	pmSpec := GetPackageManagerFromPackageJSON(projectPath)
	if pmSpec != "" {
		specManager, specVersion := ParsePackageManagerSpec(pmSpec)
		if specVersion != "" {
			result.PinnedVersion = specVersion
		}
		// If packageManager field specifies a different manager, prefer that
		if specManager != "" {
			switch specManager {
			case "pnpm":
				result.Manager = PNPM
			case "yarn":
				result.Manager = Yarn
			case "npm":
				result.Manager = NPM
			}
		}
	}

	managerName := string(result.Manager)

	// Check if the package manager is already available
	if isCommandAvailable(managerName) {
		result.Available = true
		_, result.Version = checkManagerInstalled(managerName)
		return result
	}

	// Package manager not found - try Corepack fallback for pnpm and yarn
	if result.Manager == PNPM || result.Manager == Yarn {
		// Attempt to enable via Corepack
		corepackResult := EnableCorepack(managerName)

		if corepackResult.PermissionDenied {
			result.Error = corepackResult.Error
			result.UserMessage = corepackResult.Message
			return result
		}

		if !corepackResult.Success {
			if !corepackResult.CorepackAvailable {
				result.Error = corepackResult.Error
				result.UserMessage = corepackResult.Message
			} else {
				result.Error = corepackResult.Error
				result.UserMessage = corepackResult.Message
			}
			return result
		}

		// Corepack enable succeeded
		result.EnabledViaCorepack = true
		result.Available = true
		result.NeedsDownload = true // Corepack will download on first use
		result.UserMessage = fmt.Sprintf("✅ Enabled %s via Corepack", managerName)

		// If there's a pinned version, Corepack will handle it automatically
		if result.PinnedVersion != "" {
			result.UserMessage = fmt.Sprintf("✅ Enabled %s@%s via Corepack", managerName, result.PinnedVersion)
		}

		return result
	}

	// For npm and bun, we can't use Corepack
	if result.Manager == NPM {
		result.Error = errors.New("npm is not installed")
		result.UserMessage = "❌ npm is required but not found. Please install Node.js from https://nodejs.org"
	} else if result.Manager == Bun {
		result.Error = errors.New("bun is not installed")
		result.UserMessage = "❌ bun is required but not found.\n   To install: curl -fsSL https://bun.sh/install | bash\n   Or use Node.js fallback with npm/pnpm instead."
	}

	return result
}

// BunInstallResult represents the result of attempting to install Bun
type BunInstallResult struct {
	Success      bool
	Error        error
	UserMessage  string
	UsedFallback bool
	FallbackPM   PackageManager
	BinaryPath   string // Path to the installed binary directory
}

// BunInstallCommand is the official Bun installation command
const BunInstallCommand = "curl -fsSL https://bun.sh/install | bash"

// InstallBun attempts to install Bun using the official installer
// Returns the result of the installation attempt
func InstallBun() BunInstallResult {
	result := BunInstallResult{}

	// Run the official Bun installer
	cmd := exec.Command("bash", "-c", "curl -fsSL https://bun.sh/install | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		result.Error = fmt.Errorf("failed to install bun: %w", err)
		result.UserMessage = "❌ Failed to install Bun. Please try manually: curl -fsSL https://bun.sh/install | bash"
		return result
	}

	// After installation, we need to reload PATH or source the shell config
	// The bun installer typically adds bun to ~/.bun/bin
	bunBinPath := filepath.Join(os.Getenv("HOME"), ".bun", "bin")
	bunPath := filepath.Join(bunBinPath, "bun")
	if _, err := os.Stat(bunPath); err == nil {
		// Add to current PATH for this session
		currentPath := os.Getenv("PATH")
		os.Setenv("PATH", bunBinPath+":"+currentPath)

		// Register the path for use by other components
		AddBinaryPath(bunBinPath)
		result.BinaryPath = bunBinPath
	}

	// Verify installation
	if isCommandAvailable("bun") {
		result.Success = true
		result.UserMessage = "✅ Bun installed successfully!"
	} else {
		// Installation succeeded but bun not in PATH yet - still register the path
		result.Success = true
		result.UserMessage = "✅ Bun installed! The binary is now available for this session."
	}

	return result
}

// PromptUserForBunInstall asks the user if they want to install Bun
// Returns true if user wants to install, false otherwise
func PromptUserForBunInstall(reader *bufio.Reader) bool {
	fmt.Println()
	fmt.Println("⚠️  Bun is required but not installed.")
	fmt.Println("   Would you like to install it now?")
	fmt.Println("   Command: curl -fsSL https://bun.sh/install | bash")
	fmt.Print("\n   Install Bun? [y/N]: ")

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// PromptUserForNodeFallback asks the user if they want to use npm/pnpm instead of Bun
// Returns true if user wants to use the fallback
func PromptUserForNodeFallback(reader *bufio.Reader) bool {
	fmt.Println()
	fmt.Println("💡 Most Bun projects are compatible with the Node.js ecosystem.")
	fmt.Println("   Would you like to use npm or pnpm as a fallback?")
	fmt.Print("\n   Use Node.js fallback? [y/N]: ")

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// GetNodeFallbackManager determines which Node.js package manager to use as a fallback
// Prefers pnpm (via Corepack) over npm
func GetNodeFallbackManager() (PackageManager, bool) {
	// First check if pnpm is available
	if isCommandAvailable("pnpm") {
		return PNPM, true
	}

	// Try to enable pnpm via Corepack
	if isCommandAvailable("corepack") {
		result := EnableCorepack("pnpm")
		if result.Success {
			return PNPM, true
		}
	}

	// Fallback to npm
	if isCommandAvailable("npm") {
		return NPM, true
	}

	return "", false
}

// EnsureBunWithFallback handles Bun projects with interactive installation and Node.js fallback
// This is the main entry point for handling Bun projects gracefully
type EnsureBunResult struct {
	Manager      PackageManager
	Available    bool
	Version      string
	UsedFallback bool
	Error        error
	UserMessage  string
	InstallCmd   []string
}

// EnsureBunWithFallback checks for Bun, offers to install it, or falls back to Node.js
// Pass nil for reader to use os.Stdin
func EnsureBunWithFallback(projectPath string, reader *bufio.Reader) EnsureBunResult {
	result := EnsureBunResult{
		Manager: Bun,
	}

	// Create reader if not provided
	if reader == nil {
		reader = bufio.NewReader(os.Stdin)
	}

	// Check if Bun is already available
	if isCommandAvailable("bun") {
		result.Available = true
		_, result.Version = checkManagerInstalled("bun")
		result.InstallCmd = []string{"bun", "install"}
		return result
	}

	// Bun not found - offer to install
	if PromptUserForBunInstall(reader) {
		fmt.Println()
		fmt.Println("⏳ Installing Bun...")

		installResult := InstallBun()
		if installResult.Success {
			result.Available = true
			_, result.Version = checkManagerInstalled("bun")
			result.UserMessage = installResult.UserMessage
			result.InstallCmd = []string{"bun", "install"}
			return result
		}

		// Installation failed - offer fallback
		fmt.Println(installResult.UserMessage)
	}

	// User declined Bun installation or it failed - offer Node.js fallback
	if PromptUserForNodeFallback(reader) {
		fallbackPM, available := GetNodeFallbackManager()
		if available {
			result.Manager = fallbackPM
			result.Available = true
			result.UsedFallback = true
			_, result.Version = checkManagerInstalled(string(fallbackPM))
			result.UserMessage = fmt.Sprintf("✅ Using %s as a fallback for Bun", fallbackPM)
			result.InstallCmd = []string{string(fallbackPM), "install"}
			return result
		}

		result.Error = errors.New("no Node.js package manager available for fallback")
		result.UserMessage = "❌ No Node.js package manager available. Please install Node.js from https://nodejs.org"
		return result
	}

	// User declined both options
	result.Error = errors.New("bun is required but not installed")
	result.UserMessage = "❌ Bun is required but not installed.\n   To install manually: curl -fsSL https://bun.sh/install | bash"
	return result
}

// ValidateRuntimeBeforeInstall validates that the required runtime binary exists before running install
// Returns an error with actionable fix instructions if the binary is missing
func ValidateRuntimeBeforeInstall(projectPath string) error {
	pmInfo := DetectPackageManager(projectPath)

	if len(pmInfo.InstallCommand) == 0 {
		return fmt.Errorf("no install command configured")
	}

	binary := pmInfo.InstallCommand[0]

	// Use exec.LookPath for proper PATH validation
	_, err := exec.LookPath(binary)
	if err != nil {
		return &RuntimeNotFoundError{
			Runtime:    binary,
			Manager:    pmInfo.Manager,
			FixCommand: getFixCommand(pmInfo.Manager),
		}
	}

	return nil
}

// RuntimeNotFoundError represents an error when a required runtime is not found
type RuntimeNotFoundError struct {
	Runtime    string
	Manager    PackageManager
	FixCommand string
}

func (e *RuntimeNotFoundError) Error() string {
	return fmt.Sprintf("%s is not installed", e.Runtime)
}

// getFixCommand returns the one-liner command to fix a missing package manager
func getFixCommand(manager PackageManager) string {
	switch manager {
	case Bun:
		return "curl -fsSL https://bun.sh/install | bash"
	case PNPM:
		return "corepack enable pnpm"
	case Yarn:
		return "corepack enable yarn"
	case NPM:
		return "Install Node.js from https://nodejs.org"
	default:
		return ""
	}
}

// GetFixCommand is the exported version of getFixCommand
func GetFixCommand(manager PackageManager) string {
	return getFixCommand(manager)
}

// RunWithCorepackProgress runs a package manager command with a progress indicator
// for when Corepack needs to download the package manager on first use
type ProgressCallback func(message string)

// RunInstallWithProgress runs the install command with progress feedback
// This is useful when Corepack might need to download the PM on first use
func RunInstallWithProgress(projectPath string, onProgress ProgressCallback) error {
	pmResult := EnsurePackageManager(projectPath)

	if !pmResult.Available {
		return pmResult.Error
	}

	// If Corepack needs to download, notify the user
	if pmResult.NeedsDownload && onProgress != nil {
		onProgress(fmt.Sprintf("⏳ Corepack is downloading %s...", pmResult.Manager))
	}

	// Get the install command
	pmInfo := DetectPackageManager(projectPath)
	if len(pmInfo.InstallCommand) == 0 {
		return fmt.Errorf("no install command configured for %s", pmInfo.Manager)
	}

	cmd := exec.Command(pmInfo.InstallCommand[0], pmInfo.InstallCommand[1:]...)
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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

	// Validate runtime binary exists in PATH before proceeding
	if err := ValidateRuntimeBeforeInstall(projectPath); err != nil {
		var rtErr *RuntimeNotFoundError
		if errors.As(err, &rtErr) {
			return fmt.Errorf("%s is not installed.\n   To fix: %s", rtErr.Runtime, rtErr.FixCommand)
		}
		return err
	}

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

// InstallDependenciesWithFallback runs install with interactive fallback support for Bun projects
func InstallDependenciesWithFallback(projectPath string, reader *bufio.Reader) error {
	info := DetectPackageManager(projectPath)

	// Special handling for Bun projects
	if info.Manager == Bun && !info.Installed {
		result := EnsureBunWithFallback(projectPath, reader)
		if !result.Available {
			return result.Error
		}

		if result.UserMessage != "" {
			fmt.Println(result.UserMessage)
		}

		// Use the resolved install command
		if len(result.InstallCmd) == 0 {
			return fmt.Errorf("no install command available")
		}

		cmd := exec.Command(result.InstallCmd[0], result.InstallCmd[1:]...)
		cmd.Dir = projectPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	// For non-Bun projects, use the standard flow
	return InstallDependencies(projectPath)
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
