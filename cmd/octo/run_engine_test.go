package main

import "testing"

func TestRunExecutesWithIntelligenceEngine(t *testing.T) {
	// The legacy engine flag is completely removed; runCmd executes intelligence engine directly.
	if flag := runCmd.Flags().Lookup("engine"); flag != nil {
		t.Fatalf("expected engine flag to be removed from runCmd, but found: %v", flag)
	}
}
