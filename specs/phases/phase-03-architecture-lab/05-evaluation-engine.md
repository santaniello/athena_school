# Phase 3.5 — Evaluation Engine

## Goal

Score a submitted attempt against its frozen rubric. This spec adds the `Evaluation` and
`CriterionResult` model, the deterministic score calculation, the evaluation states of an attempt
and the `EvaluationService` that runs an evaluator and stores its result. The evaluator itself,
the part that reads a diagram and calls the LLM, is [3.11](11-architecture-evaluator.md); here it
is a port with a fake in the tests.

Part of the Exercise Engine ([ADR-005](../../decisions/ADR-005-exercise-engine.md)): nothing here
knows what a diagram is.

## Decisions

1. **The model chooses levels, the code computes the score.** The evaluator returns, for each
   criterion of the frozen rubric, a level from 0 to 4 and a justification. The domain computes
   the score as the weighted sum `Σ weight × level / 4`, rounded half up to a whole number from 0
   to 100 (criteria flagged `InsufficientEvidence` are handled as in decision 3). The model never
   writes a number the domain trusts, which makes the score reproducible and testable.
2. **The evaluator's output is validated before anything is stored.** Exactly one result per
   criterion of the attempt's rubric version, no unknown or repeated criterion, a level 0 to 4, a
   non-empty justification, bounded text. Anything else fails the evaluation as
   `ErrInvalidEvaluation` and nothing is stored.
3. **Insufficient evidence is explicit and is not judged.** A criterion result may be flagged
   `InsufficientEvidence` when the submission does not give enough to judge it, for example a
   decision the user did not write on the diagram. The score measures knowledge, not a perfect
   drawing, and the tutor's questions in [3.12](12-architecture-defense.md) settle exactly these
   doubts. So a flagged criterion is **left out** of the score: the score is computed over the
   judged criteria and rescaled to 100, `round(100 × Σ weight × level / (4 × Σ judged weight))`.
   Its stored level is 0 and is ignored. To keep a nearly empty diagram from scoring high, when the
   judged criteria hold less than half of the total weight, the rule falls back to the full
   rubric with the flagged criteria counting as level 0. The flag is shown to the user, so "not
   shown" can be told apart from "wrong".
4. **One evaluation per attempt and kind, and it is final.** A successful evaluation is never
   re-run, replaced or edited (ADR-005, decision 6). The kind is `DIAGRAM` now;
   `UNDERSTANDING` is added by [3.12](12-architecture-defense.md) on the same tables.
5. **Failure leaves the attempt pending, and the user decides.** If the evaluator fails or its
   output is invalid, the attempt moves to `EVALUATION_FAILED` with a short reason, and the screen
   offers **Try evaluating again** while the user stays on it. There are no hidden automatic
   retries and no limit on manual ones. The submission is not lost while the screen is open;
   leaving it discards the attempt (decision 7).
6. **New attempt states:** `SUBMITTED → EVALUATING → EVALUATED`, and
   `EVALUATING → EVALUATION_FAILED → EVALUATING` for a retry. A second concurrent start on the
   same attempt is refused.
7. **The exam is one sitting, so an unfinished evaluation is discarded.** Drawing, evaluation and
   the tutor's questions ([3.12](12-architecture-defense.md)) are one continuous flow with no
   "waiting" status in between. Leaving the screen, closing the app or a crash while an attempt is
   in `EVALUATING` or `EVALUATION_FAILED` deletes it: the user takes the exam again. At startup
   these attempts are deleted next to the in-progress cleanup of
   [3.4](04-exercise-service.md).
8. **Technical result and pedagogical text are separate fields.** Scores and levels are the
   technical result; summary, strengths, gaps and recommendations are the feedback text. The
   feedback never changes a score.
9. **Cost is one evaluator call per attempt** in the success path, plus one per manual retry.
10. **No data compatibility to keep.**

## Domain

`internal/domain/exercise/`:

```go
type EvaluationKind string // EvaluationKindDiagram

type CriterionResult struct {
    CriterionID          string  // the criterion of the frozen rubric version
    Level                int     // 0..4
    Justification        string
    Evidence             string  // optional quote or reference into the submission
    InsufficientEvidence bool
}

type Evaluation struct {
    ID              string
    AttemptID       string
    Kind            EvaluationKind
    Model           string  // exact model id that produced it
    Score           int     // 0..100, computed by the domain
    Summary         string
    Strengths       []string
    Gaps            []string
    Recommendations []string
    Results         []CriterionResult
    CreatedAt       time.Time
}
```

Pure functions, each tested:

- `ComputeScore(rubric RubricVersion, results []CriterionResult) (int, error)`
- `NewEvaluation(attempt, rubric, output EvaluatorOutput, model string, now time.Time)
  (Evaluation, error)` runs the validation of decision 2 and computes the score.

Attempt: new states `AttemptEvaluating`, `AttemptEvaluated`, `AttemptEvaluationFailed`; the
methods `StartEvaluation`, `CompleteEvaluation` and `FailEvaluation(reason)` allow only the
transitions of decision 6. The attempt gains `FailureReason string`, kept until the next retry.

Ports:

```go
// Evaluator is implemented per lab. 3.11 provides the architecture one.
type Evaluator interface {
    Evaluate(ctx context.Context, in EvaluatorInput) (EvaluatorOutput, error)
}

type EvaluatorInput struct {
    Challenge  Challenge
    Rubric     RubricVersion
    Submission Submission // opaque content, interpreted only by the implementation
}

type EvaluatorOutput struct {
    Model           string
    Summary         string
    Strengths       []string
    Gaps            []string
    Recommendations []string
    Results         []CriterionResult
}

type EvaluationRepository interface {
    Create(ctx, Evaluation) error                              // one per attempt and kind
    GetByAttempt(ctx, attemptID string, kind EvaluationKind) (Evaluation, error) // ErrEvaluationNotFound
}
```

## Application

`internal/application/exercise.EvaluationService`:

- `Evaluate(ctx, attemptID) (Evaluation, error)`:
  1. Load the attempt; allowed only from `SUBMITTED` or `EVALUATION_FAILED`
     (`ErrAttemptNotEvaluable`). An already evaluated attempt returns its stored evaluation
     without calling the evaluator.
  2. Move to `EVALUATING` with one conditional update (`UPDATE ... WHERE id = ? AND state IN
     ('SUBMITTED', 'EVALUATION_FAILED')`) and check that one row changed. Only the call that changed
     it goes on; a concurrent call changes no row and is refused with `ErrAttemptNotEvaluable`
     without calling the evaluator, so two calls can never both reach it.
  3. Load the challenge, the frozen rubric version and the submission, and call the evaluator
     with no database transaction open.
  4. On success, build the evaluation with `NewEvaluation`, then store it, record the minimum
     passing score in force on the attempt ([3.6](06-completion-rules.md)) and move the attempt
     to `EVALUATED`, all in one transaction.
  5. On an evaluator error or `ErrInvalidEvaluation`, move to `EVALUATION_FAILED` with the
     reason and return the error. A cancelled context counts as a failure the same way.
- `GetEvaluation(ctx, attemptID)` returns the stored evaluation or `ErrEvaluationNotFound`.
- `DiscardUnfinishedEvaluations(ctx) (int, error)` implements decision 7 for attempts left in
  `EVALUATING` or `EVALUATION_FAILED`, and is called at startup with the stale-attempt cleanup.
  An error is logged, not returned. `DiscardAttempt` ([3.4](04-exercise-service.md)) also accepts
  an attempt in these states, for the Leave button.

The bindings (`EvaluateAttempt`, `GetEvaluation`) are added in [3.11](11-architecture-evaluator.md),
when a real evaluator exists. This spec has no binding.

## SQLite

New migration; `challenge_attempts` gains the failure reason:

```sql
ALTER TABLE challenge_attempts ADD COLUMN failure_reason TEXT;

CREATE TABLE evaluations (
  id              TEXT PRIMARY KEY,
  attempt_id      TEXT NOT NULL REFERENCES challenge_attempts(id),
  kind            TEXT NOT NULL,
  model           TEXT NOT NULL,
  score           INTEGER NOT NULL,
  summary         TEXT NOT NULL,
  strengths       TEXT NOT NULL,   -- JSON arrays, see jsonlist.go
  gaps            TEXT NOT NULL,
  recommendations TEXT NOT NULL,
  created_at      TEXT NOT NULL,
  UNIQUE (attempt_id, kind)
);
CREATE TABLE criterion_results (
  evaluation_id         TEXT NOT NULL REFERENCES evaluations(id),
  criterion_id          TEXT NOT NULL REFERENCES rubric_criteria(id),
  level                 INTEGER NOT NULL,
  justification         TEXT NOT NULL,
  evidence              TEXT NOT NULL,
  insufficient_evidence INTEGER NOT NULL,
  PRIMARY KEY (evaluation_id, criterion_id)
);
```

Nothing cascades, so history is never removed by a side effect. The evaluation and its criterion
results are written in one transaction with the attempt's state change. The attempt repository
gains the state update methods and `DeleteUnfinishedEvaluations`, which deletes every attempt in
`EVALUATING` or `EVALUATION_FAILED` with its submission and hints.

## Tasks

Each slice is TDD, keeps the build green and is committed on its own.

- [ ] Domain: `ComputeScore` (weights, rounding half up, bounds, flagged criteria left out and
      rescaled, the half-weight fallback)
- [ ] Domain: `NewEvaluation` validation of the evaluator output
- [ ] Domain: attempt evaluation states and transitions, `FailureReason`
- [ ] SQLite: migration and `EvaluationRepository`; attempt state updates and
      `DeleteUnfinishedEvaluations`
- [ ] Application: `Evaluate` success path with a fake evaluator
- [ ] Application: evaluator error, invalid output and cancelled context leave
      `EVALUATION_FAILED`; retry from there; no call when already evaluated
- [ ] Application: concurrent evaluation refused; `DiscardUnfinishedEvaluations`
- [ ] Mocks regenerated; docs: CHANGELOG entry

## Acceptance Criteria

- The score is the weighted sum of levels over 4, rounded half up, and is the same for the same
  input; all-zero levels give 0 and all-4 levels give 100.
- A criterion flagged `InsufficientEvidence` is left out and the score is rescaled over the judged
  weight: with five criteria of weight 20 and one flagged, levels 4, 4, 3, 3 give 88, not 70.
- When the judged criteria hold less than half of the total weight, the flagged ones count as
  level 0 over the full rubric.
- Output with a missing, unknown or repeated criterion, a level outside 0 to 4, an empty
  justification or oversized text is rejected and nothing is stored.
- A successful evaluation stores the evaluation, its criterion results and the `EVALUATED` state
  atomically, and records the model id.
- When the evaluator errors, returns invalid output or the context is cancelled, the attempt ends
  in `EVALUATION_FAILED` with a reason, the submission is intact and no evaluation exists.
- From `EVALUATION_FAILED`, `Evaluate` runs again and can succeed; on an `EVALUATED` attempt it
  returns the stored evaluation and does not call the evaluator.
- While an attempt is `EVALUATING`, a second `Evaluate` on it is refused.
- At startup, attempts left in `EVALUATING` or `EVALUATION_FAILED` are deleted with their
  submission; evaluated ones are unchanged. Discarding one of these through Leave works too.
- The evaluator is called with no open database transaction.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  domain/application code, lint and vulnerability checks pass.

## Out of scope

- The real evaluator, its prompt and the diagram input ([3.11](11-architecture-evaluator.md)),
  and the bindings that expose evaluation.
- The `UNDERSTANDING` evaluation and the defense ([3.12](12-architecture-defense.md)).
- Pass or fail and progress status ([3.6](06-completion-rules.md)).
- Re-evaluating a finished evaluation, automatic retries, a user-editable score.
- Any screen ([3.13](13-evaluation-and-history-screen.md)).
