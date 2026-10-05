package intelligence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestJevDecisionProviderAcceptsOnlyKnownOption(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"option_id":"high","value":"go run .","confidence":0.99,"reason":"Jev selected the package candidate."}`))
	}))
	defer server.Close()

	provider := &JevDecisionProvider{URL: server.URL}
	result, err := provider.Decide(context.Background(), DecisionRequest{
		Name: "run_command",
		Options: []DecisionOption{
			{ID: "low", Value: "go run main.go", Confidence: 0.7},
			{ID: "high", Value: "go run .", Confidence: 0.9},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OptionID != "high" || result.Value != "go run ." {
		t.Fatalf("result=%+v, want high/go run .", result)
	}
}

func TestJevDecisionProviderRejectsInventedOption(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"option_id":"invented","value":"rm -rf .","confidence":1}`))
	}))
	defer server.Close()

	provider := &JevDecisionProvider{URL: server.URL}
	_, err := provider.Decide(context.Background(), DecisionRequest{
		Name: "run_command",
		Options: []DecisionOption{{ID: "known", Value: "go run ."}},
	})
	if err == nil {
		t.Fatal("expected invented option to be rejected")
	}
}

func TestOptionalDecisionProviderFallsBackWithoutJev(t *testing.T) {
	oldProvider, oldURL := os.Getenv("OCTO_DECISION_PROVIDER"), os.Getenv("OCTO_JEV_URL")
	t.Cleanup(func() {
		_ = os.Setenv("OCTO_DECISION_PROVIDER", oldProvider)
		_ = os.Setenv("OCTO_JEV_URL", oldURL)
	})
	_ = os.Setenv("OCTO_DECISION_PROVIDER", "jev")
	_ = os.Unsetenv("OCTO_JEV_URL")

	result, err := OptionalDecisionProvider().Decide(context.Background(), DecisionRequest{
		Name: "run_command",
		Options: []DecisionOption{
			{ID: "low", Value: "one", Confidence: 0.2},
			{ID: "high", Value: "two", Confidence: 0.9},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OptionID != "high" {
		t.Fatalf("selected=%q, want high", result.OptionID)
	}
}
