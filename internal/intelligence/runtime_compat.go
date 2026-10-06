package intelligence

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// RuntimeCheck represents the compatibility status of a project's required runtime.
type RuntimeCheck struct {
	Runtime         string `json:"runtime"`
	RequiredVersion string `json:"required_version,omitempty"`
	DetectedVersion string `json:"detected_version,omitempty"`
	Compatible      bool   `json:"compatible"`
	VirtualEnvPath  string `json:"virtual_env_path,omitempty"`
	Message         string `json:"message,omitempty"`
}

// CheckRuntimeCompatibility evaluates whether the current host system satisfies
// the repository's runtime requirements.
func CheckRuntimeCompatibility(ctx context.Context, model ProjectModel) RuntimeCheck {
	check := RuntimeCheck{
		Runtime:         model.Language,
		RequiredVersion: model.RuntimeVersion,
		Compatible:      true,
	}

	// Detect Python virtual environment if applicable
	if strings.EqualFold(model.Language, "Python") {
		check.VirtualEnvPath = DetectVirtualEnv(model.Root)
	}

	if model.Language == "" {
		return check
	}

	var currentVersion string
	var probeErr error

	switch strings.ToLower(model.Language) {
	case "node", "javascript", "typescript":
		currentVersion, probeErr = probeCommandVersion(ctx, "node", "--version")
		if probeErr != nil {
			check.Compatible = false
			check.Message = "Node runtime not found in PATH."
			return check
		}
		check.DetectedVersion = normalizeVersion(currentVersion)
		if model.RuntimeVersion != "" && !IsVersionCompatible(model.RuntimeVersion, check.DetectedVersion) {
			check.Compatible = false
			check.Message = fmt.Sprintf("Installed Node %s does not satisfy required version constraint %s.", check.DetectedVersion, model.RuntimeVersion)
		}

	case "go", "golang":
		currentVersion, probeErr = probeCommandVersion(ctx, "go", "version")
		if probeErr != nil {
			check.Compatible = false
			check.Message = "Go toolchain not found in PATH."
			return check
		}
		check.DetectedVersion = extractGoVersion(currentVersion)
		if model.RuntimeVersion != "" && !IsGoVersionCompatible(model.RuntimeVersion, check.DetectedVersion) {
			check.Compatible = false
			check.Message = fmt.Sprintf("Installed Go %s does not satisfy required version %s.", check.DetectedVersion, model.RuntimeVersion)
		}

	case "python":
		currentVersion, probeErr = probeCommandVersion(ctx, "python3", "--version")
		if probeErr != nil {
			currentVersion, probeErr = probeCommandVersion(ctx, "python", "--version")
		}
		if probeErr != nil {
			check.Compatible = false
			check.Message = "Python interpreter not found in PATH."
			return check
		}
		check.DetectedVersion = normalizeVersion(currentVersion)
		if model.RuntimeVersion != "" && !IsVersionCompatible(model.RuntimeVersion, check.DetectedVersion) {
			check.Compatible = false
			check.Message = fmt.Sprintf("Installed Python %s does not satisfy required version constraint %s.", check.DetectedVersion, model.RuntimeVersion)
		}

	case "rust":
		currentVersion, probeErr = probeCommandVersion(ctx, "rustc", "--version")
		if probeErr != nil {
			check.Compatible = false
			check.Message = "Rust compiler (rustc) not found in PATH."
			return check
		}
		check.DetectedVersion = normalizeVersion(currentVersion)
		if model.RuntimeVersion != "" && !IsVersionCompatible(model.RuntimeVersion, check.DetectedVersion) {
			check.Compatible = false
			check.Message = fmt.Sprintf("Installed Rust %s does not satisfy required version constraint %s.", check.DetectedVersion, model.RuntimeVersion)
		}
	}

	return check
}

// DetectVirtualEnv locates an existing Python virtualenv directory in the workspace.
func DetectVirtualEnv(root string) string {
	candidates := []string{".venv", "venv", "env", ".env_py"}
	for _, c := range candidates {
		full := filepath.Join(root, c)
		// Check for bin/python or Scripts/python.exe
		if _, err := os.Stat(filepath.Join(full, "bin", "python")); err == nil {
			return full
		}
		if _, err := os.Stat(filepath.Join(full, "Scripts", "python.exe")); err == nil {
			return full
		}
		if _, err := os.Stat(filepath.Join(full, "pyvenv.cfg")); err == nil {
			return full
		}
	}
	return ""
}

func probeCommandVersion(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func normalizeVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	// Match things like v20.11.0, Python 3.11.5, rustc 1.80.0
	re := regexp.MustCompile(`(\d+(\.\d+)+)`)
	matches := re.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return matches[1]
	}
	return raw
}

func extractGoVersion(raw string) string {
	// "go version go1.24.0 darwin/arm64" -> "1.24.0"
	re := regexp.MustCompile(`go(\d+(\.\d+)*)`)
	matches := re.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return matches[1]
	}
	return normalizeVersion(raw)
}

// IsVersionCompatible evaluates if current satisfies the requirement spec.
func IsVersionCompatible(required, current string) bool {
	required = strings.TrimSpace(required)
	current = strings.TrimSpace(current)
	if required == "" || current == "" {
		return true
	}

	// Handle caret (^1.2.3 or ^20)
	if strings.HasPrefix(required, "^") {
		reqVer := strings.TrimPrefix(required, "^")
		return isCaretCompatible(reqVer, current)
	}

	// Handle tilde (~1.2.3)
	if strings.HasPrefix(required, "~") {
		reqVer := strings.TrimPrefix(required, "~")
		return isTildeCompatible(reqVer, current)
	}

	// Handle >=X.Y
	if strings.HasPrefix(required, ">=") {
		reqVer := strings.TrimSpace(strings.TrimPrefix(required, ">="))
		return compareVersions(current, reqVer) >= 0
	}

	// Handle >X.Y
	if strings.HasPrefix(required, ">") {
		reqVer := strings.TrimSpace(strings.TrimPrefix(required, ">"))
		return compareVersions(current, reqVer) > 0
	}

	// Handle <=X.Y
	if strings.HasPrefix(required, "<=") {
		reqVer := strings.TrimSpace(strings.TrimPrefix(required, "<="))
		return compareVersions(current, reqVer) <= 0
	}

	// Handle <X.Y
	if strings.HasPrefix(required, "<") {
		reqVer := strings.TrimSpace(strings.TrimPrefix(required, "<"))
		return compareVersions(current, reqVer) < 0
	}

	// Handle wildcard (e.g., 20.x, 20.*)
	if strings.HasSuffix(required, ".x") || strings.HasSuffix(required, ".*") {
		prefix := strings.TrimSuffix(strings.TrimSuffix(required, ".x"), ".*")
		return strings.HasPrefix(current, prefix)
	}

	// Exact or major-only match (e.g. required "20" vs current "20.11.0")
	reqParts := parseVersionParts(required)
	currParts := parseVersionParts(current)
	for i := range reqParts {
		if i >= len(currParts) {
			return false
		}
		if reqParts[i] != currParts[i] {
			return false
		}
	}
	return true
}

// IsGoVersionCompatible handles Go's minor-level backwards compatibility.
func IsGoVersionCompatible(required, current string) bool {
	required = strings.TrimPrefix(strings.TrimSpace(required), "go")
	current = strings.TrimPrefix(strings.TrimSpace(current), "go")
	return compareVersions(current, required) >= 0
}

func isCaretCompatible(req, curr string) bool {
	reqParts := parseVersionParts(req)
	currParts := parseVersionParts(curr)
	if len(reqParts) == 0 || len(currParts) == 0 {
		return true
	}
	// Major version must match (if non-zero)
	if reqParts[0] != 0 {
		if currParts[0] != reqParts[0] {
			return false
		}
		return compareVersions(curr, req) >= 0
	}
	// For 0.x, minor must match
	if len(reqParts) > 1 && len(currParts) > 1 {
		if currParts[1] != reqParts[1] {
			return false
		}
		return compareVersions(curr, req) >= 0
	}
	return compareVersions(curr, req) >= 0
}

func isTildeCompatible(req, curr string) bool {
	reqParts := parseVersionParts(req)
	currParts := parseVersionParts(curr)
	if len(reqParts) == 0 || len(currParts) == 0 {
		return true
	}
	if reqParts[0] != currParts[0] {
		return false
	}
	if len(reqParts) > 1 && len(currParts) > 1 && reqParts[1] != currParts[1] {
		return false
	}
	return compareVersions(curr, req) >= 0
}

func parseVersionParts(v string) []int {
	v = normalizeVersion(v)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ".")
	res := make([]int, 0, len(parts))
	for _, p := range parts {
		if n, err := strconv.Atoi(p); err == nil {
			res = append(res, n)
		} else {
			break
		}
	}
	return res
}

func compareVersions(v1, v2 string) int {
	p1 := parseVersionParts(v1)
	p2 := parseVersionParts(v2)
	maxLen := len(p1)
	if len(p2) > maxLen {
		maxLen = len(p2)
	}
	for i := 0; i < maxLen; i++ {
		n1 := 0
		if i < len(p1) {
			n1 = p1[i]
		}
		n2 := 0
		if i < len(p2) {
			n2 = p2[i]
		}
		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}
	return 0
}
