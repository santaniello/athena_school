# ADR-002 — Mutation Testing

**Status:** Accepted
**Date:** 2026-08-11

---

## Context

The project enforces an 80% line/branch coverage gate (`ci.yml`, `.githooks/pre-commit`) for both the Go backend and the React frontend. Coverage measures whether a line *executed* during a test run — it does not measure whether the test would have *noticed* a bug on that line. Following the GivenWhenThen pattern mandated by `AGENTS.md`, it is easy to write a `Then` step that only asserts `err == nil` without checking the returned value or resulting state; such a test passes 100% coverage while asserting almost nothing.

Athena's domain is non-trivial (SM-2 scheduling, gap detection, evaluation scoring, knowledge promotion — see `specs/Planning.md`) and lives in `internal/domain/`/`internal/application/` per [ADR-001](ADR-001-hexagonal-architecture.md). Weak assertions in that layer are exactly the kind of defect coverage cannot catch, and TDD (`AGENTS.md`'s mandatory red-green-refactor cycle) is only as good as the assertions written in the green phase.

The project is early-stage: `internal/domain`, `internal/application`, and `internal/infrastructure` are still empty scaffolding, and the frontend has a single component (`App.tsx`) beyond the Wails/React/TS template. This is the cheapest possible point to introduce mutation testing and its CI gate — before any test-quality debt accumulates — rather than retrofitting it once dozens of use cases exist.

---

## Decision

**Adopt mutation testing as a mandatory, CI-enforced quality gate**, alongside the existing coverage/lint/vulnerability gates:

- **Backend (Go):** [Gremlins](https://github.com/go-gremlins/gremlins) (`go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`), scoped to `internal/domain/...` and `internal/application/...`.
- **Frontend (React/TS):** [StrykerJS](https://stryker-mutator.io/) (`@stryker-mutator/core` + `@stryker-mutator/vitest-runner`), scoped to `frontend/src/**/*.{ts,tsx}` excluding entrypoints and test files.

Both run as required, separate jobs in `.github/workflows/ci.yml` (`mutation-go`, `mutation-frontend`) from the first PR that introduces them, gated by conservative starting thresholds (Gremlins `threshold-efficacy: 60` / `threshold-mcover: 80`; Stryker `break: 60`). Neither tool runs in `.githooks/pre-commit`.

`AGENTS.md` is updated so the agent (human or AI) treats a surviving mutant in `internal/domain`/`internal/application` as part of the TDD commit gate, on par with coverage — see `AGENTS.md`'s "Mandatory cycle per spec" and "Go code rules" sections.

---

## Rationale

**1. Gremlins over go-mutesting.**
Gremlins is actively maintained (v0.6.0, Dec 2025) and coverage-guided: it only mutates lines a test actually executes, using the existing `go test` coverage profile, so it skips untested code instead of wasting a test run on it. It also ships the primitives this rollout needs natively — `--diff` for incremental runs, `--threshold-efficacy`/`--threshold-mcover` for CI gating, structured JSON output. go-mutesting (the avito-tech fork of zimmski/go-mutesting) is effectively dormant, has no coverage-guided execution (it reruns the full suite per mutant), and has no built-in threshold or diff support — any CI gating would have to be hand-rolled.

**2. StrykerJS is the only viable choice for the frontend.**
There is no mature alternative mutation-testing framework for JS/TS. `@stryker-mutator/vitest-runner` (9.6.1) has a changelog entry specifically fixing hitcount/coverage handling for Vitest 4.1.x — the exact minor version this repo pins (`vitest@4.1.10`) — which de-risks adopting it now rather than waiting.

**3. Scope: domain/application only, not interfaces/desktop or (yet) infrastructure.**
`internal/interfaces/desktop` is, by the hexagonal architecture rule, a thin Wails binding with no business logic — mutating it mostly produces noise (mutants on trivial passthrough code) with no signal about domain correctness. `internal/infrastructure` will be added to scope once real adapters exist (SQLite repositories, OpenRouter client), since error-translation and mapping logic there is a plausible source of real bugs. Both `domain` and `application` are pure Go (the project's "no CGO" rule), so the `mutation-go` CI job does not need the `libgtk-3-dev`/`libwebkit2gtk-4.1-dev` Wails build dependencies the main `quality-gate` job requires.

**4. Enforced from day one, not phased in as informational-only.**
Given how little code exists today, the cost of getting this wrong is minimal and the cost of retrofitting a gate later (once surviving mutants have accumulated across many use cases) is much higher. Starting thresholds (`efficacy 60`, `mcover 80` for Go; `break 60` for the frontend) are deliberately conservative so they don't block early, small PRs, but they already force any new domain/application logic to carry assertions strong enough to kill basic mutants. Thresholds are expected to be raised over time (e.g. quarterly) as the domain grows — a config change in `.gremlins.yaml`/`stryker.config.mjs`, not a process change.

**5. Never in `.githooks/pre-commit`.**
The pre-commit hook already re-runs the full Go quality gate (tests, coverage, lint, `govulncheck`) on every commit — accepted because it stays fast. Mutation testing reruns tests once per mutant and is an order of magnitude slower, even with coverage-guided execution; adding it there would break the tight TDD red-green-refactor loop `AGENTS.md` requires. This is called out explicitly so it is not "discovered" later as a mistake by analogy with the other pre-commit checks.

---

## Rejected Alternatives

**go-mutesting (Go):** rejected — dormant maintenance, no coverage-guided execution (slow), no native CI threshold/diff support.

**Manual/no tooling, relying on code review alone:** rejected — weak assertions are exactly the kind of defect that's easy to miss in review (the test *looks* complete; only running mutants reveals it asserts too little). Not automatable or CI-gateable.

**Phased/informational-only rollout (gate added later, once a baseline exists):** considered and rejected per explicit product decision — the codebase is cheapest to gate now, before test-quality debt accumulates across the domain layer.

---

## Consequences

**Positive:**
- Weak assertions in business-rule tests (the layer that matters most per ADR-001) are caught automatically, both locally (`make mutation-go`, `npm run mutation`) and in CI, before merge.
- Mutation testing complements the existing rule against `mock.Anything`/`mock.AnythingOfType` (which addresses weak call-argument assertions): together they cover both major classes of "test that doesn't actually test anything."
- Scoping to `domain`/`application` keeps the Go mutation job fast and CGO-free, independent of the Wails build.

**Negative:**
- Two more required CI jobs; total PR feedback time increases, though scoping and coverage-guided execution keep this bounded while the codebase is small.
- CI must handle the empty-package case gracefully (`internal/domain`/`internal/application` have no `.go` files yet) — a skip check was added to `mutation-go` for this transitional period.
- Thresholds are static config values today; if the team lets them go stale (never ratcheted up as the domain grows), mutation testing loses value over time in the same way an un-raised coverage threshold would — this needs the same periodic-review discipline as the 80% coverage gate.
