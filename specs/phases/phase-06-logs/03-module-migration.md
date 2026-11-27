# Phase 6.3 — Module Migration

## Goal

Make every module log through the central infrastructure: inject a module-scoped `*slog.Logger`,
replace the remaining `log.Printf` calls with structured events and add the relevant Storage and AI
Engine events. After this increment the spec's criterion "all modules use the same logging
infrastructure" holds.

Depends on [6.2](02-config-and-bootstrap.md). It can ship in several PRs, one step at a time; modules
not yet migrated keep using `log.Printf` and nothing breaks in between.

## Why this increment also covers hangs (incident, 2026-11-27)

While testing Phase 7.2, the study chat and the Architecture Lab challenge generation hung for minutes
with a loading animation and **no error anywhere**. Nothing could be read from the app: it wrote no
log file, and the few `log.Printf` calls it had (37, all in unrelated failure paths) never ran. The
cause was found only by capturing a goroutine dump by hand (`kill -QUIT`) and pasting it from the
terminal.

The cause was a deadlock, not a failure. The SQLite pool has a single connection
(`SetMaxOpenConns(1)`). `SendMessage` reads the session's folder inside its transaction, but
`FolderRepository.GetByID` used the pool directly instead of `execer(ctx, r.db)`, so it waited for a
second connection while the transaction held the only one. The Architecture Lab hit the same path
when saving a generated challenge. The model call itself finished in about 30 seconds (visible only
on the OpenRouter dashboard); the app then waited forever. Fixed in PR #206.

Lessons that shape this spec:

- A log records what the code decides to record. A hang is the absence of an event, so a log made only
  of failures and of "finished" events cannot show it. Operations must also record when they
  **start**, and something must notice when one does not end.
- A wait with no timeout never becomes an error, so it never reaches an ERROR event. Waits that can
  block forever must be bounded.
- The first thing needed in such an incident is the log file on disk, readable without asking the user
  to copy a terminal.

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

8. **Operation tracing.** Every relevant operation records an INFO "started" event when it begins
   and an INFO "finished" (or ERROR "failed") event when it ends, both with `operation`,
   the identifiers of the entities involved and, on the end event, `duration_ms`. The operations are
   at least: sending a study message, generating a challenge, accepting an exercise, quiz or mind map
   offer, generating a quiz and a mind map, starting and evaluating an attempt, the defense turns and
   every `WithinTx` transaction (named by its caller). An event that starts without ever ending is
   how a hang shows up in the file.
9. **Slow-operation watchdog.** An operation still running after `slow_operation_seconds` (default
   30, configured under `logging:` in 6.2) records one WARN "operation still running" with its
   `operation`, identifiers and `elapsed_ms`, then keeps waiting. It records nothing when the
   operation ends first, and never cancels or alters it. It is implemented once, in a small
   standard-library-only helper in `internal/application` (for example `oplog`), so that
   `application` still does not import `infrastructure` (ADR-001). The helper returns a function the
   caller defers.
10. **Goroutine dump on the first slow event.** The first time the watchdog fires in a run it
    also records the goroutine dump (`runtime/pprof` goroutine profile) as one WARN event, truncated
    to a fixed size (for example 64 KB) and written once per run. This replaces capturing it by hand
    with `SIGQUIT`, whose output goes to `stderr` and is lost when the app is not started from a
    terminal. The dump contains function names and addresses, not study content; it is still passed
    through the redaction of 6.1.
11. **Database pool health.** The `storage` module records a WARN when waiting for a connection
    takes longer than `db_wait_warn_seconds` (default 5), with `wait_ms`, and the pool statistics
    (`in_use`, `open`, `wait_count`, `wait_duration_ms` from `sql.DB.Stats()`) on the slow-operation
    and shutdown events. With a single connection, any wait longer than a few milliseconds means
    something is holding it.
12. **Bounded waits.** The SQLite adapter bounds the wait for a connection to `db_wait_timeout_seconds`
    (default 30). When it expires the call fails with an error that names the cause ("timed out
    waiting for a database connection"), which the caller logs as ERROR, instead of waiting forever.
    The bound applies only to acquiring the connection, never to a statement, a model call or a whole
    transaction, so a legitimately long operation is not cut. This is a behavior change, not only
    logging, and ships with its own regression test (a repository method that ignores the
    transaction in the context must fail with this error within the bound).
13. **Never silent in the file.** Operations that run in a goroutine started by the application
    (streaming a reply, offer generation, background indexing) recover from a panic, record it as
    an ERROR with the stack and end the operation event, so the file never ends with a start and
    no explanation after a crash.

## Steps

Each step is one PR with its own tests.

| Step | Scope | Existing `log.*` calls |
|---|---|---|
| 6.3.1 | `application` services: `study`, `knowledge`, `ingest`, `reset`, `progress` | 15 |
| 6.3.2 | `interfaces/desktop` bindings: `study`, `ingest`, `exercise`, `quiz`, `mindmap` | 10 |
| 6.3.3 | AI Engine: `openrouter` client logs provider, model, request start/end, `duration_ms`, tokens when available, errors, timeouts, retries and fallback (no prompt or response content) | new |
| 6.3.4 | Storage: `sqlite` initialization, migrations, connection failures and persistence errors, as `storage` events; pool health and bounded connection wait (decisions 11 and 12); then the lint rule that forbids the standard `log` package | new |
| 6.3.5 | Operation tracing, slow-operation watchdog and goroutine dump (decisions 8, 9, 10 and 13) for the operations listed in decision 8 | new |

The AI Engine, Storage and Operation tracing steps add new events rather than migrating calls, and
are the only ones with non-trivial design; keep them small and limited to the events listed above.
6.3.5 can ship before 6.3.1 to 6.3.3 for the operations it covers, since it needs only the logger of
6.2: it is what would have made the incident above visible.

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
- Every operation of decision 8 writes a "started" event and a "finished" or "failed" event with
  `duration_ms`; a test makes an operation block and checks that only the start event exists.
- An operation that runs longer than `slow_operation_seconds` writes exactly one WARN with its
  identifiers, and a fast one writes none; a test uses a short threshold. The first slow event
  also writes one truncated goroutine dump.
- Regression for the incident: with a repository method that ignores the transaction inside
  `WithinTx` (a test double), the file shows the operation "started", a WARN for the slow
  operation, and an ERROR "timed out waiting for a database connection", and the operation fails
  instead of hanging.
- A panic in a goroutine started by the application is logged as ERROR with its stack.
- The incident is reproducible from the log alone: someone who reads only the file can tell which
  operation was running, for how long, and where it was waiting.
