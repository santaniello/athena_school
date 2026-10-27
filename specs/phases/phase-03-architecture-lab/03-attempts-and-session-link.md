# Phase 3.3 — Attempts, Submissions and the Session Link

## Goal

An attempt is one exam sitting on a challenge. This spec adds the `ChallengeAttempt` and
`Submission` model, the time limit, the `SessionExercise` link between a study session and a
challenge, and their SQLite schema. It stores and guards the data; the use cases that start,
submit and discard attempts are [3.4](04-exercise-service.md), and scoring is
[3.5](05-evaluation-engine.md).

Part of the Exercise Engine ([ADR-005](../../decisions/ADR-005-exercise-engine.md), decisions 5
and 9).

## Decisions

1. **An attempt is an exam, not a draft.** No pause, no resume, no intermediate versions. The
   diagram being drawn lives only in the UI. The backend stores no work in progress, only the
   attempt record (who, which challenge, when it started, when it must end).
2. **The backend owns the clock.** Starting an attempt records `StartedAt` and
   `Deadline = StartedAt + the challenge's time limit`. The UI shows a countdown but never decides
   when the attempt ends.
3. **Time limit per challenge.** `challenges` gains `time_limit_minutes`, filled on creation
   from the difficulty (30 / 45 / 60 minutes for beginner / intermediate / advanced), adjustable
   per challenge from 1 to 180. The attempt copies the limit through `Deadline`, so a later edit
   never changes a running or past attempt.
4. **Abandoning discards.** An attempt that is not finished (any state other than `FINISHED`, which
   [3.12](12-architecture-defense.md) introduces) when the user leaves the lab,
   closes the app or the app crashes is deleted, with no submission, score or history entry.
   The repository exposes the deletion; the use cases that trigger it (on leave and at startup)
   are 3.4.
5. **Only the states this phase needs now.** `IN_PROGRESS` and `SUBMITTED` are defined here. The
   states for evaluation and defense are added by [3.5](05-evaluation-engine.md) and
   [3.12](12-architecture-defense.md), each with its transitions. An attempt has no `CANCELLED`
   state, because discarding deletes it.
6. **Submitting is the only way a diagram is stored.** A submission is created together with the
   `IN_PROGRESS → SUBMITTED` transition, in one transaction, and is immutable. It records why the
   attempt ended: `USER` (clicked submit) or `TIME_EXPIRED`.
7. **The artifact is an opaque payload.** A `Submission` stores a content type string, the
   content as text and a SHA-256 checksum. `exercise` never parses it: the diagram format is
   decided by the editor spike ([3.9](09-diagram-editor-spike.md)) and interpreted only by the
   architecture package. It is stored in SQLite, not in files, so there are no paths to validate
   and no permissions to manage. Content is bounded (2 MiB).
8. **`SessionExercise` is created when the user accepts**, not when the tutor proposes
   (offers and the accept/decline flow belong to [3.14](14-ai-challenge-generation.md)). It links one
   session to one challenge, once. An attempt started from a session carries its
   `SessionExerciseID`; a standalone attempt has none, and the context is derived from that, not
   stored.
9. **Deleting a session removes only its links.** `session_exercises` cascades from `sessions`;
   attempts reference it with `ON DELETE SET NULL`, so attempts, submissions and evaluations stay
   in the global history (ADR-005, decision 5).
10. **Practicing again is a new attempt** on the same challenge. A challenge can have many
    attempts; a submitted attempt is never reopened.
11. **No data compatibility to keep.** The tables are new; the `challenges` column is added by
    migration with the default below.

## Domain

`internal/domain/exercise/`:

```go
type AttemptState string // AttemptInProgress, AttemptSubmitted

type SubmitReason string // SubmitReasonUser, SubmitReasonTimeExpired

type ChallengeAttempt struct {
    ID                string
    ChallengeID       string
    RubricVersionID   string    // the version frozen when the attempt started (3.2)
    SessionExerciseID string    // "" for a standalone attempt
    State             AttemptState
    StartedAt         time.Time
    Deadline          time.Time
    SubmittedAt       time.Time // zero until submitted
    SubmitReason      SubmitReason
}

type Submission struct {
    ID          string
    AttemptID   string
    ContentType string
    Content     string
    Checksum    string // hex SHA-256 of Content
    SubmittedAt time.Time
}

type SessionExercise struct {
    ID          string
    SessionID   string
    ChallengeID string
    MessageID   string // the assistant message the chat card hangs under (3.17)
    CreatedAt   time.Time
}
```

Rules, each tested and each with its own error:

- `NewAttempt(challenge, rubricVersion, sessionExerciseID, now)` sets `Deadline` from the
  challenge's time limit; the rubric version must belong to the challenge.
- `Attempt.Expired(now)` is true from the deadline on.
- `Attempt.Submit(content, reason, now)` is allowed only from `IN_PROGRESS`, requires non-empty
  content within the size limit and returns the attempt and the `Submission` with the checksum.
  Submitting twice fails.
- `Challenge.TimeLimitMinutes` is validated from 1 to 180; `DefaultTimeLimit(difficulty)`
  returns 30, 45 or 60.

Ports:

```go
type AttemptRepository interface {
    Create(ctx, ChallengeAttempt) error
    Get(ctx, id string) (ChallengeAttempt, error)         // ErrAttemptNotFound
    Submit(ctx, ChallengeAttempt, Submission) error       // one transaction; fails if not IN_PROGRESS
    ListByChallenge(ctx, challengeID string) ([]ChallengeAttempt, error)
    FindInProgress(ctx) (ChallengeAttempt, error)         // ErrAttemptNotFound when none
    DeleteUnfinished(ctx, id string) error                // any state except FINISHED, with its children
    DeleteAllUnfinished(ctx) (int, error)
}
type SubmissionRepository interface {
    GetByAttempt(ctx, attemptID string) (Submission, error) // ErrSubmissionNotFound
}
type SessionExerciseRepository interface {
    Create(ctx, SessionExercise) error                    // ErrSessionExerciseExists
    GetBySessionAndChallenge(ctx, sessionID, challengeID string) (SessionExercise, error)
    ListBySession(ctx, sessionID string) ([]SessionExercise, error)
}
```

`DeleteUnfinished` and `DeleteAllUnfinished` delete an attempt in any state other than `FINISHED`
(`FINISHED` is added by [3.12](12-architecture-defense.md); until then every attempt is unfinished),
together with its submission and any child rows ([3.5](05-evaluation-engine.md),
[3.10.1](10-01-tutor-hints.md), [3.12](12-architecture-defense.md)), in one transaction. A
`FINISHED` attempt is never touched.

## SQLite

New migration:

```sql
ALTER TABLE challenges ADD COLUMN time_limit_minutes INTEGER NOT NULL DEFAULT 45;

CREATE TABLE session_exercises (
  id           TEXT PRIMARY KEY,
  session_id   TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  challenge_id TEXT NOT NULL REFERENCES challenges(id),
  message_id   TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  created_at   TEXT NOT NULL,
  UNIQUE (session_id, challenge_id)
);
CREATE TABLE challenge_attempts (
  id                  TEXT PRIMARY KEY,
  challenge_id        TEXT NOT NULL REFERENCES challenges(id),
  rubric_version_id   TEXT NOT NULL REFERENCES rubric_versions(id),
  session_exercise_id TEXT REFERENCES session_exercises(id) ON DELETE SET NULL,
  state               TEXT NOT NULL,
  started_at          TEXT NOT NULL,
  deadline            TEXT NOT NULL,
  submitted_at        TEXT,
  submit_reason       TEXT
);
CREATE INDEX idx_attempts_challenge ON challenge_attempts(challenge_id);
-- at most one exam running: the database holds the rule, not only the service
CREATE UNIQUE INDEX idx_attempts_one_in_progress ON challenge_attempts(state) WHERE state = 'IN_PROGRESS';
CREATE TABLE submissions (
  id           TEXT PRIMARY KEY,
  attempt_id   TEXT NOT NULL UNIQUE REFERENCES challenge_attempts(id),
  content_type TEXT NOT NULL,
  content      TEXT NOT NULL,
  checksum     TEXT NOT NULL,
  submitted_at TEXT NOT NULL
);
```

Nothing cascades from `challenge_attempts` or `submissions`, except that deleting an in-progress
attempt has no submission to remove. `Submit` updates the attempt and inserts the submission in a
single transaction, and the update only matches a row still in `IN_PROGRESS`, so a concurrent or
repeated submit cannot create a second submission. Repositories live under
`internal/infrastructure/sqlite/`; mocks go to `exercise/mocks` (ADR-003).

The spec 3.1 challenge repository and `Challenge` gain `TimeLimitMinutes`, with the validation
above.

## Tasks

Each slice is TDD, keeps the build green and is committed on its own.

- [ ] Domain: `Challenge.TimeLimitMinutes` and `DefaultTimeLimit`, with validation
- [ ] Domain: `ChallengeAttempt`, `NewAttempt`, `Expired`, errors
- [ ] Domain: `Submission` and `Attempt.Submit` (state, size limit, checksum, reason)
- [ ] Domain: `SessionExercise` and ports
- [ ] SQLite: migration (column and three tables) and the `challenges` repository change
- [ ] SQLite: `AttemptRepository` (`Create`, `Get`, `Submit` in one transaction,
      `ListByChallenge`, `FindInProgress`)
- [ ] SQLite: `DeleteUnfinished` and `DeleteAllUnfinished`, never touching a `FINISHED` attempt
- [ ] SQLite: `SubmissionRepository` and `SessionExerciseRepository`
- [ ] SQLite: deleting a session removes its `SessionExercise` rows and keeps attempts,
      submissions and challenges
- [ ] Mocks regenerated; docs: CHANGELOG entry

## Acceptance Criteria

- An attempt gets `Deadline = StartedAt + time limit`, and editing the challenge's limit afterward
  leaves the stored deadline unchanged.
- `Expired` is false before the deadline and true at and after it.
- Submitting an in-progress attempt stores one immutable submission with the correct checksum,
  moves the attempt to `SUBMITTED` and records the reason, atomically; a second submit fails and
  stores nothing.
- Submitting empty or oversized content fails and leaves the attempt `IN_PROGRESS`.
- `DeleteUnfinished` and `DeleteAllUnfinished` remove attempts in any state other than `FINISHED`
  with their submission and child rows; a `FINISHED` attempt is never removed.
- A second `IN_PROGRESS` attempt cannot be stored: the unique index refuses it.
- A challenge can have several attempts; `ListByChallenge` returns them.
- `FindInProgress` returns the one in-progress attempt, or `ErrAttemptNotFound` when there is none.
- Creating a second `SessionExercise` for the same session and challenge fails with
  `ErrSessionExerciseExists`.
- Deleting a session removes its `SessionExercise` rows; the attempts that used them remain with an
  empty `SessionExerciseID`, and their submissions are readable.
- Challenges created without an explicit limit get 30, 45 or 60 minutes by difficulty; values
  outside 1–180 are rejected.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  domain/application code, lint and vulnerability checks pass.

## Out of scope

- Starting, submitting and discarding as use cases, expiry handling and the startup cleanup
  ([3.4](04-exercise-service.md)).
- Evaluation, defense and completion states and transitions ([3.5](05-evaluation-engine.md),
  [3.12](12-architecture-defense.md), [3.6](06-completion-rules.md)).
- The diagram format and the editor ([3.9](09-diagram-editor-spike.md),
  [3.10](10-diagram-workspace.md)).
- Offers, accept/decline and the chat blocking ([3.15](15-open-lab-from-chat.md)).
- Pause, resume, drafts, autosave, snapshots.
- An attempt modality (guided or assessment): every attempt is an exam, so the field of the
  original idea is dropped.
