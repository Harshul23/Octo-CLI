---
name: Ecosystem / Language Support Request
about: Propose adding or expanding support for a language, package manager, or framework
title: "[ECOSYSTEM]: Support for "
labels: ["ecosystem", "enhancement"]
assignees: ""
---

### Ecosystem Summary
- **Language / Runtime**: [e.g. Kotlin, Scala, Swift, Dart, Zig]
- **Primary Package Manager / Build Tool**: [e.g. gradle, sbt, swiftpm, pub, zig build]
- **Canonical Manifests & Signals**: [e.g. `build.gradle.kts`, `Package.swift`, `pubspec.yaml`]
- **Lockfile(s)**: [e.g. `gradle.lockfile`, `Package.resolved`, `pubspec.lock`]

### Typical Entry Points & Development Commands
- Development / Start: [e.g. `swift run`, `dart run`, `gradle bootRun`]
- Build / Setup: [e.g. `swift package resolve`, `dart pub get`]
- Port Conventions: [e.g. explicit flags, environment variables, default configs]

### Sample Repositories
<!-- Link 1-2 open-source repositories using this ecosystem -->
- 

### Proposed Extension Strategy
- [ ] New `SignalDefinition` / detector via `RegisterProjectSignal` or `RegisterProjectDetector`
- [ ] New `CandidateProvider` via `RegisterCandidateProvider`
- [ ] Hermetic table-driven regression test case in `internal/intelligence/`
