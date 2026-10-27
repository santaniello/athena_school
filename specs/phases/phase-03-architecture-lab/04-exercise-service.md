# Phase 3.4 — Exercise Service

## Goal

The use cases that run one exam attempt: start it, submit it, discard it, and clean up the ones
that were left behind. This is where the exam rules of
[ADR-005](../../decisions/ADR-005-exercise-engine.md) (decision 9) are enforced, on top of the
model and repositories from [3.1](01-challenge-domain-and-persistence.md),
[3.2](02-rubrics.md) and [3.3](03-attempts-and-session-link.md). It also exposes them to the UI
through thin Wails bindings.

Scoring starts in [3.5](05-evaluation-engine.md), so a submitted attempt stays `SUBMITTED` here.

## Decisions

1. **Two entry points use this service.** Starting from the lab catalog (standalone) and starting
   from an accepted offer in a study session. Generating a new challenge on the spot belongs to
   [3.14](14-ai-challenge-generation.md): that spec creates the challenge and then calls
   `StartAttempt` like any other.
2. **One exam at a time in the whole app.** If an attempt is already in progress, starting
   another fails with `ErrAttemptInProgress`, which carries the id of the running attempt. The
   UI shows a warning that starting a new lab discards the running one. Only when the user
   confirms is the call repeated with `DiscardInProgress: true`, which deletes the old attempt and
   starts the new one in a single transaction. The backend never discards without that explicit
   flag.
3. **A challenge needs a rubric to be started.** Without one, `StartAttempt` fails with
   `ErrChallengeHasNoRubric`. The attempt freezes the challenge's latest rubric version.
4. **A session start uses the accepted link; it never creates it.** When `SessionID` is given, the
   service loads the `SessionExercise` for that session and challenge, created when the user
   accepted the offer ([3.14](14-ai-challenge-generation.md)), and the attempt carries it. A session
   that does not exist fails with `ErrSourceSessionNotFound`; one that is not linked to the
   challenge fails with `ErrSessionExerciseNotFound`. A standalone start has no link.
5. **The UI submits at the deadline, the backend checks the clock.** The diagram exists only in
   the UI, so the countdown reaching zero makes the UI call `SubmitAttempt`. The backend decides
   the outcome from its own time:
   - before the deadline: accepted, reason `USER`
   - from the deadline to 15 seconds after it: accepted, reason `TIME_EXPIRED`
   - later than that: refused with `ErrSubmissionTooLate` and the attempt is discarded
6. **Leaving discards.** `DiscardAttempt` deletes an unfinished attempt when the user leaves the
   lab: one in any state other than `FINISHED`. It never deletes a `FINISHED` one. The UI asks for
   confirmation before calling it (UI spec [3.8](08-lab-layout-sketch.md)). The state
   `FINISHED` is introduced by [3.12](12-architecture-defense.md); until then no attempt is
   finished, so a submitted one is also removed when the user leaves or the app restarts (the exam
   is one sitting, and nothing waits).
7. **Crashes are cleaned at startup.** `DiscardStaleAttempts` deletes every unfinished attempt
   (any state other than `FINISHED`) and runs once when the application starts, before the UI can start a new one. It
   logs how many it removed and never fails the startup: an error is logged, not returned.
   The evaluating and defense states of [3.5](05-evaluation-engine.md) and
   [3.12](12-architecture-defense.md) fall under the same rule.
8. **The clock is a port.** The service reads time through a `Clock` interface, so the deadline,
   the grace period and the too-late rule are testable without sleeping.
9. **No data compatibility to keep.**

## Application

`internal/application/exercise.AttemptService`, next to `ChallengeService` and `RubricService`:

```go
type StartInput struct {
    ChallengeID       string
    SessionID         string // "" for a standalone attempt
    DiscardInProgress bool
}

func (s *AttemptService) StartAttempt(ctx, StartInput) (AttemptView, error)
func (s *AttemptService) SubmitAttempt(ctx, attemptID, contentType, content string) (AttemptView, error)
func (s *AttemptService) DiscardAttempt(ctx, attemptID string) error
func (s *AttemptService) DiscardStaleAttempts(ctx) (int, error)
func (s *AttemptService) GetAttempt(ctx, attemptID string) (AttemptView, error)
func (s *AttemptService) ListAttempts(ctx, challengeID string) ([]AttemptView, error)
```

`AttemptView` carries the attempt, the challenge's title and topic, the rubric version's criteria
and, once submitted, the submission. It is the read model the bindings return; the challenge
statement is read through `ChallengeService`, not copied into it.

`StartAttempt` order of work:

1. Load the challenge (`ErrChallengeNotFound`) and its latest rubric
   (`ErrChallengeHasNoRubric`).
2. If a session id is given, check the session exists through the narrow `SessionExists` port
   (`ErrSourceSessionNotFound`), so the package does not import `study`.
3. `FindInProgress`: if there is one and `DiscardInProgress` is false, return
   `ErrAttemptInProgress`.
4. In one transaction: delete the running attempt if the flag was set, create the attempt with
   `NewAttempt`. A violation of the one-running-exam index of
   [3.3](03-attempts-and-session-link.md) is mapped to `ErrAttemptInProgress`, so two concurrent
   starts cannot both succeed. The existing transactor pattern
   (`application/study/transactor.go`) is reused for the port.

`SubmitAttempt`: loads the attempt (`ErrAttemptNotFound`), refuses one that is not in progress
(`ErrAttemptNotInProgress`), applies the deadline rule from decision 5, calls `Attempt.Submit`
and `AttemptRepository.Submit`. On `ErrSubmissionTooLate` it deletes the attempt before
returning.

All errors are exported sentinels in `application/exercise/errors.go`.

## Desktop bindings

Thin adapters in `internal/interfaces/desktop/exercise.go`: validate input, call the service,
map `AttemptView` to a DTO, return a clear error code per sentinel. No rule lives here.

- `StartAttempt(challengeId, sessionId, discardInProgress)`
- `SubmitAttempt(attemptId, contentType, content)`
- `DiscardAttempt(attemptId)`
- `GetAttempt(attemptId)` and `ListAttempts(challengeId)`

`App.startup` calls `DiscardStaleAttempts` once, before the frontend's first call. `wailsjs` is
regenerated. There is no screen in this spec.

## Tasks

Each slice is TDD, keeps the build green and is committed on its own.

- [ ] Application: `Clock` and `SessionExists` ports, `AttemptView`, error sentinels
- [ ] `StartAttempt`: standalone, rubric required, challenge not found
- [ ] `StartAttempt`: in-progress conflict, and `DiscardInProgress` replacing it atomically
- [ ] `StartAttempt`: session start loads the accepted `SessionExercise` and never creates it;
      unknown session and unlinked session; two concurrent starts leave one attempt
- [ ] `SubmitAttempt`: user submit, grace period, too late discards, not in progress
- [ ] `DiscardAttempt` and `DiscardStaleAttempts` (any state other than `FINISHED`)
- [ ] `GetAttempt` and `ListAttempts` read models
- [ ] Bindings, `startup` cleanup hook, `wailsjs` regenerated, mocks regenerated
- [ ] Docs: CHANGELOG entry

## Acceptance Criteria

- Starting from the catalog creates a standalone attempt with `Deadline` at start plus the
  challenge's limit, frozen to the latest rubric version.
- Starting on a challenge with no rubric fails with `ErrChallengeHasNoRubric` and stores nothing.
- With an attempt running, starting another fails with `ErrAttemptInProgress` and the running
  attempt is untouched. Repeating with `DiscardInProgress` deletes it and creates the new one; if
  the new one cannot be created, the old one is still there.
- Starting with a session id uses the existing `SessionExercise` of that session and challenge and
  creates none. An unknown session fails with `ErrSourceSessionNotFound` and a session not linked
  to the challenge fails with `ErrSessionExerciseNotFound`; both store nothing.
- A submission before the deadline is stored as `USER`; at the deadline or up to 15 seconds after,
  as `TIME_EXPIRED`; later than that it fails with `ErrSubmissionTooLate` and the attempt is gone.
- A second submit fails with `ErrAttemptNotInProgress` and stores nothing.
- `DiscardAttempt` removes an attempt in any state other than `FINISHED` and refuses a `FINISHED`
  one.
- `DiscardStaleAttempts` removes every unfinished attempt, keeps every `FINISHED` one and does not
  stop the app starting when it errors.
- All of the above run against a fake `Clock`; no test sleeps.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks pass.

## Out of scope

- Evaluating a submission, the defense and completion ([3.5](05-evaluation-engine.md),
  [3.12](12-architecture-defense.md), [3.6](06-completion-rules.md)).
- The catalog and the lab screens ([3.7](07-challenge-catalog-ui.md),
  [3.8](08-lab-layout-sketch.md)), including the discard warnings.
- Generating challenges ([3.14](14-ai-challenge-generation.md)) and chat offers
  ([3.15](15-open-lab-from-chat.md)).
- Pause, resume, drafts, a backend copy of the diagram while drawing.
