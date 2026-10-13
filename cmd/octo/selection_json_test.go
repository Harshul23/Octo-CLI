package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	data, _ := io.ReadAll(r)
	return string(data)
}

func TestSelectionFailureIsStructuredJSONWithJSONFlag(t *testing.T) {
	model := intelligence.ProjectModel{Components: []intelligence.Component{
		{Name: "web", Path: "apps/web", RunCommand: "node web.js", Role: "application"},
		{Name: "api", Path: "apps/api", RunCommand: "node api.js", Role: "application"},
	}}
	var returned error
	out := captureStdout(t, func() {
		returned = selectionFailure(true, "selection_required", errors.New("multiple runnable targets detected"), model)
	})
	var parsed struct {
		Success bool `json:"success"`
		Error   struct {
			Code             string   `json:"code"`
			Message          string   `json:"message"`
			AvailableTargets []string `json:"available_targets"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out)
	}
	if parsed.Success || parsed.Error.Code != "selection_required" || len(parsed.Error.AvailableTargets) != 2 {
		t.Fatalf("unexpected payload: %+v", parsed)
	}
	var reported reportedError
	if !errors.As(returned, &reported) {
		t.Fatal("expected a reportedError so the exit code stays non-zero without duplicate stderr output")
	}
}

func TestSelectionFailureWithoutJSONFlagReturnsPlainError(t *testing.T) {
	want := errors.New("multiple runnable targets detected")
	out := captureStdout(t, func() {
		if got := selectionFailure(false, "selection_required", want, intelligence.ProjectModel{}); got != want {
			t.Fatalf("expected the original error, got %v", got)
		}
	})
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected no stdout output, got %q", out)
	}
}
