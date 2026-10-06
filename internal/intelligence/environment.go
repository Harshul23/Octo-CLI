package intelligence

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
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
	return resolveEnvironment(root, model, nil)
}

func ResolveProjectEnvironment(root string, project ProjectModel) (ResolvedEnvironment, error) {
	env, err := resolveEnvironment(root, project.Environment, project.Components)
	if err != nil {
		return env, err
	}
	if strings.EqualFold(project.Language, "Python") {
		if venv := DetectVirtualEnv(root); venv != "" {
			venvBin := filepath.Join(venv, "bin")
			if runtime.GOOS == "windows" {
				venvBin = filepath.Join(venv, "Scripts")
			}
			currentPath := os.Getenv("PATH")
			if env.Values == nil {
				env.Values = make(map[string]string)
			}
			env.Values["PATH"] = venvBin + string(os.PathListSeparator) + currentPath
			env.Values["VIRTUAL_ENV"] = venv
		}
	}
	return env, nil
}

func resolveEnvironment(root string, model EnvironmentModel, components []Component) (ResolvedEnvironment, error) {
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

	// Component-local values are isolated by component path.
	scoped := make(map[string]map[string]string)
	for _, component := range components {
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

	// The caller's environment is authoritative over all project files.
	for _, entry := range os.Environ() {
		name, value, ok := splitEnv(entry)
		if ok {
			values[name] = value
			for path := range scoped {
				if scoped[path] == nil {
					scoped[path] = make(map[string]string)
				}
				scoped[path][name] = value
			}
		}
	}

	// Validate required variables against the shared environment or the
	// component scopes that actually reference them. A value in one
	// component must never satisfy a requirement belonging to another.
	resolved := make(map[string]string)
	var missing []string
	for _, variable := range model.Variables {
		if value, ok := values[variable.Name]; ok && value != "" {
			resolved[variable.Name] = value
			continue
		}
		if !variable.Required {
			continue
		}

		scopes, ownershipKnown := requiredEnvironmentScopes(variable, components)
		if !ownershipKnown {
			// Legacy/source-less variables have no reliable component owner.
			// Preserve the pre-scoping behavior: any component-local value can
			// satisfy the requirement, while runtime isolation still applies.
			found := false
			for _, componentValues := range scoped {
				if value, ok := componentValues[variable.Name]; ok && value != "" {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, variable.Name)
			}
			continue
		}
		if len(scopes) == 0 {
			// The variable has known ownership, but no known component owns
			// the source. It therefore requires a shared/root value.
			missing = append(missing, variable.Name)
			continue
		}
		for _, scope := range scopes {
			componentValues := scoped[scope]
			if value, ok := componentValues[variable.Name]; !ok || value == "" {
				missing = append(missing, variable.Name)
				break
			}
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		return ResolvedEnvironment{}, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return ResolvedEnvironment{Values: resolved, ScopedValues: scoped}, nil
}


func requiredEnvironmentScopes(variable EnvironmentVariable, components []Component) ([]string, bool) {
	if len(variable.Sources) == 0 {
		// No source ownership was recorded. The caller must use the legacy
		// any-component-scope fallback.
		return nil, false
	}
	if len(components) == 0 {
		// Sources exist, but there are no component scopes to own them.
		return nil, true
	}

	// Prefer the most specific component path when components are nested.
	matched := make(map[string]struct{})
	for _, source := range variable.Sources {
		source = filepath.ToSlash(filepath.Clean(source))
		best := ""
		for _, component := range components {
			path := filepath.ToSlash(filepath.Clean(component.Path))
			if path == "" || path == "." {
				continue
			}
			prefix := path + "/"
			if source == path || len(source) > len(prefix) && source[:len(prefix)] == prefix {
				if len(path) > len(best) {
					best = path
				}
			}
		}
		if best != "" {
			matched[best] = struct{}{}
		} else {
			// A source outside every component requires a shared value.
			return nil, true
		}
	}

	scopes := make([]string, 0, len(matched))
	for scope := range matched {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return scopes, true
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
