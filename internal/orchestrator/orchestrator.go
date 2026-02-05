package orchestrator

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/harshul/octo-cli/internal/blueprint"
	"github.com/harshul/octo-cli/internal/ports"
	"github.com/harshul/octo-cli/internal/provisioner"
	"github.com/harshul/octo-cli/internal/secrets"
	"github.com/harshul/octo-cli/internal/thermal"
	"github.com/harshul/octo-cli/internal/ui"
)

// ExecutionPhase represents a phase in the boot sequence
type ExecutionPhase string

const (
	PhaseSetup ExecutionPhase = "setup"
	PhaseRun   ExecutionPhase = "run"
)

// Options controls how the orchestrator runs the application.
type Options struct {
	WorkDir       string
	Environment   string
	RunBuild      bool
	Watch         bool
	Detach        bool
	PortOverride  int  // If > 0, use this port instead of config default
	NoPortShift   bool // If true, disable automatic port shifting
	SkipSetup     bool // If true, skip the setup phase
	SkipEnvCheck  bool // If true, skip environment variable validation
	UseDashboard  bool // If true, use TUI dashboard instead of scrolling output
}

type Orchestrator struct {
	bp          blueprint.Blueprint
	opts        Options
	envVars     map[string]string // Loaded env vars for global injection
	hwInfo      thermal.HardwareInfo
	concurrency int
	batchSize   int
	dashboard   *ui.DashboardRunner // Optional TUI dashboard
}

func New(bp blueprint.Blueprint, opts Options) (*Orchestrator, error) {
	// Detect hardware for thermal management
	hwInfo := thermal.DetectHardware()

	// Determine concurrency based on hardware and config
	concurrency := thermal.GetOptimalConcurrency(hwInfo, bp.Thermal.Concurrency)

	// If thermal mode is "performance", use all cores
	if bp.Thermal.Mode == "performance" {
		concurrency = hwInfo.NumCPU
	} else if bp.Thermal.Mode == "cool" {
		// In "cool" mode, be more conservative
		concurrency = hwInfo.NumCPU / 2
		if concurrency < 1 {
			concurrency = 1
		}
	}

	o := &Orchestrator{
		bp:          bp,
		opts:        opts,
		envVars:     make(map[string]string),
		hwInfo:      hwInfo,
		concurrency: concurrency,
		batchSize:   bp.Thermal.BatchSize,
	}

	// Initialize dashboard if requested
	if opts.UseDashboard {
		projects := []*ui.Project{
			ui.NewProject(bp.Name, opts.WorkDir),
		}
		o.dashboard = ui.NewDashboardRunner(ui.DashboardConfig{
			Projects:       projects,
			MaxConcurrency: concurrency,
		})
	}

	return o, nil
}

// runtimeCommands maps language names to their runtime check commands.
var runtimeCommands = map[string]string{
	"node":       "node",
	"nodejs":     "node",
	"javascript": "node",
	"typescript": "node",
	"java":       "java",
	"python":     "python3",
	"go":         "go",
	"golang":     "go",
	"ruby":       "ruby",
	"rust":       "cargo",
}

// checkRuntime checks if the required runtime is available on the host machine.
func (o *Orchestrator) checkRuntime() {
	if o.bp.Language == "" {
		return
	}

	lang := strings.ToLower(o.bp.Language)
	runtimeCmd, ok := runtimeCommands[lang]
	if !ok {
		// Unknown language, skip the check
		return
	}

	_, err := exec.LookPath(runtimeCmd)
	if err != nil {
		fmt.Printf("⚠️  Warning: %s not found. Please install it.\n", o.bp.Language)
	}
}

// displayThermalInfo shows hardware and thermal configuration information
func (o *Orchestrator) displayThermalInfo() {
	// Only display detailed info for monorepos or when thermal mode is explicitly set
	if !o.bp.IsMonorepo && o.bp.Thermal.Mode == "" {
		return
	}

	hwDesc := thermal.FormatHardwareInfo(o.hwInfo)
	fmt.Printf("🖥️  Hardware: %s\n", hwDesc)

	// Determine what mode we're running in
	modeDesc := "auto"
	if o.bp.Thermal.Mode != "" {
		modeDesc = o.bp.Thermal.Mode
	}

	// Show concurrency info
	if o.hwInfo.IsMacBookAir && modeDesc != "performance" {
		fmt.Printf("🌡️  Thermal mode: %s (MacBook Air detected - reduced concurrency for quiet operation)\n", modeDesc)
	} else if o.hwInfo.IsDarwin && o.hwInfo.IsAppleSilicon && modeDesc != "performance" {
		fmt.Printf("🌡️  Thermal mode: %s (Apple Silicon - optimized concurrency)\n", modeDesc)
	}

	fmt.Printf("⚡ Concurrency: %d workers\n", o.concurrency)

	// Check current thermal status on macOS
	if o.hwInfo.IsDarwin && (modeDesc == "auto" || modeDesc == "cool") {
		status := thermal.GetThermalStatus(o.hwInfo)
		if status.Level != "cool" {
			fmt.Printf("🌡️  Thermal status: %s - %s\n", status.Level, status.Message)
		}
	}
}

// injectConcurrencyFlags adds concurrency flags to supported tools in the command
func (o *Orchestrator) injectConcurrencyFlags(command string) string {
	// Skip if performance mode - let tools use their defaults
	if o.bp.Thermal.Mode == "performance" {
		return command
	}

	return thermal.InjectConcurrencyFlag(command, o.concurrency)
}

func (o *Orchestrator) Run() error {
	fmt.Printf("🚀 Starting %s (env=%s, build=%v, watch=%v, detach=%v)\n",
		o.bp.Name, o.opts.Environment, o.opts.RunBuild, o.opts.Watch, o.opts.Detach)

	// Display thermal/hardware info
	o.displayThermalInfo()

	// Handle options that are currently not implemented to avoid silently ignoring them.
	if o.opts.Watch {
		fmt.Println("⚠️  Warning: Watch option is not implemented yet; changes will not be watched automatically.")
	}
	if o.opts.Detach {
		fmt.Println("⚠️  Warning: Detach option is not implemented yet; the process will run in the foreground.")
	}
	// Check if the required runtime is available
	o.checkRuntime()

	// Determine working directory
	// For monorepos, use the monorepo root if specified
	workDir := o.opts.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	if o.bp.IsMonorepo && o.bp.MonorepoRoot != "" {
		// Use monorepo root as the working directory
		if info, err := os.Stat(o.bp.MonorepoRoot); err == nil && info.IsDir() {
			workDir = o.bp.MonorepoRoot
			fmt.Printf("📂 Using monorepo root: %s\n", workDir)
		} else {
			fmt.Printf("⚠️  Warning: monorepo_root %s does not exist, using current directory\n", o.bp.MonorepoRoot)
		}
	}

	// ==========================================
	// PHASE 0: Monorepo Linking (for pnpm workspaces)
	// ==========================================
	if o.bp.IsMonorepo && o.bp.PackageManager == "pnpm" {
		if err := o.ensurePnpmWorkspaceLinked(workDir); err != nil {
			fmt.Printf("⚠️  Warning: pnpm workspace linking failed: %v\n", err)
		}
	}

	// Check and install dependencies if needed (e.g., node_modules for Node projects)
	if err := o.checkAndInstallDependencies(workDir); err != nil {
		fmt.Printf("⚠️  Warning: dependency check failed: %v\n", err)
	}

	// Check environment variables (unless skipped)
	if !o.opts.SkipEnvCheck {
		if err := o.checkEnvVars(); err != nil {
			return err
		}
	} else {
		// Still load env vars for injection even if we skip validation
		o.loadEnvVarsForInjection(workDir)
	}

	// ==========================================
	// PHASE 1: Setup Phase (Mandatory Pre-Run)
	// ==========================================
	if o.bp.SetupRequired && o.bp.SetupCommand != "" && !o.opts.SkipSetup {
		fmt.Println("\n📋 ═══════════════════════════════════════════════")
		fmt.Println("   PHASE 1: Setup (Mandatory Pre-Run)")
		fmt.Println("   ═══════════════════════════════════════════════")
		fmt.Printf("   Command: %s\n", o.bp.SetupCommand)
		fmt.Println("   ═══════════════════════════════════════════════")
		fmt.Println()

		if err := o.executeSetupPhase(workDir, o.bp.SetupCommand); err != nil {
			return fmt.Errorf("setup phase failed (this is a mandatory step): %w", err)
		}

		fmt.Println("\n✅ Setup phase completed successfully!")
		fmt.Println()
	}

	// ==========================================
	// PHASE 2: Run Phase
	// ==========================================
	if o.bp.SetupRequired && o.bp.SetupCommand != "" && !o.opts.SkipSetup {
		fmt.Println("📋 ═══════════════════════════════════════════════")
		fmt.Println("   PHASE 2: Run")
		fmt.Println("   ═══════════════════════════════════════════════")
		fmt.Println()
	}

	// Check if we have a run command
	if o.bp.RunCommand == "" {
		return fmt.Errorf("no run command specified in configuration")
	}

	// Start with the configured run command
	runCommand := o.bp.RunCommand

	// Auto-build logic: If run command references a local binary (./), check for build requirements
	if err := o.autoBuildIfNeeded(workDir, runCommand); err != nil {
		return fmt.Errorf("auto-build failed: %w", err)
	}

	// Check if this is a simple HTML project (opens in browser)
	isHTMLProject := strings.ToLower(o.bp.Language) == "html"
	
	// Handle port override if specified (skip for HTML projects)
	if !isHTMLProject {
		// First, check if there's already a process on the target port
		portInfo := ports.ExtractPort(runCommand)
		if portInfo.Found {
			if processOnPort := o.checkProcessOnPort(portInfo.Port); processOnPort {
				if !o.opts.NoPortShift {
					// Find an available port and shift
					newPort := ports.FindAvailablePort(portInfo.Port + 1)
					if newPort > 0 {
						fmt.Printf("⚠️  Port %d already has a running process. Shifting to %d.\n", portInfo.Port, newPort)
						runCommand = ports.ShiftPort(runCommand, portInfo.Port, newPort)
					} else {
						fmt.Printf("⚠️  Port %d is busy and no available ports found nearby.\n", portInfo.Port)
					}
				} else {
					fmt.Printf("⚠️  Port %d already has a running process. Use --no-port-shift=false to auto-shift.\n", portInfo.Port)
				}
			}
		}

		if o.opts.PortOverride > 0 {
			portInfo := ports.ExtractPort(runCommand)
			if portInfo.Found {
				runCommand = ports.ShiftPort(runCommand, portInfo.Port, o.opts.PortOverride)
				fmt.Printf("📌 Using specified port %d\n", o.opts.PortOverride)
			} else {
				// No port flag exists, append one based on language
				runCommand = ports.AppendPortFlag(runCommand, o.bp.Language, o.opts.PortOverride)
				fmt.Printf("📌 Adding port %d to command\n", o.opts.PortOverride)
			}
		} else if !o.opts.NoPortShift {
			// Check for port conflicts and auto-shift if needed
			newCommand, newPort, wasShifted, err := ports.CheckAndShift(runCommand)
			if err != nil {
				fmt.Printf("⚠️  Warning: %v\n", err)
			} else if wasShifted {
				// Extract original port for the message
				portInfo := ports.ExtractPort(runCommand)
				fmt.Printf("⚠️  Port %d busy, shifting command to %d.\n", portInfo.Port, newPort)
				runCommand = newCommand
			}
		}
	}

	// Parse and execute the run command with proper path handling
	// Handle nested commands like "cd frontend && npm start"
	if err := o.executeWithPathCorrection(workDir, runCommand, isHTMLProject); err != nil {
		return err
	}

	return nil
}

// checkEnvVars verifies environment variables and gives user option to skip.
// It also attempts to auto-bootstrap from templates if .env files are missing.
func (o *Orchestrator) checkEnvVars() error {
	workDir := o.opts.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// ==========================================
	// Step 1: Auto-bootstrap from templates
	// ==========================================
	bootstrapped, bootstrappedPaths, err := secrets.AutoBootstrapEnvFiles(workDir)
	if err != nil {
		fmt.Printf("⚠️  Warning: template detection failed: %v\n", err)
	} else if bootstrapped > 0 {
		fmt.Printf("📋 Auto-bootstrapped %d .env file(s) from templates:\n", bootstrapped)
		for _, path := range bootstrappedPaths {
			fmt.Printf("   ✅ %s\n", path)
		}
		fmt.Println()
	}

	// ==========================================
	// Step 2: Load all env vars for global injection
	// ==========================================
	o.loadEnvVarsForInjection(workDir)

	if len(o.bp.EnvVars) == 0 {
		return nil
	}

	// Build a map of all defined env vars from .env files AND current environment
	definedVars := make(map[string]bool)
	
	// First, check current environment
	for _, v := range o.bp.EnvVars {
		if os.Getenv(v.Name) != "" {
			definedVars[v.Name] = true
		}
	}
	
	// Then, read from .env files in the project (root + common subdirectories)
	envFilePaths := []string{
		filepath.Join(workDir, ".env"),
		filepath.Join(workDir, ".env.local"),
		filepath.Join(workDir, "apps/client/.env"),
		filepath.Join(workDir, "apps/client/.env.local"),
		filepath.Join(workDir, "apps/server/.env"),
		filepath.Join(workDir, "apps/server/.env.local"),
		filepath.Join(workDir, "apps/web/.env"),
		filepath.Join(workDir, "apps/api/.env"),
	}
	
	for _, envPath := range envFilePaths {
		if envVars, err := secrets.ReadEnvFile(envPath); err == nil {
			for k := range envVars {
				definedVars[k] = true
			}
		}
	}

	var missingRequired []string
	var missingOptional []string

	for _, v := range o.bp.EnvVars {
		if !definedVars[v.Name] {
			if v.Required {
				missingRequired = append(missingRequired, v.Name)
			} else {
				missingOptional = append(missingOptional, v.Name)
			}
		}
	}

	// If no missing variables at all, proceed
	if len(missingRequired) == 0 && len(missingOptional) == 0 {
		return nil
	}

	// If only optional variables are missing, just log and proceed
	if len(missingRequired) == 0 {
		if len(missingOptional) > 0 {
			fmt.Printf("ℹ️  Note: %d optional environment variable(s) not set. Proceeding anyway.\n", len(missingOptional))
		}
		return nil
	}

	// Required variables are missing - give user choice
	fmt.Printf("\n⚠️  Missing %d environment variable(s) that may be needed:\n", len(missingRequired))
	for _, name := range missingRequired {
		fmt.Printf("   • %s\n", name)
	}
	fmt.Println()

	// Ask user what they want to do
	fmt.Print("Options: [s]kip and run anyway, [p]rovide values, [q]uit? (s/p/q): ")
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(strings.ToLower(text))

	switch text {
	case "s", "skip", "":
		// User chose to skip - proceed without the env vars
		fmt.Println("⏭️  Skipping environment variables. The app may not work correctly.")
		return nil
	case "q", "quit", "exit":
		return fmt.Errorf("aborted by user")
	case "p", "provide", "y", "yes":
		// User wants to provide values
		descriptions := make(map[string]string)
		for _, name := range missingRequired {
			descriptions[name] = "Environment variable"
		}

		values := ui.PromptForSecrets(missingRequired, descriptions)

		// Set the provided values in the current process environment
		// and add to envVars for global injection
		for k, v := range values {
			if v != "" {
				os.Setenv(k, v)
				o.envVars[k] = v
			}
		}

		fmt.Println("✅ Environment variables set for this session.")
		return nil
	default:
		// Unknown input - default to skip
		fmt.Println("⏭️  Skipping environment variables. The app may not work correctly.")
		return nil
	}
}

// loadEnvVarsForInjection loads all env vars from .env files for global injection
// into command environments. This ensures all phases (Setup, Build, Run) have
// access to the same environment variables.
func (o *Orchestrator) loadEnvVarsForInjection(workDir string) {
	// Get all env vars from .env files
	allVars := secrets.GetAllEnvVars(workDir)
	
	// Merge into orchestrator's envVars map
	for k, v := range allVars {
		if _, exists := o.envVars[k]; !exists {
			o.envVars[k] = v
		}
	}

	// Also add any env vars from the current environment that match blueprint requirements
	for _, ev := range o.bp.EnvVars {
		if val := os.Getenv(ev.Name); val != "" {
			if _, exists := o.envVars[ev.Name]; !exists {
				o.envVars[ev.Name] = val
			}
		}
	}

	if len(o.envVars) > 0 {
		fmt.Printf("🔐 Loaded %d environment variable(s) for global injection\n", len(o.envVars))
	}
}

// buildEnvWithSecrets creates an environment slice with all detected/provided secrets
// injected. This is used for all command executions (Setup, Build, Run phases).
func (o *Orchestrator) buildEnvWithSecrets(baseEnv []string) []string {
	if len(o.envVars) == 0 {
		return baseEnv
	}

	// Create a map of existing env vars for quick lookup
	existingVars := make(map[string]int) // key -> index in baseEnv
	for i, e := range baseEnv {
		if idx := strings.Index(e, "="); idx > 0 {
			key := e[:idx]
			existingVars[key] = i
		}
	}

	// Create result slice
	result := make([]string, len(baseEnv))
	copy(result, baseEnv)

	// Add or update env vars from our loaded secrets
	for key, value := range o.envVars {
		envEntry := key + "=" + value
		if idx, exists := existingVars[key]; exists {
			// Update existing entry
			result[idx] = envEntry
		} else {
			// Append new entry
			result = append(result, envEntry)
		}
	}

	return result
}

// checkAndInstallDependencies checks for project dependencies and installs them if missing.
// Supports: Node.js with npm, pnpm, or yarn (auto-detected from lock files)
func (o *Orchestrator) checkAndInstallDependencies(workDir string) error {
	// Check for Node.js project (package.json)
	packageJSONPath := filepath.Join(workDir, "package.json")
	if _, err := os.Stat(packageJSONPath); err == nil {
		if err := o.installNodeDependencies(workDir, ""); err != nil {
			return err
		}
	}

	// Check for nested frontend directories (common in Go + React projects)
	frontendDirs := []string{"frontend", "client", "web", "ui"}
	for _, dir := range frontendDirs {
		frontendPath := filepath.Join(workDir, dir)
		packageJSONPath := filepath.Join(frontendPath, "package.json")
		if _, err := os.Stat(packageJSONPath); err == nil {
			if err := o.installNodeDependencies(frontendPath, dir); err != nil {
				return err
			}
		}
	}

	return nil
}

// installNodeDependencies installs Node.js dependencies using the detected package manager.
// It checks for lock files to determine whether to use npm, pnpm, or yarn.
// It uses enhanced environment to ensure newly installed package managers are available.
func (o *Orchestrator) installNodeDependencies(projectPath string, subDir string) error {
	nodeModulesPath := filepath.Join(projectPath, "node_modules")
	if _, err := os.Stat(nodeModulesPath); err == nil {
		// node_modules already exists, skip installation
		return nil
	}

	// Detect the package manager
	pmCheck := provisioner.Check(projectPath)

	// Check if the required package manager is installed
	if !pmCheck.IsAvailable {
		return fmt.Errorf("%s", pmCheck.InstallHint)
	}

	// Get the install command
	installCmd := provisioner.GetInstallCommand(projectPath)
	if len(installCmd) == 0 {
		return fmt.Errorf("no install command found for package manager")
	}

	// Build the display message
	managerName := provisioner.GetManagerName(pmCheck.Manager)
	if subDir != "" {
		if pmCheck.IsMonorepo {
			fmt.Printf("📦 Detected %s monorepo in %s/. Running %s...\n", managerName, subDir, strings.Join(installCmd, " "))
		} else {
			fmt.Printf("📦 Detected package.json in %s/ but node_modules is missing. Running %s...\n", subDir, strings.Join(installCmd, " "))
		}
	} else {
		if pmCheck.IsMonorepo {
			fmt.Printf("📦 Detected %s monorepo. Running %s...\n", managerName, strings.Join(installCmd, " "))
		} else {
			fmt.Printf("📦 Detected package.json but node_modules is missing. Running %s...\n", strings.Join(installCmd, " "))
		}
	}

	// Execute the install command with enhanced environment
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, installCmd[0], installCmd[1:]...)
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Use enhanced environment to ensure newly installed binaries are available
	cmd.Env = provisioner.BuildEnhancedEnvironment()

	if err := cmd.Run(); err != nil {
		if subDir != "" {
			return fmt.Errorf("%s in %s failed: %w", strings.Join(installCmd, " "), subDir, err)
		}
		return fmt.Errorf("%s failed: %w", strings.Join(installCmd, " "), err)
	}

	if subDir != "" {
		fmt.Printf("✅ Dependencies installed successfully in %s/.\n", subDir)
	} else {
		fmt.Println("✅ Dependencies installed successfully.")
	}

	return nil
}

// autoBuildIfNeeded checks if the run command references a local binary and builds it if necessary.
// This supports Makefile and Go projects.
func (o *Orchestrator) autoBuildIfNeeded(workDir string, runCommand string) error {
	// Check if the run command references a local binary (starts with ./)
	if !strings.HasPrefix(runCommand, "./") {
		// Also check for commands that might use a binary after && or ;
		parts := strings.FieldsFunc(runCommand, func(r rune) bool {
			return r == '&' || r == ';' || r == '|'
		})
		hasLocalBinary := false
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "./") {
				hasLocalBinary = true
				break
			}
		}
		if !hasLocalBinary {
			return nil
		}
	}

	// Extract the binary path from the command
	binaryPath := extractBinaryPath(runCommand)
	if binaryPath == "" {
		return nil
	}

	// Check if the binary already exists
	fullBinaryPath := filepath.Join(workDir, binaryPath)
	if _, err := os.Stat(fullBinaryPath); err == nil {
		// Binary exists, skip build unless RunBuild is explicitly set
		if !o.opts.RunBuild {
			return nil
		}
	}

	fmt.Printf("🔨 Local binary %s not found or build requested. Attempting auto-build...\n", binaryPath)

	// Check for Makefile
	makefilePath := filepath.Join(workDir, "Makefile")
	if _, err := os.Stat(makefilePath); err == nil {
		fmt.Println("📋 Found Makefile. Running make...")
		cmd := exec.Command("make")
		cmd.Dir = workDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("make failed: %w", err)
		}
		fmt.Println("✅ Build completed successfully.")
		return nil
	}

	// Check for Go project (go.mod)
	goModPath := filepath.Join(workDir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		fmt.Println("📋 Found go.mod. Running go build...")
		
		// Determine the output binary name
		outputName := strings.TrimPrefix(binaryPath, "./")
		
		// Check if there's a cmd directory
		cmdDir := filepath.Join(workDir, "cmd")
		var cmd *exec.Cmd
		if _, err := os.Stat(cmdDir); err == nil {
			// Build from cmd directory
			cmd = exec.Command("go", "build", "-o", outputName, "./cmd/...")
		} else {
			// Build from root
			cmd = exec.Command("go", "build", "-o", outputName, ".")
		}
		
		cmd.Dir = workDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("go build failed: %w", err)
		}
		fmt.Println("✅ Build completed successfully.")
		return nil
	}

	// No supported build system found
	fmt.Printf("⚠️  No Makefile or go.mod found. Cannot auto-build %s.\n", binaryPath)
	return nil
}

// extractBinaryPath extracts the local binary path from a run command.
// e.g., "./bin/app --flag" -> "./bin/app"
//       "make && ./app" -> "./app"
func extractBinaryPath(runCommand string) string {
	// Split by common command separators
	parts := strings.FieldsFunc(runCommand, func(r rune) bool {
		return r == '&' || r == ';' || r == '|'
	})

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "./") {
			// Extract just the binary path (first word)
			fields := strings.Fields(part)
			if len(fields) > 0 {
				return fields[0]
			}
		}
	}

	// Check the beginning of the command directly
	if strings.HasPrefix(runCommand, "./") {
		fields := strings.Fields(runCommand)
		if len(fields) > 0 {
			return fields[0]
		}
	}

	return ""
}

// executeWithPathCorrection executes a command with proper handling of directory changes.
// It correctly handles nested commands like "cd frontend && npm start" by
// resolving the working directory for each sub-command.
// It also injects the enhanced environment with newly installed binary paths
// and all detected/provided secrets for global availability.
// Thermal management: Automatically injects concurrency flags for supported tools.
func (o *Orchestrator) executeWithPathCorrection(workDir string, runCommand string, isHTMLProject bool) error {
	// Check if the command contains directory changes
	resolvedWorkDir, resolvedCommand := o.resolveNestedCommand(workDir, runCommand)

	// Inject concurrency flags for thermal management
	resolvedCommand = o.injectConcurrencyFlags(resolvedCommand)

	// Detect the package manager for this project
	pmInfo := provisioner.DetectPackageManager(resolvedWorkDir)

	// Build the enhanced environment with additional paths
	var baseEnv []string
	if o.usesTurbo(resolvedCommand) {
		// For Turbo, add npm_config_user_agent to help it detect the package manager
		baseEnv = provisioner.BuildEnhancedEnvironmentWithTurbo(pmInfo.Manager, pmInfo.Version)
		fmt.Printf("🔧 Turbo detected - setting npm_config_user_agent for %s\n", pmInfo.Manager)
	} else {
		baseEnv = provisioner.BuildEnhancedEnvironment()
	}

	// Inject all detected/provided secrets into the environment
	env := o.buildEnvWithSecrets(baseEnv)

	// Log if we're using additional paths
	additionalPaths := provisioner.GetAdditionalPaths()
	if len(additionalPaths) > 0 {
		fmt.Printf("🔗 Injecting %d additional binary path(s) into environment\n", len(additionalPaths))
	}

	// Parse and execute the run command
	// Use shell to handle complex commands with pipes, redirects, etc.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", resolvedCommand)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", resolvedCommand)
	}

	// Set the resolved working directory
	cmd.Dir = resolvedWorkDir

	// Set the enhanced environment with secrets
	cmd.Env = env

	// For HTML projects, we just open the browser and exit
	if isHTMLProject {
		fmt.Printf("🌐 Opening in browser: %s\n", resolvedCommand)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to open browser: %w", err)
		}
		fmt.Println("✅ Opened in default browser!")
		return nil
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if resolvedWorkDir != workDir {
		fmt.Printf("📂 Working directory: %s\n", resolvedWorkDir)
	}
	fmt.Printf("📦 Executing: %s\n", resolvedCommand)

	// Run the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// usesTurbo checks if the command uses Turbo (turborepo)
func (o *Orchestrator) usesTurbo(command string) bool {
	lowerCmd := strings.ToLower(command)
	return strings.Contains(lowerCmd, "turbo") ||
		strings.Contains(lowerCmd, "turbo run") ||
		strings.Contains(lowerCmd, "turbo build") ||
		strings.Contains(lowerCmd, "turbo dev")
}

// resolveNestedCommand handles commands with directory changes like "cd frontend && npm start".
// It returns the resolved working directory and the remaining command to execute.
func (o *Orchestrator) resolveNestedCommand(workDir string, runCommand string) (string, string) {
	// Check for patterns like "cd <dir> && <command>" or "cd <dir>; <command>"
	cdPatterns := []string{" && ", "; ", " & "}
	
	for _, pattern := range cdPatterns {
		if strings.Contains(runCommand, pattern) {
			parts := strings.SplitN(runCommand, pattern, 2)
			if len(parts) == 2 {
				firstPart := strings.TrimSpace(parts[0])
				remainder := strings.TrimSpace(parts[1])
				
				// Check if the first part is a cd command
				if strings.HasPrefix(firstPart, "cd ") {
					targetDir := strings.TrimPrefix(firstPart, "cd ")
					targetDir = strings.TrimSpace(targetDir)
					
					// Resolve the target directory relative to workDir
					var resolvedDir string
					if filepath.IsAbs(targetDir) {
						resolvedDir = targetDir
					} else {
						resolvedDir = filepath.Join(workDir, targetDir)
					}
					
					// Verify the directory exists
					if info, err := os.Stat(resolvedDir); err == nil && info.IsDir() {
						// Check for dependencies in the new directory
						if err := o.checkAndInstallDependencies(resolvedDir); err != nil {
							fmt.Printf("⚠️  Warning: dependency check in %s failed: %v\n", targetDir, err)
						}
						
						// Recursively resolve any further cd commands in remainder
						return o.resolveNestedCommand(resolvedDir, remainder)
					} else {
						// Directory doesn't exist, return original command
						fmt.Printf("⚠️  Warning: directory %s does not exist\n", targetDir)
						return workDir, runCommand
					}
				}
			}
		}
	}
	
	// No cd command found or pattern doesn't match, return as-is
	return workDir, runCommand
}

// executeSetupPhase runs the setup phase command and waits for it to complete with exit code 0.
// This is a blocking operation that must complete successfully before the run phase can start.
// It injects all detected/provided environment variables for global availability.
// Thermal management: Automatically injects concurrency flags for supported tools.
func (o *Orchestrator) executeSetupPhase(workDir string, setupCommand string) error {
	// Resolve any nested directory changes in the setup command
	resolvedWorkDir, resolvedCommand := o.resolveNestedCommand(workDir, setupCommand)

	// Inject concurrency flags for thermal management
	resolvedCommand = o.injectConcurrencyFlags(resolvedCommand)

	// Build the enhanced environment with all detected secrets injected
	baseEnv := provisioner.BuildEnhancedEnvironment()
	env := o.buildEnvWithSecrets(baseEnv)

	// Create a context with a generous timeout for setup (30 minutes for large monorepos)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", resolvedCommand)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", resolvedCommand)
	}

	cmd.Dir = resolvedWorkDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if resolvedWorkDir != workDir {
		fmt.Printf("📂 Working directory: %s\n", resolvedWorkDir)
	}
	fmt.Printf("🔧 Executing setup: %s\n", resolvedCommand)

	// Run the setup command and wait for completion
	if err := cmd.Run(); err != nil {
		// Check if it was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("setup command timed out after 30 minutes")
		}
		return fmt.Errorf("setup command exited with error: %w", err)
	}

	return nil
}

// checkProcessOnPort checks if there's already a process listening on the given port.
// This helps prevent "force killing" issues by detecting port conflicts before spawning.
func (o *Orchestrator) checkProcessOnPort(port int) bool {
	// Use the ports package to check availability
	return !ports.IsPortAvailable(port)
}

// ensurePnpmWorkspaceLinked ensures that pnpm workspace links are properly set up.
// For pnpm monorepos, this runs `pnpm install` at the root to create all workspace links.
func (o *Orchestrator) ensurePnpmWorkspaceLinked(workDir string) error {
	// Check if pnpm-workspace.yaml exists
	pnpmWorkspacePath := filepath.Join(workDir, "pnpm-workspace.yaml")
	if _, err := os.Stat(pnpmWorkspacePath); os.IsNotExist(err) {
		// Not a pnpm workspace, nothing to do
		return nil
	}

	// Check if node_modules exists and has .pnpm directory (indicates linked workspace)
	pnpmDir := filepath.Join(workDir, "node_modules", ".pnpm")
	if _, err := os.Stat(pnpmDir); err == nil {
		// Already linked
		return nil
	}

	fmt.Println("📦 Detected pnpm workspace. Running pnpm install to link packages...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pnpm", "install")
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = provisioner.BuildEnhancedEnvironment()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pnpm install failed: %w", err)
	}

	fmt.Println("✅ pnpm workspace linked successfully!")
	return nil
}

// GetProcessInfoOnPort returns information about a process listening on a port (if any).
// This is useful for debugging port conflicts.
func (o *Orchestrator) GetProcessInfoOnPort(port int) (string, error) {
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "darwin", "linux":
		// Use lsof to find the process
		cmd = exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-t")
	case "windows":
		// Use netstat on Windows
		cmd = exec.Command("netstat", "-ano", "-p", "TCP")
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	output, err := cmd.Output()
	if err != nil {
		// No process found or command failed
		return "", nil
	}

	return strings.TrimSpace(string(output)), nil
}

// ==========================================
// Thermal & Resource Management - Batch Processing
// ==========================================

// MonorepoPackage represents a package in a monorepo
type MonorepoPackage struct {
	Name string
	Path string
}

// DetectMonorepoPackages detects packages in a monorepo workspace
func (o *Orchestrator) DetectMonorepoPackages(workDir string) ([]MonorepoPackage, error) {
	var packages []MonorepoPackage

	// Check for pnpm workspace
	pnpmWorkspacePath := filepath.Join(workDir, "pnpm-workspace.yaml")
	if _, err := os.Stat(pnpmWorkspacePath); err == nil {
		return o.detectPnpmPackages(workDir)
	}

	// Check for npm/yarn workspaces in package.json
	packageJSONPath := filepath.Join(workDir, "package.json")
	if _, err := os.Stat(packageJSONPath); err == nil {
		return o.detectNpmWorkspacePackages(workDir)
	}

	return packages, nil
}

// detectPnpmPackages detects packages in a pnpm workspace
func (o *Orchestrator) detectPnpmPackages(workDir string) ([]MonorepoPackage, error) {
	var packages []MonorepoPackage

	// Common pnpm workspace patterns
	patterns := []string{
		"packages/*",
		"apps/*",
		"libs/*",
		"services/*",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(workDir, pattern))
		if err != nil {
			continue
		}

		for _, match := range matches {
			if info, err := os.Stat(match); err == nil && info.IsDir() {
				// Check if it has a package.json
				if _, err := os.Stat(filepath.Join(match, "package.json")); err == nil {
					packages = append(packages, MonorepoPackage{
						Name: filepath.Base(match),
						Path: match,
					})
				}
			}
		}
	}

	return packages, nil
}

// detectNpmWorkspacePackages detects packages in an npm/yarn workspace
func (o *Orchestrator) detectNpmWorkspacePackages(workDir string) ([]MonorepoPackage, error) {
	var packages []MonorepoPackage

	// Common workspace locations
	workspaceDirs := []string{"packages", "apps", "libs", "services"}

	for _, dir := range workspaceDirs {
		wsPath := filepath.Join(workDir, dir)
		if info, err := os.Stat(wsPath); err == nil && info.IsDir() {
			entries, err := os.ReadDir(wsPath)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				if entry.IsDir() {
					pkgPath := filepath.Join(wsPath, entry.Name())
					if _, err := os.Stat(filepath.Join(pkgPath, "package.json")); err == nil {
						packages = append(packages, MonorepoPackage{
							Name: entry.Name(),
							Path: pkgPath,
						})
					}
				}
			}
		}
	}

	return packages, nil
}

// BatchProcessor handles batch processing of tasks for thermal management
type BatchProcessor struct {
	BatchSize   int
	CoolDownMs  int
	TotalItems  int
	HwInfo      thermal.HardwareInfo
}

// NewBatchProcessor creates a new batch processor with optimal settings
func (o *Orchestrator) NewBatchProcessor(totalItems int) *BatchProcessor {
	batchSize := thermal.GetOptimalBatchSize(o.hwInfo, totalItems, o.batchSize)
	
	coolDownMs := o.bp.Thermal.CoolDownMs
	if coolDownMs == 0 {
		coolDownMs = thermal.DefaultCoolDownMs
	}

	return &BatchProcessor{
		BatchSize:   batchSize,
		CoolDownMs:  coolDownMs,
		TotalItems:  totalItems,
		HwInfo:      o.hwInfo,
	}
}

// ShouldBatch returns true if batching should be used
func (bp *BatchProcessor) ShouldBatch() bool {
	return bp.TotalItems > thermal.DefaultBatchThreshold
}

// GetBatches returns the items split into batches
func (bp *BatchProcessor) GetBatches(items []string) [][]string {
	if !bp.ShouldBatch() {
		return [][]string{items}
	}

	var batches [][]string
	for i := 0; i < len(items); i += bp.BatchSize {
		end := i + bp.BatchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	return batches
}

// CoolDown pauses between batches for thermal management
func (bp *BatchProcessor) CoolDown() {
	if bp.CoolDownMs > 0 {
		time.Sleep(time.Duration(bp.CoolDownMs) * time.Millisecond)
	}
}

// ExecuteInBatches executes a function for each item in batches with cool-down periods
func (o *Orchestrator) ExecuteInBatches(items []string, fn func(item string) error) error {
	processor := o.NewBatchProcessor(len(items))

	if !processor.ShouldBatch() {
		// No batching needed, execute all at once
		for _, item := range items {
			if err := fn(item); err != nil {
				return err
			}
		}
		return nil
	}

	batches := processor.GetBatches(items)
	fmt.Printf("📦 Processing %d items in %d batches (batch size: %d, cool-down: %dms)\n",
		len(items), len(batches), processor.BatchSize, processor.CoolDownMs)

	for i, batch := range batches {
		fmt.Printf("\n🔄 Batch %d/%d (%d items)\n", i+1, len(batches), len(batch))

		for _, item := range batch {
			if err := fn(item); err != nil {
				return err
			}
		}

		// Cool down between batches (but not after the last batch)
		if i < len(batches)-1 {
			fmt.Printf("🌡️  Cooling down for %dms...\n", processor.CoolDownMs)
			processor.CoolDown()
		}
	}

	return nil
}

// GetThermalConfig returns the effective thermal configuration
func (o *Orchestrator) GetThermalConfig() thermal.Config {
	return thermal.Config{
		Concurrency: o.concurrency,
		BatchSize:   o.batchSize,
		CoolDownMs:  o.bp.Thermal.CoolDownMs,
		ThermalMode: o.bp.Thermal.Mode,
	}
}

// GetHardwareInfo returns the detected hardware information
func (o *Orchestrator) GetHardwareInfo() thermal.HardwareInfo {
	return o.hwInfo
}

// ==========================================
// Dashboard Integration
// ==========================================

// HasDashboard returns true if dashboard mode is enabled
func (o *Orchestrator) HasDashboard() bool {
	return o.dashboard != nil
}

// GetDashboard returns the dashboard runner (may be nil)
func (o *Orchestrator) GetDashboard() *ui.DashboardRunner {
	return o.dashboard
}

// RunWithDashboard runs the orchestrator with the TUI dashboard active
// This is the preferred method when dashboard mode is enabled
func (o *Orchestrator) RunWithDashboard() error {
	if o.dashboard == nil {
		// Fall back to standard Run if no dashboard
		return o.Run()
	}

	// Update project in dashboard
	o.dashboard.UpdateProject(0, ui.PhaseIdle, ui.StatusPending)

	// Start dashboard in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- o.dashboard.Start()
	}()

	// Run the orchestrator
	runErr := o.runWithDashboardUpdates()

	// Stop the dashboard
	o.dashboard.Stop()

	// Wait for dashboard to finish
	select {
	case dashErr := <-errChan:
		if runErr != nil {
			return runErr
		}
		return dashErr
	}
}

// runWithDashboardUpdates runs the main execution with dashboard updates
func (o *Orchestrator) runWithDashboardUpdates() error {
	// Update status to running
	o.dashboard.UpdateProject(0, ui.PhaseRun, ui.StatusRunning)

	// Determine working directory
	// For monorepos, use the monorepo root if specified
	workDir := o.opts.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	if o.bp.IsMonorepo && o.bp.MonorepoRoot != "" {
		// Use monorepo root as the working directory
		if info, err := os.Stat(o.bp.MonorepoRoot); err == nil && info.IsDir() {
			workDir = o.bp.MonorepoRoot
			o.logToDashboard(0, fmt.Sprintf("📂 Using monorepo root: %s", workDir))
		} else {
			o.logToDashboard(0, fmt.Sprintf("⚠️  Warning: monorepo_root %s does not exist, using current directory", o.bp.MonorepoRoot))
		}
	}

	// Log to dashboard
	o.logToDashboard(0, fmt.Sprintf("🚀 Starting %s (env=%s)", o.bp.Name, o.opts.Environment))

	// Check runtime
	o.checkRuntime()

	// Monorepo linking
	if o.bp.IsMonorepo && o.bp.PackageManager == "pnpm" {
		o.logToDashboard(0, "📦 Checking pnpm workspace links...")
		if err := o.ensurePnpmWorkspaceLinked(workDir); err != nil {
			o.logToDashboard(0, fmt.Sprintf("⚠️  Warning: pnpm workspace linking failed: %v", err))
		}
	}

	// Check dependencies
	if err := o.checkAndInstallDependencies(workDir); err != nil {
		o.logToDashboard(0, fmt.Sprintf("⚠️  Warning: dependency check failed: %v", err))
	}

	// Check env vars (skip interactive prompts in dashboard mode)
	o.loadEnvVarsForInjection(workDir)

	// Setup phase
	if o.bp.SetupRequired && o.bp.SetupCommand != "" && !o.opts.SkipSetup {
		o.dashboard.UpdateProject(0, ui.PhaseSetup, ui.StatusRunning)
		o.logToDashboard(0, fmt.Sprintf("🔧 Running setup: %s", o.bp.SetupCommand))

		if err := o.executeSetupPhaseWithDashboard(workDir, o.bp.SetupCommand); err != nil {
			o.dashboard.UpdateProject(0, ui.PhaseSetup, ui.StatusError)
			o.logToDashboard(0, fmt.Sprintf("❌ Setup failed: %v", err))
			return err
		}

		o.logToDashboard(0, "✅ Setup completed successfully")
	}

	// Run phase
	if o.bp.RunCommand == "" {
		o.dashboard.UpdateProject(0, ui.PhaseRun, ui.StatusError)
		return fmt.Errorf("no run command specified in configuration")
	}

	o.dashboard.UpdateProject(0, ui.PhaseRun, ui.StatusRunning)
	runCommand := o.bp.RunCommand

	// Auto-build if needed
	if err := o.autoBuildIfNeeded(workDir, runCommand); err != nil {
		o.dashboard.UpdateProject(0, ui.PhaseRun, ui.StatusError)
		return fmt.Errorf("auto-build failed: %w", err)
	}

	// Port handling
	isHTMLProject := strings.ToLower(o.bp.Language) == "html"
	if !isHTMLProject {
		runCommand = o.handlePortConfiguration(runCommand)
	}

	// Execute
	o.logToDashboard(0, fmt.Sprintf("📦 Executing: %s", runCommand))
	if err := o.executeWithDashboard(workDir, runCommand, isHTMLProject); err != nil {
		o.dashboard.UpdateProject(0, ui.PhaseRun, ui.StatusError)
		o.logToDashboard(0, fmt.Sprintf("❌ Command failed: %v", err))
		return err
	}

	o.dashboard.UpdateProject(0, ui.PhaseRun, ui.StatusSuccess)
	o.logToDashboard(0, "✅ Completed successfully")
	return nil
}

// logToDashboard sends a log line to the dashboard
func (o *Orchestrator) logToDashboard(projectIndex int, line string) {
	if o.dashboard != nil {
		writer := o.dashboard.GetWriter(projectIndex)
		writer.Write([]byte(line + "\n"))
	}
}

// handlePortConfiguration handles port override and conflict detection
func (o *Orchestrator) handlePortConfiguration(runCommand string) string {
	portInfo := ports.ExtractPort(runCommand)
	finalPort := portInfo.Port
	
	if portInfo.Found {
		if processOnPort := o.checkProcessOnPort(portInfo.Port); processOnPort {
			if !o.opts.NoPortShift {
				newPort := ports.FindAvailablePort(portInfo.Port + 1)
				if newPort > 0 {
					o.logToDashboard(0, fmt.Sprintf("⚠️  Port %d busy, shifting to %d", portInfo.Port, newPort))
					runCommand = ports.ShiftPort(runCommand, portInfo.Port, newPort)
					finalPort = newPort
				}
			}
		}
	}

	if o.opts.PortOverride > 0 {
		if portInfo.Found {
			runCommand = ports.ShiftPort(runCommand, portInfo.Port, o.opts.PortOverride)
		} else {
			runCommand = ports.AppendPortFlag(runCommand, o.bp.Language, o.opts.PortOverride)
		}
		finalPort = o.opts.PortOverride
		o.logToDashboard(0, fmt.Sprintf("📌 Using port %d", o.opts.PortOverride))
	} else if !o.opts.NoPortShift {
		newCommand, newPort, wasShifted, err := ports.CheckAndShift(runCommand)
		if err == nil && wasShifted {
			o.logToDashboard(0, fmt.Sprintf("⚠️  Port conflict detected, shifted to %d", newPort))
			runCommand = newCommand
			finalPort = newPort
		}
	}

	// Update dashboard with port information for URL display
	if o.dashboard != nil && finalPort > 0 {
		if p := o.dashboard.GetProject(0); p != nil {
			p.SetPort(finalPort)
		}
	}

	return runCommand
}

// executeSetupPhaseWithDashboard runs setup with output to dashboard
func (o *Orchestrator) executeSetupPhaseWithDashboard(workDir string, setupCommand string) error {
	resolvedWorkDir, resolvedCommand := o.resolveNestedCommand(workDir, setupCommand)
	resolvedCommand = o.injectConcurrencyFlags(resolvedCommand)

	baseEnv := provisioner.BuildEnhancedEnvironment()
	env := o.buildEnvWithSecrets(baseEnv)

	ctx, cancel := context.WithTimeout(o.dashboard.GetContext(), 30*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", resolvedCommand)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", resolvedCommand)
	}

	cmd.Dir = resolvedWorkDir
	cmd.Env = env

	// Capture output to dashboard
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	// Stream output to dashboard
	go o.streamToDashboard(0, stdout, "")
	go o.streamToDashboard(0, stderr, "ERR: ")

	return cmd.Wait()
}

// executeWithDashboard executes a command with output to dashboard
func (o *Orchestrator) executeWithDashboard(workDir string, runCommand string, isHTMLProject bool) error {
	resolvedWorkDir, resolvedCommand := o.resolveNestedCommand(workDir, runCommand)
	resolvedCommand = o.injectConcurrencyFlags(resolvedCommand)

	pmInfo := provisioner.DetectPackageManager(resolvedWorkDir)

	var baseEnv []string
	if o.usesTurbo(resolvedCommand) {
		baseEnv = provisioner.BuildEnhancedEnvironmentWithTurbo(pmInfo.Manager, pmInfo.Version)
	} else {
		baseEnv = provisioner.BuildEnhancedEnvironment()
	}

	env := o.buildEnvWithSecrets(baseEnv)

	ctx := o.dashboard.GetContext()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", resolvedCommand)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", resolvedCommand)
	}

	cmd.Dir = resolvedWorkDir
	cmd.Env = env
	
	// Set process group so we can kill all child processes together
	// This is critical for killing dev servers spawned by shell commands
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	if isHTMLProject {
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to open browser: %w", err)
		}
		o.logToDashboard(0, "🌐 Opened in browser")
		return nil
	}

	// Capture output to dashboard
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	// Store the command reference in the project for graceful shutdown
	if project := o.dashboard.GetProject(0); project != nil {
		project.SetCmd(cmd)
	}

	// Stream output to dashboard
	go o.streamToDashboard(0, stdout, "")
	go o.streamToDashboard(0, stderr, "ERR: ")

	return cmd.Wait()
}

// streamToDashboard streams reader output to the dashboard
func (o *Orchestrator) streamToDashboard(projectIndex int, reader interface{ Read([]byte) (int, error) }, prefix string) {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if prefix != "" {
			line = prefix + line
		}
		o.logToDashboard(projectIndex, line)
	}
}