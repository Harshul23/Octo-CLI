# Octo Roadmap

This roadmap describes the direction of Octo. It is capability-oriented rather than tied to arbitrary release dates.

## Vision Summary

Octo's mission is simple: **Make “how do I run this?” disappear.**

A repository should contain enough evidence to derive a safe, deterministic, explainable path toward execution—for both human developers and autonomous AI coding agents.

---

## Phase 0 — Foundation

**Status: substantially complete**

- native repository analysis
- canonical `ProjectModel`
- topology graph
- evidence-backed execution candidates
- provider-neutral decision layer
- deterministic planning
- runtime adapters
- secret-safe environment handling
- application port discovery and verification
- long-running application execution
- structured execution failures
- verified strategy persistence through `.octo.lock`
- compatibility path for the legacy engine

The purpose of this phase was to prove that Octo can reason about execution without becoming a collection of unrelated heuristics.

---

## Phase 1 — Reliability across Real Repositories & Legacy Convergence

**Status: completed**

Dogfood Octo against unfamiliar open-source repositories and eliminate architectural fragmentation.

### 1. Ecosystem Reliability
- Node, Go, Python, Rust, Java, and Ruby applications.
- Common monorepo layouts and workspace boundaries.
- Repositories mixed with examples, tools, scripts, and documentation.
- Resolving multiple plausible entry points without speculative guessing.
- Infrastructure dependencies (Docker Compose services).

### 2. Legacy Convergence & Architecture Decoupling
- Formally retired legacy dependencies and made the headless intelligence engine the default execution path.
- Consolidated 100% of execution into the native intelligence engine.
- Eliminated configuration friction: Octo works zero-config out of the box.

### 3. Execution Completeness
- Brought log multiplexing, process lifecycle management, signal handling, `--watch` (file change reload), and `--detach` (background runs) natively into the intelligence runtime adapter.
- Turned real-world test failures into deterministic regression tests.

---

## Phase 2 — Machine-Readable Core & MCP (Model Context Protocol) Server

**Status: completed**

**The Execution Substrate for AI Coding Agents**

Modern software development is increasingly executed by autonomous AI coding agents (Claude Code, Cursor, Devin, GitHub Copilot, SWE-bench runners). When an AI agent enters a repository, it frequently fails at execution: guessing flags, hallucinating startup commands, missing database dependencies, and lacking verification.

Octo's core philosophy—**Evidence before inference**, **Bounded candidates**, **Deterministic planning**, and **Verified state**—makes it the ideal execution and verification engine for AI agents.

### 1. Machine-Readable Interfaces
- First-class `--json` flag across all core commands (`octo inspect --json`, `octo explain --json`, `octo graph --json`, `octo plan --json`, `octo run --json`, `octo verify --json`).
- Strongly typed, versioned JSON schemas for `ProjectModel`, `TopologyGraph`, `ExecutionPlan`, and `ExecutionReport`.

### 2. Native Model Context Protocol (MCP) Server
- Provided native MCP server (`octo mcp`) over standard stdio using pure Go (zero external dependencies).
- Full JSON-RPC 2.0 (protocol version `2024-11-05`) implementation with concurrent request safety.
- Bounded, deterministic execution tools for AI assistants:
  - `octo_inspect`: Discovers repository evidence and returns the canonical `ProjectModel`.
  - `octo_topology`: Returns the validated dependency DAG for components and infrastructure services.
  - `octo_plan`: Generates the execution plan with bounded candidate options.
  - `octo_run_and_verify`: Executes the plan via runtime adapters and verifies runtime health (ports, TCP, healthchecks).
  - `octo_verify`: Verifies expected runtime state and manages `.octo.lock`.
  - `octo_diagnose`: Explains execution failures without inventing commands.
- Core resources via URI scheme:
  - `octo://topology`: Repository component and service topology DAG.
  - `octo://execution-plan`: Deterministic step-by-step execution plan.
  - `octo://lock`: Verified execution strategy cache from `.octo.lock`.

### 3. Agent Closed-Loop Verification
- AI agents can test, run, and verify repositories deterministically without trial-and-error shell hallucination.

---

## Phase 3 — Dependency and Environment Reproduction

**Status: completed**

Moved from command execution toward reproducible local environments.

- **Runtime version detection & compatibility**:
  - Semantic version constraint matching (semver ranges like `>=18`, `^20`, `20.x`, Go minor compatibility).
  - Pre-flight runtime toolchain verification against the host (`node`, `go`, `python3`, `rustc`).
  - Python virtual environment (`.venv`, `venv`) automatic discovery, wiring into `PATH` and setting `VIRTUAL_ENV`.
- **Package-manager boundaries**:
  - Explicit detection and isolation across `pnpm`, `yarn`, `npm`, `uv`, `poetry`, `pip`, `cargo`, and `go`.
  - Installation prerequisites with actionable remediation guidance.
- **Environment variables & safe templates**:
  - Missing required vs optional variable discovery without secret exposure (`octo env`, `octo_env`).
  - Safe placeholder `.env.template` generation (`octo env template`).
- **Machine-change previews**:
  - Complete pre-execution inspection of projected filesystem mutations, network port listeners, and processes before any command runs (`octo preview`, `octo_preview`).
- **Service provisioning checks**:
  - Pre-flight infrastructure checks (`service.docker`) verified before execution.

---

## Phase 4 — Multi-Component and Service Topology

**Status: completed**

Treat repositories as systems rather than single processes.

- **Monorepo component discovery**: Full workspace discovery across npm/yarn/pnpm workspaces, Go workspaces (`go.work`), and Cargo workspaces (`[workspace]` in `Cargo.toml`).
- **Docker Compose service discovery & lifecycle**: Compose services identified with dependencies, ports, and lifecycle commands.
- **Coordinated dynamic port shifting**: Host ports declared in Compose services are pre-reserved; component port conflicts dynamically shift and rebind `PORT` environment variables smoothly.
- **Strict topological startup ordering**: Directed acyclic graph startup ordering strictly adhering to explicit `depends_on` relationships with cycle detection.
- **Network-reference awareness**: `network_reference` edges for cross-service URLs and connections without introducing artificial startup delays.
- **Partial failure handling & graceful group teardown**: If any step in a multi-component startup sequence fails or times out, previously launched background processes and Compose services are automatically stopped with `TeardownPerformed` recorded in the `ExecutionReport`.

---

## Phase 5 — Adaptive Execution & Closed-Loop Healing

**Status: completed**

Introduced controlled adaptation without sacrificing determinism.

- **Bounded fallback strategies**: If Candidate A fails during startup or verification, Candidate B from the bounded candidate set is evaluated automatically, recording decision traces and failure history.
- **Failure classification as evidence**: Structured categorization of execution failures into `port_conflict`, `missing_dependency`, `compilation_syntax`, `process_crash`, and `timeout`, converted directly into observable evidence.
- **Verified strategy caching (`.octo.lock`)**:
  - Reuses verified execution strategies across runs when repository fingerprints match.
  - Automatically invalidates strategies on changes to source code, manifests, lockfiles (`poetry.lock`, `uv.lock`, `go.work`, `Cargo.lock`), or Docker Compose manifests.
  - On fallback recovery, `.octo.lock` persists the successful fallback candidate.
  - `octo verify` reports cached strategy status (active vs invalidated).
- **External decision providers**:
  - Bounded tie-breaking via Jev (`OCTO_JEV_URL`), typed LLMs / external endpoints (`OCTO_DECISION_URL`), and interactive terminal prompts (`ui.InteractiveDecisionProvider`).
  - Strict bounded validation: external endpoints can only select from provided candidates and are prevented from hallucinating new commands.

---

## Phase 6 — Sandboxed & Containerized Runtimes

**Status: completed**

Safe execution boundaries for developers and automated AI agents.

- **Containerized runtime adapter (`ContainerAdapter`)**:
  - Ephemeral, reproducible execution inside Docker or Podman containers (`--rm`).
  - Safe runtime isolation protecting host bare metal from machine pollution or unsafe repository scripts.
- **Dynamic base image derivation (`ResolveContainerImage`)**:
  - Language and runtime version-aware base image derivation (Node Alpine, Go Alpine, Python Slim, Rust Alpine, OpenJDK Temurin, Ruby Alpine).
  - Automatically maps versions detected from `.nvmrc`, `package.json`, `go.mod`, `pyproject.toml`, etc.
- **Topology-derived volume mounts and port forwarding**:
  - Host root volume mounting (`-v <host_root>:/app -w /app/<workdir>`).
  - Container port forwarding (`-p <port>:<port>`) and scoped environment variable pass-through (`-e KEY=VAL`).
- **CLI & Environment integration**:
  - First-class `--sandbox` / `-s` flag on `octo run`.
  - Global `OCTO_SANDBOX=1` environment variable support.
  - Full compatibility across foreground, `--detach`, and `--watch` modes.
- **AI Agent Safe Execution via MCP**:
  - Added `"sandbox": boolean` option to the `octo_run_and_verify` MCP tool.
  - Allows autonomous AI coding agents (Claude Code, Cursor, Devin, SWE-bench) to safely run and verify unverified code in sandboxed containers.
- **Container process lifecycle & teardown**:
  - Detached container process tracking (`containerProcess`) with log multiplexing (`.octo/logs/<component>.log`).
  - Coordinated teardown on failure or cancellation via `docker/podman stop -t 2` and `rm -f`.

---

## Phase 7 — Ecosystem Scale & Extension Points

**Status: completed**

Expand Octo through modular contracts rather than language-specific conditional blocks.

- **Extensible Detector Registry (`ProjectDetectorRegistry`)**:
  - Modular project detectors (`ProjectDetector`) and signal definitions (`SignalDefinition`) registered dynamically without modifying core algorithms.
  - Native global registration via `RegisterProjectDetector` and `RegisterProjectSignal`.
  - Added built-in detection for PHP (`composer.json`) and Elixir (`mix.exs`).
- **Pluggable Candidate Provider Registry (`CandidateProviderRegistry`)**:
  - Decoupled candidate providers from hardcoded lists: any ecosystem can register via `RegisterCandidateProvider`.
  - Added built-in candidate providers for PHP (Laravel `artisan serve`, `composer run dev`, PHP CLI server) and Elixir (Phoenix `mix phx.server`, `mix run --no-halt`).
- **Extensible Verification Providers (`VerificationRegistry`)**:
  - Decoupled runtime readiness checks into pluggable `VerificationProvider` implementations.
  - Added `HTTPVerificationProvider` (`VerificationHTTP`) to verify endpoint health, HTTP status codes, and paths (e.g. `/healthz`).
  - Allows custom protocols and probes via `RegisterVerificationProvider`.
- **Repository-Specific Strategy Overrides (`.octo.yaml` / `.octo.yml`)**:
  - Declarative developer configuration overriding auto-detected run commands, strict ports, environments, and custom verification endpoints.
  - Generates explicit `override.*` candidates with 1.0 confidence and clear `EvidenceConfig` tracing.
- **Pluggable Runtime Adapter Registry (`RuntimeAdapterRegistry`)**:
  - Dynamic runtime adapter registration (`RegisterRuntimeAdapter`) for custom execution engines.

---

## Phase 8 — Developer Trust & Open-Source Community

**Immediate priority**

Make reliability and transparency product properties.

- Explainability at every step: every candidate, edge, and step can explain *why* it exists and the evidence supporting it.
- Large regression test suite drawn from real, popular open-source repositories.
- Contributor documentation, good-first-issues, and architecture guides.
- Transparent design discussions and release discipline.
- Educational and mentorship pathways (e.g., GSoC) once the core has proven real-world stability.

---

## Measuring Progress

The important metrics are not lines of code or number of languages.

Track:

- Successful execution rate on unseen repositories without manual intervention
- Zero command hallucinations (100% evidence-backed candidates)
- Actionable failure explanations when execution cannot safely proceed
- Verified-strategy reuse efficiency via `.octo.lock`
- AI agent integration and adoption via the MCP server
- Regression coverage from real repositories
- Time from clone to verified running application
- Independent developer and contributor adoption

The ultimate metric:

> **How often can a developer or AI agent clone an unfamiliar repository and get it running without manually reconstructing its environment?**

