package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvPromptResult captures the outcome of the missing environment prompt.
type EnvPromptResult struct {
	EnteredValues  map[string]string
	ShouldContinue bool
	Skipped        bool
}

// PromptMissingEnvironment displays an interactive prompt when required variables are missing.
func PromptMissingEnvironment(root string, missingVars []string, templateContent string) (EnvPromptResult, error) {
	fmt.Println()
	fmt.Println(WarningStyle.Render("⚠️  Missing Required Environment Variables:"))
	for _, v := range missingVars {
		fmt.Printf("   %s %s\n", Accent.Render("•"), CommandStyle.Render(v))
	}
	fmt.Println()

	options := []SelectOption{
		{
			Label:       "Enter values interactively",
			Value:       "enter",
			Description: "Provide values in terminal for this execution",
		},
		{
			Label:       "Generate .env template",
			Value:       "template",
			Description: "Create a template file with safe placeholder comments",
		},
		{
			Label:       "Skip and continue anyway",
			Value:       "skip",
			Description: "Proceed execution without providing these variables",
		},
	}

	selected, err := RunSelectPrompt("How would you like to handle missing environment variables?", "Select an option using arrow keys", options)
	if err != nil {
		return EnvPromptResult{}, err
	}

	switch selected.Value {
	case "enter":
		entered := make(map[string]string)
		for _, varName := range missingVars {
			val, err := RunTextInputPrompt(
				fmt.Sprintf("Enter value for %s", varName),
				"Required environment variable",
				"value",
				"",
			)
			if err != nil {
				return EnvPromptResult{}, err
			}
			entered[varName] = val
		}

		// Prompt to save to .env.local
		save, err := RunYesNoPrompt(
			"Save entered values to .env.local?",
			"Avoids re-entering them on subsequent runs (.env.local is gitignored)",
			true,
		)
		if err == nil && save {
			savePath := filepath.Join(root, ".env.local")
			if err := appendOrWriteEnv(savePath, entered); err != nil {
				fmt.Println(WarningLine(fmt.Sprintf("Failed to save .env.local: %v", err)))
			} else {
				fmt.Println(SuccessLine("Saved values to .env.local"))
			}
		}

		return EnvPromptResult{
			EnteredValues:  entered,
			ShouldContinue: true,
		}, nil

	case "template":
		writeToEnv, err := RunYesNoPrompt(
			"Write template directly to .env?",
			"Choose No to create .env.template instead",
			false,
		)
		if err != nil {
			return EnvPromptResult{}, err
		}

		targetFile := ".env.template"
		if writeToEnv {
			targetFile = ".env"
		}
		targetPath := filepath.Join(root, targetFile)

		if err := os.WriteFile(targetPath, []byte(templateContent), 0644); err != nil {
			return EnvPromptResult{}, fmt.Errorf("failed to write %s: %w", targetFile, err)
		}

		fmt.Println(SuccessLine(fmt.Sprintf("Created %s with placeholder definitions.", targetFile)))
		fmt.Println(InfoLine(fmt.Sprintf("Please review and configure %s, then run 'octo run' again.", targetFile)))
		return EnvPromptResult{
			ShouldContinue: false,
		}, nil

	case "skip":
		fmt.Println(WarningLine("Continuing execution with missing environment variables..."))
		return EnvPromptResult{
			ShouldContinue: true,
			Skipped:        true,
		}, nil

	default:
		return EnvPromptResult{ShouldContinue: false}, fmt.Errorf("cancelled by user")
	}
}

func appendOrWriteEnv(path string, values map[string]string) error {
	var lines []string
	if existing, err := os.ReadFile(path); err == nil && len(existing) > 0 {
		lines = append(lines, strings.TrimRight(string(existing), "\n"))
	}

	for k, v := range values {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0600)
}
