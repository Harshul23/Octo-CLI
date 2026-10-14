package ui

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	// Section title styles
	helpSectionMainTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0f766e", Dark: "#2dd4bf"}) // Vibrant Teal/Mint

	helpSectionOtherTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#4338ca", Dark: "#818cf8"}) // Sleek Indigo

	helpSectionUsageTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#6b21a8", Dark: "#c084fc"}) // Modern Purple

	helpSectionFlagsTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0369a1", Dark: "#38bdf8"}) // Crisp Sky Blue

	helpSectionDefaultTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#fbbf24"}) // Warm Amber

	// Command name styles
	helpMainCommandStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0d9488", Dark: "#2dd4bf"})

	helpOtherCommandStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#60a5fa"})

	helpAdditionalCommandStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#94a3b8"})

	// Flag styles
	helpFlagTriggerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0284c7", Dark: "#38bdf8"})

	helpFlagTypeStyle = lipgloss.NewStyle().
				Italic(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#94a3b8"})

	// Text & description styles
	helpDescriptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#334155", Dark: "#cbd5e1"})

	helpMutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#94a3b8"})

	helpUsageCommandStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#1e293b", Dark: "#f8fafc"})

	helpUsageArgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#94a3b8"})

	helpFooterCmdStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#6b21a8", Dark: "#c084fc"})
)

var (
	commandLineRegex = regexp.MustCompile(`^(\s{2,})([a-zA-Z0-9_\-]+)(\s{2,})(.*)$`)
	usageLineRegex   = regexp.MustCompile(`^(\s{2,})([a-zA-Z0-9_\-]+)(\s+)(.*)$`)
)

// SetupColoredHelp configures custom colored help for the given command and all subcommands.
func SetupColoredHelp(cmd *cobra.Command) {
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		PrintColoredHelp(c)
	})
}

// PrintColoredHelp formats and outputs colorized help to the command's designated output.
func PrintColoredHelp(c *cobra.Command) {
	var raw bytes.Buffer
	usage := c.Long
	if usage == "" {
		usage = c.Short
	}
	usage = strings.TrimRight(usage, " \t\r\n")
	if usage != "" {
		raw.WriteString(usage)
		raw.WriteString("\n\n")
	}
	if c.Runnable() || c.HasSubCommands() {
		raw.WriteString(c.UsageString())
	}

	useColor := shouldColorize()
	if !useColor {
		fmt.Fprint(c.OutOrStdout(), raw.String())
		return
	}

	if lipgloss.ColorProfile() == termenv.Ascii {
		lipgloss.SetColorProfile(termenv.TrueColor)
	}

	colored := ColorizeHelp(raw.String())
	fmt.Fprint(c.OutOrStdout(), colored)
}

func shouldColorize() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" || os.Getenv("CLICOLOR_FORCE") != "" {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// ColorizeHelp parses raw Cobra help output and applies curated Lipgloss styles.
func ColorizeHelp(raw string) string {
	lines := strings.Split(raw, "\n")
	var result strings.Builder
	currentSection := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Empty lines reset section if no items followed
		if trimmed == "" {
			result.WriteString("\n")
			continue
		}

		// Detect section header (lines ending in ":" with no leading whitespace)
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(line, ":") {
			currentSection = strings.TrimSuffix(line, ":")
			result.WriteString(styleSectionTitle(currentSection))
			result.WriteString("\n")
			continue
		}

		// Footer: Use "octo [command] --help" for more information about a command.
		if strings.HasPrefix(line, "Use \"") && strings.Contains(line, "--help\"") {
			result.WriteString(styleFooter(line))
			if i < len(lines)-1 {
				result.WriteString("\n")
			}
			continue
		}

		// Section item styling
		switch currentSection {
		case "Main Commands":
			if styled, ok := styleCommandItem(line, helpMainCommandStyle); ok {
				result.WriteString(styled)
			} else {
				result.WriteString(helpDescriptionStyle.Render(line))
			}
		case "Other Commands":
			if styled, ok := styleCommandItem(line, helpOtherCommandStyle); ok {
				result.WriteString(styled)
			} else {
				result.WriteString(helpDescriptionStyle.Render(line))
			}
		case "Additional Commands":
			if styled, ok := styleCommandItem(line, helpAdditionalCommandStyle); ok {
				result.WriteString(styled)
			} else {
				result.WriteString(helpMutedStyle.Render(line))
			}
		case "Flags", "Global Flags":
			result.WriteString(styleFlagLine(line))
		case "Usage":
			result.WriteString(styleUsageLine(line))
		case "Aliases":
			result.WriteString(styleAliasesLine(line))
		default:
			result.WriteString(helpDescriptionStyle.Render(line))
		}

		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func styleSectionTitle(section string) string {
	switch section {
	case "Main Commands":
		return helpSectionMainTitle.Render(section + ":")
	case "Other Commands":
		return helpSectionOtherTitle.Render(section + ":")
	case "Usage":
		return helpSectionUsageTitle.Render(section + ":")
	case "Flags", "Global Flags":
		return helpSectionFlagsTitle.Render(section + ":")
	case "Aliases":
		return helpSectionDefaultTitle.Render(section + ":")
	default:
		return helpSectionDefaultTitle.Render(section + ":")
	}
}

func styleCommandItem(line string, cmdStyle lipgloss.Style) (string, bool) {
	matches := commandLineRegex.FindStringSubmatch(line)
	if len(matches) != 5 {
		return "", false
	}
	indent := matches[1]
	cmdName := matches[2]
	spacing := matches[3]
	desc := matches[4]

	return indent + cmdStyle.Render(cmdName) + spacing + helpDescriptionStyle.Render(desc), true
}

func styleFlagLine(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indentLen := len(line) - len(trimmed)
	indent := line[:indentLen]

	sepIdx := strings.Index(trimmed, "  ")
	if sepIdx == -1 {
		return line
	}

	spec := strings.TrimRight(trimmed[:sepIdx], " ")
	trailing := trimmed[sepIdx:]
	sepLen := len(trailing) - len(strings.TrimLeft(trailing, " "))
	spaces := trailing[:sepLen]
	desc := strings.TrimLeft(trailing, " ")

	// Check if spec has a type suffix (e.g. "-o, --only stringArray")
	lastSpace := strings.LastIndex(spec, " ")
	if lastSpace != -1 {
		potentialType := spec[lastSpace+1:]
		if !strings.HasPrefix(potentialType, "-") {
			triggers := spec[:lastSpace]
			return indent + helpFlagTriggerStyle.Render(triggers) + " " + helpFlagTypeStyle.Render(potentialType) + spaces + helpDescriptionStyle.Render(desc)
		}
	}

	return indent + helpFlagTriggerStyle.Render(spec) + spaces + helpDescriptionStyle.Render(desc)
}

func styleUsageLine(line string) string {
	matches := usageLineRegex.FindStringSubmatch(line)
	if len(matches) == 5 {
		indent := matches[1]
		cmd := matches[2]
		spacing := matches[3]
		args := matches[4]
		return indent + helpUsageCommandStyle.Render(cmd) + spacing + helpUsageArgStyle.Render(args)
	}
	return helpUsageCommandStyle.Render(line)
}

func styleAliasesLine(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indentLen := len(line) - len(trimmed)
	indent := line[:indentLen]
	return indent + helpDefaultCmdStyle().Render(trimmed)
}

func helpDefaultCmdStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#fbbf24"})
}

func styleFooter(line string) string {
	// Pattern: Use "octo [command] --help" for more information about a command.
	startIdx := strings.Index(line, "\"")
	endIdx := strings.LastIndex(line, "\"")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		prefix := line[:startIdx]
		cmd := line[startIdx+1 : endIdx]
		suffix := line[endIdx+1:]
		return helpMutedStyle.Render(prefix) + "\"" + helpFooterCmdStyle.Render(cmd) + "\"" + helpMutedStyle.Render(suffix)
	}
	return helpMutedStyle.Render(line)
}
