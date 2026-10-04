# Octo CLI

**Make “how do I run this?” disappear.**

Octo is an execution-intelligence CLI for software repositories. It analyzes a repository, discovers evidence about what exists and how it can run, builds a canonical project model and topology, derives evidence-backed execution candidates, creates a deterministic execution plan, executes it through runtime adapters, verifies the result, and can remember verified execution strategies.

The goal is not to build another collection of language-specific startup heuristics. The goal is to make repository execution **discoverable, explainable, deterministic, provider-neutral, and progressively self-correcting**.

## Core vision

The problem Octo is solving is simple:

> **“I cloned this repository. How the hell do I run it?”**

Octo's long-term mission is:

> **Make “how do I run this?” disappear.**

A repository should contain enough evidence for Octo to understand its structure and derive a safe path toward execution.

The core architecture is:

```text
Repository
    ↓
Evidence Discovery
    ↓
ProjectModel
    ↓
Topology + Execution Candidates
    ↓
Bounded Decision
    ↓
ExecutionPlan
    ↓
Runtime Adapters
    ↓
Execute
    ↓
Verify
    ↓
.octo.lock
```

The important distinction is **verified execution**.

Octo should not simply ask an LLM to invent a command. It should discover what the repository actually supports, produce bounded candidates, select among those candidates, execute them, learn from failures without inventing new facts, verify success, and persist only strategies that have actually worked.

## Design principles

### 1. Evidence before inference

Octo's intelligence layer starts from repository evidence:

- manifests
- lockfiles
- scripts
- configuration
- source files
- workspace declarations
- Compose files
- explicit dependencies
- explicit health checks
- execution failures

Detectors produce facts and evidence. They should not silently turn guesses into repository truth.

### 2. Candidates before decisions

When multiple execution strategies are possible, ecosystem-specific providers produce a **bounded candidate set**.

For example, a Go component containing multiple non-test Go files can produce:

```text
go run main.go
go run .
```

The decision layer chooses between those candidates. It does not invent a third command.

### 3. Decisions are provider-neutral

Octo defines a `DecisionProvider` boundary.

Possible providers include:

- deterministic rules
- Jev
- an LLM
- a human

External intelligence is optional. The deterministic provider is the offline baseline, so Octo remains useful without Jev, an LLM, a network connection, or a paid service.

A decision provider may select from supplied candidates, but it must not invent commands, runtimes, paths, dependencies, or other execution facts outside that candidate set.

### 4. Topology is a first-class concept

A repository is not necessarily one runnable program.

Octo models:

- application components
- infrastructure services
- explicit dependencies
- informational network references
- health/readiness relationships

These become a canonical topology graph rather than being reconstructed independently by every subsystem.

### 5. Planning is separate from execution

The `ExecutionPlan` is the deterministic bridge between repository understanding and runtime execution.

The plan describes:

- what steps exist
- their dependencies
- their working directories
- their commands
- candidate information
- provisioning requirements
- environment bindings
- ports
- explanations

Runtime adapters decide **how** a plan step is executed.

### 6. Secrets never become planning data

Environment requirements may be represented as names, ownership, and source locations.

Resolved environment values are runtime-only.

They must not be stored in:

- `ProjectModel`
- `TopologyGraph`
- `ExecutionPlan`
- decision traces
- execution reports
- `.octo.lock`

### 7. Verification is part of execution intelligence

A command finishing is not automatically the same thing as successful application execution.

Octo derives deterministic verification checks from known facts, such as:

- resolved application ports
- explicit service health checks

A strategy becomes a **verified strategy** only after the complete execution plan and deterministic verification succeed.

### 8. Verified state is different from desired state

Octo separates configuration intent from verified execution state.

- `.octo.yaml` represents the older serialized configuration/intent path.
- `.octo.lock` records execution strategies that Octo has actually verified.

The lock is not a replacement for repository understanding. It is a cache of verified knowledge that is invalidated when execution-relevant repository state changes.

## How the intelligence engine works

The canonical execution path is:

```text
1. Analyze repository
2. Build ProjectModel
3. Load and validate verified strategies
4. Build topology
5. Generate execution plan
6. Resolve runtime environment
7. Execute plan in dependency order
8. Verify expected runtime state
9. Persist successful strategies
```

The current CLI uses the intelligence engine by default for `octo run`.

The explicit legacy engine remains available temporarily for compatibility and feature gaps.

```bash
octo run
```

To use the older execution path explicitly:

```bash
octo run --engine legacy
```

The legacy path is transitional. New execution capabilities belong in the intelligence architecture.

## Repository analysis

The native intelligence detector builds a `ProjectModel` from repository evidence.

The model can represent:

- project identity
- root
- language
- runtime version
- framework
- package manager
- setup command
- execution command
- monorepo state
- components
- infrastructure services
- environment requirements
- topology references
- confidence
- supporting evidence

Supported native detection currently includes repository signals for ecosystems such as:

- Node
- Go
- Python
- Rust
- Java
- Ruby

Detection and execution are deliberately separate.

A detector can establish that a repository is a Go project without deciding that `go run main.go` is the correct execution strategy.

## Execution candidates

Execution candidates are generated by `CandidateProvider` implementations.

The current candidate-provider boundary is provider-neutral and extensible.

For example, Go detection can produce a stronger `go run .` candidate when multiple non-test Go files are present. This matters because:

```text
go run main.go
```

may fail when `main.go` depends on symbols defined in sibling files, while:

```text
go run .
```

compiles the complete package.

A failed candidate is evidence. It is not permission to invent a command.

The intended feedback loop is:

```text
Candidate A
    ↓
Execute
    ↓
Failure
    ↓
Failure becomes evidence
    ↓
Candidate B from the existing candidate set
    ↓
Execute
    ↓
Success
    ↓
Verify
    ↓
Persist verified strategy
```

## Topology

Octo represents repository components and infrastructure services in one graph.

Node namespaces are explicit:

```text
component:<name>
service:<name>
```

Relationship kinds currently include:

- `depends_on` — execution dependency
- `network_reference` — informational communication relationship

Only `depends_on` affects execution ordering.

The graph rejects:

- unknown nodes
- duplicate node identities
- invalid references
- unknown relationship kinds
- dependency cycles

This keeps topology facts separate from execution policy.

## Multi-component repositories

Octo does not flatten a monorepo into one project merely because multiple directories exist.

Explicit workspace boundaries can produce separate components, each retaining its own:

- path
- runtime information
- package manager
- execution candidates
- command
- confidence
- evidence

Dependencies are only created when repository evidence supports them.

For example, an explicit Node `workspace:` dependency can become a topology dependency. Merely having two folders with similar names cannot.

## Infrastructure services

Infrastructure is modeled separately from source-code components.

Docker Compose is a supported explicit topology source.

Compose discovery can preserve:

- service names
- images
- build contexts
- published ports
- `depends_on`
- health checks
- supporting evidence

Octo does not invent an application command for an infrastructure service.

Instead, the runtime adapter boundary decides how a known service execution step can be executed.

## Environment handling

The intelligence architecture separates **environment requirements** from **environment values**.

The model may know:

```text
DATABASE_URL
required: true
source: apps/api/...
```

It must not know the value of `DATABASE_URL`.

At execution time, Octo resolves values using scoped precedence:

1. process environment
2. component `.env.local`
3. component `.env`
4. repository-root `.env.local`
5. repository-root `.env`

Component-local values remain scoped to the component's execution steps.

Required-variable validation also respects ownership. A value defined for one component must not accidentally satisfy another component's requirement.

## Network references

Octo can discover informational application-to-application references from explicit static configuration.

For example:

```text
api
 └── DATABASE_URL → service:postgres
```

This produces metadata about the relationship, not a secret value.

Interpolated values are not guessed.

A `network_reference` does not automatically become a startup dependency.

## Provisioning

Provisioning is an explicit planning phase.

The planner can derive machine prerequisites from repository facts, such as required package managers.

Provisioning checks happen before dependency installation.

Planning itself should not mutate the user's machine. Any automatic installation or bootstrap behavior must be explicit and visible.

## Ports

Ports are execution resources.

For application components, Octo can:

1. inspect the requested port
2. detect a conflict
3. select an available TCP port
4. record requested and resolved ports in the execution plan
5. expose the resolved port to the component as `PORT`

Infrastructure service ports declared by Compose are currently treated as strict declarations. Conflicts are reported rather than silently rewriting Compose configuration.

## Runtime adapters

Execution plans are runtime-neutral.

A `RuntimeAdapter` maps an executable plan step to a concrete execution mechanism.

The current built-in boundary includes:

- **shell** — ordinary command execution
- **compose** — Docker Compose execution

Adapters are selected by capability rather than by a global language switch.

This allows Octo to grow runtime support without putting runtime-specific behavior into the repository detector.

## Verification

Verification is a separate post-execution phase.

Current deterministic checks include:

- application TCP-port reachability
- explicit service health/readiness conditions

Verification results are structured and safe to serialize.

Verification does not modify the execution plan.

## Verified strategies and .octo.lock

After successful execution and verification, Octo may persist a verified strategy in `.octo.lock`.

A strategy contains:

- component
- candidate ID
- command
- execution-relevant repository fingerprint
- verification timestamp

Before reuse, Octo checks that:

1. the candidate still exists
2. its command is unchanged
3. its execution-relevant fingerprint still matches

Changes to execution-relevant source, manifests, scripts, lockfiles, or runtime configuration invalidate the strategy.

Unrelated documentation or generated/dependency trees should not invalidate it.

A stale lock entry falls back to normal discovery and candidate selection.

## Explainability

Octo is intended to be explainable.

Evidence, candidates, topology edges, execution steps, verification results, and decision traces are structured rather than hidden inside a large heuristic function.

A decision trace can explain:

- what was selected
- which bounded option was selected
- why it was selected
- confidence
- supporting evidence
- whether a candidate failed or succeeded

The trace must not expose resolved environment values or command output.

## Current architecture boundary

The repository still contains an older execution path based around:

```text
octo init
    ↓
.octo.yaml
    ↓
legacy Blueprint/Orchestrator
```

That path is being retired progressively.

The intelligence path is the architectural destination:

```text
octo run
    ↓
intelligence.Analyze
    ↓
ProjectModel
    ↓
TopologyGraph
    ↓
ExecutionPlan
    ↓
RuntimeAdapters
    ↓
Verification
    ↓
.octo.lock
```

This distinction is important when reading the repository: some legacy files exist for compatibility, but they should not be treated as the model for new features.

## Repository architecture

At a high level:

```text
cmd/
 ├── CLI commands and user-facing entry points
 └── engine selection

internal/intelligence/
 ├── repository detection
 ├── ProjectModel
 ├── evidence
 ├── topology
 ├── execution candidates
 ├── decision providers
 ├── planning
 ├── provisioning
 ├── environment resolution
 ├── runtime adapters
 ├── execution reports
 ├── verification
 ├── decision traces
 └── verified strategy / lock state

internal/analyzer/
 └── legacy analyzer surface being retired

internal/blueprint/
 └── legacy .octo.yaml blueprint representation

internal/orchestrator/
 └── legacy execution path

internal/ui/
 └── terminal/TUI presentation and interactive legacy flows

spec/
 └── versioned Octo architecture and behavioral contract
```

The exact implementation will continue to evolve, but new intelligence capabilities should respect these boundaries.

## Architecture rule of thumb

When adding a new ecosystem or capability, prefer:

```text
Evidence detector
      ↓
Native model facts
      ↓
Provider / candidate adapter
      ↓
Bounded decision
      ↓
Generic planner / runtime
```

Avoid:

```text
if Go ...
if Node ...
if Python ...
if Rust ...
if Java ...
if ...
```

The core should provide generic primitives. Ecosystem-specific knowledge should live behind detectors, candidate providers, and runtime adapters.

## Status

Octo is actively migrating from its original local-deployment CLI architecture toward the execution-intelligence architecture described above.

The intelligence system already includes major foundations for:

- native repository detection
- evidence-backed execution candidates
- provider-neutral decisions
- unified topology
- dependency-aware execution planning
- provisioning checks
- scoped runtime environments
- secret-safe environment bindings
- runtime adapters
- deterministic verification
- execution reports
- decision traces
- verified execution strategies

The remaining work is primarily convergence: retiring compatibility paths, increasing ecosystem coverage, improving discovery and runtime adapters, and making verified execution increasingly robust.

For the full behavioral contract, see [spec/v1/specification.md](spec/v1/specification.md).

## Installation

### From source

```bash
git clone https://github.com/Harshul23/Octo-CLI.git
cd Octo-CLI
./scripts/install.sh
```

### Using Go

```bash
go install github.com/harshul/octo-cli/cmd@latest
```

## Quick start

For the current intelligence path:

```bash
cd your-project
octo run
```

Octo analyzes the repository directly, builds its execution model and plan, executes the plan, verifies the result, and records verified strategies when execution succeeds.

If you specifically need the transitional legacy engine:

```bash
octo run --engine legacy
```

The legacy configuration flow may require:

```bash
octo init
```

That command belongs to the compatibility path and should not be interpreted as the long-term architecture.

## Contributing

Contributions are welcome. Before adding a new detector, provider, planner rule, or runtime adapter, read:

- [Architecture.md](Architecture.md)
- [spec/v1/specification.md](spec/v1/specification.md)
- [CONTRIBUTING.md](CONTRIBUTING.md)

The key question for every architectural change is:

> **Does this add generic intelligence primitives, or does it add another hidden ecosystem-specific heuristic?**

Prefer the former.

## License

MIT License — see [LICENSE.md](LICENSE.md).
