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

// TopologyEdge describes an explicit dependency between topology nodes.
type TopologyEdge struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
	Kind string `json:"kind" yaml:"kind"`
}

// TopologyGraph is the canonical dependency graph for a ProjectModel.
type TopologyGraph struct {
	Nodes []TopologyNode `json:"nodes" yaml:"nodes"`
	Edges []TopologyEdge `json:"edges" yaml:"edges"`
}

// BuildTopologyGraph converts components and services into one validated graph.
func BuildTopologyGraph(model ProjectModel) (TopologyGraph, error) {
	graph := TopologyGraph{}
	ids := make(map[string]struct{})

	addNode := func(node TopologyNode) error {
		if strings.TrimSpace(node.Name) == "" {
			return fmt.Errorf("topology node has empty name")
		}
		if _, exists := ids[node.ID]; exists {
			return fmt.Errorf("duplicate topology node %q", node.ID)
		}
		ids[node.ID] = struct{}{}
		graph.Nodes = append(graph.Nodes, node)
		return nil
	}

	components := append([]Component(nil), model.Components...)
	sort.Slice(components, func(i, j int) bool { return components[i].Name < components[j].Name })
	for _, component := range components {
		if err := addNode(TopologyNode{
			ID: topologyID(NodeComponent, component.Name),
			Name: component.Name, Kind: NodeComponent,
			Path: component.Path, Confidence: component.Confidence,
		}); err != nil {
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
		}); err != nil {
			return TopologyGraph{}, err
		}
	}

	addEdge := func(from, to string) error {
		if _, ok := ids[from]; !ok {
			return fmt.Errorf("topology dependency references unknown node %q", from)
		}
		if _, ok := ids[to]; !ok {
			return fmt.Errorf("topology dependency references unknown node %q", to)
		}
		graph.Edges = append(graph.Edges, TopologyEdge{From: from, To: to, Kind: "depends_on"})
		return nil
	}

	for _, component := range components {
		from := topologyID(NodeComponent, component.Name)
		for _, dep := range component.DependsOn {
			if err := addEdge(from, topologyDependencyID(dep, ids)); err != nil {
				return TopologyGraph{}, fmt.Errorf("component %q: %w", component.Name, err)
			}
		}
	}
	for _, service := range services {
		from := topologyID(NodeService, service.Name)
		for _, dep := range service.DependsOn {
			target := topologyID(NodeService, dep)
			if err := addEdge(from, target); err != nil {
				return TopologyGraph{}, fmt.Errorf("service %q: %w", service.Name, err)
			}
		}
	}

	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	sort.Slice(graph.Edges, func(i, j int) bool {
		if graph.Edges[i].From == graph.Edges[j].From {
			return graph.Edges[i].To < graph.Edges[j].To
		}
		return graph.Edges[i].From < graph.Edges[j].From
	})

	if err := ValidateTopologyGraph(graph); err != nil {
		return TopologyGraph{}, err
	}
	return graph, nil
}

func topologyID(kind NodeKind, name string) string {
	return string(kind) + ":" + name
}

func topologyDependencyID(name string, ids map[string]struct{}) string {
	componentID := topologyID(NodeComponent, name)
	if _, ok := ids[componentID]; ok {
		return componentID
	}
	serviceID := topologyID(NodeService, name)
	if _, ok := ids[serviceID]; ok {
		return serviceID
	}
	return componentID
}

// ValidateTopologyGraph rejects dangling edges, duplicate edges, and cycles.
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
		key := edge.From + "->" + edge.To + ":" + edge.Kind
		if _, exists := edges[key]; exists {
			return fmt.Errorf("duplicate topology edge %q", key)
		}
		edges[key] = struct{}{}
		indegree[edge.From]++
		out[edge.To] = append(out[edge.To], edge.From)
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
