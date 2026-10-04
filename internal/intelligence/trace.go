package intelligence

import (
	"fmt"
)

// DecisionTrace records why Octo made important repository and execution decisions.
type DecisionTrace struct {
	Decision string   `json:"decision" yaml:"decision"`
	Value    string   `json:"value" yaml:"value"`
	Reason   string   `json:"reason,omitempty" yaml:"reason,omitempty"`
	Evidence []Evidence `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	Confidence float64 `json:"confidence" yaml:"confidence"`
}

// BuildDecisionTrace derives explainable decisions from the ProjectModel and plan.
func BuildDecisionTrace(model ProjectModel, plan ExecutionPlan) []DecisionTrace {
	trace := make([]DecisionTrace, 0, 8)

	if model.Language != "" {
		trace = append(trace, DecisionTrace{
			Decision: "language", Value: model.Language,
			Reason: "selected from repository language signals",
			Evidence: filterEvidence(model.Evidence, EvidenceSignalFile),
			Confidence: model.Confidence,
		})
	}
	if model.PackageManager != "" {
		trace = append(trace, DecisionTrace{
			Decision: "package_manager", Value: model.PackageManager,
			Reason: "selected from repository manifests and lockfiles",
			Evidence: filterEvidence(model.Evidence, EvidenceLockfile),
			Confidence: model.Confidence,
		})
	}
	if model.RunCommand != "" {
		trace = append(trace, DecisionTrace{
			Decision: "run_command", Value: model.RunCommand,
			Reason: "selected from the repository's detected run script",
			Evidence: filterEvidence(model.Evidence, EvidenceScript),
			Confidence: model.Confidence,
		})
	}
	if model.Framework != "" {
		trace = append(trace, DecisionTrace{
			Decision: "framework", Value: model.Framework,
			Reason: "framework dependency detected in repository metadata",
			Evidence: filterEvidence(model.Evidence, EvidenceManifest),
			Confidence: model.Confidence,
		})
	}

	for _, step := range plan.Steps {
		if step.Explanation == "" {
			continue
		}
		trace = append(trace, DecisionTrace{
			Decision: "execution_step." + step.ID,
			Value: step.Command,
			Reason: step.Explanation,
			Confidence: 1,
		})
	}

	for _, port := range plan.Ports {
		reason := fmt.Sprintf("requested port %d; resolved port %d", port.Requested, port.Resolved)
		if !port.Automatic {
			reason = fmt.Sprintf("requested port %d was available", port.Requested)
		}
		trace = append(trace, DecisionTrace{
			Decision: "port." + port.Component,
			Value: fmt.Sprintf("%d", port.Resolved),
			Reason: reason,
			Confidence: 1,
		})
	}

	return trace
}

func filterEvidence(all []Evidence, kind EvidenceKind) []Evidence {
	out := make([]Evidence, 0)
	for _, evidence := range all {
		if evidence.Kind == kind {
			out = append(out, evidence)
		}
	}
	return out
}
