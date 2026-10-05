# Octo Roadmap

This roadmap describes the direction of Octo. It is capability-oriented rather than tied to arbitrary release dates.

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

## Phase 1 — Reliability across real repositories

**Next priority**

Dogfood Octo against unfamiliar open-source repositories.

Focus on:

- Node, Go, Python, Rust, Java, and Ruby applications
- common monorepo layouts
- applications mixed with examples, tools, and docs
- multiple plausible entry points
- infrastructure dependencies

Success is not the number of supported languages.

Success is the percentage of real repositories where Octo reaches the correct execution path without unsafe guessing.

### Reliability rules

1. Prefer repository evidence over convention.
2. Never select auxiliary entry points as production applications without evidence.
3. Never silently invent missing commands or dependencies.
4. Explain why execution cannot proceed.
5. Turn real failures into regression tests.
6. Preserve deterministic behavior when external intelligence is unavailable.

## Phase 2 — Dependency and environment reproduction

Move from command execution toward reproducible local environments.

Potential capabilities:

- runtime compatibility checks
- package-manager detection and installation boundaries
- dependency installation planning
- environment requirement discovery
- service provisioning
- database/cache/message-broker workflows
- containerized dependencies
- stronger health/readiness verification
- explicit machine-change previews

Planning should be inspectable before it mutates the machine.

## Phase 3 — Multi-component execution

Treat repositories as systems rather than single processes.

Potential capabilities:

- reliable monorepo component discovery
- component dependency graphs
- service dependency ordering
- safe parallel execution
- component-scoped environments
- network-reference awareness
- coordinated port allocation
- partial failure handling
- component-level verification
- richer topology explanations

The goal is that `octo run` remains useful even when a repository requires several coordinated processes.

## Phase 4 — Adaptive execution

Introduce controlled adaptation without sacrificing determinism.

Potential capabilities:

- bounded fallback strategies
- failure classification
- execution feedback loops
- verified-strategy reuse and invalidation
- stronger decision-provider integrations
- optional Jev/LLM assistance
- human approval for ambiguous or potentially destructive actions

External intelligence must remain bounded by repository evidence and candidate sets.

## Phase 5 — Ecosystem scale

Expand Octo through extension points rather than a giant language-specific switch statement.

Potential extensions:

- framework detectors
- candidate providers
- runtime adapters
- provisioning providers
- verification providers
- repository-specific strategy providers
- community-maintained ecosystem packages

The core should provide stable primitives and contracts. Ecosystem knowledge should live behind those contracts.

## Phase 6 — Developer trust

Make reliability a product property.

Goals:

- explain important decisions
- provide useful dry-run/planning output
- make machine mutations explicit
- provide safe recovery paths
- make failures actionable
- maintain deterministic offline behavior
- keep verified state auditable
- build a large regression corpus from real repositories

A developer should trust Octo because they can understand what it did, not because Octo claims to be intelligent.

## Phase 7 — Open-source ecosystem

Grow the project around contributors once the technical foundation is strong enough.

Priorities:

- contributor documentation
- architecture guides
- good first issues
- ecosystem-specific issue labels
- clear contribution pathways
- reproducible development environments
- automated regression testing
- release discipline
- maintainer guidelines
- transparent roadmap discussions

### GSoC and similar programs

GSoC should be an outcome of project maturity, not the reason to build Octo.

A credible GSoC-ready project should have:

- a stable, understandable architecture
- active public development
- meaningful contributor opportunities
- well-scoped projects
- documentation that gets new contributors productive
- mentors with enough project context
- a real user/community base

If Octo reaches that point, GSoC can become one channel for bringing strong contributors into the ecosystem.

## Measuring progress

The important metrics are not lines of code or number of languages.

Track:

- successful execution rate on unseen repositories
- false-positive execution rate
- unsafe/unsupported guesses
- actionable failure explanations
- verified-strategy reuse
- regression coverage from real repositories
- time from clone to successful execution
- contributor growth
- independent users

The ultimate metric:

> **How often can a developer clone an unfamiliar repository and get it running without manually reconstructing its environment?**
