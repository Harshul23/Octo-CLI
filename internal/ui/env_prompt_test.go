package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewMissingEnvModelInitialState(t *testing.T) {
	missing := []string{"DATABASE_URL", "API_KEY", "DATABASE_URL"}
	model := newMissingEnvModel(".", missing, "# template")

	// Verify deduplication
	if len(model.missingVars) != 2 {
		t.Fatalf("expected 2 deduped vars, got %d", len(model.missingVars))
	}
	if model.missingVars[0] != "DATABASE_URL" || model.missingVars[1] != "API_KEY" {
		t.Fatalf("unexpected missingVars: %v", model.missingVars)
	}

	// Verify initial state is "enter env" state (input focused)
	if model.activeFocus != focusInput {
		t.Fatalf("expected activeFocus to be focusInput, got %v", model.activeFocus)
	}
	if !model.textInput.Focused() {
		t.Fatal("expected textInput to be focused by default")
	}
	if model.currentIndex != 0 {
		t.Fatalf("expected currentIndex 0, got %d", model.currentIndex)
	}
}

func TestMissingEnvModelNavigation(t *testing.T) {
	missing := []string{"DATABASE_URL", "API_KEY"}
	m := newMissingEnvModel(".", missing, "")

	// Initial focus is input
	if m.activeFocus != focusInput {
		t.Fatalf("initial focus want %v, got %v", focusInput, m.activeFocus)
	}

	// Press Tab -> switches to focusSkip
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(MissingEnvModel)
	if m.activeFocus != focusSkip {
		t.Fatalf("after tab focus want %v, got %v", focusSkip, m.activeFocus)
	}
	if m.textInput.Focused() {
		t.Fatal("textInput should be blurred when focus is on skip")
	}

	// Press Tab again -> switches to focusSkipAll
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(MissingEnvModel)
	if m.activeFocus != focusSkipAll {
		t.Fatalf("after second tab focus want %v, got %v", focusSkipAll, m.activeFocus)
	}

	// Press Tab again -> cycles back to focusInput
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(MissingEnvModel)
	if m.activeFocus != focusInput {
		t.Fatalf("after third tab focus want %v, got %v", focusInput, m.activeFocus)
	}
	if !m.textInput.Focused() {
		t.Fatal("textInput should be focused again")
	}

	// Press Shift+Tab -> backward to focusSkipAll
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(MissingEnvModel)
	if m.activeFocus != focusSkipAll {
		t.Fatalf("after shift+tab focus want %v, got %v", focusSkipAll, m.activeFocus)
	}
}

func TestMissingEnvModelEnteringValues(t *testing.T) {
	missing := []string{"PORT", "SECRET"}
	m := newMissingEnvModel(".", missing, "")

	// Enter value for PORT
	m.textInput.SetValue("8080")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(MissingEnvModel)

	if m.enteredValues["PORT"] != "8080" {
		t.Fatalf("expected PORT=8080, got %q", m.enteredValues["PORT"])
	}
	if m.currentIndex != 1 {
		t.Fatalf("expected currentIndex 1 for second var, got %d", m.currentIndex)
	}
	if m.activeFocus != focusInput {
		t.Fatalf("expected focus reset to focusInput for next var, got %v", m.activeFocus)
	}
	if m.textInput.Value() != "" {
		t.Fatalf("expected textInput reset, got %q", m.textInput.Value())
	}

	// Enter value for SECRET
	m.textInput.SetValue("supersecret")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(MissingEnvModel)

	if m.enteredValues["SECRET"] != "supersecret" {
		t.Fatalf("expected SECRET=supersecret, got %q", m.enteredValues["SECRET"])
	}
	if !m.confirmed {
		t.Fatal("expected model to be confirmed after completing all variables")
	}
	if cmd == nil {
		t.Fatal("expected quit cmd after last variable")
	}
}

func TestMissingEnvModelSkipControls(t *testing.T) {
	// Test Tab -> Enter skips variable
	missing := []string{"VAR1", "VAR2"}
	m := newMissingEnvModel(".", missing, "")

	// Switch to skip
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(MissingEnvModel)
	if m.activeFocus != focusSkip {
		t.Fatalf("expected focusSkip, got %v", m.activeFocus)
	}

	// Press Enter to skip VAR1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(MissingEnvModel)
	if _, ok := m.enteredValues["VAR1"]; ok {
		t.Fatal("VAR1 should not have an entered value")
	}
	if m.currentIndex != 1 {
		t.Fatalf("expected currentIndex 1, got %d", m.currentIndex)
	}

	// Test Esc skips all remaining
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(MissingEnvModel)
	if !m.skippedAll || !m.confirmed {
		t.Fatalf("expected skippedAll and confirmed on Esc, got skippedAll=%v confirmed=%v", m.skippedAll, m.confirmed)
	}
	if cmd == nil {
		t.Fatal("expected quit cmd on Esc")
	}
}

func TestMissingEnvModelColonSkip(t *testing.T) {
	missing := []string{"API_TOKEN"}
	m := newMissingEnvModel(".", missing, "")

	m.textInput.SetValue(":skip")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(MissingEnvModel)

	if _, ok := m.enteredValues["API_TOKEN"]; ok {
		t.Fatal("API_TOKEN should have been skipped when typing :skip")
	}
	if !m.confirmed {
		t.Fatal("expected confirmed after skipping single variable")
	}
}

func TestMissingEnvModelCtrlC(t *testing.T) {
	missing := []string{"DATABASE_URL"}
	m := newMissingEnvModel(".", missing, "")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(MissingEnvModel)
	if !m.cancelled {
		t.Fatal("expected cancelled true on Ctrl+C")
	}
	if cmd == nil {
		t.Fatal("expected quit cmd on Ctrl+C")
	}
}

func TestAppendOrWriteEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env.local")

	// Write initial values
	initial := map[string]string{"PORT": "3000"}
	if err := appendOrWriteEnv(envPath, initial); err != nil {
		t.Fatalf("appendOrWriteEnv failed: %v", err)
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "PORT=3000") {
		t.Fatalf("expected PORT=3000 in .env.local, got %s", string(content))
	}

	// Append more values
	more := map[string]string{"SECRET": "abc"}
	if err := appendOrWriteEnv(envPath, more); err != nil {
		t.Fatalf("second appendOrWriteEnv failed: %v", err)
	}

	content2, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content2), "PORT=3000") || !strings.Contains(string(content2), "SECRET=abc") {
		t.Fatalf("expected both PORT and SECRET, got %s", string(content2))
	}
}
