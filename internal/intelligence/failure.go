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

// FailureClassification categorizes execution and startup failures into structured classes.
type FailureClassification string

const (
	FailureClassPortConflict      FailureClassification = "port_conflict"
	FailureClassMissingDependency FailureClassification = "missing_dependency"
	FailureClassCompilationSyntax FailureClassification = "compilation_syntax"
	FailureClassProcessCrash      FailureClassification = "process_crash"
	FailureClassTimeout           FailureClassification = "timeout"
	FailureClassUnknown           FailureClassification = "unknown"
)

// ClassifyExecutionFailure inspects failure messages and errors to determine the failure category.
func ClassifyExecutionFailure(reason string, err error) FailureClassification {
	msg := reason
	if err != nil {
		if msg == "" {
			msg = err.Error()
		} else {
			msg = msg + " " + err.Error()
		}
	}
	lower := strings.ToLower(msg)

	// 1. Port conflicts
	if strings.Contains(lower, "address already in use") ||
		strings.Contains(lower, "port is not reachable") ||
		strings.Contains(lower, "bind: address already in use") ||
		strings.Contains(lower, "port conflict") ||
		strings.Contains(lower, "port collision") ||
		(strings.Contains(lower, "startup verification failed") && strings.Contains(lower, "port")) {
		return FailureClassPortConflict
	}

	// 2. Missing dependencies & missing executables
	if strings.Contains(lower, "cannot find module") ||
		strings.Contains(lower, "modulenotfounderror") ||
		strings.Contains(lower, "no module named") ||
		strings.Contains(lower, "package not found") ||
		strings.Contains(lower, "command not found") ||
		strings.Contains(lower, "not installed") ||
		strings.Contains(lower, "executable file not found") ||
		strings.Contains(lower, "missing dependency") ||
		strings.Contains(lower, "could not find a declaration file") ||
		strings.Contains(lower, "import error") ||
		strings.Contains(lower, "importerror") ||
		(strings.Contains(lower, "no such file or directory") && (strings.Contains(lower, "bin") || strings.Contains(lower, "node_modules"))) {
		return FailureClassMissingDependency
	}

	// 3. Compilation & syntax errors
	if strings.Contains(lower, "syntaxerror") ||
		strings.Contains(lower, "syntax error") ||
		strings.Contains(lower, "compilation error") ||
		strings.Contains(lower, "compile error") ||
		strings.Contains(lower, "build failed") ||
		strings.Contains(lower, "typeerror") ||
		strings.Contains(lower, "type error") ||
		strings.Contains(lower, "undefined reference") ||
		strings.Contains(lower, "cannot find symbol") ||
		strings.Contains(lower, "parse error") ||
		strings.Contains(lower, "parseerror") {
		return FailureClassCompilationSyntax
	}

	// 4. Timeouts & deadline exceeded
	if strings.Contains(lower, "deadline exceeded") ||
		strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "timeout") {
		return FailureClassTimeout
	}

	// 5. Process crashes & abrupt exits
	if strings.Contains(lower, "exit status") ||
		strings.Contains(lower, "segmentation fault") ||
		strings.Contains(lower, "sigsegv") ||
		strings.Contains(lower, "signal: killed") ||
		strings.Contains(lower, "signal: terminated") ||
		strings.Contains(lower, "crashed") ||
		strings.Contains(lower, "panic:") {
		return FailureClassProcessCrash
	}

	return FailureClassUnknown
}
