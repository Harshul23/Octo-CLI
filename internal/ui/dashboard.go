package ui

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Phase represents the current execution phase of a project
type Phase string

const (
	PhaseIdle    Phase = "Idle"
	PhaseSetup   Phase = "Setup"
	PhaseBuild   Phase = "Build"
	PhaseRun     Phase = "Run"
	PhaseStopped Phase = "Stopped"
)

// Status represents the current status of a project
type Status string

const (
	StatusPending Status = "Pending"
	StatusRunning Status = "Running"
	StatusSuccess Status = "Success"
	StatusError   Status = "Error"
	StatusStopped Status = "Stopped"
)

// hyperlink creates a clickable terminal hyperlink using OSC 8 escape sequence
// Note: Disabled because bubbletea's alt screen mode doesn't render OSC 8 properly
// The 'o' key shortcut provides browser opening functionality instead
func hyperlink(url, text string) string {
	// Simply return the text without escape sequences
	// OSC 8 doesn't work well with bubbletea's rendering
	return text
}

// Project represents a project in the dashboard
type Project struct {
	Name        string
	Path        string
	Phase       Phase
	Status      Status
	Logs        []string
	Error       error
	StartTime   time.Time
	Port        int       // Port the project is running on (for URL display)
	URL         string    // Full URL to access the project
	Cmd         *exec.Cmd // Running command for graceful shutdown
	urlPriority int       // Priority score for URL (higher = more likely to be frontend)
	mu          sync.RWMutex
}

// NewProject creates a new project entry
func NewProject(name, path string) *Project {
	return &Project{
		Name:   name,
		Path:   path,
		Phase:  PhaseIdle,
		Status: StatusPending,
		Logs:   make([]string, 0, 1000),
	}
}

// AppendLog adds a log line to the project (thread-safe)
// Also auto-detects URLs from common dev server output patterns
func (p *Project) AppendLog(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Keep last 1000 lines
	if len(p.Logs) >= 1000 {
		p.Logs = p.Logs[1:]
	}
	p.Logs = append(p.Logs, line)
	
	// Auto-detect URL from common dev server patterns
	// Uses intelligent priority scoring to prefer frontend URLs over backend APIs
	p.detectURLFromLog(line)
}

// URLCandidate represents a detected URL with its priority score
type URLCandidate struct {
	URL      string
	Port     int
	Priority int // Higher = more likely to be the frontend the user wants
	Source   string
}

// detectURLFromLog extracts URL from common dev server log patterns
// Uses intelligent scoring to prioritize frontend URLs over backend URLs
func (p *Project) detectURLFromLog(line string) {
	candidate := p.extractURLCandidate(line)
	if candidate == nil {
		return
	}
	
	// Get current URL's priority (0 if none set)
	currentPriority := 0
	if p.URL != "" {
		currentPriority = p.urlPriority
	}
	
	// Only replace if new candidate has higher or equal priority
	// Equal priority allows later URLs to override (e.g., when frontend starts after backend)
	if candidate.Priority >= currentPriority {
		p.URL = candidate.URL
		p.Port = candidate.Port
		p.urlPriority = candidate.Priority
	}
}

// extractURLCandidate parses a log line and returns a URL candidate with priority scoring
func (p *Project) extractURLCandidate(line string) *URLCandidate {
	lowerLine := strings.ToLower(line)
	
	// Pattern to extract any localhost URL
	urlPattern := regexp.MustCompile(`(https?://(?:localhost|127\.0\.0\.1|0\.0\.0\.0):(\d+))`)
	matches := urlPattern.FindStringSubmatch(line)
	if len(matches) < 3 {
		return nil
	}
	
	url := strings.TrimSuffix(matches[1], "/")
	url = strings.Replace(url, "://0.0.0.0:", "://localhost:", 1)
	url = strings.Replace(url, "://127.0.0.1:", "://localhost:", 1)
	
	port, _ := strconv.Atoi(matches[2])
	
	// Calculate priority score based on multiple signals
	priority := 50 // Base score
	
	// === FRONTEND SIGNALS (increase priority) ===
	
	// Next.js patterns (very high priority - clearly a frontend)
	if strings.Contains(lowerLine, "ready started server") || 
	   strings.Contains(lowerLine, "next dev") ||
	   strings.Contains(lowerLine, "▲ next") {
		priority += 100
	}
	
	// Vite patterns (very high priority)
	if strings.Contains(lowerLine, "local:") && 
	   (strings.Contains(lowerLine, "➜") || strings.Contains(lowerLine, "vite")) {
		priority += 100
	}
	
	// React/Vue/Angular dev server patterns
	if strings.Contains(lowerLine, "webpack compiled") ||
	   strings.Contains(lowerLine, "compiled successfully") ||
	   strings.Contains(lowerLine, "dev server running") {
		priority += 80
	}
	
	// Log prefix contains frontend-related keywords
	if strings.Contains(lowerLine, "client") ||
	   strings.Contains(lowerLine, "frontend") ||
	   strings.Contains(lowerLine, "web:") ||
	   strings.Contains(lowerLine, "app:") ||
	   strings.Contains(lowerLine, "ui:") {
		priority += 60
	}
	
	// Common frontend ports
	switch port {
	case 3000, 3001: // Next.js, Create React App default
		priority += 30
	case 5173, 5174: // Vite default
		priority += 30
	case 4200: // Angular default
		priority += 30
	case 8080: // Common but ambiguous
		priority += 5
	}
	
	// === BACKEND SIGNALS (decrease priority) ===
	
	// Explicit backend/API frameworks
	if strings.Contains(lowerLine, "hono") ||
	   strings.Contains(lowerLine, "express") ||
	   strings.Contains(lowerLine, "fastify") ||
	   strings.Contains(lowerLine, "nestjs") ||
	   strings.Contains(lowerLine, "koa") {
		priority -= 40
	}
	
	// Log prefix contains backend-related keywords
	if strings.Contains(lowerLine, "server:") ||
	   strings.Contains(lowerLine, "api:") ||
	   strings.Contains(lowerLine, "backend:") {
		priority -= 50
	}
	
	// Generic "HTTP listening" without frontend context (likely backend)
	if strings.Contains(lowerLine, "http listening") ||
	   strings.Contains(lowerLine, "listening on http") {
		// Only penalize if no frontend signals present
		if !strings.Contains(lowerLine, "client") && 
		   !strings.Contains(lowerLine, "frontend") &&
		   !strings.Contains(lowerLine, "local:") {
			priority -= 30
		}
	}
	
	return &URLCandidate{
		URL:      url,
		Port:     port,
		Priority: priority,
		Source:   line,
	}
}

// GetLogs returns a copy of the logs (thread-safe)
func (p *Project) GetLogs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	logs := make([]string, len(p.Logs))
	copy(logs, p.Logs)
	return logs
}

// SetPhase updates the project phase (thread-safe)
func (p *Project) SetPhase(phase Phase) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Phase = phase
}

// SetStatus updates the project status (thread-safe)
func (p *Project) SetStatus(status Status) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = status
	if status == StatusRunning && p.StartTime.IsZero() {
		p.StartTime = time.Now()
	}
}

// SetPort sets the port the project is running on (thread-safe)
func (p *Project) SetPort(port int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Port = port
	if port > 0 {
		p.URL = fmt.Sprintf("http://localhost:%d", port)
	}
}

// SetURL sets the URL for the project (thread-safe)
func (p *Project) SetURL(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.URL = url
}

// GetURL returns the project URL (thread-safe)
func (p *Project) GetURL() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.URL
}

// SetCmd sets the running command for the project (thread-safe)
func (p *Project) SetCmd(cmd *exec.Cmd) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Cmd = cmd
}

// GetCmd returns the running command (thread-safe)
func (p *Project) GetCmd() *exec.Cmd {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Cmd
}

// GracefulStop attempts to stop the project's process immediately
// Sends SIGINT to the process group, then SIGKILL if needed
func (p *Project) GracefulStop() error {
	p.mu.Lock()
	cmd := p.Cmd
	p.mu.Unlock()
	
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	
	pid := cmd.Process.Pid
	
	// First, try to kill the entire process group with SIGTERM for graceful shutdown
	syscall.Kill(-pid, syscall.SIGTERM)
	
	// Give processes a brief moment to handle SIGTERM
	time.Sleep(100 * time.Millisecond)
	
	// Then force kill the process group with SIGKILL
	// This ensures child processes spawned by shells are also killed
	syscall.Kill(-pid, syscall.SIGKILL)
	
	// Also try direct kill as fallback
	cmd.Process.Kill()
	
	// Kill any processes that might be listening on common dev server ports
	// This catches orphaned processes that escaped the process group
	p.killProcessesOnPort()
	
	return nil
}

// killProcessesOnPort kills any processes listening on the project's port
func (p *Project) killProcessesOnPort() {
	if p.Port <= 0 {
		return
	}
	
	// Use lsof to find processes on the port and kill them
	// This catches orphaned processes that might have escaped the process group
	cmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", p.Port))
	output, err := cmd.Output()
	if err != nil {
		return // No process found or lsof failed
	}
	
	// Kill each PID found
	pids := strings.Fields(strings.TrimSpace(string(output)))
	for _, pidStr := range pids {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		// Kill the process and its group
		syscall.Kill(-pid, syscall.SIGKILL)
		syscall.Kill(pid, syscall.SIGKILL)
	}
}

// ResourceStats holds system resource information
type ResourceStats struct {
	CPUPercent  float64
	MemoryUsed  uint64
	MemoryTotal uint64
	MemPercent  float64
	CPUTemp     float64 // in Celsius, -1 if unavailable
}

// DashboardModel is the main bubbletea model for the TUI dashboard
type DashboardModel struct {
	// Projects
	projects      []*Project
	selectedIndex int
	focusedIndex  int // -1 means no project is focused
	
	// Concurrency
	activeProcesses int
	maxConcurrency  int
	
	// Resources
	resources ResourceStats
	
	// UI state
	width           int
	height          int
	viewport        viewport.Model
	compactViewport viewport.Model // Viewport for logs in compact mode
	showHelp        bool
	quitting        bool
	compactMode     bool // Toggle between dashboard and compact mode (Tab key)
	logsFocused     bool // Whether logs are focused in compact mode (enables scrolling)
	
	// Channels for updates
	updateChan chan tea.Msg
	
	// Key bindings
	keys keyMap
	
	// Styles
	styles *Styles
}

// keyMap defines the key bindings for the dashboard
type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	Escape     key.Binding
	Help       key.Binding
	Quit       key.Binding
	StopAll    key.Binding
	ToggleMode key.Binding
	OpenURL    key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "focus/unfocus"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		StopAll: key.NewBinding(
			key.WithKeys("ctrl+x"),
			key.WithHelp("ctrl+x", "stop all"),
		),
		ToggleMode: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle view"),
		),
		OpenURL: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in browser"),
		),
	}
}

// Styles holds all lipgloss styles for the dashboard
type Styles struct {
	// Base styles
	App          lipgloss.Style
	Header       lipgloss.Style
	Footer       lipgloss.Style
	
	// Project list styles
	ProjectList     lipgloss.Style
	ProjectItem     lipgloss.Style
	ProjectSelected lipgloss.Style
	ProjectFocused  lipgloss.Style
	
	// Status styles
	StatusPending lipgloss.Style
	StatusRunning lipgloss.Style
	StatusSuccess lipgloss.Style
	StatusError   lipgloss.Style
	StatusStopped lipgloss.Style
	
	// Phase styles
	PhaseIdle  lipgloss.Style
	PhaseSetup lipgloss.Style
	PhaseBuild lipgloss.Style
	PhaseRun   lipgloss.Style
	
	// Monitor styles
	MonitorBox      lipgloss.Style
	ProgressBar     lipgloss.Style
	ProgressFill    lipgloss.Style
	ProgressEmpty   lipgloss.Style
	
	// Log styles
	LogViewport lipgloss.Style
	LogLine     lipgloss.Style
	LogError    lipgloss.Style
	
	// Help styles
	Help     lipgloss.Style
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
}

// DefaultStyles returns the default color scheme
func DefaultStyles() *Styles {
	subtle := lipgloss.AdaptiveColor{Light: "#666", Dark: "#999"}
	highlight := lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#AD8EE6"}
	success := lipgloss.AdaptiveColor{Light: "#00AA00", Dark: "#00FF00"}
	warning := lipgloss.AdaptiveColor{Light: "#AAAA00", Dark: "#FFFF00"}
	errorColor := lipgloss.AdaptiveColor{Light: "#AA0000", Dark: "#FF0000"}
	info := lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#00AAFF"}
	
	return &Styles{
		App: lipgloss.NewStyle().
			Padding(1, 2),
		
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(subtle).
			MarginBottom(1).
			Padding(0, 1),
		
		Footer: lipgloss.NewStyle().
			Foreground(subtle).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(subtle).
			MarginTop(1).
			Padding(0, 1),
		
		ProjectList: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(0, 1),
		
		ProjectItem: lipgloss.NewStyle().
			Padding(0, 1),
		
		ProjectSelected: lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.AdaptiveColor{Light: "#E0E0E0", Dark: "#333333"}).
			Bold(true),
		
		ProjectFocused: lipgloss.NewStyle().
			Padding(0, 1).
			Background(highlight).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true),
		
		StatusPending: lipgloss.NewStyle().
			Foreground(subtle),
		
		StatusRunning: lipgloss.NewStyle().
			Foreground(info).
			Bold(true),
		
		StatusSuccess: lipgloss.NewStyle().
			Foreground(success),
		
		StatusError: lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true),
		
		StatusStopped: lipgloss.NewStyle().
			Foreground(warning),
		
		PhaseIdle: lipgloss.NewStyle().
			Foreground(subtle),
		
		PhaseSetup: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#9933FF", Dark: "#CC99FF"}),
		
		PhaseBuild: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FF9900", Dark: "#FFCC00"}),
		
		PhaseRun: lipgloss.NewStyle().
			Foreground(info),
		
		MonitorBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(0, 1).
			MarginTop(1),
		
		ProgressBar: lipgloss.NewStyle(),
		
		ProgressFill: lipgloss.NewStyle().
			Foreground(success),
		
		ProgressEmpty: lipgloss.NewStyle().
			Foreground(subtle),
		
		LogViewport: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(highlight).
			Padding(0, 1),
		
		LogLine: lipgloss.NewStyle().
			Foreground(subtle),
		
		LogError: lipgloss.NewStyle().
			Foreground(errorColor),
		
		Help: lipgloss.NewStyle().
			Foreground(subtle),
		
		HelpKey: lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true),
		
		HelpDesc: lipgloss.NewStyle().
			Foreground(subtle),
	}
}

// Messages for bubbletea
type tickMsg time.Time
type resourceUpdateMsg ResourceStats
type projectUpdateMsg struct {
	index  int
	phase  Phase
	status Status
}
type logMsg struct {
	index int
	line  string
}
type quitMsg struct{}

// NewDashboard creates a new dashboard model
func NewDashboard(projects []*Project, maxConcurrency int) *DashboardModel {
	vp := viewport.New(80, 20)
	vp.SetContent("")
	vp.MouseWheelEnabled = true
	
	// Compact viewport for scrollable logs
	cvp := viewport.New(80, 20)
	cvp.SetContent("")
	cvp.MouseWheelEnabled = true
	
	return &DashboardModel{
		projects:        projects,
		selectedIndex:   0,
		focusedIndex:    -1,
		maxConcurrency:  maxConcurrency,
		viewport:        vp,
		compactViewport: cvp,
		keys:            defaultKeyMap(),
		styles:          DefaultStyles(),
		updateChan:      make(chan tea.Msg, 100),
		compactMode:     true, // Default to compact (normal scrolling) view
		logsFocused:     true, // Logs are focused by default for scrolling
	}
}

// Init implements tea.Model
func (m *DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		m.listenForUpdates(),
	)
}

// tickCmd returns a command that ticks every second
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// listenForUpdates listens for external updates
func (m *DashboardModel) listenForUpdates() tea.Cmd {
	return func() tea.Msg {
		return <-m.updateChan
	}
}

// Update implements tea.Model
func (m *DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Handle quit FIRST - before anything else can consume the key
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, m.keys.Quit) {
			m.quitting = true
			// Stop all running processes SYNCHRONOUSLY before quitting
			// This ensures servers are killed before the program exits
			m.GracefulShutdown()
			return m, tea.Quit
		}
	}
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.ToggleMode):
			m.compactMode = !m.compactMode
			
		case key.Matches(msg, m.keys.OpenURL):
			// Open project URL in browser
			// In compact mode: open first project with URL
			// In dashboard mode: open selected project
			var targetProject *Project
			
			if m.compactMode {
				// Find first project with a URL (prefer running ones)
				for _, p := range m.projects {
					if p.URL != "" || p.Port > 0 {
						targetProject = p
						break
					}
				}
			} else {
				// Use selected project in dashboard mode
				if m.selectedIndex >= 0 && m.selectedIndex < len(m.projects) {
					targetProject = m.projects[m.selectedIndex]
				}
			}
			
			if targetProject != nil {
				url := targetProject.URL
				if url == "" && targetProject.Port > 0 {
					url = fmt.Sprintf("http://localhost:%d", targetProject.Port)
				}
				if url != "" {
					m.openInBrowser(url)
				}
			}
			
		case key.Matches(msg, m.keys.Up):
			if m.compactMode && m.logsFocused {
				// Scroll compact viewport up
				var cmd tea.Cmd
				m.compactViewport, cmd = m.compactViewport.Update(msg)
				cmds = append(cmds, cmd)
			} else if m.focusedIndex >= 0 {
				// Scroll viewport up when focused in dashboard mode
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			} else if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			
		case key.Matches(msg, m.keys.Down):
			if m.compactMode && m.logsFocused {
				// Scroll compact viewport down
				var cmd tea.Cmd
				m.compactViewport, cmd = m.compactViewport.Update(msg)
				cmds = append(cmds, cmd)
			} else if m.focusedIndex >= 0 {
				// Scroll viewport down when focused in dashboard mode
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			} else if m.selectedIndex < len(m.projects)-1 {
				m.selectedIndex++
			}
			
		case key.Matches(msg, m.keys.Enter):
			if m.compactMode {
				// Toggle logs focus in compact mode
				m.logsFocused = !m.logsFocused
			} else if m.focusedIndex >= 0 {
				// Unfocus in dashboard mode
				m.focusedIndex = -1
			} else if len(m.projects) > 0 {
				// Focus selected project in dashboard mode
				m.focusedIndex = m.selectedIndex
				m.updateViewportContent()
			}
			
		case key.Matches(msg, m.keys.Escape):
			if m.compactMode && m.logsFocused {
				m.logsFocused = false
			} else if m.focusedIndex >= 0 {
				m.focusedIndex = -1
			}
			
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
		}
		
	case tea.MouseMsg:
		// Handle mouse wheel scrolling
		if m.compactMode {
			var cmd tea.Cmd
			m.compactViewport, cmd = m.compactViewport.Update(msg)
			cmds = append(cmds, cmd)
		} else if m.focusedIndex >= 0 {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
		
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Dashboard mode viewport
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 15 // Leave room for header/footer
		// Compact mode viewport - use most of terminal height for logs
		m.compactViewport.Width = msg.Width - 4
		m.compactViewport.Height = msg.Height - 8 // Header(2) + URL(1) + Footer(2) + margins
		if m.focusedIndex >= 0 {
			m.updateViewportContent()
		}
		if m.compactMode {
			m.updateCompactViewportContent()
		}
		
	case tickMsg:
		// Update resource stats
		cmds = append(cmds, tickCmd())
		cmds = append(cmds, m.fetchResourceStats())
		if m.focusedIndex >= 0 {
			m.updateViewportContent()
		}
		if m.compactMode {
			m.updateCompactViewportContent()
		}
		
	case resourceUpdateMsg:
		m.resources = ResourceStats(msg)
		
	case projectUpdateMsg:
		if msg.index >= 0 && msg.index < len(m.projects) {
			m.projects[msg.index].SetPhase(msg.phase)
			m.projects[msg.index].SetStatus(msg.status)
		}
		cmds = append(cmds, m.listenForUpdates())
		
	case logMsg:
		if msg.index >= 0 && msg.index < len(m.projects) {
			m.projects[msg.index].AppendLog(msg.line)
			if m.focusedIndex == msg.index {
				m.updateViewportContent()
			}
			if m.compactMode {
				m.updateCompactViewportContent()
			}
		}
		cmds = append(cmds, m.listenForUpdates())
		
	case quitMsg:
		m.quitting = true
		return m, tea.Quit
	}
	
	return m, tea.Batch(cmds...)
}

// openInBrowser opens a URL in the default browser
func (m *DashboardModel) openInBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	cmd.Start()
}

// fetchResourceStats fetches system resource statistics
func (m *DashboardModel) fetchResourceStats() tea.Cmd {
	return func() tea.Msg {
		stats := GetResourceStats()
		return resourceUpdateMsg(stats)
	}
}

// updateViewportContent updates the viewport with current logs
func (m *DashboardModel) updateViewportContent() {
	if m.focusedIndex < 0 || m.focusedIndex >= len(m.projects) {
		return
	}
	
	logs := m.projects[m.focusedIndex].GetLogs()
	
	// Check if user is at the bottom before updating content
	atBottom := m.viewport.AtBottom()
	
	content := strings.Join(logs, "\n")
	m.viewport.SetContent(content)
	
	// Only auto-scroll to bottom if user was already at the bottom
	if atBottom {
		m.viewport.GotoBottom()
	}
}

// updateCompactViewportContent updates the compact viewport with all project logs
func (m *DashboardModel) updateCompactViewportContent() {
	var lines []string
	
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"})
	
	for _, p := range m.projects {
		if p.Status == StatusRunning || p.Status == StatusError {
			logs := p.GetLogs()
			
			// Project name with status indicator
			statusIcon := "●"
			statusStyle := m.styles.StatusRunning
			if p.Status == StatusError {
				statusIcon = "✗"
				statusStyle = m.styles.StatusError
			}
			
			lines = append(lines, statusStyle.Render(fmt.Sprintf("%s %s", statusIcon, p.Name)))
			
			for _, log := range logs {
				// Truncate long lines
				if len(log) > m.width-4 {
					log = log[:m.width-7] + "..."
				}
				lines = append(lines, dimStyle.Render("  "+log))
			}
			lines = append(lines, "") // Add spacing between projects
		}
	}
	
	// Check if user is at the bottom before updating content
	atBottom := m.compactViewport.AtBottom()
	
	content := strings.Join(lines, "\n")
	m.compactViewport.SetContent(content)
	
	// Only auto-scroll to bottom if user was already at the bottom
	if atBottom {
		m.compactViewport.GotoBottom()
	}
}

// View implements tea.Model
func (m *DashboardModel) View() string {
	if m.quitting {
		return "Shutting down...\n"
	}
	
	// Compact mode shows minimal info with streaming logs
	if m.compactMode {
		return m.renderCompactView()
	}
	
	var b strings.Builder
	
	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")
	
	if m.focusedIndex >= 0 {
		// Focused view - show logs
		b.WriteString(m.renderFocusedView())
	} else {
		// Main view - show project list and monitors
		b.WriteString(m.renderMainView())
	}
	
	// Footer
	footer := m.renderFooter()
	b.WriteString("\n")
	b.WriteString(footer)
	
	return m.styles.App.Render(b.String())
}

// renderHeader renders the dashboard header
func (m *DashboardModel) renderHeader() string {
	title := "🐙 Octo Dashboard"
	
	// Count active processes
	active := 0
	for _, p := range m.projects {
		if p.Status == StatusRunning {
			active++
		}
	}
	m.activeProcesses = active
	
	status := fmt.Sprintf("Projects: %d | Active: %d/%d",
		len(m.projects), active, m.maxConcurrency)
	
	// Add resource info
	if m.resources.CPUPercent > 0 {
		status += fmt.Sprintf(" | CPU: %.1f%%", m.resources.CPUPercent)
	}
	if m.resources.MemPercent > 0 {
		status += fmt.Sprintf(" | Mem: %.1f%%", m.resources.MemPercent)
	}
	if m.resources.CPUTemp > 0 {
		status += fmt.Sprintf(" | Temp: %.0f°C", m.resources.CPUTemp)
	}
	
	headerWidth := m.width - 4
	if headerWidth < 40 {
		headerWidth = 40
	}
	
	padding := headerWidth - lipgloss.Width(title) - lipgloss.Width(status)
	if padding < 1 {
		padding = 1
	}
	
	return m.styles.Header.Width(headerWidth).Render(
		title + strings.Repeat(" ", padding) + status,
	)
}

// renderMainView renders the main dashboard view
func (m *DashboardModel) renderMainView() string {
	var b strings.Builder
	
	// Project list
	b.WriteString(m.renderProjectList())
	b.WriteString("\n")
	
	// Concurrency monitor
	b.WriteString(m.renderConcurrencyMonitor())
	b.WriteString("\n")
	
	// Resource monitor
	b.WriteString(m.renderResourceMonitor())
	
	return b.String()
}

// renderProjectList renders the list of projects
func (m *DashboardModel) renderProjectList() string {
	var items []string
	
	listWidth := m.width - 6
	if listWidth < 60 {
		listWidth = 60
	}
	
	for i, p := range m.projects {
		item := m.renderProjectItem(i, p, listWidth)
		items = append(items, item)
	}
	
	content := strings.Join(items, "\n")
	return m.styles.ProjectList.Width(listWidth).Render(content)
}

// renderProjectItem renders a single project item
func (m *DashboardModel) renderProjectItem(index int, p *Project, width int) string {
	// Determine style based on selection state
	style := m.styles.ProjectItem
	if index == m.selectedIndex {
		style = m.styles.ProjectSelected
	}
	
	// Project name (truncate if needed)
	name := p.Name
	maxNameLen := 25
	if len(name) > maxNameLen {
		name = name[:maxNameLen-3] + "..."
	}
	
	// Phase indicator
	phase := m.renderPhase(p.Phase)
	
	// Status indicator
	status := m.renderStatus(p.Status)
	
	// Duration (if running)
	duration := ""
	if p.Status == StatusRunning && !p.StartTime.IsZero() {
		d := time.Since(p.StartTime).Round(time.Second)
		duration = fmt.Sprintf(" %s", d)
	}
	
	// URL (if running and available)
	urlInfo := ""
	if p.Status == StatusRunning {
		if p.URL != "" {
			urlInfo = m.styles.StatusRunning.Render(fmt.Sprintf(" → %s", p.URL))
		} else if p.Port > 0 {
			urlInfo = m.styles.StatusRunning.Render(fmt.Sprintf(" → http://localhost:%d", p.Port))
		}
	}
	
	// Build the line
	line := fmt.Sprintf("%-*s  %s  %s%s%s",
		maxNameLen, name, phase, status, duration, urlInfo)
	
	return style.Width(width - 2).Render(line)
}

// renderPhase renders a phase indicator
func (m *DashboardModel) renderPhase(phase Phase) string {
	var style lipgloss.Style
	var icon string
	
	switch phase {
	case PhaseSetup:
		style = m.styles.PhaseSetup
		icon = "⚙️"
	case PhaseBuild:
		style = m.styles.PhaseBuild
		icon = "🔨"
	case PhaseRun:
		style = m.styles.PhaseRun
		icon = "▶️"
	case PhaseStopped:
		style = m.styles.StatusStopped
		icon = "⏹️"
	default:
		style = m.styles.PhaseIdle
		icon = "⏸️"
	}
	
	return style.Render(fmt.Sprintf("%s %-6s", icon, phase))
}

// renderStatus renders a status indicator
func (m *DashboardModel) renderStatus(status Status) string {
	var style lipgloss.Style
	var icon string
	
	switch status {
	case StatusRunning:
		style = m.styles.StatusRunning
		icon = "●"
	case StatusSuccess:
		style = m.styles.StatusSuccess
		icon = "✓"
	case StatusError:
		style = m.styles.StatusError
		icon = "✗"
	case StatusStopped:
		style = m.styles.StatusStopped
		icon = "○"
	default:
		style = m.styles.StatusPending
		icon = "◌"
	}
	
	return style.Render(fmt.Sprintf("%s %s", icon, status))
}

// renderConcurrencyMonitor renders the concurrency monitor
func (m *DashboardModel) renderConcurrencyMonitor() string {
	title := "Concurrency"
	
	// Calculate progress
	progress := float64(m.activeProcesses) / float64(m.maxConcurrency)
	if progress > 1 {
		progress = 1
	}
	
	barWidth := 30
	filled := int(progress * float64(barWidth))
	empty := barWidth - filled
	
	bar := m.styles.ProgressFill.Render(strings.Repeat("█", filled)) +
		m.styles.ProgressEmpty.Render(strings.Repeat("░", empty))
	
	text := fmt.Sprintf("%s: [%s] %d/%d",
		title, bar, m.activeProcesses, m.maxConcurrency)
	
	return m.styles.MonitorBox.Render(text)
}

// renderResourceMonitor renders the resource monitor
func (m *DashboardModel) renderResourceMonitor() string {
	var parts []string
	
	// CPU bar
	cpuProgress := m.resources.CPUPercent / 100
	cpuBar := m.renderProgressBar("CPU", cpuProgress, 20)
	parts = append(parts, cpuBar)
	
	// Memory bar
	memProgress := m.resources.MemPercent / 100
	memBar := m.renderProgressBar("Mem", memProgress, 20)
	parts = append(parts, memBar)
	
	// Temperature (if available)
	if m.resources.CPUTemp > 0 {
		tempColor := m.styles.ProgressFill
		if m.resources.CPUTemp > 80 {
			tempColor = m.styles.StatusError
		} else if m.resources.CPUTemp > 60 {
			tempColor = m.styles.StatusStopped
		}
		tempStr := tempColor.Render(fmt.Sprintf("🌡️ %.0f°C", m.resources.CPUTemp))
		parts = append(parts, tempStr)
	}
	
	content := strings.Join(parts, "  ")
	return m.styles.MonitorBox.Render(content)
}

// renderProgressBar renders a progress bar
func (m *DashboardModel) renderProgressBar(label string, progress float64, width int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	
	filled := int(progress * float64(width))
	empty := width - filled
	
	bar := m.styles.ProgressFill.Render(strings.Repeat("█", filled)) +
		m.styles.ProgressEmpty.Render(strings.Repeat("░", empty))
	
	return fmt.Sprintf("%s [%s] %5.1f%%", label, bar, progress*100)
}

// renderFocusedView renders the focused project view with logs
func (m *DashboardModel) renderFocusedView() string {
	if m.focusedIndex < 0 || m.focusedIndex >= len(m.projects) {
		return ""
	}
	
	p := m.projects[m.focusedIndex]
	
	var b strings.Builder
	
	// Project info header
	info := fmt.Sprintf("📋 %s | %s | %s",
		p.Name, m.renderPhase(p.Phase), m.renderStatus(p.Status))
	b.WriteString(info)
	b.WriteString("\n\n")
	
	// Log viewport
	viewportWidth := m.width - 6
	if viewportWidth < 60 {
		viewportWidth = 60
	}
	
	m.viewport.Width = viewportWidth
	b.WriteString(m.styles.LogViewport.Width(viewportWidth).Render(m.viewport.View()))
	
	return b.String()
}

// renderCompactView renders a minimal view with logs
func (m *DashboardModel) renderCompactView() string {
	var b strings.Builder
	
	// Compact header with essential info
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#AD8EE6"})
	
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"})
	
	urlStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#00AA00", Dark: "#00FF00"}).
		Underline(true)
	
	// Count active projects
	active := 0
	for _, p := range m.projects {
		if p.Status == StatusRunning {
			active++
		}
	}
	
	b.WriteString(headerStyle.Render("🐙 Octo"))
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d running", active, len(m.projects))))
	
	// Show resource stats inline
	if m.resources.CPUPercent > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  CPU: %.0f%%", m.resources.CPUPercent)))
	}
	if m.resources.CPUTemp > 0 {
		tempStyle := dimStyle
		if m.resources.CPUTemp > 70 {
			tempStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6600"))
		}
		b.WriteString(tempStyle.Render(fmt.Sprintf("  🌡️%.0f°C", m.resources.CPUTemp)))
	}
	b.WriteString("\n")
	
	// Show project URLs - display for any project with a port/URL
	for _, p := range m.projects {
		url := p.URL
		if url == "" && p.Port > 0 {
			url = fmt.Sprintf("http://localhost:%d", p.Port)
		}
		if url != "" {
			// Use different style based on running state
			linkStyle := urlStyle
			if p.Status != StatusRunning {
				linkStyle = dimStyle
			}
			b.WriteString(linkStyle.Render(fmt.Sprintf("  ➜ %s: %s", p.Name, url)))
			b.WriteString("\n")
		}
	}
	
	b.WriteString("\n")
	
	// Use viewport for scrollable logs
	if m.logsFocused {
		// When focused, show the scrollable viewport
		viewportStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#AD8EE6"}).
			Padding(0, 1)
		b.WriteString(viewportStyle.Render(m.compactViewport.View()))
	} else {
		// When not focused, show the scrollable viewport without highlighted border
		viewportStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#CCCCCC", Dark: "#444444"}).
			Padding(0, 1)
		b.WriteString(viewportStyle.Render(m.compactViewport.View()))
	}
	
	b.WriteString("\n")
	
	// Compact footer with focus-aware help
	var helpText string
	if m.logsFocused {
		helpText = fmt.Sprintf("%s scroll • %s unfocus • %s toggle view • %s open browser • %s quit",
			m.styles.HelpKey.Render("↑↓/scroll"),
			m.styles.HelpKey.Render("esc"),
			m.styles.HelpKey.Render("tab"),
			m.styles.HelpKey.Render("o"),
			m.styles.HelpKey.Render("q"))
	} else {
		helpText = fmt.Sprintf("%s focus logs • %s toggle view • %s open browser • %s quit",
			m.styles.HelpKey.Render("enter"),
			m.styles.HelpKey.Render("tab"),
			m.styles.HelpKey.Render("o"),
			m.styles.HelpKey.Render("q"))
	}
	b.WriteString(dimStyle.Render(helpText))
	
	return b.String()
}

// renderFooter renders the dashboard footer with help
func (m *DashboardModel) renderFooter() string {
	var help string
	
	modeIndicator := "📊 Dashboard"
	if m.compactMode {
		modeIndicator = "📋 Compact"
	}
	
	if m.focusedIndex >= 0 {
		help = fmt.Sprintf("%s • %s scroll • %s back • %s quit",
			modeIndicator,
			m.styles.HelpKey.Render("↑↓/jk"),
			m.styles.HelpKey.Render("esc/enter"),
			m.styles.HelpKey.Render("q"))
	} else {
		// Check if any project has a URL
		hasURL := false
		for _, p := range m.projects {
			if p.URL != "" || p.Port > 0 {
				hasURL = true
				break
			}
		}
		
		if hasURL {
			help = fmt.Sprintf("%s • %s nav • %s focus • %s open • %s view • %s quit",
				modeIndicator,
				m.styles.HelpKey.Render("↑↓"),
				m.styles.HelpKey.Render("enter"),
				m.styles.HelpKey.Render("o"),
				m.styles.HelpKey.Render("tab"),
				m.styles.HelpKey.Render("q"))
		} else {
			help = fmt.Sprintf("%s • %s nav • %s focus • %s view • %s quit",
				modeIndicator,
				m.styles.HelpKey.Render("↑↓"),
				m.styles.HelpKey.Render("enter"),
				m.styles.HelpKey.Render("tab"),
				m.styles.HelpKey.Render("q"))
		}
	}
	
	footerWidth := m.width - 4
	if footerWidth < 40 {
		footerWidth = 40
	}
	
	return m.styles.Footer.Width(footerWidth).Render(help)
}

// Public methods for external updates

// SendProjectUpdate sends a project update to the dashboard
func (m *DashboardModel) SendProjectUpdate(index int, phase Phase, status Status) {
	select {
	case m.updateChan <- projectUpdateMsg{index: index, phase: phase, status: status}:
	default:
		// Channel full, drop update
	}
}

// SendLog sends a log line to a project
func (m *DashboardModel) SendLog(index int, line string) {
	select {
	case m.updateChan <- logMsg{index: index, line: line}:
	default:
		// Channel full, drop log
	}
}

// SendQuit sends a quit signal to the dashboard
func (m *DashboardModel) SendQuit() {
	select {
	case m.updateChan <- quitMsg{}:
	default:
	}
}

// GracefulShutdown stops all running projects immediately
func (m *DashboardModel) GracefulShutdown() {
	var wg sync.WaitGroup
	
	for _, p := range m.projects {
		if p.Status == StatusRunning || p.Cmd != nil {
			wg.Add(1)
			go func(proj *Project) {
				defer wg.Done()
				proj.GracefulStop()
			}(p)
		}
	}
	
	// Wait for all processes to stop (with reasonable timeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All processes stopped
	case <-time.After(3 * time.Second):
		// Timeout - force kill any remaining processes
		for _, p := range m.projects {
			if p.Cmd != nil && p.Cmd.Process != nil {
				syscall.Kill(-p.Cmd.Process.Pid, syscall.SIGKILL)
				p.Cmd.Process.Kill()
			}
			// Also kill by port as last resort
			p.killProcessesOnPort()
		}
	}
}

// GetUpdateChannel returns the update channel for external use
func (m *DashboardModel) GetUpdateChannel() chan tea.Msg {
	return m.updateChan
}
