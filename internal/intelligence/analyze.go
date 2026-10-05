package intelligence

import (
  "context"
  "encoding/json"
  "os"
  "path/filepath"
  "strings"
    "github.com/harshul/octo-cli/internal/secrets"
  "sort"
)

func Analyze(path string) (ProjectModel, error) {
  info, err := detectProject(path)
  if err != nil { return ProjectModel{}, err }
  root, err := filepath.Abs(path)
  if err != nil { return ProjectModel{}, err }

  m := ProjectModel{Name: info.Name, Root: root, Language: info.Language, RuntimeVersion: info.Version,
    PackageManager: info.PackageManager, RunCommand: info.RunCommand, SetupCommand: info.SetupCommand,
    Monorepo: info.IsMonorepo, Port: info.Port, Confidence: 0.20}

  if f := signalFile(root, info.Language); f != "" {
    m.Evidence = append(m.Evidence, Evidence{Kind: EvidenceSignalFile, Path:f, Detail:"Primary language signal.", Strength:0.85})
  }
  if f := lockfile(root, info.PackageManager); f != "" {
    m.Evidence = append(m.Evidence, Evidence{Kind: EvidenceLockfile, Path:f, Detail:"Lockfile supports the detected package manager.", Strength:0.95})
  }
   if f := monorepoMarker(root); f != "" {
    m.Evidence = append(m.Evidence, Evidence{Kind: EvidenceConfig, Path:f, Detail:"Workspace configuration indicates a monorepo.", Strength:0.95})
  }
  m.Framework = detectFramework(root, info.Language)
  if m.Framework != "" {
    m.Evidence = append(m.Evidence, Evidence{Kind:EvidenceManifest, Path:frameworkPath(root, info.Language),
      Detail:"Detected framework: "+m.Framework, Strength:0.90})
  }
  m.Confidence = confidence(m.Evidence, false)
  components, workspaceEvidence, discovered, err := discoverWorkspaceComponents(root, info)
  if err != nil { return ProjectModel{}, err }
  m.Evidence = append(m.Evidence, workspaceEvidence...)
  if discovered {
    m.Components = components
    m.Monorepo = true
  } else {
    m.Components = []Component{{Name:m.Name, Path:".", Language:m.Language, Framework:m.Framework,
      PackageManager:m.PackageManager, RunCommand:m.RunCommand, Port:m.Port, Confidence:m.Confidence, Evidence:m.Evidence}}
  }
  envVars, err := discoverEnvironment(root, info.Language)
  if err != nil { return ProjectModel{}, err }
  m.Environment = envVars

  services, composeEvidence, composeFound, err := discoverComposeServices(root)
  if err != nil { return ProjectModel{}, err }
  if composeFound {
    m.Services = services
    m.Evidence = append(m.Evidence, composeEvidence...)
  }
  if err := discoverComponentNetworkReferences(root, m.Components, m.Services); err != nil {
    return ProjectModel{}, err
  }
  m.Environment.Resolutions = ResolveEnvironmentBindings(m)

  providers := NewExecutionCandidateProviders()
	decisionProvider := OptionalDecisionProvider()
  for i := range m.Components {
    candidates, err := providers.Candidates(context.Background(), root, m.Components[i])
    if err != nil {
      return ProjectModel{}, err
    }
    m.Components[i].ExecutionCandidates = candidates
    if len(candidates) == 0 {
      m.Components[i].RunCommand = ""
      continue
    }
    selected, err := SelectExecutionCandidateWithProvider(context.Background(), candidates, decisionProvider)
    if err != nil {
      return ProjectModel{}, err
    }
    m.Components[i].RunCommand = selected.Command
    m.Components[i].Confidence = selected.Confidence

    port, portEvidence, portStrict, err := discoverComponentPort(root, m.Components[i], selected.Command)
    if err != nil {
      return ProjectModel{}, err
    }
    if port > 0 {
      m.Components[i].Port = port
      m.Components[i].PortStrict = portStrict
      m.Components[i].Evidence = append(m.Components[i].Evidence, portEvidence...)
      if m.Components[i].Path == "." && i == 0 {
        m.Port = port
        m.Evidence = append(m.Evidence, portEvidence...)
      }
    }

    if m.Components[i].Path == "." && i == 0 {
      m.RunCommand = selected.Command
      m.Confidence = confidence(m.Evidence, true)
    }
  }
  return m,nil
}

func signalFile(root, lang string) string {
  files:=map[string]string{"Node":"package.json","Java":"pom.xml","Python":"pyproject.toml","Go":"go.mod","Rust":"Cargo.toml","Ruby":"Gemfile"}
  f:=files[lang]; if f=="" { return "" }; if _,e:=os.Stat(filepath.Join(root,f)); e!=nil{return ""}; return f
}
func lockfile(root, pm string) string {
  files:=map[string]string{"pnpm":"pnpm-lock.yaml","yarn":"yarn.lock","bun":"bun.lock","npm":"package-lock.json"}
  f:=files[pm]; if f=="" {return ""}; if _,e:=os.Stat(filepath.Join(root,f));e!=nil{return ""};return f
}
func monorepoMarker(root string) string {
  for _,f:=range []string{"pnpm-workspace.yaml","nx.json","turbo.json","lerna.json","rush.json"} {
    if _,e:=os.Stat(filepath.Join(root,f));e==nil{return f}
  }; return ""
}
func detectFramework(root, lang string) string {
  if lang=="Node" {
    data,e:=os.ReadFile(filepath.Join(root,"package.json")); if e!=nil{return ""}
    var p struct{Dependencies map[string]string `json:"dependencies"`; DevDependencies map[string]string `json:"devDependencies"`}
    if json.Unmarshal(data,&p)!=nil{return ""}
    deps:=map[string]bool{};for k:=range p.Dependencies{deps[k]=true};for k:=range p.DevDependencies{deps[k]=true}
    for _,f:=range []struct{name,dep string}{{"Next.js","next"},{"React","react"},{"Vue","vue"},{"Nuxt","nuxt"},{"Svelte","svelte"},{"Express","express"},{"Fastify","fastify"},{"NestJS","@nestjs/core"}}{
      if deps[f.dep]{return f.name}
    }
  }
  if lang=="Python" {
    for _,f:=range []string{"requirements.txt","pyproject.toml"}{
      data,e:=os.ReadFile(filepath.Join(root,f));if e!=nil{continue};s:=strings.ToLower(string(data))
      if strings.Contains(s,"fastapi"){return "FastAPI"};if strings.Contains(s,"django"){return "Django"};if strings.Contains(s,"flask"){return "Flask"}
    }
  }
  return ""
}
func frameworkPath(root,lang string) string {
  if lang=="Node"{return "package.json"};if _,e:=os.Stat(filepath.Join(root,"requirements.txt"));e==nil{return "requirements.txt"};return "pyproject.toml"
}
func confidence(ev []Evidence, hasRun bool) float64 {
  if len(ev)==0{return 0.20};var sum float64;for _,e:=range ev{sum+=e.Strength};c:=sum/float64(len(ev));if hasRun{c+=0.10};if c>0.99{c=0.99};return c
}


func discoverEnvironment(root, language string) (EnvironmentModel, error) {
	vars, err := secrets.ScanForEnvVars(root, language)
	if err != nil {
		return EnvironmentModel{}, err
	}

	byName := make(map[string]EnvironmentVariable)
	for _, v := range vars {
		item := byName[v.Name]
		item.Name = v.Name
		item.Required = item.Required || v.Required
		if v.File != "" {
			rel, err := filepath.Rel(root, v.File)
			if err == nil {
				rel = filepath.ToSlash(rel)
				item.Sources = append(item.Sources, rel)
			}
		}
		byName[v.Name] = item
	}

	out := EnvironmentModel{Variables: make([]EnvironmentVariable, 0, len(byName))}
	for _, item := range byName {
		sort.Strings(item.Sources)
		out.Variables = append(out.Variables, item)
	}
	sort.Slice(out.Variables, func(i, j int) bool { return out.Variables[i].Name < out.Variables[j].Name })
	return out, nil
}
