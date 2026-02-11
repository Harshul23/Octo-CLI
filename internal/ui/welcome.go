package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Big ASCII Art "Welcome to Octo" ---

var welcomeTextLarge = []string{
	" ██╗    ██╗███████╗██╗      ██████╗ ██████╗ ███╗   ███╗███████╗",
	" ██║    ██║██╔════╝██║     ██╔════╝██╔═══██╗████╗ ████║██╔════╝",
	" ██║ █╗ ██║█████╗  ██║     ██║     ██║   ██║██╔████╔██║█████╗  ",
	" ██║███╗██║██╔══╝  ██║     ██║     ██║   ██║██║╚██╔╝██║██╔══╝  ",
	" ╚███╔███╔╝███████╗███████╗╚██████╗╚██████╔╝██║ ╚═╝ ██║███████╗",
	"  ╚══╝╚══╝ ╚══════╝╚══════╝ ╚═════╝ ╚═════╝ ╚═╝     ╚═╝╚══════╝",
}

var toOctoTextLarge = []string{
	" ████████╗ ██████╗      ██████╗  ██████╗████████╗ ██████╗ ",
	" ╚══██╔══╝██╔═══██╗    ██╔═══██╗██╔════╝╚══██╔══╝██╔═══██╗",
	"    ██║   ██║   ██║    ██║   ██║██║        ██║   ██║   ██║",
	"    ██║   ██║   ██║    ██║   ██║██║        ██║   ██║   ██║",
	"    ██║   ╚██████╔╝    ╚██████╔╝╚██████╗   ██║   ╚██████╔╝",
	"    ╚═╝    ╚═════╝      ╚═════╝  ╚═════╝   ╚═╝    ╚═════╝ ",
}

// --- Welcome Styles ---
var (
	welcomeGradient = []string{
		"#022c22", "#064e3b", "#065f46", "#047857", "#059669",
		"#10b981", "#34d399", "#6ee7b7", "#a7f3d0", "#d1fae5",
		"#ecfdf5", "#ffffff", "#ecfdf5", "#d1fae5", "#a7f3d0",
		"#6ee7b7", "#34d399", "#10b981", "#059669", "#047857",
	}

	welcomeAccentGreen = lipgloss.Color("#10b981")
	welcomeAccentDim   = lipgloss.Color("#475569")
	welcomeAccentBright = lipgloss.Color("#34d399")

	welcomeSubtitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#a7f3d0")).
				Bold(true)

	welcomeCommandStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#34d399")).
				Bold(true)

	welcomeDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#94a3b8"))

	welcomeDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#475569"))

	welcomeSectionTitle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6ee7b7")).
				Bold(true).
				Underline(true)

	welcomeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#065f46")).
			Padding(1, 3)

	welcomeQuitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748b")).
			Italic(true)
)

// --- Welcome Model ---

type WelcomeModel struct {
	TickCount int
	Width     int
	Height    int
	Quitting  bool
	viewport  viewport.Model
	ready     bool
}

func NewWelcomeModel() WelcomeModel {
	return WelcomeModel{
		TickCount: 0,
	}
}

type welcomeTickMsg time.Time

func welcomeTickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(t time.Time) tea.Msg {
		return welcomeTickMsg(t)
	})
}

func (m WelcomeModel) Init() tea.Cmd {
	return tea.Batch(
		welcomeTickCmd(),
		tea.EnterAltScreen,
	)
}

// footerView renders the fixed footer bar
func (m WelcomeModel) footerView() string {
	width := m.Width
	if width == 0 {
		width = 100
	}

	// Blinking dot
	dot := ""
	if m.TickCount%10 < 5 {
		dot = lipgloss.NewStyle().Foreground(welcomeAccentBright).Render("●")
	} else {
		dot = lipgloss.NewStyle().Foreground(welcomeAccentDim).Render("●")
	}

	quitText := welcomeQuitStyle.Render("Press ") +
		lipgloss.NewStyle().Foreground(welcomeAccentGreen).Bold(true).Render("q") +
		welcomeQuitStyle.Render(" to exit")

	scrollHint := welcomeDimStyle.Render("↑/↓ scroll")

	bar := dot + "  " + quitText + "    " + scrollHint + "  " + dot

	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color("#475569")).
		Render(bar)
}

func (m WelcomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		footerHeight := 2 // footer line + blank line above it
		viewHeight := m.Height - footerHeight
		if viewHeight < 1 {
			viewHeight = 1
		}

		if !m.ready {
			m.viewport = viewport.New(m.Width, viewHeight)
			m.viewport.SetContent(m.renderContent())
			m.ready = true
		} else {
			m.viewport.Width = m.Width
			m.viewport.Height = viewHeight
			m.viewport.SetContent(m.renderContent())
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.Quitting = true
			return m, tea.Quit
		}

	case welcomeTickMsg:
		m.TickCount++
		if m.ready {
			m.viewport.SetContent(m.renderContent())
		}
		cmds = append(cmds, welcomeTickCmd())
	}

	// Forward remaining messages to viewport (for scrolling, mouse, etc.)
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

func (m WelcomeModel) View() string {
	if m.Quitting {
		return ""
	}
	if !m.ready {
		return "\n  Loading..."
	}

	return m.viewport.View() + "\n" + m.footerView()
}

// renderContent builds the full scrollable content
func (m WelcomeModel) renderContent() string {
	width := m.Width
	if width == 0 {
		width = 100
	}

	var content strings.Builder

	center := func(text string) string {
		return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(text)
	}

	// Small top padding
	content.WriteString("\n\n")

	// 1. "WELCOME" in big text with wave animation
	for i, line := range welcomeTextLarge {
		colorIdx := (m.TickCount + i*2) % len(welcomeGradient)
		color := welcomeGradient[colorIdx]
		styled := lipgloss.NewStyle().
			Foreground(lipgloss.Color(color)).
			Bold(true).
			Render(line)
		content.WriteString(center(styled))
		content.WriteString("\n")
	}

	// 2. "TO OCTO" in big text with offset wave
	for i, line := range toOctoTextLarge {
		colorIdx := (m.TickCount + (i+6)*2) % len(welcomeGradient)
		color := welcomeGradient[colorIdx]
		styled := lipgloss.NewStyle().
			Foreground(lipgloss.Color(color)).
			Bold(true).
			Render(line)
		content.WriteString(center(styled))
		content.WriteString("\n")
	}

	content.WriteString("\n")

	// 3. Subheading - "Now run anything you want" with a pulsing glow
	pulseColors := []string{"#6ee7b7", "#a7f3d0", "#d1fae5", "#ecfdf5", "#ffffff", "#ecfdf5", "#d1fae5", "#a7f3d0", "#6ee7b7"}
	pulseIdx := (m.TickCount / 2) % len(pulseColors)
	subheading := lipgloss.NewStyle().
		Foreground(lipgloss.Color(pulseColors[pulseIdx])).
		Bold(true).
		Render("🚀  Now run anything you want  🚀")
	content.WriteString(center(subheading))
	content.WriteString("\n\n")

	// 4. Separator
	sepWidth := 56
	if sepWidth > width-4 {
		sepWidth = width - 4
	}
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#065f46")).
		Render(strings.Repeat("─", sepWidth))
	content.WriteString(center(separator))
	content.WriteString("\n\n")

	// 5. Usage section
	content.WriteString(center(welcomeSectionTitle.Render("How to Use Octo")))
	content.WriteString("\n\n")

	usageItems := []struct {
		cmd  string
		desc string
	}{
		{"octo init", "Analyze your project & generate .octo.yaml config"},
		{"octo run", "Run your project with zero-config deployment"},
		{"octo run --watch", "Run with auto-restart on file changes"},
		{"octo run --env production", "Run in production mode"},
		{"octo run --port 3000", "Override the default port"},
		{"octo run --no-tui", "Run with plain scrolling output"},
	}

	for _, item := range usageItems {
		cmd := welcomeCommandStyle.Render(fmt.Sprintf("  %-28s", item.cmd))
		desc := welcomeDescStyle.Render(item.desc)
		content.WriteString(center(cmd + "" + desc))
		content.WriteString("\n")
	}

	content.WriteString("\n")

	// 6. Quick start box
	quickStart := welcomeDimStyle.Render("cd your-project") + " → " +
		welcomeCommandStyle.Render("octo init") + " → " +
		welcomeCommandStyle.Render("octo run")
	content.WriteString(center(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#475569")).
		Render("Quick Start:  ") + quickStart))
	content.WriteString("\n\n")

	// 7. Separator
	content.WriteString(center(separator))
	content.WriteString("\n")

	return content.String()
}

// RunWelcomeScreen launches the persistent welcome TUI.
// Returns true if it exited normally.
func RunWelcomeScreen() bool {
	p := tea.NewProgram(
		NewWelcomeModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err == nil
}

// IsOctoProject checks whether the given blueprint represents the Octo CLI project itself.
func IsOctoProject(name string, language string, workDir string) bool {
	// Check by name
	nameMatch := strings.EqualFold(name, "octo-cli") || strings.EqualFold(name, "octo")

	// Check language is Go
	langMatch := strings.EqualFold(language, "go") || strings.EqualFold(language, "golang")

	// Check for cmd/main.go with octo structure (a strong signal)
	_, cmdErr := os.Stat(filepath.Join(workDir, "cmd", "main.go"))
	_, runErr := os.Stat(filepath.Join(workDir, "cmd", "run.go"))
	_, initErr := os.Stat(filepath.Join(workDir, "cmd", "init.go"))
	hasOctoStructure := cmdErr == nil && runErr == nil && initErr == nil

	return nameMatch && langMatch && hasOctoStructure
}
