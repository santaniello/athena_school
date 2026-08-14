# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Repository scaffold: go.mod, Makefile, directory structure, .gitignore
- Wails v2 + React + TypeScript desktop shell (`wails dev`/`wails build`)
- CI workflow (`.github/workflows/ci.yml`): tests with coverage, 80% coverage gate, security-suppression check, `golangci-lint`, `govulncheck` on every push and PR to `main`/`develop`
- Release workflow (`.github/workflows/release.yml`): cross-platform `wails build` (Windows, Linux) and GitHub Release publishing on `v*` tags
- Frontend quality gate: ESLint, Prettier, and Vitest + React Testing Library with an 80% coverage threshold, wired into `frontend/package.json` scripts and the `quality-gate` CI job
- `secret-scan` CI job: full-history `gitleaks` scan on every push and PR
- `commit-lint` CI job: validates every commit message on a PR against the Conventional Commits format
- `.githooks/commit-msg` hook: local Conventional Commits validation, installed via `make install-hooks`
- `.github/dependabot.yml`: weekly automated update PRs for Go modules, npm packages, and GitHub Actions
- Expanded `.golangci.yml`: added `revive`, `gocritic`, `bodyclose`, `sqlclosecheck`, `misspell` linters and a `formatters` block (`gofmt`, `goimports`)
- `frontend/scripts/postinstall.mjs`: writes a throwaway `go.mod` into `frontend/node_modules` after every install, firewalling it from the root Go module so stray `.go` files shipped inside npm packages (e.g. `flatted`) don't break `go build`/`go vet`/`go test`/`golangci-lint` from the repo root
- Mutation testing: [Gremlins](https://github.com/go-gremlins/gremlins) for `internal/domain`/`internal/application` (`.gremlins.yaml`, `make mutation-go`) and [StrykerJS](https://stryker-mutator.io/) for `frontend/src` (`frontend/stryker.config.mjs`, `make mutation-frontend`/`npm run mutation`), wired as required `mutation-go`/`mutation-frontend` CI jobs; see [ADR-002](specs/decisions/ADR-002-mutation-testing.md)
- `AGENTS.md`: TDD commit gate and Go code rules now require no surviving mutants in changed `internal/domain`/`internal/application` code before commit
- Mocking strategy: [Mockery](https://vektra.github.io/mockery/) wired up for the Go backend (`.mockery.yaml`, `make mock`), generating mocks into a sibling `mocks` subpackage excluded from the coverage gate (`ci.yml`, `.githooks/pre-commit`) and from `make mutation-go` (`.gremlins.yaml`); frontend keeps Vitest's native `vi.mock()`/`vi.fn()` with no added library; see [ADR-003](specs/decisions/ADR-003-mocking-strategy.md)
- Local auth core (`internal/domain/auth`, `internal/application/auth`): `Account` model, `AccountRepository`/`SessionStore` ports, and `Register`/`Login`/`ResetLocalAccount` use cases, backed by a SQLite `AccountRepository` (`internal/infrastructure/sqlite`, pure-Go `modernc.org/sqlite` driver) and a JSON session marker at `~/.athena/session.json` (`internal/infrastructure/session`); see [01-auth-backend.md](specs/phases/phase-01-desktop-mvp/01-auth-backend.md)
- Desktop auth UI (`internal/interfaces/desktop`, `frontend/src/screens`): `Login`, `Register`, `ResetLocalAccount` and `HasLocalSession` Wails bindings wired to the local auth service, plus Login/Register/Reset-local-account React screens and a placeholder main screen so a new user lands directly in the app and an existing user (or an existing local session) skips straight past the login screen; see [02-auth-ui.md](specs/phases/phase-01-desktop-mvp/02-auth-ui.md)

### Changed
- Moved the Wails entrypoint from `cmd/athena/` to root `main.go`: Wails v2 does not support a main package outside the project root