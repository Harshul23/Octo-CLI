package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// EnvPromptResult captures the outcome of the missing environment prompt.
type EnvPromptResult struct {
	EnteredValues  map[string]string
	ShouldContinue bool
	Skipped        bool
}

type envFocusState int

const (
	focusInput envFocusState = iota
	focusSkip
	focusSkipAll
)

// MissingEnvModel provides an interactive Bubble Tea interface for missing environment variables.
// The model is initially focused on the text input so the user can immediately type values,
// while allowing effortless navigation to Skip via Tab, Down, or Esc.
type MissingEnvModel struct {
	root            string
	missingVars     []string
	currentIndex    int
	enteredValues   map[string]string
	textInput       textinput.Model
	activeFocus     envFocusState
	confirmed       bool
	cancelled       bool
	skippedAll      bool
	templateContent string
	templateSaved   bool
	targetFile      string
}

func newMissingEnvModel(root string, missingVars []string, templateContent string) MissingEnvModel {
	// Deduplicate variable names
	deduped := make([]string, 0, len(missingVars))
	seen := make(map[string]bool)
	for _, v := range missingVars {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			deduped = append(deduped, trimmed)
		}
	}

	ti := textinput.New()
	ti.Placeholder = "Enter value (or press Tab to select Skip)..."
	ti.Focus()
	ti.CharLimit = 1024
	ti.Width = 60

	return MissingEnvModel{
		root:            root,
		missingVars:     deduped,
		currentIndex:    0,
		enteredValues:   make(map[string]string),
		textInput:       ti,
		activeFocus:     focusInput,
		templateContent: templateContent,
	}
}

func (m MissingEnvModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m MissingEnvModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit

		case "ctrl+t":
			// Generate safe .env template
			targetFile := ".env.template"
			targetPath := filepath.Join(m.root, targetFile)
			_ = os.WriteFile(targetPath, []byte(m.templateContent), 0644)
			m.templateSaved = true
			m.targetFile = targetFile
			return m, tea.Quit

		case "esc":
			// Esc immediately skips all remaining variables and continues
			m.skippedAll = true
			m.confirmed = true
			return m, tea.Quit

		case "tab", "down", "right":
			// Cycle focus forward: Input -> Skip (-> Skip All if multiple) -> Input
			if len(m.missingVars) > 1 {
				m.activeFocus = (m.activeFocus + 1) % 3
			} else {
				if m.activeFocus == focusInput {
					m.activeFocus = focusSkip
				} else {
					m.activeFocus = focusInput
				}
			}
			if m.activeFocus == focusInput {
				m.textInput.Focus()
			} else {
				m.textInput.Blur()
			}
			return m, nil

		case "shift+tab", "up", "left":
			// Cycle focus backward
			if len(m.missingVars) > 1 {
				if m.activeFocus == focusInput {
					m.activeFocus = focusSkipAll
				} else {
					m.activeFocus--
				}
			} else {
				if m.activeFocus == focusInput {
					m.activeFocus = focusSkip
				} else {
					m.activeFocus = focusInput
				}
			}
			if m.activeFocus == focusInput {
				m.textInput.Focus()
			} else {
				m.textInput.Blur()
			}
			return m, nil

		case "enter":
			if m.currentIndex >= len(m.missingVars) {
				m.confirmed = true
				return m, tea.Quit
			}
			currentVar := m.missingVars[m.currentIndex]

			if m.activeFocus == focusInput {
				val := strings.TrimSpace(m.textInput.Value())
				if strings.EqualFold(val, ":skip") {
					return m.advanceNext()
				}
				if val != "" {
					m.enteredValues[currentVar] = val
				}
				// Empty value on Enter acts as skip for this variable
				return m.advanceNext()
			}

			if m.activeFocus == focusSkip {
				// Explicitly skipped current variable
				return m.advanceNext()
			}

			if m.activeFocus == focusSkipAll {
				// Skip all variables
				m.skippedAll = true
				m.confirmed = true
				return m, tea.Quit
			}
		}
	}

	if m.activeFocus == focusInput {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m MissingEnvModel) advanceNext() (tea.Model, tea.Cmd) {
	m.currentIndex++
	if m.currentIndex >= len(m.missingVars) {
		m.confirmed = true
		return m, tea.Quit
	}

	m.textInput.Reset()
	m.activeFocus = focusInput
	m.textInput.Focus()
	return m, nil
}

func (m MissingEnvModel) View() string {
	if m.templateSaved || m.cancelled || m.currentIndex >= len(m.missingVars) {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")

	// Title banner
	b.WriteString(WarningStyle.Render("⚠️  Missing Required Environment Variables") + "\n")
	b.WriteString(promptSubtitleStyle.Render("Octo detected required environment variables that are not yet set.") + "\n\n")

	currentVar := m.missingVars[m.currentIndex]

	// Current Variable Header with index
	counter := ""
	if len(m.missingVars) > 1 {
		counter = fmt.Sprintf("[%d/%d] ", m.currentIndex+1, len(m.missingVars))
	}
	b.WriteString("  " + promptDimStyle.Render(counter) + promptTitleStyle.Render(currentVar) + "\n")

	// Text Input
	b.WriteString("  ▸ " + m.textInput.View() + "\n\n")

	// Controls / Actions
	b.WriteString("  ")
	if m.activeFocus == focusInput {
		b.WriteString(promptCursorStyle.Render("● ") + promptSelectedStyle.Render("[ Enter Value ]") + "    ")
		b.WriteString(promptDimStyle.Render("○ [ Skip ]"))
	} else if m.activeFocus == focusSkip {
		b.WriteString(promptDimStyle.Render("○ [ Enter Value ]") + "    ")
		b.WriteString(promptCursorStyle.Render("● ") + promptSelectedStyle.Render("[ Skip ]"))
	} else if m.activeFocus == focusSkipAll {
		b.WriteString(promptDimStyle.Render("○ [ Enter Value ]") + "    ")
		b.WriteString(promptDimStyle.Render("○ [ Skip ]"))
	}

	if len(m.missingVars) > 1 {
		b.WriteString("    ")
		if m.activeFocus == focusSkipAll {
			b.WriteString(promptCursorStyle.Render("● ") + promptSelectedStyle.Render("[ Skip All ]"))
		} else {
			b.WriteString(promptDimStyle.Render("○ [ Skip All ]"))
		}
	}
	b.WriteString("\n\n")

	// Helper hotkeys
	help := "Enter: confirm • Tab/↓: select Skip • Esc: skip all • Ctrl+T: generate template • Ctrl+C: cancel"
	b.WriteString(promptDimStyle.Render("  " + help) + "\n")

	return b.String()
}

// PromptMissingEnvironment displays an interactive prompt when required variables are missing.
func PromptMissingEnvironment(root string, missingVars []string, templateContent string) (EnvPromptResult, error) {
	if len(missingVars) == 0 {
		return EnvPromptResult{ShouldContinue: true}, nil
	}

	model := newMissingEnvModel(root, missingVars, templateContent)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return EnvPromptResult{}, err
	}

	resModel, ok := finalModel.(MissingEnvModel)
	if !ok {
		return EnvPromptResult{}, fmt.Errorf("unexpected model type returned from environment prompt")
	}

	if resModel.templateSaved {
		fmt.Println()
		fmt.Println(SuccessLine(fmt.Sprintf("Created %s with placeholder definitions.", resModel.targetFile)))
		fmt.Println(InfoLine(fmt.Sprintf("Please review and configure %s, then run 'octo run' again.", resModel.targetFile)))
		return EnvPromptResult{ShouldContinue: false}, nil
	}

	if resModel.cancelled {
		return EnvPromptResult{ShouldContinue: false}, fmt.Errorf("execution cancelled by user")
	}

	entered := resModel.enteredValues

	// Prompt to save entered values to .env.local if any were provided
	if len(entered) > 0 {
		fmt.Println()
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
	}

	allSkipped := len(entered) == 0 || resModel.skippedAll
	if allSkipped {
		fmt.Println()
		fmt.Println(WarningLine("Continuing execution with missing environment variables..."))
	} else if len(entered) < len(resModel.missingVars) {
		fmt.Println()
		fmt.Println(InfoLine("Continuing execution with provided environment variables (some were skipped)..."))
	}

	return EnvPromptResult{
		EnteredValues:  entered,
		ShouldContinue: true,
		Skipped:        allSkipped || len(entered) < len(resModel.missingVars),
	}, nil
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
