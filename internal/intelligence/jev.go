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

// ExternalDecisionProvider delegates bounded decisions to an external HTTP
// or typed LLM service. The endpoint receives only the decision name and bounded options,
// and can only pick from the bounded options without inventing commands.
type ExternalDecisionProvider struct {
	Name   string
	URL    string
	Token  string
	Client *http.Client
}

// NewExternalDecisionProviderFromEnv creates an optional external decision provider from:
//   OCTO_DECISION_URL   — external decision endpoint (or OCTO_LLM_URL)
//   OCTO_DECISION_TOKEN — optional bearer token (or OCTO_LLM_TOKEN)
func NewExternalDecisionProviderFromEnv() *ExternalDecisionProvider {
	url := strings.TrimSpace(os.Getenv("OCTO_DECISION_URL"))
	if url == "" {
		url = strings.TrimSpace(os.Getenv("OCTO_LLM_URL"))
	}
	if url == "" {
		return nil
	}
	token := strings.TrimSpace(os.Getenv("OCTO_DECISION_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("OCTO_LLM_TOKEN"))
	}
	return &ExternalDecisionProvider{
		Name:   "external",
		URL:    strings.TrimRight(url, "/"),
		Token:  token,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *ExternalDecisionProvider) Decide(ctx context.Context, request DecisionRequest) (DecisionResult, error) {
	if p == nil || strings.TrimSpace(p.URL) == "" {
		return DecisionResult{}, fmt.Errorf("external decision provider is not configured")
	}
	if request.Name == "" {
		return DecisionResult{}, fmt.Errorf("decision name is required")
	}
	if len(request.Options) == 0 {
		return DecisionResult{}, fmt.Errorf("decision %q has no options", request.Name)
	}

	payload, err := json.Marshal(jevDecisionRequest{Name: request.Name, Options: request.Options})
	if err != nil {
		return DecisionResult{}, fmt.Errorf("encode external decision request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(payload))
	if err != nil {
		return DecisionResult{}, fmt.Errorf("create external decision request: %w", err)
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
		return DecisionResult{}, fmt.Errorf("call external decision provider: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DecisionResult{}, fmt.Errorf("external decision provider returned HTTP %d", resp.StatusCode)
	}

	var result DecisionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return DecisionResult{}, fmt.Errorf("decode external decision response: %w", err)
	}

	for _, option := range request.Options {
		if option.ID != result.OptionID {
			continue
		}
		if option.Value != result.Value {
			return DecisionResult{}, fmt.Errorf("external provider changed the value of option %q", result.OptionID)
		}
		if result.Value == "" {
			return DecisionResult{}, fmt.Errorf("external provider selected option %q with an empty value", result.OptionID)
		}
		return result, nil
	}
	return DecisionResult{}, fmt.Errorf("external provider selected unknown option %q", result.OptionID)
}

// OptionalDecisionProvider returns Jev or external decision provider when explicitly enabled and falls back
// to the deterministic provider whenever external services are unavailable or reject a
// decision. This keeps the intelligence path fully usable offline.
func OptionalDecisionProvider() DecisionProvider {
	providerType := strings.ToLower(strings.TrimSpace(os.Getenv("OCTO_DECISION_PROVIDER")))
	if providerType == "jev" {
		if jev := NewJevDecisionProviderFromEnv(); jev != nil {
			return FallbackDecisionProvider{
				Primary:  jev,
				Fallback: DeterministicDecisionProvider{},
			}
		}
	}
	if providerType == "external" || providerType == "llm" || os.Getenv("OCTO_DECISION_URL") != "" || os.Getenv("OCTO_LLM_URL") != "" {
		if ext := NewExternalDecisionProviderFromEnv(); ext != nil {
			return FallbackDecisionProvider{
				Primary:  ext,
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
