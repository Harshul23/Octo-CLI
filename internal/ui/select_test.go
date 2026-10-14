package ui

import (
	"strings"
	"testing"

	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
)

func TestDecisionModelViewCircles(t *testing.T) {
	items := []decisionItem{
		{option: intelligence.DecisionOption{ID: "opt-1", Value: "npm run dev", Confidence: 0.95}},
		{option: intelligence.DecisionOption{ID: "opt-2", Value: "npm start", Confidence: 0.85}},
		{option: intelligence.DecisionOption{ID: "opt-3", Value: "node index.js", Confidence: 0.70}},
	}

	model := newDecisionModel("Select run command", items)
	model.cursor = 1 // second item selected
	view := model.View()

	// Should not contain the old pointing arrow
	if strings.Contains(view, "❯") {
		t.Errorf("View should not contain old pointing symbol '❯'")
	}

	// Should contain the blue filled circle for selected item
	if !strings.Contains(view, "●") {
		t.Errorf("View should contain filled circle '●' for selected option, got:\n%s", view)
	}

	// Should contain outline circle for unselected items
	if !strings.Contains(view, "○") {
		t.Errorf("View should contain outline circle '○' for unselected options, got:\n%s", view)
	}

	// Count occurrences of outline circle: should have 2 unselected items
	outlineCount := strings.Count(view, "○")
	if outlineCount != 2 {
		t.Errorf("Expected 2 unselected outline circles, got %d", outlineCount)
	}

	// Count occurrences of filled circle: should have 1 selected item
	filledCount := strings.Count(view, "●")
	if filledCount != 1 {
		t.Errorf("Expected 1 selected filled circle, got %d", filledCount)
	}
}

func TestYesNoPromptViewCircles(t *testing.T) {
	prompt := NewYesNoPrompt("Confirm action", "Are you sure?", true)
	view := prompt.View()

	// Initially defaultYes is true (Yes is selected)
	if !strings.Contains(view, "●") {
		t.Errorf("YesNo prompt should contain filled circle '●', got:\n%s", view)
	}
	if !strings.Contains(view, "○") {
		t.Errorf("YesNo prompt should contain outline circle '○', got:\n%s", view)
	}
	if strings.Contains(view, "❯") {
		t.Errorf("YesNo prompt should not contain old pointing symbol '❯'")
	}
}
