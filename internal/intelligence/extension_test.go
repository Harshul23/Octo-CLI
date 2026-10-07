package intelligence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

type customTestDetector struct{}

func (c *customTestDetector) Detect(path string) (DetectedProject, error) {
	if _, err := os.Stat(filepath.Join(path, "custom.manifest")); err == nil {
		return DetectedProject{
			Name:       "custom-app",
			Language:   "CustomLang",
			RunCommand: "custom-run",
			Port:       9000,
		}, nil
	}
	return DetectedProject{}, nil
}

func TestCustomProjectDetectorRegistration(t *testing.T) {
	registry := NewProjectDetectorRegistry()
	registry.RegisterDetector(&customTestDetector{})

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "custom.manifest"), []byte("manifest"), 0644); err != nil {
		t.Fatal(err)
	}

	proj, err := registry.Detect(root)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if proj.Language != "CustomLang" || proj.RunCommand != "custom-run" {
		t.Fatalf("unexpected detected project: %+v", proj)
	}
}

type customTestCandidateProvider struct{}

func (c *customTestCandidateProvider) Name() string { return "custom" }
func (c *customTestCandidateProvider) Supports(comp Component) bool {
	return comp.Language == "CustomLang"
}
func (c *customTestCandidateProvider) Candidates(_ context.Context, _ string, _ Component) ([]ExecutionCandidate, error) {
	return []ExecutionCandidate{
		{
			ID:         "custom.start",
			Command:    "custom-cli start --prod",
			Confidence: 0.99,
		},
	}, nil
}

func TestCustomCandidateProviderRegistration(t *testing.T) {
	registry := NewCandidateProviderRegistry()
	registry.Register(&customTestCandidateProvider{})

	providers := ExecutionCandidateProviders{providers: registry.Providers()}
	candidates, err := providers.Candidates(context.Background(), ".", Component{Language: "CustomLang"})
	if err != nil {
		t.Fatalf("Candidates failed: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Command != "custom-cli start --prod" {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
}

func TestHTTPVerificationProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}

	provider := &HTTPVerificationProvider{}

	// Successful check
	check := VerificationCheck{
		ID:             "web.health",
		Kind:           VerificationHTTP,
		Host:           u.Hostname(),
		Port:           port,
		Path:           "/healthz",
		ExpectedStatus: 200,
		Timeout:        2 * time.Second,
	}

	res, err := provider.Verify(context.Background(), check)
	if err != nil || !res.Passed {
		t.Fatalf("expected HTTP verification to pass, err=%v, res=%+v", err, res)
	}

	// Failing check with non-matching status
	checkFail := VerificationCheck{
		ID:             "web.health.fail",
		Kind:           VerificationHTTP,
		Host:           u.Hostname(),
		Port:           port,
		Path:           "/not-found",
		ExpectedStatus: 200,
		Timeout:        2 * time.Second,
	}

	resFail, err := provider.Verify(context.Background(), checkFail)
	if err == nil || resFail.Passed {
		t.Fatalf("expected HTTP verification to fail on 404, got err=%v", err)
	}
}

func TestCustomVerificationProviderRegistration(t *testing.T) {
	registry := NewVerificationRegistry()

	customProviderCalled := false
	customProvider := &mockCustomVerificationProvider{called: &customProviderCalled}
	registry.Register(customProvider)

	checks := []VerificationCheck{
		{
			ID:   "custom.check",
			Kind: "custom_kind",
		},
	}

	results, err := registry.Verify(context.Background(), checks)
	if err != nil {
		t.Fatalf("registry.Verify failed: %v", err)
	}
	if !customProviderCalled {
		t.Fatal("expected custom verification provider to be called")
	}
	if len(results) != 1 || !results[0].Passed {
		t.Fatalf("unexpected results: %+v", results)
	}
}

type mockCustomVerificationProvider struct {
	called *bool
}

func (m *mockCustomVerificationProvider) Kind() VerificationKind {
	return VerificationKind("custom_kind")
}

func (m *mockCustomVerificationProvider) Verify(_ context.Context, check VerificationCheck) (VerificationResult, error) {
	*m.called = true
	return VerificationResult{CheckID: check.ID, Passed: true}, nil
}

func TestPHPAndElixirDetectionAndCandidates(t *testing.T) {
	t.Run("PHP Composer Project", func(t *testing.T) {
		root := t.TempDir()
		composerJSON := `{
  "name": "acme/blog",
  "scripts": {
    "dev": "php -S 127.0.0.1:8080 -t public"
  }
}`
		if err := os.WriteFile(filepath.Join(root, "composer.json"), []byte(composerJSON), 0644); err != nil {
			t.Fatal(err)
		}

		proj, err := detectPHPProject(root)
		if err != nil {
			t.Fatalf("detectPHPProject failed: %v", err)
		}
		if proj.Language != "PHP" || proj.RunCommand != "composer run dev" {
			t.Fatalf("unexpected php project: %+v", proj)
		}

		provider := PHPExecutionCandidateProvider{}
		if !provider.Supports(Component{Language: "PHP"}) {
			t.Fatal("provider should support PHP")
		}
		candidates, err := provider.Candidates(context.Background(), root, Component{Language: "PHP", Path: "."})
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates) == 0 || candidates[0].Command != "composer run dev" {
			t.Fatalf("unexpected candidates: %+v", candidates)
		}
	})

	t.Run("Elixir Phoenix Project", func(t *testing.T) {
		root := t.TempDir()
		mixContent := `
defmodule MyApp.MixProject do
  use Mix.Project

  defp deps do
    [
      {:phoenix, "~> 1.7.0"}
    ]
  end
end
`
		if err := os.WriteFile(filepath.Join(root, "mix.exs"), []byte(mixContent), 0644); err != nil {
			t.Fatal(err)
		}

		proj, err := detectElixirProject(root)
		if err != nil {
			t.Fatalf("detectElixirProject failed: %v", err)
		}
		if proj.Language != "Elixir" || proj.RunCommand != "mix phx.server" {
			t.Fatalf("unexpected elixir project: %+v", proj)
		}

		provider := ElixirExecutionCandidateProvider{}
		if !provider.Supports(Component{Language: "Elixir"}) {
			t.Fatal("provider should support Elixir")
		}
		candidates, err := provider.Candidates(context.Background(), root, Component{Language: "Elixir", Path: "."})
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates) == 0 || candidates[0].Command != "mix phx.server" {
			t.Fatalf("unexpected elixir candidates: %+v", candidates)
		}
	})
}
