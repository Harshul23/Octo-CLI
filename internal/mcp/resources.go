package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/harshul/octo-cli/internal/intelligence"
)

func DefaultResources() []Resource {
	return []Resource{
		{
			URI:         "octo://topology",
			Name:        "Repository Topology",
			Description: "Unified dependency and relationship graph of components and infrastructure services.",
			MimeType:    "application/json",
		},
		{
			URI:         "octo://execution-plan",
			Name:        "Execution Plan",
			Description: "Deterministic step-by-step execution plan and topological order.",
			MimeType:    "application/json",
		},
		{
			URI:         "octo://lock",
			Name:        "Strategy Lockfile",
			Description: "Cache of verified execution candidates and content fingerprints.",
			MimeType:    "application/json",
		},
	}
}

func ReadResource(ctx context.Context, uriStr string) (ReadResourceResult, error) {
	parsed, err := url.Parse(uriStr)
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("invalid resource URI %q: %w", uriStr, err)
	}

	target := "."
	if p := parsed.Query().Get("path"); p != "" {
		target = p
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("resolve path %q: %w", target, err)
	}

	hostPath := parsed.Host + parsed.Path

	switch hostPath {
	case "topology":
		return readTopologyResource(target, uriStr)
	case "execution-plan":
		return readPlanResource(ctx, target, uriStr)
	case "lock":
		return readLockResource(target, uriStr)
	default:
		return ReadResourceResult{}, fmt.Errorf("resource not found: %s", uriStr)
	}
}

func readTopologyResource(target, uri string) (ReadResourceResult, error) {
	model, err := intelligence.Analyze(target)
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("analyze failed: %w", err)
	}

	graph, err := intelligence.BuildTopologyGraph(model)
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("build topology failed: %w", err)
	}

	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("marshal topology: %w", err)
	}

	return ReadResourceResult{
		Contents: []ResourceContent{{
			URI:      uri,
			MimeType: "application/json",
			Text:     string(data),
		}},
	}, nil
}

func readPlanResource(ctx context.Context, target, uri string) (ReadResourceResult, error) {
	model, err := intelligence.Analyze(target)
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("analyze failed: %w", err)
	}

	planner := intelligence.DeterministicPlanner{}
	plan, err := planner.Plan(ctx, model)
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("plan failed: %w", err)
	}

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("marshal plan: %w", err)
	}

	return ReadResourceResult{
		Contents: []ResourceContent{{
			URI:      uri,
			MimeType: "application/json",
			Text:     string(data),
		}},
	}, nil
}

func readLockResource(target, uri string) (ReadResourceResult, error) {
	lockPath := filepath.Join(target, ".octo.lock")
	if _, statErr := os.Stat(lockPath); os.IsNotExist(statErr) {
		return ReadResourceResult{
			Contents: []ResourceContent{{
				URI:      uri,
				MimeType: "application/json",
				Text:     `{"status":"no_lockfile"}`,
			}},
		}, nil
	}

	lock, err := intelligence.LoadOctoLock(target)
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("load lockfile: %w", err)
	}

	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return ReadResourceResult{}, fmt.Errorf("marshal lockfile: %w", err)
	}

	return ReadResourceResult{
		Contents: []ResourceContent{{
			URI:      uri,
			MimeType: "application/json",
			Text:     string(data),
		}},
	}, nil
}
