package intelligence

import "testing"

func TestExecutionCandidateProvidersUsesSupportedProviders(t *testing.T) {
	providers := NewExecutionCandidateProviders()
	if !providers.providers[0].Supports(Component{Language: "Go"}) {
		t.Fatal("Go provider should support Go components")
	}
	if providers.providers[0].Supports(Component{Language: "Python"}) {
		t.Fatal("Go provider should not support Python components")
	}
	if !providers.providers[1].Supports(Component{Language: "Node"}) {
		t.Fatal("Node provider should support Node components")
	}
	if providers.providers[1].Supports(Component{Language: "Go"}) {
		t.Fatal("Node provider should not support Go components")
	}
}
