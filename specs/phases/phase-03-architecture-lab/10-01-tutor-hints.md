# Phase 3.10.1 — Tutor Hints (during the exam)

## Goal

Let the user ask the tutor for a hint while the exam runs, up to three per attempt. A hint points
in a direction or asks a question; it never gives the solution. The hint panel of
[3.8](08-lab-layout-sketch.md) is wired to a real backend here.

Hints are the only tutor interaction during the exam. The tutor's evaluating questions come after
submitting ([3.12](12-architecture-defense.md)).

## Decisions

1. **At most 3 hints per attempt,** counted and enforced by the backend, not by the screen. The
   limit is a constant of the exercise engine (`MaxHints = 3`).
2. **A hint is a direction or a question, never a solution.** It does not name the component to
   use, describe the architecture to draw, or grade the diagram. Example: "What happens to the
   data if this service goes down?" The prompt carries the rule and a test asserts the
   instruction is present; the prompt is the only control ([3.6.1](06-01-challenge-generation.md)
   works the same way).
3. **The tutor sees the current board.** The screen sends the latest XML with the request, since
   it lives only in memory ([3.9](09-diagram-editor-spike.md)), together with the challenge, the
   rubric and the hints already given. It is bounded by the 2 MiB submission limit. A request with
   no board content still works: the tutor then points at how to start.
4. **Hints do not affect the score or the result.** The score comes only from the rubric
   ([3.5](05-evaluation-engine.md)). The number of hints used is stored and shown in the result
   screen ([3.13](13-evaluation-and-history-screen.md)), nothing more.
5. **A hint is spent only when delivered.** A provider error or a cancelled call stores nothing
   and does not use one of the three. One request per attempt at a time; a second while one runs
   is refused.
6. **Only while the exam runs.** A request for an attempt that is not in progress, or whose
   deadline has passed, is refused.
7. **Hints belong to the attempt.** Discarding an attempt ([3.4](04-exercise-service.md)) deletes
   its hints with it. A finished attempt keeps them, read-only.
8. **Language and length.** The hint is in the user's assistant language and short (at most about
   60 words). Neither is guaranteed by code; both are in the prompt.
9. **Cost is one LLM call per hint,** with the Architecture Lab model
   ([3.6.1](06-01-challenge-generation.md) decision 10) through the existing provider and usage
   accounting ([1.5](../phase-01-desktop-mvp/05-llm-service.md)).
10. **No data compatibility to keep.**

## Domain

`internal/domain/exercise/`:

```go
const MaxHints = 3

type Hint struct {
    ID        string
    AttemptID string
    Position  int // 1..MaxHints, in the order given
    Content   string
    CreatedAt time.Time
}
```

`ErrNoHintsLeft`, `ErrHintInProgress`. `HintRepository` (`Add`, `ListByAttempt`) with `Add`
refusing a fourth hint for the same attempt even under concurrency (the check and the insert are
one statement or one transaction).

## Application

`internal/application/exercise.HintService`:

```go
func (s *HintService) RequestHint(ctx, attemptID, boardXML string) (Hint, error)
func (s *HintService) ListHints(ctx, attemptID string) ([]Hint, error)
```

`RequestHint`: load the attempt (`ErrAttemptNotFound`), require it to be in progress and before its
deadline (`ErrAttemptNotInProgress`), refuse when `MaxHints` were given (`ErrNoHintsLeft`) or one
is running (`ErrHintInProgress`), call the `HintGenerator` with no transaction open, then store the
hint. Errors are exported sentinels.

`HintGenerator` is a port in the exercise package. Its architecture implementation lives in
`internal/application/architecture` (created here; [3.11](11-architecture-evaluator.md) adds the
evaluator beside it) so the generic engine knows nothing about diagrams. The prompt is a pure
function of the challenge, the rubric, the board XML and the previous hints.

## SQLite

```sql
CREATE TABLE attempt_hints (
  id         TEXT PRIMARY KEY,
  attempt_id TEXT NOT NULL REFERENCES challenge_attempts(id),
  position   INTEGER NOT NULL,
  content    TEXT NOT NULL,
  created_at TEXT NOT NULL,
  UNIQUE (attempt_id, position)
);
```

`DeleteInProgress` and `DeleteAllInProgress` ([3.3](03-attempts-and-session-link.md)) also delete
the attempt's hints in the same transaction. The `UNIQUE (attempt_id, position)` plus a
position limit makes the cap hold at the database level.

## Desktop bindings and frontend

- Bindings in `internal/interfaces/desktop/exercise.go`: `RequestHint(attemptId, boardXml)` and
  `ListHints(attemptId)`, thin, with an error code per sentinel. `wailsjs` regenerated.
- `lib/exercise.ts` wrappers. `LabExamScreen` ([3.10](10-diagram-workspace.md)) passes the hints,
  the counter and an `onAskHint` handler to `lab-tutor-panel` ([3.8](08-lab-layout-sketch.md)).
  While a request runs the button shows progress and is disabled; on failure the panel shows
  "Could not get a hint. Try again." and the counter is unchanged.
- All UI text is in English.

## Tasks

Each slice is TDD (Go tests with testify and Mockery mocks; Vitest for the frontend), keeps the
build green and is committed on its own.

- [ ] Domain: `Hint`, `MaxHints`, errors
- [ ] SQLite: migration, `HintRepository`, the cap at the database level, delete with the attempt
- [ ] Application: `RequestHint` success, limit, not in progress, past the deadline, concurrent
      request refused, provider error stores nothing; `ListHints`
- [ ] Architecture `HintGenerator`: pure prompt (language, length, no-solution rule, board XML,
      previous hints) and the LLM call with usage accounting
- [ ] Bindings, `wailsjs` and mocks regenerated
- [ ] Frontend: wrappers and the panel wired in `LabExamScreen`, error and progress states
- [ ] Docs: CHANGELOG

## Acceptance Criteria

- The first three requests return a hint each; the fourth fails with `ErrNoHintsLeft` and calls
  no LLM.
- A failed request leaves the counter and the table unchanged and can be repeated.
- A second request while one is running is refused.
- A request after the deadline or on a submitted or discarded attempt is refused.
- Discarding the attempt removes its hints; a submitted attempt keeps them.
- The prompt contains the assistant language, the length limit and the instruction not to reveal a
  solution, and includes the board XML and earlier hints.
- The panel shows each hint as a message, the counter goes 3, 2, 1, none, the button disappears at
  zero, and a failure shows the retry message without spending a hint.
- No score or result changes depending on how many hints were used.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks, and the frontend tests, coverage, lint,
  typecheck, `npx prettier --check .` and Stryker on the changed files pass.

## Out of scope

- Free chat with the tutor during the exam, and any hint beyond three.
- The evaluating questions after submitting ([3.12](12-architecture-defense.md)).
- Penalties, hint-aware scoring, and showing hints in the result
  ([3.13](13-evaluation-and-history-screen.md) owns that screen).
- Verifying that a hint hides the solution beyond the prompt rule.
