package intelligence

import (
    "github.com/harshul/octo-cli/internal/analyzer"
)

// DetectedProject is the minimal repository-level detection result consumed by
// the intelligence pipeline. Keeping this contract here lets the legacy
// analyzer implementation be replaced without changing the rest of the system.
type DetectedProject struct {
    Name           string
    Language       string
    Version        string
    RunCommand     string
    Port           int
    PackageManager string
    SetupCommand   string
    SetupRequired  bool
    IsMonorepo     bool
    MonorepoRoot   string
}

// ProjectDetector discovers repository facts needed by the intelligence layer.
type ProjectDetector interface {
    Detect(path string) (DetectedProject, error)
}

// LegacyProjectDetector is the temporary compatibility implementation.
// The analyzer package can be removed once a native detector implements the
// same contract.
type LegacyProjectDetector struct{}

func (LegacyProjectDetector) Detect(path string) (DetectedProject, error) {
    info, err := analyzer.AnalyzeProject(path)
    if err != nil {
        return DetectedProject{}, err
    }
    return DetectedProject{
        Name:           info.Name,
        Language:       info.Language,
        Version:        info.Version,
        RunCommand:     info.RunCommand,
        Port:           info.PortConfig.Port,
        PackageManager: info.PackageManager,
        SetupCommand:   info.SetupCommand,
        SetupRequired:  info.SetupRequired,
        IsMonorepo:     info.IsMonorepo,
        MonorepoRoot:   info.MonorepoRoot,
    }, nil
}

func detectProject(path string) (DetectedProject, error) {
    return (LegacyProjectDetector{}).Detect(path)
}
