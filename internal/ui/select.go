package ui

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
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

	items := make([]decisionItem, 0, len(request.Options))
	for _, option := range request.Options {
		items = append(items, decisionItem{option: option})
	}

	model := newDecisionModel(request.Name, items)
	program := tea.NewProgram(model, tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout))
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
		OptionID:   selected.ID,
		Value:      selected.Value,
		Confidence: selected.Confidence,
		Reason:     "Selected interactively from the evidence-backed candidate set.",
	}, nil
}

func interactiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

type decisionItem struct {
	option intelligence.DecisionOption
}

func (i decisionItem) FilterValue() string { return i.option.Value }
func (i decisionItem) Title() string       { return i.option.Value }
func (i decisionItem) Description() string {
	if len(i.option.Evidence) == 0 {
		return fmt.Sprintf("confidence %.0f%%", i.option.Confidence*100)
	}
	return i.option.Evidence[0].Detail
}

type decisionModel struct {
	list      list.Model
	selected  int
	cancelled bool
}

func newDecisionModel(title string, items []decisionItem) decisionModel {
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	delegate.Styles.NormalTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	delegate.Styles.NormalDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	l := list.New(itemsToListItems(items), delegate, 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetShowHelp(true)
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{}
	}
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{}
	}

	return decisionModel{list: l, selected: -1}
}

func itemsToListItems(items []decisionItem) []list.Item {
	out := make([]list.Item, len(items))
	for index := range items {
		out[index] = items[index]
	}
	return out
}

func (m decisionModel) Init() tea.Cmd {
	return nil
}

func (m decisionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.String() == "enter":
			m.selected = m.list.Index()
			return m, tea.Quit
		case key.Matches(msg, list.DefaultKeyMap().Quit):
			m.cancelled = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m decisionModel) View() string {
	return m.list.View()
}
