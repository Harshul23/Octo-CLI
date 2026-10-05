package ui

import "github.com/charmbracelet/lipgloss"

var (
	Accent = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	ErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	WarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	InfoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	CommandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	PathStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	SelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	HeadingStyle = lipgloss.NewStyle().Bold(true)
)

func SuccessLine(message string) string {
	return SuccessStyle.Render("✓") + " " + message
}

func ErrorLine(message string) string {
	return ErrorStyle.Render("✗") + " " + message
}

func WarningLine(message string) string {
	return WarningStyle.Render("!") + " " + message
}

func InfoLine(message string) string {
	return InfoStyle.Render("•") + " " + message
}

func Command(command string) string {
	return CommandStyle.Render(command)
}

func Path(path string) string {
	return PathStyle.Render(path)
}
