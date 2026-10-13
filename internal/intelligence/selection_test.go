package intelligence

import (
	"context"
	"strings"
	"testing"
)

func TestSelectionWebOnly(t *testing.T) {
	model := ProjectModel{
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "npm run dev", Role: "application"},
			{Name: "api", Path: "apps/api", RunCommand: "go run .", Role: "application"},
		},
		Services: []Service{
			{Name: "postgres", Image: "postgres:15"},
		},
	}

	// Typed selector
	res, err := ResolveSelection(model, SelectionOptions{Only: []string{"component:web"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.SelectedComponents) != 1 || res.SelectedComponents[0].Name != "web" {
		t.Fatalf("expected only web component, got %+v", res.SelectedComponents)
	}
	if len(res.SelectedServices) != 0 {
		t.Fatalf("expected 0 services, got %+v", res.SelectedServices)
	}

	// Bare name selector
	resBare, err := ResolveSelection(model, SelectionOptions{Only: []string{"web"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resBare.SelectedComponents) != 1 || resBare.SelectedComponents[0].Name != "web" {
		t.Fatalf("expected only web component, got %+v", resBare.SelectedComponents)
	}
}

func TestSelectionApiAndDatabase(t *testing.T) {
	model := ProjectModel{
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "npm run dev", Role: "application"},
			{Name: "api", Path: "apps/api", RunCommand: "go run .", Role: "application"},
		},
		Services: []Service{
			{Name: "postgres", Image: "postgres:15"},
			{Name: "redis", Image: "redis:7"},
		},
	}

	res, err := ResolveSelection(model, SelectionOptions{Only: []string{"component:api", "service:postgres"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.SelectedComponents) != 1 || res.SelectedComponents[0].Name != "api" {
		t.Fatalf("expected api component, got %+v", res.SelectedComponents)
	}
	if len(res.SelectedServices) != 1 || res.SelectedServices[0].Name != "postgres" {
		t.Fatalf("expected postgres service, got %+v", res.SelectedServices)
	}
}

func TestSelectionServiceOnly(t *testing.T) {
	model := ProjectModel{
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "npm run dev", Role: "application"},
		},
		Services: []Service{
			{Name: "postgres", Image: "postgres:15"},
		},
	}

	res, err := ResolveSelection(model, SelectionOptions{Only: []string{"service:postgres"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.SelectedComponents) != 0 {
		t.Fatalf("expected 0 components, got %+v", res.SelectedComponents)
	}
	if len(res.SelectedServices) != 1 || res.SelectedServices[0].Name != "postgres" {
		t.Fatalf("expected 1 service postgres, got %+v", res.SelectedServices)
	}
}

func TestSelectionMultiHopDependenciesBothListOrders(t *testing.T) {
	// A depends on B, B depends on C (service)
	// Order 1: A first, then B
	model1 := ProjectModel{
		Components: []Component{
			{Name: "comp-a", Path: "a", RunCommand: "run-a", DependsOn: []string{"comp-b"}},
			{Name: "comp-b", Path: "b", RunCommand: "run-b", DependsOn: []string{"service:db"}},
		},
		Services: []Service{
			{Name: "db", Image: "db:latest"},
		},
	}

	res1, err := ResolveSelection(model1, SelectionOptions{Only: []string{"component:comp-a"}})
	if err != nil {
		t.Fatalf("order 1 failed: %v", err)
	}
	if len(res1.SelectedComponents) != 2 {
		t.Fatalf("order 1: expected 2 components, got %d", len(res1.SelectedComponents))
	}
	if len(res1.SelectedServices) != 1 {
		t.Fatalf("order 1: expected 1 service, got %d", len(res1.SelectedServices))
	}
	if len(res1.ForcedDependencies) != 2 {
		t.Fatalf("order 1: expected 2 forced dependencies, got %d: %+v", len(res1.ForcedDependencies), res1.ForcedDependencies)
	}

	// Order 2: B first, then A (reversed list order)
	model2 := ProjectModel{
		Components: []Component{
			{Name: "comp-b", Path: "b", RunCommand: "run-b", DependsOn: []string{"service:db"}},
			{Name: "comp-a", Path: "a", RunCommand: "run-a", DependsOn: []string{"comp-b"}},
		},
		Services: []Service{
			{Name: "db", Image: "db:latest"},
		},
	}

	res2, err := ResolveSelection(model2, SelectionOptions{Only: []string{"component:comp-a"}})
	if err != nil {
		t.Fatalf("order 2 failed: %v", err)
	}
	if len(res2.SelectedComponents) != 2 {
		t.Fatalf("order 2: expected 2 components, got %d", len(res2.SelectedComponents))
	}
	if len(res2.SelectedServices) != 1 {
		t.Fatalf("order 2: expected 1 service, got %d", len(res2.SelectedServices))
	}
}

func TestSelectionComponentServiceSameName(t *testing.T) {
	model := ProjectModel{
		Components: []Component{
			{Name: "redis", Path: "services/redis", RunCommand: "node worker.js", Role: "application"},
		},
		Services: []Service{
			{Name: "redis", Image: "redis:7"},
		},
	}

	// Bare name must fail as ambiguous
	_, err := ResolveSelection(model, SelectionOptions{Only: []string{"redis"}})
	if err == nil || !strings.Contains(err.Error(), "ambiguous selector") {
		t.Fatalf("expected ambiguous selector error, got %v", err)
	}

	// Explicit typed selectors must resolve unambiguously
	resComp, err := ResolveSelection(model, SelectionOptions{Only: []string{"component:redis"}})
	if err != nil {
		t.Fatalf("unexpected error for component:redis: %v", err)
	}
	if len(resComp.SelectedComponents) != 1 || resComp.SelectedComponents[0].Name != "redis" || len(resComp.SelectedServices) != 0 {
		t.Fatalf("expected only component redis, got %+v", resComp)
	}

	resSvc, err := ResolveSelection(model, SelectionOptions{Only: []string{"service:redis"}})
	if err != nil {
		t.Fatalf("unexpected error for service:redis: %v", err)
	}
	if len(resSvc.SelectedServices) != 1 || resSvc.SelectedServices[0].Name != "redis" || len(resSvc.SelectedComponents) != 0 {
		t.Fatalf("expected only service redis, got %+v", resSvc)
	}
}

func TestSelectionLibraries(t *testing.T) {
	model := ProjectModel{
		Name: "demo",
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "npm run dev", Role: "application", DependsOn: []string{"common"}},
			{Name: "common", Path: "packages/common", Role: "library", NonRunnableReason: "workspace library without executable entry point"},
		},
	}

	// Requesting library directly must be rejected
	_, err := ResolveSelection(model, SelectionOptions{Only: []string{"component:common"}})
	if err == nil || !strings.Contains(err.Error(), "workspace library and cannot be requested as a runnable launch target") {
		t.Fatalf("expected library launch rejection error, got %v", err)
	}

	// Selecting web should pull in common as a build/install dependency via transitive closure
	res, err := ResolveSelection(model, SelectionOptions{Only: []string{"component:web"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.SelectedComponents) != 2 {
		t.Fatalf("expected 2 components (web + common library), got %d: %+v", len(res.SelectedComponents), res.SelectedComponents)
	}
	if res.ForcedDependencies[TargetID{Kind: TargetKindComponent, Name: "common"}] == "" {
		t.Fatal("expected common to be recorded as a forced dependency")
	}

	// Planner must generate plan without failing on library missing executable
	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), res.FilteredModel)
	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}
	for _, step := range plan.Steps {
		if step.Component == "common" && step.Phase == PhaseStart {
			t.Fatal("library common must not receive a start step in plan")
		}
	}
}

func TestSelectionUnknownID(t *testing.T) {
	model := ProjectModel{
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "npm run dev", Role: "application"},
		},
	}

	_, err := ResolveSelection(model, SelectionOptions{Only: []string{"component:nonexistent"}})
	if err == nil || !strings.Contains(err.Error(), "unknown component") {
		t.Fatalf("expected unknown component error, got %v", err)
	}
}

func TestSelectionDependencyCycles(t *testing.T) {
	model := ProjectModel{
		Components: []Component{
			{Name: "a", Path: "a", RunCommand: "run-a", DependsOn: []string{"b"}},
			{Name: "b", Path: "b", RunCommand: "run-b", DependsOn: []string{"a"}},
		},
	}

	_, err := ResolveSelection(model, SelectionOptions{Only: []string{"component:a"}})
	if err == nil || !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Fatalf("expected dependency cycle error, got %v", err)
	}
}

func TestSelectionOmittedNetworkReferences(t *testing.T) {
	model := ProjectModel{
		Name: "demo",
		Components: []Component{
			{
				Name:       "web",
				Path:       "apps/web",
				RunCommand: "npm run dev",
				Role:       "application",
				References: []Reference{
					{
						Target: "postgres",
						Kind:   RelationshipNetworkReference,
					},
				},
			},
		},
		Services: []Service{
			{Name: "postgres", Image: "postgres:15"},
		},
	}

	// Select web only, omitting postgres
	res, err := ResolveSelection(model, SelectionOptions{Only: []string{"component:web"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.SelectedServices) != 0 {
		t.Fatalf("expected postgres to be omitted from services, got %d", len(res.SelectedServices))
	}

	// Planning the selected subgraph must succeed without failure despite network reference to omitted postgres
	planner := DeterministicPlanner{}
	plan, err := planner.Plan(context.Background(), res.FilteredModel)
	if err != nil {
		t.Fatalf("Plan failed on omitted network reference: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Fatal("expected plan steps")
	}
}

func TestSelectionComposeBuildWrapperUnified(t *testing.T) {
	model := ProjectModel{
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "npm run dev", Role: "application"},
		},
		Services: []Service{
			{Name: "web", Build: "apps/web"},
			{Name: "postgres", Image: "postgres:15"},
		},
	}

	// Requesting service:web resolves to component:web and does not run container service
	res, err := ResolveSelection(model, SelectionOptions{Only: []string{"service:web"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.SelectedComponents) != 1 || res.SelectedComponents[0].Name != "web" {
		t.Fatalf("expected component web selected, got %+v", res.SelectedComponents)
	}
	if len(res.SelectedServices) != 0 {
		t.Fatalf("expected compose wrapper service not started as container, got %+v", res.SelectedServices)
	}
}

func TestSelectionWatchReanalysis(t *testing.T) {
	initialModel := ProjectModel{
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "npm run dev", Role: "application", DependsOn: []string{"db"}},
		},
		Services: []Service{
			{Name: "db", Image: "postgres:15"},
		},
	}

	res, err := ResolveSelection(initialModel, SelectionOptions{Only: []string{"component:web"}})
	if err != nil {
		t.Fatalf("initial selection failed: %v", err)
	}
	requestedIdentities := res.RequestedTargets

	// 1. Reanalysis with new component added: new target must NOT be silently added
	reanalyzedWithNew := ProjectModel{
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "npm run dev", Role: "application", DependsOn: []string{"db"}},
			{Name: "docs", Path: "docs", RunCommand: "npm start", Role: "application"},
		},
		Services: []Service{
			{Name: "db", Image: "postgres:15"},
		},
	}

	reRes, err := ResolveSelectionTargets(reanalyzedWithNew, requestedIdentities)
	if err != nil {
		t.Fatalf("reanalysis failed: %v", err)
	}
	if len(reRes.SelectedComponents) != 1 || reRes.SelectedComponents[0].Name != "web" {
		t.Fatalf("docs should not be silently added, got %+v", reRes.SelectedComponents)
	}

	// 2. Reanalysis with selected target deleted: must stop with clear error
	reanalyzedWithDeleted := ProjectModel{
		Components: []Component{
			{Name: "docs", Path: "docs", RunCommand: "npm start", Role: "application"},
		},
		Services: []Service{
			{Name: "db", Image: "postgres:15"},
		},
	}

	_, err = ResolveSelectionTargets(reanalyzedWithDeleted, requestedIdentities)
	if err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Fatalf("expected target disappeared error, got %v", err)
	}
}

func TestSelectionNonInteractiveBehavior(t *testing.T) {
	// Single runnable target: auto-selected
	singleModel := ProjectModel{
		Components: []Component{
			{Name: "web", Path: ".", RunCommand: "npm start", Role: "application"},
		},
	}
	resSingle, err := ResolveSelection(singleModel, SelectionOptions{})
	if err != nil {
		t.Fatalf("single target auto-selection failed: %v", err)
	}
	if len(resSingle.SelectedComponents) != 1 {
		t.Fatalf("expected 1 component, got %d", len(resSingle.SelectedComponents))
	}

	// Multiple runnable targets: requires --only or --all
	multiModel := ProjectModel{
		Components: []Component{
			{Name: "web", Path: "apps/web", RunCommand: "npm start", Role: "application"},
			{Name: "api", Path: "apps/api", RunCommand: "go run .", Role: "application"},
		},
	}
	_, err = ResolveSelection(multiModel, SelectionOptions{})
	if err == nil || !strings.Contains(err.Error(), "multiple runnable targets detected") {
		t.Fatalf("expected noninteractive multiple targets error, got %v", err)
	}

	// Specifying both --only and --all is mutually exclusive
	_, err = ResolveSelection(multiModel, SelectionOptions{Only: []string{"web"}, All: true})
	if err == nil || !strings.Contains(err.Error(), "cannot combine --only and --all") {
		t.Fatalf("expected mutual exclusion error, got %v", err)
	}

	// Specifying --all succeeds
	resAll, err := ResolveSelection(multiModel, SelectionOptions{All: true})
	if err != nil {
		t.Fatalf("expected --all to succeed, got %v", err)
	}
	if len(resAll.SelectedComponents) != 2 {
		t.Fatalf("expected all 2 components selected, got %d", len(resAll.SelectedComponents))
	}
}
