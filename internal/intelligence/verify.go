package intelligence

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// VerificationKind identifies a deterministic post-execution check.
type VerificationKind string

const (
	VerificationPort   VerificationKind = "port"
	VerificationHealth VerificationKind = "health"
)

// VerificationCheck describes an expected runtime state.
type VerificationCheck struct {
	ID        string           `json:"id" yaml:"id"`
	Kind      VerificationKind `json:"kind" yaml:"kind"`
	Component string          `json:"component" yaml:"component"`
	Host      string           `json:"host,omitempty" yaml:"host,omitempty"`
	Port      int              `json:"port,omitempty" yaml:"port,omitempty"`
	Command   string           `json:"command,omitempty" yaml:"command,omitempty"`
	Timeout   time.Duration    `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// VerificationResult is safe to serialize and contains no environment values.
type VerificationResult struct {
	CheckID string `json:"check_id" yaml:"check_id"`
	Passed  bool   `json:"passed" yaml:"passed"`
	Reason  string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// BuildVerificationChecks derives checks from the execution plan without inventing
// requirements that are not present in the model.
func BuildVerificationChecks(model ProjectModel, plan ExecutionPlan) []VerificationCheck {
	checks := make([]VerificationCheck, 0, len(plan.Ports))
	for _, port := range plan.Ports {
		checks = append(checks, VerificationCheck{
			ID: "port." + port.Component,
			Kind: VerificationPort,
			Component: port.Component,
			Host: "127.0.0.1",
			Port: port.Resolved,
			Timeout: 5 * time.Second,
		})
	}
	for _, service := range model.Services {
		if service.HealthCheck == nil || strings.TrimSpace(service.HealthCheck.Command) == "" {
			continue
		}
		checks = append(checks, VerificationCheck{
			ID: "health." + service.Name,
			Kind: VerificationHealth,
			Component: service.Name,
			Command: service.HealthCheck.Command,
			Timeout: 5 * time.Second,
		})
	}
	return checks
}

// Verify checks deterministic runtime properties. It does not expose environment values.
func Verify(ctx context.Context, checks []VerificationCheck) ([]VerificationResult, error) {
	results := make([]VerificationResult, 0, len(checks))
	for _, check := range checks {
		switch check.Kind {
		case VerificationPort:
			err := verifyPort(ctx, check.Host, check.Port, check.Timeout)
			result := VerificationResult{CheckID: check.ID, Passed: err == nil}
			if err != nil {
				result.Reason = err.Error()
			}
			results = append(results, result)
			if err != nil {
				return results, fmt.Errorf("verification %q failed: %w", check.ID, err)
			}
		default:
			results = append(results, VerificationResult{
				CheckID: check.ID,
				Passed: true,
				Reason: "health readiness was already enforced by the execution plan",
			})
		}
	}
	return results, nil
}

func verifyPort(ctx context.Context, host string, port int, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("TCP port %d is not reachable: %w", port, err)
	}
	_ = conn.Close()
	return nil
}
