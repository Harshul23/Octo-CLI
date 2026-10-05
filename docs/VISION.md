# Octo Vision

> **Make “how do I run this?” disappear.**

Octo is an open-source execution-intelligence project built around one stubborn developer problem:

> I cloned this repository. How the hell do I run it?

The long-term goal is not to collect startup commands for every language. It is to make repository execution a first-class, explainable, verifiable capability.

## What Octo should become

A developer should be able to enter an unfamiliar repository and run:

```bash
octo run
```

Octo should inspect the repository, understand what is actually present, identify safe execution strategies, resolve bounded ambiguity, execute them, verify that the result is real, and remember only what it has proven.

```text
Repository
    ↓
Evidence
    ↓
Project model
    ↓
Execution candidates
    ↓
Bounded decision
    ↓
Execution plan
    ↓
Runtime
    ↓
Verification
    ↓
Verified strategy
```

If Octo cannot safely determine how to run something, it should explain why.

**Refusing to guess is a feature.**

## The deeper problem

Modern repositories can contain multiple applications, workspaces, infrastructure services, runtimes, generated code, legacy entry points, undocumented assumptions, environment requirements, conflicting ports, and several plausible commands.

The real problem is not:

> “What command starts a Node app?”

It is:

> **“Given this repository as evidence, what execution path is actually justified?”**

That distinction defines Octo.

## Core principles

### Evidence before inference

Prefer facts discovered from repository artifacts over assumptions based on language names or conventions.

### Candidates before decisions

When several strategies are plausible, generate a bounded candidate set and select from it. External intelligence must not invent commands.

### Verification before trust

A command finishing successfully is not automatically proof that an application is working. Verify meaningful runtime state where evidence allows it.

### Failure becomes evidence

A failed strategy can improve the decision process, but it must never become permission to invent an unrelated strategy.

### Verified state is valuable

A strategy that has executed and verified successfully can be reused until execution-relevant repository state changes.

### Optional intelligence

LLMs and agents may improve bounded decisions, but Octo's core must remain useful without a paid model, network connection, or external intelligence service.

### Explainability is a product feature

Developers should be able to understand what Octo detected, which candidates existed, why one was selected, what failed, and why success was accepted.

## What Octo is not

Octo is not intended to become:

- a giant hard-coded list of shell commands
- an LLM wrapper that guesses commands
- a replacement for Git
- a package manager
- a generic CI platform
- a magic system that hides failures

Octo should sit above these systems and answer one question extremely well:

> **How can this repository be executed safely and reproducibly on this machine?**

## Long-term ambition

Octo should become infrastructure that open-source projects can rely on, not a demo that only works on carefully prepared repositories.

The project should grow toward:

- a strong contributor ecosystem
- stable extension points
- reproducible execution behavior
- excellent documentation
- transparent design decisions
- a healthy issue and PR culture
- ecosystem-specific contributors
- educational opportunities such as GSoC when the project has the maturity and community required for them

The goal is not fame for its own sake.

> **Build something useful enough that developers want it to exist, improve it, and eventually depend on it.**
