package intelligence

// Analysis is the lightweight repository analysis used by interactive flows.
type Analysis struct {
	Root string
	Name string
}

// PortConfig contains detected port information.
type PortConfig struct {
	Port       int
	Detected   bool
	FlagType   string
	IsDefault  bool
}

// AnalysisOptions configures repository analysis.
type AnalysisOptions struct {
	Environment string
}

func DefaultAnalysisOptions() AnalysisOptions {
	return AnalysisOptions{Environment: "development"}
}

// ProjectInfo is the compatibility-facing project summary exposed by the
// intelligence layer while callers migrate to ProjectModel.
type ProjectInfo struct {
	Name           string
	Language       string
	Version        string
	RunCommand     string
	PortConfig     PortConfig
	PackageManager string
	SetupCommand   string
	SetupRequired  bool
	IsMonorepo     bool
	MonorepoRoot   string
}

// AnalyzeProjectWithOptions derives the legacy-shaped summary from the native
// intelligence model. Environment-specific execution selection will be handled
// by the execution planner rather than repository detection.
func AnalyzeProjectWithOptions(path string, _ AnalysisOptions) (ProjectInfo, error) {
	model, err := Analyze(path)
	if err != nil {
		return ProjectInfo{}, err
	}

	return ProjectInfo{
		Name:           model.Name,
		Language:       model.Language,
		Version:        model.RuntimeVersion,
		RunCommand:     model.RunCommand,
		PortConfig: PortConfig{
			Port:       model.Port,
			Detected:   model.Port > 0,
			IsDefault:  model.Port > 0,
		},
		PackageManager: model.PackageManager,
		SetupCommand:   model.SetupCommand,
		SetupRequired:  model.SetupCommand != "",
		IsMonorepo:     model.Monorepo,
		MonorepoRoot:   model.Root,
	}, nil
}

func AnalyzeProject(path string) (ProjectInfo, error) {
	return AnalyzeProjectWithOptions(path, DefaultAnalysisOptions())
}
