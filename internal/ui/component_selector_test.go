package ui

import (
	"strings"
	"testing"

	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
	tea "github.com/charmbracelet/bubbletea"
)

func TestComponentSelectorModelView(t *testing.T) {
	model := intelligence.ProjectModel{
		Components: []intelligence.Component{
			{
				Name:       "web",
				Language:   "Node",
				Framework:  "Next.js",
				Path:       "apps/web",
				RunCommand: "npm run dev",
				Port:       3000,
			},
			{
				Name:       "api",
				Language:   "Go",
				Path:       "services/api",
				RunCommand: "go run .",
				Port:       8080,
				DependsOn:  []string{"postgres"},
			},
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
	if !strings.Contains(view, "MULTI-TARGET SELECTOR") {
		t.Fatal("expected view to contain multi-target selector header")
	}
	if !strings.Contains(view, "web") || !strings.Contains(view, "api") || !strings.Contains(view, "postgres") {
		t.Fatal("expected view to contain all targets")
	}
	if !strings.Contains(view, "apps/web") || !strings.Contains(view, "services/api") {
		t.Fatal("expected view to contain paths")
	}
	if !strings.Contains(view, "npm run dev") || !strings.Contains(view, "go run .") {
		t.Fatal("expected view to contain commands")
	}
	if !strings.Contains(view, "→ port 3000") || !strings.Contains(view, "→ 5432:5432") {
		t.Fatal("expected view to contain port info")
	}
	if !strings.Contains(view, "deps: postgres") {
		t.Fatal("expected view to contain dependency info")
	}
}

func TestComponentSelectorModelToggle(t *testing.T) {
	model := intelligence.ProjectModel{
		Components: []intelligence.Component{
			{Name: "web", RunCommand: "npm run dev"},
			{Name: "api", RunCommand: "go run ."},
		},
	}

	selModel := newComponentSelectorModel(model)
	if !selModel.items[0].Selected || !selModel.items[1].Selected {
		t.Fatal("expected items initially selected")
	}

	// Toggle item 0 with space
	updated, _ := selModel.Update(tea.KeyMsg{Runes: []rune(" "), Type: tea.KeyRunes})
	m := updated.(componentSelectorModel)
	if m.items[0].Selected {
		t.Fatal("expected item 0 unselected after space")
	}
	if !m.items[1].Selected {
		t.Fatal("expected item 1 still selected")
	}

	// Toggle all with 'a'
	updated, _ = m.Update(tea.KeyMsg{Runes: []rune("a"), Type: tea.KeyRunes})
	m = updated.(componentSelectorModel)
	if !m.items[0].Selected || !m.items[1].Selected {
		t.Fatal("expected all items selected after pressing 'a'")
	}

	// Toggle all off with 'a'
	updated, _ = m.Update(tea.KeyMsg{Runes: []rune("a"), Type: tea.KeyRunes})
	m = updated.(componentSelectorModel)
	if m.items[0].Selected || m.items[1].Selected {
		t.Fatal("expected all items unselected after pressing 'a' again")
	}
}

func TestComponentSelectorNavigation(t *testing.T) {
	model := intelligence.ProjectModel{
		Components: []intelligence.Component{
			{Name: "web", RunCommand: "npm run dev"},
			{Name: "api", RunCommand: "go run ."},
			{Name: "worker", RunCommand: "python worker.py"},
		},
	}

	selModel := newComponentSelectorModel(model)
	if selModel.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", selModel.cursor)
	}

	// Move down
	updated, _ := selModel.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := updated.(componentSelectorModel)
	if m.cursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", m.cursor)
	}

	// Move down with 'j'
	updated, _ = m.Update(tea.KeyMsg{Runes: []rune("j"), Type: tea.KeyRunes})
	m = updated.(componentSelectorModel)
	if m.cursor != 2 {
		t.Fatalf("expected cursor at 2, got %d", m.cursor)
	}

	// Move down at boundary should stay at 2
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(componentSelectorModel)
	if m.cursor != 2 {
		t.Fatalf("expected cursor clamped at 2, got %d", m.cursor)
	}

	// Move up with 'k'
	updated, _ = m.Update(tea.KeyMsg{Runes: []rune("k"), Type: tea.KeyRunes})
	m = updated.(componentSelectorModel)
	if m.cursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", m.cursor)
	}
}

func TestComponentSelectorScrolling(t *testing.T) {
	var comps []intelligence.Component
	for i := 0; i < 10; i++ {
		comps = append(comps, intelligence.Component{
			Name:       string(rune('a' + i)),
			RunCommand: "start",
		})
	}
	model := intelligence.ProjectModel{Components: comps}

	selModel := newComponentSelectorModel(model)
	// Simulate small window: height 13 -> maxVisible = 3
	updated, _ := selModel.Update(tea.WindowSizeMsg{Width: 80, Height: 13})
	m := updated.(componentSelectorModel)

	view := m.View()
	if !strings.Contains(view, "▼") {
		t.Fatal("expected scroll down indicator ▼ in initial view")
	}

	// Scroll cursor down past visible window
	for i := 0; i < 6; i++ {
		upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = upd.(componentSelectorModel)
	}

	view = m.View()
	if !strings.Contains(view, "▲") {
		t.Fatal("expected scroll up indicator ▲ after scrolling down")
	}
}

func TestComponentSelectorConfirmationFlow(t *testing.T) {
	model := intelligence.ProjectModel{
		Name: "test-app",
		Components: []intelligence.Component{
			{Name: "api", Path: "services/api", RunCommand: "go run .", DependsOn: []string{"postgres"}},
		},
		Services: []intelligence.Service{
			{Name: "postgres", Image: "postgres:16", Ports: []string{"5432:5432"}},
		},
	}

	selModel := newComponentSelectorModel(model)

	// Unselect postgres (item 1) so only api (item 0) is requested
	selModel.items[1].Selected = false

	// Press Enter to review selection
	updated, _ := selModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := updated.(componentSelectorModel)

	if m.state != stateConfirming {
		t.Fatalf("expected stateConfirming, got %v", m.state)
	}

	view := m.View()
	if !strings.Contains(view, "CONFIRM LAUNCH SELECTION") {
		t.Fatal("expected confirmation header in view")
	}
	if !strings.Contains(view, "Requested Launches:") || !strings.Contains(view, "component:api") {
		t.Fatal("expected requested launches to show component:api")
	}
	if !strings.Contains(view, "Dependencies Forced by Choice:") || !strings.Contains(view, "service:postgres") {
		t.Fatal("expected forced dependencies to show service:postgres")
	}
	if !strings.Contains(view, "required by component:api") {
		t.Fatal("expected reason for forced dependency")
	}

	// Press Esc to return to selection
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(componentSelectorModel)
	if m.state != stateSelecting {
		t.Fatalf("expected stateSelecting after Esc, got %v", m.state)
	}

	// Press Enter again to go back to confirming
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(componentSelectorModel)

	// Press Enter to confirm
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(componentSelectorModel)
	if !m.confirmed {
		t.Fatal("expected confirmed to be true")
	}
	if cmd == nil {
		t.Fatal("expected tea.Quit command upon confirmation")
	}
}

func TestComponentSelectorCtrlC(t *testing.T) {
	model := intelligence.ProjectModel{
		Components: []intelligence.Component{
			{Name: "web", RunCommand: "npm run dev"},
			{Name: "api", RunCommand: "go run ."},
		},
	}

	selModel := newComponentSelectorModel(model)
	updated, cmd := selModel.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m := updated.(componentSelectorModel)
	if !m.cancelled {
		t.Fatal("expected cancelled to be true on Ctrl+C")
	}
	if cmd == nil {
		t.Fatal("expected tea.Quit command on Ctrl+C")
	}
}

func TestComponentSelectorEmptySelection(t *testing.T) {
	model := intelligence.ProjectModel{
		Components: []intelligence.Component{
			{Name: "web", RunCommand: "npm run dev"},
		},
	}

	selModel := newComponentSelectorModel(model)
	// Unselect the only item
	selModel.items[0].Selected = false

	// Press Enter
	updated, _ := selModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := updated.(componentSelectorModel)
	if m.state != stateSelecting {
		t.Fatal("expected to stay in stateSelecting when no items are selected")
	}
}

func TestComponentSelectorTerminalResizeSmallAndLarge(t *testing.T) {
	var comps []intelligence.Component
	for i := 0; i < 8; i++ {
		comps = append(comps, intelligence.Component{
			Name:       string(rune('a' + i)),
			RunCommand: "run",
		})
	}
	model := intelligence.ProjectModel{Components: comps}
	selModel := newComponentSelectorModel(model)

	// Test very small terminal (height 6): must clamp to min visible (3) without panic
	updated, _ := selModel.Update(tea.WindowSizeMsg{Width: 40, Height: 6})
	mSmall := updated.(componentSelectorModel)
	viewSmall := mSmall.View()
	if !strings.Contains(viewSmall, "MULTI-TARGET SELECTOR") {
		t.Fatal("expected header in small terminal view")
	}
	if !strings.Contains(viewSmall, "▼ (+6 more below)") {
		t.Fatalf("expected 6 items hidden below in small view, got:\n%s", viewSmall)
	}

	// Test large terminal (height 40): all 8 items fit, no scroll arrows
	updated, _ = selModel.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	mLarge := updated.(componentSelectorModel)
	viewLarge := mLarge.View()
	if strings.Contains(viewLarge, "▲") || strings.Contains(viewLarge, "▼") {
		t.Fatal("expected no scroll indicators when all items fit in large terminal")
	}
}

func TestComponentSelectorScrollBoundaryBackAndForth(t *testing.T) {
	var comps []intelligence.Component
	for i := 0; i < 20; i++ {
		comps = append(comps, intelligence.Component{
			Name:       string(rune('a'+(i/26))) + string(rune('a'+(i%26))),
			RunCommand: "start",
		})
	}
	model := intelligence.ProjectModel{Components: comps}
	selModel := newComponentSelectorModel(model)

	// Set window height 23 -> maxVisible = 5
	updated, _ := selModel.Update(tea.WindowSizeMsg{Width: 80, Height: 23})
	m := updated.(componentSelectorModel)

	// At top (cursor 0): no ▲, has ▼ (+15 more below)
	view := m.View()
	if strings.Contains(view, "▲") {
		t.Fatal("expected no scroll up arrow at top")
	}
	if !strings.Contains(view, "▼ (+15 more below)") {
		t.Fatalf("expected ▼ (+15 more below) at top, got:\n%s", view)
	}

	// Scroll all the way to the bottom (19 steps down)
	for i := 0; i < 19; i++ {
		upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = upd.(componentSelectorModel)
	}
	if m.cursor != 19 {
		t.Fatalf("expected cursor at 19, got %d", m.cursor)
	}
	if m.scrollOffset != 15 {
		t.Fatalf("expected scrollOffset at 15, got %d", m.scrollOffset)
	}

	// At bottom: has ▲ (+15 more above), no ▼
	view = m.View()
	if !strings.Contains(view, "▲ (+15 more above)") {
		t.Fatalf("expected ▲ (+15 more above) at bottom, got:\n%s", view)
	}
	if strings.Contains(view, "▼") {
		t.Fatal("expected no scroll down arrow at bottom")
	}

	// Scroll back up to top (19 steps up)
	for i := 0; i < 19; i++ {
		upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = upd.(componentSelectorModel)
	}
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}
	if m.scrollOffset != 0 {
		t.Fatalf("expected scrollOffset at 0, got %d", m.scrollOffset)
	}

	// Back at top: no ▲, has ▼ (+15 more below)
	view = m.View()
	if strings.Contains(view, "▲") {
		t.Fatal("expected no scroll up arrow when back at top")
	}
	if !strings.Contains(view, "▼ (+15 more below)") {
		t.Fatalf("expected ▼ (+15 more below) when back at top, got:\n%s", view)
	}
}

func TestComponentSelectorResizeMaintainsCursorVisibility(t *testing.T) {
	var comps []intelligence.Component
	for i := 0; i < 15; i++ {
		comps = append(comps, intelligence.Component{
			Name:       string(rune('a' + i)),
			RunCommand: "start",
		})
	}
	model := intelligence.ProjectModel{Components: comps}
	selModel := newComponentSelectorModel(model)

	// Move cursor down to item 10 in default large window
	for i := 0; i < 10; i++ {
		upd, _ := selModel.Update(tea.KeyMsg{Type: tea.KeyDown})
		selModel = upd.(componentSelectorModel)
	}
	if selModel.cursor != 10 {
		t.Fatalf("expected cursor at 10, got %d", selModel.cursor)
	}

	// Now resize terminal to small height (14 -> maxVisible = 4)
	updated, _ := selModel.Update(tea.WindowSizeMsg{Width: 80, Height: 14})
	m := updated.(componentSelectorModel)

	// Cursor 10 must be visible: scrollOffset <= 10 < scrollOffset + 4
	if m.cursor < m.scrollOffset || m.cursor >= m.scrollOffset+4 {
		t.Fatalf("cursor %d not visible with scrollOffset %d and maxVisible 4", m.cursor, m.scrollOffset)
	}
}
