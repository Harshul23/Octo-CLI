---
name: Bug Report
about: Report an unexpected behavior, incorrect candidate, or execution failure
title: "[BUG]: "
labels: ["bug"]
assignees: ""
---

### Describe the Bug
<!-- A clear and concise description of what went wrong. -->

### Repository Layout
<!-- Provide the directory layout and key manifest files of the repository being analyzed. -->
```text
my-repo/
├── package.json (or go.mod, pyproject.toml, Cargo.toml, etc.)
└── ...
```

### Steps to Reproduce
1. Clone or navigate to the repository: `cd my-repo`
2. Run command: `octo plan --json` (or `octo run`, `octo inspect`)
3. See error or unexpected output

### Expected Behavior
<!-- What command, candidate, or plan did you expect Octo to produce? -->

### Actual Output / Diagnosis
<!-- Provide the output from `octo explain` or `--json` output. -->
```text
```

### Environment
- OS: [e.g. macOS 14, Ubuntu 22.04, Windows 11]
- Octo Version: [e.g. `octo --version` or git commit]
- Installed Tools: [e.g. Node 20, Go 1.24, Python 3.12, Docker]
