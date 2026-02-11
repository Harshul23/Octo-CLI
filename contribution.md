# Contributing

Thanks for your interest in contributing to **Octo CLI**. We're happy to have you here.

Please take a moment to review this document before submitting your first pull request. We also strongly recommend that you check for open issues and pull requests to see if someone else is working on something similar.

## Ways to Contribute

You can contribute in multiple ways:

* Reporting bugs
* Suggesting features
* Improving detection logic
* Improving documentation
* Writing tests
* Fixing runtime or framework support
* Improving performance

Even small improvements matter.


## Development Setup

### 1. Fork the repository

Click **Fork** on GitHub and clone your fork:

```bash
git clone https://github.com/<your-username>/octo-cli.git
cd octo-cli
```

### 2. Just run it !

No need to worry for Go installation Octo will handle it smoothly:

```bash
octo run
```

Or after building:

```bash
go build -o octo ./cmd
./octo init
```

## Branching Strategy

Use clear branch names:

```
feat/add-python-detection

fix/node-runtime-version-bug

docs/update-readme

refactor/detection-engine
```

Format:

```
<type>/<short-description>
```

**Types:**

* feat
* fix
* docs
* refactor
* test
* chore

## Contribution Areas

### 1. Language Detection

Improve detection logic in:

* Runtime detection
* Version inference
* Framework recognition
* Package manager detection

Keep detection:

* Fast
* Deterministic
* Non-invasive

No guessing. Detection should be explainable.

---

### 2. Framework Support

If adding support for a new framework:

* Add detection rules
* Add proper run command generation
* Add test cases
* Update README table

---

### 3. Runtime Support

If improving Docker / Nix / Shell execution:

* Keep configs minimal
* Avoid hardcoded assumptions
* Ensure reproducibility

## Testing

Before submitting a PR:

```bash
go test ./...
```

If adding detection logic:

* Add unit tests
* Add example project samples if needed

PRs without tests may be delayed.

## Reporting Issues

When opening an issue, include:

* OS (macOS/Linux/Windows)
* Language/framework
* Sample project structure
* Expected behavior
* Actual behavior
* Logs (if any)

Clear issues get fixed faster.

## Commit Message Guidelines

Use clear, meaningful commit messages:

```
feat: add support for FastAPI detection
fix: resolve node version parsing bug
docs: update installation instructions
refactor: simplify runtime resolver logic
```

Keep it short. Be specific.

## Pull Request Guidelines

Before opening a PR:

* Make sure tests pass
* Rebase with latest `main`
* Keep PR focused on one change
* Update docs if needed

PR template should include:

* What changed
* Why it changed
* How to test
* Screenshots (if relevant)

## Code Standards

* Follow Go idioms
* Keep functions small and readable
* Avoid unnecessary abstractions
* Favor clarity over cleverness
* No breaking changes without discussion

---

## Adding New Language Support (Checklist)

If adding a new language:

* [ ] Add language detection logic
* [ ] Add version detection
* [ ] Add run command template
* [ ] Add tests
* [ ] Update README supported languages table
* [ ] Ensure `octo init` generates valid `.octo.yaml`

## Design Philosophy

Octo CLI follows:

* Zero configuration first
* Smart defaults
* Predictable behavior
* Local-first execution
* Developer productivity > complexity

If your change adds complexity, justify it clearly.

---

## Code of Conduct

Be respectful.

Constructive criticism only.
No ego-driven discussions.
Focus on improving the tool.

---

## First Contribution?

Good starting areas:

* Improve error messages
* Add tests
* Fix documentation typos
* Improve CLI help descriptions
* Add support for a small framework

---

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

If you're building tools for developers, you're building leverage.

Let's make local deployment effortless.
