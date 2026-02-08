package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Cached pixel art logo
var pixelArtLogo []string
var pixelArtLogoSmall []string

// --- Large ASCII Art "OCTO" Text (GitHub CLI style) ---

var octoTextLarge = []string{
	"  ██████╗  ██████╗████████╗ ██████╗ ",
	" ██╔═══██╗██╔════╝╚══██╔══╝██╔═══██╗",
	" ██║   ██║██║        ██║   ██║   ██║",
	" ██║   ██║██║        ██║   ██║   ██║",
	" ╚██████╔╝╚██████╗   ██║   ╚██████╔╝",
	"  ╚═════╝  ╚═════╝   ╚═╝    ╚═════╝ ",
}

var octoTextSmall = []string{
	" ╔═╗╔═╗╔╦╗╔═╗ ",
	" ║ ║║   ║ ║ ║ ",
	" ╚═╝╚═╝ ╩ ╚═╝ ",
}

// --- Hand-crafted Pixel Art Logo ---
// Based on the Octo logo: circular black background with white octopus/play shape

func init() {
	// Create hand-crafted pixel art logo that matches the actual logo.png
	pixelArtLogo = createOctoLogo()
	pixelArtLogoSmall = createOctoLogoSmall()
}

// createOctoLogo creates the main logo using ANSI colors
// Logo: Black circle with white octopus head + play button
func createOctoLogo() []string {
	// Using block characters with ANSI colors
	// B = black (logo background), W = white (octopus shape), _ = transparent
	
	b := "\x1b[38;2;0;0;0m█\x1b[0m"           // Black block
	w := "\x1b[38;2;255;255;255m█\x1b[0m"     // White block
	s := " "                                   // Space (transparent)
	
	// Half block versions for smoother edges
	bTop := "\x1b[38;2;0;0;0m▀\x1b[0m"
	bBot := "\x1b[38;2;0;0;0m▄\x1b[0m"
	
	return []string{
		s + s + s + bBot + b + b + b + b + b + b + bBot + s + s + s,
		s + s + bBot + b + b + b + b + b + b + b + b + bBot + s + s,
		s + bBot + b + b + w + w + w + w + w + b + b + b + bBot + s,
		s + b + b + w + w + w + w + w + w + w + w + b + b + s,
		s + b + b + w + w + b + b + w + w + w + w + b + b + s,
		s + b + b + w + w + b + b + b + w + w + w + b + b + s,
		s + b + b + w + w + b + b + b + b + w + w + b + b + s,
		s + b + b + w + w + b + b + b + w + w + w + b + b + s,
		s + b + b + w + w + b + b + w + w + w + w + b + b + s,
		s + b + b + w + w + w + w + w + w + w + w + b + b + s,
		s + b + b + b + w + w + w + w + w + b + b + b + b + s,
		s + b + b + b + bTop + w + bTop + w + bTop + b + b + b + b + s,
		s + s + bTop + b + b + w + b + w + b + w + b + bTop + s + s,
		s + s + s + bTop + b + bTop + b + bTop + b + bTop + bTop + s + s + s,
	}
}

// createOctoLogoSmall creates a smaller version
func createOctoLogoSmall() []string {
	b := "\x1b[38;2;0;0;0m█\x1b[0m"
	w := "\x1b[38;2;255;255;255m█\x1b[0m"
	s := " "
	bBot := "\x1b[38;2;0;0;0m▄\x1b[0m"
	bTop := "\x1b[38;2;0;0;0m▀\x1b[0m"
	
	return []string{
		s + s + bBot + b + b + b + b + bBot + s + s,
		s + b + b + w + w + w + w + b + b + s,
		s + b + w + w + b + w + w + w + b + s,
		s + b + w + w + b + w + w + w + b + s,
		s + b + b + w + w + w + w + b + b + s,
		s + b + b + bTop + w + bTop + w + b + b + s,
		s + s + bTop + b + bTop + b + bTop + bTop + s + s,
	}
}

// --- Styles ---

var (
	// The "Octo-Green" Gradient Palette - vibrant emerald tones
	scanPalette = []string{
		"#022c22", "#064e3b", "#065f46", "#047857", "#059669",
		"#10b981", "#34d399", "#6ee7b7", "#a7f3d0", "#d1fae5",
		"#ecfdf5", "#ffffff", "#ecfdf5", "#d1fae5", "#a7f3d0",
		"#6ee7b7", "#34d399", "#10b981", "#059669", "#047857",
	}

	// Accent colors
	accentGreen  = lipgloss.Color("#10b981")
	accentWhite  = lipgloss.Color("#f8fafc")
	accentDim    = lipgloss.Color("#475569")
	accentBright = lipgloss.Color("#34d399")

	// Styles
	subtitleStyle = lipgloss.NewStyle().
			Foreground(accentGreen).
			Italic(true)

	subtleStyle = lipgloss.NewStyle().
			Foreground(accentDim)

	textStyle = lipgloss.NewStyle().
			Foreground(accentWhite).
			Bold(true)

	versionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748b"))

	progressBgStyle = lipgloss.NewStyle().
			Foreground(accentDim)
)

// --- Model ---

type IntroModel struct {
	TickCount      int
	Quitting       bool
	Finished       bool
	Width          int
	Height         int
	UseCompactLogo bool
}

func NewIntroModel() IntroModel {
	return IntroModel{
		TickCount: 0,
	}
}

func (m IntroModel) Init() tea.Cmd {
	return tea.Batch(
		introTickCmd(),
		tea.EnterAltScreen,
	)
}

// --- Update ---

type introTickMsg time.Time

func introTickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*45, func(t time.Time) tea.Msg {
		return introTickMsg(t)
	})
}

func (m IntroModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.UseCompactLogo = m.Width < 50 || m.Height < 25

	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "esc", " ", "q":
			m.Quitting = true
			return m, tea.Quit
		}

	case introTickMsg:
		if m.TickCount > 65 { // Animation duration (~3s)
			m.Finished = true
			m.Quitting = true
			return m, tea.Quit
		}
		m.TickCount++
		return m, introTickCmd()
	}

	return m, nil
}

// --- View ---

func (m IntroModel) View() string {
	if m.Quitting {
		return ""
	}

	var s strings.Builder
	width := m.Width
	if width == 0 {
		width = 80
	}
	height := m.Height
	if height == 0 {
		height = 24
	}

	// Determine if we need compact layout (height < 15 lines)
	compactLayout := height < 15

	if compactLayout {
		// === COMPACT LAYOUT: OCTO on left, info on right ===
		return renderCompactLayout(m.TickCount, width, height)
	}

	// === FULL LAYOUT ===
	// Calculate vertical centering
	contentHeight := 14 // OCTO (6) + tagline (1) + version (1) + skip (1) + spacing (5)
	topPadding := (height - contentHeight) / 3
	if topPadding < 1 {
		topPadding = 1
	}

	s.WriteString(strings.Repeat("\n", topPadding))

	// === RENDER OCTO TEXT (always large) ===
	s.WriteString(renderOctoText(m.TickCount, width, false))

	s.WriteString("\n")

	// === TAGLINE ===
	tagline := subtitleStyle.Render("Zero-Config Local Deployment")
	s.WriteString(lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(tagline))
	s.WriteString("\n\n")

	// === VERSION INFO ===
	version := versionStyle.Render("v0.1.0")
	s.WriteString(lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(version))
	s.WriteString("\n\n")

	// === SKIP INSTRUCTION ===
	skipOpacity := m.TickCount % 20
	if skipOpacity > 10 {
		skipOpacity = 20 - skipOpacity
	}
	skipStyle := subtleStyle
	if skipOpacity > 5 {
		skipStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b"))
	}
	s.WriteString(lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(
		skipStyle.Render("Press [Enter] to continue"),
	))

	return s.String()
}

// renderCompactLayout renders OCTO text on left with info on right for short terminals
func renderCompactLayout(tick int, width int, height int) string {
	var result strings.Builder

	// Calculate top padding
	topPadding := (height - len(octoTextLarge)) / 2
	if topPadding < 0 {
		topPadding = 0
	}
	result.WriteString(strings.Repeat("\n", topPadding))

	// Build info lines for the right side
	infoLines := []string{
		"",
		subtitleStyle.Render("Zero-Config Local Deployment"),
		"",
		versionStyle.Render("v0.1.0"),
		"",
		subtleStyle.Render("Press [Enter] to continue"),
	}

	// Render OCTO text with info on the right
	maxLines := len(octoTextLarge)
	if len(infoLines) > maxLines {
		maxLines = len(infoLines)
	}

	// Calculate offset to vertically center info with OCTO
	infoOffset := (len(octoTextLarge) - len(infoLines)) / 2

	for i := 0; i < maxLines; i++ {
		var octoLine string
		if i < len(octoTextLarge) {
			// Animated color for OCTO
			colorIdx := (tick + i*2) % len(scanPalette)
			color := scanPalette[colorIdx]
			octoLine = lipgloss.NewStyle().
				Foreground(lipgloss.Color(color)).
				Bold(true).
				Render(octoTextLarge[i])
		} else {
			octoLine = strings.Repeat(" ", len(octoTextLarge[0]))
		}

		var infoLine string
		infoIdx := i - infoOffset
		if infoIdx >= 0 && infoIdx < len(infoLines) {
			infoLine = infoLines[infoIdx]
		}

		// Combine: OCTO + spacing + info
		combined := octoLine + "    " + infoLine
		result.WriteString(lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(combined))
		result.WriteString("\n")
	}

	return result.String()
}

// --- Render OCTO Text Only (Fallback) ---

func renderOctoText(tick int, width int, compact bool) string {
	var result strings.Builder

	octoText := octoTextLarge
	if compact {
		octoText = octoTextSmall
	}

	for i, line := range octoText {
		// Wave animation effect
		colorIdx := (tick + i*2) % len(scanPalette)
		color := scanPalette[colorIdx]

		styledLine := lipgloss.NewStyle().
			Foreground(lipgloss.Color(color)).
			Bold(true).
			Render(line)

		result.WriteString(lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(styledLine))
		result.WriteString("\n")
	}

	return result.String()
}

// --- Render ASCII Logo with OCTO Text Side by Side ---

func renderASCIILogoWithText(tick int, width int, compact bool) string {
	var result strings.Builder

	// Get the appropriate text and logo
	octoText := octoTextLarge
	logo := pixelArtLogo
	if compact {
		octoText = octoTextSmall
		logo = pixelArtLogoSmall
	}

	// If pixel art failed, return just text
	if len(logo) == 0 {
		return renderOctoText(tick, width, compact)
	}

	// Calculate animated color for OCTO text
	octoColor := getAnimatedColor(tick)

	// Styles
	octoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(octoColor)).
		Bold(true)

	// Build combined layout - OCTO text on left, logo on right
	maxLines := len(octoText)
	if len(logo) > maxLines {
		maxLines = len(logo)
	}

	// Calculate vertical offset to center them
	octoOffset := (maxLines - len(octoText)) / 2
	logoOffset := (maxLines - len(logo)) / 2

	for i := 0; i < maxLines; i++ {
		var octoLine string
		octoIdx := i - octoOffset
		if octoIdx >= 0 && octoIdx < len(octoText) {
			octoLine = octoStyle.Render(octoText[octoIdx])
		} else {
			// Pad with spaces to maintain alignment
			if len(octoText) > 0 {
				octoLine = strings.Repeat(" ", len(octoText[0]))
			}
		}

		var logoLine string
		logoIdx := i - logoOffset
		if logoIdx >= 0 && logoIdx < len(logo) {
			// Logo already has colors from PNG conversion
			logoLine = logo[logoIdx]
		} else {
			// Pad with spaces (approximate width)
			logoLine = strings.Repeat(" ", 24)
		}

		// Combine: OCTO text + spacing + logo (logo on RIGHT side)
		combined := octoLine + "  " + logoLine
		result.WriteString(lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(combined))
		result.WriteString("\n")
	}

	return result.String()
}

// --- Animation Helpers ---

func getAnimatedColor(tick int) string {
	// Cycle through the palette for a smooth color transition
	idx := (tick / 3) % len(scanPalette)
	return scanPalette[idx]
}

func getAnimatedLoadingText(tick int) string {
	messages := []struct {
		start int
		text  string
	}{
		{0, "Initializing Octo Engine"},
		{18, "Scanning Project Structure"},
		{32, "Loading Dependency Graph"},
		{46, "Preparing Environment"},
		{58, "Ready to go!"},
	}

	var currentMsg string
	var msgStart int

	for _, m := range messages {
		if tick >= m.start {
			currentMsg = m.text
			msgStart = m.start
		}
	}

	if currentMsg == "" {
		return ""
	}

	// Typing effect
	elapsed := tick - msgStart
	charsToShow := elapsed * 2
	if charsToShow > len(currentMsg) {
		charsToShow = len(currentMsg)
	}

	visibleText := currentMsg[:charsToShow]

	// Blinking cursor
	cursor := ""
	if tick%8 < 4 && charsToShow < len(currentMsg) {
		cursor = "▋"
	} else if charsToShow >= len(currentMsg) && currentMsg != "Ready to go!" {
		dots := strings.Repeat(".", (tick/4)%4)
		visibleText += dots
	}

	// Add checkmark for "Ready to go!"
	if currentMsg == "Ready to go!" && charsToShow >= len(currentMsg) {
		return lipgloss.NewStyle().
			Foreground(accentBright).
			Bold(true).
			Render("✓ " + visibleText)
	}

	return textStyle.Render(visibleText + cursor)
}

func renderGradientBar(current, max, barWidth int) string {
	percent := float64(current) / float64(max)
	filled := int(percent * float64(barWidth))

	if filled > barWidth {
		filled = barWidth
	}

	var bar strings.Builder

	// Gradient colors for the progress bar
	gradientColors := []string{
		"#064e3b", "#047857", "#059669", "#10b981",
		"#34d399", "#6ee7b7", "#a7f3d0", "#ecfdf5",
	}

	bar.WriteString("  ")

	for i := 0; i < barWidth; i++ {
		if i < filled {
			gradientPos := (i * len(gradientColors)) / barWidth
			if gradientPos >= len(gradientColors) {
				gradientPos = len(gradientColors) - 1
			}
			color := gradientColors[gradientPos]

			// Shimmer effect
			if i >= filled-2 && current < max {
				shimmerPhase := (current * 3) % 3
				if (i - filled + 3) == shimmerPhase {
					color = "#ffffff"
				}
			}

			bar.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(color)).
				Render("█"))
		} else {
			bar.WriteString(progressBgStyle.Render("░"))
		}
	}

	bar.WriteString("  ")

	// Percentage
	percentText := fmt.Sprintf(" %d%%", int(percent*100))
	if percent >= 1.0 {
		bar.WriteString(lipgloss.NewStyle().
			Foreground(accentBright).
			Bold(true).
			Render("✓"))
	} else {
		bar.WriteString(subtleStyle.Render(percentText))
	}

	return bar.String()
}

// RunIntro runs the intro animation and returns true if it completed normally
func RunIntro() bool {
	p := tea.NewProgram(NewIntroModel())
	model, err := p.Run()
	if err != nil {
		return false
	}

	introModel, ok := model.(IntroModel)
	if !ok {
		return false
	}

	return introModel.Finished
}
