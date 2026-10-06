package intelligence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTakeFileSnapshotAndChangeDetection(t *testing.T) {
	tmp := t.TempDir()

	// Create test structure
	goFile := filepath.Join(tmp, "main.go")
	if err := os.WriteFile(goFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ignoredDir := filepath.Join(tmp, "node_modules", "package")
	if err := os.MkdirAll(ignoredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ignoredDir, "index.js"), []byte("// ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	model := ProjectModel{
		Name:     "test-project",
		Language: "Go",
	}

	snap1, err := TakeFileSnapshot(tmp, model)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := snap1["main.go"]; !ok {
		t.Fatal("expected main.go to be in snapshot")
	}
	if _, ok := snap1["node_modules/package/index.js"]; ok {
		t.Fatal("node_modules should be ignored")
	}

	// Unchanged check
	changed, snap2, err := HasSnapshotChanged(tmp, model, snap1)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected no change")
	}

	// Modify file
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(goFile, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, _, err = HasSnapshotChanged(tmp, model, snap2)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected modification to be detected")
	}
}

func TestWatchForChangesCancellation(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	model := ProjectModel{Language: "Go"}
	changed, err := WatchForChanges(ctx, tmp, model, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected changed to be false on context cancellation")
	}
}
