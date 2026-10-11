package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandAliases(t *testing.T) {
	aliasTests := []struct {
		alias       string
		expectedCmd string
	}{
		{"r", "run"},
		{"start", "run"},
		{"ins", "inspect"},
		{"info", "inspect"},
		{"why", "explain"},
		{"exp", "explain"},
		{"p", "plan"},
		{"v", "verify"},
		{"check", "verify"},
		{"prev", "preview"},
		{"dry-run", "preview"},
		{"g", "graph"},
		{"topo", "graph"},
		{"e", "env"},
		{"bench", "benchmark"},
		{"eval", "benchmark"},
	}

	for _, tt := range aliasTests {
		cmd, _, err := rootCmd.Find([]string{tt.alias})
		if err != nil {
			t.Errorf("failed to find alias %q: %v", tt.alias, err)
			continue
		}
		if cmd == nil || cmd.Name() != tt.expectedCmd {
			t.Errorf("alias %q resolved to %v, want %q", tt.alias, cmd, tt.expectedCmd)
		}
	}

	// Subcommand alias: octo env tpl
	cmd, _, err := rootCmd.Find([]string{"env", "tpl"})
	if err != nil {
		t.Fatalf("failed to find env template alias: %v", err)
	}
	if cmd == nil || cmd.Name() != "template" {
		t.Fatalf("alias tpl resolved to %v, want template", cmd)
	}
}

func TestUniversalJsonFlagShorthand(t *testing.T) {
	type commandWrapper struct {
		name string
		cmd  *cobra.Command
	}

	cmds := []commandWrapper{
		{"run", runCmd},
		{"inspect", inspectCmd},
		{"explain", explainCmd},
		{"plan", planCmd},
		{"verify", verifyCmd},
		{"preview", previewCmd},
		{"graph", graphCmd},
		{"env", envCmd},
		{"benchmark", benchmarkCmd},
	}

	for _, c := range cmds {
		shorthand := c.cmd.Flags().ShorthandLookup("j")
		if shorthand == nil {
			t.Errorf("expected -j shorthand on %s command", c.name)
		} else if shorthand.Name != "json" {
			t.Errorf("expected -j to map to json on %s command, got %s", c.name, shorthand.Name)
		}
	}
}

func TestQuickFlagShorthands(t *testing.T) {
	// runCmd shorthands
	if sh := runCmd.Flags().ShorthandLookup("k"); sh == nil || sh.Name != "skip-env-check" {
		t.Errorf("expected -k to map to skip-env-check on runCmd, got %v", sh)
	}
	if fl := runCmd.Flags().Lookup("skip-env"); fl == nil {
		t.Error("expected --skip-env alias flag on runCmd")
	}
	if sh := runCmd.Flags().ShorthandLookup("T"); sh == nil || sh.Name != "no-tui" {
		t.Errorf("expected -T to map to no-tui on runCmd, got %v", sh)
	}
	if sh := runCmd.Flags().ShorthandLookup("N"); sh == nil || sh.Name != "no-port-shift" {
		t.Errorf("expected -N to map to no-port-shift on runCmd, got %v", sh)
	}

	// benchmarkCmd shorthands
	if sh := benchmarkCmd.Flags().ShorthandLookup("s"); sh == nil || sh.Name != "suite" {
		t.Errorf("expected -s to map to suite on benchmarkCmd, got %v", sh)
	}

	// initCmd shorthands
	if sh := initCmd.Flags().ShorthandLookup("s"); sh == nil || sh.Name != "skip-install" {
		t.Errorf("expected -s to map to skip-install on initCmd, got %v", sh)
	}
	if sh := initCmd.Flags().ShorthandLookup("y"); sh == nil || sh.Name != "auto-install" {
		t.Errorf("expected -y to map to auto-install on initCmd, got %v", sh)
	}
	if sh := initCmd.Flags().ShorthandLookup("S"); sh == nil || sh.Name != "skip-secrets" {
		t.Errorf("expected -S to map to skip-secrets on initCmd, got %v", sh)
	}
}
