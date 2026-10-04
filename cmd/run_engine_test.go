package main

import "testing"

func TestRunDefaultsToIntelligenceEngine(t *testing.T) {
	flag := runCmd.Flags().Lookup("engine")
	if flag == nil {
		t.Fatal("engine flag is not registered")
	}
	if flag.DefValue != "intelligence" {
		t.Fatalf("engine default=%q, want intelligence", flag.DefValue)
	}
}
