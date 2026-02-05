package ui

import (
	"bytes"
	"testing"
	"time"
)

func TestNewProject(t *testing.T) {
	p := NewProject("test-project", "/path/to/project")

	if p.Name != "test-project" {
		t.Errorf("expected name 'test-project', got '%s'", p.Name)
	}
	if p.Path != "/path/to/project" {
		t.Errorf("expected path '/path/to/project', got '%s'", p.Path)
	}
	if p.Phase != PhaseIdle {
		t.Errorf("expected phase PhaseIdle, got '%s'", p.Phase)
	}
	if p.Status != StatusPending {
		t.Errorf("expected status StatusPending, got '%s'", p.Status)
	}
}

func TestProjectAppendLog(t *testing.T) {
	p := NewProject("test", "/test")

	p.AppendLog("line 1")
	p.AppendLog("line 2")
	p.AppendLog("line 3")

	logs := p.GetLogs()
	if len(logs) != 3 {
		t.Errorf("expected 3 logs, got %d", len(logs))
	}
	if logs[0] != "line 1" {
		t.Errorf("expected 'line 1', got '%s'", logs[0])
	}
}

func TestProjectURLDetection(t *testing.T) {
	tests := []struct {
		name         string
		logLine      string
		expectedURL  string
		expectedPort int
	}{
		{
			name:         "Vite Local URL",
			logLine:      "  ➜  Local:   http://localhost:5173/",
			expectedURL:  "http://localhost:5173",
			expectedPort: 5173,
		},
		{
			name:         "Vite without arrow",
			logLine:      "Local: http://localhost:3000/",
			expectedURL:  "http://localhost:3000",
			expectedPort: 3000,
		},
		{
			name:         "Plain localhost URL",
			logLine:      "Server running at http://localhost:8080",
			expectedURL:  "http://localhost:8080",
			expectedPort: 8080,
		},
		{
			name:         "0.0.0.0 address converted to localhost",
			logLine:      "Listening on http://0.0.0.0:8080",
			expectedURL:  "http://localhost:8080",
			expectedPort: 8080,
		},
		{
			name:         "127.0.0.1 address converted to localhost",
			logLine:      "Server at http://127.0.0.1:3000",
			expectedURL:  "http://localhost:3000",
			expectedPort: 3000,
		},
		{
			name:         "No URL",
			logLine:      "Starting build process...",
			expectedURL:  "",
			expectedPort: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProject("test", "/test")
			p.AppendLog(tt.logLine)

			if p.URL != tt.expectedURL {
				t.Errorf("expected URL '%s', got '%s'", tt.expectedURL, p.URL)
			}
			if p.Port != tt.expectedPort {
				t.Errorf("expected port %d, got %d", tt.expectedPort, p.Port)
			}
		})
	}
}

func TestURLPriorityDetection(t *testing.T) {
	t.Run("Frontend URL overrides backend URL", func(t *testing.T) {
		p := NewProject("monorepo", "/test")
		
		// Backend starts first (like Hono on 8080) - still gets picked up
		p.AppendLog("server:dev: HTTP listening on http://0.0.0.0:8080")
		
		// Backend URL should be picked up (even with low priority, it's better than nothing)
		if p.URL == "" {
			t.Logf("Backend URL was skipped due to low priority (expected behavior)")
		}
		
		// Frontend starts later (like Next.js on 3000)
		p.AppendLog("client:dev: ready started server on http://localhost:3000")
		
		// Should now have the frontend URL (higher priority)
		if p.URL != "http://localhost:3000" {
			t.Errorf("expected frontend URL 'http://localhost:3000', got '%s'", p.URL)
		}
		if p.Port != 3000 {
			t.Errorf("expected port 3000, got %d", p.Port)
		}
	})
	
	t.Run("Client prefix boosts priority", func(t *testing.T) {
		p := NewProject("monorepo", "/test")
		
		// Server URL comes first
		p.AppendLog("server:dev: HTTP listening on http://0.0.0.0:8080")
		
		// Client URL comes second - should win due to "client:" prefix
		p.AppendLog("client:dev: Local: http://localhost:3000")
		
		if p.URL != "http://localhost:3000" {
			t.Errorf("expected client URL 'http://localhost:3000', got '%s'", p.URL)
		}
	})
	
	t.Run("Next.js URL gets high priority", func(t *testing.T) {
		p := NewProject("app", "/test")
		
		// Generic server first
		p.AppendLog("HTTP listening on http://0.0.0.0:4000")
		
		// Next.js pattern should override
		p.AppendLog("ready started server on http://localhost:3000")
		
		if p.URL != "http://localhost:3000" {
			t.Errorf("expected Next.js URL 'http://localhost:3000', got '%s'", p.URL)
		}
	})
	
	t.Run("Vite URL gets high priority", func(t *testing.T) {
		p := NewProject("app", "/test")
		
		// Backend first
		p.AppendLog("Express listening on http://localhost:4000")
		
		// Vite pattern should override
		p.AppendLog("  ➜  Local:   http://localhost:5173/")
		
		if p.URL != "http://localhost:5173" {
			t.Errorf("expected Vite URL 'http://localhost:5173', got '%s'", p.URL)
		}
	})
	
	t.Run("Backend with server prefix has lower priority than frontend", func(t *testing.T) {
		p := NewProject("monorepo", "/test")
		
		// Even if backend URL has same port, frontend context wins
		p.AppendLog("api:dev: Server running on http://localhost:3000")
		initialURL := p.URL
		
		// Frontend with client prefix should win
		p.AppendLog("client:dev: ready started server on http://localhost:3001")
		
		if p.URL == initialURL {
			t.Errorf("expected frontend URL to override backend, but still have '%s'", p.URL)
		}
		if p.URL != "http://localhost:3001" {
			t.Errorf("expected 'http://localhost:3001', got '%s'", p.URL)
		}
	})
}

func TestProjectSetPhase(t *testing.T) {
	p := NewProject("test", "/test")

	p.SetPhase(PhaseSetup)
	if p.Phase != PhaseSetup {
		t.Errorf("expected PhaseSetup, got '%s'", p.Phase)
	}

	p.SetPhase(PhaseRun)
	if p.Phase != PhaseRun {
		t.Errorf("expected PhaseRun, got '%s'", p.Phase)
	}
}

func TestProjectSetStatus(t *testing.T) {
	p := NewProject("test", "/test")

	p.SetStatus(StatusRunning)
	if p.Status != StatusRunning {
		t.Errorf("expected StatusRunning, got '%s'", p.Status)
	}
	if p.StartTime.IsZero() {
		t.Error("expected StartTime to be set when status is Running")
	}

	p.SetStatus(StatusSuccess)
	if p.Status != StatusSuccess {
		t.Errorf("expected StatusSuccess, got '%s'", p.Status)
	}
}

func TestNewDashboard(t *testing.T) {
	projects := []*Project{
		NewProject("project1", "/p1"),
		NewProject("project2", "/p2"),
	}

	dashboard := NewDashboard(projects, 4)

	if len(dashboard.projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(dashboard.projects))
	}
	if dashboard.maxConcurrency != 4 {
		t.Errorf("expected maxConcurrency 4, got %d", dashboard.maxConcurrency)
	}
	if dashboard.selectedIndex != 0 {
		t.Errorf("expected selectedIndex 0, got %d", dashboard.selectedIndex)
	}
	if dashboard.focusedIndex != -1 {
		t.Errorf("expected focusedIndex -1, got %d", dashboard.focusedIndex)
	}
}

func TestDefaultStyles(t *testing.T) {
	styles := DefaultStyles()
	if styles == nil {
		t.Fatal("DefaultStyles() returned nil")
	}
}

func TestLogBuffer(t *testing.T) {
	lb := NewLogBuffer(5)

	for i := 0; i < 3; i++ {
		lb.Append("line")
	}

	if lb.Len() != 3 {
		t.Errorf("expected length 3, got %d", lb.Len())
	}

	for i := 0; i < 5; i++ {
		lb.Append("overflow")
	}

	if lb.Len() != 5 {
		t.Errorf("expected max length 5, got %d", lb.Len())
	}

	last := lb.GetLast(2)
	if len(last) != 2 {
		t.Errorf("expected 2 items, got %d", len(last))
	}

	lb.Clear()
	if lb.Len() != 0 {
		t.Errorf("expected length 0 after clear, got %d", lb.Len())
	}
}

func TestProjectWriter(t *testing.T) {
	projects := []*Project{
		NewProject("project1", "/p1"),
	}
	dashboard := NewDashboard(projects, 4)
	multiplexer := NewLogMultiplexer(projects, dashboard)

	writer := multiplexer.GetWriter(0)
	writer.Write([]byte("hello\nworld\n"))

	time.Sleep(10 * time.Millisecond)

	logs := projects[0].GetLogs()
	if len(logs) < 2 {
		t.Errorf("expected at least 2 log lines, got %d", len(logs))
	}
}

func TestCombinedWriter(t *testing.T) {
	projects := []*Project{
		NewProject("project1", "/p1"),
	}
	multiplexer := NewLogMultiplexer(projects, nil)

	var buf bytes.Buffer
	writer := multiplexer.GetCombinedWriter(0, &buf)
	writer.Write([]byte("test output\n"))

	if buf.String() != "test output\n" {
		t.Errorf("expected 'test output\\n', got '%s'", buf.String())
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}

	for _, tt := range tests {
		got := FormatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestDashboardRunnerAddProject(t *testing.T) {
	runner := NewDashboardRunner(DashboardConfig{
MaxConcurrency: 4,
FallbackMode:   true,
})

	idx := runner.AddProject("test-project", "/path")

	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}

	if runner.GetProjectCount() != 1 {
		t.Errorf("expected project count 1, got %d", runner.GetProjectCount())
	}

	p := runner.GetProject(0)
	if p == nil {
		t.Fatal("GetProject(0) returned nil")
	}
	if p.Name != "test-project" {
		t.Errorf("expected name 'test-project', got '%s'", p.Name)
	}
}

func TestSimpleRunner(t *testing.T) {
	runner := NewSimpleRunner()

	idx := runner.AddProject("simple-test", "/path")
	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}

	runner.UpdateProject(0, PhaseRun, StatusRunning)

	p := runner.GetProject(0)
	if p.Phase != PhaseRun {
		t.Errorf("expected PhaseRun, got %s", p.Phase)
	}
	if p.Status != StatusRunning {
		t.Errorf("expected StatusRunning, got %s", p.Status)
	}
}
