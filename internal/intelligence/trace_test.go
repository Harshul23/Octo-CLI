package intelligence

import "testing"

func TestBuildDecisionTraceIncludesEvidenceAndPlanReasons(t *testing.T) {
	model := ProjectModel{
		Name: "demo",
		Language: "Node",
		PackageManager: "pnpm",
		RunCommand: "pnpm dev",
		Confidence: 0.9,
		Evidence: []Evidence{
			{Kind: EvidenceSignalFile, Path: "package.json", Detail: "Node signal", Strength: 0.85},
			{Kind: EvidenceLockfile, Path: "pnpm-lock.yaml", Detail: "pnpm lockfile", Strength: 0.95},
			{Kind: EvidenceScript, Path: "package.json", Detail: "dev script", Strength: 0.8},
		},
		Components: []Component{{
			Name: "demo", Path: ".", PackageManager: "pnpm", RunCommand: "pnpm dev",
		}},
	}
	plan, err := (DeterministicPlanner{}).Plan(nilContext{}, model)
	if err != nil {
		t.Fatal(err)
	}

	trace := BuildDecisionTrace(model, plan)
	if len(trace) == 0 {
		t.Fatal("expected decision trace")
	}

	var packageDecision, stepDecision bool
	for _, decision := range trace {
		if decision.Decision == "package_manager" {
			packageDecision = true
			if len(decision.Evidence) != 1 || decision.Evidence[0].Path != "pnpm-lock.yaml" {
				t.Fatalf("package decision=%+v", decision)
			}
		}
		if decision.Decision == "execution_step.component.demo.start" {
			stepDecision = true
			if decision.Reason == "" {
				t.Fatal("expected execution-step reason")
			}
		}
	}
	if !packageDecision || !stepDecision {
		t.Fatalf("trace=%+v", trace)
	}
}
