package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// Vite-style Interactive Prompts using Bubbletea
// ============================================================================

var (
	// Styles for prompts
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
				Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#AD8EE6"})

	promptCheckmarkStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#00AA00", Dark: "#00FF00"})

	promptHighlightStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#00AAFF"})

	promptDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"})
)

// ============================================================================
// Yes/No Prompt (Arrow-key navigation)
// ============================================================================

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
	yesCursor := "  "
	noCursor := "  "

	if m.selected {
		yesStyle = promptSelectedStyle
		yesCursor = promptCursorStyle.Render("❯ ")
	} else {
		noStyle = promptSelectedStyle
		noCursor = promptCursorStyle.Render("❯ ")
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

// ============================================================================
// List Selection Prompt (Arrow-key navigation)
// ============================================================================

// SelectOption represents an option in the select prompt
type SelectOption struct {
	Label       string
	Value       string
	Description string
}

// SelectPrompt creates an interactive list selection prompt
type SelectPrompt struct {
	title       string
	description string
	options     []SelectOption
	cursor      int
	confirmed   bool
	cancelled   bool
}

// NewSelectPrompt creates a new selection prompt
func NewSelectPrompt(title, description string, options []SelectOption) *SelectPrompt {
	return &SelectPrompt{
		title:       title,
		description: description,
		options:     options,
		cursor:      0,
	}
}

func (m SelectPrompt) Init() tea.Cmd {
	return nil
}

func (m SelectPrompt) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
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

func (m SelectPrompt) View() string {
	var b strings.Builder

	// Title
	b.WriteString(promptTitleStyle.Render("? "+m.title) + "\n")

	// Description
	if m.description != "" {
		b.WriteString(promptDimStyle.Render("  "+m.description) + "\n")
	}

	b.WriteString("\n")

	// Options
	for i, opt := range m.options {
		cursor := "  "
		style := promptUnselectedStyle

		if i == m.cursor {
			cursor = promptCursorStyle.Render("❯ ")
			style = promptSelectedStyle
		}

		b.WriteString(cursor + style.Render(opt.Label))

		if opt.Description != "" && i == m.cursor {
			b.WriteString(promptDimStyle.Render(" - " + opt.Description))
		}
		b.WriteString("\n")
	}

	// Help text
	b.WriteString("\n")
	b.WriteString(promptDimStyle.Render("  ↑ ↓ to navigate • enter to select • esc to cancel"))

	return b.String()
}

// Result returns the selected option and whether it was confirmed
func (m SelectPrompt) Result() (SelectOption, bool) {
	if m.cursor < 0 || m.cursor >= len(m.options) {
		return SelectOption{}, false
	}
	return m.options[m.cursor], m.confirmed && !m.cancelled
}

// RunSelectPrompt runs the selection prompt and returns the result
func RunSelectPrompt(title, description string, options []SelectOption) (SelectOption, error) {
	prompt := NewSelectPrompt(title, description, options)
	p := tea.NewProgram(prompt)

	model, err := p.Run()
	if err != nil {
		return SelectOption{}, err
	}

	result := model.(SelectPrompt)
	selected, confirmed := result.Result()

	if !confirmed {
		return SelectOption{}, nil
	}

	return selected, nil
}

// ============================================================================
// Text Input Prompt
// ============================================================================

// TextInputPrompt creates an interactive text input prompt
type TextInputPrompt struct {
	title       string
	description string
	placeholder string
	defaultVal  string
	input       textinput.Model
	confirmed   bool
	cancelled   bool
}

// NewTextInputPrompt creates a new text input prompt
func NewTextInputPrompt(title, description, placeholder, defaultVal string) *TextInputPrompt {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	if defaultVal != "" {
		ti.SetValue(defaultVal)
	}

	return &TextInputPrompt{
		title:       title,
		description: description,
		placeholder: placeholder,
		defaultVal:  defaultVal,
		input:       ti,
	}
}

func (m TextInputPrompt) Init() tea.Cmd {
	return textinput.Blink
}

func (m TextInputPrompt) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.confirmed = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m TextInputPrompt) View() string {
	var b strings.Builder

	// Title
	b.WriteString(promptTitleStyle.Render("? "+m.title) + "\n")

	// Description
	if m.description != "" {
		b.WriteString(promptDimStyle.Render("  "+m.description) + "\n")
	}

	b.WriteString("\n")

	// Input field
	b.WriteString("  " + m.input.View() + "\n")

	// Show default hint
	if m.defaultVal != "" && m.input.Value() == "" {
		b.WriteString(promptDimStyle.Render(fmt.Sprintf("  Press enter to use: %s", m.defaultVal)) + "\n")
	}

	// Help text
	b.WriteString("\n")
	b.WriteString(promptDimStyle.Render("  enter to confirm • esc to cancel"))

	return b.String()
}

// Result returns the entered value and whether it was confirmed
func (m TextInputPrompt) Result() (string, bool) {
	value := m.input.Value()
	if value == "" && m.defaultVal != "" {
		value = m.defaultVal
	}
	return value, m.confirmed && !m.cancelled
}

// RunTextInputPrompt runs the text input prompt and returns the result
func RunTextInputPrompt(title, description, placeholder, defaultVal string) (string, error) {
	prompt := NewTextInputPrompt(title, description, placeholder, defaultVal)
	p := tea.NewProgram(prompt)

	model, err := p.Run()
	if err != nil {
		return "", err
	}

	result := model.(TextInputPrompt)
	value, confirmed := result.Result()

	if !confirmed {
		return "", nil
	}

	return value, nil
}

// ============================================================================
// Multi-Select Prompt (Checkboxes)
// ============================================================================

// MultiSelectPrompt allows selecting multiple items
type MultiSelectPrompt struct {
	title       string
	description string
	options     []SelectOption
	cursor      int
	selected    map[int]bool
	confirmed   bool
	cancelled   bool
}

// NewMultiSelectPrompt creates a new multi-select prompt
func NewMultiSelectPrompt(title, description string, options []SelectOption) *MultiSelectPrompt {
	return &MultiSelectPrompt{
		title:       title,
		description: description,
		options:     options,
		cursor:      0,
		selected:    make(map[int]bool),
	}
}

func (m MultiSelectPrompt) Init() tea.Cmd {
	return nil
}

func (m MultiSelectPrompt) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case " ", "x":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "a":
			// Toggle all
			allSelected := true
			for i := range m.options {
				if !m.selected[i] {
					allSelected = false
					break
				}
			}
			for i := range m.options {
				m.selected[i] = !allSelected
			}
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

func (m MultiSelectPrompt) View() string {
	var b strings.Builder

	// Title
	b.WriteString(promptTitleStyle.Render("? "+m.title) + "\n")

	// Description
	if m.description != "" {
		b.WriteString(promptDimStyle.Render("  "+m.description) + "\n")
	}

	b.WriteString("\n")

	// Options
	for i, opt := range m.options {
		cursor := "  "
		style := promptUnselectedStyle
		checkbox := "○"

		if i == m.cursor {
			cursor = promptCursorStyle.Render("❯ ")
			style = promptHighlightStyle
		}

		if m.selected[i] {
			checkbox = promptCheckmarkStyle.Render("●")
		}

		b.WriteString(cursor + checkbox + " " + style.Render(opt.Label))

		if opt.Description != "" && i == m.cursor {
			b.WriteString(promptDimStyle.Render(" - " + opt.Description))
		}
		b.WriteString("\n")
	}

	// Count
	count := 0
	for _, v := range m.selected {
		if v {
			count++
		}
	}

	b.WriteString("\n")
	b.WriteString(promptDimStyle.Render(fmt.Sprintf("  %d selected", count)) + "\n")

	// Help text
	b.WriteString("\n")
	b.WriteString(promptDimStyle.Render("  ↑ ↓ navigate • space to toggle • a to toggle all • enter to confirm"))

	return b.String()
}

// Result returns the selected options and whether it was confirmed
func (m MultiSelectPrompt) Result() ([]SelectOption, bool) {
	var result []SelectOption
	for i, opt := range m.options {
		if m.selected[i] {
			result = append(result, opt)
		}
	}
	return result, m.confirmed && !m.cancelled
}

// RunMultiSelectPrompt runs the multi-select prompt and returns the results
func RunMultiSelectPrompt(title, description string, options []SelectOption) ([]SelectOption, error) {
	prompt := NewMultiSelectPrompt(title, description, options)
	p := tea.NewProgram(prompt)

	model, err := p.Run()
	if err != nil {
		return nil, err
	}

	result := model.(MultiSelectPrompt)
	selected, confirmed := result.Result()

	if !confirmed {
		return nil, nil
	}

	return selected, nil
}

// ============================================================================
// Styled Output Helpers
// ============================================================================

// PrintHeader prints a styled header
func PrintHeader(text string) {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#AD8EE6"}).
		MarginBottom(1)
	fmt.Println(style.Render("  " + text))
}

// PrintStep prints a step in the process
func PrintStep(step int, total int, text string) {
	stepStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#00AAFF"})
	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#EEEEEE"})

	stepNum := fmt.Sprintf("[%d/%d]", step, total)
	fmt.Println(stepStyle.Render(stepNum) + " " + textStyle.Render(text))
}

// PrintSuccess prints a success message with checkmark
func PrintSuccess(text string) {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#00AA00", Dark: "#00FF00"})
	fmt.Println(style.Render("✔") + " " + text)
}

// PrintWarning prints a warning message
func PrintWarning(text string) {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#CC6600", Dark: "#FFAA00"})
	fmt.Println(style.Render("⚠") + " " + text)
}

// PrintError prints an error message
func PrintError(text string) {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF0000"})
	fmt.Println(style.Render("✖") + " " + text)
}

// PrintInfo prints an info message
func PrintInfo(text string) {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#00AAFF"})
	fmt.Println(style.Render("ℹ") + " " + text)
}

// PrintHighlight prints highlighted text
func PrintHighlight(label, value string) {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"})
	valueStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#FFFFFF"})
	fmt.Println("  " + labelStyle.Render(label+":") + " " + valueStyle.Render(value))
}

// PrintDivider prints a styled divider
func PrintDivider() {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"})
	fmt.Println(style.Render("  " + strings.Repeat("─", 50)))
}

// PrintBox prints text in a styled box
func PrintBox(title, content string) {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#AD8EE6"}).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#AD8EE6"})

	if title != "" {
		fmt.Println(titleStyle.Render("  " + title))
	}
	fmt.Println(boxStyle.Render(content))
}
