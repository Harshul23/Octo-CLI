package ui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/harshul/octo-cli/internal/analyzer"
	"github.com/harshul/octo-cli/internal/doctor"
)

type Spinner struct {
	msg     string
	running bool
}

func NewSpinner(message string) *Spinner {
	return &Spinner{msg: message}
}

func (s *Spinner) Start() {
	if s == nil || s.running {
		return
	}
	s.running = true
	fmt.Println("⏳", s.msg)
}

func (s *Spinner) Stop() {
	if s == nil || !s.running {
		return
	}
	s.running = false
	// Neutral stop - no status indicator
}

func (s *Spinner) StopWithStatus(status, message string) {
	if s == nil || !s.running {
		return
	}
	s.running = false
	if message != "" {
		fmt.Println(status, message)
	}
}

func (s *Spinner) Success(msg string) {
	s.StopWithStatus("✅", msg)
}

func (s *Spinner) Fail(msg string) {
	s.StopWithStatus("❌", msg)
}

func Success(msg string) {
	fmt.Println("✅", msg)
}

func Info(msg string) {
	fmt.Println("ℹ️", msg)
}

func Warn(msg string) {
	fmt.Println("⚠️", msg)
}

func Error(msg string) {
	fmt.Println("❌", msg)
}

// PromptForConfirmation is a minimal interactive stub.
// For now, it simply echoes the provided analysis without changes.
func PromptForConfirmation(a analyzer.Analysis) (analyzer.Analysis, error) {
	// In a richer UI, we'd prompt the user to confirm or adjust fields.
	// Keeping this non-interactive for now to avoid extra deps.
	// Still, provide a tiny hint to the user.
	base := filepath.Base(a.Root)
	fmt.Println("🔍 Using detected project:", base)
	return a, nil
}

// DisplayDiagnosis shows the health check results to the user
func DisplayDiagnosis(diagnosis doctor.Diagnosis) {
	fmt.Println()
	fmt.Println("🩺 Project Health Check")
	fmt.Println(strings.Repeat("-", 40))

	// Runtime status
	if diagnosis.Runtime.Installed {
		fmt.Printf("✅ Runtime: %s %s\n", diagnosis.Runtime.Name, diagnosis.Runtime.Version)
		if diagnosis.Runtime.Path != "" {
			fmt.Printf("   Path: %s\n", diagnosis.Runtime.Path)
		}
	} else {
		fmt.Printf("❌ Runtime: %s is not installed\n", diagnosis.Runtime.Name)
	}

	// Package manager status
	if !diagnosis.Dependencies.ManagerInstalled && diagnosis.Dependencies.Manager != "" {
		fmt.Printf("❌ Package Manager: %s is not installed\n", diagnosis.Dependencies.Manager)
		if diagnosis.Dependencies.FixCommand != "" {
			fmt.Printf("   💡 To fix: %s\n", diagnosis.Dependencies.FixCommand)
		}
	}

	// Dependencies status
	if diagnosis.Dependencies.ConfigFile != "" {
		if diagnosis.Dependencies.Installed {
			fmt.Printf("✅ Dependencies: Installed (%s)\n", diagnosis.Dependencies.Manager)
		} else {
			fmt.Printf("⚠️  Dependencies: Not installed (%s)\n", diagnosis.Dependencies.Manager)
			if len(diagnosis.Dependencies.MissingPackages) > 0 {
				fmt.Printf("   Missing packages: %s\n", strings.Join(diagnosis.Dependencies.MissingPackages, ", "))
			}
		}
	}

	fmt.Println(strings.Repeat("-", 40))

	// Overall status
	if diagnosis.Healthy {
		fmt.Println("✅ Project is healthy and ready to run!")
	} else {
		fmt.Println("⚠️  Project has issues that need attention")
		for _, issue := range diagnosis.Issues {
			fmt.Printf("   • %s\n", issue)
		}
		// Show actionable fix if available
		if diagnosis.Dependencies.FixCommand != "" && !diagnosis.Dependencies.ManagerInstalled {
			fmt.Println()
			fmt.Println("💡 Quick fix:")
			fmt.Printf("   %s\n", diagnosis.Dependencies.FixCommand)
		}
	}
	fmt.Println()
}

// PromptForInstall asks the user if they want to install dependencies
func PromptForInstall(language string, configFile string, missingPackages []string) bool {
	fmt.Println()

	// Build the prompt message
	var prompt string
	if len(missingPackages) > 0 {
		if len(missingPackages) <= 3 {
			prompt = fmt.Sprintf("I detected a %s project, but you're missing '%s'. Should I install all requirements now? (y/n): ",
				language, strings.Join(missingPackages, "', '"))
		} else {
			prompt = fmt.Sprintf("I detected a %s project, but you're missing %d packages. Should I install all requirements now? (y/n): ",
				language, len(missingPackages))
		}
	} else {
		prompt = fmt.Sprintf("I detected a %s project with %s. Should I install dependencies now? (y/n): ",
			language, configFile)
	}

	fmt.Print("🤖 ", prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// PromptForRuntimeInstall asks the user about missing runtime
func PromptForRuntimeInstall(runtimeName string) {
	fmt.Println()
	fmt.Printf("❌ %s runtime is not installed on your system.\n", runtimeName)
	fmt.Println()
	fmt.Println("Please install it before continuing:")
	
	switch runtimeName {
	case "Node.js":
		fmt.Println("   • macOS: brew install node")
		fmt.Println("   • Ubuntu: sudo apt install nodejs npm")
		fmt.Println("   • Or visit: https://nodejs.org/")
	case "Python":
		fmt.Println("   • macOS: brew install python3")
		fmt.Println("   • Ubuntu: sudo apt install python3 python3-pip")
		fmt.Println("   • Or visit: https://www.python.org/")
	case "Java":
		fmt.Println("   • macOS: brew install openjdk")
		fmt.Println("   • Ubuntu: sudo apt install openjdk-17-jdk")
		fmt.Println("   • Or visit: https://adoptium.net/")
	case "Go":
		fmt.Println("   • macOS: brew install go")
		fmt.Println("   • Ubuntu: sudo apt install golang")
		fmt.Println("   • Or visit: https://go.dev/")
	case "Ruby":
		fmt.Println("   • macOS: brew install ruby")
		fmt.Println("   • Ubuntu: sudo apt install ruby")
		fmt.Println("   • Or visit: https://www.ruby-lang.org/")
	case "Rust":
		fmt.Println("   • All platforms: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh")
		fmt.Println("   • Or visit: https://www.rust-lang.org/")
	}
	fmt.Println()
}

// DisplayInstallProgress shows installation progress
func DisplayInstallProgress(installCommand string) *Spinner {
	spinner := NewSpinner(fmt.Sprintf("Running: %s", installCommand))
	spinner.Start()
	return spinner
}

// DisplayVerificationResult shows the result of post-install verification
func DisplayVerificationResult(healthy bool) {
	fmt.Println()
	if healthy {
		fmt.Println("✅ Verification complete: All dependencies installed successfully!")
	} else {
		fmt.Println("⚠️  Verification: Some issues remain. Please check the output above.")
	}
}

// DisplayMissingSecrets shows the list of missing environment variables
func DisplayMissingSecrets(missing []string, descriptions map[string]string) {
	fmt.Println()
	fmt.Println("🔐 Missing Environment Variables")
	fmt.Println(strings.Repeat("-", 40))
	
	for _, name := range missing {
		desc := descriptions[name]
		if desc != "" {
			fmt.Printf("   • %s (%s)\n", name, desc)
		} else {
			fmt.Printf("   • %s\n", name)
		}
	}
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println()
}

// PromptForSecrets asks the user to enter values for missing secrets
// Returns a map of variable names to their values
func PromptForSecrets(missing []string, descriptions map[string]string) map[string]string {
	values := make(map[string]string)
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("🔐 Secret Onboarding")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("I detected that this app needs some environment variables.")
	fmt.Println("Please enter the values below (or press Enter to skip):")
	fmt.Println()

	for _, name := range missing {
		desc := descriptions[name]
		if desc != "" {
			fmt.Printf("🤖 I see this app needs '%s' (%s).\n", name, desc)
		} else {
			fmt.Printf("🤖 I see this app needs '%s'.\n", name)
		}
		fmt.Printf("   Please paste it here (or Enter to skip): ")

		value, err := reader.ReadString('\n')
		if err != nil {
			continue
		}

		value = strings.TrimSpace(value)
		if value != "" {
			values[name] = value
			fmt.Println("   ✅ Saved!")
		} else {
			fmt.Println("   ⏭️  Skipped")
		}
		fmt.Println()
	}

	return values
}

// PromptForSecretsOnboarding asks if user wants to set up missing secrets
func PromptForSecretsOnboarding(missingCount int) bool {
	fmt.Println()
	fmt.Printf("🤖 I found %d environment variable(s) that might need configuration.\n", missingCount)
	fmt.Print("   Would you like to set them up now? (y/n): ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// DisplaySecretsResult shows the result of secrets setup
func DisplaySecretsResult(envFile string, savedCount int, skippedCount int) {
	fmt.Println()
	fmt.Println(strings.Repeat("-", 40))
	
	if savedCount > 0 {
		fmt.Printf("✅ Saved %d secret(s) to %s\n", savedCount, envFile)
	}
	if skippedCount > 0 {
		fmt.Printf("⏭️  Skipped %d secret(s) - you can add them later to %s\n", skippedCount, envFile)
	}
	
	// Remind about .gitignore
	fmt.Println()
	fmt.Println("💡 Tip: Make sure .env is in your .gitignore to keep secrets safe!")
	fmt.Println()
}