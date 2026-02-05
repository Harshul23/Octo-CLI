package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
)

// DashboardRunner manages the TUI dashboard lifecycle
type DashboardRunner struct {
	dashboard    *DashboardModel
	multiplexer  *LogMultiplexer
	program      *tea.Program
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.Mutex
	running      bool
	fallbackMode bool // Use fallback mode (no TUI) when terminal is not interactive
}

// DashboardConfig holds configuration for the dashboard
type DashboardConfig struct {
	Projects       []*Project
	MaxConcurrency int
	FallbackMode   bool // If true, use simple output instead of TUI
}

// NewDashboardRunner creates a new dashboard runner
func NewDashboardRunner(config DashboardConfig) *DashboardRunner {
	ctx, cancel := context.WithCancel(context.Background())

	// Create projects if not provided
	projects := config.Projects
	if projects == nil {
		projects = make([]*Project, 0)
	}

	// Create dashboard model
	dashboard := NewDashboard(projects, config.MaxConcurrency)

	// Create log multiplexer
	multiplexer := NewLogMultiplexer(projects, dashboard)

	return &DashboardRunner{
		dashboard:    dashboard,
		multiplexer:  multiplexer,
		ctx:          ctx,
		cancel:       cancel,
		fallbackMode: config.FallbackMode,
	}
}

// Start starts the dashboard TUI
func (dr *DashboardRunner) Start() error {
	dr.mu.Lock()
	if dr.running {
		dr.mu.Unlock()
		return fmt.Errorf("dashboard already running")
	}
	dr.running = true
	dr.mu.Unlock()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-sigChan:
			dr.Stop()
		case <-dr.ctx.Done():
		}
	}()

	if dr.fallbackMode {
		// In fallback mode, just wait for context cancellation
		<-dr.ctx.Done()
		return nil
	}

	// Create and run the bubbletea program
	dr.program = tea.NewProgram(
		dr.dashboard,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Run the program
	_, err := dr.program.Run()
	
	// Ensure all processes are killed when program exits
	dr.dashboard.GracefulShutdown()
	
	return err
}

// Stop stops the dashboard and gracefully shuts down all running processes
func (dr *DashboardRunner) Stop() {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	if !dr.running {
		return
	}

	dr.running = false
	
	// Gracefully shutdown all running processes
	dr.dashboard.GracefulShutdown()
	
	dr.cancel()

	if dr.program != nil && !dr.fallbackMode {
		dr.dashboard.SendQuit()
		dr.program.Quit()
	}
}

// Wait waits for the dashboard to finish
func (dr *DashboardRunner) Wait() {
	dr.wg.Wait()
}

// IsRunning returns whether the dashboard is running
func (dr *DashboardRunner) IsRunning() bool {
	dr.mu.Lock()
	defer dr.mu.Unlock()
	return dr.running
}

// GetContext returns the dashboard context
func (dr *DashboardRunner) GetContext() context.Context {
	return dr.ctx
}

// GetMultiplexer returns the log multiplexer
func (dr *DashboardRunner) GetMultiplexer() *LogMultiplexer {
	return dr.multiplexer
}

// GetDashboard returns the dashboard model
func (dr *DashboardRunner) GetDashboard() *DashboardModel {
	return dr.dashboard
}

// AddProject adds a new project to the dashboard
func (dr *DashboardRunner) AddProject(name, path string) int {
	project := NewProject(name, path)
	dr.dashboard.projects = append(dr.dashboard.projects, project)
	dr.multiplexer.projects = append(dr.multiplexer.projects, project)
	return len(dr.dashboard.projects) - 1
}

// GetProjectCount returns the number of projects
func (dr *DashboardRunner) GetProjectCount() int {
	return len(dr.dashboard.projects)
}

// GetProject returns a project by index
func (dr *DashboardRunner) GetProject(index int) *Project {
	if index < 0 || index >= len(dr.dashboard.projects) {
		return nil
	}
	return dr.dashboard.projects[index]
}

// UpdateProject updates a project's phase and status
func (dr *DashboardRunner) UpdateProject(index int, phase Phase, status Status) {
	if dr.fallbackMode {
		// In fallback mode, print to stdout
		if index >= 0 && index < len(dr.dashboard.projects) {
			p := dr.dashboard.projects[index]
			p.SetPhase(phase)
			p.SetStatus(status)
			fmt.Printf("[%s] %s: %s\n", phase, p.Name, status)
		}
		return
	}

	dr.dashboard.SendProjectUpdate(index, phase, status)
}

// GetWriter returns an io.Writer for a project's logs
func (dr *DashboardRunner) GetWriter(index int) io.Writer {
	if dr.fallbackMode {
		// In fallback mode, return stdout with project prefix
		return &prefixWriter{
			writer: os.Stdout,
			prefix: fmt.Sprintf("[%d] ", index),
		}
	}
	return dr.multiplexer.GetWriter(index)
}

// GetCombinedWriter returns a writer that writes to both logs and stdout
func (dr *DashboardRunner) GetCombinedWriter(index int) io.Writer {
	if dr.fallbackMode {
		return os.Stdout
	}
	return dr.multiplexer.GetCombinedWriter(index, nil)
}

// prefixWriter adds a prefix to each line written
type prefixWriter struct {
	writer io.Writer
	prefix string
	buffer []byte
}

func (pw *prefixWriter) Write(p []byte) (n int, err error) {
	pw.buffer = append(pw.buffer, p...)

	for {
		// Find newline
		newlineIdx := -1
		for i, b := range pw.buffer {
			if b == '\n' {
				newlineIdx = i
				break
			}
		}

		if newlineIdx < 0 {
			break
		}

		// Write prefixed line
		line := pw.prefix + string(pw.buffer[:newlineIdx+1])
		pw.writer.Write([]byte(line))
		pw.buffer = pw.buffer[newlineIdx+1:]
	}

	return len(p), nil
}

// RunWithDashboard runs a function with the dashboard active
// The function receives the dashboard runner and should use it to update project status
func RunWithDashboard(config DashboardConfig, fn func(*DashboardRunner) error) error {
	runner := NewDashboardRunner(config)

	// Start dashboard in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- runner.Start()
	}()

	// Run the provided function
	fnErr := fn(runner)

	// Stop the dashboard
	runner.Stop()

	// Wait for dashboard to finish
	select {
	case dashErr := <-errChan:
		if fnErr != nil {
			return fnErr
		}
		return dashErr
	}
}

// SimpleRunner provides a simple non-TUI interface that matches DashboardRunner
// Use this when TUI is not available or not desired
type SimpleRunner struct {
	projects []*Project
	mu       sync.Mutex
}

// NewSimpleRunner creates a simple runner
func NewSimpleRunner() *SimpleRunner {
	return &SimpleRunner{
		projects: make([]*Project, 0),
	}
}

// AddProject adds a project
func (sr *SimpleRunner) AddProject(name, path string) int {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	project := NewProject(name, path)
	sr.projects = append(sr.projects, project)
	fmt.Printf("📦 Added project: %s\n", name)
	return len(sr.projects) - 1
}

// UpdateProject updates project status
func (sr *SimpleRunner) UpdateProject(index int, phase Phase, status Status) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if index < 0 || index >= len(sr.projects) {
		return
	}

	p := sr.projects[index]
	p.SetPhase(phase)
	p.SetStatus(status)

	icon := "⏳"
	switch status {
	case StatusRunning:
		icon = "🔄"
	case StatusSuccess:
		icon = "✅"
	case StatusError:
		icon = "❌"
	case StatusStopped:
		icon = "⏹️"
	}

	fmt.Printf("%s [%s] %s: %s\n", icon, phase, p.Name, status)
}

// GetWriter returns a writer for project logs
func (sr *SimpleRunner) GetWriter(index int) io.Writer {
	return os.Stdout
}

// GetProject returns a project by index
func (sr *SimpleRunner) GetProject(index int) *Project {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if index < 0 || index >= len(sr.projects) {
		return nil
	}
	return sr.projects[index]
}
