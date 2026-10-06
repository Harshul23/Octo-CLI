package intelligence

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// EnvironmentStatus summarizes present and missing environment variables for a project.
type EnvironmentStatus struct {
	TotalVariables  int      `json:"total_variables"`
	Present         []string `json:"present"`
	MissingRequired []string `json:"missing_required"`
	MissingOptional []string `json:"missing_optional"`
	Template        string   `json:"template,omitempty"`
}

// InspectEnvironmentStatus scans the repository to compare declared variables against
// active process and local file environment variables.
func InspectEnvironmentStatus(root string, model ProjectModel) EnvironmentStatus {
	// Read existing local files
	existing := make(map[string]string)
	for _, f := range []string{".env", ".env.local"} {
		vals, _ := readEnvFile(filepath.Join(root, f))
		for k, v := range vals {
			existing[k] = v
		}
	}
	// Caller's process environment
	for _, entry := range os.Environ() {
		name, value, ok := splitAssignment(entry)
		if ok && name != "" && value != "" {
			existing[name] = value
		}
	}

	status := EnvironmentStatus{
		TotalVariables: len(model.Environment.Variables),
	}

	for _, v := range model.Environment.Variables {
		val, ok := existing[v.Name]
		if ok && strings.TrimSpace(val) != "" {
			status.Present = append(status.Present, v.Name)
		} else if v.Required {
			status.MissingRequired = append(status.MissingRequired, v.Name)
		} else {
			status.MissingOptional = append(status.MissingOptional, v.Name)
		}
	}

	sort.Strings(status.Present)
	sort.Strings(status.MissingRequired)
	sort.Strings(status.MissingOptional)

	status.Template = GenerateEnvTemplate(root, model)
	return status
}

// GenerateEnvTemplate produces safe, placeholder-only .env template content.
// It never outputs secret values.
func GenerateEnvTemplate(root string, model ProjectModel) string {
	if len(model.Environment.Variables) == 0 {
		return "# No environment variables detected for this repository.\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Environment configuration template for %s\n", model.Name))
	sb.WriteString("# Generated automatically by Octo (zero secrets exposed)\n\n")

	// Split into required and optional
	var required []EnvironmentVariable
	var optional []EnvironmentVariable

	for _, v := range model.Environment.Variables {
		if v.Required {
			required = append(required, v)
		} else {
			optional = append(optional, v)
		}
	}

	if len(required) > 0 {
		sb.WriteString("### REQUIRED VARIABLES ###\n")
		for _, v := range required {
			src := ""
			if len(v.Sources) > 0 {
				src = fmt.Sprintf(" (required by: %s)", strings.Join(v.Sources, ", "))
			}
			sb.WriteString(fmt.Sprintf("#%s\n", src))
			sb.WriteString(fmt.Sprintf("%s=%s\n\n", v.Name, placeholderFor(v.Name, model)))
		}
	}

	if len(optional) > 0 {
		sb.WriteString("### OPTIONAL / DETECTED VARIABLES ###\n")
		for _, v := range optional {
			src := ""
			if len(v.Sources) > 0 {
				src = fmt.Sprintf(" (referenced in: %s)", strings.Join(v.Sources, ", "))
			}
			sb.WriteString(fmt.Sprintf("#%s\n", src))
			sb.WriteString(fmt.Sprintf("# %s=%s\n\n", v.Name, placeholderFor(v.Name, model)))
		}
	}

	return sb.String()
}

func placeholderFor(name string, model ProjectModel) string {
	upper := strings.ToUpper(name)
	switch {
	case upper == "PORT":
		if model.Port > 0 {
			return fmt.Sprintf("%d", model.Port)
		}
		return "3000"
	case strings.Contains(upper, "DATABASE_URL") || strings.Contains(upper, "DB_URL"):
		return "postgresql://postgres:postgres@localhost:5432/dbname"
	case strings.Contains(upper, "REDIS_URL"):
		return "redis://localhost:6379"
	case upper == "NODE_ENV" || upper == "ENVIRONMENT" || upper == "ENV" || upper == "APP_ENV":
		return "development"
	case upper == "DEBUG":
		return "true"
	case strings.Contains(upper, "HOST"):
		return "localhost"
	case strings.HasSuffix(upper, "_KEY") || strings.HasSuffix(upper, "_SECRET") || strings.HasSuffix(upper, "_TOKEN"):
		return "your_" + strings.ToLower(name) + "_here"
	default:
		return "your_" + strings.ToLower(name) + "_here"
	}
}
