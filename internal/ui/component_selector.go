package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
)

var (
	glowBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#FF5F87")). // Glow pink badge
			Padding(0, 1)

	glowConfirmBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#00FF88")).
			Padding(0, 1)

	glowSubtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#767676"))

	glowBarActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5F87")).
			SetString(" │ ")

	glowBarInactive = lipgloss.NewStyle().
			SetString("   ")

	glowCheckOn = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF88"))

	glowCheckOff = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	glowTitleActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5F87"))

	glowTitleInactive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E6EDF3"))

	glowDetailActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C678DD"))

	glowDetailInactive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	glowTagActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF87AF"))

	glowTagInactive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#767676"))

	glowPortStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5C07B"))

	glowSectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E6EDF3"))

	glowForcedHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E5C07B"))

	glowHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))
)

type SelectableItem struct {
	ID        intelligence.TargetID
	Name      string
	IsService bool
	Badge     string
	Path      string
	Runtime   string
	Command   string
	PortInfo  string
	DependsOn []string
	Selected  bool
}

type selectorState int

const (
	stateSelecting selectorState = iota
	stateConfirming
)

type componentSelectorModel struct {
	model        intelligence.ProjectModel
	items        []SelectableItem
	cursor       int
	scrollOffset int
	state        selectorState
	confirmed    bool
	cancelled    bool
	width        int
	height       int
	resolved     intelligence.ResolvedSelection
	resolveErr   error
}

func newComponentSelectorModel(model intelligence.ProjectModel) componentSelectorModel {
	runnable := intelligence.RunnableTargets(model)
	items := make([]SelectableItem, 0, len(runnable))

	for _, t := range runnable {
		items = append(items, SelectableItem{
			ID:        t.ID,
			Name:      t.Name,
			IsService: t.Kind == intelligence.TargetKindService,
			Badge:     t.Runtime,
			Path:      t.Path,
			Runtime:   t.Runtime,
			Command:   t.Command,
			PortInfo:  t.PortInfo,
			DependsOn: t.DependsOn,
			Selected:  true, // Default selected
		})
	}

	return componentSelectorModel{
		model:        model,
		items:        items,
		cursor:       0,
		scrollOffset: 0,
		state:        stateSelecting,
		width:        80,
		height:       24,
	}
}

func (m componentSelectorModel) Init() tea.Cmd {
	return nil
}

func (m *componentSelectorModel) updateScroll() {
	itemHeight := 3
	availableLines := m.height - 8
	maxVisible := availableLines / itemHeight
	if maxVisible < 2 {
		maxVisible = 2
	}
	if maxVisible > len(m.items) {
		maxVisible = len(m.items)
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+maxVisible {
		m.scrollOffset = m.cursor - maxVisible + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	if maxOffset := len(m.items) - maxVisible; maxOffset > 0 && m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}

func (m componentSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateScroll()
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		switch m.state {
		case stateSelecting:
			switch key {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
					m.updateScroll()
				}
			case "down", "j":
				if m.cursor < len(m.items)-1 {
					m.cursor++
					m.updateScroll()
				}
			case " ", "x":
				if m.cursor >= 0 && m.cursor < len(m.items) {
					m.items[m.cursor].Selected = !m.items[m.cursor].Selected
				}
			case "a":
				hasUnselected := false
				for _, it := range m.items {
					if !it.Selected {
						hasUnselected = true
						break
					}
				}
				for i := range m.items {
					m.items[i].Selected = hasUnselected
				}
			case "enter":
				var selectedIDs []intelligence.TargetID
				for _, it := range m.items {
					if it.Selected {
						selectedIDs = append(selectedIDs, it.ID)
					}
				}
				if len(selectedIDs) == 0 {
					return m, nil
				}
				resolved, err := intelligence.ResolveSelectionTargets(m.model, selectedIDs)
				if err != nil {
					m.resolveErr = err
					return m, nil
				}
				m.resolveErr = nil
				m.resolved = resolved
				m.state = stateConfirming
				return m, nil
			case "esc", "q", "ctrl+c":
				m.cancelled = true
				return m, tea.Quit
			}

		case stateConfirming:
			switch key {
			case "enter", "y", "Y":
				m.confirmed = true
				return m, tea.Quit
			case "esc", "b", "B", "left", "backspace":
				m.state = stateSelecting
				return m, nil
			case "ctrl+c", "q":
				m.cancelled = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m componentSelectorModel) View() string {
	if m.state == stateConfirming {
		var b strings.Builder
		b.WriteString("\n")
		b.WriteString("  " + glowConfirmBadge.Render(" CONFIRM LAUNCH SELECTION ") + "\n\n")
		b.WriteString("  " + glowSubtitleStyle.Render("Review requested targets and forced dependencies before starting:") + "\n\n")

		b.WriteString("  " + glowSectionStyle.Render("Requested Launches:") + "\n\n")
		for _, req := range m.resolved.RequestedTargets {
			details := ""
			for _, it := range m.items {
				if it.ID == req {
					parts := []string{}
					if it.Command != "" {
						parts = append(parts, it.Command)
					}
					if it.PortInfo != "" {
						parts = append(parts, it.PortInfo)
					}
					if len(parts) > 0 {
						details = strings.Join(parts, "  •  ")
					}
					break
				}
			}
			b.WriteString(glowBarActive.String() + glowTitleActive.Render(req.String()) + "\n")
			if details != "" {
				b.WriteString(glowBarActive.String() + glowDetailActive.Render(details) + "\n")
			}
			b.WriteString("\n")
		}

		if len(m.resolved.ForcedDependencies) > 0 {
			b.WriteString("  " + glowForcedHeader.Render("Dependencies Forced by Choice:") + "\n\n")
			for id, reason := range m.resolved.ForcedDependencies {
				b.WriteString(glowBarActive.String() + glowPortStyle.Render(id.String()) + "\n")
				b.WriteString(glowBarActive.String() + glowDetailInactive.Render(reason) + "\n\n")
			}
		}

		b.WriteString("  " + glowHelpStyle.Render("enter confirm & start • esc back • q quit") + "\n")
		return b.String()
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + glowBadgeStyle.Render(" OCTO ") + " " + glowSectionStyle.Render("MULTI-TARGET SELECTOR") + "\n\n")

	compCount := 0
	svcCount := 0
	for _, it := range m.items {
		if it.IsService {
			svcCount++
		} else {
			compCount++
		}
	}
	summary := fmt.Sprintf("%d targets (%d components · %d services)", len(m.items), compCount, svcCount)
	b.WriteString("  " + glowSubtitleStyle.Render(summary) + "\n\n")

	if m.resolveErr != nil {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Bold(true).Render("Error: "+m.resolveErr.Error()) + "\n\n")
	}

	itemHeight := 3
	availableLines := m.height - 8
	maxVisible := availableLines / itemHeight
	if maxVisible < 2 {
		maxVisible = 2
	}
	if maxVisible > len(m.items) {
		maxVisible = len(m.items)
	}

	start := m.scrollOffset
	end := start + maxVisible
	if end > len(m.items) {
		end = len(m.items)
	}

	if start > 0 {
		b.WriteString("  " + glowDetailInactive.Render(fmt.Sprintf("▲ (+%d more above)", start)) + "\n\n")
	}

	for i := start; i < end; i++ {
		item := m.items[i]
		isActive := (i == m.cursor)

		bar := glowBarInactive.String()
		if isActive {
			bar = glowBarActive.String()
		}

		check := glowCheckOff.Render("[ ]")
		if item.Selected {
			check = glowCheckOn.Render("[✓]")
		}

		name := glowTitleInactive.Render(item.Name)
		tag := glowTagInactive.Render(item.Badge)
		if isActive {
			name = glowTitleActive.Render(item.Name)
			tag = glowTagActive.Render(item.Badge)
		}

		port := ""
		if item.PortInfo != "" {
			port = glowPortStyle.Render(item.PortInfo)
		}

		// Line 1: Bar + Check + Name + Tag + Port
		b.WriteString(fmt.Sprintf("%s%s %s  %s", bar, check, name, tag))
		if port != "" {
			b.WriteString("  " + port)
		}
		b.WriteString("\n")

		// Line 2: Bar + Details (path, command, deps)
		var detailParts []string
		if item.Path != "" && item.Path != "." {
			detailParts = append(detailParts, item.Path)
		}
		if item.Command != "" {
			detailParts = append(detailParts, item.Command)
		}
		if len(item.DependsOn) > 0 {
			detailParts = append(detailParts, "deps: "+strings.Join(item.DependsOn, ", "))
		}

		if len(detailParts) > 0 {
			detailStr := strings.Join(detailParts, "  •  ")
			if isActive {
				b.WriteString(bar + "    " + glowDetailActive.Render(detailStr) + "\n")
			} else {
				b.WriteString(bar + "    " + glowDetailInactive.Render(detailStr) + "\n")
			}
		}

		b.WriteString("\n")
	}

	if end < len(m.items) {
		b.WriteString("  " + glowDetailInactive.Render(fmt.Sprintf("▼ (+%d more below)", len(m.items)-end)) + "\n\n")
	}

	b.WriteString("  " + glowHelpStyle.Render("↑/↓ navigate • space toggle • a select all • enter confirm • q quit") + "\n")
	return b.String()
}

// RunTargetSelector launches the interactive selector and returns the resolved selection.
func RunTargetSelector(model intelligence.ProjectModel) (selection intelligence.ResolvedSelection, cancelled bool, err error) {
	if !InteractiveTerminal() {
		return intelligence.ResolvedSelection{}, false, fmt.Errorf("cannot run interactive target selector in non-interactive environment; specify --only <target> or --all.\nAvailable targets:\n%s", intelligence.FormatAvailableTargets(model))
	}

	p := tea.NewProgram(newComponentSelectorModel(model), tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return intelligence.ResolvedSelection{}, false, fmt.Errorf("component selector failed: %w", err)
	}

	m, ok := finalModel.(componentSelectorModel)
	if !ok || m.cancelled || !m.confirmed {
		return intelligence.ResolvedSelection{}, true, nil
	}

	return m.resolved, false, nil
}
