package intelligence

import "sort"

// ResolveEnvironmentBindings derives secret-safe environment bindings from
// explicit topology references. It never reads or stores environment values.
func ResolveEnvironmentBindings(model ProjectModel) []EnvironmentResolution {
	resolutions := make([]EnvironmentResolution, 0)
	for _, component := range model.Components {
		for _, ref := range component.References {
			if ref.Variable == "" || !ref.Kind.IsKnown() {
				continue
			}
			resolutions = append(resolutions, EnvironmentResolution{
				Variable:   ref.Variable,
				Source:     topologyID(NodeComponent, component.Name),
				Target:     ref.Target,
				Kind:       ref.Kind,
				Confidence: ref.Confidence,
				Evidence:   append([]Evidence(nil), ref.Evidence...),
			})
		}
	}
	for _, service := range model.Services {
		for _, ref := range service.References {
			if ref.Variable == "" || !ref.Kind.IsKnown() {
				continue
			}
			resolutions = append(resolutions, EnvironmentResolution{
				Variable:   ref.Variable,
				Source:     topologyID(NodeService, service.Name),
				Target:     ref.Target,
				Kind:       ref.Kind,
				Confidence: ref.Confidence,
				Evidence:   append([]Evidence(nil), ref.Evidence...),
			})
		}
	}
	return sortEnvironmentResolutions(resolutions)
}

func sortEnvironmentResolutions(values []EnvironmentResolution) []EnvironmentResolution {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Source != values[j].Source {
			return values[i].Source < values[j].Source
		}
		if values[i].Variable != values[j].Variable {
			return values[i].Variable < values[j].Variable
		}
		if values[i].Target != values[j].Target {
			return values[i].Target < values[j].Target
		}
		return values[i].Kind < values[j].Kind
	})
	return values
}
