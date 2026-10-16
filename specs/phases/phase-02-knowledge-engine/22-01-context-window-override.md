# Phase 2.22.1 — Development Context-Window Override

## Goal

The model window Athena measures against comes from the OpenRouter catalog, and the default tutor
model reports 1,000,000 tokens. Reaching the 80% warning or the 95% block of spec 2.6 by hand would
take hundreds of thousands of tokens of conversation, so those states (and, later, compaction —
spec 2.25) are impractical to exercise manually. This spec adds a switch that makes every model
report a small, chosen window, and can be turned on and off without touching code.

## Decisions

1. **An environment variable, off by default.** `ATHENA_CONTEXT_WINDOW=<tokens>` turns it on.
   Unset, empty, `0` or `off` (any case) is off. A positive integer is the window in tokens. Any
   other value (negative, decimal, text) is logged and **ignored** — a typo never silently turns
   the override on or off.
2. **Development builds only.** The variable is read in `context_override_dev.go`
   (`//go:build !production`). `wails build` adds the `production` tag, so
   `context_override_prod.go` returns an empty setting and the real catalog window is the only
   limit. `wails dev` and `go run` do not set `production`, so the override works there.
3. **A decorator at the composition root.** `modelcatalog.OverrideResolver` wraps the catalog
   `Service` and reports the overridden length for every non-blank model ID, with no I/O. A blank
   model ID still delegates to the wrapped resolver, so an unresolved model is never given an
   invented limit. Domain and application code (`study.Service`) are unchanged: they only see a
   `domainllm.ModelContextResolver`. The catalog `Warm` call still uses the real service.
4. **Visible when on.** A startup `log.Printf("WARNING: ATHENA_CONTEXT_WINDOW is on …")` makes an
   accidentally exported variable easy to spot.
5. **When it takes effect.** A session whose persisted limit is unknown (new sessions, and
   sessions with `context_length = 0`) resolves to the override as soon as it is opened or its
   opening turn is measured. A session that already persisted a real limit keeps it until its next
   send, because spec 2.6 re-measures only then (and recomputes in either direction if the limit
   changed). This is the existing 2.6 behavior, not something the override adds.

## Usage

```
ATHENA_CONTEXT_WINDOW=20000 make dev   # on: warning at 16,000 tokens, block at 19,000
make dev                               # off: real catalog windows
```

## Tasks

1. `modelcatalog.ParseContextWindowOverride` and `OverrideResolver` (unit tests first).
2. Build-tag files and `newContextResolver` in the `main` package; wire it into `study.NewService`.
3. `CHANGELOG.md`.

## Acceptance Criteria

- With the variable unset, empty, `0` or `off`, behavior is identical to before.
- With a positive value, a new session measures against that window and shows the warning and
  blocked states of spec 2.6 at 80% and 95% of it.
- An invalid value is logged and ignored.
- A `production`-tagged build ignores the variable even when it is set.
- Both build variants compile (`go build -tags webkit2_41` and `-tags webkit2_41,production`).

## Out of Scope

- A UI toggle, a persisted setting, per-model overrides.
- Changing how or when spec 2.6 re-measures a session that already has a persisted limit.
