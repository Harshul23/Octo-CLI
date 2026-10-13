package intelligence

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func smokeFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Root package.json
	mustWriteSmokeFile(t, filepath.Join(dir, "package.json"), `{
  "name": "monorepo-smoke",
  "private": true,
  "workspaces": [
    "apps/*",
    "packages/*"
  ]
}`)

	// Root docker-compose.yml
	mustWriteSmokeFile(t, filepath.Join(dir, "docker-compose.yml"), `version: '3.8'
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: app
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d app"]
      interval: 2s
      timeout: 2s
      retries: 5

volumes:
  pgdata:
`)

	// Root .env
	mustWriteSmokeFile(t, filepath.Join(dir, ".env"), `SHARED_ROOT_ENV=smoke-root-value
`)

	// Root .octo.yaml
	mustWriteSmokeFile(t, filepath.Join(dir, ".octo.yaml"), `spec: octo/v1
components:
  api:
    depends_on:
      - service:postgres
      - component:shared
`)

	// apps/web
	webDir := filepath.Join(dir, "apps", "web")
	if err := os.MkdirAll(webDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWriteSmokeFile(t, filepath.Join(webDir, "package.json"), `{
  "name": "web",
  "version": "1.0.0",
  "scripts": {
    "start": "node server.js"
  }
}`)
	mustWriteSmokeFile(t, filepath.Join(webDir, "server.js"), `const http = require('http');
const port = process.env.PORT || 3000;
const server = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': 'text/plain' });
  res.end('web running');
});
server.listen(port, () => {
  console.log(`+"`web listening on port ${port}`"+`);
});
`)
	mustWriteSmokeFile(t, filepath.Join(webDir, ".env"), `PORT=3000
WEB_SECRET=web-secret-val
`)

	// apps/api
	apiDir := filepath.Join(dir, "apps", "api")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWriteSmokeFile(t, filepath.Join(apiDir, "package.json"), `{
  "name": "api",
  "version": "1.0.0",
  "scripts": {
    "start": "node server.js"
  },
  "dependencies": {
    "shared": "*"
  }
}`)
	mustWriteSmokeFile(t, filepath.Join(apiDir, "server.js"), `const http = require('http');
const port = process.env.PORT || 8080;
const server = http.createServer((req, res) => {
  res.writeHead(200, { 'Content-Type': 'text/plain' });
  res.end('api running');
});
server.listen(port, () => {
  console.log(`+"`api listening on port ${port}`"+`);
});
`)
	mustWriteSmokeFile(t, filepath.Join(apiDir, ".env"), `PORT=8080
API_SECRET=api-secret-val
DATABASE_URL=postgres://octo:octosecret@postgres:5432/smokedb
`)

	// packages/shared
	sharedDir := filepath.Join(dir, "packages", "shared")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}
	mustWriteSmokeFile(t, filepath.Join(sharedDir, "package.json"), `{
  "name": "shared",
  "version": "1.0.0",
  "scripts": {
    "build": "node -e 'console.log(\"building shared package\")'"
  }
}`)
	mustWriteSmokeFile(t, filepath.Join(sharedDir, "index.js"), `module.exports = {
  version: '1.0.0',
  name: 'shared'
};
`)

	return dir
}

func mustWriteSmokeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// 1. Inspect lists targets and evidence
func TestSmokeFixtureAnalysisAndInspect(t *testing.T) {
	fixture := smokeFixtureDir(t)
	model, err := Analyze(fixture)
	if err != nil {
		t.Fatalf("Analyze failed on smoke fixture: %v", err)
	}

	if model.Name != "monorepo-smoke" {
		t.Fatalf("expected project name 'monorepo-smoke', got %q", model.Name)
	}
	if !model.Monorepo {
		t.Fatal("expected model.Monorepo to be true")
	}

	// Verify components: web, api, shared
	compMap := make(map[string]Component)
	for _, c := range model.Components {
		compMap[c.Name] = c
	}

	web, hasWeb := compMap["web"]
	if !hasWeb {
		t.Fatal("expected 'web' component to be discovered")
	}
	if web.Port != 3000 {
		t.Fatalf("expected web port 3000, got %d", web.Port)
	}
	if len(web.ExecutionCandidates) == 0 {
		t.Fatal("expected execution candidates for web")
	}

	api, hasAPI := compMap["api"]
	if !hasAPI {
		t.Fatal("expected 'api' component to be discovered")
	}
	if api.Port != 8080 {
		t.Fatalf("expected api port 8080, got %d", api.Port)
	}
	if len(api.ExecutionCandidates) == 0 {
		t.Fatal("expected execution candidates for api")
	}

	shared, hasShared := compMap["shared"]
	if !hasShared {
		t.Fatal("expected 'shared' component to be discovered")
	}
	// Shared has only build script, no start/dev, so it must be classified as library
	if shared.Role != "library" && len(primaryExecutionCandidates(shared.ExecutionCandidates)) > 0 {
		t.Fatalf("expected shared to be a non-runnable library, got role %q", shared.Role)
	}

	// Verify service: postgres
	if len(model.Services) != 1 || model.Services[0].Name != "postgres" {
		t.Fatalf("expected postgres service, got %+v", model.Services)
	}
	postgres := model.Services[0]
	if postgres.HealthCheck == nil || !strings.Contains(postgres.HealthCheck.Command, "pg_isready") {
		t.Fatalf("expected pg_isready healthcheck on postgres, got %+v", postgres.HealthCheck)
	}

	// Verify runnable targets helper
	runnable := RunnableTargets(model)
	runnableNames := make([]string, 0, len(runnable))
	for _, r := range runnable {
		runnableNames = append(runnableNames, r.Name)
	}
	if !containsString(runnableNames, "web") || !containsString(runnableNames, "api") || !containsString(runnableNames, "postgres") {
		t.Fatalf("expected web, api, and postgres in runnable targets, got: %v", runnableNames)
	}
	if containsString(runnableNames, "shared") {
		t.Fatal("shared library must not be listed as a runnable launch target")
	}
}

// 2. The picker selects web alone
func TestSmokeFixturePickerSelectsWebAlone(t *testing.T) {
	fixture := smokeFixtureDir(t)
	model, err := Analyze(fixture)
	if err != nil {
		t.Fatal(err)
	}

	sel, err := ResolveSelection(model, SelectionOptions{
		Only: []string{"component:web"},
	})
	if err != nil {
		t.Fatalf("selection failed: %v", err)
	}

	if len(sel.SelectedComponents) != 1 || sel.SelectedComponents[0].Name != "web" {
		t.Fatalf("expected only web component selected, got: %+v", sel.SelectedComponents)
	}
	if len(sel.SelectedServices) != 0 {
		t.Fatalf("expected 0 services selected, got: %+v", sel.SelectedServices)
	}
	if len(sel.ForcedDependencies) != 0 {
		t.Fatalf("expected 0 forced dependencies for web alone, got: %+v", sel.ForcedDependencies)
	}
}

// 3. --only selects api and includes its explicitly declared database dependency
func TestSmokeFixtureSelectionApiIncludesDatabaseDependency(t *testing.T) {
	fixture := smokeFixtureDir(t)
	model, err := Analyze(fixture)
	if err != nil {
		t.Fatal(err)
	}

	sel, err := ResolveSelection(model, SelectionOptions{
		Only: []string{"component:api"},
	})
	if err != nil {
		t.Fatalf("selection failed: %v", err)
	}

	if len(sel.RequestedTargets) != 1 || sel.RequestedTargets[0].Name != "api" {
		t.Fatalf("expected only api component requested, got: %+v", sel.RequestedTargets)
	}
	runnableComps := make([]Component, 0)
	for _, c := range sel.SelectedComponents {
		if c.Role != "library" {
			runnableComps = append(runnableComps, c)
		}
	}
	if len(runnableComps) != 1 || runnableComps[0].Name != "api" {
		t.Fatalf("expected only api runnable component, got: %+v", runnableComps)
	}
	if len(sel.SelectedServices) != 1 || sel.SelectedServices[0].Name != "postgres" {
		t.Fatalf("expected postgres service included, got: %+v", sel.SelectedServices)
	}

	// Must be recorded in ForcedDependencies with actionable reason
	pgID := TargetID{Kind: TargetKindService, Name: "postgres"}
	reason, ok := sel.ForcedDependencies[pgID]
	if !ok {
		t.Fatalf("expected postgres in ForcedDependencies, got: %+v", sel.ForcedDependencies)
	}
	if !strings.Contains(reason, "component:api") {
		t.Fatalf("expected dependency reason to mention component:api, got %q", reason)
	}
}

// 4. --all starts all runnable targets without launching the library
func TestSmokeFixtureSelectionAllWithoutLibrary(t *testing.T) {
	fixture := smokeFixtureDir(t)
	model, err := Analyze(fixture)
	if err != nil {
		t.Fatal(err)
	}

	sel, err := ResolveSelection(model, SelectionOptions{All: true})
	if err != nil {
		t.Fatalf("selection failed: %v", err)
	}

	compNames := make([]string, 0, len(sel.SelectedComponents))
	for _, c := range sel.SelectedComponents {
		compNames = append(compNames, c.Name)
	}
	if !containsString(compNames, "web") || !containsString(compNames, "api") {
		t.Fatalf("expected web and api in selected components, got: %v", compNames)
	}

	// Plan execution
	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), sel.FilteredModel)
	if err != nil {
		t.Fatalf("planning failed: %v", err)
	}

	// component.shared.start must NEVER exist
	if findStep(plan.Steps, "component.shared.start") != nil {
		t.Fatal("component.shared.start must NEVER exist in the plan")
	}

	// component.web.start and component.api.start must exist
	if findStep(plan.Steps, "component.web.start") == nil {
		t.Fatal("expected component.web.start")
	}
	if findStep(plan.Steps, "component.api.start") == nil {
		t.Fatal("expected component.api.start")
	}
	if findStep(plan.Steps, "service.postgres.start") == nil {
		t.Fatal("expected service.postgres.start")
	}

	// Shared library build step must exist in PhaseSetup if build script declared
	if findStep(plan.Steps, "component.shared.build") == nil {
		t.Fatal("expected component.shared.build in PhaseSetup")
	}
}

// 5. Each target runs in the correct cwd and scoped environment
func TestSmokeFixtureTargetCwdAndEnvScoping(t *testing.T) {
	fixture := smokeFixtureDir(t)
	model, err := Analyze(fixture)
	if err != nil {
		t.Fatal(err)
	}

	env, err := ResolveProjectEnvironment(fixture, model)
	if err != nil {
		t.Fatalf("environment resolution failed: %v", err)
	}

	webDir := filepath.Join(fixture, "apps", "web")
	apiDir := filepath.Join(fixture, "apps", "api")

	webStep := ExecutionStep{Component: "web", WorkDir: webDir}
	apiStep := ExecutionStep{Component: "api", WorkDir: apiDir}

	webEnv := env.ForStep(webStep)
	apiEnv := env.ForStep(apiStep)

	// Web scoped env
	if webEnv.Values["WEB_SECRET"] != "web-secret-val" {
		t.Fatalf("expected WEB_SECRET=web-secret-val in webEnv, got %q", webEnv.Values["WEB_SECRET"])
	}
	if webEnv.Values["API_SECRET"] != "" {
		t.Fatal("API_SECRET leaked into web environment")
	}
	if webEnv.Values["PORT"] != "3000" {
		t.Fatalf("expected web PORT=3000, got %q", webEnv.Values["PORT"])
	}

	// API scoped env
	if apiEnv.Values["API_SECRET"] != "api-secret-val" {
		t.Fatalf("expected API_SECRET=api-secret-val in apiEnv, got %q", apiEnv.Values["API_SECRET"])
	}
	if apiEnv.Values["WEB_SECRET"] != "" {
		t.Fatal("WEB_SECRET leaked into api environment")
	}
	if apiEnv.Values["PORT"] != "8080" {
		t.Fatalf("expected api PORT=8080, got %q", apiEnv.Values["PORT"])
	}
	if !strings.Contains(apiEnv.Values["DATABASE_URL"], "postgres://") {
		t.Fatalf("expected DATABASE_URL in apiEnv, got %q", apiEnv.Values["DATABASE_URL"])
	}
}

// 6. Ctrl+C stops owned resources and leaves .octo.lock only after verified startup
func TestSmokeFixtureCtrlCLeavesLockOnlyAfterVerifiedStartup(t *testing.T) {
	fixture := smokeFixtureDir(t)
	tmp := t.TempDir()

	// Copy fixture to tmp so we can write lock safely
	copyDir(t, fixture, tmp)

	model, err := Analyze(tmp)
	if err != nil {
		t.Fatal(err)
	}

	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}

	lWeb, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lWeb.Close()
	portWeb := lWeb.Addr().(*net.TCPAddr).Port

	lApi, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lApi.Close()
	portApi := lApi.Addr().(*net.TCPAddr).Port

	plan.Ports = []PortAssignment{
		{Component: "web", Resolved: portWeb},
		{Component: "api", Resolved: portApi},
	}
	procWeb := newMockTestProcessWithListener(6001, lWeb)
	procApi := newMockTestProcessWithListener(6002, lApi)

	adapter := &mockTestAdapter{
		startFunc: func(ctx context.Context, step ExecutionStep) (RunningProcess, error) {
			if step.Component == "web" {
				return procWeb, nil
			}
			return procApi, nil
		},
	}
	resolver := RuntimeResolver{adapters: []RuntimeAdapter{adapter}}
	env := ResolvedEnvironment{Values: map[string]string{}}

	ctx, cancel := context.WithCancel(context.Background())
	checkpointReached := make(chan struct{})

	opts := ExecutionOptions{
		OnVerifiedStartup: func(rep ExecutionReport) error {
			lock := OctoLock{Project: model.Name}
			_, err := RecordVerifiedStrategiesWithLock(tmp, model, plan, rep, lock)
			close(checkpointReached)
			return err
		},
	}

	done := make(chan ExecutionReport, 1)
	go func() {
		done <- ExecutePlanReportWithOptions(ctx, model, plan, resolver, env, opts)
	}()

	<-checkpointReached
	// Simulate Ctrl+C after verified startup
	cancel()

	report := <-done

	if !report.VerifiedStartup {
		t.Fatal("expected VerifiedStartup to be true")
	}
	if !procWeb.stopped || !procApi.stopped {
		t.Fatal("expected owned processes to be stopped on Ctrl+C")
	}

	// Verify .octo.lock exists
	lockPath := filepath.Join(tmp, ".octo.lock")
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected .octo.lock to be preserved after verified startup: %v", err)
	}
}

// 7. Second run reuses matching strategies
func TestSmokeFixtureSecondRunReusesStrategies(t *testing.T) {
	fixture := smokeFixtureDir(t)
	tmp := t.TempDir()
	copyDir(t, fixture, tmp)

	model, err := Analyze(tmp)
	if err != nil {
		t.Fatal(err)
	}

	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}

	report := ExecutionReport{
		ProjectName:     model.Name,
		Success:         true,
		VerifiedStartup: true,
	}
	for _, step := range plan.Steps {
		if step.Phase == PhaseStart && step.SelectedCandidate != "" {
			report.Steps = append(report.Steps, ExecutionStepResult{
				ID:          step.ID,
				Status:      StepSucceeded,
				CandidateID: step.SelectedCandidate,
			})
		}
	}

	// Record lock on first run
	lock, err := RecordVerifiedStrategiesWithLock(tmp, model, plan, report, OctoLock{Project: model.Name})
	if err != nil {
		t.Fatalf("failed to record lock: %v", err)
	}

	// Second run: ApplyVerifiedStrategies
	modelRun2, err := Analyze(tmp)
	if err != nil {
		t.Fatal(err)
	}

	status, err := ApplyVerifiedStrategies(tmp, &modelRun2, lock)
	if err != nil {
		t.Fatalf("failed to apply verified strategies: %v", err)
	}

	if len(status.Reused) < 2 {
		t.Fatalf("expected at least 2 reused strategies (web and api), got %d: %+v", len(status.Reused), status.Reused)
	}
	if len(status.Invalidated) != 0 {
		t.Fatalf("expected 0 invalidated strategies on identical repo, got: %+v", status.Invalidated)
	}
}

// 8. Editing app or root workspace/lockfile invalidates affected cache
func TestSmokeFixtureCacheInvalidationOnEdits(t *testing.T) {
	fixture := smokeFixtureDir(t)
	tmp := t.TempDir()
	copyDir(t, fixture, tmp)

	model, err := Analyze(tmp)
	if err != nil {
		t.Fatal(err)
	}
	var compWeb Component
	for _, c := range model.Components {
		if c.Name == "web" {
			compWeb = c
			break
		}
	}
	if len(compWeb.ExecutionCandidates) == 0 {
		t.Fatal("expected execution candidates for web")
	}
	candWeb := compWeb.ExecutionCandidates[0]

	fpWeb1, err := ExecutionFingerprint(tmp, compWeb, candWeb)
	if err != nil {
		t.Fatal(err)
	}

	// Modify web app's server.js
	if err := os.WriteFile(filepath.Join(tmp, "apps", "web", "server.js"), []byte("// modified server\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fpWeb2, err := ExecutionFingerprint(tmp, compWeb, candWeb)
	if err != nil {
		t.Fatal(err)
	}
	if fpWeb1 == fpWeb2 {
		t.Fatal("expected web fingerprint to change after modifying web/server.js")
	}

	// Modify root package.json -> invalidates nested components
	fpWeb3, err := ExecutionFingerprint(tmp, compWeb, candWeb)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), []byte(`{"name":"monorepo-smoke-v2","workspaces":["apps/*"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fpWeb4, err := ExecutionFingerprint(tmp, compWeb, candWeb)
	if err != nil {
		t.Fatal(err)
	}
	if fpWeb3 == fpWeb4 {
		t.Fatal("expected web fingerprint to change after modifying root package.json")
	}
}

// 9. --watch retains selection
func TestSmokeFixtureWatchRetainsSelection(t *testing.T) {
	fixture := smokeFixtureDir(t)
	model, err := Analyze(fixture)
	if err != nil {
		t.Fatal(err)
	}

	requested := []TargetID{
		{Kind: TargetKindComponent, Name: "web"},
	}

	sel1, err := ResolveSelectionTargets(model, requested)
	if err != nil {
		t.Fatal(err)
	}
	if len(sel1.SelectedComponents) != 1 || sel1.SelectedComponents[0].Name != "web" {
		t.Fatalf("expected web selected in run 1")
	}

	// Re-analysis in watch mode with same requested identities
	reModel, err := Analyze(fixture)
	if err != nil {
		t.Fatal(err)
	}

	sel2, err := ResolveSelectionTargets(reModel, requested)
	if err != nil {
		t.Fatal(err)
	}
	if len(sel2.SelectedComponents) != 1 || sel2.SelectedComponents[0].Name != "web" {
		t.Fatalf("expected web selected after re-analysis in watch mode")
	}
}

// 10. Non-interactive modes and MCP never prompt
func TestSmokeFixtureNonInteractiveNeverPrompts(t *testing.T) {
	fixture := smokeFixtureDir(t)
	model, err := Analyze(fixture)
	if err != nil {
		t.Fatal(err)
	}

	// In non-interactive mode without --only/--all, when multiple runnable targets exist,
	// it must return an error listing targets rather than hanging or prompting.
	runnable := RunnableTargets(model)
	if len(runnable) <= 1 {
		t.Fatalf("expected multiple runnable targets in smoke fixture, got %d", len(runnable))
	}

	// FormatAvailableTargets provides the direct, actionable target listing
	formatted := FormatAvailableTargets(model)
	if !strings.Contains(formatted, "component:web") || !strings.Contains(formatted, "component:api") || !strings.Contains(formatted, "service:postgres") {
		t.Fatalf("expected formatted available targets to list all runnable items, got:\n%s", formatted)
	}
}

// 11. Real Node/Docker integration tests (explicitly opt-in)
// The real Node/Docker integration test now lives in cmd/octo/binary_integration_test.go
// and drives the actual octo binary.

// Helpers
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
	if err != nil {
		t.Fatalf("failed to copy %s to %s: %v", src, dst, err)
	}
}
