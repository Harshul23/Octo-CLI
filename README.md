<p align="center">
  <img src="assets/octo-banner.png" alt="Octo CLI — Clone it. Run it." width="100%" />
</p>

<p align="center">
  <strong>Make “how do I run this?” disappear.</strong><br>
  Zero-configuration execution intelligence for human developers and autonomous AI agents.
</p>

<p align="center">
  <a href="https://github.com/Octo-sh/Octo-CLI/releases"><img src="https://img.shields.io/github/v/release/Octo-sh/Octo-CLI?style=flat-square&logo=github&logoColor=white&label=Release&labelColor=18181b&color=2563eb" alt="Release" /></a>
  <a href="https://github.com/Octo-sh/homebrew-tap"><img src="https://img.shields.io/badge/Brew-Octo--sh%2Ftap-27272a?style=flat-square&logo=homebrew&logoColor=f59e0b&labelColor=18181b" alt="Homebrew" /></a>
  <a href="cmd/octo/mcp.go"><img src="https://img.shields.io/badge/MCP-Protocol%20Ready-7c3aed?style=flat-square&logo=anthropic&logoColor=white&labelColor=18181b" alt="MCP Ready" /></a>
  <a href="https://github.com/Octo-sh/Octo-CLI/releases"><img src="https://img.shields.io/badge/macOS-27272a?style=flat-square&logo=apple&logoColor=white" alt="macOS" /></a>
  <a href="https://github.com/Octo-sh/Octo-CLI/releases"><img src="https://img.shields.io/badge/Linux-27272a?style=flat-square&logo=linux&logoColor=white" alt="Linux" /></a>
  <a href="https://github.com/Octo-sh/Octo-CLI/releases"><img src="https://img.shields.io/badge/Windows-27272a?style=flat-square&logo=windows&logoColor=white" alt="Windows" /></a>
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/License-MIT-10b981?style=flat-square&labelColor=18181b" alt="License" /></a>
</p>

<p align="center">
  <a href="#quick-install">Quick Install</a>
  <span>&nbsp;&nbsp;•&nbsp;&nbsp;</span>
  <a href="#30-second-quickstart">30s Quickstart</a>
  <span>&nbsp;&nbsp;•&nbsp;&nbsp;</span>
  <a href="#ai-agent-companion-octo-mcp">AI Agent Companion</a>
  <span>&nbsp;&nbsp;•&nbsp;&nbsp;</span>
  <a href="#why-octo">Why Octo?</a>
  <span>&nbsp;&nbsp;•&nbsp;&nbsp;</span>
  <a href="#supported-ecosystems--capabilities">Ecosystems</a>
  <span>&nbsp;&nbsp;•&nbsp;&nbsp;</span>
  <a href="#documentation--architecture">Documentation</a>
</p>

---

## Quick Install

Choose your preferred installation method:

### 1. Homebrew (macOS & Linux)
Install prebuilt binaries without requiring Go:
```bash
brew tap Octo-sh/tap
brew install octo
```

### 2. Curl Script (macOS & Linux)
Automatic OS and architecture detection, integrity verification via SHA-256 checksums, and installation to `/usr/local/bin` (falls back to `~/.local/bin`):
```bash
curl -fsSL https://raw.githubusercontent.com/Octo-sh/Octo-CLI/main/install.sh | sh
```

### 3. PowerShell One-Liner (Windows)
Automatic architecture detection, SHA-256 verification, installation to `%LOCALAPPDATA%\Programs\octo`, and PATH registration:
```powershell
irm https://raw.githubusercontent.com/Octo-sh/Octo-CLI/main/install.ps1 | iex
```

### 4. Scoop (Windows)
Install via the Octo bucket or directly from the manifest URL:
```powershell
# Via bucket
scoop bucket add octo https://github.com/Octo-sh/Octo-CLI
scoop install octo

# Or direct manifest URL
scoop install https://raw.githubusercontent.com/Octo-sh/Octo-CLI/main/bucket/octo.json
```

### 5. Go Install (Any Platform with Go 1.25.6+)
Build and install from source:
```bash
go install github.com/Octo-sh/Octo-CLI/cmd/octo@latest
```

### 6. Manual Release Binaries
Precompiled standalone archives and `checksums.txt` are available on the [GitHub Releases page](https://github.com/Octo-sh/Octo-CLI/releases/latest) for macOS (Apple Silicon & Intel), Linux (x86_64 & ARM64), and Windows (x64; ARM64 via x64 emulation, native build coming next release).


---

## 30-Second Quickstart

```bash
cd your-project

# 1. Inspect: Discover repository facts, language, framework & package manager
octo inspect   # or: octo ins

# 2. Preview: Inspect port allocations, dependencies & machine changes before running
octo preview   # or: octo prev

# 3. Run: Execute in dependency order, verify runtime health, and teardown cleanly on Ctrl+C
octo run       # or: octo r
```

### Quick Command & Flag Shortcuts

| Quick Command | Full Command | Description |
| :--- | :--- | :--- |
| `octo r` / `octo start` | `octo run` | Analyze, plan, run, and verify the stack |
| `octo ins` / `octo info` | `octo inspect` | Inspect detected languages, frameworks, and signals |
| `octo p` | `octo plan` | Show deterministic execution plan |
| `octo prev` / `octo dry-run` | `octo preview` | Preview machine mutations, ports, and processes |
| `octo v` / `octo check` | `octo verify` | Verify runtime health checks & `.octo.lock` integrity |
| `octo g` / `octo topo` | `octo graph` | Render dependency topology graph |
| `octo e` | `octo env` | Inspect environment variable status |
| `octo e tpl` | `octo env template` | Generate safe `.env` template |
| `octo bench` / `octo eval` | `octo benchmark` | Benchmark against raw shell baselines |

**Everyday Flag Shortcuts:**
- `-j`: Universal shorthand for `--json` across all commands (`octo r -j`, `octo ins -j`, `octo p -j`, etc.).
- `-k` / `--skip-env`: Shorthand for `--skip-env-check` (`octo r -k`).
- `-T`: Shorthand for `--no-tui` (suppress dashboard for standard terminal logs).
- `-N`: Shorthand for `--no-port-shift` (disable automatic port conflict reallocation).
- `-i`: Interactive component selection (`octo r -i`).
- `-s`: Ephemeral container environment (`octo r -s`) or archetype suite (`octo bench -s`). (Note: project env vars pass through).


---

## AI Agent Companion (`octo mcp`)

When AI coding agents (Claude Code, Cursor, Windsurf, Cline) execute code directly through raw bash commands, they can burn a large share of their context tokens trial-and-erroring missing packages, colliding ports, and orphaning background processes.

Octo includes a native **JSON-RPC 2.0 Model Context Protocol (MCP)** server over stdio. Connect Octo to your favorite AI agent to give it deterministic, zero-hallucination execution capabilities:

### Add to Cursor (`.cursor/mcp.json`):
```json
{
  "mcpServers": {
    "octo": {
      "command": "octo",
      "args": ["mcp"]
    }
  }
}
```

### Add to Claude Code:
```bash
claude mcp add octo -- octo mcp
```

### Available Agent Tools:
- `octo_inspect`: Analyzes the repository to detect language, framework, runtime version, and package manager.
- `octo_plan`: Generates the deterministic dependency-ordered execution plan.
- `octo_preview`: Previews machine changes, required packages, and network port reservations.
- `octo_run_and_verify`: Autonomously executes the stack and verifies runtime health without raw shell guessing.
- `octo_verify`: Validates live services and lockfile fingerprint integrity.

---

## Why Octo?

| Feature | Octo CLI | Raw Shell Scripts | Docker Compose Alone | LLM Bash Agents |
| :--- | :---: | :---: | :---: | :---: |
| **Zero Configuration Required** | ✓ | — | — (requires compose file) | — |
| **Automatic Port Conflict Shifting** | ✓ | — (EADDRINUSE error) | — | — |
| **Backing Service Coordination** | ✓ (Starts DB before app) | — | Partial | — (hangs or fails) |
| **Graceful Teardown on Ctrl+C** | ✓ (Drains process tree) | — (Leaves orphans) | Partial | — (Leaves zombie PIDs) |
| **Zero-Hallucination AI Companion (estimated)** | ✓ (via MCP) | N/A | N/A | — (hallucinates commands) |
| **Verified Strategy Caching** | ✓ (`.octo.lock`) | — | — | — |

---

## Supported Ecosystems & Capabilities

<p align="center">
  <img src="https://skillicons.dev/icons?i=nodejs,nextjs,express,go,python,fastapi,rust,docker&theme=dark" alt="Supported Technologies" />
</p>

- **Languages and Frameworks**: Native evidence detectors for Node.js (Next.js, Express, Vite), Go (Gin, standard library), Python (FastAPI, Django, Flask), Rust (Cargo), Ruby, Java.
- **Port Intelligence**: Detects listening port collisions and automatically rebinds components to free ports via dynamic environment injection (`PORT`).
- **Infrastructure Coordination**: Discovers and boots backing services (PostgreSQL, Redis, MySQL) via Docker Compose before launching application processes.
- **Safe Environment Scoping**: Resolves local environment precedence without ever logging or persisting secrets to planning models or lockfiles.
- **Clean Process Lifecycle**: Traps `SIGINT`/`SIGTERM` to gracefully drain active children and kill zombie daemon PIDs.
- **Strategy Fingerprinting (`.octo.lock`)**: Caches proven startup strategies and only invalidates when code or dependency manifests change.

---

## Documentation & Architecture

Deep-dive documentation and formal specifications live in dedicated guides:

- **[Architecture Deep-Dive](Architecture.md)** — Detailed specification of the canonical execution pipeline, bounded decision engine, topology graph, and runtime adapters.
- **[Vision & Core Principles](docs/VISION.md)** — The foundational philosophy: *Evidence before inference, candidates before decisions*.
- **[Specification Contract](spec/v1/specification.md)** — Formal behavioral contract, candidate schemas, and JSON models.
- **[Benchmark Results](docs/BENCHMARK.md)** — Detection speed and execution startup benchmarks across archetypes.
- **[Project Roadmap](docs/ROADMAP.md)** — Phased milestones, plugin ecosystems, and future adapter targets.
- **[Community & Mentorship](docs/COMMUNITY_AND_MENTORSHIP.md)** — Guidelines for community engagement and contributor onboarding.

---

## Monorepo Support & Target Selection

Octo supports multi-target repositories (Node workspaces, `pnpm`, Go `go.work`, Cargo workspaces, and heuristic multi-service directory layouts) and root Docker Compose infrastructure services:

### Selection Examples
```bash
# Interactive selection (TUI with scrolling and confirmation screen)
octo run

# Launch all runnable targets simultaneously
octo run --all

# Launch a specific application alone
octo run --only component:web

# Launch an API and automatically include its explicitly declared database dependency
octo run --only component:api

# Launch multiple specific targets
octo run --only component:web --only service:postgres
```

### Supported Scope & Important Boundaries
- **Supported Discovery**: Declared Node workspaces (`apps/*`, `packages/*`), `pnpm-workspace.yaml`, Go `go.work`, and Cargo workspaces. Automatic fallback directory scan for sibling components when no root manifest exists.
- **Compose Services**: Root `compose.yaml` / `docker-compose.yml` services are mapped into the topology graph. Static network references in component `.env` files (e.g. `DATABASE_URL=postgres://...`) or explicit declarations in `.octo.yaml` are automatically resolved.
- **Library Safety**: Shared workspace libraries without runnable entry points (e.g., packages with only build/typecheck scripts) are built during `PhaseSetup` when depended on, but are *never launched as daemons*.
- **Resource Discipline**: On `Ctrl+C`, Octo stops owned processes and containers, leaving created database volumes untouched.
- **Boundaries**: *Octo does not claim all monorepos or databases work out of the box*. Proprietary build graphs (e.g. custom Bazel/Buck rules, complex Turborepo/Nx pipeline hooks, Yarn Berry PnP) or distributed multi-cluster databases require explicit manifest or `.octo.yaml` configuration.

### Non-Interactive & AI Agent Rules
In non-interactive environments (`--json`, CI runners without a TTY, or MCP calls from Claude Code, Cursor, Windsurf):
- Octo **never prompts or hangs**.
- When multiple runnable targets exist and neither `--only` nor `--all` is specified, Octo returns a non-zero exit code with a structured error listing available targets and flags.

---

## Testing & Verification

### CI Commands
Tests are divided into offline unit/smoke suites and opt-in host integration suites:

```bash
# 1. Offline Test Suite (Runs on all CI platforms & offline macOS/Linux without Docker)
go test -count=1 ./...
go test -race ./...

# 2. Opt-in Real Node/Docker Integration Suite (Requires local Docker daemon and Node runtime)
OCTO_INTEGRATION_TESTS=1 go test -v -count=1 ./internal/intelligence -run TestSmokeFixtureRealIntegrationOptIn
```

### Manual Mac Smoke-Test Checklist
Follow this checklist to manually verify a monorepo workspace on macOS:

- [ ] **1. Build Local Binary**:
  ```bash
  go build -o bin/octo ./cmd/octo
  ```
- [ ] **2. Inspect Targets & Evidence**:
  ```bash
  ./bin/octo inspect path/to/monorepo
  ```
  *Verify*: Lists runnable components, non-runnable libraries, and backing Compose services.
- [ ] **3. Non-Interactive Guard**:
  ```bash
  ./bin/octo run path/to/monorepo --json
  ```
  *Verify*: Exits immediately without prompting or hanging; returns JSON error listing targets.
- [ ] **4. Select Web Alone**:
  ```bash
  ./bin/octo run path/to/monorepo --only component:web
  ```
  *Verify*: Starts `web` alone on port 3000 without starting independent sibling components or services.
- [ ] **5. Select API with Database Dependency**:
  ```bash
  ./bin/octo run path/to/monorepo --only component:api
  ```
  *Verify*: Automatically includes declared dependencies (e.g., `service:postgres`) before launching `api`.
- [ ] **6. Select All Runnable Targets**:
  ```bash
  ./bin/octo run path/to/monorepo --all
  ```
  *Verify*: Starts all runnable components and services; builds shared libraries in setup phase; never starts libraries.
- [ ] **7. Verified Startup & Ctrl+C Persistence**:
  *Verify*: Stop with `Ctrl+C` after startup verification. Child processes and Compose services stop cleanly, database volumes are preserved, and `.octo.lock` is saved on disk.
- [ ] **8. Re-run Strategy Reuse**:
  ```bash
  ./bin/octo run path/to/monorepo --all
  ```
  *Verify*: Reuses matching strategies from `.octo.lock` without re-evaluating from scratch.
- [ ] **9. Cache Invalidation**:
  *Verify*: Edit a component source file or root `package.json`; re-running detects the changed fingerprint and invalidates only affected strategies.
- [ ] **10. Watch Mode Selection Retention**:
  ```bash
  ./bin/octo run path/to/monorepo --watch --only component:web
  ```
  *Verify*: Retains `--only component:web` selection across file-change reloads.
- [ ] **11. Interactive Picker & Scrolling**:
  Run `./bin/octo run path/to/monorepo` in a terminal window:
  - Arrow keys / `j` / `k` navigate items.
  - `Space` toggles individual selections; `a` toggles all.
  - Resize terminal height to test scrolling boundaries (`▲`, `▼`).
  - Press `Enter` to view the review & confirmation screen (displays requested targets and forced dependencies with reasons).
  - Press `Ctrl+C` to abort cleanly before anything is launched.


---

## Contributing

Contributions are welcome! To add a new detector, candidate provider, or runtime adapter, please review [CONTRIBUTING.md](CONTRIBUTING.md) and [Architecture.md](Architecture.md) before opening a PR.

---

## License

MIT License — see [LICENSE.md](LICENSE.md).
