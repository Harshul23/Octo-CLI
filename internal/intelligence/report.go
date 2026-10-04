package intelligence

import (
	"context"
	"fmt"
)

// StepStatus describes the outcome of one planned execution step.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepSucceeded StepStatus = "succeeded"
	StepSkipped   StepStatus = "skipped"
	StepFailed    StepStatus = "failed"
)

// ExecutionStepResult is safe to serialize and contains no command output or environment values.
type ExecutionStepResult struct {
	ID          string     `json:"id" yaml:"id"`
	Status      StepStatus `json:"status" yaml:"status"`
	Adapter     string     `json:"adapter,omitempty" yaml:"adapter,omitempty"`
	CandidateID string     `json:"candidate_id,omitempty" yaml:"candidate_id,omitempty"`
	Reason      string     `json:"reason,omitempty" yaml:"reason,omitempty"`
}

type ExecutionFailure struct {
	StepID      string   `json:"step_id" yaml:"step_id"`
	CandidateID string   `json:"candidate_id,omitempty" yaml:"candidate_id,omitempty"`
	Reason      string   `json:"reason" yaml:"reason"`
	Evidence    Evidence `json:"evidence" yaml:"evidence"`
}

// ExecutionReport is the canonical result of an intelligence execution.
type ExecutionReport struct {
	ProjectName   string               `json:"project_name" yaml:"project_name"`
	Confidence    float64              `json:"confidence" yaml:"confidence"`
	Plan          ExecutionPlan        `json:"plan" yaml:"plan"`
	Steps         []ExecutionStepResult `json:"steps" yaml:"steps"`
	Verification []VerificationResult `json:"verification,omitempty" yaml:"verification,omitempty"`
	Failures     []ExecutionFailure  `json:"failures,omitempty" yaml:"failures,omitempty"`
	Success      bool                 `json:"success" yaml:"success"`
	FailureReason string               `json:"failure_reason,omitempty" yaml:"failure_reason,omitempty"`
}

// ExecutePlanReport executes a plan and returns a serializable execution report.
func ExecutePlanReport(ctx context.Context, model ProjectModel, plan ExecutionPlan, resolver RuntimeResolver, env ResolvedEnvironment) ExecutionReport {
	report := ExecutionReport{
		ProjectName: model.Name,
		Confidence: model.Confidence,
		Plan: plan,
		Success: false,
		Steps: make([]ExecutionStepResult, 0, len(plan.Steps)),
	}

	if err := ValidateExecutionPlan(plan); err != nil {
		report.FailureReason = fmt.Sprintf("invalid execution plan: %v", err)
		return report
	}

	order, err := TopologicalOrder(plan)
	if err != nil {
		report.FailureReason = err.Error()
		return report
	}

	steps := make(map[string]ExecutionStep, len(plan.Steps))
	for _, step := range plan.Steps {
		steps[step.ID] = step
	}

	for _, id := range order {
		if err := ctx.Err(); err != nil {
			report.FailureReason = err.Error()
			report.Steps = append(report.Steps, ExecutionStepResult{ID: id, Status: StepFailed, Reason: err.Error()})
			return report
		}

		step := steps[id]
		if step.Command == "" {
			report.Steps = append(report.Steps, ExecutionStepResult{ID: id, Status: StepSkipped, Reason: "no executable command"})
			continue
		}

		candidates := executionCandidatesForStep(step)
		for _, candidate := range candidates {
			attempt := step
			attempt.Command = candidate.Command
			adapter, err := resolver.Resolve(attempt)
			if err != nil {
				report.FailureReason = err.Error()
				report.Steps = append(report.Steps, ExecutionStepResult{ID: id, Status: StepFailed, CandidateID: candidate.ID, Reason: err.Error()})
				return report
			}

			result := ExecutionStepResult{ID: id, Status: StepRunning, Adapter: adapter.Name(), CandidateID: candidate.ID}
			// Resolve the environment at the execution boundary so component-local
			// values are visible only to steps belonging to that component.
			stepEnv := env.ForStep(attempt)
			if err := adapter.Execute(ctx, attempt, stepEnv); err != nil {
				result.Status = StepFailed
				result.Reason = err.Error()
				report.Steps = append(report.Steps, result)
				report.Failures = append(report.Failures, ExecutionFailure{
					StepID: id, CandidateID: candidate.ID, Reason: err.Error(),
					Evidence: Evidence{Kind: EvidenceExecutionFailure, Path: id, Detail: "Execution candidate failed: " + err.Error(), Strength: 1},
				})
				report.FailureReason = err.Error()
				continue
			}
			result.Status = StepSucceeded
			report.Steps = append(report.Steps, result)
			goto stepSucceeded
		}
		return report

	stepSucceeded:
	}

	checks := BuildVerificationChecks(model, plan)
	verification, err := Verify(ctx, checks)
	report.Verification = verification
	if err != nil {
		report.FailureReason = err.Error()
		return report
	}

	report.Success = true
	return report
}

// ExecutePlan remains a small compatibility wrapper for callers that only need an error.
func ExecutePlan(ctx context.Context, plan ExecutionPlan, resolver RuntimeResolver, env ResolvedEnvironment) error {
	report := ExecutePlanReport(ctx, ProjectModel{Name: plan.ProjectName, Root: plan.Root}, plan, resolver, env)
	if !report.Success {
		return fmt.Errorf("%s", report.FailureReason)
	}
	return nil
}

func executionCandidatesForStep(step ExecutionStep) []ExecutionCandidate {
	if len(step.Candidates) == 0 {
		if step.Command == "" { return nil }
		return []ExecutionCandidate{{ID: step.SelectedCandidate, Command: step.Command}}
	}
	ordered := make([]ExecutionCandidate, 0, len(step.Candidates))
	for _, candidate := range step.Candidates {
		if candidate.ID == step.SelectedCandidate || (step.SelectedCandidate == "" && candidate.Command == step.Command) {
			ordered = append(ordered, candidate)
		}
	}
	for _, candidate := range step.Candidates {
		if candidate.ID == step.SelectedCandidate || (step.SelectedCandidate == "" && candidate.Command == step.Command) { continue }
		ordered = append(ordered, candidate)
	}
	return ordered
}
