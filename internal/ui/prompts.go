package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	promptTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#AD8EE6"})

	promptSubtitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"})

	promptSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#00AA00", Dark: "#00FF00"})

	promptUnselectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"})

	promptCursorStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"})

	promptDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
)

// YesNoPrompt creates an interactive yes/no prompt
type YesNoPrompt struct {
	question    string
	description string
	selected    bool // true = Yes, false = No
	confirmed   bool
	cancelled   bool
}

// NewYesNoPrompt creates a new yes/no prompt
func NewYesNoPrompt(question, description string, defaultYes bool) *YesNoPrompt {
	return &YesNoPrompt{
		question:    question,
		description: description,
		selected:    defaultYes,
	}
}

func (m YesNoPrompt) Init() tea.Cmd {
	return nil
}

func (m YesNoPrompt) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h", "y", "Y":
			m.selected = true
		case "right", "l", "n", "N":
			m.selected = false
		case "tab":
			m.selected = !m.selected
		case "enter":
			m.confirmed = true
			return m, tea.Quit
		case "ctrl+c", "esc", "q":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m YesNoPrompt) View() string {
	var b strings.Builder

	// Question
	b.WriteString(promptTitleStyle.Render("? "+m.question) + "\n")

	// Description (if any)
	if m.description != "" {
		b.WriteString(promptDimStyle.Render("  "+m.description) + "\n")
	}

	// Options
	yesStyle := promptUnselectedStyle
	noStyle := promptUnselectedStyle
	yesCursor := promptDimStyle.Render("○ ")
	noCursor := promptDimStyle.Render("○ ")

	if m.selected {
		yesStyle = promptSelectedStyle
		yesCursor = promptCursorStyle.Render("● ")
	} else {
		noStyle = promptSelectedStyle
		noCursor = promptCursorStyle.Render("● ")
	}

	b.WriteString("\n")
	b.WriteString(yesCursor + yesStyle.Render("Yes") + "    ")
	b.WriteString(noCursor + noStyle.Render("No") + "\n")

	// Help text
	b.WriteString("\n")
	b.WriteString(promptDimStyle.Render("  ← → to select • enter to confirm • esc to cancel"))

	return b.String()
}

// Result returns the selected value and whether it was confirmed
func (m YesNoPrompt) Result() (bool, bool) {
	return m.selected, m.confirmed && !m.cancelled
}

// RunYesNoPrompt runs the yes/no prompt and returns the result
func RunYesNoPrompt(question, description string, defaultYes bool) (bool, error) {
	prompt := NewYesNoPrompt(question, description, defaultYes)
	p := tea.NewProgram(prompt)

	model, err := p.Run()
	if err != nil {
		return false, err
	}

	result := model.(YesNoPrompt)
	selected, confirmed := result.Result()

	if !confirmed {
		return false, nil
	}

	return selected, nil
}
