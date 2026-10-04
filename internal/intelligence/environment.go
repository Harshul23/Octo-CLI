package intelligence

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ResolvedEnvironment contains runtime-only environment values.
// It must never be embedded in ProjectModel or ExecutionPlan.
type ResolvedEnvironment struct {
	Values map[string]string
	// ScopedValues contains component-local runtime values keyed by the
	// component work directory. Values never enter ProjectModel or ExecutionPlan.
	ScopedValues map[string]map[string]string
}

func (e ResolvedEnvironment) ForStep(step ExecutionStep) ResolvedEnvironment {
	values := make(map[string]string, len(e.Values))
	for name, value := range e.Values {
		values[name] = value
	}
	if scoped, ok := e.ScopedValues[step.WorkDir]; ok {
		for name, value := range scoped {
			values[name] = value
		}
	}
	// The caller's process environment is always authoritative.
	for _, entry := range os.Environ() {
		name, value, ok := splitEnv(entry)
		if ok {
			values[name] = value
		}
	}
	return ResolvedEnvironment{Values: values}
}

// ResolveEnvironment resolves required and optional variables for a project.
// Precedence is: existing process environment > .env.local > .env.
// Only variables declared in the ProjectModel are returned.
func ResolveEnvironment(root string, model EnvironmentModel) (ResolvedEnvironment, error) {
	values := make(map[string]string)

	for _, path := range []string{filepath.Join(root, ".env"), filepath.Join(root, ".env.local")} {
		fileValues, err := readEnvFile(path)
		if err != nil {
			return ResolvedEnvironment{}, err
		}
		for name, value := range fileValues {
			values[name] = value
		}
	}

	// The caller's environment is authoritative over project files.
	for _, entry := range os.Environ() {
		name, value, ok := splitEnv(entry)
		if ok {
			values[name] = value
		}
	}

	resolved := make(map[string]string)
	var missing []string
	for _, variable := range model.Variables {
		value, ok := values[variable.Name]
		if !ok || value == "" {
			if variable.Required {
				missing = append(missing, variable.Name)
			}
			continue
		}
		resolved[variable.Name] = value
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		return ResolvedEnvironment{}, fmt.Errorf("missing required environment variables: %v", missing)
	}

	scoped := make(map[string]map[string]string)
	for _, component := range model.Components {
		if component.Path == "" || component.Path == "." {
			continue
		}
		componentRoot := filepath.Join(root, filepath.FromSlash(component.Path))
		componentValues := make(map[string]string)
		for _, path := range []string{filepath.Join(componentRoot, ".env"), filepath.Join(componentRoot, ".env.local")} {
			fileValues, err := readEnvFile(path)
			if err != nil {
				return ResolvedEnvironment{}, err
			}
			for name, value := range fileValues {
				componentValues[name] = value
			}
		}
		if len(componentValues) > 0 {
			scoped[component.Path] = componentValues
		}
	}

	return ResolvedEnvironment{Values: resolved, ScopedValues: scoped}, nil
}

func readEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read environment file %s: %w", path, err)
	}

	values := make(map[string]string)
	for _, line := range splitLines(string(data)) {
		line = trimSpace(line)
		if line == "" || hasPrefix(line, "#") {
			continue
		}
		name, value, ok := splitAssignment(line)
		if !ok || name == "" {
			continue
		}
		values[name] = trimQuotes(value)
	}
	return values, nil
}

func splitLines(value string) []string { return split(value, "\n") }
func trimSpace(value string) string { return trim(value, " \t\r") }
func hasPrefix(value, prefix string) bool { return len(value) >= len(prefix) && value[:len(prefix)] == prefix }
func trim(value, cutset string) string {
	start, end := 0, len(value)
	for start < end && containsRune(cutset, value[start]) { start++ }
	for end > start && containsRune(cutset, value[end-1]) { end-- }
	return value[start:end]
}
func containsRune(set string, b byte) bool {
	for i := 0; i < len(set); i++ {
		if set[i] == b { return true }
	}
	return false
}
func split(value, sep string) []string {
	var out []string
	for {
		i := index(value, sep)
		if i < 0 { return append(out, value) }
		out = append(out, value[:i])
		value = value[i+len(sep):]
	}
}
func index(value, sep string) int {
	for i := 0; i+len(sep) <= len(value); i++ {
		if value[i:i+len(sep)] == sep { return i }
	}
	return -1
}
func splitAssignment(line string) (string, string, bool) {
	i := index(line, "=")
	if i < 0 { return "", "", false }
	return trimSpace(line[:i]), trimSpace(line[i+1:]), true
}
func splitEnv(entry string) (string, string, bool) { return splitAssignment(entry) }
func trimQuotes(value string) string {
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		return value[1:len(value)-1]
	}
	return value
}
