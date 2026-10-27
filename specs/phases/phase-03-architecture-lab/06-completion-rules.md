# Phase 3.6 — Completion Rules

## Goal

Separate "submitted" from "passed". This spec adds the pass policy, the per-attempt result, the
per-challenge progress status and the minimum-score setting the user can change in Settings.

It is pure domain and application logic over the evaluations of
[3.5](05-evaluation-engine.md); the defense that produces the second score is
[3.12](12-architecture-defense.md).

## Decisions

1. **One minimum score for every challenge, default 80.** It is a user setting, not a field of
   the challenge or of the difficulty. It ranges from 50 to 100.
2. **An attempt passes only if all of these hold:**
   - the diagram score is at least the minimum score;
   - the understanding score is at least the minimum score;
   - every mandatory criterion ([3.2](02-rubrics.md)) is at level 2 or higher in **both**
     evaluations, except that a criterion flagged `InsufficientEvidence` in an evaluation
     ([3.5](05-evaluation-engine.md)) is not checked there. It must then be judged in the other
     evaluation: a mandatory criterion flagged in **both** evaluations counts as not met, since
     nothing ever showed the user's knowledge of it.

   Both scores must pass on their own. They are never averaged, so a good diagram cannot make up
   for a defense the user could not give, or the other way around. The level 2 floor for
   mandatory criteria is fixed in code.
3. **The minimum score is frozen per attempt.** When the diagram evaluation completes, the
   attempt records the minimum score in force at that moment, and the same value is used for the
   understanding score. Changing the setting later never changes the result of a past attempt.
4. **The result says why.** A result is `PENDING`, `PASSED` or `FAILED`, and a failed one lists
   its reasons: diagram score below the minimum, understanding score below the minimum, and each
   mandatory criterion that fell short in each evaluation. The result screen
   ([3.13](13-evaluation-and-history-screen.md)) shows them.
5. **`PENDING` until both evaluations exist.** With no understanding evaluation yet, the result
   is `PENDING`, never `PASSED` or `FAILED`. The defense is not optional for completing a
   challenge (ADR-005), so a challenge cannot be completed until [3.12](12-architecture-defense.md)
   delivers it. `PENDING` exists only inside a running sitting and is never stored: the exam is one
   sitting and only a finished attempt is kept ([3.12](12-architecture-defense.md)).
6. **Progress is derived per challenge, never stored.** Computed from its attempts:
   - `NOT_STARTED`: no finished attempt and none running
   - `IN_PROGRESS`: an attempt is running, or one is submitted and not yet `PASSED` or `FAILED`
   - `NEEDS_IMPROVEMENT`: at least one finished attempt and none passed
   - `COMPLETED`: at least one attempt `PASSED`

   A later failed attempt never removes a completion. The summary also carries the attempt count,
   the best diagram score and the date of the first pass.
7. **Settings placement.** The minimum score lives with the other application settings and is
   edited in the existing Settings screen ([1.8](../phase-01-desktop-mvp/08-settings.md)).
8. **No data compatibility to keep.**

## Domain

`internal/domain/exercise/`:

```go
type AttemptResult string // ResultPending, ResultPassed, ResultFailed

type FailureReason struct {
    Kind        FailureKind // DiagramScore, UnderstandingScore, MandatoryCriterion
    Evaluation  EvaluationKind
    CriterionID string // set for MandatoryCriterion
}

type AttemptOutcome struct {
    Result  AttemptResult
    Reasons []FailureReason
}

type ProgressStatus string // NotStarted, InProgress, NeedsImprovement, Completed

type ChallengeProgress struct {
    Status          ProgressStatus
    Attempts        int
    BestDiagramScore int       // 0 when none
    FirstPassedAt   time.Time  // zero when never
}

const MandatoryMinLevel = 2
const MinPassScore, MaxPassScore, DefaultPassScore = 50, 100, 80
```

Pure functions, each tested:

- `EvaluateOutcome(passScore int, rubric RubricVersion, diagram, understanding *Evaluation)
  AttemptOutcome`: `PENDING` while either evaluation is missing; otherwise `PASSED`, or `FAILED`
  with every reason that applies, not only the first.
- `ValidatePassScore(int) error`
- `DeriveProgress(attempts []AttemptSummary) ChallengeProgress`, where an `AttemptSummary` is an
  attempt, whether it is running, and its outcome.

The attempt gains `PassScore int`, set when the diagram evaluation completes (extends step 4 of
`Evaluate` in 3.5, which now reads it through the `PassScoreProvider` port below).

## Application

- `ProgressService.GetChallengeProgress(ctx, challengeID)` and `GetAttemptOutcome(ctx, attemptID)`
  read the stored attempts, evaluations and rubric, and call the domain functions.
- `PassScoreProvider.Current(ctx) int` is a port; the settings service implements it.
- `SettingsService` gains `GetPassScore` and `SetPassScore`: validate with `ValidatePassScore`
  (`ErrInvalidPassScore`), persist with the existing settings store, and return the default when
  nothing is stored.
- `PassScore` and the attempt outcome are exposed by the bindings added in later specs; this spec
  adds only the settings bindings.

## Persistence

`challenge_attempts` gains the frozen value:

```sql
ALTER TABLE challenge_attempts ADD COLUMN pass_score INTEGER;  -- NULL until the diagram is evaluated
```

The minimum score setting is stored where the application's other settings live
(`~/.athena/config.yaml` through the existing config service), file permission `0600` like the
rest. Nothing else is persisted: results and progress are computed on read.

## Settings screen

One new field in the existing Settings screen:

- **Minimum passing score**, a number input from 50 to 100, default 80, with a short helper text:
  "A challenge is passed when both the diagram and your explanation reach this score. Changing it
  does not affect attempts you already finished."
- Saving an out-of-range value shows the validation message and stores nothing.
- It follows the same save pattern, copy tone and test style as the neighboring settings.

This is the first thing in the phase the user can see.

## Tasks

Each slice is TDD, keeps the build green and is committed on its own.

- [ ] Domain: `ValidatePassScore`, constants, `EvaluateOutcome` (pending, passed, failed with
      all reasons, mandatory level 2 in both evaluations)
- [ ] Domain: `DeriveProgress` (all four statuses, later failure keeps completion, best score,
      first pass date)
- [ ] Application: settings `GetPassScore` and `SetPassScore`, default and validation
- [ ] SQLite: `pass_score` column; set it when the diagram evaluation completes
      (`PassScoreProvider` wired into `Evaluate`)
- [ ] Application: `ProgressService` (`GetChallengeProgress`, `GetAttemptOutcome`)
- [ ] Bindings `GetPassScore` and `SetPassScore`; `wailsjs` regenerated; mocks regenerated
- [ ] Frontend: Settings field with validation, save and error states (Vitest)
- [ ] Docs: CHANGELOG, README and `lib/documentation.ts` mention of the setting

## Acceptance Criteria

- With no understanding evaluation the outcome is `PENDING`, even when the diagram score is high.
- With both evaluations at or above the minimum and all mandatory criteria at level 2 or higher,
  the outcome is `PASSED`.
- A diagram score of 86 with a mandatory criterion at level 0 is `FAILED` with that criterion in
  the reasons; if the understanding score is also below the minimum, both reasons are listed.
- A mandatory criterion flagged `InsufficientEvidence` in the diagram and judged at level 3 in the
  defense does not fail the attempt; flagged in both evaluations, it does, with that criterion in
  the reasons.
- Scores are never averaged: diagram 95 with understanding 70 at a minimum of 80 is `FAILED`.
- A score exactly at the minimum passes.
- Changing the minimum score after an attempt's diagram evaluation does not change that
  attempt's outcome.
- Progress is `NOT_STARTED`, `IN_PROGRESS`, `NEEDS_IMPROVEMENT` or `COMPLETED` as defined, a
  later failed attempt keeps `COMPLETED`, and the best score, attempt count and first pass date
  are correct.
- The setting returns 80 until changed, rejects values below 50 or above 100, survives a
  restart, and the Settings screen shows the stored value and saves a valid change.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  domain/application code, lint, vulnerability checks, and the frontend tests, coverage, lint,
  typecheck and `npx prettier --check .` pass.

## Out of scope

- The defense and the understanding evaluation ([3.12](12-architecture-defense.md)); until it
  ships, no attempt can leave `PENDING`.
- The catalog, result screens and history ([3.7](07-challenge-catalog-ui.md),
  [3.13](13-evaluation-and-history-screen.md)) that display this data.
- A per-challenge or per-difficulty minimum, a configurable mandatory level, averaging rules.
- Re-grading old attempts after the setting changes.
