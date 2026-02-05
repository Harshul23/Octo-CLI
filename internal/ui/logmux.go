package ui

import (
	"bufio"
	"io"
	"sync"
	"time"
)

// LogMultiplexer manages log streams for multiple projects
type LogMultiplexer struct {
	projects   []*Project
	dashboard  *DashboardModel
	writers    map[int]*ProjectWriter
	mu         sync.RWMutex
	maxLines   int
	timeFormat string
}

// NewLogMultiplexer creates a new log multiplexer
func NewLogMultiplexer(projects []*Project, dashboard *DashboardModel) *LogMultiplexer {
	return &LogMultiplexer{
		projects:   projects,
		dashboard:  dashboard,
		writers:    make(map[int]*ProjectWriter),
		maxLines:   1000,
		timeFormat: "15:04:05",
	}
}

// GetWriter returns a writer for a specific project index
func (lm *LogMultiplexer) GetWriter(index int) io.Writer {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if writer, exists := lm.writers[index]; exists {
		return writer
	}

	writer := &ProjectWriter{
		multiplexer: lm,
		index:       index,
		buffer:      make([]byte, 0, 4096),
	}
	lm.writers[index] = writer
	return writer
}

// GetCombinedWriter returns a writer that writes to both stdout and project logs
func (lm *LogMultiplexer) GetCombinedWriter(index int, stdout io.Writer) io.Writer {
	return &CombinedWriter{
		project: lm.GetWriter(index),
		stdout:  stdout,
	}
}

// appendLog adds a log line to a project
func (lm *LogMultiplexer) appendLog(index int, line string) {
	if index < 0 || index >= len(lm.projects) {
		return
	}

	// Add timestamp
	timestamp := time.Now().Format(lm.timeFormat)
	formattedLine := "[" + timestamp + "] " + line

	// Append to project
	lm.projects[index].AppendLog(formattedLine)

	// Send to dashboard if available
	if lm.dashboard != nil {
		lm.dashboard.SendLog(index, formattedLine)
	}
}

// ProjectWriter is an io.Writer that captures output for a specific project
type ProjectWriter struct {
	multiplexer *LogMultiplexer
	index       int
	buffer      []byte
	mu          sync.Mutex
}

// Write implements io.Writer
func (pw *ProjectWriter) Write(p []byte) (n int, err error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	// Append to buffer
	pw.buffer = append(pw.buffer, p...)

	// Process complete lines
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

		// Extract line (without newline)
		line := string(pw.buffer[:newlineIdx])
		pw.buffer = pw.buffer[newlineIdx+1:]

		// Send to multiplexer
		if line != "" {
			pw.multiplexer.appendLog(pw.index, line)
		}
	}

	return len(p), nil
}

// Flush writes any remaining buffer content
func (pw *ProjectWriter) Flush() {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	if len(pw.buffer) > 0 {
		line := string(pw.buffer)
		pw.buffer = pw.buffer[:0]
		if line != "" {
			pw.multiplexer.appendLog(pw.index, line)
		}
	}
}

// CombinedWriter writes to both a project log and stdout
type CombinedWriter struct {
	project io.Writer
	stdout  io.Writer
}

// Write implements io.Writer
func (cw *CombinedWriter) Write(p []byte) (n int, err error) {
	// Write to project log
	cw.project.Write(p)

	// Write to stdout if available
	if cw.stdout != nil {
		cw.stdout.Write(p)
	}

	return len(p), nil
}

// LogCapture captures stdout/stderr from a command and routes it to the multiplexer
type LogCapture struct {
	multiplexer *LogMultiplexer
	index       int
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// NewLogCapture creates a new log capture for a project
func NewLogCapture(multiplexer *LogMultiplexer, index int) *LogCapture {
	return &LogCapture{
		multiplexer: multiplexer,
		index:       index,
		stopChan:    make(chan struct{}),
	}
}

// CaptureReader captures output from a reader and sends it to the multiplexer
func (lc *LogCapture) CaptureReader(reader io.Reader, prefix string) {
	lc.wg.Add(1)
	go func() {
		defer lc.wg.Done()

		scanner := bufio.NewScanner(reader)
		// Increase buffer size for long lines
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			select {
			case <-lc.stopChan:
				return
			default:
				line := scanner.Text()
				if prefix != "" {
					line = prefix + line
				}
				lc.multiplexer.appendLog(lc.index, line)
			}
		}
	}()
}

// Stop stops the log capture
func (lc *LogCapture) Stop() {
	close(lc.stopChan)
	lc.wg.Wait()
}

// Wait waits for all capture goroutines to complete
func (lc *LogCapture) Wait() {
	lc.wg.Wait()
}

// LogBuffer provides a simple ring buffer for logs
type LogBuffer struct {
	lines    []string
	maxLines int
	mu       sync.RWMutex
}

// NewLogBuffer creates a new log buffer
func NewLogBuffer(maxLines int) *LogBuffer {
	return &LogBuffer{
		lines:    make([]string, 0, maxLines),
		maxLines: maxLines,
	}
}

// Append adds a line to the buffer
func (lb *LogBuffer) Append(line string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.lines) >= lb.maxLines {
		// Remove oldest line
		copy(lb.lines, lb.lines[1:])
		lb.lines = lb.lines[:len(lb.lines)-1]
	}
	lb.lines = append(lb.lines, line)
}

// GetAll returns all lines in the buffer
func (lb *LogBuffer) GetAll() []string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make([]string, len(lb.lines))
	copy(result, lb.lines)
	return result
}

// GetLast returns the last n lines
func (lb *LogBuffer) GetLast(n int) []string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if n >= len(lb.lines) {
		result := make([]string, len(lb.lines))
		copy(result, lb.lines)
		return result
	}

	start := len(lb.lines) - n
	result := make([]string, n)
	copy(result, lb.lines[start:])
	return result
}

// Clear clears all lines from the buffer
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.lines = lb.lines[:0]
}

// Len returns the number of lines in the buffer
func (lb *LogBuffer) Len() int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return len(lb.lines)
}
