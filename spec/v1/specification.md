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
