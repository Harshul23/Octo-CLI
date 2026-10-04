package intelligence

import (
	"fmt"
	"net"
	"sort"
)

// PortAssignment describes a host port selected for an application component.
type PortAssignment struct {
	Component string `json:"component" yaml:"component"`
	Requested int    `json:"requested" yaml:"requested"`
	Resolved  int    `json:"resolved" yaml:"resolved"`
	Automatic bool   `json:"automatic" yaml:"automatic"`
}

// PortAllocator finds deterministic free TCP ports.
type PortAllocator struct {
	StartOffset int
}

func (a PortAllocator) Allocate(requested int, reserved map[int]struct{}) (int, bool, error) {
	if requested <= 0 {
		return 0, false, nil
	}
	start := requested
	if a.StartOffset > 0 {
		start += a.StartOffset
	}
	for port := start; port < 65536; port++ {
		if _, exists := reserved[port]; exists {
			continue
		}
		if portAvailable(port) {
			return port, port != requested, nil
		}
	}
	return 0, false, fmt.Errorf("no available TCP port found starting at %d", start)
}

func portAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func AllocateComponentPorts(components []Component) ([]PortAssignment, error) {
	ordered := append([]Component(nil), components...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })

	reserved := make(map[int]struct{})
	assignments := make([]PortAssignment, 0)
	allocator := PortAllocator{}

	for _, component := range ordered {
		if component.Port <= 0 {
			continue
		}
		port, automatic, err := allocator.Allocate(component.Port, reserved)
		if err != nil {
			return nil, fmt.Errorf("allocate port for component %q: %w", component.Name, err)
		}
		reserved[port] = struct{}{}
		assignments = append(assignments, PortAssignment{
			Component: component.Name,
			Requested: component.Port,
			Resolved: port,
			Automatic: automatic,
		})
	}
	return assignments, nil
}
