# Octo Specification v1

Octo is moving toward a provider-neutral specification for describing how a repository is intended to run.

## Principles

1. Declarative intent.
2. Discoverable from repository evidence.
3. Provider-neutral execution backends.
4. Reproducible desired and verified state.
5. Explainable decisions.
6. Composable multi-component repositories.
7. Offline-first core.

## Lifecycle

Repository -> Discovery -> Evidence -> Project Model -> Execution Plan -> Verification

The Project Model is the canonical internal representation. `.octo.yaml` is a serialized declaration of intent; `.octo.lock` will later record verified execution state.

## Planned shape

    spec: octo.dev/v1
    project:
      name: my-app
    components:
      web:
        type: application
        source:
          path: apps/web
        runtime:
          language: node
          version: "22"
        command:
          dev: pnpm dev
        ports:
          - 3000
        depends_on:
          - api
    services:
      postgres:
        type: database
        image: postgres:17
    environment:
      required:
        DATABASE_URL:
          secret: true

This is a design contract, not a claim that the complete v1 schema is implemented.

## Decision providers

Decision making is an interface, not a vendor dependency. A bounded decision may eventually be answered by deterministic rules, Jev, an LLM, or a human. Octo must remain fully functional without an external decision provider.

## Execution Plan

The execution plan is the deterministic bridge between repository understanding and execution.

An implementation should model execution as a directed acyclic graph where:
- each node is an explicit execution step;
- each dependency is an explicit edge;
- step ordering is derived from dependencies rather than hard-coded globally;
- invalid references and dependency cycles are rejected before execution;
- each step can explain why it exists.

The initial implementation provides a deterministic planner and supports component-level dependencies. It does not yet claim complete multi-service discovery. The analyzer will progressively populate richer component and service graphs as repository detection improves.

Decision systems such as Jev or an LLM may eventually resolve bounded ambiguity, but they are not required to produce a valid baseline plan.

## Repository Topology Discovery

When a repository explicitly declares workspace boundaries, Octo should preserve those boundaries in the ProjectModel rather than flattening the repository into one runnable component.

The initial topology rules are intentionally conservative:
- declared Node workspaces are discoverable from npm/yarn-style `workspaces` or `pnpm-workspace.yaml`;
- each discovered workspace must contain a supported project signal such as `package.json`;
- workspace dependencies are edges only when the dependency is explicitly declared with the `workspace:` protocol;
- every discovered component retains its own path, runtime information, run command, confidence, and evidence;
- folders that merely look like services are not treated as services without repository evidence.

This makes the execution graph evidence-backed. Future detectors can add Docker Compose services, explicit service manifests, or other ecosystem-specific boundaries without changing the graph contract.

## Dependency Edge Evidence

Topology dependencies are not opaque relationships. Every dependency edge MUST retain:
- the source and target node IDs;
- the dependency kind;
- a confidence score;
- supporting repository evidence when available.

For explicitly declared dependencies, the evidence should point to the manifest or configuration source that declared the relationship. Octo MUST NOT increase confidence merely because two components have compatible names or happen to coexist in the same repository.

This distinction is important for extensibility: the core graph represents facts and evidence, while detectors are responsible for discovering those facts. Adding support for another ecosystem should add a detector or adapter rather than embedding ecosystem-specific execution rules into the topology planner.

Relationship edges such as `network_reference` represent communication discovered from explicit repository configuration. They MUST NOT automatically become execution-order dependencies. For example, a static URL whose hostname exactly matches a declared Compose service may produce a `network_reference` edge, while only an explicit `depends_on` declaration produces a `depends_on` edge used for startup ordering. Interpolated runtime values are not resolved during topology discovery.

## Infrastructure Service Discovery

Infrastructure services are modeled separately from source-code components.

Docker Compose is an explicit topology source. When a supported Compose file is present:
- each declared entry under `services` becomes a Service;
- `image`, build context, published ports, and `depends_on` are preserved as topology facts;
- dependency edges are created only for service names actually declared in the Compose file;
- the service retains evidence pointing to the Compose file;
- Compose discovery does not invent an application run command for the service.

This separation allows application components and infrastructure services to participate in one future execution graph without conflating source-code execution with infrastructure provisioning.

## Unified Topology Graph

Components and infrastructure services are represented as typed nodes in one canonical dependency graph.

Node IDs use explicit namespaces:
- `component:<name>`
- `service:<name>`

Dependency edges point from a dependent node to the node it depends on. The graph rejects dangling references, duplicate edges, and dependency cycles.

This graph is the topology layer. The ExecutionPlan MUST retain the exact validated TopologyGraph that was used to derive execution-step dependencies. Consumers such as `inspect`, `graph`, execution reporting, and future runtime orchestration can therefore reason over the same canonical topology rather than reconstructing it independently.

## Topology-Aware Execution Planning

Execution planning consumes the unified topology graph.

For each application component, the planner creates deterministic preparation, installation, setup, and start steps. For each known infrastructure service, it creates a service start step and maps topology dependencies to step dependencies.

A planner must not invent a runtime command when no runtime adapter is known. Instead, the plan should retain the node and explain that execution support is unavailable.

This separates:
- **topology** — what exists and what depends on what;
- **planning** — the ordered execution steps required;
- **runtime adapters** — how a specific node is actually executed.

## Runtime Adapters

Execution plans are runtime-neutral. A RuntimeAdapter maps an executable plan step to a concrete execution mechanism.

The built-in boundary currently includes:
- **shell** for ordinary command execution;
- **compose** for Docker Compose commands.

Adapters are selected explicitly by capability rather than by a global language switch.

The execution engine validates the plan, obtains a stable topological order, resolves an adapter for each executable step, and executes steps in dependency order.

The new execution engine is intentionally separate from the legacy Blueprint/Orchestrator path while it matures.

## Intelligence Run Engine

The CLI may execute the new intelligence engine explicitly with:

`octo run --engine intelligence`

This path analyzes the current repository directly, builds the ProjectModel and TopologyGraph, creates a deterministic ExecutionPlan, and executes it through RuntimeAdapters.

The legacy engine remains the default while parity is established. The intelligence engine currently does not claim support for legacy watch, detach, dashboard, interactive environment provisioning, or port-shifting behavior.

This is an intentional migration boundary, not a second permanent execution architecture.

## Environment Requirements

The ProjectModel may contain an environment requirement model.

Environment requirements contain only:
- variable name;
- whether the current detector considers the variable required;
- source files where the variable was referenced.

Environment values are never stored in ProjectModel, ExecutionPlan, topology output, or explain output.

Environment value resolution is a separate runtime concern. Future providers may read shell variables, local env files, templates, or explicit user input while keeping secret values out of the planning model.

## Runtime Environment Resolution

Environment values are resolved only at execution time and are never part of the ProjectModel or ExecutionPlan.

Resolution precedence is:
1. existing process environment;
2. repository `.env.local`;
3. repository `.env`.

Only variables declared by the ProjectModel are exposed to runtime commands. Missing required variables prevent execution before the first executable step starts.

Resolved values remain in memory for the duration of execution and are not included in plan explanations or serialized artifacts.

## Health and Readiness

A topology dependency is not necessarily a readiness dependency.

When a service declares an explicit health check, the planner creates a `health` step after its start step. Dependents wait for the health step instead of merely waiting for process/container startup.

For Docker Compose, the health probe is executed inside the declared service container using the Compose configuration that provided the evidence.

Health checks are evidence-backed and are not invented from ports alone.

## Port Allocation

Ports are execution resources, not just metadata.

For application components, the planner checks the requested host port. If it is unavailable, Octo selects the next available TCP port and records both the requested and resolved values in the ExecutionPlan. The resolved port is injected as `PORT` for the component's runtime process.

Infrastructure service ports declared by Docker Compose are currently treated as strict declarations. Octo reports conflicts rather than silently rewriting Compose configuration. Future runtime adapters may support parameterized or safely rewritten service port mappings.

## Verification

Verification is a post-execution phase that checks expected runtime state without exposing environment values.

Verification checks are derived from explicit execution-plan/model facts. Current deterministic checks include resolved application TCP ports. Explicit service health checks are already enforced as readiness dependencies by the execution plan.

Verification failures are returned as structured results and do not mutate the execution plan.

## Execution Report

The intelligence execution path produces a serializable ExecutionReport.

The report contains:
- project identity and analysis confidence;
- the execution plan;
- per-step status and selected runtime adapter;
- deterministic verification results;
- a top-level success state and safe failure reason.

Command output and resolved environment values are not part of the report.

## Decision Trace

Octo exposes a decision trace derived from repository evidence and the deterministic execution plan.

Each decision records:
- the decision name and selected value;
- the reason for the decision;
- supporting evidence where available;
- a confidence score.

Execution-step explanations are included as decisions so users can understand dependency ordering, runtime selection, health/readiness gates, and port allocation.

Decision traces contain no resolved environment values or command output.

## Default Execution Engine

The intelligence execution path is the canonical Octo execution path.

The CLI may retain a legacy execution engine temporarily for compatibility and feature gaps, but new execution capabilities MUST target the intelligence architecture first.

The intelligence path is responsible for analysis, planning, environment resolution, execution, verification, and reporting.

## Provisioning

Provisioning is an explicit execution phase.

The planner derives machine prerequisites from repository facts and emits deterministic provisioning checks before dependency installation. The initial provisioning implementation verifies required package managers with `command -v`.

Planning MUST NOT mutate the user's machine. Automatic installation or bootstrap providers such as Corepack or Bun installation require an explicit provisioning policy and must remain visible in the execution report.

## Application Network References

Octo may discover informational application-to-application relationships from explicit static configuration. The initial detector reads component-local dotenv files (`.env` and `.env.*`) and recognizes URL values whose hostname exactly matches a known component or Compose service.

These references MUST:
- be represented as `network_reference` relationships, not startup dependencies;
- include evidence identifying the source file and environment variable name without storing the value;
- ignore interpolated or otherwise unresolved values;
- reject ambiguous targets when a component and service share the same name rather than guessing.

Raw environment values, credentials, tokens, and URL contents MUST NOT be serialized into the ProjectModel, topology, plan, explanation, or report.


## Topology Relationship Semantics

Topology edges have an explicit relationship kind. Octo v1 defines:

- `depends_on`: an execution dependency. A consumer MUST NOT be scheduled before the referenced node satisfies the planner's required readiness gate.
- `network_reference`: an informational communication relationship. It MUST NOT create execution ordering or participate in dependency-cycle detection.

Consumers MUST reject unknown relationship kinds rather than silently assigning execution semantics. This keeps topology facts and execution policy separate and makes future relationship kinds safe to introduce deliberately.

The relationship kind is part of the canonical topology identity for duplicate-edge validation: two nodes may have both a `depends_on` and a `network_reference` relationship without those relationships being treated as duplicates.
