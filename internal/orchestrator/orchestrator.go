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
	"time"

	"github.com/harshul/octo-cli/internal/blueprint"
	"github.com/harshul/octo-cli/internal/ports"
	"github.com/harshul/octo-cli/internal/provisioner"
	"github.com/harshul/octo-cli/internal/secrets"
	"github.com/harshul/octo-cli/internal/ui"
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
}

type Orchestrator struct {
	bp   blueprint.Blueprint
	opts Options
}

func New(bp blueprint.Blueprint, opts Options) (*Orchestrator, error) {
	return &Orchestrator{bp: bp, opts: opts}, nil
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

func (o *Orchestrator) Run() error {
	fmt.Printf("🚀 Starting %s (env=%s, build=%v, watch=%v, detach=%v)\n",
		o.bp.Name, o.opts.Environment, o.opts.RunBuild, o.opts.Watch, o.opts.Detach)

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
	workDir := o.opts.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Check and install dependencies if needed (e.g., node_modules for Node projects)
	if err := o.checkAndInstallDependencies(workDir); err != nil {
		fmt.Printf("⚠️  Warning: dependency check failed: %v\n", err)
	}

	// Check environment variables
	if err := o.checkEnvVars(); err != nil {
		return err
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
func (o *Orchestrator) checkEnvVars() error {
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
		filepath.Join(o.opts.WorkDir, ".env"),
		filepath.Join(o.opts.WorkDir, ".env.local"),
		filepath.Join(o.opts.WorkDir, "apps/client/.env"),
		filepath.Join(o.opts.WorkDir, "apps/client/.env.local"),
		filepath.Join(o.opts.WorkDir, "apps/server/.env"),
		filepath.Join(o.opts.WorkDir, "apps/server/.env.local"),
		filepath.Join(o.opts.WorkDir, "apps/web/.env"),
		filepath.Join(o.opts.WorkDir, "apps/api/.env"),
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
		for k, v := range values {
			if v != "" {
				os.Setenv(k, v)
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
// It also injects the enhanced environment with newly installed binary paths.
func (o *Orchestrator) executeWithPathCorrection(workDir string, runCommand string, isHTMLProject bool) error {
	// Check if the command contains directory changes
	resolvedWorkDir, resolvedCommand := o.resolveNestedCommand(workDir, runCommand)

	// Detect the package manager for this project
	pmInfo := provisioner.DetectPackageManager(resolvedWorkDir)

	// Build the enhanced environment with additional paths
	var env []string
	if o.usesTurbo(resolvedCommand) {
		// For Turbo, add npm_config_user_agent to help it detect the package manager
		env = provisioner.BuildEnhancedEnvironmentWithTurbo(pmInfo.Manager, pmInfo.Version)
		fmt.Printf("🔧 Turbo detected - setting npm_config_user_agent for %s\n", pmInfo.Manager)
	} else {
		env = provisioner.BuildEnhancedEnvironment()
	}

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

	// Set the enhanced environment
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