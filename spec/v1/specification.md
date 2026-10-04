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

This graph is the topology layer. Execution planning remains a separate concern and can map topology nodes to runtime-specific execution steps later.

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

