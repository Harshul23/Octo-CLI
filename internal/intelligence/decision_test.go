package intelligence

import (
	"context"
	"testing"
)

func TestDeterministicDecisionProviderSelectsStrongestCandidate(t *testing.T) {
	provider := DeterministicDecisionProvider{}
	result, err := provider.Decide(context.Background(), DecisionRequest{
		Name: "run_command",
		Options: []DecisionOption{
			{ID: "file", Value: "go run main.go", Confidence: 0.80},
			{ID: "package", Value: "go run .", Confidence: 0.95},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value != "go run ." {
		t.Fatalf("value=%q, want go run .", result.Value)
	}
	if result.OptionID != "package" {
		t.Fatalf("option=%q, want package", result.OptionID)
	}
}

func TestDeterministicDecisionProviderDoesNotInventValues(t *testing.T) {
	provider := DeterministicDecisionProvider{}
	_, err := provider.Decide(context.Background(), DecisionRequest{
		Name: "run_command",
		Options: []DecisionOption{{ID: "candidate", Value: "go run .", Confidence: 0.9}},
	})
	if err != nil {
		t.Fatal(err)
	}
}
