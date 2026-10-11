# Contributing to Octo CLI

Thanks for your interest in contributing to **Octo CLI**. We're excited to have you here!

Octo's mission is simple: **Make “how do I run this?” disappear.**

A repository should contain enough evidence to derive a safe, deterministic, explainable path toward execution—for both human developers and autonomous AI coding agents.

---

## Architectural North Star

Every contribution must preserve Octo's core execution philosophy:

1. **Evidence before inference**: Never guess flags, ports, or startup commands. Every decision must be traceable to observable facts in the repository (manifests, lockfiles, scripts, explicit configs).
2. **Bounded candidates**: When multiple entry points or commands are plausible, generate a bounded set of scored candidates. Bounded ambiguity is resolved through explicit decision providers or maintainer overrides (`.octo.yaml`), never by hallucinating unsupported commands.
3. **Deterministic planning**: Repository evidence and decisions must compile into a repeatable, topological execution plan.
4. **Machine-readable core**: Every capability must be accessible to AI coding agents via strongly typed JSON (`--json`) and native Model Context Protocol (MCP) tools (`octo mcp`).
5. **Hermetic testing**: All unit tests must execute in milliseconds without external network calls or running daemon dependencies.

---

## The Contributor Recipe: Adding a New Ecosystem

To add support for a new language, framework, or runtime, contributors work against small, isolated interfaces without modifying core graph traversal or planning algorithms.

### Step 1: Register Project Signals & Detectors
In `internal/intelligence/detector.go`, register your ecosystem manifest:
```go
// Register file marker and detection function
RegisterProjectSignal(SignalDefinition{
    File:     "pubspec.yaml",
    Language: "Dart",
    Detect:   detectDartProject,
})
```

### Step 2: Implement Candidate Provider
In `internal/intelligence/candidates.go` (or an ecosystem-specific file), implement `CandidateProvider`:
```go
type DartExecutionCandidateProvider struct{}

func (DartExecutionCandidateProvider) Name() string { return "dart" }
func (DartExecutionCandidateProvider) Supports(comp Component) bool {
    return strings.EqualFold(comp.Language, "dart")
}
func (DartExecutionCandidateProvider) Candidates(ctx context.Context, root string, comp Component) ([]ExecutionCandidate, error) {
    // Return bounded, confidence-scored execution candidates backed by evidence
}
```
Register it via:
```go
RegisterCandidateProvider(DartExecutionCandidateProvider{})
```

### Step 3: Implement Verification (if custom protocol needed)
If the ecosystem requires specific health checking (e.g. custom HTTP endpoint, socket probe):
```go
RegisterVerificationProvider(&CustomVerificationProvider{})
```

### Step 4: Write Hermetic Regression Tests
Add table-driven unit tests in `internal/intelligence/` using `t.TempDir()`. Ensure tests run in milliseconds with 0 external network dependencies.

---

## Good First Issues

If you're making your first open-source contribution to Octo, look for:
- **Ecosystem Manifest Refinements**: Improving version parsing or lockfile attribution for existing languages.
- **Port Discovery Patterns**: Adding explicit port regex patterns for popular frameworks.
- **Explainability Tracing**: Adding clearer reason descriptions to `BuildDecisionTrace`.
- **CLI & Diagnostic Polish**: Improving error messages in `octo doctor` or `octo explain`.
- **Regression Tests**: Submitting test cases based on popular open-source repositories to `regression_repos_test.go`.

---

## Development Setup

### 1. Fork and Clone
```bash
git clone https://github.com/<your-username>/octo-cli.git
cd octo-cli
```

### 2. Build and Test
Octo requires Go 1.24+ and compiles with standard Go toolchains:
```bash
# Run all tests
go test -count=1 ./...

# Build local binary
go build -o octo ./cmd
```

---

## Branching & Commit Discipline

### Branch Naming
- `feat/<feature-name>` (e.g. `feat/elixir-phoenix-detection`)
- `fix/<bug-name>` (e.g. `fix/port-parsing-regex`)
- `docs/<description>` (e.g. `docs/update-architecture`)
- `test/<description>` (e.g. `test/add-monorepo-regression`)

### Conventional Commits
Use semantic, descriptive commit messages:
```
feat(intelligence): add candidate provider for Phoenix framework
fix(ports): avoid conflicting with reserved compose ports
docs(architecture): document extension provider registries
test(intelligence): add regression suite for Next.js app router
```

---

## Pull Request Guidelines

Before submitting your pull request:
1. Ensure all tests pass: `go test -count=1 ./...`.
2. Confirm the binary builds without errors: `go build -o /dev/null ./cmd`.
3. Fill out the PR template (`.github/pull_request_template.md`), explaining what changed and the evidence supporting it.
4. Keep pull requests focused on a single logical change.

---

## Release Discipline & Semantic Versioning

Octo adheres to strict Semantic Versioning (`vMAJOR.MINOR.PATCH`):
- **MAJOR**: Breaking changes to CLI schemas, MCP tool inputs/outputs, or core contracts.
- **MINOR**: New capabilities, new language/ecosystem support, new MCP tools/resources.
- **PATCH**: Bug fixes, performance improvements, and non-breaking detection refinements.

---

## Code of Conduct

Be welcoming, constructive, and respectful. Focus on technical clarity, evidence-based solutions, and collaborative progress.
