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
