package intelligence

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const networkReferenceKind = "network_reference"

// discoverComponentNetworkReferences finds explicit static URL references in
// component-local dotenv files. Values are used only for matching and are never
// retained as evidence.
func discoverComponentNetworkReferences(root string, components []Component, services []Service) error {
	componentIDs := make(map[string]string, len(components))
	serviceIDs := make(map[string]string, len(services))

	for _, component := range components {
		componentIDs[component.Name] = topologyID(NodeComponent, component.Name)
	}
	for _, service := range services {
		serviceIDs[service.Name] = topologyID(NodeService, service.Name)
	}

	for i := range components {
		files, err := dotenvFiles(filepath.Join(root, filepath.FromSlash(components[i].Path)))
		if err != nil {
			return err
		}
		refs, err := discoverDotenvReferences(files, components[i].Name, componentIDs, serviceIDs)
		if err != nil {
			return err
		}
		components[i].References = mergeReferences(components[i].References, refs)
	}

	return nil
}

func dotenvFiles(componentRoot string) ([]string, error) {
	entries, err := os.ReadDir(componentRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == ".env" || strings.HasPrefix(name, ".env.") {
			files = append(files, filepath.Join(componentRoot, name))
		}
	}
	sort.Strings(files)
	return files, nil
}

func discoverDotenvReferences(files []string, sourceComponent string, componentIDs, serviceIDs map[string]string) ([]Reference, error) {
	refs := make([]Reference, 0)

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			key, value, ok := parseDotenvAssignment(scanner.Text())
			if !ok || value == "" || strings.Contains(value, "$"+"{") || strings.Contains(value, "$"+"(") {
				continue
			}

			parsed, err := url.Parse(value)
			if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
				continue
			}

			targetName := parsed.Hostname()
			var targetID string
			if id, ok := componentIDs[targetName]; ok {
				targetID = id
			}
			if id, ok := serviceIDs[targetName]; ok {
				if targetID != "" {
					_ = f.Close()
					return nil, fmt.Errorf("ambiguous network reference target %q: both component:%s and service:%s exist", targetName, targetName, targetName)
				}
				targetID = id
			}
			if targetID == "" || targetID == topologyID(NodeComponent, sourceComponent) {
				continue
			}

			refs = append(refs, Reference{
				Target: targetName,
				Kind: networkReferenceKind,
				Confidence: 0.95,
				Evidence: []Evidence{{
					Kind: EvidenceConfig,
					Path: filepath.ToSlash(file),
					Detail: "Static environment variable " + key + " references a known application or service by hostname.",
					Strength: 0.95,
				}},
			})
		}
		if err := scanner.Err(); err != nil {
			_ = f.Close()
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	}

	return mergeReferences(nil, refs), nil
}

func parseDotenvAssignment(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" {
		return "", "", false
	}
	value = strings.TrimSpace(strings.Trim(value, "'\""))
	if idx := strings.Index(value, " #"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	return key, value, true
}

func mergeReferences(existing, discovered []Reference) []Reference {
	byTarget := make(map[string]Reference, len(existing)+len(discovered))
	for _, ref := range append(existing, discovered...) {
		key := string(ref.Kind) + "\x00" + ref.Target
		if current, ok := byTarget[key]; ok {
			current.Evidence = append(current.Evidence, ref.Evidence...)
			if ref.Confidence > current.Confidence {
				current.Confidence = ref.Confidence
			}
			byTarget[key] = current
			continue
		}
		byTarget[key] = ref
	}

	out := make([]Reference, 0, len(byTarget))
	for _, ref := range byTarget {
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Target < out[j].Target
	})
	return out
}
