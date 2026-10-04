package intelligence

import "testing"

func TestBuildTopologyGraphCombinesComponentsAndServices(t *testing.T) {
	model := ProjectModel{
		Name: "stack",
		Components: []Component{
			{Name: "web", Path: "web", DependsOn: []string{"api"}, Confidence: 0.9, Evidence: []Evidence{{Kind: EvidenceManifest, Path: "web/package.json", Strength: 0.95}}},
			{Name: "api", Path: "api", Confidence: 0.95},
		},
		Services: []Service{
			{Name: "postgres", Image: "postgres:17", Confidence: 0.99},
			{Name: "redis", Image: "redis:7", Confidence: 0.99, DependsOn: []string{"postgres"}, Evidence: []Evidence{{Kind: EvidenceConfig, Path: "compose.yaml", Strength: 0.99}}},
		},
	}

	graph, err := BuildTopologyGraph(model)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 4 {
		t.Fatalf("nodes=%d, want 4", len(graph.Nodes))
	}
	if len(graph.Edges) != 2 {
		t.Fatalf("edges=%d, want 2", len(graph.Edges))
	}
	if !hasEdge(graph, "component:web", "component:api") {
		t.Fatal("missing web -> api edge")
	}
	if !hasEdge(graph, "service:redis", "service:postgres") {
		t.Fatal("missing redis -> postgres edge")
	}
}

func TestTopologyEdgesRetainConfidenceAndEvidence(t *testing.T) {
	model := ProjectModel{
		Components: []Component{
			{
				Name: "web",
				DependsOn: []string{"api"},
				Evidence: []Evidence{{
					Kind: EvidenceManifest,
					Path: "web/package.json",
					Strength: 0.95,
				}},
			},
			{Name: "api"},
		},
	}

	graph, err := BuildTopologyGraph(model)
	if err != nil {
		t.Fatal(err)
	}

	if len(graph.Edges) != 1 {
		t.Fatalf("edges=%d, want 1", len(graph.Edges))
	}
	edge := graph.Edges[0]
	if edge.Confidence != 0.99 {
		t.Fatalf("confidence=%v, want 0.99", edge.Confidence)
	}
	if len(edge.Evidence) != 1 {
		t.Fatalf("evidence=%d, want 1", len(edge.Evidence))
	}
	if edge.Evidence[0].Path != "web/package.json" {
		t.Fatalf("evidence path=%q, want web/package.json", edge.Evidence[0].Path)
	}
	if edge.Evidence[0].Kind != EvidenceManifest {
		t.Fatalf("evidence kind=%q, want manifest", edge.Evidence[0].Kind)
	}
}

func TestTopologyGraphDetectsCycles(t *testing.T) {
	model := ProjectModel{
		Components: []Component{
			{Name: "a", DependsOn: []string{"b"}},
			{Name: "b", DependsOn: []string{"a"}},
		},
	}
	if _, err := BuildTopologyGraph(model); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestTopologyGraphRejectsUnknownDependency(t *testing.T) {
	model := ProjectModel{
		Components: []Component{{Name: "web", DependsOn: []string{"api"}}},
	}
	if _, err := BuildTopologyGraph(model); err == nil {
		t.Fatal("expected unknown dependency error")
	}
}

func hasEdge(graph TopologyGraph, from, to string) bool {
	for _, edge := range graph.Edges {
		if edge.From == from && edge.To == to {
			return true
		}
	}
	return false
}
