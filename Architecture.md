# Octo Architecture

## 1. Purpose & Core Philosophy

Octo is an **execution-intelligence engine for software repositories**, designed for both human developers and autonomous AI coding agents.

The central problem Octo solves:

> **A repository has been cloned. How do we determine a safe, evidence-backed, reproducible way to run it?**

Octo's architectural goal is to **make "how do I run this?" disappear** through four foundational principles:

1. **Evidence before inference**: Ground every execution step in observable repository artifacts (manifests, lockfiles, scripts, source code, Compose definitions) rather than speculative heuristics or language name guessing.
2. **Candidates before decisions**: Generate bounded, confidence-scored execution candidates from repository evidence. External agents or decision providers select only from valid candidates and cannot invent arbitrary commands.
3. **Verification before trust**: A command starting or exiting 0 is not proof of health. Octo deterministically verifies TCP socket listeners, HTTP readiness probes, and process states.
4. **Verified state is valuable**: Strategies that pass execution and verification are fingerprinted and cached in `.octo.lock`, enabling instant reuse until repository state changes.

---

## 2. Canonical Pipeline

The Octo execution lifecycle follows a single, deterministic pipeline:

```text
Repository Artifacts
        ↓
Evidence Discovery (manifests, lockfiles, source files, compose.yaml)
        ↓
ProjectModel (canonical descriptive representation)
        ↓
Topology DAG (components, services, depends_on, network_reference)
        ↓
Execution Candidates (bounded, confidence-scored options)
        ↓
Bounded Decision Layer (deterministic rules, human interactive choice, or external provider)
        ↓
ExecutionPlan (topological step-by-step recipe: Setup → Services → Applications)
        ↓
Runtime Adapters (native host process or ephemeral container sandbox)
        ↓
Readiness Verification (TCP sockets, HTTP probes, process liveness)
        ↓
Verified Strategy (.octo.lock state persistence)
```

100% of execution runs through this native intelligence pipeline. Legacy orchestrators, blueprints, and transitional migration flags have been retired.

---

## 3. Core Subsystems

### 3.1 Evidence Discovery & ProjectModel

`ProjectModel` is the canonical internal representation of repository facts. Evidence is discovered deterministically:

- **Manifests & Lockfiles**: `package.json`, `pnpm-lock.yaml`, `go.mod`, `go.work`, `Cargo.toml`, `pyproject.toml`, `poetry.lock`, `Gemfile`, `pom.xml`, `build.gradle`, `mix.exs`, `composer.json`.
- **Runtimes & Toolchains**: Version constraints evaluated against host tools (Node, Go, Python, Rust, Java, Ruby, PHP, Elixir) and virtual environments (`.venv`, `venv`).
- **Configuration & Services**: Root Docker Compose definitions (`compose.yaml`, `docker-compose.yml`), custom maintainer overrides (`.octo.yaml`).

The model is descriptive: it records what exists and why, without embedding speculative execution policies.

### 3.2 Topology DAG & Target Selection

Octo models multi-target repositories (monorepos, microservices) as a directed graph of `Component` and `Service` nodes:

- **Relationship Semantics**:
  - `depends_on`: Strict startup ordering edge. A dependent component will not be launched until all dependencies pass their readiness gates.
  - `network_reference`: Informational communication edge (e.g. `DATABASE_URL` pointing to a Compose service). Does not introduce artificial startup delays or participate in cycle checks.
- **Transitive Target Selection**:
  - `--only <target>`: Focuses on specific components or services (e.g. `--only component:web`).
  - Automatically resolves transitive dependencies (selecting `component:api` auto-includes its required `service:postgres`).
  - `--all`: Launches all detected runnable applications and backing services.
  - Non-runnable workspace libraries (packages without dev/start commands) are scheduled for building in `PhaseSetup`, never launched as daemons.
- **Interactive Component Picker**: When multiple targets exist in an interactive terminal, Octo presents a Bubble Tea selector with target navigation, dependency preview, and confirmation. In non-interactive contexts (CI, `--json`, MCP), Octo aborts immediately with an actionable target list rather than hanging.

### 3.3 Bounded Decision Layer

When repository evidence supports multiple plausible ways to run an application, Octo generates a **bounded candidate set**:

```go
type DecisionProvider interface {
    Decide(ctx context.Context, request DecisionRequest) (DecisionResult, error)
}
```

- **Deterministic Provider**: Offline baseline selecting highest-confidence evidence-backed candidates.
- **Interactive Decision Provider**: Prompts the developer in interactive terminal sessions.
- **External Providers**: Jev or typed external endpoints resolve ties.
- **Hard Safety Boundary**: Decision providers MUST choose from supplied candidates. They are strictly prohibited from manufacturing unsupported commands or mutating paths.

### 3.4 Deterministic Execution Planner

The planner converts the decided `ProjectModel` into a concrete `ExecutionPlan`:

1. **PhaseSetup**: Builds shared libraries and workspace prerequisites.
2. **PhaseProvision**: Verifies toolchain binaries and Docker requirements.
3. **PhaseInfrastructure**: Launches backing Compose services and awaits health readiness.
4. **PhaseApplication**: Launches application components in topological dependency order.

- **Dynamic Port Shifting**: Pre-reserves host ports declared by Compose services (e.g., `5432:5432`). Conflicting non-strict application ports dynamically shift to free host ports with updated `PORT` environment variables.

### 3.5 Runtime Adapters

Runtime adapters execute plan steps without performing independent repository analysis:

- **NativeRuntimeAdapter**:
  - Spawns host processes within dedicated process groups.
  - Multiplexes stdout/stderr into cleanly prefixed terminal streams and `.octo/logs/`.
  - Signal Management: Traps `SIGINT`/`SIGTERM` to gracefully drain child process trees.
  - Support for background runs (`--detach`) and file change reloading (`--watch`).
- **ContainerAdapter (`--sandbox` / `-s`)**:
  - Executes steps in ephemeral Docker or Podman containers (`--rm`).
  - Derives official minimal base images (Alpine/Slim) matched to detected language versions.
  - Mounts repository root into `/app` with relative working directories and forwards negotiated ports.
  - Clean container lifecycle teardown on cancellation or failure (`stop -t 2` + `rm -f`).

### 3.6 Verification & Strategy Caching (`.octo.lock`)

- **Deterministic Readiness Probes**:
  - TCP socket connectivity tests on assigned component ports.
  - HTTP status probes (e.g. `GET /` or `/healthz`) for web services.
  - Process liveness monitoring for non-server background workers.
- **Immutable Checkpoint**: Once all components verify successfully, the winning strategy is recorded to `.octo.lock`.
- **Hermetic Invalidation**: `.octo.lock` stores SHA-256 fingerprints of source files, package manifests, and workspace configs. Modifying code or dependencies invalidates the cached strategy, guaranteeing safe re-evaluation.

### 3.7 Environment Safety & Missing Variable Prompting

- **Zero-Secret Scoping**: Environment values, tokens, and credentials are NEVER serialized to `ProjectModel`, `ExecutionPlan`, decision traces, execution reports, or `.octo.lock`.
- **Missing Variable Prompt**: When required variables lack definitions, `octo run` launches an interactive Bubble Tea prompt focused directly on the text input, allowing developers to type values, skip individual items, or skip all, with an optional prompt to persist entered values to `.env.local`.

---

## 4. Machine-Readable Core & AI Agent MCP Server

Octo serves as the execution substrate for autonomous AI coding agents (Claude Code, Cursor, Windsurf, Devin):

### 4.1 CLI JSON Mode (`--json` / `-j`)

Every core command supports structured, versioned JSON output:
- `octo inspect --json`: Complete `ProjectModel` with detected signals and evidence.
- `octo plan --json`: Deterministic step-by-step `ExecutionPlan`.
- `octo run --json`: Non-interactive execution with structured `ExecutionReport`.
- `octo verify --json`: Runtime health check status and lockfile integrity.
- `octo benchmark --json`: Empirical agent execution benchmarks.

### 4.2 Native Model Context Protocol (MCP) Server (`octo mcp`)

Octo embeds a native JSON-RPC 2.0 MCP server over stdio in pure Go with zero third-party runtime dependencies:

- **Tools**:
  - `octo_inspect`: Analyzes repository evidence and returns `ProjectModel`.
  - `octo_topology`: Returns the component and service dependency DAG.
  - `octo_plan`: Generates the execution plan with bounded candidates.
  - `octo_run_and_verify`: Executes the plan with optional `"sandbox": true` isolation and verifies runtime health.
  - `octo_verify`: Verifies live services and lockfile integrity.
  - `octo_diagnose`: Classifies execution failures without inventing commands.
  - `octo_env`: Inspects missing environment variables and template requirements.
  - `octo_preview`: Previews projected machine changes and port allocations.
  - `octo_benchmark`: Measures execution pass@1, token savings, and hallucination metrics.
- **Resources**:
  - `octo://topology`: Live repository topology graph.
  - `octo://execution-plan`: Deterministic execution plan.
  - `octo://lock`: Cached verified strategies.

---

## 5. Extension Registries

Octo separates core analysis and planning algorithms from ecosystem-specific logic via typed registries:

1. **ProjectDetectorRegistry**: Register language and framework detectors (`RegisterProjectDetector`) and signal definitions (`RegisterProjectSignal`).
2. **CandidateProviderRegistry**: Register candidate generators (`RegisterCandidateProvider`) turning manifests into bounded `ExecutionCandidate` sets.
3. **VerificationRegistry**: Register custom health probes (`RegisterVerificationProvider`) for protocol-specific verification.
4. **RuntimeAdapterRegistry**: Register custom execution backends (`RegisterRuntimeAdapter`).

New languages and ecosystems are added through these isolated interfaces without modifying core graph traversal or planner logic.

---

## 6. Architectural North Star

Every Octo execution should be traceable end-to-end:

```text
Repository Evidence
        ↓
Why Octo believes something exists
        ↓
Which candidate strategies were derived
        ↓
Why a specific candidate was selected
        ↓
What commands were executed
        ↓
How runtime readiness was verified
        ↓
Why the resulting strategy can be trusted and reused
```
