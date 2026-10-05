package intelligence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var explicitPortPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?:--port(?:=|\s+)|-p(?:=|\s+))(\d{1,5})\b`),
	regexp.MustCompile(`(?:^|\s)PORT=(\d{1,5})(?:\s|$)`),
	regexp.MustCompile(`-Dserver\.port=(\d{1,5})\b`),
	regexp.MustCompile(`(?:https?://|localhost:|127\.0\.0\.1:|0\.0\.0\.0:)(\d{4,5})\b`),
}

// discoverComponentPort returns a port only when repository evidence explicitly
// declares one. It deliberately does not infer ports from languages or frameworks.
func discoverComponentPort(root string, component Component, command string) (int, []Evidence, error) {
	componentRoot := root
	if component.Path != "" && component.Path != "." {
		componentRoot = filepath.Join(root, filepath.FromSlash(component.Path))
	}

	if port, ok := extractExplicitPort(command); ok {
		return port, []Evidence{{
			Kind: EvidenceScript,
			Path: filepath.ToSlash(filepath.Join(component.Path, "run command")),
			Detail: "Selected execution command explicitly declares TCP port " + strconv.Itoa(port) + ".",
			Strength: 0.98,
		}}, nil
	}

	if component.Language == "Node" {
		if port, evidence, err := discoverNodeScriptPort(componentRoot, component.Path, command); err != nil {
			return 0, nil, err
		} else if port > 0 {
			return port, evidence, nil
		}
	}

	if component.Language == "Java" {
		if port, evidence, err := discoverJavaApplicationPort(componentRoot, component.Path); err != nil {
			return 0, nil, err
		} else if port > 0 {
			return port, evidence, nil
		}
	}

	return 0, nil, nil
}

func extractExplicitPort(value string) (int, bool) {
	for _, pattern := range explicitPortPatterns {
		matches := pattern.FindStringSubmatch(value)
		if len(matches) < 2 {
			continue
		}
		port, err := strconv.Atoi(matches[1])
		if err == nil && port > 0 && port < 65536 {
			return port, true
		}
	}
	return 0, false
}

func discoverNodeScriptPort(root, componentPath, command string) (int, []Evidence, error) {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		return 0, nil, err
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return 0, nil, err
	}

	fields := strings.Fields(command)
	if len(fields) == 0 {
		return 0, nil, nil
	}
	script := fields[len(fields)-1]
	run, ok := pkg.Scripts[script]
	if !ok {
		return 0, nil, nil
	}
	port, ok := extractExplicitPort(run)
	if !ok {
		return 0, nil, nil
	}
	return port, []Evidence{{
		Kind: EvidenceScript,
		Path: filepath.ToSlash(filepath.Join(componentPath, "package.json")),
		Detail: "The selected package script explicitly declares TCP port " + strconv.Itoa(port) + ".",
		Strength: 0.97,
	}}, nil
}

func discoverJavaApplicationPort(root, componentPath string) (int, []Evidence, error) {
	paths := []string{
		filepath.Join(root, "src", "main", "resources", "application.properties"),
		filepath.Join(root, "application.properties"),
	}
	pattern := regexp.MustCompile(`(?m)^\s*server\.port\s*=\s*(\d+)\s*$`)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, nil, err
		}
		matches := pattern.FindStringSubmatch(string(data))
		if len(matches) < 2 {
			continue
		}
		port, err := strconv.Atoi(matches[1])
		if err != nil || port <= 0 || port >= 65536 {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(componentPath, strings.TrimPrefix(path, root+string(filepath.Separator))))
		return port, []Evidence{{
			Kind: EvidenceConfig,
			Path: rel,
			Detail: "Spring application configuration explicitly declares server.port " + strconv.Itoa(port) + ".",
			Strength: 0.98,
		}}, nil
	}
	return 0, nil, nil
}
