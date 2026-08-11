# AGENTS.md — Athena Development Rules

## Mandatory cycle per spec

For each spec, the cycle MUST be followed in this exact order:

1. **Write the test first (TDD — Red phase)**
    - The test must fail before any implementation
    - Never write production code before the test

2. **Implement the minimum necessary (Green phase)**
    - Write only the code needed to make the test pass
    - No over-engineering, no anticipating future requirements

3. **Refactor (Refactor phase)**
    - Improve quality without changing behavior
    - Tests must continue to pass after any refactoring

4. **Commit**
    - Only after: tests passing + coverage ≥ threshold + no surviving mutants in changed
      `internal/domain`/`internal/application` code + lint ok + vulnerability ok

## Development methodology (Extreme Programming)

- **One spec at a time** — never work on multiple behaviors simultaneously
- **Do not skip steps** — do not implement before having a test; do not refactor before going green
- **Smallest possible implementation** — if the test passes with 5 lines, do not write 20
- **No big design upfront** — do not create abstractions for hypothetical future needs
- **Test is documentation** — the test name must describe the expected behavior

## Language

- All code must be written in **English**: variable names, functions, types, constants, comments, internal error messages, and file names
- User-facing UI text is in **Portuguese (BR)**

## Architecture rules

The solution uses **Hexagonal Architecture** (also known as Clean Architecture). See `specs/decisions/ADR-001-hexagonal-architecture.md` for the full rationale.

Layers and dependency rule — dependencies point inward only:

```
interfaces/desktop  →  application  →  domain  ←  infrastructure
```

- Business rules live exclusively in `internal/domain/` — never in `internal/infrastructure/` or `interfaces/desktop/`
- Use cases live in `internal/application/` — they orchestrate domain and infrastructure
- Wails bindings in `internal/interfaces/desktop/` are thin adapters: validate input, call use case, return result
- `internal/infrastructure/` depends on `internal/domain/` interfaces; never the reverse
- Frontend (React) calls Wails bindings only — it never contains business logic

## Go code rules

- Small functions with single responsibility
- Errors always handled explicitly (never `_` for errors)
- Descriptive English names
- Tests in the same package with `_test.go` suffix
- Test coverage: every behavior documented in a spec must have a test
- Minimum coverage threshold: **80%** — CI will reject PRs below this value
- Mutation testing: changes to `internal/domain/` or `internal/application/` must be
  verified with `make mutation-go` before commit — a surviving mutant means the test
  asserts too little, not that a new test is missing; strengthen the existing
  assertion instead of adding a redundant one
- Tests must follow the **GivenWhenThen** pattern: arrange the initial state (Given), execute the action (When), assert the outcome (Then)
- Use **testify** for assertions (`assert` / `require`)
- Use **Mockery** to generate mocks from interfaces
- Mock parameters must be typed explicitly — avoid `mock.AnythingOfType` and `mock.Anything` unless there is no viable alternative; prefer exact values or typed matchers

## Security rules

- **No command injection** — never concatenate user input into shell strings; always pass arguments as separate values to `exec.Command`
- **No shell wrapping** — never invoke subprocesses via `exec.Command("sh", "-c", userInput)`; call binaries directly with explicit args
- **Path traversal prevention** — when building paths inside `~/.athena/`, validate that the resulting path stays within the expected directory
- **No hardcoded secrets** — no API keys, tokens, or credentials anywhere in the code; all secrets come from `~/.athena/config.yaml` or environment variables
- **File permissions** — files written to `~/.athena/` must use permission `0600` (owner read/write only)
- **No suppression of findings** — never use `#nosec`, `//nolint:gosec`, or any mechanism to silence security or vulnerability warnings; fix the root cause instead

## Versioning

- The project follows **Semantic Versioning** as defined at [semver.org](https://semver.org/)
- Format: `MAJOR.MINOR.PATCH`
    - `MAJOR` — breaking change to user data format or auth API contract
    - `MINOR` — new feature added in a backward-compatible manner
    - `PATCH` — backward-compatible bug fix
- Version is bumped manually when releasing — never automatically on every commit
- Version is injected at build time via `-ldflags "-X main.version=..."`

## Commits

- All commit messages must follow the **Conventional Commits** standard and be written in **English**
- Format: `<type>(<scope>): <description>` — e.g. `feat(onboarding): add user profile persistence`
- Common types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`
- Description must be lowercase, imperative mood, and without a trailing period

## Documentation

- After every change, evaluate whether `README.md` needs to be updated — update it if the change affects setup, features, or project structure
- `specs/Planning.md` is the authoritative source of truth for implementation decisions — consult it before starting any spec
- `CHANGELOG.md` follows the [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) standard — every change must add an entry to the `[Unreleased]` section under the appropriate category (`Added`, `Changed`, `Fixed`, `Removed`, `Deprecated`, `Security`)
- Every release tag must correspond to a versioned section in `CHANGELOG.md` — never tag without promoting `[Unreleased]` first

## What the AI must NOT do

- Implement code before there is a test for it
- Create helpers or abstractions not required by the current spec
- Anticipate future specs in the current code
- Commit without the quality gate passing
- Skip the refactoring phase
- Put business logic in Wails bindings or React components
- Add CGO dependencies (SQLite driver must remain pure Go)
