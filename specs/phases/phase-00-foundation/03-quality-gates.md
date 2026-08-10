# Phase 0.3 — Quality Gates

## Goal

Every commit passes automated quality checks before it can be pushed.

## Tasks

- [ ] `.githooks/pre-commit` script running:
  - `go test -coverprofile=coverage.out ./...` (single pass for tests + coverage)
  - Coverage check — fail if below 80%
  - Suppression check — fail if any `.go` file contains `//nolint:.*gosec` or `//nosec`
  - `golangci-lint run` (falls back to `go vet` if not installed)
  - `govulncheck ./...`
- [ ] `make install-hooks` wires the hook path via `git config core.hooksPath .githooks`
- [ ] `.golangci.yml` with a baseline linter set (errcheck, govet, staticcheck, unused)

## Acceptance Criteria

- `make install-hooks` exits 0
- Committing with a failing test aborts the commit with a clear error message
- `golangci-lint run` exits 0 on the initial codebase
