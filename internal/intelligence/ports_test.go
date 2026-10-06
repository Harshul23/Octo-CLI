package intelligence

import (
	"context"
	"fmt"
	"net"
	"testing"
)

func TestAllocateComponentPortsKeepsAvailableRequestedPort(t *testing.T) {
	port := freeTestPort(t)
	assignments, err := AllocateComponentPorts([]Component{{Name: "web", Port: port}})
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 1 || assignments[0].Resolved != port || assignments[0].Automatic {
		t.Fatalf("assignments=%+v", assignments)
	}
}

func TestAllocateComponentPortsMovesOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	occupied := listener.Addr().(*net.TCPAddr).Port
	defer listener.Close()

	assignments, err := AllocateComponentPorts([]Component{{Name: "web", Port: occupied}})
	if err != nil {
		t.Fatal(err)
	}
	if assignments[0].Resolved == occupied || !assignments[0].Automatic {
		t.Fatalf("assignments=%+v", assignments)
	}
}

func TestPlannerInjectsResolvedPort(t *testing.T) {
	port := freeTestPort(t)
	model := ProjectModel{
		Name: "web",
		Root: t.TempDir(),
		Components: []Component{{Name: "web", Path: ".", RunCommand: "echo $PORT", Port: port}},
	}
	plan, err := (DeterministicPlanner{}).Plan(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ports) != 1 || plan.Ports[0].Resolved != port {
		t.Fatalf("ports=%+v", plan.Ports)
	}
	step := findExecutionStep(plan, "component.web.start")
	if step == nil || step.Environment["PORT"] != fmt.Sprintf("%d", port) {
		t.Fatalf("step environment=%v", step)
	}
}

func freeTestPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}


func TestAllocateComponentPortsRejectsOccupiedStrictPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	occupied := listener.Addr().(*net.TCPAddr).Port
	defer listener.Close()

	_, err = AllocateComponentPorts([]Component{{Name: "web", Port: occupied, PortStrict: true}})
	if err == nil {
		t.Fatal("expected strict occupied port to fail")
	}
}

func TestAllocateComponentPortsShiftsNonStrictPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	occupied := listener.Addr().(*net.TCPAddr).Port
	defer listener.Close()

	assignments, err := AllocateComponentPorts([]Component{{Name: "web", Port: occupied, PortStrict: false}})
	if err != nil {
		t.Fatal(err)
	}
	if assignments[0].Resolved == occupied || !assignments[0].Automatic || assignments[0].Strict {
		t.Fatalf("assignments=%+v", assignments)
	}
}

func TestAllocateComponentPortsShiftsPortConflictingWithService(t *testing.T) {
	servicePort := freeTestPort(t)
	services := []Service{
		{
			Name:  "postgres",
			Ports: []string{fmt.Sprintf("%d:5432", servicePort)},
		},
	}
	components := []Component{
		{Name: "api", Port: servicePort, PortStrict: false},
	}

	assignments, err := AllocateComponentPortsWithServices(components, services)
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(assignments))
	}
	if assignments[0].Resolved == servicePort {
		t.Fatalf("expected port to shift away from service port %d, got %d", servicePort, assignments[0].Resolved)
	}
	if !assignments[0].Automatic {
		t.Fatal("expected assignment to be marked automatic")
	}
}

func TestAllocateComponentPortsStrictFailsWhenConflictingWithService(t *testing.T) {
	servicePort := freeTestPort(t)
	services := []Service{
		{
			Name:  "redis",
			Ports: []string{fmt.Sprintf("%d:6379", servicePort)},
		},
	}
	components := []Component{
		{Name: "cache", Port: servicePort, PortStrict: true},
	}

	_, err := AllocateComponentPortsWithServices(components, services)
	if err == nil {
		t.Fatal("expected strict port conflict with service to fail")
	}
}

