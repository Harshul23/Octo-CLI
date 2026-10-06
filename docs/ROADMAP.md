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

**Immediate priority**

Dogfood Octo against unfamiliar open-source repositories and eliminate architectural fragmentation.

### 1. Ecosystem Reliability
- Node, Go, Python, Rust, Java, and Ruby applications.
- Common monorepo layouts and workspace boundaries.
- Repositories mixed with examples, tools, scripts, and documentation.
- Resolving multiple plausible entry points without speculative guessing.
- Infrastructure dependencies (Docker Compose services).

### 2. Legacy Convergence & Architecture Decoupling
- Formally retire the legacy `.octo.yaml`-based blueprint orchestrator, web server (`cmd/serve.go`, `internal/server`), and external database dependencies.
- Consolidate 100% of execution into the native headless intelligence engine.
- Eliminate configuration friction: Octo must work zero-config out of the box.

### 3. Execution Completeness
- Bring log multiplexing, process lifecycle management, signal handling, `--watch` (file change reload), and `--detach` (background runs) natively into the intelligence runtime adapter.
- Turn every real-world failure into a deterministic regression test.

---

## Phase 2 — Machine-Readable Core & MCP (Model Context Protocol) Server

**The Execution Substrate for AI Coding Agents**

Modern software development is increasingly executed by autonomous AI coding agents (Claude Code, Cursor, Devin, GitHub Copilot, SWE-bench runners). When an AI agent enters a repository, it frequently fails at execution: guessing flags, hallucinating startup commands, missing database dependencies, and lacking verification.

Octo's core philosophy—**Evidence before inference**, **Bounded candidates**, **Deterministic planning**, and **Verified state**—makes it the ideal execution and verification engine for AI agents.

### 1. Machine-Readable Interfaces
- First-class `--json` flag across all core commands (`octo inspect --json`, `octo graph --json`, `octo plan --json`, `octo run --json`, `octo verify --json`).
- Strongly typed, versioned JSON schemas for `ProjectModel`, `TopologyGraph`, `ExecutionPlan`, and `ExecutionReport`.

### 2. Native Model Context Protocol (MCP) Server
- Provide a native MCP server (`octo mcp`) supporting standard stdio and SSE transports.
- Expose bounded, deterministic execution tools to AI assistants:
  - `octo_inspect`: Discovers repository evidence and returns the canonical `ProjectModel`.
  - `octo_topology`: Returns the validated dependency DAG for components and infrastructure services.
  - `octo_plan`: Generates the execution plan with bounded candidate options.
  - `octo_run_and_verify`: Executes the plan via runtime adapters and verifies runtime health (ports, TCP, healthchecks).
  - `octo_verify`: Verifies expected runtime state and manages `.octo.lock`.
  - `octo_diagnose`: Explains execution failures without inventing commands.

### 3. Agent Closed-Loop Verification
- Enable AI agents to test, run, and verify repositories deterministically without trial-and-error shell hallucination.

---

## Phase 3 — Dependency and Environment Reproduction

Move from command execution toward reproducible local environments.

- Runtime version detection and compatibility checks (e.g., Node 18 vs 22, Go 1.22+, Python venvs).
- Package-manager detection and installation boundaries (pnpm, yarn, npm, uv, poetry, cargo).
- Missing environment variable discovery and safe `.env` template generation (without exposing or storing secrets).
- Machine-change previews: planning is inspectable and explainable before any machine mutation occurs.
- Service provisioning checks before dependency installation.

---

## Phase 4 — Multi-Component and Service Topology

Treat repositories as systems rather than single processes.

- Monorepo component discovery (npm/yarn/pnpm workspaces, Go workspaces, Cargo workspaces).
- Docker Compose service discovery and lifecycle management.
- Coordinated dynamic port shifting (avoiding collisions and dynamically rebinding `PORT` environment variables).
- Topological startup ordering based strictly on explicit `depends_on` relationships.
- Network-reference awareness (`network_reference` edges for cross-service URLs and connections).
- Partial failure handling and graceful group teardown.

---

## Phase 5 — Adaptive Execution & Closed-Loop Healing

Introduce controlled adaptation without sacrificing determinism.

- Bounded fallback strategies: if Candidate A fails during verification, Candidate B from the bounded candidate set is evaluated.
- Failure classification as evidence (distinguishing syntax/compilation errors, missing dependencies, port conflicts, and process crashes).
- Verified strategy caching: reuse `.octo.lock` entries when repository fingerprints match; invalidate cleanly when source or manifests change.
- External decision providers (Jev, typed LLM, or interactive human prompts) to resolve bounded ties when multiple valid candidates exist.

---

## Phase 6 — Sandboxed & Containerized Runtimes

Safe execution boundaries for developers and automated agents.

- Running arbitrary repository scripts on host bare metal poses security and machine pollution risks.
- Containerized runtime adapters: Docker/Podman container adapter to run plan steps inside ephemeral, reproducible containers.
- Volume mounts, port mappings, and container networks derived directly from the canonical `TopologyGraph`.
- Safe execution sandboxes for AI agents testing unverified third-party repositories.

---

## Phase 7 — Ecosystem Scale & Extension Points

Expand Octo through modular contracts rather than language-specific conditional blocks.

- Extensible detector interface for new languages, frameworks, and tools.
- Pluggable candidate providers and runtime adapters.
- Custom healthcheck and verification providers.
- Repository-specific strategy overrides and community-maintained ecosystem packages.

---

## Phase 8 — Developer Trust & Open-Source Community

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

