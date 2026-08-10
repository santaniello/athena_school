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
```

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
