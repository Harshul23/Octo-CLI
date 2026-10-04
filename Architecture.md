# Octo Architecture

## 1. Purpose

Octo is evolving from a conventional local-deployment CLI into an **execution-intelligence system for software repositories**.

The central problem is:

> **A repository has been cloned. How can Octo determine a safe, evidence-backed, reproducible way to run it?**

The architecture is designed around one long-term objective:

> **Make “how do I run this?” disappear.**

This does not mean that Octo should contain an enormous list of language-specific startup rules. It means Octo should discover repository evidence, represent that evidence in a canonical model, generate bounded execution possibilities, choose among them, execute them, verify the result, and remember only strategies that have actually worked.

The intended lifecycle is:

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
Execution
    ↓
Verification
    ↓
.octo.lock
```

This document explains the architectural responsibilities, boundaries, data flow, safety rules, and migration state of the repository.

---

# 2. Core architectural principles

## 2.1 Evidence before inference

Octo's repository understanding begins with evidence.

Evidence can come from:

- signal files
- manifests
- lockfiles
- package scripts
- configuration
- workspace declarations
- source files
- Compose files
- explicit dependency declarations
- explicit environment references
- execution failures

An evidence record contains a kind, a repository path, a human-readable detail, and a strength/confidence value.

The important rule is:

> **A detector discovers facts; it should not silently manufacture execution behavior.**

For example, finding `main.go` is evidence that a Go entry file exists. It is not, by itself, proof that `go run main.go` is the correct way to execute the entire repository.

---

## 2.2 The core must remain provider-neutral

Octo needs ecosystem-specific knowledge, but that knowledge should live behind explicit boundaries.

The core should provide generic primitives for:

- evidence
- models
- topology
- candidates
- decisions
- plans
- execution
- verification

Ecosystem-specific behavior should be implemented by:

- detectors
- candidate providers
- runtime adapters

This prevents the core from becoming a giant conditional tree:

```text
if Go ...
if Node ...
if Python ...
if Rust ...
if Java ...
if Ruby ...
...
```

Instead:

```text
Repository evidence
       ↓
Native detector
       ↓
ProjectModel
       ↓
Ecosystem-specific candidate provider
       ↓
Generic decision layer
       ↓
Generic planner
       ↓
Runtime adapter
```

---

## 2.3 Bounded ambiguity

Octo will inevitably encounter ambiguity.

For example:

- a project may expose both `dev` and `start` scripts;
- a Go repository may contain both `main.go` and sibling package files;
- multiple runtime strategies may be technically possible;
- a human may need to choose between valid alternatives.

Octo handles this through a **bounded decision boundary**.

The decision layer receives explicit options and selects one.

It does not discover new repository facts.

It does not invent commands.

It does not mutate the repository model with unsupported guesses.

---

## 2.4 Offline-first core

The intelligence engine must remain useful without an external AI provider.

The deterministic decision provider is therefore the baseline.

Optional decision providers can eventually include:

- deterministic logic
- Jev
- an LLM
- a human

Jev or an LLM can improve decisions under ambiguity, but neither is allowed to become a hidden dependency of Octo's core execution path.

---

## 2.5 Explainability

Every important decision should be explainable from structured information.

Octo therefore preserves:

- evidence
- confidence
- candidate IDs
- candidate evidence
- topology edges
- execution-step explanations
- verification results
- decision traces

The goal is that a user can ask:

> Why did Octo choose this command?

and the system can answer from repository evidence instead of an opaque heuristic.

---

# 3. System overview

The complete conceptual system is:

```text
                         ┌─────────────────────┐
                         │     Repository      │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │ Evidence Discovery  │
                         │                     │
                         │ detectors/signals   │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │    ProjectModel     │
                         │                     │
                         │ components          │
                         │ services            │
                         │ environment model   │
                         │ evidence            │
                         └───────┬───────┬─────┘
                                 │       │
                       ┌─────────┘       └──────────┐
                       ▼                            ▼
              ┌─────────────────┐         ┌────────────────────┐
              │ Topology Graph  │         │ Execution Candidates│
              │                 │         │                    │
              │ dependencies    │         │ ecosystem providers│
              │ references      │         │ evidence-backed    │
              │ readiness       │         └─────────┬──────────┘
              └────────┬────────┘                   │
                       │                            ▼
                       │                  ┌────────────────────┐
                       │                  │ Decision Provider  │
                       │                  │                    │
                       │                  │ deterministic/Jev/ │
                       │                  │ LLM/human          │
                       │                  └─────────┬──────────┘
                       │                            │
                       └──────────────┬─────────────┘
                                      ▼
                         ┌─────────────────────┐
                         │   ExecutionPlan     │
                         │                     │
                         │ steps               │
                         │ dependencies        │
                         │ candidates          │
                         │ ports               │
                         │ provisioning        │
                         │ environment metadata│
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │ Runtime Adapters    │
                         │                     │
                         │ shell / compose     │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │     Execution       │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │    Verification     │
                         └──────────┬──────────┘
                                    │
                           success │
                                    ▼
                         ┌─────────────────────┐
                         │    .octo.lock       │
                         │ verified strategies │
                         └─────────────────────┘
```

---

# 4. Repository intelligence

## 4.1 Native intelligence layer

The native intelligence layer owns repository understanding.

Its central contract is the `ProjectModel`.

The model currently represents:

- project name
- repository root
- language
- runtime version
- framework
- package manager
- setup command
- run command
- monorepo state
- port
- confidence
- components
- services
- environment requirements
- environment resolutions
- supporting evidence

The important architectural rule is:

> **ProjectModel is the canonical internal representation of repository understanding.**

It is not merely a compatibility structure around the old analyzer.

---

# 5. Evidence model

## 5.1 EvidenceKind

The intelligence layer categorizes evidence.

Current evidence categories include:

- `signal_file`
- `lockfile`
- `manifest`
- `script`
- `config`
- `execution_failure`

The categories describe where an inference came from.

## 5.2 Evidence

An evidence record contains:

```text
kind
path
detail
strength
```

Example:

```text
kind: signal_file
path: go.mod
detail: The component is a Go module.
strength: 0.95
```

Evidence is deliberately retained throughout the intelligence pipeline so later decisions can remain explainable.

---

# 6. ProjectModel

The ProjectModel is the canonical repository-understanding object.

Conceptually:

```text
ProjectModel
├── identity
├── runtime information
├── execution information
├── components
├── services
├── environment
└── evidence
```

A top-level model can contain:

- `Name`
- `Root`
- `Language`
- `RuntimeVersion`
- `Framework`
- `PackageManager`
- `RunCommand`
- `SetupCommand`
- `Monorepo`
- `Port`
- `Confidence`
- `Components`
- `Services`
- `Environment`
- `Evidence`

The model describes what Octo has learned.

It should not contain resolved secrets or runtime-only environment values.

---

# 7. Components

A Component represents an executable application part of a repository.

A component can contain:

- name
- path
- language
- framework
- package manager
- run command
- dependencies
- references
- port
- confidence
- evidence
- execution candidates

A component is not necessarily the whole repository.

This is critical for monorepos.

For example:

```text
repository/
├── apps/
│   ├── web/
│   └── api/
└── packages/
    └── shared/
```

Octo should preserve explicit application boundaries instead of pretending the repository is one executable unit.

---

# 8. Infrastructure services

A Service represents infrastructure discovered from explicit repository evidence.

A service can contain:

- name
- image
- build context
- dependencies
- references
- ports
- health check
- confidence
- evidence

Docker Compose is an explicit topology source.

For a Compose file:

```yaml
services:
  postgres:
    image: postgres:17
```

Octo can represent `postgres` as a service node.

The service remains infrastructure. Octo does not turn it into a source-code application component.

---

# 9. Topology

## 9.1 Why topology exists

Repositories can contain relationships that affect execution.

Examples:

```text
web → api
api → postgres
worker → redis
```

These relationships should not be rediscovered independently by every subsystem.

Octo therefore creates a canonical `TopologyGraph`.

---

## 9.2 Topology nodes

Nodes have explicit namespaces:

```text
component:<name>
service:<name>
```

This prevents collisions between:

```text
component:api
service:api
```

The graph knows that these are different entities.

---

## 9.3 Topology edges

An edge contains:

- source
- target
- relationship kind
- confidence
- evidence

Current relationship kinds are:

### `depends_on`

An execution dependency.

If:

```text
web depends_on api
```

the planner must ensure that `api` satisfies the relevant readiness gate before `web` is started.

### `network_reference`

An informational communication relationship.

For example:

```text
api
 └── DATABASE_URL → service:postgres
```

This does not automatically mean:

```text
api depends_on postgres
```

The two concepts remain separate.

---

## 9.4 Topology validation

The topology builder rejects:

- empty node names
- duplicate node IDs
- unknown source nodes
- unknown target nodes
- invalid relationship kinds
- invalid dependency references

Dependency semantics are therefore validated before execution planning.

---

## 9.5 Dependency cycles

Execution dependencies must form a valid directed acyclic graph.

A cycle such as:

```text
A → B
B → C
C → A
```

cannot produce a deterministic startup order.

The topology/planning system therefore rejects dependency cycles rather than attempting to guess an order.

---

# 10. Repository topology discovery

Octo uses repository evidence to discover topology.

## 10.1 Workspace discovery

Explicit Node workspace declarations can define repository boundaries.

Supported evidence can include:

- npm/yarn `workspaces`
- `pnpm-workspace.yaml`
- workspace dependency declarations

A directory is not treated as a component merely because it looks like one.

A discovered workspace must contain supported project signals.

---

## 10.2 Explicit dependency evidence

Workspace dependencies become topology edges only when the repository explicitly declares them.

For example:

```json
{
  "dependencies": {
    "@repo/shared": "workspace:*"
  }
}
```

can provide evidence for a dependency relationship.

Octo does not increase confidence simply because two components have compatible names.

---

# 11. Execution candidates

## 11.1 Why candidates exist

Repository detection and command selection are different responsibilities.

Detection may discover:

```text
Language = Go
main.go exists
multiple Go files exist
go.mod exists
```

That evidence can produce multiple execution candidates.

The candidate system prevents the detector from hardcoding one command as truth.

---

## 11.2 ExecutionCandidate

A candidate contains:

- ID
- command
- confidence
- evidence

Conceptually:

```text
ExecutionCandidate
├── ID
├── Command
├── Confidence
└── Evidence
```

---

## 11.3 CandidateProvider

The `CandidateProvider` abstraction allows ecosystem-specific execution knowledge without contaminating the generic planner.

A provider answers:

1. Does this provider support the component?
2. What evidence-backed candidates can it produce?

Current providers include:

- Go
- Node

The registry is intentionally extensible.

Future ecosystems should be added by implementing providers rather than expanding the core planner with language-specific branches.

---

# 12. The Go candidate problem

Dogfooding Octo exposed an important example.

A repository can contain:

```text
main.go
gum.go
choose/
confirm/
...
```

If `main.go` refers to symbols defined in `gum.go`, then:

```bash
go run main.go
```

does not compile the complete package.

But:

```bash
go run .
```

does.

This demonstrates why Octo must reason from repository structure rather than relying on simplistic:

```text
Go + main.go → go run main.go
```

logic.

The candidate system can represent both strategies and rank the package-level execution higher when multiple Go files provide evidence that the complete package should be compiled.

---

# 13. Decision architecture

## 13.1 DecisionProvider

The decision layer is defined by a provider-neutral interface.

Conceptually:

```go
type DecisionProvider interface {
    Decide(ctx context.Context, request DecisionRequest) (DecisionResult, error)
}
```

The interface receives:

```text
DecisionRequest
├── Name
└── Options[]
```

Each option contains:

- ID
- value
- confidence
- evidence

The result contains:

- selected option ID
- selected value
- confidence
- reason

---

## 13.2 Deterministic provider

The deterministic provider is the offline baseline.

Its behavior is deliberately simple:

1. validate the decision request
2. reject an empty candidate set
3. order candidates by confidence
4. use the candidate ID as a deterministic tie-breaker
5. select the strongest candidate

No external service is required.

---

## 13.3 Jev and LLM providers

Jev and other LLM-based providers are optional decision providers.

They can eventually help with bounded ambiguity.

They do not own:

- repository discovery
- topology construction
- execution
- secret resolution
- arbitrary command generation

The safe boundary is:

```text
Octo discovers candidates
        ↓
Provider chooses among candidates
        ↓
Octo executes selected candidate
```

Not:

```text
LLM sees repository
        ↓
LLM invents command
        ↓
Octo executes it
```

---

# 14. ExecutionPlan

## 14.1 Purpose

The ExecutionPlan is the deterministic bridge between repository understanding and actual execution.

It represents an executable graph.

Each step can contain:

- ID
- component
- topology node
- phase
- command
- candidates
- selected candidate
- working directory
- dependencies
- environment metadata
- explanation

The plan also retains:

- the exact topology graph used
- port assignments
- provisioning requirements
- environment bindings

---

## 14.2 Execution phases

The planner can create phases such as:

```text
prepare
provision
install
setup
start
health
```

The exact phases are derived from repository facts and known execution requirements.

For an application component, a typical path is:

```text
prepare
   ↓
provision
   ↓
install
   ↓
setup
   ↓
start
```

Not every component needs every phase.

---

## 14.3 Planning rules

The deterministic planner:

- requires a project name
- requires at least one component or service
- builds and validates topology
- creates deterministic component steps
- maps topology dependencies into step dependencies
- creates infrastructure service steps
- creates health steps when explicit health checks exist
- allocates application ports
- injects resolved port information into runtime step metadata
- validates the final execution graph

The planner must not invent a runtime command when no known execution adapter can support the discovered node.

---

# 15. Execution ordering

Execution dependencies are represented as graph edges.

Octo calculates a stable topological order.

If:

```text
postgres
   ↓
api
   ↓
web
```

the execution order must respect:

```text
postgres → api → web
```

Independent nodes can remain independent.

The planner does not rely on a globally hardcoded sequence.

---

# 16. Readiness and health

Starting a process is not necessarily equivalent to making a dependency ready.

If a service declares an explicit health check:

```text
service.start
      ↓
service.health
      ↓
dependent.start
```

The dependent waits for the health gate rather than merely waiting for the process/container to start.

For Compose services, the health command can be executed inside the declared service container using the Compose configuration that supplied the evidence.

Octo does not invent health checks from ports alone.

---

# 17. Provisioning

Provisioning is modeled as an explicit execution phase.

The planner can derive prerequisites from repository facts.

The initial implementation includes package-manager availability checks such as:

```bash
command -v npm
command -v pnpm
command -v yarn
command -v bun
```

The architectural rule is:

> **Planning should describe prerequisites; planning should not silently mutate the user's machine.**

Automatic installation or bootstrap behavior requires an explicit provisioning policy.

---

# 18. Runtime adapters

## 18.1 Why adapters exist

The execution plan should not be coupled to one execution technology.

A plan can say:

```text
Run this command in this working directory.
```

A runtime adapter determines how that step is actually executed.

---

## 18.2 Current adapter boundary

The built-in execution boundary currently includes:

### Shell

Used for ordinary commands.

Examples include:

```text
npm install
go run .
python ...
```

### Compose

Used for Docker Compose-backed infrastructure.

Examples include:

```bash
docker compose -f compose.yml up -d postgres
```

The adapter is selected by capability rather than by a global language switch.

---

# 19. Environment architecture

## 19.1 Environment requirements

The ProjectModel can contain environment requirements.

An environment variable model contains:

- variable name
- required status
- source paths

The model does not contain the value.

---

## 19.2 Runtime-only values

Resolved environment values exist only at execution time.

They must never be embedded into:

- ProjectModel
- ExecutionPlan
- TopologyGraph
- DecisionTrace
- ExecutionReport
- .octo.lock

This is a hard architectural boundary.

---

## 19.3 Resolution precedence

The intelligence environment resolver uses this precedence:

1. process environment
2. component `.env.local`
3. component `.env`
4. repository-root `.env.local`
5. repository-root `.env`

Process environment therefore has highest authority.

---

## 19.4 Component scoping

For monorepos, environment values are component-scoped.

Suppose:

```text
apps/api/.env
DATABASE_URL=...
```

That value should be available to the API component's execution steps.

It must not automatically become visible to:

```text
apps/web
```

The execution engine therefore resolves environment separately for each executable step using the step's component/work directory.

---

## 19.5 Required-variable ownership

If a required variable is referenced by a component, a value from an unrelated component must not satisfy the requirement.

This prevents:

```text
apps/web/.env
API_ONLY=...
```

from falsely satisfying a requirement owned by:

```text
apps/api
```

When ownership cannot be reliably determined, Octo preserves a conservative fallback rather than inventing ownership.

---

# 20. Secret-safe environment bindings

Octo can represent relationships such as:

```text
api
 └── DATABASE_URL → service:postgres
```

The binding contains metadata such as:

- variable name
- source node
- target
- relationship kind
- confidence
- evidence

It does not contain:

- resolved URL
- password
- token
- credential
- raw environment value

Static topology information is therefore safe to retain in planning structures.

---

# 21. Network references

Application network references can be discovered from explicit static configuration.

The current model is conservative.

A reference can be created when a static URL's hostname exactly matches a known component or Compose service.

Interpolated values are not resolved.

For example:

```text
DATABASE_URL=http://postgres:5432
```

can provide evidence for a reference to `service:postgres`.

But:

```text
DATABASE_URL=http://${POSTGRES_HOST}:5432
```

is unresolved and should not be guessed.

Network references remain informational.

They do not become startup dependencies automatically.

---

# 22. Port allocation

Ports are runtime resources.

For an application component:

1. Octo reads the requested port.
2. It checks availability.
3. If unavailable, it selects an available TCP port.
4. It records requested and resolved ports.
5. The resolved value is exposed to the component as `PORT`.

This means the plan can distinguish:

```text
requested: 3000
resolved: 3001
automatic: true
```

Infrastructure ports declared by Compose are currently stricter.

Octo reports conflicts rather than silently rewriting Compose service definitions.

---

# 23. Execution engine

The intelligence execution path is the canonical architecture.

The current CLI path is:

```text
octo run
   ↓
intelligence.Analyze
   ↓
Load .octo.lock
   ↓
Apply valid verified strategies
   ↓
DeterministicPlanner
   ↓
ResolveProjectEnvironment
   ↓
ExecutePlanReport
   ↓
Verification
   ↓
RecordVerifiedStrategies
```

The intelligence path currently does not claim all legacy execution features such as:

- watch
- detach
- dashboard
- interactive legacy environment provisioning
- legacy port-shifting behavior

Those limitations are part of the migration boundary.

The older engine remains temporarily available through:

```bash
octo run --engine legacy
```

This is compatibility, not a second permanent architecture.

---

# 24. Execution reports

The intelligence engine produces a structured `ExecutionReport`.

The report can contain:

- project identity
- analysis confidence
- execution plan
- per-step status
- selected runtime adapter
- deterministic verification results
- overall success
- safe failure reason

The report must not contain:

- resolved environment values
- credentials
- command output

This makes the report safe to serialize and use for explanation.

---

# 25. Decision traces

Decision traces provide an explainable record of bounded decisions.

A trace entry can include:

- decision name
- selected option
- selected value
- confidence
- reason
- supporting evidence
- outcome

For execution feedback, outcomes can represent that a candidate was:

- selected
- failed
- succeeded

The trace records reasoning without storing command output or secrets.

---

# 26. Bounded execution feedback

Execution failure is itself useful evidence.

Suppose:

```text
Candidate A:
go run main.go
```

fails because a sibling file is required.

Octo may then try:

```text
Candidate B:
go run .
```

if Candidate B was already part of the validated candidate set.

The forbidden behavior is:

```text
Candidate A fails
      ↓
Octo invents arbitrary command C
      ↓
Execute C
```

The safety rule is:

> **Failure can change which existing candidate is selected; it cannot manufacture a new execution strategy.**

---

# 27. Verified strategies

## 27.1 Why .octo.lock exists

Repository analysis can be repeated, but successful execution contains valuable knowledge.

If Octo has already proved that:

```text
component X
candidate Y
command Z
```

works for a specific repository state, that knowledge should not be discarded.

`.octo.lock` stores this verified state.

---

## 27.2 VerifiedStrategy

A verified strategy contains:

- component
- candidate
- command
- execution-relevant fingerprint
- verification timestamp

The lock therefore represents:

> **This strategy was successfully verified for this repository state.**

---

## 27.3 Fingerprints

The execution fingerprint is designed to include execution-relevant repository state.

Relevant files can include:

- source files
- package manifests
- lockfiles
- scripts/configuration
- runtime configuration
- candidate evidence

Generated/dependency trees such as:

- `node_modules`
- `vendor`
- `target`
- `dist`
- `build`
- `.git`

are excluded.

Documentation changes should not invalidate a verified execution strategy.

---

## 27.4 Lock reuse

Before reusing a strategy, Octo checks:

1. the candidate still exists
2. the command still matches
3. the execution fingerprint still matches

If the strategy is stale:

```text
.octo.lock strategy
      ↓
fingerprint mismatch
      ↓
discard/reject reuse
      ↓
normal discovery
      ↓
candidate selection
```

A successful new execution can refresh the lock.

---

# 28. Verification

Verification is not optional decoration.

It establishes whether the selected execution strategy actually produced the expected runtime state.

Current deterministic verification includes:

## Port verification

Octo checks whether the resolved application TCP port is reachable.

## Health verification

Explicit service health checks are already enforced as readiness dependencies in the execution plan.

Verification results are structured:

```text
CheckID
Passed
Reason
```

Verification does not mutate the execution plan.

---

# 29. The legacy architecture

Octo originally centered around:

```text
octo init
   ↓
Analyze project
   ↓
.octo.yaml
   ↓
Blueprint
   ↓
Orchestrator
   ↓
Run
```

This architecture is still present in parts of the repository because migration is incremental.

The legacy path includes concepts such as:

- `internal/blueprint`
- `internal/orchestrator`
- legacy `.octo.yaml` workflows
- compatibility UI flows

It should not be used as the architectural template for new intelligence features.

---

# 30. Why the migration happened

The old architecture mixed repository understanding with compatibility-shaped execution behavior.

That created several problems:

1. the legacy analyzer became a second source of truth;
2. command inference was too tightly coupled to ecosystem heuristics;
3. topology was not originally a canonical first-class graph;
4. execution decisions were harder to explain;
5. successful execution knowledge was not treated as durable verified state.

The migration therefore proceeded incrementally.

---

# 31. Intelligence migration history

The major architectural progression was:

### PR #3
Established the execution-intelligence foundation.

### PR #18
Continued intelligence architecture work.

### PR #19
Introduced explicit provisioning.

### PR #20
Made topology a first-class concept.

### PR #21
Continued the topology/execution migration.

### PR #22
Introduced network-reference concepts.

### PR #23
Made topology identity and readiness deterministic.

### PR #24
Added application network-reference discovery.

### PR #25
Formalized topology relationship kinds.

### PR #26
Added secret-safe environment bindings.

### PR #27
Added component-scoped runtime environments.

### PR #28
Applied scoped environments at the actual execution boundary.

### PR #29
Corrected required environment ownership semantics.

### PR #30
Introduced the provider-neutral decision layer.

### PR #31
Introduced evidence-backed execution candidates.

### PR #32
Made candidate providers extensible.

### PR #33
Added bounded execution feedback and fallback.

### PR #34
Exposed execution decision traces.

### PR #35
Introduced verified execution strategies through `.octo.lock`.

### PR #36
Migrated consumers toward the intelligence boundary.

### PR #37
Isolated legacy project detection behind a temporary boundary.

### PR #38
Replaced the legacy project detector with native detection.

### PR #39
Removed the legacy analyzer package.

### PR #40
Moved `octo init` and blueprint generation onto the native `ProjectModel`.

### PR #41
Migrated server analysis from `ProjectInfo` to `ProjectModel`.

### PR #42
Removed the remaining legacy `ProjectInfo` compatibility layer.

This history matters because the current architecture is the result of deliberate convergence, not an accidental collection of subsystems.

---

# 32. Current migration state

The architecture is now substantially native to `internal/intelligence`.

The important current state is:

```text
Legacy analyzer
      X
      │
      │ removed
      ▼
Native intelligence
      │
      ├── ProjectModel
      ├── topology
      ├── candidates
      ├── decisions
      ├── planning
      ├── environments
      ├── execution
      ├── verification
      └── lock state
```

The repository still contains legacy execution infrastructure because the intelligence engine is still being brought to feature parity.

That compatibility code is a migration boundary, not the desired final design.

---

# 33. What .octo.yaml means now

`.octo.yaml` should be understood carefully.

It belongs to the original configuration/blueprint workflow.

It is not the canonical internal representation of repository intelligence.

The canonical internal representation is:

```text
ProjectModel
```

The intelligence execution path analyzes the repository directly rather than requiring an existing `.octo.yaml`.

This is why the modern command is:

```bash
octo run
```

rather than:

```text
octo init
octo run
```

The latter remains useful only for the transitional legacy workflow.

---

# 34. What .octo.lock means

The two files have fundamentally different roles.

```text
.octo.yaml
    ↓
configuration / serialized legacy intent

.octo.lock
    ↓
verified execution state
```

The lock is not a configuration replacement.

It is evidence that a strategy has already worked for a specific repository state.

---

# 35. CLI architecture

The CLI layer in `cmd/` should remain relatively thin.

Its responsibility is to:

- parse command-line input
- select an execution path
- call the appropriate intelligence or compatibility layer
- present results/errors

The intelligence package should own repository understanding and execution semantics.

The CLI should not become another place where ecosystem-specific inference is implemented.

---

# 36. Runtime boundary

The runtime boundary exists to prevent repository intelligence from becoming coupled to execution technology.

The conceptual split is:

```text
Intelligence
    │
    │ ExecutionPlan
    ▼
Runtime
    │
    ├── shell
    ├── compose
    └── future adapters
```

This means adding another runtime should not require rewriting repository detection.

---

# 37. Adding a new ecosystem

Suppose Octo wants to improve Python support.

The preferred architecture is:

```text
Python repository evidence
        ↓
Python detector
        ↓
ProjectModel facts
        ↓
Python CandidateProvider
        ↓
ExecutionCandidate[]
        ↓
DecisionProvider
        ↓
generic planner
        ↓
runtime adapter
```

Do not add:

```text
if language == Python {
    ...
}
```

to the central planner for every new behavior.

The ecosystem should provide evidence and candidates; generic layers should consume them.

---

# 38. Adding a new runtime

Suppose a new runtime backend is introduced.

It should implement the runtime adapter contract and advertise its capabilities.

The intelligence layer should continue producing the same runtime-neutral execution plan.

This preserves the separation:

```text
What should happen?
    ↓
ExecutionPlan

How should this step happen?
    ↓
RuntimeAdapter
```

---

# 39. Adding an AI provider

An AI provider should implement the decision boundary.

It receives bounded options.

For example:

```text
Decision:
run_command

Options:
A → go run main.go
B → go run .
```

The provider can select A or B.

It should not return:

```text
go build && ./some-other-command
```

unless that strategy was already present in the supplied candidate set.

This keeps AI useful without making the system dependent on unconstrained model behavior.

---

# 40. Failure handling

Octo distinguishes between:

### Discovery failure

Octo cannot establish sufficient repository facts.

### Planning failure

Known facts cannot produce a valid execution plan.

### Execution failure

A planned step could not execute successfully.

### Verification failure

Execution completed but the expected runtime state was not verified.

These failures should remain structured.

A failure should improve the system's evidence without silently changing architectural contracts.

---

# 41. Safety boundaries

The architecture intentionally enforces several safety boundaries.

## No invented execution commands

Commands must originate from evidence-backed candidates or explicitly known runtime behavior.

## No secret persistence

Resolved values never enter planning artifacts.

## No implicit topology semantics

A network reference does not automatically become a startup dependency.

## No silent topology ambiguity

Unknown or ambiguous topology references are rejected rather than guessed.

## No unsafe lock reuse

Verified strategies are reused only when their candidate and execution fingerprint remain valid.

## No mandatory AI dependency

The deterministic provider remains capable of making baseline decisions.

---

# 42. Determinism

Determinism is important because Octo is intended to be an execution system, not a conversational guesser.

The architecture uses deterministic behavior in:

- candidate sorting
- decision tie-breaking
- topology validation
- topological execution ordering
- port allocation policy
- plan validation
- verification
- lock fingerprinting

When two valid choices have equal confidence, stable IDs and ordering should provide predictable behavior.

---

# 43. What Octo is not

Octo is not intended to be:

### A giant shell-command database

It should discover execution strategies from repository evidence.

### An LLM wrapper

AI is an optional decision provider, not the architecture.

### A language-specific build system

The core should not become a collection of ecosystem-specific branches.

### A secret manager

Octo resolves environment values for execution but does not make resolved values part of its persistent intelligence model.

### A Docker-only orchestrator

Docker Compose is one runtime/topology source. The architecture is runtime-neutral.

### A configuration-file-first tool

The modern intelligence engine analyzes repositories directly. `.octo.yaml` is part of the transitional legacy path.

---

# 44. Long-term architecture

The desired mature architecture is:

```text
                         ┌──────────────────────┐
                         │      Repository      │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │  Evidence Discovery  │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │     ProjectModel     │
                         └───────┬────────┬─────┘
                                 │        │
                    ┌────────────┘        └────────────┐
                    ▼                                  ▼
             ┌──────────────┐                  ┌──────────────┐
             │   Topology   │                  │  Candidates  │
             └──────┬───────┘                  └──────┬───────┘
                    │                                  │
                    └──────────────┬───────────────────┘
                                   ▼
                         ┌──────────────────────┐
                         │   DecisionProvider   │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │    ExecutionPlan     │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │   Runtime Adapters   │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │      Execution       │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │     Verification     │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │     .octo.lock       │
                         └──────────────────────┘
```

The architecture should become stronger by adding better evidence and providers, not by accumulating special cases in the core.

---

# 45. The central Octo loop

Everything ultimately comes back to this loop:

```text
Understand
    ↓
Generate bounded possibilities
    ↓
Decide
    ↓
Plan
    ↓
Execute
    ↓
Verify
    ↓
Remember what worked
```

Or, more precisely:

```text
Evidence
  ↓
Knowledge
  ↓
Candidates
  ↓
Bounded decision
  ↓
Plan
  ↓
Execution
  ↓
Evidence from outcome
  ↓
Verification
  ↓
Verified knowledge
```

That final feedback loop is what separates Octo from a simple command generator.

---

# 46. Architectural north star

The simplest statement of the architecture is:

> **Generic primitives in the core; ecosystem-specific knowledge behind providers and detectors; execution strategies must be evidence-backed and verified.**

And the simplest statement of the product vision is:

> **Make “how do I run this?” disappear.**

Everything added to Octo should be evaluated against those two statements.
