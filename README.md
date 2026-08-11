# Athena

AI-powered learning assistant for people preparing for technical interviews, exams, and professional challenges.

Athena conducts adaptive study sessions, builds a personal knowledge base from your notes, identifies gaps in your understanding, and simulates realistic interviews — all personalized to your area, level, and goals.

---

## Stack

```text
Frontend         React + TypeScript
Desktop bridge   Wails v2
Core             Go (Clean/Hexagonal architecture)
Local DB         SQLite (modernc.org/sqlite — pure Go, no CGO)
Vector store     Local cosine similarity search
LLM              OpenRouter API
Auth backend     Go HTTP API (accounts + licenses)
CI/CD            GitHub Actions
Payments         Paddle
```

---

## Features (by phase)

| Phase | Feature |
|---|---|
| 0 | Repo setup, Wails scaffold, pre-commit quality gates, GitHub Actions CI/CD |
| 1 | Conversational onboarding, personalized study sessions, streaming LLM responses |
| 2 | Personal knowledge base, Markdown notes import, RAG retrieval |
| 3 | Challenge mode, gap detection, spaced repetition flashcards (SM-2) |
| 4 | Interview simulation with timer, per-answer evaluation, domain-aware feedback |
| 5 | Plan management, Paddle payments, macOS + Linux + Windows distribution |
| 6 | Voice interviews (STT + TTS) |
| 7 | Knowledge graph, architecture whiteboard, algorithm mode |

---

## Architecture

Athena uses **Hexagonal Architecture** (Clean Architecture). Dependencies point inward only — domain never depends on infrastructure or UI.

```text
┌─────────────────────────────────────────┐
│  interfaces/desktop  (Wails bindings)   │
│  ┌───────────────────────────────────┐  │
│  │  application  (use cases)         │  │
│  │  ┌─────────────────────────────┐  │  │
│  │  │  domain  (pure Go rules)    │  │  │
│  │  └─────────────────────────────┘  │  │
│  └───────────────────────────────────┘  │
│  infrastructure  (SQLite, OpenRouter)   │
└─────────────────────────────────────────┘
```

- **`domain/`** — business rules with no infrastructure dependencies; fully unit-testable
- **`application/`** — use cases that orchestrate domain and infrastructure via interfaces
- **`infrastructure/`** — adapters: SQLite repositories, OpenRouter client, vector store
- **`interfaces/desktop/`** — thin Wails bindings: validate input → call use case → return result
- **`frontend/`** — React UI that calls Wails bindings only; contains no business logic

See [ADR-001](specs/decisions/ADR-001-hexagonal-architecture.md) for the full rationale behind this choice.

---

## Getting Started

### Prerequisites

- [Go 1.22+](https://golang.org/dl/)
- [Node.js 20+](https://nodejs.org/)
- [Wails v2](https://wails.io/docs/gettingstarted/installation)
- An [OpenRouter](https://openrouter.ai/) API key

### Development

```bash
make install-hooks   # install pre-commit quality gates
make dev             # start Wails dev server with hot reload
```

### Build

```bash
make build           # produces binary in build/bin/
```

### Test

```bash
make test            # go test ./... with race detector
make lint            # golangci-lint run

cd frontend
npm run lint          # eslint .
npm run format:check  # prettier --check .
npm run test:coverage # vitest run --coverage (80% threshold)
```

### Mutation Testing

Coverage only proves a line *ran* during a test, not that the test would *notice* a bug on that line. Mutation testing checks this: it introduces small, deliberate bugs ("mutants" — e.g. `>` becomes `>=`) into the code and re-runs the tests against each one. A mutant that survives (all tests still pass) means the test asserts too little.

```bash
make mutation-go        # Gremlins, scoped to internal/domain and internal/application
make mutation-frontend  # StrykerJS, scoped to frontend/src
```

Both run as required CI jobs (`mutation-go`, `mutation-frontend`) on every PR, alongside the coverage gate — never in the pre-commit hook, since re-running tests per mutant is too slow for a tight TDD loop. See [ADR-002](specs/decisions/ADR-002-mutation-testing.md) for the full rationale, tool comparison, and threshold rollout plan.

---

## CI/CD

### Continuous Integration (`.github/workflows/ci.yml`)

Runs on pull requests targeting `main` or `develop`, and on direct pushes to `main`/`develop`. Feature branches are validated once their PR is open, avoiding duplicate runs for the same commit. It enforces the same quality gate as the local pre-commit hook (`make install-hooks`), so nothing that would fail locally can pass on CI:

1. `go test -race -coverprofile=coverage.out` — the root `main` package is excluded, since it only wires `wails.Run()` and can't run under `go test`
2. Coverage must be **≥ 80%**
3. No `//nosec` or `//nolint:gosec` suppression anywhere in `.go` files
4. `golangci-lint run`
5. `govulncheck ./...`

A failing step makes the job report a failing status on the PR. The workflow also builds the frontend (`npm ci && npm run build`), lints and format-checks it (`npm run lint`, `npm run format:check`), runs its tests with an 80% coverage gate (`npm run test:coverage`), and installs the Linux `libgtk-3-dev`/`libwebkit2gtk-4.1-dev` headers first, since the `main` package embeds `frontend/dist` and requires cgo to compile.

Three more jobs run alongside `quality-gate`:

- **`mutation-go`** — [Gremlins](https://github.com/go-gremlins/gremlins) mutation testing on `internal/domain`/`internal/application` (skipped gracefully while those packages are still empty)
- **`mutation-frontend`** — [StrykerJS](https://stryker-mutator.io/) mutation testing on `frontend/src`
- **`secret-scan`** — full git-history scan with `gitleaks`, on every push and PR
- **`commit-lint`** — validates every commit message in a PR against the Conventional Commits format (see below); runs only on `pull_request` events

Dependency updates (Go modules, npm packages, GitHub Actions) are proposed weekly by Dependabot (`.github/dependabot.yml`).

> **Checks are visible, not merge-blocking (for now).** This repository is private on the GitHub Free plan, which does not support branch protection rules or repository rulesets for private repos (`gh api repos/.../branches/main/protection` and `.../rulesets` both return `403 Upgrade to GitHub Pro or make this repository public`). This means none of the jobs above — including the pre-existing `quality-gate` — are wired as *required status checks*: a red CI run shows clearly on the PR, but the "Merge pull request" button is not disabled by GitHub. Until the repo goes public or upgrades to a paid plan, avoiding a bad merge relies on checking the PR status before merging, not on GitHub enforcement.

### Conventional Commits

Commit messages must follow `<type>(<scope>): <description>` (scope optional), with `type` one of `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`. `make install-hooks` installs a local `commit-msg` hook that rejects non-conforming messages before they're committed; the `commit-lint` CI job is the backstop for anyone who skips the hook or commits with `--no-verify`.

### Releases (`.github/workflows/release.yml`)

Triggered only by pushing a tag matching `v*` — regular pushes and PRs never create a release. It builds `wails build` on `ubuntu-latest` and `windows-latest`, then publishes both binaries to a GitHub Release for that tag.

To cut a release:

```bash
git tag v0.1.0
git push --tags
```

This creates a GitHub Release with `athena` (Linux) and `athena.exe` (Windows) attached. macOS isn't in the build matrix yet — it needs an Apple Developer certificate for code signing, planned for a later phase.

---

## Project Structure

```text
athena/
├── main.go                  # Wails entrypoint (required at project root)
├── wails.json                # Wails project config
├── internal/
│   ├── domain/               # pure business rules
│   ├── application/          # use cases
│   ├── infrastructure/       # SQLite, OpenRouter, filesystem
│   └── interfaces/
│       └── desktop/          # Wails bindings (App struct)
├── frontend/                 # React + TypeScript (Vite)
├── build/                    # Wails packaging assets (icons, platform manifests)
├── server/                   # Auth + license HTTP server
├── specs/                    # Product and implementation specs
│   ├── Athena.md              # Product spec
│   ├── Planning.md            # Implementation plan (authoritative)
│   ├── phases/                # Per-phase detailed specs
│   └── decisions/             # Architecture Decision Records (ADRs)
├── go.mod
└── Makefile
```

> Note: `main.go` lives at the project root, not under `cmd/`, because Wails v2 requires the main package (and its `//go:embed all:frontend/dist` directive) to sit next to `wails.json` and `frontend/`. The real Wails bindings still live in `internal/interfaces/desktop/`, kept as thin as the hexagonal architecture rules require.

---

## Local Data

All user data is stored on-device:

```text
~/.athena/
├── config.yaml        # OpenRouter key, model preferences
├── profile.json       # User profile (name, area, level, goals)
├── session.json       # Auth token cache
├── athena.db          # SQLite (sessions, knowledge, flashcards, progress)
├── vectors/           # Local embeddings
└── logs/              # Structured execution logs
```

The auth server only manages accounts and licenses. Your notes and knowledge base never leave your machine.

---

## Docs

- [Product Spec](specs/Athena.md)
- [Implementation Plan](specs/Planning.md)
- [Phase Specs](specs/phases/README.md)
- [ADR-001 — Hexagonal Architecture](specs/decisions/ADR-001-hexagonal-architecture.md)
- [ADR-002 — Mutation Testing](specs/decisions/ADR-002-mutation-testing.md)
