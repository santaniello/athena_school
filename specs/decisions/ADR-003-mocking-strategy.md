# ADR-003 — Mocking Strategy

**Status:** Accepted
**Date:** 2026-08-11

---

## Context

`AGENTS.md` already mandates "Use Mockery to generate mocks from interfaces" for the Go backend, but the tool has never been wired up: there is no `.mockery.yaml`, no Makefile target, and no generated mock in the repository. `internal/domain`, `internal/application`, and `internal/infrastructure` are still empty scaffolding (`.gitkeep` only), so this is the cheapest point to configure the tool — before any interface exists to mock — mirroring the reasoning in [ADR-002](ADR-002-mutation-testing.md) for introducing quality gates early.

The frontend (React + TypeScript, tested with Vitest + React Testing Library) has no HTTP client anywhere in `frontend/src` — no `fetch`, no `axios`. The app only talks to the backend through Wails-generated bindings (`frontend/wailsjs/go`), which are plain JS/TS modules, not network calls. A request-mocking library (e.g. MSW) has nothing to intercept today.

Separately, `ci.yml`'s coverage step and `.gremlins.yaml`'s mutation scope were both written before generated code existed in the module tree. Per ADR-002, `gremlins unleash <path>` recurses into subpackages on its own — so once mocks live under `internal/domain/mocks` or `internal/application/mocks`, an unscoped `gremlins unleash ./internal/domain` would mutate the generated mock files too, producing meaningless "surviving mutant" noise on code nobody hand-wrote.

---

## Decision

**Backend (Go):** [Mockery](https://vektra.github.io/mockery/) (`go run github.com/vektra/mockery/v2@v2.53.3`, pinned like Gremlins in `mutation-go`), generating `testify/mock`-based mocks (already a dependency) for interfaces in `internal/domain` and `internal/application`. Mocks are generated into a `mocks` subpackage next to each source package (not in-package), with `with-expecter: true` so call expectations are typed — consistent with `AGENTS.md`'s rule against `mock.Anything`/`mock.AnythingOfType`.

**Frontend (React/TS):** Vitest's native `vi.fn()` / `vi.mock()`, with no added library. If the app ever gains real outbound HTTP calls (e.g. a future cloud-sync feature), that is the trigger to reconsider MSW — not before.

**Generated mocks are excluded from both the coverage gate and mutation testing**, in both CI (`ci.yml`) and the local pre-commit hook (`.githooks/pre-commit`):
- The `PACKAGES` filter used for `go test -coverprofile` excludes `mocks` packages explicitly (rather than relying on the implicit fact that a package with no `_test.go` produces no coverage data — that behavior breaks silently if `-coverpkg=./...` is ever added).
- `.gremlins.yaml`'s `exclude-files` list excludes generated mock files by path pattern, since Gremlins recurses into subpackages regardless of directory scoping.

No equivalent exclusion is needed on the frontend: `vi.mock()` mocks are written inline in test files, not generated as separate source files, so there is nothing new for Vitest coverage or Stryker to skip.

---

## Rationale

**1. Mockery + testify/mock, not gomock.**
`testify` is already a dependency and `AGENTS.md` already requires it for assertions; Mockery generates mocks that implement `testify/mock.Mock`, keeping one assertion/mocking vocabulary across the codebase instead of introducing `gomock`'s separate `Controller`/matcher API.

**2. Mocks in a separate `mocks` subpackage, not in-package.**
In-package mocks (`mockery --inpackage`) would compile generated code directly into `internal/domain`/`internal/application`, making it indistinguishable from hand-written code to both the coverage gate and Gremlins. A dedicated subpackage makes the generated boundary explicit and gives the coverage/mutation exclusions a single, stable path pattern to match (`**/mocks/**`).

**3. No frontend mocking library beyond Vitest's built-in.**
MSW (or similar) mocks HTTP requests. This app has none — the frontend/backend boundary is Wails IPC bindings, which `vi.mock('../../wailsjs/go/...')` already handles as an ordinary module mock. Adding MSW now would be tooling with no use case, contradicting `AGENTS.md`'s "no big design upfront" rule.

**4. Excluding generated mocks from coverage/mutation is a correctness fix, not a nice-to-have.**
A mutated line inside a Mockery-generated stub (e.g. flipping a return value in generated boilerplate) doesn't test any business rule — it's a false positive "surviving mutant" that would block a commit for no reason. Likewise, a mocks package inflates the denominator of the coverage percentage with code that was never meant to demonstrate test intent. Both gates exist to catch weak assertions in `internal/domain`/`internal/application`; generated code has no assertions to be weak.

---

## Rejected Alternatives

**gomock (Go):** rejected — would introduce a second mocking API alongside testify/mock for no added benefit; `AGENTS.md` already standardizes on testify.

**In-package mocks (`mockery --inpackage`):** rejected — indistinguishable from hand-written code to coverage/mutation tooling, undermining the exclusion rule.

**MSW (frontend):** rejected for now — no HTTP layer exists to intercept; revisit only if one is added.

**Relying on Gremlins' directory scoping (no explicit `exclude-files`) to keep mocks out of mutation scope:** rejected — per ADR-002, `gremlins unleash <path>` recurses into subpackages on its own, so this would not actually exclude a `mocks` subpackage.

---

## Consequences

**Positive:**
- Mockery is ready to use the moment the first interface is added to `internal/domain` or `internal/application` — no setup delay mid-spec.
- Coverage and mutation-testing gates stay meaningful: they measure only hand-written business logic, not generated boilerplate.
- Frontend stays dependency-light; no unused mocking library to maintain.

**Negative:**
- The `PACKAGES` filter in `ci.yml`/`.githooks/pre-commit` and `.gremlins.yaml`'s `exclude-files` both need a matching `mocks` path pattern going forward — if a future contributor generates mocks into a differently-named directory, the exclusion silently stops working. This should be caught by the coverage/mutation numbers moving unexpectedly, but is worth flagging in review.
