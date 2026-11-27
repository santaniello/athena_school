# Phase 6.3 — Module Migration

## Goal

Make every module log through the central infrastructure: inject a module-scoped `*slog.Logger`,
replace the remaining `log.Printf` calls with structured events and add the relevant Storage and AI
Engine events. After this increment the spec's criterion "all modules use the same logging
infrastructure" holds.

Depends on [6.2](02-config-and-bootstrap.md). It can ship in several PRs, one step at a time; modules
not yet migrated keep using `log.Printf` and nothing breaks in between.

## Decisions

1. **Injection, no globals.** Constructors in `application` and `interfaces/desktop` receive a
   `*slog.Logger` already scoped with `With("module", "<name>")`, built in `main.go`. They depend
   on the standard library only, so ADR-001 holds. No `slog.SetDefault`, no package-level logger.
   `domain` never logs.
2. **Tests** pass `slog.New(slog.DiscardHandler)`, never `nil`. Where an event is the behavior
   under test, the test uses a handler that captures records into a buffer.
3. **Modules and names:** `application`, `storage`, `config`, `study-session`, `arch-lab`,
   `exercise-engine`, `quiz`, `mind-map`, `progress`, `ai-engine`, `openrouter`. The name set may
   grow when a module has no fitting one; the format stays `kebab-case`.
4. **Level per situation:**
   - ERROR: an operation failed and the user or data is affected.
   - WARN: unexpected but non-blocking (the existing "…unavailable" and "dropping orphaned chunk"
     messages are WARN).
   - INFO: start/finish of relevant operations and lifecycle.
   - DEBUG: investigation detail only.
5. **What to log:** start and end of relevant operations, failures, unexpected behavior and the
   identifiers of the entities involved (`session_id`, `challenge_id`, `attempt_id`). Not every
   method call. Never prompts, responses, notes, study content or credentials (6.1 redaction is a
   safety net, not a license). Errors from external libraries are passed through the redaction.
6. **Migration of existing calls:** each `log.Printf` becomes `logger.<Level>(msg, attrs…)` with
   the module and the useful identifiers as attributes instead of formatted into the message.
   Existing log tests keep their meaning.
7. **After the last step no `log.Printf`/`log.Print`/`log.Println`/`log.Fatalf` of the standard
   package remains in `internal/`, `main.go` or `context_override.go`.** A test or lint rule (e.g.
   `depguard`/`forbidigo` for the `log` package) enforces it.

## Steps

Each step is one PR with its own tests.

| Step | Scope | Existing `log.*` calls |
|---|---|---|
| 6.3.1 | `application` services: `study`, `knowledge`, `ingest`, `reset`, `progress` | 15 |
| 6.3.2 | `interfaces/desktop` bindings: `study`, `ingest`, `exercise`, `quiz`, `mindmap` | 10 |
| 6.3.3 | AI Engine: `openrouter` client logs provider, model, request start/end, `duration_ms`, tokens when available, errors, timeouts, retries and fallback (no prompt or response content) | new |
| 6.3.4 | Storage: `sqlite` initialization, migrations, connection failures and persistence errors, as `storage` events; then the lint rule that forbids the standard `log` package | new |

The AI Engine and Storage steps add new events rather than migrating calls, and are the only ones
with non-trivial design; keep them small and limited to the events listed above.

## Acceptance Criteria

- Every module logs through an injected, module-scoped logger; every event carries its `module`.
- No standard-library `log` call remains outside the logging package's own `stderr` fallback,
  enforced by lint or test.
- OpenRouter requests are logged with provider, model, duration, status and tokens (when
  available), with no prompt, response or credential in the file.
- Storage initialization, migrations and persistence errors appear as `storage` events.
- A failing module operation produces an ERROR with the relevant identifier and no sensitive
  content.
- Constructors take the logger as an explicit argument; tests pass a discard logger.
- Same quality gate as 6.1; mutation testing applies to changed `internal/application` code.
