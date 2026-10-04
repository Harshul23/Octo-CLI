package intelligence

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image string `yaml:"image"`
	Build interface{} `yaml:"build"`
	DependsOn interface{} `yaml:"depends_on"`
	Ports interface{} `yaml:"ports"`
	Environment interface{} `yaml:"environment"`
	Healthcheck interface{} `yaml:"healthcheck"`
}

func discoverComposeServices(root string) ([]Service, []Evidence, bool, error) {
	path := findComposeFile(root)
	if path == "" { return nil, nil, false, nil }
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil { return nil, nil, false, fmt.Errorf("read %s: %w", path, err) }
	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil { return nil, nil, false, fmt.Errorf("parse %s: %w", path, err) }
	if len(compose.Services) == 0 { return nil, nil, false, nil }

	names := make(map[string]struct{}, len(compose.Services))
	for name := range compose.Services { names[name] = struct{}{} }
	namesList := make([]string, 0, len(names))
	for name := range names { namesList = append(namesList, name) }
	sort.Strings(namesList)

	services := make([]Service, 0, len(namesList))
	for _, name := range namesList {
		spec := compose.Services[name]
		dependsOn := composeDependsOn(spec.DependsOn)
		validDeps := dependsOn[:0]
		for _, dep := range dependsOn {
			if _, ok := names[dep]; ok && dep != name { validDeps = append(validDeps, dep) }
		}
		sort.Strings(validDeps)

		evidence := []Evidence{{Kind: EvidenceConfig, Path: path, Detail: "Service is explicitly declared in Docker Compose.", Strength: 0.99}}
		if spec.Image != "" {
			evidence = append(evidence, Evidence{Kind: EvidenceConfig, Path: path, Detail: "Compose service declares image " + spec.Image + ".", Strength: 0.99})
		}
		if spec.Build != nil {
			evidence = append(evidence, Evidence{Kind: EvidenceConfig, Path: path, Detail: "Compose service declares a build context.", Strength: 0.99})
		}
		if len(validDeps) > 0 {
			evidence = append(evidence, Evidence{Kind: EvidenceConfig, Path: path, Detail: "Compose service declares depends_on: " + strings.Join(validDeps, ", ") + ".", Strength: 0.99})
		}

		services = append(services, Service{
			Name: name, Image: spec.Image, Build: composeBuild(spec.Build),
			DependsOn: validDeps,
			References: composeNetworkReferences(spec.Environment, names, name, path),
			Ports: composePorts(spec.Ports),
			HealthCheck: composeHealthCheck(spec.Healthcheck),
			Confidence: 0.99, Evidence: evidence,
		})
	}

	return services, []Evidence{{Kind: EvidenceConfig, Path: path, Detail: fmt.Sprintf("Discovered %d infrastructure services from Docker Compose.", len(services)), Strength: 0.99}}, true, nil
}

// composeNetworkReferences discovers static URLs whose hostname exactly matches
// another declared Compose service. Interpolated values are deliberately ignored.
func composeNetworkReferences(value interface{}, services map[string]struct{}, source, path string) []Reference {
	values := composeEnvironmentValues(value)
	refs := make([]Reference, 0)
	seen := make(map[string]struct{})

	for key, rawValue := range values {
		rawValue = strings.TrimSpace(rawValue)
		if rawValue == "" || strings.Contains(rawValue, "$"+"{") || strings.Contains(rawValue, "$"+"(") { continue }
		parsed, err := url.Parse(rawValue)
		if err != nil || parsed.Hostname() == "" { continue }
		target := parsed.Hostname()
		if target == source { continue }
		if _, ok := services[target]; !ok { continue }

		dedupeKey := target + ":" + key
		if _, ok := seen[dedupeKey]; ok { continue }
		seen[dedupeKey] = struct{}{}

		refs = append(refs, Reference{
			Target: target, Kind: RelationshipNetworkReference, Variable: key, Confidence: 0.95,
			Evidence: []Evidence{{Kind: EvidenceConfig, Path: path,
				Detail: fmt.Sprintf("Compose environment variable %q contains a static URL referencing service %q.", key, target),
				Strength: 0.95}},
		})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Target < refs[j].Target })
	return refs
}

func composeEnvironmentValues(value interface{}) map[string]string {
	out := make(map[string]string)
	switch v := value.(type) {
	case map[string]interface{}:
		for key, raw := range v { if s, ok := raw.(string); ok { out[key] = s } }
	case map[interface{}]interface{}:
		for rawKey, raw := range v {
			key, ok := rawKey.(string); if !ok { continue }
			if s, ok := raw.(string); ok { out[key] = s }
		}
	case []interface{}:
		for _, raw := range v {
			s, ok := raw.(string); if !ok { continue }
			parts := strings.SplitN(s, "=", 2)
			if len(parts) == 2 { out[parts[0]] = parts[1] }
		}
	case []string:
		for _, s := range v {
			parts := strings.SplitN(s, "=", 2)
			if len(parts) == 2 { out[parts[0]] = parts[1] }
		}
	}
	return out
}

func findComposeFile(root string) string {
	for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil { return name }
	}
	return ""
}

func composeBuild(value interface{}) string {
	switch v := value.(type) {
	case string: return v
	case map[string]interface{}:
		if context, ok := v["context"].(string); ok { return context }
	case map[interface{}]interface{}:
		if context, ok := v["context"].(string); ok { return context }
	}
	return ""
}

func composeDependsOn(value interface{}) []string {
	switch v := value.(type) {
	case []interface{}:
		out := make([]string, 0, len(v)); for _, item := range v { if name, ok := item.(string); ok { out = append(out, name) } }; return out
	case []string: return append([]string(nil), v...)
	case map[string]interface{}:
		out := make([]string, 0, len(v)); for name := range v { out = append(out, name) }; return out
	case map[interface{}]interface{}:
		out := make([]string, 0, len(v)); for name := range v { if s, ok := name.(string); ok { out = append(out, s) } }; return out
	default: return nil
	}
}

func composePorts(value interface{}) []string {
	switch v := value.(type) {
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			switch p := item.(type) {
			case string: out = append(out, p)
			case int: out = append(out, strconv.Itoa(p))
			case uint64: out = append(out, strconv.FormatUint(p, 10))
			}
		}
		sort.Strings(out); return out
	case []string:
		out := append([]string(nil), v...); sort.Strings(out); return out
	default: return nil
	}
}

func composeHealthCheck(value interface{}) *HealthCheck {
	switch v := value.(type) {
	case map[string]interface{}:
		command := composeHealthTest(v["test"]); if command == "" { return nil }
		return &HealthCheck{Command: command, Interval: composeString(v["interval"]), Timeout: composeString(v["timeout"]), Retries: composeInt(v["retries"])}
	case map[interface{}]interface{}:
		command := composeHealthTest(v["test"]); if command == "" { return nil }
		return &HealthCheck{Command: command, Interval: composeString(v["interval"]), Timeout: composeString(v["timeout"]), Retries: composeInt(v["retries"])}
	default: return nil
	}
}

func composeHealthTest(value interface{}) string {
	switch v := value.(type) {
	case string: return v
	case []interface{}:
		if len(v) == 0 { return "" }
		parts := make([]string, 0, len(v)); for _, item := range v { if s, ok := item.(string); ok { parts = append(parts, s) } }
		if len(parts) > 0 && (parts[0] == "CMD-SHELL" || parts[0] == "CMD") { parts = parts[1:] }
		return strings.Join(parts, " ")
	case []string:
		parts := append([]string(nil), v...)
		if len(parts) > 0 && (parts[0] == "CMD-SHELL" || parts[0] == "CMD") { parts = parts[1:] }
		return strings.Join(parts, " ")
	default: return ""
	}
}

func composeString(value interface{}) string { if s, ok := value.(string); ok { return s }; return "" }

func composeInt(value interface{}) int {
	switch v := value.(type) {
	case int: return v
	case uint64: return int(v)
	case float64: return int(v)
	default: return 0
	}
}
