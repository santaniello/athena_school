# Phase 0.1 — Repository Setup

## Goal

Initialize the repository with the directory structure, tooling config, and module definition needed for all subsequent phases.

## Tasks

- [ ] `.gitignore` covering Go, Node, Wails, and OS files
- [ ] `CLAUDE.md` with development rules (TDD, commit conventions, Go standards)
- [ ] `go.mod` with module name `github.com/<user>/athena`
- [ ] Directory structure:

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

- [ ] `Makefile` with targets:

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