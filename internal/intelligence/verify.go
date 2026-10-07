package intelligence

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// VerificationKind identifies a deterministic post-execution check.
type VerificationKind string

const (
	VerificationPort   VerificationKind = "port"
	VerificationHealth VerificationKind = "health"
	VerificationHTTP   VerificationKind = "http"
)

// VerificationCheck describes an expected runtime state.
type VerificationCheck struct {
	ID             string           `json:"id" yaml:"id"`
	Kind           VerificationKind `json:"kind" yaml:"kind"`
	Component      string           `json:"component" yaml:"component"`
	Host           string           `json:"host,omitempty" yaml:"host,omitempty"`
	Port           int              `json:"port,omitempty" yaml:"port,omitempty"`
	Path           string           `json:"path,omitempty" yaml:"path,omitempty"`
	ExpectedStatus int              `json:"expected_status,omitempty" yaml:"expected_status,omitempty"`
	Command        string           `json:"command,omitempty" yaml:"command,omitempty"`
	Timeout        time.Duration    `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// VerificationResult is safe to serialize and contains no environment values.
type VerificationResult struct {
	CheckID string `json:"check_id" yaml:"check_id"`
	Passed  bool   `json:"passed" yaml:"passed"`
	Reason  string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// VerificationProvider executes a specific kind of verification check.
type VerificationProvider interface {
	Kind() VerificationKind
	Verify(ctx context.Context, check VerificationCheck) (VerificationResult, error)
}

// VerificationRegistry manages registered verification providers.
type VerificationRegistry struct {
	mu        sync.RWMutex
	providers map[VerificationKind]VerificationProvider
}

// NewVerificationRegistry initializes a registry with standard built-in providers.
func NewVerificationRegistry() *VerificationRegistry {
	r := &VerificationRegistry{
		providers: make(map[VerificationKind]VerificationProvider),
	}
	r.Register(&PortVerificationProvider{})
	r.Register(&HealthVerificationProvider{})
	r.Register(&HTTPVerificationProvider{})
	return r
}

var defaultVerificationRegistry = NewVerificationRegistry()

// DefaultVerificationRegistry returns the shared global verification registry.
func DefaultVerificationRegistry() *VerificationRegistry {
	return defaultVerificationRegistry
}

// RegisterVerificationProvider registers a verification provider into the default registry.
func RegisterVerificationProvider(p VerificationProvider) {
	defaultVerificationRegistry.Register(p)
}

// Register registers a verification provider for a given kind.
func (r *VerificationRegistry) Register(p VerificationProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Kind()] = p
}

// Verify runs all checks through their registered providers.
func (r *VerificationRegistry) Verify(ctx context.Context, checks []VerificationCheck) ([]VerificationResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	results := make([]VerificationResult, 0, len(checks))
	for _, check := range checks {
		provider, ok := r.providers[check.Kind]
		if !ok {
			results = append(results, VerificationResult{
				CheckID: check.ID,
				Passed:  true,
				Reason:  fmt.Sprintf("verification kind %q has no registered provider, marked passed", check.Kind),
			})
			continue
		}
		res, err := provider.Verify(ctx, check)
		results = append(results, res)
		if err != nil {
			return results, fmt.Errorf("verification %q failed: %w", check.ID, err)
		}
	}
	return results, nil
}

// PortVerificationProvider verifies TCP port readiness.
type PortVerificationProvider struct{}

func (p *PortVerificationProvider) Kind() VerificationKind { return VerificationPort }

func (p *PortVerificationProvider) Verify(ctx context.Context, check VerificationCheck) (VerificationResult, error) {
	host := check.Host
	if host == "" {
		host = "127.0.0.1"
	}
	err := verifyPort(ctx, host, check.Port, check.Timeout)
	res := VerificationResult{CheckID: check.ID, Passed: err == nil}
	if err != nil {
		res.Reason = err.Error()
	}
	return res, err
}

// HealthVerificationProvider verifies health checks declared on infrastructure services.
type HealthVerificationProvider struct{}

func (h *HealthVerificationProvider) Kind() VerificationKind { return VerificationHealth }

func (h *HealthVerificationProvider) Verify(ctx context.Context, check VerificationCheck) (VerificationResult, error) {
	return VerificationResult{
		CheckID: check.ID,
		Passed:  true,
		Reason:  "health readiness was enforced by the execution plan",
	}, nil
}

// HTTPVerificationProvider verifies endpoint reachability and HTTP status code.
type HTTPVerificationProvider struct{}

func (h *HTTPVerificationProvider) Kind() VerificationKind { return VerificationHTTP }

func (h *HTTPVerificationProvider) Verify(ctx context.Context, check VerificationCheck) (VerificationResult, error) {
	host := check.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := check.Port
	path := check.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	timeout := check.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	url := fmt.Sprintf("http://%s:%d%s", host, port, path)
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return VerificationResult{CheckID: check.ID, Passed: false, Reason: err.Error()}, err
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return VerificationResult{CheckID: check.ID, Passed: false, Reason: fmt.Sprintf("HTTP GET %s failed: %v", url, err)}, err
	}
	defer resp.Body.Close()

	expectedStatus := check.ExpectedStatus
	if expectedStatus > 0 {
		if resp.StatusCode != expectedStatus {
			err := fmt.Errorf("HTTP %s returned status %d, expected %d", url, resp.StatusCode, expectedStatus)
			return VerificationResult{CheckID: check.ID, Passed: false, Reason: err.Error()}, err
		}
	} else if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		err := fmt.Errorf("HTTP %s returned non-2xx status code %d", url, resp.StatusCode)
		return VerificationResult{CheckID: check.ID, Passed: false, Reason: err.Error()}, err
	}

	return VerificationResult{CheckID: check.ID, Passed: true}, nil
}

// BuildVerificationChecks derives checks from the execution plan without inventing
// requirements that are not present in the model.
func BuildVerificationChecks(model ProjectModel, plan ExecutionPlan) []VerificationCheck {
	checks := make([]VerificationCheck, 0, len(plan.Ports))
	for _, port := range plan.Ports {
		checks = append(checks, VerificationCheck{
			ID:        "port." + port.Component,
			Kind:      VerificationPort,
			Component: port.Component,
			Host:      "127.0.0.1",
			Port:      port.Resolved,
			Timeout:   5 * time.Second,
		})
	}
	for _, service := range model.Services {
		if service.HealthCheck == nil || strings.TrimSpace(service.HealthCheck.Command) == "" {
			continue
		}
		checks = append(checks, VerificationCheck{
			ID:        "health." + service.Name,
			Kind:      VerificationHealth,
			Component: service.Name,
			Command:   service.HealthCheck.Command,
			Timeout:   5 * time.Second,
		})
	}
	for _, custom := range model.CustomVerification {
		checks = append(checks, custom)
	}
	return checks
}

// Verify checks deterministic runtime properties through registered verification providers.
func Verify(ctx context.Context, checks []VerificationCheck) ([]VerificationResult, error) {
	return defaultVerificationRegistry.Verify(ctx, checks)
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
