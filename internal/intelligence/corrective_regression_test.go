package intelligence

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// installFakeDocker puts a fake `docker` first on PATH. It logs every call to
// docker.log. `ps` prints psOutput (or exits 1 when psFails). `up` fails for
// any service listed in failUp.
func installFakeDocker(t *testing.T, psOutput string, psFails bool, failUp string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake docker uses a POSIX shell script")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "docker.log")
	psExit := "0"
	if psFails {
		psExit = "1"
	}
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> '" + logPath + "'\n" +
		"case \" $* \" in\n" +
		"  *\" ps \"*) printf '%s' '" + psOutput + "'; exit " + psExit + " ;;\n" +
		"esac\n" +
		"if [ -n '" + failUp + "' ]; then case \" $* \" in *\" up \"*\" " + failUp + "\"*) exit 1 ;; esac; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func composeTestModel(t *testing.T, services ...string) (ProjectModel, ExecutionPlan) {
	t.Helper()
	root := t.TempDir()
	composePath := filepath.Join(root, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	model := ProjectModel{Name: "compose-demo", Root: root}
	plan := ExecutionPlan{ProjectName: "compose-demo", Root: root}
	prev := ""
	for _, name := range services {
		model.Services = append(model.Services, Service{
			Name:     name,
			Evidence: []Evidence{{Kind: EvidenceConfig, Path: composePath}},
		})
		step := ExecutionStep{
			ID: "service." + name + ".start", Component: name, Phase: PhaseStart,
			Command: "docker compose -f " + composePath + " up -d " + name, WorkDir: root,
		}
		if prev != "" {
			step.DependsOn = []string{prev}
		}
		plan.Steps = append(plan.Steps, step)
		prev = step.ID
	}
	return model, plan
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	data, _ := os.ReadFile(path)
	return string(data)
}

// Regression: selecting a Compose service with --detach used to fail with
// `runtime adapter "compose" cannot start long-running step`.
func TestComposeServiceStartWorksInDetachMode(t *testing.T) {
	logPath := installFakeDocker(t, "", false, "")
	model, plan := composeTestModel(t, "postgres")

	report := ExecutePlanReportWithOptions(context.Background(), model, plan, NewRuntimeResolver(), ResolvedEnvironment{}, ExecutionOptions{Detach: true, Silent: true})
	if !report.Success {
		t.Fatalf("expected detached compose start to succeed, got: %s", report.FailureReason)
	}
	if !strings.Contains(readLog(t, logPath), "up -d postgres") {
		t.Fatalf("expected compose up to run, log:\n%s", readLog(t, logPath))
	}
	if len(report.OwnedServices) != 1 || report.OwnedServices[0] != "postgres" {
		t.Fatalf("expected postgres to be owned, got %v", report.OwnedServices)
	}
	if strings.Contains(readLog(t, logPath), " stop ") {
		t.Fatalf("a successful detached run must leave services running, log:\n%s", readLog(t, logPath))
	}
}

// Regression: a failed detached run must stop the services this run started,
// and must never remove volumes.
func TestFailedDetachStopsOwnedComposeServices(t *testing.T) {
	logPath := installFakeDocker(t, "", false, "redis")
	model, plan := composeTestModel(t, "postgres", "redis")

	report := ExecutePlanReportWithOptions(context.Background(), model, plan, NewRuntimeResolver(), ResolvedEnvironment{}, ExecutionOptions{Detach: true, Silent: true})
	if report.Success {
		t.Fatal("expected failure when redis fails to start")
	}
	log := readLog(t, logPath)
	if !strings.Contains(log, "stop postgres") {
		t.Fatalf("expected owned postgres to be stopped on failed detach, log:\n%s", log)
	}
	if strings.Contains(log, "down") || strings.Contains(log, "-v") || strings.Contains(log, "volume") {
		t.Fatalf("cleanup must never remove volumes or run down, log:\n%s", log)
	}
}

// Regression: a failing `ps` query is "unknown", not "not running". Octo must
// not claim ownership (and therefore must not stop) in that case.
func TestUnknownComposeStateIsNotClaimedAsOwned(t *testing.T) {
	logPath := installFakeDocker(t, "", true, "")
	model, plan := composeTestModel(t, "postgres")

	report := ExecutePlanReportWithOptions(context.Background(), model, plan, NewRuntimeResolver(), ResolvedEnvironment{}, ExecutionOptions{Detach: true, Silent: true})
	if !report.Success {
		t.Fatalf("unexpected failure: %s", report.FailureReason)
	}
	if len(report.OwnedServices) != 0 {
		t.Fatalf("unknown state must not be claimed as owned, got %v", report.OwnedServices)
	}
	_ = logPath
}

// A pre-existing running service is never claimed or stopped.
func TestRunningComposeServiceIsNotOwned(t *testing.T) {
	installFakeDocker(t, "abc123\n", false, "")
	model, plan := composeTestModel(t, "postgres")

	report := ExecutePlanReportWithOptions(context.Background(), model, plan, NewRuntimeResolver(), ResolvedEnvironment{}, ExecutionOptions{Detach: true, Silent: true})
	if !report.Success {
		t.Fatalf("unexpected failure: %s", report.FailureReason)
	}
	if len(report.OwnedServices) != 0 {
		t.Fatalf("pre-existing service must not be owned, got %v", report.OwnedServices)
	}
}

type flakyAdapter struct{ failures, calls int }

func (a *flakyAdapter) Name() string                  { return "flaky" }
func (a *flakyAdapter) Supports(ExecutionStep) bool   { return true }
func (a *flakyAdapter) Execute(context.Context, ExecutionStep, ResolvedEnvironment) error {
	a.calls++
	if a.calls <= a.failures {
		return errors.New("not ready")
	}
	return nil
}

// Regression: the health check used to run once right after `up`.
func TestHealthCheckRetriesUntilHealthy(t *testing.T) {
	adapter := &flakyAdapter{failures: 3}
	hc := &HealthCheck{Command: "pg_isready", Interval: "5ms", Retries: 5, Timeout: "1s"}
	if err := executeStep(context.Background(), adapter, ExecutionStep{ID: "health"}, ResolvedEnvironment{}, hc); err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if adapter.calls != 4 {
		t.Fatalf("expected 4 attempts, got %d", adapter.calls)
	}
}

func TestHealthCheckIsBoundedByRetries(t *testing.T) {
	adapter := &flakyAdapter{failures: 100}
	hc := &HealthCheck{Command: "pg_isready", Interval: "1ms", Retries: 3}
	start := time.Now()
	err := executeStep(context.Background(), adapter, ExecutionStep{ID: "health"}, ResolvedEnvironment{}, hc)
	if err == nil {
		t.Fatal("expected failure after retries are exhausted")
	}
	if adapter.calls != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", adapter.calls)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("retry loop was not bounded")
	}
}

func TestNonHealthStepsAreNotRetried(t *testing.T) {
	adapter := &flakyAdapter{failures: 100}
	if err := executeStep(context.Background(), adapter, ExecutionStep{ID: "x"}, ResolvedEnvironment{}, nil); err == nil {
		t.Fatal("expected failure")
	}
	if adapter.calls != 1 {
		t.Fatalf("expected a single attempt, got %d", adapter.calls)
	}
}

// Regression: --only web still required variables sourced only from omitted apps/api.
func TestFilterEnvironmentVariablesDropsOmittedComponentSources(t *testing.T) {
	all := []Component{{Name: "apps/web", Path: "apps/web"}, {Name: "apps/api", Path: "apps/api"}}
	selected := all[:1]
	vars := []EnvironmentVariable{
		{Name: "REVIEW_API_SECRET", Required: true, Sources: []string{"apps/api/.env.example"}},
		{Name: "WEB_SECRET", Required: true, Sources: []string{"apps/web/.env.example"}},
		{Name: "SHARED", Required: true, Sources: []string{".env.example"}},
		{Name: "BOTH", Required: true, Sources: []string{"apps/api/.env.example", "apps/web/.env.example"}},
		{Name: "LEGACY", Required: true},
	}
	got := filterEnvironmentVariables(vars, all, selected)
	names := map[string]EnvironmentVariable{}
	for _, v := range got {
		names[v.Name] = v
	}
	if _, ok := names["REVIEW_API_SECRET"]; ok {
		t.Fatal("variable sourced only from the omitted component must be dropped")
	}
	for _, keep := range []string{"WEB_SECRET", "SHARED", "BOTH", "LEGACY"} {
		if _, ok := names[keep]; !ok {
			t.Fatalf("expected %s to be kept", keep)
		}
	}
	if len(names["BOTH"].Sources) != 1 || names["BOTH"].Sources[0] != "apps/web/.env.example" {
		t.Fatalf("expected BOTH to keep only the selected source, got %v", names["BOTH"].Sources)
	}
}

// Regression: renaming a component during disambiguation left depends_on stale.
func TestDisambiguationRewritesDependsOn(t *testing.T) {
	components := []Component{
		{Name: "web", Path: "apps/web"},
		{Name: "api", Path: "apps/api", DependsOn: []string{"web"}},
	}
	services := []Service{{Name: "web", Build: "apps/web"}}
	got := disambiguateComponentNames(components, services)
	if got[0].Name != "apps/web" {
		t.Fatalf("expected web to be renamed to apps/web, got %q", got[0].Name)
	}
	if len(got[1].DependsOn) != 1 || got[1].DependsOn[0] != "apps/web" {
		t.Fatalf("expected api to depend on apps/web, got %v", got[1].DependsOn)
	}
}

func TestDisambiguationLeavesAmbiguousDependenciesAlone(t *testing.T) {
	components := []Component{
		{Name: "app", Path: "a/app"},
		{Name: "app", Path: "b/app"},
		{Name: "api", Path: "api", DependsOn: []string{"app"}},
	}
	got := disambiguateComponentNames(components, nil)
	if got[2].DependsOn[0] != "app" {
		t.Fatalf("ambiguous dependency must not be guessed, got %v", got[2].DependsOn)
	}
}

// Regression: the resolver accepted service:postgres but the planner built service:service:postgres.
func TestTopologyAcceptsBareAndTypedServiceDependencies(t *testing.T) {
	for _, dep := range []string{"postgres", "service:postgres"} {
		model := ProjectModel{
			Name: "x", Root: t.TempDir(),
			Services: []Service{
				{Name: "postgres", Image: "postgres"},
				{Name: "cache", Image: "redis", DependsOn: []string{dep}},
			},
		}
		graph, err := BuildTopologyGraph(model)
		if err != nil {
			t.Fatalf("dependency %q: %v", dep, err)
		}
		found := false
		for _, edge := range graph.Edges {
			if edge.From == "service:cache" && edge.To == "service:postgres" {
				found = true
			}
		}
		if !found {
			t.Fatalf("dependency %q: expected edge service:cache -> service:postgres", dep)
		}
	}
}
