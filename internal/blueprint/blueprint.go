package blueprint

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/harshul/octo-cli/internal/analyzer"
)

// Blueprint is a configuration derived from project analysis.
type Blueprint struct {
	Name       string
	Language   string
	Version    string
	RunCommand string
}

// FromAnalysis converts an analysis result into a basic blueprint.
func FromAnalysis(a analyzer.Analysis) Blueprint {
	return Blueprint{Name: a.Name}
}

// FromProjectInfo converts a ProjectInfo result into a full blueprint.
func FromProjectInfo(p analyzer.ProjectInfo) Blueprint {
	return Blueprint{
		Name:       p.Name,
		Language:   p.Language,
		Version:    p.Version,
		RunCommand: p.RunCommand,
	}
}

// Write writes the blueprint as a YAML file.
func Write(path string, bp Blueprint) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write YAML content
	_, err = fmt.Fprintf(f, "name: %q\n", bp.Name)
	if err != nil {
		return err
	}

	if bp.Language != "" {
		_, err = fmt.Fprintf(f, "language: %s\n", bp.Language)
		if err != nil {
			return err
		}
	}

	if bp.Version != "" {
		_, err = fmt.Fprintf(f, "version: %s\n", bp.Version)
		if err != nil {
			return err
		}
	}

	if bp.RunCommand != "" {
		_, err = fmt.Fprintf(f, "run: %s\n", bp.RunCommand)
		if err != nil {
			return err
		}
	}

	return nil
}

// Read reads a YAML-like file and extracts the blueprint fields.
func Read(path string) (Blueprint, error) {
	f, err := os.Open(path)
	if err != nil {
		return Blueprint{}, err
	}
	defer f.Close()

	bp := Blueprint{}
	s := bufio.NewScanner(f)

	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		if strings.HasPrefix(strings.ToLower(line), "name:") {
			bp.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		} else if strings.HasPrefix(strings.ToLower(line), "language:") {
			bp.Language = strings.TrimSpace(strings.TrimPrefix(line, "language:"))
		} else if strings.HasPrefix(strings.ToLower(line), "version:") {
			bp.Version = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
		} else if strings.HasPrefix(strings.ToLower(line), "run:") {
			bp.RunCommand = strings.TrimSpace(strings.TrimPrefix(line, "run:"))
		}
	}

	if err := s.Err(); err != nil {
		return Blueprint{}, err
	}

	if bp.Name == "" {
		return Blueprint{}, errors.New("invalid configuration: missing name")
	}

	return bp, nil
}