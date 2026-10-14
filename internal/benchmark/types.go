package benchmark

import (
	"time"

	"github.com/Octo-sh/Octo-CLI/internal/intelligence"
)

// BenchmarkResult records the autonomous evaluation metrics for a single repository.
type BenchmarkResult struct {
	Repository    string                `json:"repository"`
	Archetype     string                `json:"archetype"`
	Timestamp     time.Time             `json:"timestamp"`
	OctoScore     ScoreMetrics          `json:"octo_score"`
	BaselineScore ScoreMetrics          `json:"baseline_score"`
	Comparison    ComparisonMetrics     `json:"comparison"`
	Diagnostics   *ExecutionDiagnostics `json:"diagnostics,omitempty"`
}

// ScoreMetrics captures execution efficiency, reliability, and safety.
type ScoreMetrics struct {
	OneShotSuccess       bool    `json:"one_shot_success"`
	HallucinationRate    float64 `json:"hallucination_rate"`     // 0.0 - 1.0 (fraction of invented / invalid commands)
	CandidateConfidence  float64 `json:"candidate_confidence"`   // 0.0 - 1.0
	LatencyMs            int64   `json:"latency_ms"`             // Planning & discovery latency in ms
	EstimatedTokens      int     `json:"estimated_tokens"`       // Estimated context tokens consumed
	VerifiedReadiness    bool    `json:"verified_readiness"`     // Whether runtime readiness was deterministically verified
	ServicesResolved     int     `json:"services_resolved"`      // Infrastructure backing services identified & ordered
	PortConflictsAvoided int     `json:"port_conflicts_avoided"` // Dynamic port shifts performed
	EvidenceSignals      int     `json:"evidence_signals"`       // Number of observable facts gathered
}

// ComparisonMetrics shows Octo's measured improvement over the raw agent shell baseline.
type ComparisonMetrics struct {
	TokenReductionPct float64 `json:"token_reduction_pct"` // % context tokens saved
	LatencySpeedupX   float64 `json:"latency_speedup_x"`   // Speedup multiplier
	SafetyAdvantage   string  `json:"safety_advantage"`    // e.g. "Zero hallucinations, port conflict avoided"
}

// BenchmarkSuiteReport compiles evaluation results across an archetype corpus.
type BenchmarkSuiteReport struct {
	SuiteName                  string            `json:"suite_name"`
	Timestamp                  time.Time         `json:"timestamp"`
	TotalCases                 int               `json:"total_cases"`
	OctoPassRate               float64           `json:"octo_pass_rate"`     // pass@1 percentage (e.g. 100.0)
	BaselinePassRate           float64           `json:"baseline_pass_rate"` // baseline pass@1 percentage (e.g. 35.0)
	AvgTokenSavingsPct         float64           `json:"avg_token_savings_pct"`
	AvgLatencyMs               int64             `json:"avg_latency_ms"`
	ZeroHallucinationConfirmed bool              `json:"zero_hallucination_confirmed"`
	Results                    []BenchmarkResult `json:"results"`
}

// ExecutionDiagnostics preserves failed execution evidence without serializing
// the project model, environment, or entire execution plan.
type ExecutionDiagnostics struct {
	FailureReason string                             `json:"failure_reason"`
	ContextError  string                             `json:"context_error,omitempty"`
	Steps         []intelligence.ExecutionStepResult `json:"steps,omitempty"`
	Verification  []intelligence.VerificationResult  `json:"verification,omitempty"`
}
