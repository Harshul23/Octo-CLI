package main

import (
	"bufio"
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
	// STEP 1: Analyze the codebase
	// ========================================
	spinner := ui.NewSpinner("Analyzing codebase...")
	spinner.Start()

	// Build analysis options based on environment flag
	opts := analyzer.AnalysisOptions{
		Environment: env,
	}

	// Analyze the project using options-based analysis
	projectInfo, err := analyzer.AnalyzeProjectWithOptions(cwd, opts)
	if err != nil {
		spinner.Fail("Analysis failed")
		return fmt.Errorf("analysis failed: %w", err)
	}

	spinner.Success("Analysis complete")

	// Display detected project information
	ui.Info(fmt.Sprintf("Detected language: %s", projectInfo.Language))
	if projectInfo.PackageManager != "" {
		ui.Info(fmt.Sprintf("Package manager: %s", projectInfo.PackageManager))
	}
	if projectInfo.Version != "" {
		ui.Info(fmt.Sprintf("Detected version: %s", projectInfo.Version))
	}
	if projectInfo.RunCommand != "" {
		ui.Info(fmt.Sprintf("Run command: %s", projectInfo.RunCommand))
	}

	// ========================================
	// STEP 2: Diagnose (The Doctor)
	// ========================================
	diagSpinner := ui.NewSpinner("Running health check...")
	diagSpinner.Start()

	diagnosis := doctor.Diagnose(cwd, projectInfo.Language)

	diagSpinner.Success("Health check complete")

	// Display diagnosis results
	ui.DisplayDiagnosis(diagnosis)

	// ========================================
	// STEP 3: Handle Runtime Issues
	// ========================================
	if !diagnosis.Runtime.Installed {
		ui.PromptForRuntimeInstall(diagnosis.Runtime.Name)
		ui.Warn("Continuing without runtime verification. Some features may not work.")
	}

	// ========================================
	// STEP 4: Prompt for Provisioning
	// ========================================
	if !skipInstall && diagnosis.Dependencies.ConfigFile != "" && !diagnosis.Dependencies.Installed {
		// Check if package manager needs to be installed first
		if !diagnosis.Dependencies.ManagerInstalled {
			pmInfo := provisioner.DetectPackageManager(cwd)
			reader := bufio.NewReader(os.Stdin)

			// Handle Bun specially with interactive install/fallback
			if pmInfo.Manager == provisioner.Bun {
				bunResult := provisioner.EnsureBunWithFallback(cwd, reader)
				
				if !bunResult.Available {
					ui.Error(bunResult.UserMessage)
					ui.Warn("Skipping dependency installation. Please install Bun manually and run 'octo init' again.")
				} else {
					// Bun is now available (either installed or using fallback)
					if bunResult.UserMessage != "" {
						ui.Success(bunResult.UserMessage)
					}

					// Run install with the resolved package manager
					installCmd := bunResult.InstallCmd
					if len(installCmd) > 0 {
						installSpinner := ui.DisplayInstallProgress(installCmd[0] + " install")
						
						err := doctor.InstallDependencies(cwd, fmt.Sprintf("%s install", installCmd[0]))
						
						if err != nil {
							installSpinner.Fail("Installation failed")
							ui.Error(fmt.Sprintf("Failed to install dependencies: %v", err))
						} else {
							installSpinner.Success("Dependencies installed")

							// Verify installation
							verifySpinner := ui.NewSpinner("Verifying installation...")
							verifySpinner.Start()
							newDiagnosis := doctor.VerifyInstallation(cwd, projectInfo.Language)
							verifySpinner.Success("Verification complete")
							ui.DisplayVerificationResult(newDiagnosis.Dependencies.Installed)
						}
					}
				}
			} else {
				// For other package managers (pnpm, yarn), try Corepack
				pmResult := provisioner.EnsurePackageManager(cwd)
				
				if !pmResult.Available {
					ui.Error(pmResult.UserMessage)
					if diagnosis.Dependencies.FixCommand != "" {
						ui.Info(fmt.Sprintf("To fix: %s", diagnosis.Dependencies.FixCommand))
					}
					ui.Warn("Skipping dependency installation.")
				} else {
					if pmResult.EnabledViaCorepack {
						ui.Success(pmResult.UserMessage)
					}

					// Proceed with installation
					installSpinner := ui.DisplayInstallProgress(diagnosis.Dependencies.InstallCommand)
					err := doctor.InstallDependencies(cwd, diagnosis.Dependencies.InstallCommand)

					if err != nil {
						installSpinner.Fail("Installation failed")
						ui.Error(fmt.Sprintf("Failed to install dependencies: %v", err))
					} else {
						installSpinner.Success("Dependencies installed")
						verifySpinner := ui.NewSpinner("Verifying installation...")
						verifySpinner.Start()
						newDiagnosis := doctor.VerifyInstallation(cwd, projectInfo.Language)
						verifySpinner.Success("Verification complete")
						ui.DisplayVerificationResult(newDiagnosis.Dependencies.Installed)
					}
				}
			}
		} else {
			// Package manager is installed, proceed normally
			shouldInstall := autoInstall

			if !autoInstall {
				// Prompt the user
				shouldInstall = ui.PromptForInstall(
					projectInfo.Language,
					diagnosis.Dependencies.ConfigFile,
					diagnosis.Dependencies.MissingPackages,
				)
			}

			if shouldInstall {
				// ========================================
				// STEP 5: Execute Installation
				// ========================================
				installSpinner := ui.DisplayInstallProgress(diagnosis.Dependencies.InstallCommand)

				err := doctor.InstallDependencies(cwd, diagnosis.Dependencies.InstallCommand)

				if err != nil {
					installSpinner.Fail("Installation failed")
					ui.Error(fmt.Sprintf("Failed to install dependencies: %v", err))
				} else {
					installSpinner.Success("Dependencies installed")

					// ========================================
					// STEP 6: Verify Installation
					// ========================================
					verifySpinner := ui.NewSpinner("Verifying installation...")
					verifySpinner.Start()

					newDiagnosis := doctor.VerifyInstallation(cwd, projectInfo.Language)

					verifySpinner.Success("Verification complete")

					ui.DisplayVerificationResult(newDiagnosis.Dependencies.Installed)
				}
			} else {
				ui.Info("Skipping dependency installation. You can install manually later.")
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
		secretsSpinner := ui.NewSpinner("Scanning for environment variables and README defaults...")
		secretsSpinner.Start()

		// Use README-enhanced env status check
		envStatus, err := secrets.CheckEnvStatusWithReadme(cwd, projectInfo.Language)
		
		secretsSpinner.Success("Environment scan complete")

		if err != nil {
			ui.Warn(fmt.Sprintf("Could not scan for environment variables: %v", err))
		} else {
			allDetectedVars = envStatus.Required // Save for blueprint
			
			// Show README defaults found
			if len(envStatus.ReadmeDefaults) > 0 {
				ui.Info(fmt.Sprintf("📖 Found %d default value(s) from README", len(envStatus.ReadmeDefaults)))
			}

			// Show target directories if monorepo
			if len(envStatus.EnvTargets) > 1 {
				targetPaths := make([]string, 0, len(envStatus.EnvTargets))
				for _, t := range envStatus.EnvTargets {
					targetPaths = append(targetPaths, t.Path)
				}
				ui.DisplayEnvTargets(targetPaths)
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

				// Ask if user wants to set them up
				if ui.PromptForSecretsOnboarding(len(envStatus.Missing)) {
					// Use enhanced prompt with defaults
					values := ui.PromptForSecretsWithDefaults(varsWithDefaults)

					if len(values) > 0 {
						// Write to appropriate .env files based on targets
						if len(envStatus.EnvTargets) > 0 {
							// Multi-target write
							if err := secrets.WriteEnvFilesToTargets(envStatus.EnvTargets, values); err != nil {
								ui.Error(fmt.Sprintf("Failed to write .env files: %v", err))
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
								ui.DisplaySecretsResultWithTargets(results)
								ensureGitignore(cwd)
							}
						} else {
							// Single .env file (fallback)
							envPath := filepath.Join(cwd, ".env")
							if err := secrets.AppendToEnvFile(envPath, values); err != nil {
								ui.Error(fmt.Sprintf("Failed to write .env file: %v", err))
							} else {
								ui.DisplaySecretsResult(envPath, len(values), len(envStatus.Missing)-len(values))
								ensureGitignore(cwd)
							}
						}
					} else {
						ui.Info("No secrets saved. You can add them manually to .env later.")
					}
				} else {
					ui.Info("Skipping secrets setup. You can add them manually to .env later.")
				}
			} else if len(envStatus.Required) > 0 {
				ui.Success(fmt.Sprintf("Found %d environment variable(s) - all configured!", len(envStatus.Required)))
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

	// Write the configuration file
	if err := blueprint.Write(outputPath, bp); err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	ui.Success(fmt.Sprintf("Configuration written to %s", outputPath))
	ui.Info("Run 'octo run' to start your application")

	return nil
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
