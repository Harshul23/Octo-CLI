package intelligence

import "github.com/harshul/octo-cli/internal/analyzer"

// ProjectInfo is the compatibility-facing project summary exposed by the
// intelligence layer. Its shape remains stable while analysis moves out of
// the legacy analyzer package.
type ProjectInfo = analyzer.ProjectInfo

type PortConfig = analyzer.PortConfig
type Analysis = analyzer.Analysis
type AnalysisOptions = analyzer.AnalysisOptions

func DefaultAnalysisOptions() AnalysisOptions {
	return analyzer.DefaultAnalysisOptions()
}

// AnalyzeProjectWithOptions keeps legacy init/server consumers behind the
// intelligence boundary. The underlying implementation will be migrated
// incrementally without exposing analyzer outside this package.
func AnalyzeProjectWithOptions(path string, opts AnalysisOptions) (ProjectInfo, error) {
	return analyzer.AnalyzeProjectWithOptions(path, opts)
}

func AnalyzeProject(path string) (ProjectInfo, error) {
	return AnalyzeProjectWithOptions(path, DefaultAnalysisOptions())
}
