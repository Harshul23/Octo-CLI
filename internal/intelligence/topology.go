package intelligence

import (
	"fmt"
	"sort"
	"strings"
)

// NodeKind identifies the type of a topology node.
type NodeKind string

const (
	NodeComponent NodeKind = "component"
	NodeService   NodeKind = "service"
)

// TopologyNode is a source repository or infrastructure entity in the unified graph.
type TopologyNode struct {
	ID         string   `json:"id" yaml:"id"`
	Name       string   `json:"name" yaml:"name"`
	Kind       NodeKind `json:"kind" yaml:"kind"`
	Path       string   `json:"path,omitempty" yaml:"path,omitempty"`
	Confidence float64  `json:"confidence" yaml:"confidence"`
}

// TopologyEdge describes a relationship between topology nodes.
type TopologyEdge struct {
	From       string     `json:"from" yaml:"from"`
	To         string     `json:"to" yaml:"to"`
	Kind       RelationshipKind `json:"kind" yaml:"kind"`
	Confidence float64    `json:"confidence" yaml:"confidence"`
	Evidence   []Evidence `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

// TopologyGraph is the canonical dependency and relationship graph for a ProjectModel.
type TopologyGraph struct {
	Nodes []TopologyNode `json:"nodes" yaml:"nodes"`
	Edges []TopologyEdge `json:"edges" yaml:"edges"`
}

// BuildTopologyGraph converts components and services into one validated graph.
func BuildTopologyGraph(model ProjectModel) (TopologyGraph, error) {
	graph := TopologyGraph{}
	ids := make(map[string]struct{})
	nodeEvidence := make(map[string][]Evidence)

	addNode := func(node TopologyNode, evidence []Evidence) error {
		if strings.TrimSpace(node.Name) == "" {
			return fmt.Errorf("topology node has empty name")
		}
		if _, exists := ids[node.ID]; exists {
			return fmt.Errorf("duplicate topology node %q", node.ID)
		}
		ids[node.ID] = struct{}{}
		graph.Nodes = append(graph.Nodes, node)
		nodeEvidence[node.ID] = append([]Evidence(nil), evidence...)
		return nil
	}

	components := append([]Component(nil), model.Components...)
	sort.Slice(components, func(i, j int) bool { return components[i].Name < components[j].Name })
	for _, component := range components {
		if err := addNode(TopologyNode{
			ID: topologyID(NodeComponent, component.Name),
			Name: component.Name, Kind: NodeComponent,
			Path: component.Path, Confidence: component.Confidence,
		}, component.Evidence); err != nil {
			return TopologyGraph{}, err
		}
	}

	services := append([]Service(nil), model.Services...)
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	for _, service := range services {
		if err := addNode(TopologyNode{
			ID: topologyID(NodeService, service.Name),
			Name: service.Name, Kind: NodeService,
			Confidence: service.Confidence,
		}, service.Evidence); err != nil {
			return TopologyGraph{}, err
		}
	}

	addEdge := func(from, to string, kind RelationshipKind, confidence float64, evidence []Evidence) error {
		if _, ok := ids[from]; !ok {
			return fmt.Errorf("topology relationship references unknown source %q", from)
		}
		if _, ok := ids[to]; !ok {
			return fmt.Errorf("topology relationship references unknown target %q", to)
		}
		graph.Edges = append(graph.Edges, TopologyEdge{
			From: from, To: to, Kind: kind,
			Confidence: confidence, Evidence: evidence,
		})
		return nil
	}

	for _, component := range components {
		from := topologyID(NodeComponent, component.Name)
		for _, dep := range component.DependsOn {
			target, err := topologyDependencyID(dep, ids)
			if err != nil {
				return TopologyGraph{}, fmt.Errorf("component %q: %w", component.Name, err)
			}
			evidence := dependencyEvidence(nodeEvidence[from], dep, "Component dependency is explicitly declared in the component manifest.")
			if err := addEdge(from, target, RelationshipDependsOn, 0.99, evidence); err != nil {
				return TopologyGraph{}, fmt.Errorf("component %q: %w", component.Name, err)
			}
		}
		for _, reference := range component.References {
			target, err := topologyDependencyID(reference.Target, ids)
			if err != nil {
				return TopologyGraph{}, fmt.Errorf("component %q: %w", component.Name, err)
			}
			if !reference.Kind.IsKnown() {
				return TopologyGraph{}, fmt.Errorf("component %q: unknown topology relationship kind %q", component.Name, reference.Kind)
			}
			if err := addEdge(from, target, reference.Kind, reference.Confidence, reference.Evidence); err != nil {
				return TopologyGraph{}, fmt.Errorf("component %q: %w", component.Name, err)
			}
		}
	}
	for _, service := range services {
		from := topologyID(NodeService, service.Name)
		for _, dep := range service.DependsOn {
			target := topologyID(NodeService, dep)
			evidence := dependencyEvidence(nodeEvidence[from], dep, "Service dependency is explicitly declared in the Compose configuration.")
			if err := addEdge(from, target, RelationshipDependsOn, 0.99, evidence); err != nil {
				return TopologyGraph{}, fmt.Errorf("service %q: %w", service.Name, err)
			}
		}
		for _, reference := range service.References {
			target := topologyID(NodeService, reference.Target)
			if _, ok := ids[target]; !ok {
				resolved, err := topologyDependencyID(reference.Target, ids)
				if err != nil {
					return TopologyGraph{}, fmt.Errorf("service %q: %w", service.Name, err)
				}
				target = resolved
			}
			if !reference.Kind.IsKnown() {
				return TopologyGraph{}, fmt.Errorf("service %q: unknown topology relationship kind %q", service.Name, reference.Kind)
			}
			if err := addEdge(from, target, reference.Kind, reference.Confidence, reference.Evidence); err != nil {
				return TopologyGraph{}, fmt.Errorf("service %q: %w", service.Name, err)
			}
		}
	}

	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	sort.Slice(graph.Edges, func(i, j int) bool {
		if graph.Edges[i].From == graph.Edges[j].From {
			if graph.Edges[i].To == graph.Edges[j].To {
				return graph.Edges[i].Kind < graph.Edges[j].Kind
			}
			return graph.Edges[i].To < graph.Edges[j].To
		}
		return graph.Edges[i].From < graph.Edges[j].From
	})

	if err := ValidateTopologyGraph(graph); err != nil {
		return TopologyGraph{}, err
	}
	return graph, nil
}

func dependencyEvidence(source []Evidence, dependency, fallbackDetail string) []Evidence {
	evidence := make([]Evidence, 0, len(source))
	for _, item := range source {
		if item.Kind == EvidenceManifest || item.Kind == EvidenceConfig {
			detail := fallbackDetail
			if dependency != "" {
				detail += " Dependency: " + dependency + "."
			}
			evidence = append(evidence, Evidence{
				Kind: item.Kind, Path: item.Path, Detail: detail, Strength: item.Strength,
			})
		}
	}
	return evidence
}

func topologyID(kind NodeKind, name string) string {
	return string(kind) + ":" + name
}

func topologyDependencyID(name string, ids map[string]struct{}) (string, error) {
	componentID := topologyID(NodeComponent, name)
	serviceID := topologyID(NodeService, name)
	_, hasComponent := ids[componentID]
	_, hasService := ids[serviceID]

	if hasComponent && hasService {
		return "", fmt.Errorf("ambiguous dependency %q: both %q and %q exist; use an explicit node namespace", name, componentID, serviceID)
	}
	if hasComponent {
		return componentID, nil
	}
	if hasService {
		return serviceID, nil
	}
	return "", fmt.Errorf("unknown dependency %q", name)
}

// ValidateTopologyGraph rejects dangling edges, duplicate edges, and dependency cycles.
// Only depends_on edges participate in execution dependency ordering.
func ValidateTopologyGraph(graph TopologyGraph) error {
	ids := make(map[string]struct{}, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if _, exists := ids[node.ID]; exists {
			return fmt.Errorf("duplicate topology node %q", node.ID)
		}
		ids[node.ID] = struct{}{}
	}

	indegree := make(map[string]int, len(graph.Nodes))
	out := make(map[string][]string, len(graph.Nodes))
	edges := make(map[string]struct{}, len(graph.Edges))
	for _, edge := range graph.Edges {
		if _, ok := ids[edge.From]; !ok {
			return fmt.Errorf("edge references unknown source %q", edge.From)
		}
		if _, ok := ids[edge.To]; !ok {
			return fmt.Errorf("edge references unknown target %q", edge.To)
		}
		key := edge.From + "->" + edge.To + ":" + string(edge.Kind)
		if _, exists := edges[key]; exists {
			return fmt.Errorf("duplicate topology edge %q", key)
		}
		edges[key] = struct{}{}

		if edge.Kind == RelationshipDependsOn {
			indegree[edge.From]++
			out[edge.To] = append(out[edge.To], edge.From)
		}
	}

	ready := make([]string, 0)
	for id := range ids {
		if indegree[id] == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)

	visited := 0
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		visited++
		next := append([]string(nil), out[id]...)
		sort.Strings(next)
		for _, child := range next {
			indegree[child]--
			if indegree[child] == 0 {
				ready = append(ready, child)
				sort.Strings(ready)
			}
		}
	}
	if visited != len(ids) {
		return fmt.Errorf("topology graph contains a dependency cycle")
	}
	return nil
}
