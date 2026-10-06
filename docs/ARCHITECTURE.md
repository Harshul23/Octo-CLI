# Octo Architecture

This document defines the architectural direction of Octo. It is a guide for future changes, not permission to bypass existing contracts.

## Canonical pipeline

```text
Repository
    ↓
Evidence Discovery
    ↓
ProjectModel
    ↓
Topology
    ↓
Execution Candidates
    ↓
Bounded Decision
    ↓
ExecutionPlan
    ↓
Runtime
    ↓
Verification
    ↓
Verified Strategy
```

Each stage should have a clear responsibility.

## Evidence discovery

Discover facts from repository artifacts:

- manifests and lockfiles
- package scripts
- source files
- workspace configuration
- Compose files
- runtime declarations
- explicit health checks
- configuration
- relevant execution failures

Evidence should be attributable to something observable.

## ProjectModel

`ProjectModel` is the canonical representation of repository analysis.

It should describe:

- what exists
- where it lives
- runtime/ecosystem information
- visible dependencies
- execution candidates
- environment requirements
- confidence and supporting evidence

The model is descriptive. It should not silently contain speculative execution policy.

## Topology

Topology describes relationships between components and infrastructure.

```text
component:<name>
service:<name>
```

Relationship semantics:

- `depends_on` — affects execution order
- `network_reference` — describes communication without automatically creating startup order

Topology should remain canonical so analysis, planning, explanation, and execution do not invent separate dependency models.

## Execution candidates

Candidate providers turn repository evidence into possible execution strategies.

A candidate should carry enough information to compare it:

- stable identifier
- command
- confidence
- evidence
- classification/kind

Candidates should be bounded.

> “I found these three plausible strategies, but none is safe enough to choose.”

is better than inventing a fourth.

## Decision providers

Decision providers resolve bounded ambiguity.

Conceptually:

```go
type DecisionProvider interface {
    Decide(ctx context.Context, request DecisionRequest) (DecisionResult, error)
}
```

Possible providers include:

- deterministic rules
- interactive human choice
- Jev
- future model/provider integrations

Providers select among supplied candidates. They must not manufacture unsupported execution facts.

## Execution planning

The planner converts repository understanding and decisions into an `ExecutionPlan`.

The plan should make execution explicit:

- steps
- ordering
- working directories
- commands
- dependencies
- environment bindings
- ports
- provisioning requirements
- verification requirements
- explanations

Planning and execution remain separate.

## Runtime adapters

Runtime adapters execute plans through concrete mechanisms such as shell processes or Compose/container execution.

Adapters should execute plans produced by the intelligence layer rather than becoming a second repository-detection system.

## Verification

Verification determines whether execution achieved the state the plan expected.

Possible evidence includes:

- TCP reachability
- explicit health checks
- process state
- runtime-specific readiness
- application-level checks where safely derivable

Verification should be deterministic wherever practical.

## Verified strategies

`.octo.lock` stores strategies that have actually worked.

A stored strategy is reusable only while its relevant repository execution fingerprint remains compatible and the candidate still exists.

```text
Discover
   ↓
Candidate
   ↓
Execute
   ↓
Verify
   ↓
Remember
   ↓
Reuse
   ↓
Invalidate when evidence changes
```

The lock is verified knowledge, not permanent truth.

## Environment safety

Environment requirements and environment values are different concepts.

The intelligence layer may describe:

```text
DATABASE_URL
required: true
source: apps/api
```

It must not persist the resolved secret value.

Runtime-only values must stay out of:

- ProjectModel
- TopologyGraph
- ExecutionPlan
- decision traces
- execution reports
- .octo.lock

## Legacy compatibility

The legacy engine exists for compatibility while intelligence coverage grows.

It should not become the destination for new execution capabilities.

When adding a feature, ask:

> Does this belong in repository intelligence, planning, runtime, verification, or compatibility?

New execution capabilities should normally land in the intelligence pipeline.

## Machine-Readable Core & MCP Server

Octo acts as the deterministic execution substrate for AI coding agents (Claude Code, Cursor, Windsurf, Cline) via two primary machine interfaces:

1. **CLI JSON Stream**: Every core command provides `--json` output formatted against strongly-typed schemas (`octo inspect --json`, `octo explain --json`, `octo graph --json`, `octo plan --json`, `octo run --json`, `octo verify --json`).
2. **Model Context Protocol (MCP)**: Native stdio server launched via `octo mcp`. Built in pure Go with zero external dependencies, providing JSON-RPC 2.0 (version `2024-11-05`).

AI agents consume bounded, deterministic tools and resources:
- Tools: `octo_inspect`, `octo_topology`, `octo_plan`, `octo_run_and_verify`, `octo_verify`, `octo_diagnose`, `octo_env`, `octo_preview`.
- Resources: `octo://topology`, `octo://execution-plan`, `octo://lock`.

## Dependency & Environment Reproduction

Octo separates configuration requirements from secrets and mutations:
- **Runtime Compatibility**: Detects version constraints (`.nvmrc`, `package.json`, `go.mod`, `pyproject.toml`, `Cargo.toml`) and evaluates compatibility against host tools without altering global environments.
- **Virtual Environment Isolation**: Detects workspace Python virtualenvs (`.venv`, `venv`) and prioritizes them in step execution paths without polluting system site-packages.
- **Safe Environment Templates**: Automatically discovers missing required vs optional environment variables from repository evidence and provides zero-secret `.env` templates (`octo env template`, `octo_env`).
- **Machine Mutation Previews**: Before any process or filesystem mutation occurs, agents and developers can inspect projected impact (`octo preview`, `octo_preview`) covering filesystem changes, network port listeners, and spawned processes.

## Multi-Component & Service Topology

Octo models multi-service architectures and monorepos as unified dependency graphs:

- **Workspace Manifest Parsers**: Discovers component boundaries, inter-package dependencies, and package-scoped run targets across Node (npm, yarn, pnpm workspaces), Go workspaces (`go.work`), and Rust Cargo workspaces (`[workspace]` in `Cargo.toml`).
- **Compose Service Discovery**: Scans and parses Docker Compose manifests (`compose.yml`, `docker-compose.yml`), mapping healthchecks, declared exposed ports, and dependency chains.
- **Port Conflict Pre-Reservation & Shifting**: Infrastructure services often declare fixed host port bindings (e.g., PostgreSQL `5432:5432`). Octo pre-reserves service host ports before allocating application component ports. Conflicting non-strict component ports dynamically shift to free host ports with updated `PORT` environment bindings.
- **Topological Startup & Teardown Lifecycle**:
  - The deterministic planner constructs a strict directed acyclic graph based on `depends_on` relationships with cycle detection.
  - In partial failure scenarios (a component crashes or fails health verification), the teardown loop automatically halts all previously launched background processes and Compose services, preventing orphaned processes and lingering containers.

## Adaptive Execution & Closed-Loop Healing

Octo introduces controlled adaptability without sacrificing determinism or introducing speculative guessing:

- **Bounded Fallback Execution**:
  - Each step carries bounded candidates ordered by confidence and decision provider choices.
  - If a primary candidate fails startup or runtime health checks, Octo halts it safely and evaluates the next evidence-backed fallback candidate.
- **Failure Classification as Evidence**:
  - Runtime errors are categorized into structured classes: `port_conflict`, `missing_dependency`, `compilation_syntax`, `process_crash`, and `timeout`.
  - Failures are recorded directly as evidence (`EvidenceExecutionFailure`), enabling upstream planning and AI agents to understand the exact nature of failures.
- **Verified Strategy Caching (`.octo.lock`)**:
  - After end-to-end plan execution and verification pass, the winning candidate is recorded into `.octo.lock`.
  - Reused on subsequent runs when repository fingerprints match.
  - Invalidation is strictly hermetic: modifying source files, workspace manifests (`go.work`), lockfiles, or Compose configurations invalidates stale entries and forces recalculation.
- **External & Interactive Decision Providers**:
  - Pluggable decision providers resolve ties among bounded candidates (Jev, external typed LLM endpoints, or interactive terminal prompts).
  - External endpoints are strictly restricted: they cannot invent new candidates or mutate commands.

## Contribution-friendly architecture

To enable developers worldwide to contribute easily without breaking core contracts, Octo enforces strict modular boundaries:

### 1. Clear, isolated extension points
Adding support for a new language, framework, or runtime should **never** require modifying core graph traversal, planning algorithms, or the decision engine. Contributors work against small, isolated interfaces:

- **Detectors**: Discover signals, manifests, lockfiles, and workspace layouts.
- **Candidate Providers**: Turn discovered evidence into bounded execution candidates (`ExecutionCandidate`).
- **Runtime Adapters**: Map plan steps to concrete execution mechanisms (`RuntimeAdapter`).
- **Verification Providers**: Implement deterministic health checks (ports, HTTP endpoints, process probes).

### 2. Avoid monolithic switch statements
Contributors should not add `if language == "XYZ"` branches across the core codebase. Instead:
- Register providers through typed registries (`CandidateProviderRegistry`).
- Keep ecosystem knowledge isolated in ecosystem-specific files or subpackages.
- The core planner operates strictly on generic primitives (`ProjectModel`, `ExecutionCandidate`, `TopologyGraph`).

### 3. Fast, hermetic testing (no external dependencies)
Contributions must be easy to test locally in milliseconds:
- Unit tests must not require internet access, live databases, or third-party daemon processes.
- Use `t.TempDir()` or lightweight mock directory structures to simulate repository manifests.
- Every new detector or candidate provider must include unit tests covering:
  - Clear positive match (canonical project layout).
  - Multi-entry point or ambiguous layout (bounded candidates).
  - Negative match (should not falsely claim unrelated repos).

### 4. The contributor recipe: "Adding a new ecosystem"
1. Define evidence kinds and signal files for the ecosystem.
2. Implement an `ExecutionCandidateProvider` returning bounded candidates with confidence scores.
3. Add table-driven tests verifying candidate generation and edge cases.
4. Register the provider in `NewExecutionCandidateProviders()`.
5. Verify end-to-end execution with `octo plan` and `octo run`.

## Architectural north star

A new contributor should be able to trace:

```text
repository evidence
      ↓
why Octo believes something
      ↓
which candidates exist
      ↓
why one was selected
      ↓
what was executed
      ↓
how success was verified
      ↓
why the result can be trusted later
```

If a feature makes that chain less explainable, it deserves scrutiny before it is merged.

