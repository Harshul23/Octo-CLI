package ui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/harshul/octo-cli/internal/intelligence"
	"golang.org/x/term"
)

type InteractiveDecisionProvider struct{}

func (InteractiveDecisionProvider) Decide(ctx context.Context, request intelligence.DecisionRequest) (intelligence.DecisionResult, error) {
	if err := ctx.Err(); err != nil {
		return intelligence.DecisionResult{}, err
	}
	if len(request.Options) == 0 {
		return intelligence.DecisionResult{}, fmt.Errorf("decision %q has no options", request.Name)
	}
	if len(request.Options) == 1 || !interactiveTerminal() {
		return (intelligence.DeterministicDecisionProvider{}).Decide(ctx, request)
	}

	items := make([]decisionItem, len(request.Options))
	for i, option := range request.Options {
		items[i] = decisionItem{option: option}
	}

	model := newDecisionModel(request.Name, items)
	program := tea.NewProgram(
		model,
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
		tea.WithAltScreen(),
	)
	finalModel, err := program.Run()
	if err != nil {
		return intelligence.DecisionResult{}, fmt.Errorf("interactive decision failed: %w", err)
	}

	result, ok := finalModel.(decisionModel)
	if !ok || result.cancelled {
		return intelligence.DecisionResult{}, fmt.Errorf("decision %q cancelled", request.Name)
	}
	if result.selected < 0 || result.selected >= len(request.Options) {
		return intelligence.DecisionResult{}, fmt.Errorf("decision %q returned an invalid selection", request.Name)
	}

	selected := request.Options[result.selected]
	return intelligence.DecisionResult{
		OptionID: selected.ID,
		Value: selected.Value,
		Confidence: selected.Confidence,
		Reason: "Selected interactively from the evidence-backed candidate set.",
	}, nil
}

func interactiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

type decisionItem struct {
	option intelligence.DecisionOption
}

type decisionModel struct {
	title     string
	items     []decisionItem
	cursor    int
	selected  int
	cancelled bool
	width     int
	height    int
}

func newDecisionModel(title string, items []decisionItem) decisionModel {
	return decisionModel{title: title, items: items, selected: -1, width: 80, height: 24}
}

func (m decisionModel) Init() tea.Cmd {
	return nil
}

func (m decisionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = m.cursor
			return m, tea.Quit
		case "esc", "q", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		default:
			if n, err := strconv.Atoi(msg.String()); err == nil && n >= 1 && n <= len(m.items) {
				m.selected = n - 1
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m decisionModel) View() string {
	var b strings.Builder
	width := m.width
	if width < 20 {
		width = 20
	}

	b.WriteString(decisionTitleStyle.Render("? " + m.title))
	b.WriteString("\n")
	b.WriteString(decisionMutedStyle.Render("  Choose a verified execution candidate."))
	b.WriteString("\n\n")

	visibleItems := m.height - 6
	if visibleItems < 5 {
		visibleItems = 5
	}
	start := m.cursor - visibleItems/2
	if start < 0 {
		start = 0
	}
	if start+visibleItems > len(m.items) {
		start = len(m.items) - visibleItems
		if start < 0 {
			start = 0
		}
	}
	end := start + visibleItems
	if end > len(m.items) {
		end = len(m.items)
	}

	if start > 0 {
		b.WriteString(decisionMutedStyle.Render("  ↑ more candidates"))
		b.WriteString("\\n")
	}

	for i := start; i < end; i++ {
		cursor := "  "
		style := decisionNormalStyle
		if i == m.cursor {
			cursor = decisionSelectedStyle.Render("❯ ")
			style = decisionSelectedStyle
		}

		number := decisionNumberStyle.Render(fmt.Sprintf("%d", i+1))
		confidence := decisionMutedStyle.Render(fmt.Sprintf("  %3.0f%%", item.option.Confidence*100))
		b.WriteString(cursor + number + "  " + style.Render(item.option.Value) + confidence)
		b.WriteString("\n")

		if i == m.cursor && len(item.option.Evidence) > 0 {
			evidence := "       " + item.option.Evidence[0].Detail
			evidenceWidth := width - 7
			if evidenceWidth < 12 {
				evidenceWidth = 12
			}
			wrapped := lipgloss.NewStyle().Width(evidenceWidth).Render(evidence)
			b.WriteString(decisionMutedStyle.Render(wrapped))
			b.WriteString("\n")
		}
	}

	if end < len(m.items) {
		b.WriteString(decisionMutedStyle.Render("  ↓ more candidates"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(decisionMutedStyle.Render("  ↑ ↓ navigate • 1-"+strconv.Itoa(len(m.items))+" select • enter confirm • esc cancel"))
	return b.String()
}

var (
	decisionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#5B21B6", Dark: "#C4B5FD"})
	decisionSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"})
	decisionNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#222222", Dark: "#E5E7EB"})
	decisionNumberStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"})
	decisionMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"})
)
