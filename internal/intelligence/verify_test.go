package intelligence

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestBuildVerificationChecksForResolvedPort(t *testing.T) {
	model := ProjectModel{}
	plan := ExecutionPlan{Ports: []PortAssignment{{Component: "web", Requested: 3000, Resolved: 3001, Automatic: true}}}
	checks := BuildVerificationChecks(model, plan)
	if len(checks) != 1 {
		t.Fatalf("checks=%+v", checks)
	}
	if checks[0].ID != "port.web" || checks[0].Port != 3001 {
		t.Fatalf("check=%+v", checks[0])
	}
}

func TestVerifyReachablePort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	results, err := Verify(context.Background(), []VerificationCheck{{
		ID: "port.web", Kind: VerificationPort, Component: "web",
		Host: "127.0.0.1", Port: port, Timeout: time.Second,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].Passed {
		t.Fatalf("results=%+v", results)
	}
}

func TestVerifyReportsUnavailablePort(t *testing.T) {
	port := freeTestPort(t)
	_, err := Verify(context.Background(), []VerificationCheck{{
		ID: "port.web", Kind: VerificationPort, Component: "web",
		Host: "127.0.0.1", Port: port, Timeout: 100 * time.Millisecond,
	}})
	if err == nil {
		t.Fatal("expected verification failure")
	}
}
