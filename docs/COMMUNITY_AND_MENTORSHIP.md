# Octo Community, Education & Mentorship Guide

Welcome to the **Octo Community & Mentorship Program**.

This guide is designed for student contributors, mentees (such as participants in **Google Summer of Code (GSoC)**, **LFX Mentorship**, and university capstone programs), and developers looking to deepen their understanding of developer tooling, compiler-like analysis pipelines, and autonomous AI infrastructure.

---

## 1. Educational Onboarding: The Octo Pipeline

Octo is structured like a multi-stage compiler pipeline, transforming raw, untrusted source code into safe, verified execution states:

```text
Repository Artifacts
        ↓
[1. Evidence Discovery]     Manifests, lockfiles, scripts, explicit configs
        ↓
[2. ProjectModel]           Canonical descriptive representation
        ↓
[3. Topology DAG]           Directed dependency graph (depends_on, network_reference)
        ↓
[4. Candidate Generation]   Bounded, confidence-scored execution strategies
        ↓
[5. Decision Resolution]    Deterministic rules, external endpoints, or .octo.yaml overrides
        ↓
[6. ExecutionPlan]          Topological step-by-step recipe
        ↓
[7. Runtime Execution]      Shell, Docker/Podman container sandbox, or Compose
        ↓
[8. Verification]           TCP connect, HTTP probes, process monitoring
        ↓
[9. Verified Cache]         .octo.lock state preservation
```

### Core Study Exercises for Mentees
1. **Trace a Project**: Follow how `Analyze(path)` in `internal/intelligence/analyze.go` processes a repository from file discovery to candidate selection.
2. **Examine Candidates**: Look at `internal/intelligence/candidates.go` to see how multiple plausible entry points are evaluated with confidence scores and evidence rather than guesswork.
3. **Inspect the MCP Server**: Look at `internal/mcp/server.go` and `internal/mcp/tools.go` to understand how AI agents (such as Claude Code or Cursor) invoke Octo to understand codebases without hallucinating commands.

---

## 2. Mentorship Project Tracks

For structured mentorship programs (GSoC, LFX, etc.), candidates can propose or choose from these high-impact project tracks:

### Track A: Polyglot Ecosystem Expansion
- **Objective**: Implement comprehensive detection, candidate generation, and version detection for emerging ecosystems (e.g. Kotlin/Gradle Kotlin DSL, Swift/SPM, Zig, Elixir/LiveView).
- **Deliverables**:
  - Modular `SignalDefinition` and `ProjectDetector`.
  - Typed `CandidateProvider` generating bounded execution options.
  - Hermetic regression tests simulating real-world open-source repositories.

### Track B: Deep AI Agent Observability & Diagnosis
- **Objective**: Expand diagnostic feedback loops (`octo_diagnose`) for AI coding agents.
- **Deliverables**:
  - Structured error classification from compilation and runtime outputs.
  - Automated remediation suggestions formatted for Model Context Protocol consumers.
  - Zero-hallucination benchmark test suite.

### Track C: Remote & Sandboxed Execution Backends
- **Objective**: Expand sandboxed runtime capabilities (`ContainerAdapter`) to support remote execution backends (e.g. Dev Containers, Kubernetes ephemeral pods, SSH hosts).
- **Deliverables**:
  - Implement a pluggable `RuntimeAdapter` for remote/cloud execution.
  - Port forwarding and volume synchronization abstractions.

---

## 3. How to Propose a Mentorship Project

1. **Engage with the Community**: Review existing issues, pull requests, and roadmap milestones.
2. **Select or Draft a Proposal**:
   - Problem statement and user impact.
   - Proposed architecture aligning with Octo's core philosophy (*Evidence before inference*).
   - Milestone breakdown (Week 1–4: Foundation; Week 5–8: Implementation; Week 9–12: Regression tests & documentation).
3. **Submit Early Draft**: Share draft proposals on GitHub Discussions or issues for maintainer feedback.

---

## 4. Evaluation Criteria for Contributions

Mentors and maintainers review contributions against four strict standards:
1. **Explainability**: Can every decision explain *why* it was made and point to concrete repository evidence?
2. **Determinism**: Does the same repository produce identical plans regardless of host environment?
3. **Hermetic Testing**: Do unit tests execute in milliseconds without external network calls or daemons?
4. **Code Quality**: Follow standard Go idioms, clear error wrapping, and clean separation of concerns.
