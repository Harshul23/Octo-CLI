package intelligence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverDotenvReferencesFindsStaticComponentURL(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "web")
	if err := os.MkdirAll(appDir, 0755); err != nil { t.Fatal(err) }
	path := filepath.Join(appDir, ".env")
	if err := os.WriteFile(path, []byte("API_URL=http://api:8080/v1\n"), 0600); err != nil { t.Fatal(err) }

	refs, err := discoverDotenvReferences([]string{path}, "web",
		map[string]string{"web": "component:web", "api": "component:api"}, nil)
	if err != nil { t.Fatal(err) }
	if len(refs) != 1 { t.Fatalf("references=%d, want 1", len(refs)) }
	if refs[0].Target != "api" { t.Fatalf("target=%q, want api", refs[0].Target) }
	if refs[0].Kind != networkReferenceKind { t.Fatalf("kind=%q, want %q", refs[0].Kind, networkReferenceKind) }
	if len(refs[0].Evidence) != 1 || refs[0].Evidence[0].Path != "web/.env" {
		t.Fatalf("unexpected evidence: %#v", refs[0].Evidence)
	}
}

func TestDiscoverDotenvReferencesIgnoresInterpolatedValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	value := "API_URL=http://" + "$" + "{API_HOST}:8080\nOTHER=http://api:8080\n"
	if err := os.WriteFile(path, []byte(value), 0600); err != nil { t.Fatal(err) }

	refs, err := discoverDotenvReferences([]string{path}, "web",
		map[string]string{"web": "component:web", "api": "component:api"}, nil)
	if err != nil { t.Fatal(err) }
	if len(refs) != 1 { t.Fatalf("references=%d, want 1", len(refs)) }
}

func TestDiscoverDotenvReferencesRejectsAmbiguousTarget(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("API_URL=http://api:8080\n"), 0600); err != nil { t.Fatal(err) }

	_, err := discoverDotenvReferences([]string{path}, "web",
		map[string]string{"web": "component:web", "api": "component:api"},
		map[string]string{"api": "service:api"})
	if err == nil { t.Fatal("expected ambiguous target error") }
}

func TestDiscoverDotenvReferencesNeverRetainsURLValue(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("API_URL=http://api:8080/secret-token\n"), 0600); err != nil { t.Fatal(err) }

	refs, err := discoverDotenvReferences([]string{path}, "web",
		map[string]string{"web": "component:web", "api": "component:api"}, nil)
	if err != nil { t.Fatal(err) }
	if len(refs) != 1 { t.Fatalf("references=%d, want 1", len(refs)) }
	for _, evidence := range refs[0].Evidence {
		if evidence.Detail == "http://api:8080/secret-token" || evidence.Path == "http://api:8080/secret-token" {
			t.Fatal("raw URL leaked into evidence")
		}
	}
}
