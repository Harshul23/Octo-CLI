package intelligence

import (
 "os"
 "path/filepath"
 "testing"
)

func TestAnalyzeNextProjectProducesEvidence(t *testing.T) {
 root:=t.TempDir()
 if err:=os.WriteFile(filepath.Join(root,"package.json"),[]byte(`{"name":"demo","scripts":{"dev":"next dev"},"dependencies":{"next":"15.0.0","react":"19.0.0"}}`),0644);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(root,"pnpm-lock.yaml"),[]byte("lockfileVersion: '9.0'"),0644);err!=nil{t.Fatal(err)}
 m,err:=Analyze(root);if err!=nil{t.Fatal(err)}
 if m.Language!="Node"{t.Fatalf("language=%q",m.Language)}
 if m.Framework!="Next.js"{t.Fatalf("framework=%q",m.Framework)}
 if m.PackageManager!="pnpm"{t.Fatalf("package manager=%q",m.PackageManager)}
 if m.RunCommand==""{t.Fatal("expected run command")}
 if len(m.Evidence)<3{t.Fatalf("evidence=%d",len(m.Evidence))}
}


func TestAnalyzeLeavesLibraryWithoutPrimaryRunCommand(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/library\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	docs := filepath.Join(root, "docs", "readme")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	example := filepath.Join(root, "examples", "hello")
	if err := os.MkdirAll(example, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(example, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	model, err := Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	if model.RunCommand != "" {
		t.Fatalf("run command=%q, want none", model.RunCommand)
	}
	if len(model.Components) != 1 || model.Components[0].RunCommand != "" {
		t.Fatalf("component=%+v, want no run command", model.Components)
	}
	if len(model.Components[0].ExecutionCandidates) != 2 {
		t.Fatalf("candidates=%+v, want all auxiliary candidates preserved", model.Components[0].ExecutionCandidates)
	}
}
