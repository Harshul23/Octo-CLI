package intelligence

import (
	"errors"
	"testing"
)

func TestClassifyExecutionFailure(t *testing.T) {
	tests := []struct {
		name     string
		reason   string
		err      error
		expected FailureClassification
	}{
		{
			name:     "port already in use",
			reason:   "listen tcp :8080: bind: address already in use",
			err:      nil,
			expected: FailureClassPortConflict,
		},
		{
			name:     "port unreachable verification",
			reason:   "Startup verification failed: port is not reachable",
			err:      errors.New("timeout connecting to 127.0.0.1:3000"),
			expected: FailureClassPortConflict,
		},
		{
			name:     "node missing module",
			reason:   "Error: Cannot find module 'express'",
			err:      nil,
			expected: FailureClassMissingDependency,
		},
		{
			name:     "python module not found",
			reason:   "ModuleNotFoundError: No module named 'fastapi'",
			err:      nil,
			expected: FailureClassMissingDependency,
		},
		{
			name:     "command not found",
			reason:   "sh: nodemon: command not found",
			err:      nil,
			expected: FailureClassMissingDependency,
		},
		{
			name:     "go syntax error",
			reason:   "syntax error: unexpected newline, expecting comma or }",
			err:      nil,
			expected: FailureClassCompilationSyntax,
		},
		{
			name:     "typescript type error",
			reason:   "TypeError: Property 'foo' does not exist on type 'Bar'",
			err:      nil,
			expected: FailureClassCompilationSyntax,
		},
		{
			name:     "context deadline exceeded",
			reason:   "context deadline exceeded",
			err:      errors.New("timeout waiting for healthcheck"),
			expected: FailureClassTimeout,
		},
		{
			name:     "process crash exit status",
			reason:   "process terminated with exit status 137",
			err:      nil,
			expected: FailureClassProcessCrash,
		},
		{
			name:     "segmentation fault",
			reason:   "segmentation fault (core dumped)",
			err:      nil,
			expected: FailureClassProcessCrash,
		},
		{
			name:     "unknown generic failure",
			reason:   "something completely unrecognized happened",
			err:      nil,
			expected: FailureClassUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyExecutionFailure(tt.reason, tt.err)
			if got != tt.expected {
				t.Fatalf("ClassifyExecutionFailure() = %q, want %q", got, tt.expected)
			}
		})
	}
}
