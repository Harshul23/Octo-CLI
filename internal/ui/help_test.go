package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestColorizeHelpSections(t *testing.T) {
	raw := `Octo is an execution intelligence CLI.

Usage:
  octo [command]

Main Commands:
  inspect     Inspect how Octo understands a repository
  run         Understand and run the repository locally

Other Commands:
  benchmark   Benchmark autonomous execution reliability
  env         Inspect environment variables

Additional Commands:
  help        Help about any command

Flags:
  -h, --help      help for octo
  -p, --port int  Override port

Use "octo [command] --help" for more information about a command.
`

	colored := ColorizeHelp(raw)

	// Verify that section titles are present and rendered
	for _, sec := range []string{"Usage:", "Main Commands:", "Other Commands:", "Additional Commands:", "Flags:"} {
		if !strings.Contains(colored, sec) {
			t.Errorf("Expected colored output to contain section title %q", sec)
		}
	}

	// Verify command items are preserved
	for _, cmd := range []string{"inspect", "run", "benchmark", "env", "help"} {
		if !strings.Contains(colored, cmd) {
			t.Errorf("Expected colored output to contain command %q", cmd)
		}
	}

	// Verify flags are preserved
	if !strings.Contains(colored, "--help") || !strings.Contains(colored, "--port") {
		t.Errorf("Expected colored output to contain flags --help and --port")
	}

	// Verify footer is preserved
	if !strings.Contains(colored, "for more information about a command.") {
		t.Errorf("Expected colored output to contain footer")
	}
}

func TestSetupColoredHelpIntegration(t *testing.T) {
	root := &cobra.Command{
		Use:   "testapp",
		Short: "A test CLI",
		Long:  "A long description of test CLI",
	}

	sub := &cobra.Command{
		Use:   "sub",
		Short: "A sub command",
		Run:   func(cmd *cobra.Command, args []string) {},
	}
	root.AddCommand(sub)

	buf := new(bytes.Buffer)
	root.SetOut(buf)

	SetupColoredHelp(root)

	// Call help function directly
	root.HelpFunc()(root, []string{})

	output := buf.String()
	if !strings.Contains(output, "testapp") {
		t.Errorf("Expected output to contain 'testapp', got:\n%s", output)
	}
	if !strings.Contains(output, "Usage:") {
		t.Errorf("Expected output to contain 'Usage:', got:\n%s", output)
	}
}
