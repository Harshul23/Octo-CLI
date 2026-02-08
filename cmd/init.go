package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/harshul/octo-cli/internal/analyzer"
	"github.com/harshul/octo-cli/internal/blueprint"
	"github.com/harshul/octo-cli/internal/doctor"
	"github.com/harshul/octo-cli/internal/provisioner"
	"github.com/harshul/octo-cli/internal/secrets"
	"github.com/harshul/octo-cli/internal/ui"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Analyze the codebase and generate a .octo.yaml file",
	Long: `The init command analyzes your current directory to detect:
- Programming languages and frameworks
- Package managers and dependencies
- Build systems and scripts
- Runtime requirements

It performs a health check to ensure all required runtimes are installed,
prompts for dependency installation if needed, and generates a .octo.yaml 
configuration file that can be used with 'octo run' to deploy your application locally.`,
	RunE: runInit,
}

func init() {
	// Add flags specific to the init command
	initCmd.Flags().StringP("output", "o", ".octo.yaml", "Output file path for the configuration")
	initCmd.Flags().BoolP("force", "f", false, "Overwrite existing .octo.yaml file")
	initCmd.Flags().BoolP("interactive", "i", false, "Run in interactive mode with prompts")
	initCmd.Flags().Bool("skip-install", false, "Skip dependency installation prompts")
	initCmd.Flags().Bool("auto-install", false, "Automatically install dependencies without prompting")
	initCmd.Flags().Bool("skip-secrets", false, "Skip secrets/environment variable setup")
	initCmd.Flags().StringP("env", "e", "development", "Target environment (development, production) - affects script selection")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Get flag values
	outputPath, _ := cmd.Flags().GetString("output")
	force, _ := cmd.Flags().GetBool("force")
	interactive, _ := cmd.Flags().GetBool("interactive")
	skipInstall, _ := cmd.Flags().GetBool("skip-install")
	autoInstall, _ := cmd.Flags().GetBool("auto-install")
	skipSecrets, _ := cmd.Flags().GetBool("skip-secrets")
	env, _ := cmd.Flags().GetString("env")

	// Resolve output path
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(cwd, outputPath)
	}

	// Check if file already exists
	if _, err := os.Stat(outputPath); err == nil && !force {
		return fmt.Errorf("configuration file already exists at %s. Use --force to overwrite", outputPath)
	}

	// ========================================
	// Show intro animation
	// ========================================
	ui.RunIntro()

	// Print header
	fmt.Println()
	ui.PrintHeader("🐙 Octo Init")
	fmt.Println()

	// ========================================
	// STEP 1: Analyze the codebase
	// ========================================
	ui.PrintStep(1, 5, "Analyzing codebase...")

	// Build analysis options based on environment flag
	opts := analyzer.AnalysisOptions{
		Environment: env,
	}

	// Analyze the project using options-based analysis
	projectInfo, err := analyzer.AnalyzeProjectWithOptions(cwd, opts)
	if err != nil {
		ui.PrintError("Analysis failed")
		return fmt.Errorf("analysis failed: %w", err)
	}

	ui.PrintSuccess("Analysis complete")
	fmt.Println()

	// Display detected project information with nice formatting
	ui.PrintDivider()
	ui.PrintHighlight("Language", projectInfo.Language)
	if projectInfo.PackageManager != "" {
		ui.PrintHighlight("Package Manager", projectInfo.PackageManager)
	}
	if projectInfo.Version != "" {
		ui.PrintHighlight("Version", projectInfo.Version)
	}
	if projectInfo.RunCommand != "" {
		ui.PrintHighlight("Run Command", projectInfo.RunCommand)
	}
	ui.PrintDivider()
	fmt.Println()

	// ========================================
	// STEP 2: Diagnose (The Doctor)
	// ========================================
	ui.PrintStep(2, 5, "Running health check...")

	diagnosis := doctor.Diagnose(cwd, projectInfo.Language)

	ui.PrintSuccess("Health check complete")
	fmt.Println()

	// Display diagnosis results (with improved UI)
	displayDiagnosisVite(diagnosis)

	// ========================================
	// STEP 3: Handle Runtime Issues
	// ========================================
	if !diagnosis.Runtime.Installed {
		ui.PrintWarning(fmt.Sprintf("%s runtime is not installed", diagnosis.Runtime.Name))
		showRuntimeInstallHelp(diagnosis.Runtime.Name)
	}

	// ========================================
	// STEP 4: Prompt for Provisioning
	// ========================================
	if !skipInstall && diagnosis.Dependencies.ConfigFile != "" && !diagnosis.Dependencies.Installed {
		// Check if package manager needs to be installed first
		if !diagnosis.Dependencies.ManagerInstalled {
			pmInfo := provisioner.DetectPackageManager(cwd)

			// Handle Bun specially with interactive install/fallback
			if pmInfo.Manager == provisioner.Bun {
				bunResult := provisioner.EnsureBunWithFallback(cwd, nil)
				
				if !bunResult.Available {
					ui.PrintError(bunResult.UserMessage)
					ui.PrintWarning("Skipping dependency installation. Please install Bun manually and run 'octo init' again.")
				} else {
					// Bun is now available (either installed or using fallback)
					if bunResult.UserMessage != "" {
						ui.PrintSuccess(bunResult.UserMessage)
					}

					// Run install with the resolved package manager
					installCmd := bunResult.InstallCmd
					if len(installCmd) > 0 {
						ui.PrintStep(3, 5, fmt.Sprintf("Installing dependencies (%s install)...", installCmd[0]))
						
						err := doctor.InstallDependencies(cwd, fmt.Sprintf("%s install", installCmd[0]))
						
						if err != nil {
							ui.PrintError(fmt.Sprintf("Installation failed: %v", err))
						} else {
							ui.PrintSuccess("Dependencies installed")

							// Verify installation
							ui.PrintStep(4, 5, "Verifying installation...")
							newDiagnosis := doctor.VerifyInstallation(cwd, projectInfo.Language)
							if newDiagnosis.Dependencies.Installed {
								ui.PrintSuccess("All dependencies verified")
							} else {
								ui.PrintWarning("Some dependencies may need attention")
							}
						}
					}
				}
			} else {
				// For other package managers (pnpm, yarn), try Corepack
				pmResult := provisioner.EnsurePackageManager(cwd)
				
				if !pmResult.Available {
					ui.PrintError(pmResult.UserMessage)
					if diagnosis.Dependencies.FixCommand != "" {
						ui.PrintInfo(fmt.Sprintf("To fix: %s", diagnosis.Dependencies.FixCommand))
					}
					ui.PrintWarning("Skipping dependency installation.")
				} else {
					if pmResult.EnabledViaCorepack {
						ui.PrintSuccess(pmResult.UserMessage)
					}

					// Proceed with installation
					ui.PrintStep(3, 5, fmt.Sprintf("Installing dependencies (%s)...", diagnosis.Dependencies.InstallCommand))
					err := doctor.InstallDependencies(cwd, diagnosis.Dependencies.InstallCommand)

					if err != nil {
						ui.PrintError(fmt.Sprintf("Installation failed: %v", err))
					} else {
						ui.PrintSuccess("Dependencies installed")
						ui.PrintStep(4, 5, "Verifying installation...")
						newDiagnosis := doctor.VerifyInstallation(cwd, projectInfo.Language)
						if newDiagnosis.Dependencies.Installed {
							ui.PrintSuccess("All dependencies verified")
						} else {
							ui.PrintWarning("Some dependencies may need attention")
						}
					}
				}
			}
		} else {
			// Package manager is installed, proceed normally
			shouldInstall := autoInstall

			if !autoInstall {
				// Prompt the user with Vite-style navigation
				fmt.Println()
				shouldInstall = promptForInstallVite(
					projectInfo.Language,
					diagnosis.Dependencies.ConfigFile,
					diagnosis.Dependencies.MissingPackages,
				)
			}

			if shouldInstall {
				// ========================================
				// STEP 5: Execute Installation
				// ========================================
				ui.PrintStep(3, 5, fmt.Sprintf("Installing dependencies (%s)...", diagnosis.Dependencies.InstallCommand))

				err := doctor.InstallDependencies(cwd, diagnosis.Dependencies.InstallCommand)

				if err != nil {
					ui.PrintError(fmt.Sprintf("Installation failed: %v", err))
				} else {
					ui.PrintSuccess("Dependencies installed")

					// ========================================
					// STEP 6: Verify Installation
					// ========================================
					fmt.Println()
					ui.PrintStep(4, 5, "Verifying installation...")

					newDiagnosis := doctor.VerifyInstallation(cwd, projectInfo.Language)

					if newDiagnosis.Dependencies.Installed {
						ui.PrintSuccess("All dependencies verified")
					} else {
						ui.PrintWarning("Some dependencies may need attention")
					}
				}
			} else {
				ui.PrintInfo("Skipping dependency installation")
			}
		}
	}

	// If interactive mode, prompt user for confirmation/modifications
	if interactive {
		// Convert to Analysis for backward compatibility with UI
		analysis := analyzer.Analysis{
			Root: cwd,
			Name: projectInfo.Name,
		}
		analysis, err = ui.PromptForConfirmation(analysis)
		if err != nil {
			return fmt.Errorf("interactive prompt failed: %w", err)
		}
		projectInfo.Name = analysis.Name
	}

	// ========================================
	// STEP 7: Smart Secrets Onboarding (README-Driven)
	// ========================================
	var allDetectedVars []secrets.EnvVar

	if !skipSecrets {
		fmt.Println()
		ui.PrintStep(4, 5, "Scanning for environment variables...")

		// Use README-enhanced env status check
		envStatus, err := secrets.CheckEnvStatusWithReadme(cwd, projectInfo.Language)

		if err != nil {
			ui.PrintWarning(fmt.Sprintf("Could not scan for environment variables: %v", err))
		} else {
			allDetectedVars = envStatus.Required // Save for blueprint
			
			// Show README defaults found
			if len(envStatus.ReadmeDefaults) > 0 {
				ui.PrintInfo(fmt.Sprintf("Found %d default value(s) from README", len(envStatus.ReadmeDefaults)))
			}

			// Show target directories if monorepo
			if len(envStatus.EnvTargets) > 1 {
				fmt.Println()
				ui.PrintInfo("Environment file targets:")
				for _, t := range envStatus.EnvTargets {
					fmt.Printf("    • %s\n", t.Path)
				}
			}
			
			if len(envStatus.Missing) > 0 {
				// Build vars with defaults for enhanced prompt
				varsWithDefaults := make([]ui.EnvVarWithDefault, 0, len(envStatus.Missing))
				for _, v := range envStatus.Missing {
					vwd := ui.EnvVarWithDefault{
						Name:        v.Name,
						Description: secrets.GetEnvVarDescription(v.Name),
						Default:     v.DefaultValue,
						TargetDir:   v.TargetDir,
					}
					
					// Try to get suggestion if no default from README
					if vwd.Default == "" {
						vwd.Default = secrets.GetEnvVarSuggestion(v.Name, envStatus.ReadmeDefaults)
					}
					
					varsWithDefaults = append(varsWithDefaults, vwd)
				}

				// Ask if user wants to set them up with Vite-style prompt
				fmt.Println()
				shouldSetup := promptForSecretsVite(len(envStatus.Missing))
				
				if shouldSetup {
					// Use enhanced prompt with defaults
					values := ui.PromptForSecretsWithDefaults(varsWithDefaults)

					if len(values) > 0 {
						// Write to appropriate .env files based on targets
						if len(envStatus.EnvTargets) > 0 {
							// Multi-target write
							if err := secrets.WriteEnvFilesToTargets(envStatus.EnvTargets, values); err != nil {
								ui.PrintError(fmt.Sprintf("Failed to write .env files: %v", err))
							} else {
								// Build results summary
								results := make(map[string]int)
								for _, target := range envStatus.EnvTargets {
									count := 0
									for _, v := range target.Variables {
										if _, ok := values[v.Name]; ok {
											count++
										}
									}
									if count > 0 {
										results[target.Path] = count
									}
								}
								showSecretsResult(results)
								ensureGitignore(cwd)
							}
						} else {
							// Single .env file (fallback)
							envPath := filepath.Join(cwd, ".env")
							if err := secrets.AppendToEnvFile(envPath, values); err != nil {
								ui.PrintError(fmt.Sprintf("Failed to write .env file: %v", err))
							} else {
								ui.PrintSuccess(fmt.Sprintf("Saved %d secret(s) to %s", len(values), envPath))
								ensureGitignore(cwd)
							}
						}
					} else {
						ui.PrintInfo("No secrets saved - you can add them later to .env")
					}
				} else {
					ui.PrintInfo("Skipping secrets setup")
				}
			} else if len(envStatus.Required) > 0 {
				ui.PrintSuccess(fmt.Sprintf("Found %d environment variable(s) - all configured!", len(envStatus.Required)))
			}
		}
	}

	// Generate the blueprint from project info
	bp := blueprint.FromProjectInfo(projectInfo)

	// Add detected environment variables to blueprint
	if len(allDetectedVars) > 0 {
		bp.EnvVars = make([]blueprint.EnvVar, len(allDetectedVars))
		for i, v := range allDetectedVars {
			bp.EnvVars[i] = blueprint.EnvVar{
				Name:     v.Name,
				Required: v.Required,
			}
		}
	}

	// ========================================
	// STEP 5: Write Configuration
	// ========================================
	fmt.Println()
	ui.PrintStep(5, 5, "Writing configuration...")

	// Write the configuration file
	if err := blueprint.Write(outputPath, bp); err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	// Final success message
	fmt.Println()
	ui.PrintDivider()
	ui.PrintSuccess(fmt.Sprintf("Configuration written to %s", outputPath))
	fmt.Println()
	ui.PrintInfo("Next steps:")
	fmt.Println("    Run " + "\033[1mocto run\033[0m" + " to start your application")
	fmt.Println()

	return nil
}

// ============================================================================
// Vite-style Helper Functions
// ============================================================================

// displayDiagnosisVite shows diagnosis results in Vite-style
func displayDiagnosisVite(diagnosis doctor.Diagnosis) {
	// Runtime status
	if diagnosis.Runtime.Installed {
		ui.PrintSuccess(fmt.Sprintf("Runtime: %s %s", diagnosis.Runtime.Name, diagnosis.Runtime.Version))
	} else {
		ui.PrintError(fmt.Sprintf("Runtime: %s is not installed", diagnosis.Runtime.Name))
	}

	// Package manager status
	if !diagnosis.Dependencies.ManagerInstalled && diagnosis.Dependencies.Manager != "" {
		ui.PrintError(fmt.Sprintf("Package Manager: %s is not installed", diagnosis.Dependencies.Manager))
	}

	// Dependencies status
	if diagnosis.Dependencies.ConfigFile != "" {
		if diagnosis.Dependencies.Installed {
			ui.PrintSuccess(fmt.Sprintf("Dependencies: Installed (%s)", diagnosis.Dependencies.Manager))
		} else {
			ui.PrintWarning(fmt.Sprintf("Dependencies: Not installed (%s)", diagnosis.Dependencies.Manager))
		}
	}

	// Overall status
	fmt.Println()
	if diagnosis.Healthy {
		ui.PrintSuccess("Project is healthy and ready to run!")
	} else {
		ui.PrintWarning("Project has issues that need attention")
		for _, issue := range diagnosis.Issues {
			fmt.Printf("    • %s\n", issue)
		}
	}
}

// showRuntimeInstallHelp shows installation help for a runtime
func showRuntimeInstallHelp(runtimeName string) {
	fmt.Println()
	fmt.Println("  Please install it before continuing:")
	switch runtimeName {
	case "Node.js":
		fmt.Println("    • macOS: brew install node")
		fmt.Println("    • Or visit: https://nodejs.org/")
	case "Python":
		fmt.Println("    • macOS: brew install python3")
		fmt.Println("    • Or visit: https://www.python.org/")
	case "Go":
		fmt.Println("    • macOS: brew install go")
		fmt.Println("    • Or visit: https://go.dev/")
	}
	fmt.Println()
}

// promptForInstallVite prompts for dependency installation using Vite-style UI
func promptForInstallVite(language, configFile string, missingPackages []string) bool {
	var description string
	if len(missingPackages) > 0 && len(missingPackages) <= 3 {
		description = fmt.Sprintf("Missing: %s", joinStrings(missingPackages, ", "))
	} else if len(missingPackages) > 3 {
		description = fmt.Sprintf("%d packages are missing", len(missingPackages))
	} else {
		description = fmt.Sprintf("Found %s", configFile)
	}

	result, err := ui.RunYesNoPrompt(
		fmt.Sprintf("Install %s dependencies?", language),
		description,
		true,
	)
	if err != nil {
		return false
	}
	return result
}

// promptForSecretsVite prompts for secrets setup using Vite-style UI
func promptForSecretsVite(missingCount int) bool {
	result, err := ui.RunYesNoPrompt(
		"Configure environment variables?",
		fmt.Sprintf("%d variable(s) need configuration", missingCount),
		true,
	)
	if err != nil {
		return false
	}
	return result
}

// showSecretsResult shows the secrets setup result
func showSecretsResult(results map[string]int) {
	totalSaved := 0
	for path, count := range results {
		if count > 0 {
			ui.PrintSuccess(fmt.Sprintf("Saved %d secret(s) to %s", count, path))
			totalSaved += count
		}
	}
	if totalSaved == 0 {
		ui.PrintInfo("No secrets were saved")
	}
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// ensureGitignore checks if .env is in .gitignore and adds it if not
func ensureGitignore(projectPath string) {
	gitignorePath := filepath.Join(projectPath, ".gitignore")
	
	// Read existing .gitignore
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		// Create new .gitignore with .env
		if os.IsNotExist(err) {
			err = os.WriteFile(gitignorePath, []byte("# Environment variables\n.env\n.env.local\n.env.*.local\n"), 0644)
			if err == nil {
				ui.Info("Created .gitignore with .env entries")
			}
		}
		return
	}

	// Check if .env is already in .gitignore
	contentStr := string(content)
	if !containsLine(contentStr, ".env") {
		// Append .env to .gitignore
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()

		_, err = f.WriteString("\n# Environment variables\n.env\n.env.local\n.env.*.local\n")
		if err == nil {
			ui.Info("Added .env to .gitignore")
		}
	}
}

// containsLine checks if a file content contains a specific line
func containsLine(content, line string) bool {
	lines := splitLines(content)
	for _, l := range lines {
		l = trimSpace(l)
		if l == line {
			return true
		}
	}
	return false
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// trimSpace trims whitespace from a string
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
