package intelligence

import (
    "errors"
    "strings"
)

// FailureCode identifies a stable, machine-readable execution failure.
type FailureCode string

const (
    FailureNoExecutableCandidates FailureCode = "NO_EXECUTABLE_CANDIDATES"
    FailureNoRunCommand           FailureCode = "NO_RUN_COMMAND"
    FailureInvalidProject         FailureCode = "INVALID_PROJECT"
    FailureInvalidComponent       FailureCode = "INVALID_COMPONENT"
    FailureInvalidService         FailureCode = "INVALID_SERVICE"
    FailureTopologyBuild          FailureCode = "TOPOLOGY_BUILD_FAILED"
    FailureDecision               FailureCode = "DECISION_FAILED"
    FailurePlanValidation         FailureCode = "PLAN_VALIDATION_FAILED"
)

// FailureReason carries structured context while keeping CLI explanations human-readable.
type FailureReason struct {
    Code      FailureCode
    Component string
    Summary   string
    Details   []string
}

func (f *FailureReason) Error() string {
    if f == nil {
        return ""
    }
    var b strings.Builder
    b.WriteString(f.Summary)
    for _, detail := range f.Details {
        if strings.TrimSpace(detail) == "" {
            continue
        }
        b.WriteString("\n  ")
        b.WriteString(detail)
    }
    return b.String()
}

// AsFailure extracts structured failure data through wrapped errors.
func AsFailure(err error) (*FailureReason, bool) {
    var failure *FailureReason
    if errors.As(err, &failure) {
        return failure, true
    }
    return nil, false
}

func newFailure(code FailureCode, component, summary string, details ...string) error {
    return &FailureReason{Code: code, Component: component, Summary: summary, Details: details}
}
