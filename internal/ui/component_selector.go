package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Harshul23/Octo-CLI/internal/intelligence"
)

var (
	glowTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00F5FF")).
			Background(lipgloss.Color("#180B2E")).
			Padding(0, 2).
			MarginBottom(1)

	glowSubtitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#B388FF")).
				MarginBottom(1)

	glowBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7928CA")).
			Padding(1, 2).
			MarginBottom(1)

	glowCursorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00F5FF"))

	glowCheckOn = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF88"))

	glowCheckOff = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	glowNameActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	glowNameInactive = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CCCCCC"))

	glowBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#6200EA")).
			Padding(0, 1)

	glowServiceBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#001F3F")).
				Background(lipgloss.Color("#00F5FF")).
				Padding(0, 1)

	glowDetailStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#777777"))

	glowPortStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB703"))

	glowHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			MarginTop(1)
)

type SelectableItem struct {
	ID        string
	Name      string
	IsService bool
	Badge     string
	Path      string
	PortInfo  string
	DependsOn []string
	Selected  bool
}

type componentSelectorModel struct {
	items     []SelectableItem
	cursor    int
	confirmed bool
	cancelled bool
	width     int
	height    int
}

func newComponentSelectorModel(model intelligence.ProjectModel) componentSelectorModel {
	var items []SelectableItem

	// Add components
	for _, comp := range model.Components {
		badgeParts := []string{}
		if comp.Language != "" {
			badgeParts = append(badgeParts, strings.ToUpper(comp.Language))
		}
		if comp.Framework != "" {
			badgeParts = append(badgeParts, strings.ToUpper(comp.Framework))
		}
		badge := strings.Join(badgeParts, " / ")
		if badge == "" {
			badge = "COMPONENT"
		}

		portStr := ""
		if comp.Port > 0 {
			portStr = fmt.Sprintf("→ port %d", comp.Port)
		}

		items = append(items, SelectableItem{
			ID:        "comp:" + comp.Name,
			Name:      comp.Name,
			IsService: false,
			Badge:     badge,
			Path:      comp.Path,
			PortInfo:  portStr,
			DependsOn: comp.DependsOn,
			Selected:  true, // Default selected
		})
	}

	// Add compose services
	for _, svc := range model.Services {
		portStr := ""
		if len(svc.Ports) > 0 {
			portStr = "→ " + strings.Join(svc.Ports, ", ")
		}
		items = append(items, SelectableItem{
			ID:        "svc:" + svc.Name,
			Name:      svc.Name,
			IsService: true,
			Badge:     "SERVICE / DOCKER",
			Path:      svc.Image,
			PortInfo:  portStr,
			DependsOn: svc.DependsOn,
			Selected:  true, // Default selected
		})
	}

	return componentSelectorModel{
		items:  items,
		cursor: 0,
		width:  80,
		height: 24,
	}
}

func (m componentSelectorModel) Init() tea.Cmd {
	return nil
}

func (m componentSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case " ", "x":
			if m.cursor >= 0 && m.cursor < len(m.items) {
				m.items[m.cursor].Selected = !m.items[m.cursor].Selected
			}
		case "a":
			// Toggle all: if any unselected, select all; else unselect all
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
			// Must have at least one selected
			anySelected := false
			for _, it := range m.items {
				if it.Selected {
					anySelected = true
					break
				}
			}
			if anySelected {
				m.confirmed = true
				return m, tea.Quit
			}
		case "esc", "q", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m componentSelectorModel) View() string {
	var b strings.Builder

	title := glowTitleStyle.Render("⚡ OCTO MULTI-COMPONENT SELECTOR")
	sub := glowSubtitleStyle.Render("Navigate and choose components & services to launch:")
	b.WriteString(title + "\n" + sub + "\n\n")

	var rows strings.Builder
	for i, item := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = glowCursorStyle.Render("▶ ")
		}

		check := glowCheckOff.Render("[○]")
		if item.Selected {
			check = glowCheckOn.Render("[◉]")
		}

		badge := glowBadgeStyle.Render(item.Badge)
		if item.IsService {
			badge = glowServiceBadge.Render(item.Badge)
		}

		name := glowNameInactive.Render(item.Name)
		if i == m.cursor {
			name = glowNameActive.Render(item.Name)
		}

		details := ""
		if item.Path != "" && item.Path != "." {
			details = glowDetailStyle.Render(item.Path)
		}
		if item.PortInfo != "" {
			if details != "" {
				details += "  "
			}
			details += glowPortStyle.Render(item.PortInfo)
		}

		row := fmt.Sprintf("%s%s %s  %s  %s", cursor, check, name, badge, details)
		rows.WriteString(row + "\n")
	}

	b.WriteString(glowBoxStyle.Render(rows.String()))
	b.WriteString("\n" + glowHelpStyle.Render("↑/↓: navigate • space: toggle • a: toggle all • enter: run selected • q: cancel"))

	return b.String()
}

// RunComponentSelector launches the interactive selector and returns the chosen components and services.
func RunComponentSelector(model intelligence.ProjectModel) (selectedComponents []string, selectedServices []string, autoIncluded []string, cancelled bool, err error) {
	if len(model.Components)+len(model.Services) <= 1 || !InteractiveTerminal() {
		// Single component or non-interactive: run all by default
		for _, c := range model.Components {
			selectedComponents = append(selectedComponents, c.Name)
		}
		for _, s := range model.Services {
			selectedServices = append(selectedServices, s.Name)
		}
		return selectedComponents, selectedServices, nil, false, nil
	}

	selModel := newComponentSelectorModel(model)
	p := tea.NewProgram(
		selModel,
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
		tea.WithAltScreen(),
	)

	finalModel, err := p.Run()
	if err != nil {
		return nil, nil, nil, false, err
	}

	res, ok := finalModel.(componentSelectorModel)
	if !ok || res.cancelled {
		return nil, nil, nil, true, nil
	}

	selectedSet := make(map[string]bool)
	for _, it := range res.items {
		if it.Selected {
			selectedSet[it.Name] = true
			if it.IsService {
				selectedServices = append(selectedServices, it.Name)
			} else {
				selectedComponents = append(selectedComponents, it.Name)
			}
		}
	}

	// Auto-include upstream dependencies of selected components
	for _, comp := range model.Components {
		if !selectedSet[comp.Name] {
			continue
		}
		for _, dep := range comp.DependsOn {
			if !selectedSet[dep] {
				// Find whether dep is service or component
				isSvc := false
				for _, s := range model.Services {
					if s.Name == dep {
						isSvc = true
						break
					}
				}
				if isSvc {
					selectedServices = append(selectedServices, dep)
				} else {
					selectedComponents = append(selectedComponents, dep)
				}
				selectedSet[dep] = true
				autoIncluded = append(autoIncluded, dep)
			}
		}
	}

	return selectedComponents, selectedServices, autoIncluded, false, nil
}
