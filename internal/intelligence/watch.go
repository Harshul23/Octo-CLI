package intelligence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TakeFileSnapshot records modification times for all execution-relevant files in the project.
func TakeFileSnapshot(root string, model ProjectModel) (map[string]int64, error) {
	snapshot := make(map[string]int64)

	// Determine languages present in model
	languages := make(map[string]struct{})
	if model.Language != "" {
		languages[model.Language] = struct{}{}
	}
	for _, comp := range model.Components {
		if comp.Language != "" {
			languages[comp.Language] = struct{}{}
		}
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if rel != "." && (ignoredExecutionDir(d.Name()) || d.Name() == ".octo") {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file is relevant under any of the detected languages or globally
		relevant := false
		for lang := range languages {
			if isExecutionRelevantFile(rel, lang) {
				relevant = true
				break
			}
		}
		if !relevant && isExecutionRelevantFile(rel, "") {
			relevant = true
		}

		if relevant {
			info, infoErr := d.Info()
			if infoErr == nil {
				snapshot[filepath.ToSlash(rel)] = info.ModTime().UnixNano()
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Also track any evidence paths explicitly declared in components
	for _, comp := range model.Components {
		for _, ev := range comp.Evidence {
			if ev.Path != "" && ev.Path != "." {
				full := filepath.Join(root, filepath.FromSlash(ev.Path))
				if info, err := os.Stat(full); err == nil && !info.IsDir() {
					rel, relErr := filepath.Rel(root, full)
					if relErr == nil && !strings.HasPrefix(rel, "..") {
						snapshot[filepath.ToSlash(rel)] = info.ModTime().UnixNano()
					}
				}
			}
		}
	}

	return snapshot, nil
}

// HasSnapshotChanged compares the current repository files with a previous snapshot.
func HasSnapshotChanged(root string, model ProjectModel, previous map[string]int64) (bool, map[string]int64, error) {
	current, err := TakeFileSnapshot(root, model)
	if err != nil {
		return false, previous, err
	}
	if len(current) != len(previous) {
		return true, current, nil
	}
	for file, modTime := range current {
		prevTime, exists := previous[file]
		if !exists || prevTime != modTime {
			return true, current, nil
		}
	}
	return false, previous, nil
}

// WatchForChanges periodically checks for execution-relevant file modifications.
// It returns true when a change is detected, or false when the context is cancelled.
func WatchForChanges(ctx context.Context, root string, model ProjectModel, interval time.Duration) (bool, error) {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}

	snapshot, err := TakeFileSnapshot(root, model)
	if err != nil {
		return false, err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, nil
		case <-ticker.C:
			changed, newSnapshot, err := HasSnapshotChanged(root, model, snapshot)
			if err != nil {
				continue
			}
			if changed {
				// Debounce: allow brief burst of file modifications to settle
				time.Sleep(150 * time.Millisecond)
				_ = newSnapshot
				return true, nil
			}
		}
	}
}
