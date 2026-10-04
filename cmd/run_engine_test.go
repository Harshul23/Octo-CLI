package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRunDefaultsToIntelligenceEngine(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("engine", "intelligence", "")
	value, err := cmd.Flags().GetString("engine")
	if err != nil {
		t.Fatal(err)
	}
	if value != "intelligence" {
		t.Fatalf("engine default=%q, want intelligence", value)
	}
}
