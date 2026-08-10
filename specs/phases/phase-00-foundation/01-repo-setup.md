# Phase 0.1 — Repository Setup

## Goal

Initialize the repository with the directory structure, tooling config, and module definition needed for all subsequent phases.

## Tasks

- [x] `.gitignore` covering Go, Node, Wails, and OS files
- [x] `CLAUDE.md` with development rules (TDD, commit conventions, Go standards)
  - Note: the actual rules live in `AGENTS.md` at the repo root. `CLAUDE.md`
    and `GEMINI.md` are 14-byte stubs containing `See AGENTS.md`, so
    AGENTS.md is the single source of truth for all agents.
- [x] `go.mod` with module name `github.com/<user>/athena` (`github.com/santaniello/athena`)
- [x] Directory structure:

```text
athena/
├── cmd/
│   └── athena/              # Wails entrypoint
├── internal/
│   ├── domain/              # pure business rules
│   ├── application/         # use cases
│   ├── infrastructure/      # SQLite, OpenRouter, filesystem
│   └── interfaces/
│       └── desktop/         # Wails bindings
├── frontend/                # React + TypeScript (Wails-generated)
├── go.mod
├── go.sum
└── Makefile
```

  - Note: the actual layout has the Wails entrypoint as `main.go` at the
    repo root instead of `cmd/athena/main.go` (Wails' default scaffold
    layout). `internal/{domain,application,infrastructure}` exist as empty
    directories (`.gitkeep`) awaiting later phases; `internal/interfaces/desktop`
    already has `app.go` + `app_test.go`.

- [x] `Makefile` with targets:

```makefile
build:         wails build
dev:           wails dev
test:          go test ./...
lint:          golangci-lint run
install-hooks: git config core.hooksPath .githooks && chmod +x .githooks/pre-commit
```

## Acceptance Criteria

- `go build ./...` succeeds on a clean clone
- `make install-hooks` exits 0 and creates the pre-commit hook
- Directory structure matches the spec above