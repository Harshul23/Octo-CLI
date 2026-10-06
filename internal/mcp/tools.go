package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/harshul/octo-cli/internal/intelligence"
)

func DefaultTools() []Tool {
	return []Tool{
		{
			Name:        "octo_inspect",
			Description: "Analyze a repository to detect language, framework, runtime version, package manager, services, and execution confidence.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Path to the repository directory (defaults to current working directory).",
					},
				},
			},
		},
		{
			Name:        "octo_topology",
			Description: "Return the validated dependency and relationship graph of components and infrastructure services.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Path to the repository directory (defaults to current working directory).",
					},
				},
			},
		},
		{
			Name:        "octo_plan",
			Description: "Generate the deterministic multi-step execution plan for a repository without executing it.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Path to the repository directory (defaults to current working directory).",
					},
				},
			},
		},
		{
			Name:        "octo_run_and_verify",
			Description: "Execute the deterministic plan for a repository, verifying startup and health, and capturing active processes.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Path to the repository directory (defaults to current working directory).",
					},
					"detach": {
						Type:        "boolean",
						Description: "Run long-running services in the background (detached mode).",
					},
				},
			},
		},
		{
			Name:        "octo_verify",
			Description: "Verify expected runtime state (ports, healthchecks) and inspect or manage .octo.lock verified state.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Path to the repository directory (defaults to current working directory).",
					},
				},
			},
		},
		{
			Name:        "octo_diagnose",
			Description: "Explain the evidence behind decisions and analyze failures for a repository.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Path to the repository directory (defaults to current working directory).",
					},
				},
			},
		},
		{
			Name:        "octo_env",
			Description: "Inspect present and missing environment variables, and generate safe non-secret configuration templates.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Path to the repository directory (defaults to current working directory).",
					},
				},
			},
		},
		{
			Name:        "octo_preview",
			Description: "Preview all machine changes, filesystem mutations, port allocations, and processes before execution.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {
						Type:        "string",
						Description: "Path to the repository directory (defaults to current working directory).",
					},
				},
			},
		},
	}
}

func ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (CallToolResult, error) {
	switch name {
	case "octo_inspect":
		return handleInspect(ctx, args)
	case "octo_topology":
		return handleTopology(ctx, args)
	case "octo_plan":
		return handlePlan(ctx, args)
	case "octo_run_and_verify":
		return handleRunAndVerify(ctx, args)
	case "octo_verify":
		return handleVerify(ctx, args)
	case "octo_diagnose":
		return handleDiagnose(ctx, args)
	case "octo_env":
		return handleEnv(ctx, args)
	case "octo_preview":
		return handlePreview(ctx, args)
	default:
		return CallToolResult{
			IsError: true,
			Content: []ContentItem{{
				Type: "text",
				Text: fmt.Sprintf("unknown tool %q", name),
			}},
		}, nil
	}
}

func handleInspect(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	target, err := resolvePath(args)
	if err != nil {
		return toolError(err.Error()), nil
	}

	model, err := intelligence.Analyze(target)
	if err != nil {
		return toolError(fmt.Sprintf("analysis failed: %v", err)), nil
	}

	data, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to serialize model: %v", err)), nil
	}

	return CallToolResult{
		Content: []ContentItem{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func handlePlan(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	target, err := resolvePath(args)
	if err != nil {
		return toolError(err.Error()), nil
	}

	model, err := intelligence.Analyze(target)
	if err != nil {
		return toolError(fmt.Sprintf("analysis failed: %v", err)), nil
	}

	planner := intelligence.DeterministicPlanner{}
	plan, err := planner.Plan(ctx, model)
	if err != nil {
		return toolError(fmt.Sprintf("planning failed: %v", err)), nil
	}

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to serialize plan: %v", err)), nil
	}

	return CallToolResult{
		Content: []ContentItem{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func handleRunAndVerify(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	target, err := resolvePath(args)
	if err != nil {
		return toolError(err.Error()), nil
	}

	detach := false
	if d, ok := args["detach"].(bool); ok {
		detach = d
	}

	model, err := intelligence.Analyze(target)
	if err != nil {
		return toolError(fmt.Sprintf("analysis failed: %v", err)), nil
	}

	lock, err := intelligence.LoadOctoLock(target)
	if err != nil {
		return toolError(fmt.Sprintf("failed to read lockfile: %v", err)), nil
	}

	_, _ = intelligence.ApplyVerifiedStrategies(target, &model, lock)

	planner := intelligence.DeterministicPlanner{}
	plan, err := planner.Plan(ctx, model)
	if err != nil {
		return toolError(fmt.Sprintf("planning failed: %v", err)), nil
	}

	env, err := intelligence.ResolveProjectEnvironment(target, model)
	if err != nil {
		return toolError(fmt.Sprintf("environment resolution failed: %v", err)), nil
	}

	opts := intelligence.ExecutionOptions{
		Detach: detach,
		Silent: true,
	}

	report := intelligence.ExecutePlanReportWithOptions(ctx, model, plan, intelligence.NewRuntimeResolver(), env, opts)

	if report.Success {
		_ = intelligence.RecordVerifiedStrategies(target, model, plan, report, lock)
		if detach {
			_ = intelligence.SaveDetachedState(target, report)
		}
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to serialize report: %v", err)), nil
	}

	return CallToolResult{
		IsError: !report.Success,
		Content: []ContentItem{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

type DiagnosisResult struct {
	ProjectName     string                         `json:"project_name"`
	Language        string                         `json:"language"`
	Framework       string                         `json:"framework,omitempty"`
	Confidence      float64                        `json:"confidence"`
	Decisions       []intelligence.DecisionTrace   `json:"decisions"`
	Evidence        []intelligence.Evidence        `json:"evidence"`
	ActiveProcesses []intelligence.ActiveProcess   `json:"active_processes,omitempty"`
}

func handleDiagnose(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	target, err := resolvePath(args)
	if err != nil {
		return toolError(err.Error()), nil
	}

	model, err := intelligence.Analyze(target)
	if err != nil {
		return toolError(fmt.Sprintf("analysis failed: %v", err)), nil
	}

	planner := intelligence.DeterministicPlanner{}
	plan, err := planner.Plan(ctx, model)
	if err != nil {
		return toolError(fmt.Sprintf("planning failed: %v", err)), nil
	}

	decisions := intelligence.BuildDecisionTrace(model, plan)
	diagnosis := DiagnosisResult{
		ProjectName: model.Name,
		Language:    model.Language,
		Framework:   model.Framework,
		Confidence:  model.Confidence,
		Decisions:   decisions,
		Evidence:    model.Evidence,
	}

	data, err := json.MarshalIndent(diagnosis, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to serialize diagnosis: %v", err)), nil
	}

	return CallToolResult{
		Content: []ContentItem{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func handleTopology(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	target, err := resolvePath(args)
	if err != nil {
		return toolError(err.Error()), nil
	}

	model, err := intelligence.Analyze(target)
	if err != nil {
		return toolError(fmt.Sprintf("analysis failed: %v", err)), nil
	}

	graph, err := intelligence.BuildTopologyGraph(model)
	if err != nil {
		return toolError(fmt.Sprintf("topology construction failed: %v", err)), nil
	}

	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to serialize topology: %v", err)), nil
	}

	return CallToolResult{
		Content: []ContentItem{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

type VerificationReport struct {
	ProjectName string                            `json:"project_name"`
	Success     bool                              `json:"success"`
	Checks      []intelligence.VerificationResult `json:"checks,omitempty"`
	LockPresent bool                              `json:"lock_present"`
	LockEntries int                               `json:"lock_entries,omitempty"`
	Error       string                            `json:"error,omitempty"`
}

func handleVerify(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	target, err := resolvePath(args)
	if err != nil {
		return toolError(err.Error()), nil
	}

	model, err := intelligence.Analyze(target)
	if err != nil {
		return toolError(fmt.Sprintf("analysis failed: %v", err)), nil
	}

	planner := intelligence.DeterministicPlanner{}
	plan, err := planner.Plan(ctx, model)
	if err != nil {
		return toolError(fmt.Sprintf("planning failed: %v", err)), nil
	}

	checks := intelligence.BuildVerificationChecks(model, plan)
	results, verifyErr := intelligence.Verify(ctx, checks)

	lock, _ := intelligence.LoadOctoLock(target)
	lockPresent := len(lock.Strategies) > 0

	report := VerificationReport{
		ProjectName: model.Name,
		Success:     verifyErr == nil,
		Checks:      results,
		LockPresent: lockPresent,
		LockEntries: len(lock.Strategies),
	}
	if verifyErr != nil {
		report.Error = verifyErr.Error()
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to serialize verification report: %v", err)), nil
	}

	return CallToolResult{
		IsError: verifyErr != nil,
		Content: []ContentItem{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func handleEnv(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	target, err := resolvePath(args)
	if err != nil {
		return toolError(err.Error()), nil
	}

	model, err := intelligence.Analyze(target)
	if err != nil {
		return toolError(fmt.Sprintf("analysis failed: %v", err)), nil
	}

	status := intelligence.InspectEnvironmentStatus(target, model)
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to serialize environment status: %v", err)), nil
	}

	return CallToolResult{
		Content: []ContentItem{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func handlePreview(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	target, err := resolvePath(args)
	if err != nil {
		return toolError(err.Error()), nil
	}

	model, err := intelligence.Analyze(target)
	if err != nil {
		return toolError(fmt.Sprintf("analysis failed: %v", err)), nil
	}

	planner := intelligence.DeterministicPlanner{}
	plan, err := planner.Plan(ctx, model)
	if err != nil {
		return toolError(fmt.Sprintf("planning failed: %v", err)), nil
	}

	preview := intelligence.BuildMachinePreview(ctx, model, plan)
	data, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("failed to serialize machine preview: %v", err)), nil
	}

	return CallToolResult{
		Content: []ContentItem{{
			Type: "text",
			Text: string(data),
		}},
	}, nil
}

func resolvePath(args map[string]interface{}) (string, error) {
	p := "."
	if val, ok := args["path"].(string); ok && val != "" {
		p = val
	}
	return filepath.Abs(p)
}

func toolError(msg string) CallToolResult {
	return CallToolResult{
		IsError: true,
		Content: []ContentItem{{
			Type: "text",
			Text: msg,
		}},
	}
}
