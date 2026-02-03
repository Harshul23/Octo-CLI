package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/harshul/octo-cli/internal/blueprint"
)

// Options controls how the orchestrator runs the application.
type Options struct {
	WorkDir     string
	Environment string
	RunBuild    bool
	Watch       bool
	Detach      bool
}

type Orchestrator struct {
	bp   blueprint.Blueprint
	opts Options
}

func New(bp blueprint.Blueprint, opts Options) (*Orchestrator, error) {
	return &Orchestrator{bp: bp, opts: opts}, nil
}

// runtimeCommands maps language names to their runtime check commands.
var runtimeCommands = map[string]string{
	"node":       "node",
	"nodejs":     "node",
	"javascript": "node",
	"typescript": "node",
	"java":       "java",
	"python":     "python3",
	"go":         "go",
	"golang":     "go",
	"ruby":       "ruby",
	"rust":       "cargo",
}

// checkRuntime checks if the required runtime is available on the host machine.
func (o *Orchestrator) checkRuntime() {
	if o.bp.Language == "" {
		return
	}

	lang := strings.ToLower(o.bp.Language)
	runtimeCmd, ok := runtimeCommands[lang]
	if !ok {
		// Unknown language, skip the check
		return
	}

	_, err := exec.LookPath(runtimeCmd)
	if err != nil {
		fmt.Printf("⚠️  Warning: %s not found. Please install it.\n", o.bp.Language)
	}
}

func (o *Orchestrator) Run() error {
	fmt.Printf("🚀 Starting %s (env=%s, build=%v, watch=%v, detach=%v)\n",
		o.bp.Name, o.opts.Environment, o.opts.RunBuild, o.opts.Watch, o.opts.Detach)

	// Handle options that are currently not implemented to avoid silently ignoring them.
	if o.opts.RunBuild {
		fmt.Println("⚠️  Warning: RunBuild option is not implemented yet; proceeding without a build step.")
	}
	if o.opts.Watch {
		fmt.Println("⚠️  Warning: Watch option is not implemented yet; changes will not be watched automatically.")
	}
	if o.opts.Detach {
		fmt.Println("⚠️  Warning: Detach option is not implemented yet; the process will run in the foreground.")
	}
	// Check if the required runtime is available
	o.checkRuntime()

	// Check if we have a run command
	if o.bp.RunCommand == "" {
		return fmt.Errorf("no run command specified in configuration")
	}

	// Parse and execute the run command
	// Use shell to handle complex commands with pipes, redirects, etc.
	cmd := exec.Command("sh", "-c", o.bp.RunCommand)

	// Set the working directory
	if o.opts.WorkDir != "" {
		cmd.Dir = o.opts.WorkDir
	}

	// Pipe stdout and stderr directly to the user's terminal
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Printf("📦 Executing: %s\n", o.bp.RunCommand)

	// Run the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}