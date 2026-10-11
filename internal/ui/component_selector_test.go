package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
)

func TestComponentSelectorModelView(t *testing.T) {
	model := intelligence.ProjectModel{
		Components: []intelligence.Component{
			{Name: "web", Language: "Node", Framework: "Next.js", Path: "apps/web", Port: 3000},
			{Name: "api", Language: "Go", Path: "services/api", Port: 8080, DependsOn: []string{"postgres"}},
		},
		Services: []intelligence.Service{
			{Name: "postgres", Image: "postgres:16", Ports: []string{"5432:5432"}},
		},
	}

	selModel := newComponentSelectorModel(model)
	if len(selModel.items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(selModel.items))
	}

	view := selModel.View()
	if !strings.Contains(view, "OCTO MULTI-COMPONENT SELECTOR") {
		t.Fatal("expected view to contain header")
	}
	if !strings.Contains(view, "web") || !strings.Contains(view, "api") || !strings.Contains(view, "postgres") {
		t.Fatal("expected view to contain all components and services")
	}
}

func TestComponentSelectorModelToggle(t *testing.T) {
	model := intelligence.ProjectModel{
		Components: []intelligence.Component{
			{Name: "web", Language: "Node"},
			{Name: "api", Language: "Go"},
		},
	}

	selModel := newComponentSelectorModel(model)
	if !selModel.items[0].Selected || !selModel.items[1].Selected {
		t.Fatal("expected items initially selected")
	}

	// Toggle all
	updated, _ := selModel.Update(tea.KeyMsg{Runes: []rune("a"), Type: tea.KeyRunes})
	m := updated.(componentSelectorModel)
	// Now all should be unselected
	if m.items[0].Selected || m.items[1].Selected {
		t.Fatal("expected all items unselected after toggle")
	}
}
