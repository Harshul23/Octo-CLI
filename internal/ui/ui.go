package ui

import (
	"fmt"
	"path/filepath"

	"github.com/harshul/octo-cli/internal/analyzer"
)

type Spinner struct {
	msg string
	running bool
}

func NewSpinner(message string) *Spinner {
	return &Spinner{msg: message}
}

func (s *Spinner) Start() {
	if s == nil || s.running {
		return
	}
	s.running = true
	fmt.Println("⏳", s.msg)
}

func (s *Spinner) Stop() {
	if s == nil || !s.running {
		return
	}
	s.running = false
	fmt.Println("Done")
}

func Success(msg string) {
	fmt.Println("✅", msg)
}

func Info(msg string) {
	fmt.Println("ℹ️", msg)
}

// PromptForConfirmation is a minimal interactive stub.
// For now, it simply echoes the provided analysis without changes.
func PromptForConfirmation(a analyzer.Analysis) (analyzer.Analysis, error) {
	// In a richer UI, we'd prompt the user to confirm or adjust fields.
	// Keeping this non-interactive for now to avoid extra deps.
	// Still, provide a tiny hint to the user.
	base := filepath.Base(a.Root)
	fmt.Println("🔍 Using detected project:", base)
	return a, nil
}