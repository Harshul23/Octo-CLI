package intelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// JevDecisionProvider delegates bounded decisions to a Jev-compatible HTTP
// service. Jev receives only the decision name and evidence-backed options;
// it cannot introduce a new option.
type JevDecisionProvider struct {
	URL        string
	Token      string
	Client     *http.Client
}

// NewJevDecisionProviderFromEnv creates an optional Jev provider from:
//   OCTO_JEV_URL   — Jev decision endpoint
//   OCTO_JEV_TOKEN — optional bearer token
func NewJevDecisionProviderFromEnv() *JevDecisionProvider {
	url := strings.TrimSpace(os.Getenv("OCTO_JEV_URL"))
	if url == "" {
		return nil
	}
	return &JevDecisionProvider{
		URL:   strings.TrimRight(url, "/"),
		Token: strings.TrimSpace(os.Getenv("OCTO_JEV_TOKEN")),
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

type jevDecisionRequest struct {
	Name    string           `json:"name"`
	Options []DecisionOption `json:"options"`
}

func (p *JevDecisionProvider) Decide(ctx context.Context, request DecisionRequest) (DecisionResult, error) {
	if p == nil || strings.TrimSpace(p.URL) == "" {
		return DecisionResult{}, fmt.Errorf("jev decision provider is not configured")
	}
	if request.Name == "" {
		return DecisionResult{}, fmt.Errorf("decision name is required")
	}
	if len(request.Options) == 0 {
		return DecisionResult{}, fmt.Errorf("decision %q has no options", request.Name)
	}

	payload, err := json.Marshal(jevDecisionRequest{Name: request.Name, Options: request.Options})
	if err != nil {
		return DecisionResult{}, fmt.Errorf("encode Jev request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(payload))
	if err != nil {
		return DecisionResult{}, fmt.Errorf("create Jev request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}

	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return DecisionResult{}, fmt.Errorf("call Jev: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DecisionResult{}, fmt.Errorf("Jev returned HTTP %d", resp.StatusCode)
	}

	var result DecisionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return DecisionResult{}, fmt.Errorf("decode Jev response: %w", err)
	}

	for _, option := range request.Options {
		if option.ID != result.OptionID {
			continue
		}
		if option.Value != result.Value {
			return DecisionResult{}, fmt.Errorf("Jev changed the value of option %q", result.OptionID)
		}
		if result.Value == "" {
			return DecisionResult{}, fmt.Errorf("Jev selected option %q with an empty value", result.OptionID)
		}
		return result, nil
	}
	return DecisionResult{}, fmt.Errorf("Jev selected unknown option %q", result.OptionID)
}

// OptionalDecisionProvider returns Jev when explicitly enabled and falls back
// to the deterministic provider whenever Jev is unavailable or rejects a
// decision. This keeps the intelligence path fully usable offline.
func OptionalDecisionProvider() DecisionProvider {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("OCTO_DECISION_PROVIDER")), "jev") {
		if jev := NewJevDecisionProviderFromEnv(); jev != nil {
			return FallbackDecisionProvider{
				Primary:  jev,
				Fallback: DeterministicDecisionProvider{},
			}
		}
	}
	return DeterministicDecisionProvider{}
}

type FallbackDecisionProvider struct {
	Primary  DecisionProvider
	Fallback DecisionProvider
}

func (p FallbackDecisionProvider) Decide(ctx context.Context, request DecisionRequest) (DecisionResult, error) {
	if p.Primary != nil {
		if result, err := p.Primary.Decide(ctx, request); err == nil {
			return result, nil
		}
	}
	if p.Fallback == nil {
		return DecisionResult{}, fmt.Errorf("no decision provider available")
	}
	return p.Fallback.Decide(ctx, request)
}
