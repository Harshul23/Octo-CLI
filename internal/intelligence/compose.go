package intelligence

import (
	"fmt"
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
	Image     string      `yaml:"image"`
	Build     interface{} `yaml:"build"`
	DependsOn interface{} `yaml:"depends_on"`
	Ports     interface{} `yaml:"ports"`
}

func discoverComposeServices(root string) ([]Service, []Evidence, bool, error) {
	path := findComposeFile(root)
	if path == "" {
		return nil, nil, false, nil
	}

	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return nil, nil, false, fmt.Errorf("read %s: %w", path, err)
	}

	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, nil, false, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(compose.Services) == 0 {
		return nil, nil, false, nil
	}

	names := make(map[string]struct{}, len(compose.Services))
	for name := range compose.Services {
		names[name] = struct{}{}
	}

	namesList := make([]string, 0, len(names))
	for name := range names {
		namesList = append(namesList, name)
	}
	sort.Strings(namesList)

	services := make([]Service, 0, len(namesList))
	for _, name := range namesList {
		spec := compose.Services[name]
		dependsOn := composeDependsOn(spec.DependsOn)
		validDeps := dependsOn[:0]
		for _, dep := range dependsOn {
			if _, ok := names[dep]; ok && dep != name {
				validDeps = append(validDeps, dep)
			}
		}
		sort.Strings(validDeps)

		ports := composePorts(spec.Ports)
		evidence := []Evidence{{
			Kind: EvidenceConfig,
			Path: path,
			Detail: "Service is explicitly declared in Docker Compose.",
			Strength: 0.99,
		}}
		if spec.Image != "" {
			evidence = append(evidence, Evidence{
				Kind: EvidenceConfig, Path: path,
				Detail: "Compose service declares image " + spec.Image + ".",
				Strength: 0.99,
			})
		}
		if spec.Build != nil {
			evidence = append(evidence, Evidence{
				Kind: EvidenceConfig, Path: path,
				Detail: "Compose service declares a build context.",
				Strength: 0.99,
			})
		}
		if len(validDeps) > 0 {
			evidence = append(evidence, Evidence{
				Kind: EvidenceConfig, Path: path,
				Detail: "Compose service declares depends_on: " + strings.Join(validDeps, ", ") + ".",
				Strength: 0.99,
			})
		}

		services = append(services, Service{
			Name: name,
			Image: spec.Image,
			Build: composeBuild(spec.Build),
			DependsOn: validDeps,
			Ports: ports,
			Confidence: 0.99,
			Evidence: evidence,
		})
	}

	return services, []Evidence{{
		Kind: EvidenceConfig,
		Path: path,
		Detail: fmt.Sprintf("Discovered %d infrastructure services from Docker Compose.", len(services)),
		Strength: 0.99,
	}}, true, nil
}

func findComposeFile(root string) string {
	for _, name := range []string{
		"compose.yaml",
		"compose.yml",
		"docker-compose.yaml",
		"docker-compose.yml",
	} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return name
		}
	}
	return ""
}

func composeBuild(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]interface{}:
		if context, ok := v["context"].(string); ok {
			return context
		}
	case map[interface{}]interface{}:
		if context, ok := v["context"].(string); ok {
			return context
		}
	}
	return ""
}

func composeDependsOn(value interface{}) []string {
	switch v := value.(type) {
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if name, ok := item.(string); ok {
				out = append(out, name)
			}
		}
		return out
	case []string:
		return append([]string(nil), v...)
	case map[string]interface{}:
		out := make([]string, 0, len(v))
		for name := range v {
			out = append(out, name)
		}
		return out
	case map[interface{}]interface{}:
		out := make([]string, 0, len(v))
		for name := range v {
			if s, ok := name.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func composePorts(value interface{}) []string {
	switch v := value.(type) {
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			switch p := item.(type) {
			case string:
				out = append(out, p)
			case int:
				out = append(out, strconv.Itoa(p))
			case uint64:
				out = append(out, strconv.FormatUint(p, 10))
			}
		}
		sort.Strings(out)
		return out
	case []string:
		out := append([]string(nil), v...)
		sort.Strings(out)
		return out
	default:
		return nil
	}
}
