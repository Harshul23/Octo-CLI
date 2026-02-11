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
	// High-density gradient for ultra-smooth transitions
	welcomeGradient = []string{
		"#059669", "#059669", "#065f46", "#064e3b", // Deep Emeralds
		"#065f46", "#059669", "#10b981", "#10b981", // Transitioning
		"#34d399", "#6ee7b7", "#a7f3d0", "#d1fae5", // Bright Mints
		"#a7f3d0", "#6ee7b7", "#34d399", "#10b981", // Transitioning back
		"#2dd4bf", "#14b8a6", "#0d9488", "#0f766e", // Teals
		"#0d9488", "#14b8a6", "#2dd4bf", "#10b981", // Return to start
	}

	welcomeAccentGreen  = lipgloss.Color("#10b981")
	welcomeAccentDim    = lipgloss.Color("#065f46")
	welcomeAccentBright = lipgloss.Color("#34d399")

	welcomeSubtitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#50ba89ff")).
				Bold(true)

	welcomeCommandStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#34d399")).
				Bold(true)

	welcomeDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#94a3b8"))

	welcomeDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748b"))

	welcomeSectionTitle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#059669")).
				Bold(true).
				Underline(true)

	welcomeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#059669")).
			Padding(1, 3)

	welcomeQuitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#475569")).
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
	// 30ms provides a fluid high-FPS experience without taxing the CPU
	return tea.Tick(time.Millisecond*70, func(t time.Time) tea.Msg {
		return welcomeTickMsg(t)
	})
}

func (m WelcomeModel) Init() tea.Cmd {
	return tea.Batch(
		welcomeTickCmd(),
		tea.EnterAltScreen,
	)
}

func (m WelcomeModel) footerView() string {
	width := m.Width
	if width == 0 {
		width = 100
	}

	dot := ""
	if m.TickCount%30 < 15 {
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
		Render(bar)
}

func (m WelcomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		footerHeight := 2
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
		return "\n  Preparing Octo environment..."
	}

	return m.viewport.View() + "\n" + m.footerView()
}

func (m WelcomeModel) renderContent() string {
	width := m.Width
	if width == 0 {
		width = 100
	}

	var content strings.Builder

	center := func(text string) string {
		return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(text)
	}

	content.WriteString("\n\n")

	// 1. "WELCOME" - Direct mapping to TickCount for high speed
	for i, line := range welcomeTextLarge {
		// No division means it moves at 1 step per 30ms
		colorIdx := (m.TickCount + i) % len(welcomeGradient)
		color := welcomeGradient[colorIdx]
		styled := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(line)
		content.WriteString(center(styled) + "\n")
	}

	// 2. "TO OCTO" - Continuous color flow from above
	for i, line := range toOctoTextLarge {
		colorIdx := (m.TickCount + i + len(welcomeTextLarge)) % len(welcomeGradient)
		color := welcomeGradient[colorIdx]
		styled := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(line)
		content.WriteString(center(styled) + "\n")
	}

	content.WriteString("\n")

	// 3. Subheading with reactive speed
	pulseIdx := (m.TickCount / 4) % len(welcomeGradient)
	subheading := lipgloss.NewStyle().
		Foreground(lipgloss.Color(welcomeGradient[pulseIdx])).
		Bold(true).
		Render("🚀  Now run anything you want  🚀")
	content.WriteString(center(subheading) + "\n\n")

	// 4. Separator
	sepWidth := 56
	if sepWidth > width-4 {
		sepWidth = width - 4
	}
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#064e3b")).
		Render(strings.Repeat("─", sepWidth))
	content.WriteString(center(separator) + "\n\n")

	// 5. Usage section
	content.WriteString(center(welcomeSectionTitle.Render("How to Use Octo")) + "\n\n")

	usageItems := []struct {
		cmd  string
		desc string
	}{
		{"octo init", "Analyze project & generate .octo.yaml"},
		{"octo run", "Run with zero-config deployment"},
		{"octo run --watch", "Auto-restart on file changes"},
		{"octo run --env production", "Run in production mode"},
		{"octo run --port 3000", "Override default port"},
		{"octo run --no-tui", "Plain scrolling output"},
	}

	for _, item := range usageItems {
		cmd := welcomeCommandStyle.Render(fmt.Sprintf("  %-28s", item.cmd))
		desc := welcomeDescStyle.Render(item.desc)
		content.WriteString(center(cmd + desc) + "\n")
	}

	content.WriteString("\n")

	// 6. Quick start box
	quickStart := welcomeDimStyle.Render("cd your-project") + " → " +
		welcomeCommandStyle.Render("octo init") + " → " +
		welcomeCommandStyle.Render("octo run")
	content.WriteString(center(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#475569")).
		Render("Quick Start:  ") + quickStart) + "\n\n")

	content.WriteString(center(separator) + "\n")

	return content.String()
}

func RunWelcomeScreen() bool {
	p := tea.NewProgram(
		NewWelcomeModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err == nil
}

func IsOctoProject(name string, language string, workDir string) bool {
	nameMatch := strings.EqualFold(name, "octo-cli") || strings.EqualFold(name, "octo")
	langMatch := strings.EqualFold(language, "go") || strings.EqualFold(language, "golang")

	_, cmdErr := os.Stat(filepath.Join(workDir, "cmd", "main.go"))
	_, runErr := os.Stat(filepath.Join(workDir, "cmd", "run.go"))
	_, initErr := os.Stat(filepath.Join(workDir, "cmd", "init.go"))
	hasOctoStructure := cmdErr == nil && runErr == nil && initErr == nil

	return nameMatch && langMatch && hasOctoStructure
}